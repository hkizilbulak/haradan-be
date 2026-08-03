# Haradan Phase-One PostgreSQL Veri Modeli Blueprint’i

## 1. Belgenin amacı ve kapsamı

Bu belge kavramsal değerlendirme değildir. Phase-one backend için exact PostgreSQL veri modelini kilitler:

- Tablo adları (`hrd_` prefix)
- Kolonlar ve PostgreSQL tipleri
- Nullability / default
- PK, FK, UNIQUE, CHECK
- Index planı
- JSONB sınırları
- Soft-delete / deactivation / physical delete
- Migration sırası
- Shared test DB güvenlik sınırı

Çalıştırılabilir SQL, Go kodu, OpenAPI veya use-case listesi üretilmez. Kaynak: `docs/01`–`docs/12` kabul edilmiş kararlar.

## 2. Çapraz tutarlılık sonucu

| Sonuç | Açıklama |
| --- | --- |
| Çelişki (şema engelleyici) | Yok |
| BLOCKER | Yok |
| Price | Sabit nullable `price_amount_minor` + `price_currency` (JSONB dışı) |
| Advert draft | Kısmi kayıt: category/district/title/description nullable; tamlık status CHECK ile |
| Seller DISABLED/CLOSED × PUBLISHED | Açık ürün; şema engellemez |
| Pasif category × PUBLISHED | Açık ürün; şema engellemez |
| Raw TJK payload retention | Açık; canonical horse tablosuna raw kolon eklenmez |
| Telefon zorunluluğu | Açık; `phone` nullable |
| Migration metadata kolonları | Tool seçimine bağlı; domain şeması blocker değil |

## 3. PostgreSQL ve shared DB standartları

| Kural | Karar |
| --- | --- |
| Tablo/index/constraint/sequence/migration adları | Hepsi `hrd_` ile başlar |
| `hr_` izolasyonu | Haradan dokunmaz; FK/join yok |
| Kimlik | Uygulama üretir `uuid` (extension zorunlu değil) |
| Zaman | `timestamptz`, UTC |
| Enum-like | `varchar` + CHECK (native ENUM yok) |
| Role | `varchar` + CHECK (`user` \| `admin`) |
| Migration metadata adı | `hrd_schema_migrations` (tool-owned; kolonlar tool’a bağlı) |
| Extension | İlk fazda zorunlu değil |
| Para | Integer minor unit; float yok |
| Secret | Belge/Git/log’a yazılmaz; `.env` okunmaz |
| AutoMigrate | Ortak DB’de production migration mekanizması değildir |

## 4. Phase-one tablo envanteri

| Domain | Exact table | Sorumluluk | Neden ayrı? |
| --- | --- | --- | --- |
| Meta | `hrd_schema_migrations` | Migration-tool-owned version tracking | Shared DB metadata çakışması |
| IAM | `hrd_users` | Kullanıcı, rol, status, credential | Domain root |
| IAM | `hrd_auth_sessions` | Refresh session, rotation, client context | 1:N |
| IAM | `hrd_one_time_credentials` | Email verify/change + password reset | Purpose-based |
| IAM | `hrd_security_events` | Minimum auth/security audit | Actor/subject |
| GEO | `hrd_provinces` | İl referansı | Ayrı |
| GEO | `hrd_districts` | İlçe + province FK | Advert yalnız district |
| CATALOG | `hrd_categories` | Ağaç | Self-parent |
| CATALOG | `hrd_category_properties` | Property definitions | Values JSONB’de |
| HORSE | `hrd_horses` | Canonical TJK horse | Advert’e kopyalanmaz |
| ADVERT | `hrd_adverts` | İlan write model | Aggregate root |
| ADVERT | `hrd_advert_status_history` | Immutable transitions | Ayrı |
| ADVERT | `hrd_favorites` | User–advert N:N | İlişki |
| MEDIA | `hrd_media_assets` | Raw/master lifecycle | Shared asset |
| MEDIA | `hrd_media_variants` | Transform variants | Ayrı lifecycle |
| MEDIA | `hrd_advert_media` | Advert–asset relation | Banner’dan ayrı |
| BANNER | `hrd_banners` | Placement banner | Shared asset |
| WORKER | `hrd_background_jobs` | Durable job queue | Broker yok |
| TJK | `hrd_tjk_sync_runs` | Sync run | Job’dan ayrı |
| TJK | `hrd_tjk_sync_item_errors` | Failed/conflict detayı | Success audit yok |

**Tablo/metadata sayısı:** 19 application/domain + 1 migration-tool-owned = 20.

## 5. Exact check/value setleri

| Set | Exact değerler |
| --- | --- |
| User role | `user`, `admin` |
| User status | `ACTIVE`, `DISABLED`, `CLOSED` |
| Client context | `PUBLIC_WEB`, `MOBILE`, `ADMIN_BO` |
| One-time purpose | `EMAIL_VERIFICATION`, `EMAIL_CHANGE_VERIFICATION`, `PASSWORD_RESET` |
| Advert status | `DRAFT`, `PENDING_REVIEW`, `CHANGES_REQUESTED`, `PUBLISHED`, `REJECTED`, `SUSPENDED`, `SOLD`, `ARCHIVED` |
| Banner status | `ACTIVE`, `INACTIVE` |
| Banner placement | `HOMEPAGE`, `LISTING_DETAIL`, `SEARCH` |
| Media asset lifecycle | `UPLOAD_PENDING`, `UPLOADED`, `VALIDATING`, `MASTER_READY`, `VALIDATION_FAILED`, `CLEANUP_CANDIDATE`, `DELETING`, `PHYSICALLY_DELETED` |
| Media variant lifecycle | `PENDING`, `PROCESSING`, `READY`, `FAILED`, `DELETING`, `PHYSICALLY_DELETED` |
| Logical media usage | `DETAIL`, `HOMEPAGE`, `SEARCH` |
| Property data type | `STRING`, `TEXT`, `INTEGER`, `DECIMAL`, `BOOLEAN`, `SINGLE_SELECT`, `YEAR` |
| Job type | `TJK_SYNC_BATCH`, `MEDIA_VALIDATE_AND_NORMALIZE`, `MEDIA_GENERATE_VARIANT`, `MEDIA_DELETE_OBJECTS`, `MEDIA_RECONCILE` |
| Job status | `QUEUED`, `LEASED`, `SUCCEEDED`, `FAILED`, `CANCELLED`, `DEAD` |

**Job status semantiği (exact set değişmez):**

| Status | Anlam |
| --- | --- |
| `QUEUED` | Claim veya retry zamanını bekliyor; `completed_at` NULL; lease alanları NULL |
| `LEASED` | Worker claim etmiş ve işliyor; `lease_owner` + `leased_until` zorunlu; `completed_at` NULL |
| `SUCCEEDED` | Başarıyla terminal tamamlandı; `completed_at` zorunlu |
| `FAILED` | Kalıcı/non-retryable hata ile terminal başarısız (ör. geçersiz/desteklenmeyen payload, kalıcı validation); `completed_at` zorunlu. `DEAD` ile aynı değildir |
| `DEAD` | Retry edilebilir hata tekrarlandı; retry bütçesi tükendi; terminal; `attempt_count = max_attempts` application transition kuralıdır; `completed_at` zorunlu |
| `CANCELLED` | Cooperative cancellation güvenli sınırda tamamlandı; `cancel_requested_at` ve `completed_at` zorunlu |

Transient hata ve retry hakkı varsa: job doğrudan terminal `FAILED` yapılmaz; lease temizlenir; yeni `available_at` belirlenir; status tekrar `QUEUED` olur; `last_error` korunur; attempt sayacı ilerletilir. `last_error` secret veya kontrolsüz payload içermez. Allowed job transition graph application/worker transaction sorumluluğudur.

| Set | Exact değerler |
| --- | --- |
| TJK sync mode | `FULL`, `INCREMENTAL`, `RECONCILIATION` |
| TJK sync run status | `QUEUED`, `RUNNING`, `SUCCEEDED`, `PARTIAL_SUCCESS`, `FAILED`, `CANCELLED` |
| TJK trigger kind | `SCHEDULED`, `MANUAL` |
| TJK sync scope | `HORSES` |
| TJK item error class | `TRANSIENT`, `PERMANENT`, `CONFLICT` |
| TJK item error status | `OPEN`, `RESOLVED`, `IGNORED` |
| Security event type | `LOGIN_SUCCESS`, `LOGIN_FAILURE`, `LOGOUT`, `SESSION_REVOKED`, `ALL_SESSIONS_REVOKED`, `REFRESH_REPLAY_DETECTED`, `PASSWORD_CHANGE`, `PASSWORD_RESET`, `EMAIL_VERIFICATION`, `EMAIL_CHANGE`, `ROLE_CHANGE`, `ACCOUNT_STATUS_CHANGE`, `BO_CONTEXT_REJECTED` |

## 6. IAM tabloları

### `hrd_schema_migrations`

**Amaç:** Migration-tool-owned version tracking. Application/domain tablosu değildir.

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| *(tool-defined)* | *(tool-defined)* | — | — | — | Kolon yapısı seçilecek migration aracına bağlıdır |

Exact nesne adı: `hrd_schema_migrations`. Kolonlar domain gibi kilitlenmez. Tool custom table name desteklemeli; `hr_` metadata’ya dokunmamalı.

### `hrd_users`

**Amaç:** Kullanıcı kimliği, rol, hesap status, password hash, ad/soyad.

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | App-generated |
| email | varchar(320) | NO | — | — | Display email |
| email_normalized | varchar(320) | NO | — | UNIQUE | Trim+lower login key |
| password_hash | varchar(255) | NO | — | — | Argon2id/bcrypt |
| role | varchar(16) | NO | `'user'` | CHECK | `user`/`admin` |
| status | varchar(16) | NO | `'ACTIVE'` | CHECK | Account status |
| email_verified_at | timestamptz | YES | NULL | — | Verify ayrı alan |
| first_name | varchar(100) | NO | — | — | Profil adı |
| last_name | varchar(100) | NO | — | — | Profil soyadı |
| phone | varchar(32) | YES | NULL | — | İlk faz zorunlu değil |
| security_stamp | uuid | NO | — | — | Global revoke/version |
| failed_login_count | integer | NO | `0` | CHECK >= 0 | Brute-force sayaç |
| locked_until | timestamptz | YES | NULL | — | Geçici lock |
| created_at | timestamptz | NO | — | — | UTC |
| updated_at | timestamptz | NO | — | — | UTC |

- PK: `hrd_users_pkey`
- UNIQUE: `hrd_users_email_normalized_key`
- CHECK: `hrd_users_role_check`, `hrd_users_status_check`, `hrd_users_failed_login_count_check`
- Index: `hrd_users_status_idx`
- Display name yok; projection `first_name`/`last_name` üretebilir

### `hrd_auth_sessions`

**Amaç:** Server-side refresh session + rotation family.

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | Session id |
| user_id | uuid | NO | — | FK → `hrd_users.id` ON DELETE RESTRICT | Owner |
| client_context | varchar(16) | NO | — | CHECK | PUBLIC_WEB/MOBILE/ADMIN_BO |
| refresh_token_hash | varchar(255) | NO | — | UNIQUE | Hash only |
| family_id | uuid | NO | — | — | Rotation family |
| replaced_by_session_id | uuid | YES | NULL | FK → `hrd_auth_sessions.id` ON DELETE SET NULL | Rotation zinciri |
| absolute_expires_at | timestamptz | NO | — | — | Absolute TTL |
| idle_expires_at | timestamptz | NO | — | — | Idle TTL |
| revoked_at | timestamptz | YES | NULL | — | Explicit revoke |
| revoke_reason | varchar(64) | YES | NULL | — | Opsiyonel |
| created_at | timestamptz | NO | — | — | — |
| last_used_at | timestamptz | NO | — | — | Heartbeat |
| user_agent | varchar(512) | YES | NULL | — | Opsiyonel |
| ip_hash | varchar(128) | YES | NULL | — | Düz IP yok |

- PK: `hrd_auth_sessions_pkey`
- FK: `hrd_auth_sessions_user_id_fkey`, `hrd_auth_sessions_replaced_by_session_id_fkey`
- UNIQUE: `hrd_auth_sessions_refresh_token_hash_key`
- CHECK: `hrd_auth_sessions_client_context_check`, `hrd_auth_sessions_idle_le_absolute_check`, `hrd_auth_sessions_created_le_last_used_check`, `hrd_auth_sessions_revoke_reason_requires_revoked_at_check`, `hrd_auth_sessions_no_self_replace_check`
- Indexes: `hrd_auth_sessions_user_id_idx`, `hrd_auth_sessions_family_id_idx`, `hrd_auth_sessions_active_lookup_idx`

### `hrd_one_time_credentials`

**Amaç:** Email verification, email change verification, password reset.

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | — |
| user_id | uuid | NO | — | FK → `hrd_users.id` ON DELETE RESTRICT | — |
| purpose | varchar(32) | NO | — | CHECK | Purpose set |
| token_hash | varchar(255) | NO | — | UNIQUE | Hash only |
| target_email | varchar(320) | YES | NULL | — | Verify/change hedefi |
| target_email_normalized | varchar(320) | YES | NULL | — | Normalize hedef |
| expires_at | timestamptz | NO | — | — | — |
| consumed_at | timestamptz | YES | NULL | — | Tek kullanımlık |
| invalidated_at | timestamptz | YES | NULL | — | Supersede |
| created_at | timestamptz | NO | — | — | — |
| request_ip_hash | varchar(128) | YES | NULL | — | Opsiyonel |

- PK: `hrd_one_time_credentials_pkey`
- FK: `hrd_one_time_credentials_user_id_fkey`
- UNIQUE: `hrd_one_time_credentials_token_hash_key`
- PARTIAL UNIQUE: `hrd_one_time_credentials_one_active_per_user_purpose_key` (`user_id`, `purpose`) WHERE `consumed_at IS NULL AND invalidated_at IS NULL` — aktif lookup’ı da bu unique destekler; ayrı non-unique active index yok
- CHECK: `hrd_one_time_credentials_purpose_check`, `hrd_one_time_credentials_target_email_by_purpose_check`, `hrd_one_time_credentials_consumed_xor_invalidated_check`, `hrd_one_time_credentials_expires_after_created_check`

### `hrd_security_events`

**Amaç:** Minimum güvenlik/audit; actor/subject ayrımı.

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | — |
| subject_user_id | uuid | YES | NULL | FK → `hrd_users.id` ON DELETE SET NULL | Etkilenen |
| actor_user_id | uuid | YES | NULL | FK → `hrd_users.id` ON DELETE SET NULL | Yapan |
| event_type | varchar(64) | NO | — | CHECK | Event type set |
| client_context | varchar(16) | YES | NULL | CHECK | NULL veya client context set |
| metadata | jsonb | NO | `'{}'` | CHECK object | Secret yok |
| created_at | timestamptz | NO | — | — | Immutable |

- PK: `hrd_security_events_pkey`
- FK: `hrd_security_events_subject_user_id_fkey`, `hrd_security_events_actor_user_id_fkey`
- CHECK: `hrd_security_events_event_type_check`, `hrd_security_events_client_context_check`, `hrd_security_events_metadata_object_check`
- Indexes: `hrd_security_events_subject_created_idx`, `hrd_security_events_actor_created_idx`, `hrd_security_events_type_created_idx`

## 7. GEO tabloları

### `hrd_provinces`

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | — |
| name | varchar(120) | NO | — | UNIQUE | Display |
| name_normalized | varchar(120) | NO | — | UNIQUE | Prefix search |
| is_active | boolean | NO | `true` | — | Soft deactivation |
| sort_order | integer | NO | `0` | CHECK >= 0 | — |
| created_at | timestamptz | NO | — | — | — |
| updated_at | timestamptz | NO | — | — | — |

- PK: `hrd_provinces_pkey`
- UNIQUE: `hrd_provinces_name_key`, `hrd_provinces_name_normalized_key`
- CHECK: `hrd_provinces_sort_order_nonnegative_check`
- Index: `hrd_provinces_name_normalized_prefix_idx` (`name_normalized` varchar_pattern_ops)

### `hrd_districts`

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | — |
| province_id | uuid | NO | — | FK → `hrd_provinces.id` ON DELETE RESTRICT | — |
| name | varchar(120) | NO | — | — | Display |
| name_normalized | varchar(120) | NO | — | — | Prefix search |
| is_active | boolean | NO | `true` | — | — |
| sort_order | integer | NO | `0` | CHECK >= 0 | — |
| created_at | timestamptz | NO | — | — | — |
| updated_at | timestamptz | NO | — | — | — |

- PK: `hrd_districts_pkey`
- FK: `hrd_districts_province_id_fkey`
- UNIQUE: `hrd_districts_province_id_name_key`, `hrd_districts_province_id_name_normalized_key`
- CHECK: `hrd_districts_sort_order_nonnegative_check`
- Indexes: `hrd_districts_province_active_sort_idx` (`province_id`, `is_active`, `sort_order` — aktif district dropdown + BO filtreli liste; yalnız `province_id` index redundant olduğu için kaldırıldı), `hrd_districts_name_normalized_prefix_idx` (varchar_pattern_ops)
- Not: `hrd_districts_province_id_name_key` ve `hrd_districts_province_id_name_normalized_key` zaten `province_id` ile başlar; ayrı `hrd_districts_province_id_idx` yoktur

## 8. CATALOG tabloları

### `hrd_categories`

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | — |
| parent_id | uuid | YES | NULL | FK → `hrd_categories.id` ON DELETE RESTRICT | Root NULL |
| slug | varchar(120) | NO | — | UNIQUE | — |
| name | varchar(160) | NO | — | — | — |
| description | text | YES | NULL | — | — |
| is_active | boolean | NO | `true` | — | — |
| sort_order | integer | NO | `0` | CHECK >= 0 | — |
| version | integer | NO | `1` | CHECK > 0 | Optimistic concurrency |
| created_at | timestamptz | NO | — | — | — |
| updated_at | timestamptz | NO | — | — | — |

- PK: `hrd_categories_pkey`
- FK: `hrd_categories_parent_id_fkey`
- UNIQUE: `hrd_categories_slug_key`
- CHECK: `hrd_categories_no_self_parent_check` (`parent_id IS NULL OR parent_id <> id`), `hrd_categories_sort_order_nonnegative_check`, `hrd_categories_version_positive_check`
- Index: `hrd_categories_parent_id_sort_idx`
- Uzun cycle engeli application TX sorumluluğu

### `hrd_category_properties`

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | — |
| category_id | uuid | NO | — | FK → `hrd_categories.id` ON DELETE RESTRICT | — |
| code | varchar(64) | NO | — | — | Immutable after use |
| title | varchar(160) | NO | — | — | — |
| help_text | text | YES | NULL | — | — |
| data_type | varchar(32) | NO | — | CHECK | Property type set |
| is_required | boolean | NO | `false` | — | — |
| is_public_visible | boolean | NO | `true` | — | — |
| is_form_visible | boolean | NO | `true` | — | — |
| is_filterable | boolean | NO | `false` | — | — |
| sort_order | integer | NO | `0` | CHECK >= 0 | — |
| is_active | boolean | NO | `true` | — | — |
| options | jsonb | NO | `'[]'` | CHECK array | — |
| validation | jsonb | NO | `'{}'` | CHECK object | — |
| default_value | jsonb | YES | NULL | — | Type-dependent; no shape CHECK |
| ui_metadata | jsonb | NO | `'{}'` | CHECK object | — |
| version | integer | NO | `1` | CHECK > 0 | — |
| created_at | timestamptz | NO | — | — | — |
| updated_at | timestamptz | NO | — | — | — |

- PK: `hrd_category_properties_pkey`
- FK: `hrd_category_properties_category_id_fkey`
- UNIQUE: `hrd_category_properties_category_id_code_key`
- CHECK: `hrd_category_properties_data_type_check`, `hrd_category_properties_sort_order_nonnegative_check`, `hrd_category_properties_options_array_check`, `hrd_category_properties_validation_object_check`, `hrd_category_properties_ui_metadata_object_check`, `hrd_category_properties_version_positive_check`
- Index: `hrd_category_properties_category_active_sort_idx`

## 9. HORSE tabloları

### `hrd_horses`

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | Stable internal id |
| tjk_number | varchar(64) | NO | — | UNIQUE | External identity |
| original_name | varchar(200) | NO | — | — | Display |
| name_normalized | varchar(200) | NO | — | — | Prefix; not unique |
| birth_year | smallint | YES | NULL | CHECK | NULL veya 1800–2200 |
| sire_name | varchar(200) | YES | NULL | — | Text |
| dam_name | varchar(200) | YES | NULL | — | Text |
| breed | varchar(120) | YES | NULL | — | — |
| gender | varchar(32) | YES | NULL | — | — |
| coat | varchar(64) | YES | NULL | — | — |
| detail | jsonb | NO | `'{}'` | CHECK object | Controlled sections |
| last_synced_at | timestamptz | YES | NULL | — | — |
| last_seen_at | timestamptz | YES | NULL | — | — |
| source_updated_at | timestamptz | YES | NULL | — | — |
| created_at | timestamptz | NO | — | — | — |
| updated_at | timestamptz | NO | — | — | — |

- PK: `hrd_horses_pkey`
- UNIQUE: `hrd_horses_tjk_number_key`
- CHECK: `hrd_horses_birth_year_check`, `hrd_horses_detail_object_check`
- Index: `hrd_horses_name_normalized_prefix_idx` (varchar_pattern_ops)
- Raw payload kolon yok

## 10. ADVERT ve FAVORITE tabloları

### `hrd_adverts`

**Amaç:** İlan write model. DRAFT/CHANGES_REQUESTED kısmi kayıt destekler; PENDING_REVIEW ve sonrası tam alan gerektirir.

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | Public identifier |
| owner_user_id | uuid | NO | — | FK → `hrd_users.id` ON DELETE RESTRICT | Zorunlu |
| category_id | uuid | YES | NULL | FK → `hrd_categories.id` ON DELETE RESTRICT | Draft 0..1; review+ zorunlu |
| district_id | uuid | YES | NULL | FK → `hrd_districts.id` ON DELETE RESTRICT | Draft 0..1; review+ zorunlu |
| horse_id | uuid | YES | NULL | FK → `hrd_horses.id` ON DELETE RESTRICT | Advert ≤1 horse; aynı horse N advert |
| title | varchar(200) | YES | NULL | — | Draft kısmi; review+ zorunlu non-blank |
| description | text | YES | NULL | — | Draft kısmi; review+ zorunlu non-blank |
| price_amount_minor | bigint | YES | NULL | — | Minor units |
| price_currency | varchar(3) | YES | NULL | — | `^[A-Z]{3}$` |
| status | varchar(32) | NO | `'DRAFT'` | CHECK | Lifecycle |
| properties | jsonb | NO | `'{}'` | CHECK object | Dynamic values |
| published_at | timestamptz | YES | NULL | — | Son PUBLISHED geçişi |
| version | integer | NO | `1` | CHECK > 0 | Content/status concurrency |
| media_version | integer | NO | `1` | CHECK > 0 | Media collection concurrency |
| deleted_at | timestamptz | YES | NULL | — | Yalnız DRAFT |
| created_at | timestamptz | NO | — | — | — |
| updated_at | timestamptz | NO | — | — | — |

- PK: `hrd_adverts_pkey`
- FK: `hrd_adverts_owner_user_id_fkey`, `hrd_adverts_category_id_fkey`, `hrd_adverts_district_id_fkey`, `hrd_adverts_horse_id_fkey`
- CHECK:
  - `hrd_adverts_status_check`
  - `hrd_adverts_price_pair_check`
  - `hrd_adverts_price_amount_minor_check`
  - `hrd_adverts_price_currency_format_check`
  - `hrd_adverts_properties_object_check`
  - `hrd_adverts_published_at_when_published_check`
  - `hrd_adverts_deleted_at_draft_only_check`
  - `hrd_adverts_version_positive_check`
  - `hrd_adverts_media_version_positive_check`
  - `hrd_adverts_reviewed_status_required_fields_check` — status in (`PENDING_REVIEW`,`PUBLISHED`,`REJECTED`,`SUSPENDED`,`SOLD`,`ARCHIVED`) ⇒ `category_id`, `district_id`, `title`, `description` NOT NULL ve `btrim(title) <> ''`, `btrim(description) <> ''`. `DRAFT` ve `CHANGES_REQUESTED` kısmi kalabilir.
- Indexes: `hrd_adverts_owner_user_id_created_idx`, `hrd_adverts_public_newest_idx`, `hrd_adverts_public_category_newest_idx`, `hrd_adverts_public_district_newest_idx`, `hrd_adverts_public_horse_newest_idx`
- Horse required / property required / min media / price required: category metadata + use-case validation; DB CHECK’e gömülmez

### `hrd_advert_status_history`

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | — |
| advert_id | uuid | NO | — | FK → `hrd_adverts.id` ON DELETE RESTRICT | — |
| from_status | varchar(32) | YES | NULL | CHECK | NULL veya advert status set |
| to_status | varchar(32) | NO | — | CHECK | Advert status set |
| actor_user_id | uuid | YES | NULL | FK → `hrd_users.id` ON DELETE RESTRICT | System’de NULL; user transition’da zorunlu |
| is_system | boolean | NO | `false` | — | — |
| reason | text | YES | NULL | — | — |
| created_at | timestamptz | NO | — | — | Immutable |

- PK: `hrd_advert_status_history_pkey`
- FK: `hrd_advert_status_history_advert_id_fkey`, `hrd_advert_status_history_actor_user_id_fkey` (ON DELETE RESTRICT — SET NULL, `actor_system_check` ile çelişirdi; user physical delete yok; tarihsel actor bütünlüğü)
- CHECK: `hrd_advert_status_history_from_status_check`, `hrd_advert_status_history_to_status_check`, `hrd_advert_status_history_from_to_check`, `hrd_advert_status_history_actor_system_check`
- Index: `hrd_advert_status_history_advert_created_idx`
- Allowed transition graph application sorumluluğu

### `hrd_favorites`

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | — |
| user_id | uuid | NO | — | FK → `hrd_users.id` ON DELETE RESTRICT | — |
| advert_id | uuid | NO | — | FK → `hrd_adverts.id` ON DELETE RESTRICT | — |
| created_at | timestamptz | NO | — | — | — |

- PK: `hrd_favorites_pkey`
- FK: `hrd_favorites_user_id_fkey`, `hrd_favorites_advert_id_fkey`
- UNIQUE: `hrd_favorites_user_id_advert_id_key`
- Indexes: `hrd_favorites_user_created_idx`, `hrd_favorites_advert_id_idx`

## 11. MEDIA tabloları

### `hrd_media_assets`

Canonical master metadata alanları (tek anlam): `content_type`, `byte_size`, `checksum_sha256`, `width_px`, `height_px`. Ham upload metadata canonical değildir; geçici bilgiler kontrollü `technical_metadata` içinde tutulabilir ve bu alanların anlamını değiştirmez.

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | — |
| owner_user_id | uuid | NO | — | FK → `hrd_users.id` ON DELETE RESTRICT | — |
| provider | varchar(32) | NO | `'B2'` | — | — |
| raw_object_key | varchar(512) | YES | NULL | — | Quarantine/raw |
| master_object_key | varchar(512) | YES | NULL | — | Canonical master |
| content_type | varchar(128) | YES | NULL | — | Canonical master content type |
| byte_size | bigint | YES | NULL | — | Canonical master size |
| checksum_sha256 | varchar(64) | YES | NULL | — | Canonical master checksum |
| width_px | integer | YES | NULL | — | Canonical master dims |
| height_px | integer | YES | NULL | — | Canonical master dims |
| lifecycle_status | varchar(32) | NO | `'UPLOAD_PENDING'` | CHECK | Asset lifecycle |
| technical_metadata | jsonb | NO | `'{}'` | CHECK object | Non-canonical |
| failure_reason | text | YES | NULL | — | — |
| created_at | timestamptz | NO | — | — | — |
| updated_at | timestamptz | NO | — | — | — |

- PK: `hrd_media_assets_pkey`
- FK: `hrd_media_assets_owner_user_id_fkey`
- CHECK: `hrd_media_assets_lifecycle_status_check`, `hrd_media_assets_technical_metadata_object_check`, `hrd_media_assets_byte_size_check`, `hrd_media_assets_dims_positive_check`, `hrd_media_assets_master_ready_fields_check` (MASTER_READY ⇒ master_object_key, content_type, byte_size, width_px, height_px NOT NULL; byte_size >= 0; dims > 0), `hrd_media_assets_validation_failed_reason_check`, `hrd_media_assets_uploaded_raw_key_check` (UPLOADED/VALIDATING ⇒ raw_object_key NOT NULL), `hrd_media_assets_raw_object_key_not_blank_check`, `hrd_media_assets_master_object_key_not_blank_check`
- PARTIAL UNIQUE: `hrd_media_assets_provider_raw_object_key_key`, `hrd_media_assets_provider_master_object_key_key`
- Indexes: `hrd_media_assets_owner_created_idx`, `hrd_media_assets_cleanup_idx`
- UPLOAD_PENDING raw key tahsisi upload implementation kararı

### `hrd_media_variants`

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | — |
| asset_id | uuid | NO | — | FK → `hrd_media_assets.id` ON DELETE RESTRICT | — |
| transform_profile | varchar(64) | NO | — | CHECK not blank | Immutable versioned profile |
| object_key | varchar(512) | YES | NULL | — | — |
| lifecycle_status | varchar(32) | NO | `'PENDING'` | CHECK | Variant lifecycle |
| width_px | integer | YES | NULL | — | — |
| height_px | integer | YES | NULL | — | — |
| byte_size | bigint | YES | NULL | — | — |
| content_type | varchar(128) | YES | NULL | — | — |
| failure_reason | text | YES | NULL | — | — |
| technical_metadata | jsonb | NO | `'{}'` | CHECK object | — |
| created_at | timestamptz | NO | — | — | — |
| updated_at | timestamptz | NO | — | — | — |

- PK: `hrd_media_variants_pkey`
- FK: `hrd_media_variants_asset_id_fkey`
- UNIQUE: `hrd_media_variants_asset_id_transform_profile_key`
- PARTIAL UNIQUE: `hrd_media_variants_object_key_key`
- CHECK: `hrd_media_variants_lifecycle_status_check`, `hrd_media_variants_technical_metadata_object_check`, `hrd_media_variants_ready_fields_check`, `hrd_media_variants_failed_reason_check`, `hrd_media_variants_dims_positive_check`, `hrd_media_variants_byte_size_check`, `hrd_media_variants_object_key_not_blank_check`, `hrd_media_variants_transform_profile_not_blank_check`
- Index: `hrd_media_variants_asset_status_idx`

### `hrd_advert_media`

Collection concurrency: `hrd_adverts.media_version`. Satır `version` yok.

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | — |
| advert_id | uuid | NO | — | FK → `hrd_adverts.id` ON DELETE RESTRICT | — |
| asset_id | uuid | NO | — | FK → `hrd_media_assets.id` ON DELETE RESTRICT | — |
| display_order | integer | NO | — | CHECK >= 0 | — |
| is_cover | boolean | NO | `false` | — | — |
| created_at | timestamptz | NO | — | — | — |
| updated_at | timestamptz | NO | — | — | — |

- PK: `hrd_advert_media_pkey`
- FK: `hrd_advert_media_advert_id_fkey`, `hrd_advert_media_asset_id_fkey`
- UNIQUE: `hrd_advert_media_advert_id_asset_id_key`, `hrd_advert_media_advert_id_display_order_key`
- PARTIAL UNIQUE: `hrd_advert_media_one_cover_key`
- CHECK: `hrd_advert_media_display_order_nonnegative_check`
- Indexes: `hrd_advert_media_asset_id_idx` (asset reverse reference / cleanup), unique order index ordering sorgularını da destekler (ayrı redundant order index yok)

## 12. BANNER tabloları

### `hrd_banners`

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | — |
| placement | varchar(32) | NO | — | CHECK | HOMEPAGE/LISTING_DETAIL/SEARCH |
| status | varchar(16) | NO | `'INACTIVE'` | CHECK | ACTIVE/INACTIVE |
| asset_id | uuid | NO | — | FK → `hrd_media_assets.id` ON DELETE RESTRICT | Shared asset |
| title | varchar(160) | YES | NULL | — | — |
| alt_text | varchar(255) | YES | NULL | — | — |
| target_url | text | YES | NULL | — | — |
| sort_order | integer | NO | `0` | CHECK >= 0 | — |
| version | integer | NO | `1` | CHECK > 0 | — |
| created_by_user_id | uuid | YES | NULL | FK → `hrd_users.id` ON DELETE SET NULL | — |
| created_at | timestamptz | NO | — | — | — |
| updated_at | timestamptz | NO | — | — | — |

- PK: `hrd_banners_pkey`
- FK: `hrd_banners_asset_id_fkey`, `hrd_banners_created_by_user_id_fkey`
- CHECK: `hrd_banners_placement_check`, `hrd_banners_status_check`, `hrd_banners_sort_order_nonnegative_check`, `hrd_banners_version_positive_check`
- Indexes: `hrd_banners_active_placement_sort_idx`, `hrd_banners_asset_id_idx` (asset reverse reference; INACTIVE dahil)

## 13. WORKER ve TJK operasyon tabloları

### `hrd_background_jobs`

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | — |
| job_type | varchar(64) | NO | — | CHECK | Exact job type set |
| status | varchar(16) | NO | `'QUEUED'` | CHECK | Job status set |
| payload | jsonb | NO | `'{}'` | CHECK object | Küçük kontrollü |
| tjk_sync_run_id | uuid | YES | NULL | FK → `hrd_tjk_sync_runs.id` ON DELETE RESTRICT | Yalnız TJK_SYNC_BATCH |
| deduplication_key | varchar(255) | YES | NULL | — | Logical uniqueness |
| attempt_count | integer | NO | `0` | — | — |
| max_attempts | integer | NO | — | CHECK > 0 | Explicit at create; no DB default |
| available_at | timestamptz | NO | — | — | — |
| leased_until | timestamptz | YES | NULL | — | — |
| lease_owner | varchar(128) | YES | NULL | — | — |
| last_error | text | YES | NULL | — | Secret/kontrolsüz payload yok; retry’da korunabilir |
| cancel_requested_at | timestamptz | YES | NULL | — | Request ≠ CANCELLED |
| version | integer | NO | `1` | CHECK > 0 | Claim concurrency |
| created_at | timestamptz | NO | — | — | — |
| updated_at | timestamptz | NO | — | — | — |
| completed_at | timestamptz | YES | NULL | — | — |

- PK: `hrd_background_jobs_pkey`
- FK: `hrd_background_jobs_tjk_sync_run_id_fkey`
- CHECK: `hrd_background_jobs_job_type_check`, `hrd_background_jobs_status_check`, `hrd_background_jobs_payload_object_check`, `hrd_background_jobs_attempt_bounds_check`, `hrd_background_jobs_max_attempts_positive_check`, `hrd_background_jobs_tjk_run_by_job_type_check` (TJK_SYNC_BATCH ⇒ tjk_sync_run_id NOT NULL; diğer type’lar ⇒ NULL), `hrd_background_jobs_lease_fields_check` (LEASED ⇒ owner+until NOT NULL; status <> LEASED ⇒ owner+until NULL), `hrd_background_jobs_completed_at_terminal_check`, `hrd_background_jobs_cancelled_requires_request_check`, `hrd_background_jobs_version_positive_check`
- PARTIAL UNIQUE: `hrd_background_jobs_deduplication_key_key`
- Indexes: `hrd_background_jobs_claim_idx`, `hrd_background_jobs_lease_recovery_idx`, `hrd_background_jobs_tjk_sync_run_id_idx`
- Lease recovery: stale LEASED → QUEUED TX’de lease alanları temizlenir
- `FAILED` ≠ `DEAD`; transient + retry hakkı ⇒ tekrar `QUEUED` (terminal FAILED değil). `last_error` secret/kontrolsüz payload içermez. Transition graph app/worker TX sorumluluğu (CHECK seti değişmez; semantik §5)

### `hrd_tjk_sync_runs`

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | — |
| mode | varchar(32) | NO | — | CHECK | FULL/INCREMENTAL/RECONCILIATION |
| status | varchar(32) | NO | `'QUEUED'` | CHECK | Run status set |
| source_adapter | varchar(64) | NO | — | — | — |
| scope_key | varchar(64) | NO | `'HORSES'` | CHECK | Exact `HORSES` |
| checkpoint | jsonb | NO | `'{}'` | CHECK object | — |
| source_snapshot | varchar(255) | YES | NULL | — | — |
| trigger_kind | varchar(32) | NO | — | CHECK | SCHEDULED/MANUAL |
| created_by_user_id | uuid | YES | NULL | FK → `hrd_users.id` ON DELETE RESTRICT | MANUAL zorunlu; SCHEDULED NULL |
| cancel_requested_at | timestamptz | YES | NULL | — | — |
| cancelled_at | timestamptz | YES | NULL | — | — |
| started_at | timestamptz | YES | NULL | — | — |
| completed_at | timestamptz | YES | NULL | — | — |
| total_count | integer | NO | `0` | CHECK >= 0 | — |
| created_count | integer | NO | `0` | CHECK >= 0 | — |
| updated_count | integer | NO | `0` | CHECK >= 0 | — |
| unchanged_count | integer | NO | `0` | CHECK >= 0 | — |
| skipped_count | integer | NO | `0` | CHECK >= 0 | — |
| failed_count | integer | NO | `0` | CHECK >= 0 | — |
| conflict_count | integer | NO | `0` | CHECK >= 0 | — |
| last_error_summary | text | YES | NULL | — | — |
| version | integer | NO | `1` | CHECK > 0 | Checkpoint/status concurrency |
| created_at | timestamptz | NO | — | — | — |
| updated_at | timestamptz | NO | — | — | — |

- PK: `hrd_tjk_sync_runs_pkey`
- FK: `hrd_tjk_sync_runs_created_by_user_id_fkey` (ON DELETE RESTRICT — SET NULL, `trigger_actor_check` ile çelişirdi; user physical delete yok)
- CHECK: `hrd_tjk_sync_runs_mode_check`, `hrd_tjk_sync_runs_status_check`, `hrd_tjk_sync_runs_trigger_kind_check`, `hrd_tjk_sync_runs_scope_key_check`, `hrd_tjk_sync_runs_checkpoint_object_check`, `hrd_tjk_sync_runs_running_started_check`, `hrd_tjk_sync_runs_terminal_completed_check`, `hrd_tjk_sync_runs_cancelled_fields_check`, `hrd_tjk_sync_runs_trigger_actor_check` (MANUAL ⇒ created_by NOT NULL; SCHEDULED ⇒ NULL), `hrd_tjk_sync_runs_time_order_check`, `hrd_tjk_sync_runs_version_positive_check`, counter >= 0 checks
- PARTIAL UNIQUE: `hrd_tjk_sync_runs_one_active_per_source_scope_key` (`source_adapter`, `scope_key`) WHERE status IN (`QUEUED`,`RUNNING`)
- Indexes: `hrd_tjk_sync_runs_created_idx`, `hrd_tjk_sync_runs_status_idx`, `hrd_tjk_sync_runs_source_scope_created_idx`
- Admin role FK ile doğrulanamaz; MANUAL trigger app + BO context

### `hrd_tjk_sync_item_errors`

| Column | PostgreSQL type | Nullable | Default | Constraint / relation | Açıklama |
| --- | --- | --- | --- | --- | --- |
| id | uuid | NO | — | PK | — |
| run_id | uuid | NO | — | FK → `hrd_tjk_sync_runs.id` ON DELETE RESTRICT | — |
| tjk_number | varchar(64) | YES | NULL | — | — |
| horse_id | uuid | YES | NULL | FK → `hrd_horses.id` ON DELETE SET NULL | — |
| batch_context | jsonb | NO | `'{}'` | CHECK object | — |
| error_class | varchar(16) | NO | — | CHECK | — |
| status | varchar(16) | NO | `'OPEN'` | CHECK | — |
| message | text | NO | — | CHECK not blank | — |
| detail | jsonb | NO | `'{}'` | CHECK object | — |
| created_at | timestamptz | NO | — | — | — |
| resolved_at | timestamptz | YES | NULL | — | — |

- PK: `hrd_tjk_sync_item_errors_pkey`
- FK: `hrd_tjk_sync_item_errors_run_id_fkey`, `hrd_tjk_sync_item_errors_horse_id_fkey`
- CHECK: `hrd_tjk_sync_item_errors_error_class_check`, `hrd_tjk_sync_item_errors_status_check`, `hrd_tjk_sync_item_errors_resolution_check`, `hrd_tjk_sync_item_errors_message_not_blank_check`, `hrd_tjk_sync_item_errors_batch_context_object_check`, `hrd_tjk_sync_item_errors_detail_object_check`
- Indexes: `hrd_tjk_sync_item_errors_run_status_idx`, `hrd_tjk_sync_item_errors_tjk_number_idx`

## 14. Relationship ve cardinality özeti

Cardinality, FK’nin bulunduğu (`From`) tarafın bakış açısıyla yazılır.

| From | Relation | To | Cardinality | FK | Delete behavior |
| --- | --- | --- | --- | --- | --- |
| `hrd_users` | has | `hrd_auth_sessions` | 1:N | `hrd_auth_sessions_user_id_fkey` | RESTRICT |
| `hrd_auth_sessions` | replaced by | `hrd_auth_sessions` | N:0..1 | `hrd_auth_sessions_replaced_by_session_id_fkey` | SET NULL |
| `hrd_users` | has | `hrd_one_time_credentials` | 1:N | `hrd_one_time_credentials_user_id_fkey` | RESTRICT |
| `hrd_security_events` | subject | `hrd_users` | N:0..1 | `hrd_security_events_subject_user_id_fkey` | SET NULL |
| `hrd_security_events` | actor | `hrd_users` | N:0..1 | `hrd_security_events_actor_user_id_fkey` | SET NULL |
| `hrd_provinces` | has | `hrd_districts` | 1:N | `hrd_districts_province_id_fkey` | RESTRICT |
| `hrd_categories` | parent of | `hrd_categories` | 1:N self | `hrd_categories_parent_id_fkey` | RESTRICT |
| `hrd_categories` | has | `hrd_category_properties` | 1:N | `hrd_category_properties_category_id_fkey` | RESTRICT |
| `hrd_adverts` | owned by | `hrd_users` | N:1, owner zorunlu | `hrd_adverts_owner_user_id_fkey` | RESTRICT |
| `hrd_adverts` | classified by | `hrd_categories` | N:0..1 DRAFT/CHANGES_REQUESTED; N:1 review+ | `hrd_adverts_category_id_fkey` | RESTRICT |
| `hrd_adverts` | located in | `hrd_districts` | N:0..1 DRAFT/CHANGES_REQUESTED; N:1 review+ | `hrd_adverts_district_id_fkey` | RESTRICT |
| `hrd_adverts` | references | `hrd_horses` | N:0..1 | `hrd_adverts_horse_id_fkey` | RESTRICT |
| `hrd_adverts` | has | `hrd_advert_status_history` | 1:N | `hrd_advert_status_history_advert_id_fkey` | RESTRICT |
| `hrd_advert_status_history` | actor | `hrd_users` | N:0..1 | `hrd_advert_status_history_actor_user_id_fkey` | RESTRICT |
| `hrd_users` ↔ `hrd_adverts` | favorites | via `hrd_favorites` | N:N | `hrd_favorites_user_id_fkey`, `hrd_favorites_advert_id_fkey` | both RESTRICT |
| `hrd_users` | uploads | `hrd_media_assets` | 1:N | `hrd_media_assets_owner_user_id_fkey` | RESTRICT |
| `hrd_media_assets` | has | `hrd_media_variants` | 1:N | `hrd_media_variants_asset_id_fkey` | RESTRICT |
| `hrd_adverts` ↔ `hrd_media_assets` | media | via `hrd_advert_media` | N:N | `hrd_advert_media_advert_id_fkey`, `hrd_advert_media_asset_id_fkey` | both RESTRICT; detach ≠ asset delete |
| `hrd_banners` | uses | `hrd_media_assets` | N:1 | `hrd_banners_asset_id_fkey` | RESTRICT |
| `hrd_banners` | created by | `hrd_users` | N:0..1 | `hrd_banners_created_by_user_id_fkey` | SET NULL |
| `hrd_background_jobs` | belongs to | `hrd_tjk_sync_runs` | N:0..1 (yalnız TJK_SYNC_BATCH) | `hrd_background_jobs_tjk_sync_run_id_fkey` | RESTRICT |
| `hrd_tjk_sync_runs` | created by | `hrd_users` | N:0..1 (MANUAL zorunlu) | `hrd_tjk_sync_runs_created_by_user_id_fkey` | RESTRICT |
| `hrd_tjk_sync_runs` | has | `hrd_tjk_sync_item_errors` | 1:N | `hrd_tjk_sync_item_errors_run_id_fkey` | RESTRICT |
| `hrd_tjk_sync_item_errors` | horse | `hrd_horses` | N:0..1 | `hrd_tjk_sync_item_errors_horse_id_fkey` | SET NULL |

Açıklamalar:

- Bir user birden fazla advert sahibi olabilir.
- Bir category birden fazla advert sınıflandırabilir.
- Bir district birden fazla advert tarafından kullanılabilir.
- Bir horse birden fazla advert tarafından referans alınabilir; nullable `horse_id` advert’in en fazla bir horse seçtiğini ifade eder, horse’un tek advert’te kullanıldığını değil.
- Status history actor: `is_system=false` iken zorunlu; FK RESTRICT tarihsel actor bütünlüğünü korur (SET NULL CHECK ile çelişirdi).
- Manual TJK creator: `trigger_kind=MANUAL` iken zorunlu; FK RESTRICT (SET NULL CHECK ile çelişirdi).
- Security event subject/actor, banner created-by, session replaced-by, TJK item-error horse SET NULL kalır.

## 15. Constraint planı

Her satır exact tek constraint’tir. Wildcard/slash/yaklaşık ad yoktur.

### Primary keys

| Exact name | Kind | Table | Rule |
| --- | --- | --- | --- |
| `hrd_users_pkey` | PK | `hrd_users` | `id` |
| `hrd_auth_sessions_pkey` | PK | `hrd_auth_sessions` | `id` |
| `hrd_one_time_credentials_pkey` | PK | `hrd_one_time_credentials` | `id` |
| `hrd_security_events_pkey` | PK | `hrd_security_events` | `id` |
| `hrd_provinces_pkey` | PK | `hrd_provinces` | `id` |
| `hrd_districts_pkey` | PK | `hrd_districts` | `id` |
| `hrd_categories_pkey` | PK | `hrd_categories` | `id` |
| `hrd_category_properties_pkey` | PK | `hrd_category_properties` | `id` |
| `hrd_horses_pkey` | PK | `hrd_horses` | `id` |
| `hrd_adverts_pkey` | PK | `hrd_adverts` | `id` |
| `hrd_advert_status_history_pkey` | PK | `hrd_advert_status_history` | `id` |
| `hrd_favorites_pkey` | PK | `hrd_favorites` | `id` |
| `hrd_media_assets_pkey` | PK | `hrd_media_assets` | `id` |
| `hrd_media_variants_pkey` | PK | `hrd_media_variants` | `id` |
| `hrd_advert_media_pkey` | PK | `hrd_advert_media` | `id` |
| `hrd_banners_pkey` | PK | `hrd_banners` | `id` |
| `hrd_background_jobs_pkey` | PK | `hrd_background_jobs` | `id` |
| `hrd_tjk_sync_runs_pkey` | PK | `hrd_tjk_sync_runs` | `id` |
| `hrd_tjk_sync_item_errors_pkey` | PK | `hrd_tjk_sync_item_errors` | `id` |

### Foreign keys

| Exact name | Kind | Table | Rule |
| --- | --- | --- | --- |
| `hrd_auth_sessions_user_id_fkey` | FK | `hrd_auth_sessions` | `user_id → hrd_users.id`, ON DELETE RESTRICT |
| `hrd_auth_sessions_replaced_by_session_id_fkey` | FK | `hrd_auth_sessions` | `replaced_by_session_id → hrd_auth_sessions.id`, ON DELETE SET NULL |
| `hrd_one_time_credentials_user_id_fkey` | FK | `hrd_one_time_credentials` | `user_id → hrd_users.id`, ON DELETE RESTRICT |
| `hrd_security_events_subject_user_id_fkey` | FK | `hrd_security_events` | `subject_user_id → hrd_users.id`, ON DELETE SET NULL |
| `hrd_security_events_actor_user_id_fkey` | FK | `hrd_security_events` | `actor_user_id → hrd_users.id`, ON DELETE SET NULL |
| `hrd_districts_province_id_fkey` | FK | `hrd_districts` | `province_id → hrd_provinces.id`, ON DELETE RESTRICT |
| `hrd_categories_parent_id_fkey` | FK | `hrd_categories` | `parent_id → hrd_categories.id`, ON DELETE RESTRICT |
| `hrd_category_properties_category_id_fkey` | FK | `hrd_category_properties` | `category_id → hrd_categories.id`, ON DELETE RESTRICT |
| `hrd_adverts_owner_user_id_fkey` | FK | `hrd_adverts` | `owner_user_id → hrd_users.id`, ON DELETE RESTRICT |
| `hrd_adverts_category_id_fkey` | FK | `hrd_adverts` | `category_id → hrd_categories.id`, ON DELETE RESTRICT |
| `hrd_adverts_district_id_fkey` | FK | `hrd_adverts` | `district_id → hrd_districts.id`, ON DELETE RESTRICT |
| `hrd_adverts_horse_id_fkey` | FK | `hrd_adverts` | `horse_id → hrd_horses.id`, ON DELETE RESTRICT |
| `hrd_advert_status_history_advert_id_fkey` | FK | `hrd_advert_status_history` | `advert_id → hrd_adverts.id`, ON DELETE RESTRICT |
| `hrd_advert_status_history_actor_user_id_fkey` | FK | `hrd_advert_status_history` | `actor_user_id → hrd_users.id`, ON DELETE RESTRICT |
| `hrd_favorites_user_id_fkey` | FK | `hrd_favorites` | `user_id → hrd_users.id`, ON DELETE RESTRICT |
| `hrd_favorites_advert_id_fkey` | FK | `hrd_favorites` | `advert_id → hrd_adverts.id`, ON DELETE RESTRICT |
| `hrd_media_assets_owner_user_id_fkey` | FK | `hrd_media_assets` | `owner_user_id → hrd_users.id`, ON DELETE RESTRICT |
| `hrd_media_variants_asset_id_fkey` | FK | `hrd_media_variants` | `asset_id → hrd_media_assets.id`, ON DELETE RESTRICT |
| `hrd_advert_media_advert_id_fkey` | FK | `hrd_advert_media` | `advert_id → hrd_adverts.id`, ON DELETE RESTRICT |
| `hrd_advert_media_asset_id_fkey` | FK | `hrd_advert_media` | `asset_id → hrd_media_assets.id`, ON DELETE RESTRICT |
| `hrd_banners_asset_id_fkey` | FK | `hrd_banners` | `asset_id → hrd_media_assets.id`, ON DELETE RESTRICT |
| `hrd_banners_created_by_user_id_fkey` | FK | `hrd_banners` | `created_by_user_id → hrd_users.id`, ON DELETE SET NULL |
| `hrd_background_jobs_tjk_sync_run_id_fkey` | FK | `hrd_background_jobs` | `tjk_sync_run_id → hrd_tjk_sync_runs.id`, ON DELETE RESTRICT |
| `hrd_tjk_sync_runs_created_by_user_id_fkey` | FK | `hrd_tjk_sync_runs` | `created_by_user_id → hrd_users.id`, ON DELETE RESTRICT |
| `hrd_tjk_sync_item_errors_run_id_fkey` | FK | `hrd_tjk_sync_item_errors` | `run_id → hrd_tjk_sync_runs.id`, ON DELETE RESTRICT |
| `hrd_tjk_sync_item_errors_horse_id_fkey` | FK | `hrd_tjk_sync_item_errors` | `horse_id → hrd_horses.id`, ON DELETE SET NULL |

### Unique / partial unique

| Exact name | Kind | Table | Rule |
| --- | --- | --- | --- |
| `hrd_users_email_normalized_key` | UNIQUE | `hrd_users` | `email_normalized` |
| `hrd_auth_sessions_refresh_token_hash_key` | UNIQUE | `hrd_auth_sessions` | `refresh_token_hash` |
| `hrd_one_time_credentials_token_hash_key` | UNIQUE | `hrd_one_time_credentials` | `token_hash` |
| `hrd_one_time_credentials_one_active_per_user_purpose_key` | PARTIAL UNIQUE | `hrd_one_time_credentials` | (`user_id`,`purpose`) WHERE consumed/invalidated NULL |
| `hrd_provinces_name_key` | UNIQUE | `hrd_provinces` | `name` |
| `hrd_provinces_name_normalized_key` | UNIQUE | `hrd_provinces` | `name_normalized` |
| `hrd_districts_province_id_name_key` | UNIQUE | `hrd_districts` | (`province_id`,`name`) |
| `hrd_districts_province_id_name_normalized_key` | UNIQUE | `hrd_districts` | (`province_id`,`name_normalized`) |
| `hrd_categories_slug_key` | UNIQUE | `hrd_categories` | `slug` |
| `hrd_category_properties_category_id_code_key` | UNIQUE | `hrd_category_properties` | (`category_id`,`code`) |
| `hrd_horses_tjk_number_key` | UNIQUE | `hrd_horses` | `tjk_number` |
| `hrd_favorites_user_id_advert_id_key` | UNIQUE | `hrd_favorites` | (`user_id`,`advert_id`) |
| `hrd_media_assets_provider_raw_object_key_key` | PARTIAL UNIQUE | `hrd_media_assets` | (`provider`,`raw_object_key`) WHERE raw NOT NULL |
| `hrd_media_assets_provider_master_object_key_key` | PARTIAL UNIQUE | `hrd_media_assets` | (`provider`,`master_object_key`) WHERE master NOT NULL |
| `hrd_media_variants_asset_id_transform_profile_key` | UNIQUE | `hrd_media_variants` | (`asset_id`,`transform_profile`) |
| `hrd_media_variants_object_key_key` | PARTIAL UNIQUE | `hrd_media_variants` | (`object_key`) WHERE NOT NULL |
| `hrd_advert_media_advert_id_asset_id_key` | UNIQUE | `hrd_advert_media` | (`advert_id`,`asset_id`) |
| `hrd_advert_media_advert_id_display_order_key` | UNIQUE | `hrd_advert_media` | (`advert_id`,`display_order`) |
| `hrd_advert_media_one_cover_key` | PARTIAL UNIQUE | `hrd_advert_media` | (`advert_id`) WHERE `is_cover = true` |
| `hrd_background_jobs_deduplication_key_key` | PARTIAL UNIQUE | `hrd_background_jobs` | (`deduplication_key`) WHERE NOT NULL |
| `hrd_tjk_sync_runs_one_active_per_source_scope_key` | PARTIAL UNIQUE | `hrd_tjk_sync_runs` | (`source_adapter`,`scope_key`) WHERE status IN (`QUEUED`,`RUNNING`) |

### Check constraints

| Exact name | Kind | Table | Rule |
| --- | --- | --- | --- |
| `hrd_users_role_check` | CHECK | `hrd_users` | role in (`user`,`admin`) |
| `hrd_users_status_check` | CHECK | `hrd_users` | status in (`ACTIVE`,`DISABLED`,`CLOSED`) |
| `hrd_users_failed_login_count_check` | CHECK | `hrd_users` | `failed_login_count >= 0` |
| `hrd_auth_sessions_client_context_check` | CHECK | `hrd_auth_sessions` | client_context set |
| `hrd_auth_sessions_idle_le_absolute_check` | CHECK | `hrd_auth_sessions` | idle ≤ absolute |
| `hrd_auth_sessions_created_le_last_used_check` | CHECK | `hrd_auth_sessions` | created ≤ last_used |
| `hrd_auth_sessions_revoke_reason_requires_revoked_at_check` | CHECK | `hrd_auth_sessions` | reason ⇒ revoked_at |
| `hrd_auth_sessions_no_self_replace_check` | CHECK | `hrd_auth_sessions` | replaced_by ≠ id |
| `hrd_one_time_credentials_purpose_check` | CHECK | `hrd_one_time_credentials` | 3 purposes |
| `hrd_one_time_credentials_target_email_by_purpose_check` | CHECK | `hrd_one_time_credentials` | verify/change target required; reset null |
| `hrd_one_time_credentials_consumed_xor_invalidated_check` | CHECK | `hrd_one_time_credentials` | not both set |
| `hrd_one_time_credentials_expires_after_created_check` | CHECK | `hrd_one_time_credentials` | expires > created |
| `hrd_security_events_event_type_check` | CHECK | `hrd_security_events` | event type set |
| `hrd_security_events_client_context_check` | CHECK | `hrd_security_events` | NULL or PUBLIC_WEB/MOBILE/ADMIN_BO |
| `hrd_security_events_metadata_object_check` | CHECK | `hrd_security_events` | jsonb object |
| `hrd_provinces_sort_order_nonnegative_check` | CHECK | `hrd_provinces` | sort_order >= 0 |
| `hrd_districts_sort_order_nonnegative_check` | CHECK | `hrd_districts` | sort_order >= 0 |
| `hrd_categories_no_self_parent_check` | CHECK | `hrd_categories` | parent NULL or <> id |
| `hrd_categories_sort_order_nonnegative_check` | CHECK | `hrd_categories` | sort_order >= 0 |
| `hrd_categories_version_positive_check` | CHECK | `hrd_categories` | version > 0 |
| `hrd_category_properties_data_type_check` | CHECK | `hrd_category_properties` | data type set |
| `hrd_category_properties_sort_order_nonnegative_check` | CHECK | `hrd_category_properties` | sort_order >= 0 |
| `hrd_category_properties_options_array_check` | CHECK | `hrd_category_properties` | jsonb array |
| `hrd_category_properties_validation_object_check` | CHECK | `hrd_category_properties` | jsonb object |
| `hrd_category_properties_ui_metadata_object_check` | CHECK | `hrd_category_properties` | jsonb object |
| `hrd_category_properties_version_positive_check` | CHECK | `hrd_category_properties` | version > 0 |
| `hrd_horses_birth_year_check` | CHECK | `hrd_horses` | NULL or 1800–2200 |
| `hrd_horses_detail_object_check` | CHECK | `hrd_horses` | jsonb object |
| `hrd_adverts_status_check` | CHECK | `hrd_adverts` | advert status set |
| `hrd_adverts_price_pair_check` | CHECK | `hrd_adverts` | both null or both set |
| `hrd_adverts_price_amount_minor_check` | CHECK | `hrd_adverts` | NULL or >= 0 |
| `hrd_adverts_price_currency_format_check` | CHECK | `hrd_adverts` | NULL or `^[A-Z]{3}$` |
| `hrd_adverts_properties_object_check` | CHECK | `hrd_adverts` | jsonb object |
| `hrd_adverts_published_at_when_published_check` | CHECK | `hrd_adverts` | PUBLISHED ⇒ published_at |
| `hrd_adverts_deleted_at_draft_only_check` | CHECK | `hrd_adverts` | deleted only DRAFT |
| `hrd_adverts_version_positive_check` | CHECK | `hrd_adverts` | version > 0 |
| `hrd_adverts_media_version_positive_check` | CHECK | `hrd_adverts` | media_version > 0 |
| `hrd_adverts_reviewed_status_required_fields_check` | CHECK | `hrd_adverts` | review+ statuses require category/district/title/description non-blank |
| `hrd_advert_status_history_from_status_check` | CHECK | `hrd_advert_status_history` | NULL or advert status set |
| `hrd_advert_status_history_to_status_check` | CHECK | `hrd_advert_status_history` | advert status set |
| `hrd_advert_status_history_from_to_check` | CHECK | `hrd_advert_status_history` | creation/transition structure |
| `hrd_advert_status_history_actor_system_check` | CHECK | `hrd_advert_status_history` | system/actor rules |
| `hrd_media_assets_lifecycle_status_check` | CHECK | `hrd_media_assets` | asset lifecycle set |
| `hrd_media_assets_technical_metadata_object_check` | CHECK | `hrd_media_assets` | jsonb object |
| `hrd_media_assets_byte_size_check` | CHECK | `hrd_media_assets` | NULL or >= 0 |
| `hrd_media_assets_dims_positive_check` | CHECK | `hrd_media_assets` | NULL or > 0 |
| `hrd_media_assets_master_ready_fields_check` | CHECK | `hrd_media_assets` | MASTER_READY fields |
| `hrd_media_assets_validation_failed_reason_check` | CHECK | `hrd_media_assets` | FAILED ⇒ reason |
| `hrd_media_assets_uploaded_raw_key_check` | CHECK | `hrd_media_assets` | UPLOADED/VALIDATING ⇒ raw key |
| `hrd_media_assets_raw_object_key_not_blank_check` | CHECK | `hrd_media_assets` | NULL or non-blank |
| `hrd_media_assets_master_object_key_not_blank_check` | CHECK | `hrd_media_assets` | NULL or non-blank |
| `hrd_media_variants_lifecycle_status_check` | CHECK | `hrd_media_variants` | variant lifecycle set |
| `hrd_media_variants_technical_metadata_object_check` | CHECK | `hrd_media_variants` | jsonb object |
| `hrd_media_variants_ready_fields_check` | CHECK | `hrd_media_variants` | READY fields |
| `hrd_media_variants_failed_reason_check` | CHECK | `hrd_media_variants` | FAILED ⇒ reason |
| `hrd_media_variants_dims_positive_check` | CHECK | `hrd_media_variants` | NULL or > 0 |
| `hrd_media_variants_byte_size_check` | CHECK | `hrd_media_variants` | NULL or >= 0 |
| `hrd_media_variants_object_key_not_blank_check` | CHECK | `hrd_media_variants` | NULL or non-blank |
| `hrd_media_variants_transform_profile_not_blank_check` | CHECK | `hrd_media_variants` | non-blank |
| `hrd_advert_media_display_order_nonnegative_check` | CHECK | `hrd_advert_media` | display_order >= 0 |
| `hrd_banners_placement_check` | CHECK | `hrd_banners` | placement set |
| `hrd_banners_status_check` | CHECK | `hrd_banners` | status set |
| `hrd_banners_sort_order_nonnegative_check` | CHECK | `hrd_banners` | sort_order >= 0 |
| `hrd_banners_version_positive_check` | CHECK | `hrd_banners` | version > 0 |
| `hrd_background_jobs_job_type_check` | CHECK | `hrd_background_jobs` | job type set |
| `hrd_background_jobs_status_check` | CHECK | `hrd_background_jobs` | job status set |
| `hrd_background_jobs_payload_object_check` | CHECK | `hrd_background_jobs` | jsonb object |
| `hrd_background_jobs_attempt_bounds_check` | CHECK | `hrd_background_jobs` | 0 ≤ attempt ≤ max |
| `hrd_background_jobs_max_attempts_positive_check` | CHECK | `hrd_background_jobs` | max_attempts > 0 |
| `hrd_background_jobs_tjk_run_by_job_type_check` | CHECK | `hrd_background_jobs` | TJK type requires run; media types forbid run |
| `hrd_background_jobs_lease_fields_check` | CHECK | `hrd_background_jobs` | LEASED vs non-LEASED lease fields |
| `hrd_background_jobs_completed_at_terminal_check` | CHECK | `hrd_background_jobs` | terminal completed_at |
| `hrd_background_jobs_cancelled_requires_request_check` | CHECK | `hrd_background_jobs` | CANCELLED ⇒ cancel_requested_at |
| `hrd_background_jobs_version_positive_check` | CHECK | `hrd_background_jobs` | version > 0 |
| `hrd_tjk_sync_runs_mode_check` | CHECK | `hrd_tjk_sync_runs` | mode set |
| `hrd_tjk_sync_runs_status_check` | CHECK | `hrd_tjk_sync_runs` | run status set |
| `hrd_tjk_sync_runs_trigger_kind_check` | CHECK | `hrd_tjk_sync_runs` | SCHEDULED/MANUAL |
| `hrd_tjk_sync_runs_scope_key_check` | CHECK | `hrd_tjk_sync_runs` | `scope_key = 'HORSES'` |
| `hrd_tjk_sync_runs_checkpoint_object_check` | CHECK | `hrd_tjk_sync_runs` | jsonb object |
| `hrd_tjk_sync_runs_running_started_check` | CHECK | `hrd_tjk_sync_runs` | RUNNING ⇒ started_at |
| `hrd_tjk_sync_runs_terminal_completed_check` | CHECK | `hrd_tjk_sync_runs` | terminal ⇒ completed_at |
| `hrd_tjk_sync_runs_cancelled_fields_check` | CHECK | `hrd_tjk_sync_runs` | CANCELLED fields |
| `hrd_tjk_sync_runs_trigger_actor_check` | CHECK | `hrd_tjk_sync_runs` | MANUAL/SCHEDULED actor |
| `hrd_tjk_sync_runs_time_order_check` | CHECK | `hrd_tjk_sync_runs` | started/completed/cancelled order |
| `hrd_tjk_sync_runs_version_positive_check` | CHECK | `hrd_tjk_sync_runs` | version > 0 |
| `hrd_tjk_sync_runs_total_count_nonnegative_check` | CHECK | `hrd_tjk_sync_runs` | total_count >= 0 |
| `hrd_tjk_sync_runs_created_count_nonnegative_check` | CHECK | `hrd_tjk_sync_runs` | created_count >= 0 |
| `hrd_tjk_sync_runs_updated_count_nonnegative_check` | CHECK | `hrd_tjk_sync_runs` | updated_count >= 0 |
| `hrd_tjk_sync_runs_unchanged_count_nonnegative_check` | CHECK | `hrd_tjk_sync_runs` | unchanged_count >= 0 |
| `hrd_tjk_sync_runs_skipped_count_nonnegative_check` | CHECK | `hrd_tjk_sync_runs` | skipped_count >= 0 |
| `hrd_tjk_sync_runs_failed_count_nonnegative_check` | CHECK | `hrd_tjk_sync_runs` | failed_count >= 0 |
| `hrd_tjk_sync_runs_conflict_count_nonnegative_check` | CHECK | `hrd_tjk_sync_runs` | conflict_count >= 0 |
| `hrd_tjk_sync_item_errors_error_class_check` | CHECK | `hrd_tjk_sync_item_errors` | error class set |
| `hrd_tjk_sync_item_errors_status_check` | CHECK | `hrd_tjk_sync_item_errors` | OPEN/RESOLVED/IGNORED |
| `hrd_tjk_sync_item_errors_resolution_check` | CHECK | `hrd_tjk_sync_item_errors` | OPEN/resolved_at rules |
| `hrd_tjk_sync_item_errors_message_not_blank_check` | CHECK | `hrd_tjk_sync_item_errors` | message non-blank |
| `hrd_tjk_sync_item_errors_batch_context_object_check` | CHECK | `hrd_tjk_sync_item_errors` | jsonb object |
| `hrd_tjk_sync_item_errors_detail_object_check` | CHECK | `hrd_tjk_sync_item_errors` | jsonb object |

Status transition graph yalnız CHECK ile korunamaz; application + version + history TX.

## 16. Index planı

Table sütunu exact tablo adıdır. Partial satırlarda predicate açık yazılır. Çalıştırılabilir SQL yoktur.

| Exact index name | Table | Columns / expression concept | Unique/partial | Exact predicate (varsa) | Desteklediği sorgu |
| --- | --- | --- | --- | --- | --- |
| `hrd_users_email_normalized_key` | `hrd_users` | `email_normalized` | UNIQUE | — | Login |
| `hrd_users_status_idx` | `hrd_users` | `status` | — | — | Status filter |
| `hrd_auth_sessions_refresh_token_hash_key` | `hrd_auth_sessions` | `refresh_token_hash` | UNIQUE | — | Refresh |
| `hrd_auth_sessions_user_id_idx` | `hrd_auth_sessions` | `user_id` | — | — | User sessions |
| `hrd_auth_sessions_family_id_idx` | `hrd_auth_sessions` | `family_id` | — | — | Family revoke |
| `hrd_auth_sessions_active_lookup_idx` | `hrd_auth_sessions` | `user_id`, `client_context` | partial | `revoked_at IS NULL` | Active sessions |
| `hrd_one_time_credentials_token_hash_key` | `hrd_one_time_credentials` | `token_hash` | UNIQUE | — | Consume |
| `hrd_one_time_credentials_one_active_per_user_purpose_key` | `hrd_one_time_credentials` | `user_id`, `purpose` | partial UNIQUE | `consumed_at IS NULL AND invalidated_at IS NULL` | One active + lookup |
| `hrd_security_events_subject_created_idx` | `hrd_security_events` | `subject_user_id`, `created_at DESC` | — | — | Subject audit |
| `hrd_security_events_actor_created_idx` | `hrd_security_events` | `actor_user_id`, `created_at DESC` | — | — | Actor audit |
| `hrd_security_events_type_created_idx` | `hrd_security_events` | `event_type`, `created_at DESC` | — | — | Type audit |
| `hrd_provinces_name_normalized_prefix_idx` | `hrd_provinces` | `name_normalized` varchar_pattern_ops | — | — | Prefix LIKE |
| `hrd_districts_province_active_sort_idx` | `hrd_districts` | `province_id`, `is_active`, `sort_order` | — | — | Province’in aktif district dropdown’u; BO aktif/pasif filtreli liste |
| `hrd_districts_name_normalized_prefix_idx` | `hrd_districts` | `name_normalized` varchar_pattern_ops | — | — | Prefix LIKE |
| `hrd_categories_parent_id_sort_idx` | `hrd_categories` | `parent_id`, `sort_order` | — | — | Children |
| `hrd_category_properties_category_active_sort_idx` | `hrd_category_properties` | `category_id`, `is_active`, `sort_order` | — | — | Metadata |
| `hrd_horses_tjk_number_key` | `hrd_horses` | `tjk_number` | UNIQUE | — | Upsert |
| `hrd_horses_name_normalized_prefix_idx` | `hrd_horses` | `name_normalized` varchar_pattern_ops | — | — | Prefix autocomplete |
| `hrd_adverts_owner_user_id_created_idx` | `hrd_adverts` | `owner_user_id`, `created_at DESC` | partial | `deleted_at IS NULL` | Owner list |
| `hrd_adverts_public_newest_idx` | `hrd_adverts` | `published_at DESC`, `id DESC` | partial | `status = 'PUBLISHED' AND deleted_at IS NULL` | Public cursor |
| `hrd_adverts_public_category_newest_idx` | `hrd_adverts` | `category_id`, `published_at DESC`, `id DESC` | partial | `status = 'PUBLISHED' AND deleted_at IS NULL` | Category search |
| `hrd_adverts_public_district_newest_idx` | `hrd_adverts` | `district_id`, `published_at DESC`, `id DESC` | partial | `status = 'PUBLISHED' AND deleted_at IS NULL` | District search |
| `hrd_adverts_public_horse_newest_idx` | `hrd_adverts` | `horse_id`, `published_at DESC`, `id DESC` | partial | `status = 'PUBLISHED' AND deleted_at IS NULL AND horse_id IS NOT NULL` | Horse filter |
| `hrd_advert_status_history_advert_created_idx` | `hrd_advert_status_history` | `advert_id`, `created_at` | — | — | History |
| `hrd_favorites_user_id_advert_id_key` | `hrd_favorites` | `user_id`, `advert_id` | UNIQUE | — | Uniqueness |
| `hrd_favorites_user_created_idx` | `hrd_favorites` | `user_id`, `created_at DESC` | — | — | Favorite list |
| `hrd_favorites_advert_id_idx` | `hrd_favorites` | `advert_id` | — | — | Enrichment |
| `hrd_advert_media_advert_id_asset_id_key` | `hrd_advert_media` | `advert_id`, `asset_id` | UNIQUE | — | No dup |
| `hrd_advert_media_advert_id_display_order_key` | `hrd_advert_media` | `advert_id`, `display_order` | UNIQUE | — | Order + ordering |
| `hrd_advert_media_one_cover_key` | `hrd_advert_media` | `advert_id` | partial UNIQUE | `is_cover = true` | One cover |
| `hrd_advert_media_asset_id_idx` | `hrd_advert_media` | `asset_id` | — | — | Asset reverse ref / cleanup |
| `hrd_media_assets_owner_created_idx` | `hrd_media_assets` | `owner_user_id`, `created_at DESC` | — | — | Owner list |
| `hrd_media_assets_cleanup_idx` | `hrd_media_assets` | `lifecycle_status`, `updated_at` | partial | `lifecycle_status IN ('CLEANUP_CANDIDATE', 'DELETING')` | Cleanup |
| `hrd_media_assets_provider_raw_object_key_key` | `hrd_media_assets` | `provider`, `raw_object_key` | partial UNIQUE | `raw_object_key IS NOT NULL` | Raw key uniqueness |
| `hrd_media_assets_provider_master_object_key_key` | `hrd_media_assets` | `provider`, `master_object_key` | partial UNIQUE | `master_object_key IS NOT NULL` | Master key uniqueness |
| `hrd_media_variants_asset_id_transform_profile_key` | `hrd_media_variants` | `asset_id`, `transform_profile` | UNIQUE | — | Profile |
| `hrd_media_variants_object_key_key` | `hrd_media_variants` | `object_key` | partial UNIQUE | `object_key IS NOT NULL` | Variant key uniqueness |
| `hrd_media_variants_asset_status_idx` | `hrd_media_variants` | `asset_id`, `lifecycle_status` | — | — | Status |
| `hrd_banners_active_placement_sort_idx` | `hrd_banners` | `placement`, `sort_order` | partial | `status = 'ACTIVE'` | Public banners |
| `hrd_banners_asset_id_idx` | `hrd_banners` | `asset_id` | — | — | Asset reverse ref (INACTIVE dahil) |
| `hrd_background_jobs_claim_idx` | `hrd_background_jobs` | `status`, `available_at`, `id` | partial | `status = 'QUEUED'` | Claim |
| `hrd_background_jobs_lease_recovery_idx` | `hrd_background_jobs` | `status`, `leased_until` | partial | `status = 'LEASED'` | Lease recovery |
| `hrd_background_jobs_deduplication_key_key` | `hrd_background_jobs` | `deduplication_key` | partial UNIQUE | `deduplication_key IS NOT NULL` | Dedup |
| `hrd_background_jobs_tjk_sync_run_id_idx` | `hrd_background_jobs` | `tjk_sync_run_id` | — | — | Run jobs |
| `hrd_tjk_sync_runs_created_idx` | `hrd_tjk_sync_runs` | `created_at DESC` | — | — | History |
| `hrd_tjk_sync_runs_status_idx` | `hrd_tjk_sync_runs` | `status` | — | — | Status |
| `hrd_tjk_sync_runs_one_active_per_source_scope_key` | `hrd_tjk_sync_runs` | `source_adapter`, `scope_key` | partial UNIQUE | `status IN ('QUEUED', 'RUNNING')` | One active source/scope |
| `hrd_tjk_sync_runs_source_scope_created_idx` | `hrd_tjk_sync_runs` | `source_adapter`, `scope_key`, `created_at DESC` | — | — | Scope history |
| `hrd_tjk_sync_item_errors_run_status_idx` | `hrd_tjk_sync_item_errors` | `run_id`, `status` | — | — | Errors |
| `hrd_tjk_sync_item_errors_tjk_number_idx` | `hrd_tjk_sync_item_errors` | `tjk_number` | — | — | TJK lookup |

**Kaldırılan redundant:**

- `hrd_one_time_credentials_user_purpose_active_idx` (partial unique aynı predicate’i kapsar)
- `hrd_advert_media_advert_order_idx` (display_order unique ordering’i de destekler)
- `hrd_districts_province_id_idx` (province_id leading unique’ler + yeni `hrd_districts_province_active_sort_idx` yeterli)

Prefix: `varchar_pattern_ops`. Geniş GIN yok. Unique constraint index’leri ikinci kez oluşturulmaz.

## 17. JSONB sınırı

| Alan | Shape CHECK | Canonical? | Default |
| --- | --- | --- | --- |
| `hrd_category_properties.options` | array | Metadata | `[]` |
| `...validation` / `ui_metadata` | object | Metadata | `{}` |
| `...default_value` | yok | Metadata | NULL |
| `hrd_adverts.properties` | object | Evet | `{}` |
| `hrd_horses.detail` | object | Evet | `{}` |
| Media technical_metadata | object | Hayır | `{}` |
| Job payload / checkpoint / error contexts | object | Operasyonel | `{}` |
| Security metadata | object | Audit | `{}` |

Ham TJK payload kolonu yok.

## 18. Optimistic concurrency ve idempotency

| Entity | Mechanism |
| --- | --- |
| Advert content/status | `version` |
| Advert media collection | `media_version` |
| Banner / category / property | `version` |
| Auth session rotation | family revoke + unique hash |
| Background job claim | `version` + lease; transient retry → QUEUED (lease clear); `FAILED` ≠ `DEAD` |
| TJK sync run | `version` (checkpoint/status/counters/cancel) |
| One-time credentials | invalidate previous + one active unique |
| Favorites | UNIQUE idempotent |

## 19. Soft delete, deactivation ve physical delete matrisi

| Entity | Soft/deactivate | Physical |
| --- | --- | --- |
| User | DISABLED/CLOSED | Hayır (phase-one hard delete yok) |
| Geo/category/property | `is_active=false` | Hayır |
| Horse | Otomatik pasif yok | Hayır |
| Advert draft | `deleted_at` (DRAFT only) | Hayır |
| Moderated/published advert | Status | Hayır |
| Favorite / advert_media relation | — | Relation row yes |
| Media asset | CLEANUP/DELETING | PHYSICALLY_DELETED + B2 |
| Banner | INACTIVE | Hayır |
| Session/OTC/job/run | Revoke/consume/terminal | Retention purge opsiyonel |

FK delete notları:

- User hard delete yoktur; buna rağmen CHECK ile zorunlu kalan audit actor’lar için RESTRICT kullanılır: `hrd_advert_status_history_actor_user_id_fkey`, `hrd_tjk_sync_runs_created_by_user_id_fkey`.
- SET NULL korunanlar: security event subject/actor, banner created-by, session replaced-by, TJK item-error horse.

## 20. Migration sırası

1. `hrd_schema_migrations` (tool-owned)
2. IAM → GEO → CATALOG → HORSE → ADVERT → MEDIA → BANNER → TJK/WORKER
3. Indexes/constraints
4. Seed provinces/districts

Nullable advert draft alanları migration’da NULL olarak yaratılır; reviewed-status CHECK aynı migration grubunda.

## 21. Seed stratejisi

Province/district idempotent seed; category ürün onayı sonrası; TJK worker import; admin bootstrap secret’siz; `hr_`’den seed yok.

## 22. Shared DB güvenlik kontrol listesi

`hrd_` allowlist; `hr_` deny; tool metadata adı sabit; AutoMigrate production değil; `.env` Git’e girmez.

## 23. Açık fakat şemayı engellemeyen ürün kararları

Seller×advert visibility; pasif category; telefon; min media; banner size/schedule; TTL; TJK adapter; keyword search; favorite count; own-advert favorite; email-change session etkisi; migration tool kolonları; category-specific horse/price required.

## 24. Şema dışına ertelenen teknik kararlar

Go/GORM, repository, use-case, OpenAPI, migration tool seçimi, TJK adapter, Railway, monitoring.

## 25. Blueprint kabul kriterleri

Bu bölüm self-contained’dır; önceki belge sürümüne veya sohbet geçmişine referans vermez.

### Shared DB

1. Bütün application tabloları `hrd_` ile mi başlıyor? **Evet** — 19 domain tablo.
2. Global DB nesneleri (index/constraint) Haradan’a özgü `hrd_` adlarıyla mı adlandırılıyor? **Evet**.
3. `hr_` tablolarına FK/query/migration bağı var mı? **Hayır** — izolasyon korunur.
4. Migration metadata adı `hrd_schema_migrations` mi? **Evet** — sabit.
5. Metadata kolonları tool-owned mı (blueprint kolon kilitlemez)? **Evet**.
6. GORM AutoMigrate ortak production DB migration mekanizması mı? **Hayır**.
7. Secret / `.env` içeriği belgede var mı? **Hayır** — okunmaz/yazılmaz.

### IAM

8. Ayrı role/permission tabloları var mı? **Hayır**.
9. Role `varchar` + CHECK (`user`/`admin`) mi? **Evet**.
10. User status exact set (`ACTIVE`/`DISABLED`/`CLOSED`) mi? **Evet**.
11. First/last name modeli (`display_name` yok) mi? **Evet**.
12. Normalize email unique mi? **Evet** — `hrd_users_email_normalized_key`.
13. Refresh token hash plaintext saklanıyor mu? **Hayır** — hash unique.
14. Session rotation/family modeli var mı? **Evet**.
15. BO client context (`ADMIN_BO`) destekleniyor mu? **Evet**.
16. Email verify/change/reset one-time credential amaçları exact mı? **Evet**.
17. Security event actor/subject ayrımı var mı? **Evet**.
18. Security event `client_context` CHECK (NULL veya PUBLIC_WEB/MOBILE/ADMIN_BO) var mı? **Evet**.
19. User hard delete phase-one’da var mı? **Hayır**.

### GEO / CATALOG

20. Province ve district ayrı tablolar mı? **Evet**.
21. Advert province’i yalnız district üzerinden mi türetir? **Evet** — province FK advert’te yok.
22. Category tree self-parent destekliyor mu? **Evet**.
23. Direct self-parent CHECK (`hrd_categories_no_self_parent_check`) var mı? **Evet**.
24. Sort/order alanları non-negative CHECK’li mi? **Evet** — provinces/districts/categories/properties/banners/advert_media.
25. Property code category-scoped unique mi? **Evet**.
26. Property inheritance var mı? **Hayır**.
27. Advert dynamic values tek JSONB (`properties`) mi? **Evet**.
28. Ayrı advert-property-value tablosu var mı? **Hayır**.

### Horse

29. TJK number unique mi? **Evet**.
30. Horse name unique mi? **Hayır** — yalnız prefix index.
31. Typed core + controlled detail JSONB modeli mi? **Evet**.
32. Raw TJK payload canonical kolon mu? **Hayır**.
33. Horse hard delete var mı? **Hayır**.

### Advert

34. DRAFT/CHANGES_REQUESTED kısmi içerik (nullable category/district/title/description) destekliyor mu? **Evet**.
35. Review+ status’larda required field CHECK var mı? **Evet** — `hrd_adverts_reviewed_status_required_fields_check`.
36. Exact advert status seti kilitli mi? **Evet**.
37. Advert optimistic `version` var mı? **Evet**.
38. Ayrı `media_version` var mı? **Evet**.
39. Status history immutable mi? **Evet**.
40. History from/to exact value CHECK’leri var mı? **Evet**.
41. Price `price_amount_minor` + `price_currency` pair mi? **Evet** — float/JSONB yok.
42. Payment/package alanı veya tablosu var mı? **Hayır**.
43. Soft delete yalnız DRAFT (`deleted_at`) mı? **Evet**.
44. History actor FK ON DELETE RESTRICT mi? **Evet** — SET NULL CHECK ile çelişirdi.
45. Category/district cardinality: draft N:0..1, review+ N:1; horse N:0..1 (çok advert aynı horse’u referans edebilir) mi? **Evet**.

### Favorite / media / banner

46. Favorite unique/idempotent mi? **Evet**.
47. Raw/master/variant ayrımı var mı? **Evet**.
48. Asset ve variant lifecycle ayrı mı? **Evet**.
49. Media `content_type`/`byte_size`/`checksum`/`dims` canonical master metadata (tek anlam) mı? **Evet**.
50. MASTER_READY `byte_size` (+ master key/content_type/dims) gerektiriyor mu? **Evet**.
51. Object key uniqueness (partial, key IS NOT NULL) var mı? **Evet**.
52. Tek cover partial unique var mı? **Evet**.
53. Display order unique ve nonnegative mi? **Evet**.
54. Asset reverse indexes (`hrd_advert_media_asset_id_idx`, `hrd_banners_asset_id_idx`) var mı? **Evet**.
55. Banner advert-media relation kullanıyor mu? **Hayır** — shared asset FK.
56. Banner tek placement modeli mi? **Evet**.
57. Banner asset reference (INACTIVE dahil) korunuyor mu? **Evet** — reverse index + RESTRICT.

### Worker / TJK

58. Durable PostgreSQL job modeli mi? **Evet** — memory-only ana model değil.
59. Exact job type/status setleri kilitli mi? **Evet**.
60. `FAILED` ile `DEAD` ayrımı tanımlı mı? **Evet** — kalıcı hata vs retry bütçesi tükenmesi.
61. Transient + retry hakkı ⇒ tekrar `QUEUED` (lease clear) mi? **Evet**.
62. Job deduplication partial unique var mı? **Evet**.
63. Lease fields CHECK (LEASED vs non-LEASED) var mı? **Evet**.
64. TJK_SYNC_BATCH job run zorunlu; media job’da run NULL mı? **Evet**.
65. TJK run / job / item-error ayrımı var mı? **Evet**.
66. Scope yalnız `HORSES` mi? **Evet** — scope CHECK.
67. Tek active source/scope run partial unique var mı? **Evet**.
68. TJK run `version` var mı? **Evet**.
69. Manual TJK creator FK ON DELETE RESTRICT mi? **Evet**.
70. Her başarılı TJK item için sonsuz audit satırı var mı? **Hayır** — yalnız item errors.

### Performance / indexes

71. Public baseline advert indexleri exact predicate ile mi? **Evet** — PUBLISHED + deleted_at IS NULL (+ horse IS NOT NULL).
72. Index planı exact tablo adları ve predicates self-contained mı? **Evet**.
73. Pattern-ops prefix indexleri var mı? **Evet**.
74. Her dynamic property’ye otomatik index var mı? **Hayır**.
75. Geniş ölçümsüz GIN zorunlu mu? **Hayır**.
76. PostgreSQL extension zorunlu mu? **Hayır**.
77. District dropdown `hrd_districts_province_active_sort_idx` mi; redundant `province_id` alone index kaldırıldı mı? **Evet**.
78. OTC redundant active non-unique index kaldırıldı mı? **Evet**.

### Envanter ve hazırlık

79. Domain/metadata sayısı 19 + 1 = 20 mi? **Evet**.
80. Constraint planında wildcard/slash/yaklaşık ad var mı? **Hayır**.
81. Security event / banner created-by SET NULL yanlışlıkla RESTRICT’e çekildi mi? **Hayır** — korunur.
82. Schema blocker var mı? **Hayır**.
83. Blueprint migration / use-case / API aşamasına hazır mı? **Evet** — şema kilitli; sonraki adım use-case listesi.

## 26. Sonraki adım

Blueprint kabulü sonrası: use-case listesi, FE/BO API, worker fonksiyonları, OpenAPI, migration tool + dosyalar, Go bootstrap, modül geliştirme.
