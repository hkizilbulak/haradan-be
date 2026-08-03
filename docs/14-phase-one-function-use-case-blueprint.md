# Haradan Phase-One Backend Fonksiyon ve Use-Case Blueprint’i

## 1. Belgenin amacı ve kapsamı

Bu belge phase-one backend için exact business/application use-case listesidir. Kaynak: `docs/01`–`docs/13` kabul edilmiş kararlar.

Belge:

- Public FE, authenticated FE, BO ve worker fonksiyonlarını ayırır
- Actor/client sınırını, tablo okuma/yazmayı, transaction, concurrency, idempotency ve side effect’leri kilitler
- API/OpenAPI aşamasına doğrudan kaynak oluşturur

Belge değildir:

- Endpoint/path, HTTP method, OpenAPI, request/response JSON
- Go/GORM/repository/handler kodu
- SQL/migration/DBML

## 2. Kaynak kararlar ve sınırlar

- Exact veri modeli: `docs/13` (19 domain table; `hrd_schema_migrations` business use-case değildir)
- `hr_` tablolarına okuma/yazma yok
- Yeni tablo varsayılmaz
- Secret/`.env` okunmaz
- Payment/package/campaign/article/contact/YKK fonksiyonu yok

## 3. Exposure sınıfları

| Exposure | Anlam |
| --- | --- |
| `PUBLIC` | Authentication zorunlu değil |
| `FE_AUTH` | ACTIVE authenticated normal kullanıcı (PUBLIC_WEB veya MOBILE session) |
| `BO_AUTH` | Canonical role=`admin` **ve** session `client_context=ADMIN_BO` |
| `INTERNAL_WORKER` | Background worker |
| `INTERNAL_SYSTEM` | Scheduler/orchestration veya başka use-case tarafından çağrılan internal primitive |

Auth-optional public projection: exposure `PUBLIC` kalır; ACTIVE user varsa `is_favorite` enrichment yapılabilir; user-specific veri public cache’e karışmaz.

## 4. Actor ve client kuralları

- FE/BO gönderilen owner/actor/role/client_context güvenilir değildir; server session’dan türetilir
- DISABLED/CLOSED korumalı işlem yapamaz
- FE/MOBILE session admin olsa bile BO fonksiyonu çağırmaz
- Admin ownership’i otomatik bypass etmez
- Worker kullanıcı session’ı taklit etmez
- System actor nullable olabilir; audit açık yazılır

## 5. Domain error taxonomy

Exact application error seti (HTTP mapping sonraki aşama):

| Code | Sınıf |
| --- | --- |
| `VALIDATION_ERROR` | Validation failure |
| `UNAUTHENTICATED` | Authentication failure |
| `FORBIDDEN` | Authorization failure (role/context) |
| `ACCOUNT_INACTIVE` | DISABLED/CLOSED account |
| `EMAIL_NOT_VERIFIED` | Verification gate (ör. submit) |
| `NOT_FOUND` | Not found (enumeration-safe olduğu yerlerde generic) |
| `CONFLICT` | Unique/state conflict |
| `STALE_VERSION` | Optimistic concurrency conflict |
| `INVALID_STATE` | Invalid lifecycle transition / status precondition |
| `OWNERSHIP_REQUIRED` | Ownership failure |
| `DUPLICATE` | Idempotent-friendly duplicate (favoritede tercih: success) |
| `TOKEN_INVALID` | Credential/token invalid |
| `TOKEN_EXPIRED` | Credential/token expired |
| `TOKEN_ALREADY_USED` | One-time already consumed |
| `SESSION_REVOKED` | Session revoked |
| `REFRESH_REPLAY_DETECTED` | Refresh replay |
| `RATE_LIMITED` | Brute-force / resend / abuse |
| `DEPENDENCY_UNAVAILABLE` | External dependency failure (B2/TinyPNG/TJK/email) |
| `PROCESSING_NOT_READY` | Media/job not ready for required use |
| `QUERY_TOO_COMPLEX` | Search filter whitelist ihlali |
| `INTERNAL_ERROR` | Unexpected internal failure |


## 5A. Security event yazım politikası (exact)

`INTERNAL-01 AppendSecurityEvent` iki kavramsal mod destekler:

| Mode | Anlam |
| --- | --- |
| `REQUIRED_CALLER_TRANSACTION` | Caller’ın mevcut DB transaction’ını kullanır; ayrı commit açmaz; insert failure caller transaction’ını başarısız yapar |
| `BEST_EFFORT_INDEPENDENT` | Ana transaction commit **sonrasında** ayrı kısa transaction; failure ana business sonucunu değiştirmez; hata loglanır ve monitoring’de görünür olmalıdır |

**“Best-effort same transaction” yoktur.**

### Zorunlu transactional security event’leri (`REQUIRED_CALLER_TRANSACTION`)

Ana güvenlik işlemiyle aynı DB transaction içinde yazılır; insert başarısızsa işlem rollback:

- `PASSWORD_RESET`
- `PASSWORD_CHANGE`
- `EMAIL_CHANGE`
- `ROLE_CHANGE`
- `ACCOUNT_STATUS_CHANGE`
- `REFRESH_REPLAY_DETECTED`

### Best-effort bağımsız security event’leri (`BEST_EFFORT_INDEPENDENT`)

Ana işlem tamamlandıktan sonra bağımsız kısa transaction ile yazılabilir; audit failure ana sonucu rollback etmez:

- `LOGIN_SUCCESS`
- `LOGIN_FAILURE`
- `LOGOUT`
- `SESSION_REVOKED`
- `ALL_SESSIONS_REVOKED`
- `EMAIL_VERIFICATION` — yalnız `AUTH-02 VerifyRegistrationEmail` başarılı tamamlandığında
- `BO_CONTEXT_REJECTED`

`AUTH-01 RegisterUser` security event yazmaz (`EMAIL_VERIFICATION` yanlış anlamda kullanılmaz; ayrı `USER_REGISTERED` / `EMAIL_VERIFICATION_REQUESTED` event type yoktur).

Writes alanlarında zorunlu event: `hrd_security_events`. Best-effort: `Conditional: hrd_security_events`. Best-effort event, caller transaction’ıyla aynı transaction içinde yazılmaz.

## 6. Fonksiyon özet envanteri

| Domain | PUBLIC | FE_AUTH | BO_AUTH | INTERNAL_WORKER | INTERNAL_SYSTEM | Total |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| SYS | 1 | 0 | 0 | 0 | 0 | 1 |
| AUTH | 8 | 6 | 0 | 0 | 0 | 14 |
| ACCOUNT | 0 | 2 | 0 | 0 | 0 | 2 |
| ADMIN-USER | 0 | 0 | 5 | 0 | 0 | 5 |
| GEO | 4 | 0 | 0 | 0 | 0 | 4 |
| CATALOG | 2 | 0 | 0 | 0 | 0 | 2 |
| ADMIN-CATALOG | 0 | 0 | 12 | 0 | 0 | 12 |
| HORSE | 2 | 0 | 0 | 0 | 0 | 2 |
| ADVERT-PUBLIC | 2 | 0 | 0 | 0 | 0 | 2 |
| ADVERT-OWNER | 0 | 11 | 0 | 0 | 0 | 11 |
| ADVERT-ADMIN | 0 | 0 | 6 | 0 | 0 | 6 |
| FAVORITE | 0 | 3 | 0 | 0 | 0 | 3 |
| MEDIA | 0 | 7 | 3 | 0 | 0 | 10 |
| BANNER | 1 | 0 | 6 | 0 | 0 | 7 |
| JOB | 0 | 0 | 0 | 5 | 2 | 7 |
| MEDIA-WORKER | 0 | 0 | 0 | 4 | 0 | 4 |
| TJK-ADMIN | 0 | 0 | 7 | 0 | 0 | 7 |
| TJK-WORKER | 0 | 0 | 0 | 6 | 1 | 7 |
| INTERNAL | 0 | 0 | 0 | 0 | 6 | 6 |
| **Toplam** | **20** | **29** | **39** | **15** | **9** | **112** |

**Exposure totals:** PUBLIC 20 · FE_AUTH 29 · BO_AUTH 39 · INTERNAL_WORKER 15 · INTERNAL_SYSTEM 9 · **Grand total 112**

Her fonksiyon tek exposure altında bir kez sayılır.

## 7. Exact function catalog (ID → name)

| ID | Exact name | Exposure |
| --- | --- | --- |
| SYS-01 | GetHealth | PUBLIC |
| AUTH-01 | RegisterUser | PUBLIC |
| AUTH-02 | VerifyRegistrationEmail | PUBLIC |
| AUTH-03 | ResendRegistrationEmailVerification | PUBLIC |
| AUTH-04 | Login | PUBLIC |
| AUTH-05 | RefreshSession | PUBLIC |
| AUTH-06 | LogoutCurrentSession | FE_AUTH |
| AUTH-07 | LogoutAllSessions | FE_AUTH |
| AUTH-08 | ListMySessions | FE_AUTH |
| AUTH-09 | RevokeMySession | FE_AUTH |
| AUTH-10 | RequestPasswordReset | PUBLIC |
| AUTH-11 | ResetPassword | PUBLIC |
| AUTH-12 | ChangePassword | FE_AUTH |
| AUTH-13 | RequestEmailChange | FE_AUTH |
| AUTH-14 | ConfirmEmailChange | PUBLIC |
| ACCOUNT-01 | GetMyProfile | FE_AUTH |
| ACCOUNT-02 | UpdateMyProfile | FE_AUTH |
| ADMIN-USER-01 | ListUsers | BO_AUTH |
| ADMIN-USER-02 | GetUserAdminDetail | BO_AUTH |
| ADMIN-USER-03 | ChangeUserStatus | BO_AUTH |
| ADMIN-USER-04 | ChangeUserRole | BO_AUTH |
| ADMIN-USER-05 | ListUserSecurityEvents | BO_AUTH |
| GEO-01 | ListActiveProvinces | PUBLIC |
| GEO-02 | SearchProvinces | PUBLIC |
| GEO-03 | ListDistrictsByProvince | PUBLIC |
| GEO-04 | SearchDistricts | PUBLIC |
| CATALOG-01 | GetPublicCategoryTree | PUBLIC |
| CATALOG-02 | GetCategoryFormDefinition | PUBLIC |
| ADMIN-CATALOG-01 | ListCategoriesAdmin | BO_AUTH |
| ADMIN-CATALOG-02 | GetCategoryAdminDetail | BO_AUTH |
| ADMIN-CATALOG-03 | CreateCategory | BO_AUTH |
| ADMIN-CATALOG-04 | UpdateCategory | BO_AUTH |
| ADMIN-CATALOG-05 | ReparentCategory | BO_AUTH |
| ADMIN-CATALOG-06 | SetCategoryActive | BO_AUTH |
| ADMIN-CATALOG-07 | ReorderCategories | BO_AUTH |
| ADMIN-CATALOG-08 | ListCategoryPropertiesAdmin | BO_AUTH |
| ADMIN-CATALOG-09 | CreateCategoryProperty | BO_AUTH |
| ADMIN-CATALOG-10 | UpdateCategoryProperty | BO_AUTH |
| ADMIN-CATALOG-11 | SetCategoryPropertyActive | BO_AUTH |
| ADMIN-CATALOG-12 | ReorderCategoryProperties | BO_AUTH |
| HORSE-01 | SearchHorsesForSelection | PUBLIC |
| HORSE-02 | GetHorsePublicDetail | PUBLIC |
| ADVERT-PUBLIC-01 | SearchPublishedAdverts | PUBLIC |
| ADVERT-PUBLIC-02 | GetPublishedAdvertDetail | PUBLIC |
| ADVERT-OWNER-01 | CreateAdvertDraft | FE_AUTH |
| ADVERT-OWNER-02 | ListMyAdverts | FE_AUTH |
| ADVERT-OWNER-03 | GetMyAdvert | FE_AUTH |
| ADVERT-OWNER-04 | UpdateAdvertDraftDetails | FE_AUTH |
| ADVERT-OWNER-05 | ChangeAdvertDraftCategory | FE_AUTH |
| ADVERT-OWNER-06 | ReplaceAdvertDynamicProperties | FE_AUTH |
| ADVERT-OWNER-07 | SubmitAdvertForReview | FE_AUTH |
| ADVERT-OWNER-08 | ResubmitAdvertForReview | FE_AUTH |
| ADVERT-OWNER-09 | SoftDeleteAdvertDraft | FE_AUTH |
| ADVERT-OWNER-10 | MarkAdvertSold | FE_AUTH |
| ADVERT-OWNER-11 | ArchiveAdvert | FE_AUTH |
| ADVERT-ADMIN-01 | ListAdvertModerationQueue | BO_AUTH |
| ADVERT-ADMIN-02 | GetAdvertModerationDetail | BO_AUTH |
| ADVERT-ADMIN-03 | ApproveAdvert | BO_AUTH |
| ADVERT-ADMIN-04 | RequestAdvertChanges | BO_AUTH |
| ADVERT-ADMIN-05 | RejectAdvert | BO_AUTH |
| ADVERT-ADMIN-06 | SuspendAdvert | BO_AUTH |
| FAVORITE-01 | AddFavorite | FE_AUTH |
| FAVORITE-02 | RemoveFavorite | FE_AUTH |
| FAVORITE-03 | ListMyFavorites | FE_AUTH |
| MEDIA-01 | InitiateMediaUpload | FE_AUTH |
| MEDIA-02 | ConfirmMediaUpload | FE_AUTH |
| MEDIA-03 | GetMediaProcessingStatus | FE_AUTH |
| MEDIA-04 | AttachMediaToAdvert | FE_AUTH |
| MEDIA-05 | DetachMediaFromAdvert | FE_AUTH |
| MEDIA-06 | ReorderAdvertMedia | FE_AUTH |
| MEDIA-07 | SetAdvertCover | FE_AUTH |
| MEDIA-ADMIN-01 | InitiateAdminMediaUpload | BO_AUTH |
| MEDIA-ADMIN-02 | ConfirmAdminMediaUpload | BO_AUTH |
| MEDIA-ADMIN-03 | GetAdminMediaProcessingStatus | BO_AUTH |
| BANNER-PUBLIC-01 | ListActiveBannersByPlacement | PUBLIC |
| BANNER-ADMIN-01 | ListBannersAdmin | BO_AUTH |
| BANNER-ADMIN-02 | GetBannerAdminDetail | BO_AUTH |
| BANNER-ADMIN-03 | CreateBanner | BO_AUTH |
| BANNER-ADMIN-04 | UpdateBanner | BO_AUTH |
| BANNER-ADMIN-05 | SetBannerStatus | BO_AUTH |
| BANNER-ADMIN-06 | ReorderBanners | BO_AUTH |
| JOB-01 | EnqueueBackgroundJob | INTERNAL_SYSTEM |
| JOB-02 | ClaimAvailableBackgroundJobs | INTERNAL_WORKER |
| JOB-03 | RenewBackgroundJobLease | INTERNAL_WORKER |
| JOB-04 | CompleteBackgroundJob | INTERNAL_WORKER |
| JOB-05 | RetryOrFailBackgroundJob | INTERNAL_WORKER |
| JOB-06 | RequestBackgroundJobCancellation | INTERNAL_SYSTEM |
| JOB-07 | RecoverExpiredBackgroundJobLeases | INTERNAL_WORKER |
| MEDIA-WORKER-01 | ValidateAndNormalizeMediaAsset | INTERNAL_WORKER |
| MEDIA-WORKER-02 | GenerateMediaVariant | INTERNAL_WORKER |
| MEDIA-WORKER-03 | DeleteMediaObjects | INTERNAL_WORKER |
| MEDIA-WORKER-04 | ReconcileMediaStorage | INTERNAL_WORKER |
| TJK-ADMIN-01 | TriggerTJKSync | BO_AUTH |
| TJK-ADMIN-02 | CancelTJKSync | BO_AUTH |
| TJK-ADMIN-03 | ListTJKSyncRuns | BO_AUTH |
| TJK-ADMIN-04 | GetTJKSyncRun | BO_AUTH |
| TJK-ADMIN-05 | ListTJKSyncItemErrors | BO_AUTH |
| TJK-ADMIN-06 | ResolveTJKSyncItemError | BO_AUTH |
| TJK-ADMIN-07 | IgnoreTJKSyncItemError | BO_AUTH |
| TJK-WORKER-01 | TriggerScheduledTJKSync | INTERNAL_SYSTEM |
| TJK-WORKER-02 | StartTJKSyncRun | INTERNAL_WORKER |
| TJK-WORKER-03 | PlanTJKSyncBatches | INTERNAL_WORKER |
| TJK-WORKER-04 | ProcessTJKSyncBatch | INTERNAL_WORKER |
| TJK-WORKER-05 | AdvanceTJKSyncCheckpoint | INTERNAL_WORKER |
| TJK-WORKER-06 | FinalizeTJKSyncRun | INTERNAL_WORKER |
| TJK-WORKER-07 | ApplyTJKSyncCancellation | INTERNAL_WORKER |
| INTERNAL-01 | AppendSecurityEvent | INTERNAL_SYSTEM |
| INTERNAL-02 | TransitionAdvertStatus | INTERNAL_SYSTEM |
| INTERNAL-03 | RevokeUserSessions | INTERNAL_SYSTEM |
| INTERNAL-04 | ValidateAdvertForSubmission | INTERNAL_SYSTEM |
| INTERNAL-05 | ValidateDynamicProperties | INTERNAL_SYSTEM |
| INTERNAL-06 | ResolvePublicAdvertProjection | INTERNAL_SYSTEM |

## 8. SYS

#### `SYS-01` — `GetHealth`

| Alan | İçerik |
| --- | --- |
| ID | `SYS-01` |
| Exact function name | `GetHealth` |
| Domain | SYS |
| Exposure | `PUBLIC` |
| Actor | Anonymous veya probe |
| Purpose | Servis readiness/liveness için minimum sağlık bilgisi döndürür. |
| Preconditions | Yok (auth yok). |
| Input summary | Opsiyonel probe kind (API aşamasında netleşir). |
| Output summary | Aggregate healthy/degraded sinyali; secret/config yok. |
| Reads | Opsiyonel hafif DB ping (implementation); business tablo yok |
| Writes | None |
| Transaction boundary | Write yok; uzun işlem yok. |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `DEPENDENCY_UNAVAILABLE`, `INTERNAL_ERROR` |
| Notes | DB secret, internal config, hassas dependency ayrıntısı sızdırmaz. |
## 9. AUTH / IAM

### Summary

| ID | Name | Exposure |
| --- | --- | --- |
| AUTH-01 | RegisterUser | PUBLIC |
| AUTH-02 | VerifyRegistrationEmail | PUBLIC |
| AUTH-03 | ResendRegistrationEmailVerification | PUBLIC |
| AUTH-04 | Login | PUBLIC |
| AUTH-05 | RefreshSession | PUBLIC |
| AUTH-06 | LogoutCurrentSession | FE_AUTH |
| AUTH-07 | LogoutAllSessions | FE_AUTH |
| AUTH-08 | ListMySessions | FE_AUTH |
| AUTH-09 | RevokeMySession | FE_AUTH |
| AUTH-10 | RequestPasswordReset | PUBLIC |
| AUTH-11 | ResetPassword | PUBLIC |
| AUTH-12 | ChangePassword | FE_AUTH |
| AUTH-13 | RequestEmailChange | FE_AUTH |
| AUTH-14 | ConfirmEmailChange | PUBLIC |

#### `AUTH-01` — `RegisterUser`

| Alan | İçerik |
| --- | --- |
| ID | `AUTH-01` |
| Exact function name | `RegisterUser` |
| Domain | IAM |
| Exposure | `PUBLIC` |
| Actor | Anonymous |
| Purpose | Yeni user hesabı oluşturur ve email verification credential üretir. |
| Preconditions | Email normalize unique; password policy; role client’tan alınmaz. |
| Input summary | Email, password, first_name, last_name; opsiyonel phone. |
| Output summary | Generic success (enumeration-safe); verification gönderildi sinyali. |
| Reads | `hrd_users`, `hrd_one_time_credentials` |
| Writes | `hrd_users`, `hrd_one_time_credentials` |
| Transaction boundary | User insert + EMAIL_VERIFICATION purpose credential create + önceki aktif invalidate aynı TX. Commit sonrası email provider çağrısı (ayrı durable email queue/tablosu yok). |
| Concurrency | email_normalized unique |
| Idempotency | Non-idempotent; duplicate email → CONFLICT/generic |
| Side effects | Commit sonrası email provider çağrısı. Güvenlik event’i yok. |
| Primary domain errors | `VALIDATION_ERROR`, `CONFLICT`, `RATE_LIMITED`, `INTERNAL_ERROR`, `DEPENDENCY_UNAVAILABLE` |
| Notes | Default role=`user`, status=`ACTIVE`; password hash; plaintext yok. `EMAIL_VERIFICATION` event yalnız AUTH-02 başarısında yazılır. |

#### `AUTH-02` — `VerifyRegistrationEmail`

| Alan | İçerik |
| --- | --- |
| ID | `AUTH-02` |
| Exact function name | `VerifyRegistrationEmail` |
| Domain | IAM |
| Exposure | `PUBLIC` |
| Actor | Anonymous (token) |
| Purpose | Registration email verification credential’ını consume eder. |
| Preconditions | Token hash lookup; expired/used/invalidated reddi. |
| Input summary | Verification token. |
| Output summary | Verified confirmation. |
| Reads | `hrd_one_time_credentials`, `hrd_users` |
| Writes | `hrd_one_time_credentials`, `hrd_users`; Conditional: `hrd_security_events` |
| Transaction boundary | Credential consume + `email_verified_at` set aynı TX. |
| Concurrency | one-active / consume race → conflict |
| Idempotency | Conditional: already verified → idempotent success tercih |
| Side effects | AppendSecurityEvent(EMAIL_VERIFICATION) mode=`BEST_EFFORT_INDEPENDENT` commit sonrası |
| Primary domain errors | `TOKEN_INVALID`, `TOKEN_EXPIRED`, `TOKEN_ALREADY_USED`, `NOT_FOUND`, `INTERNAL_ERROR` |
| Notes | Purpose yalnız EMAIL_VERIFICATION. |

#### `AUTH-03` — `ResendRegistrationEmailVerification`

| Alan | İçerik |
| --- | --- |
| ID | `AUTH-03` |
| Exact function name | `ResendRegistrationEmailVerification` |
| Domain | IAM |
| Exposure | `PUBLIC` |
| Actor | Anonymous |
| Purpose | Aktif olmayan verification için yeni credential üretir. |
| Preconditions | Rate limit; user ACTIVE; henüz verified değil. |
| Input summary | Email. |
| Output summary | Generic success (enumeration-safe). |
| Reads | `hrd_users`, `hrd_one_time_credentials` |
| Writes | `hrd_one_time_credentials` |
| Transaction boundary | Önceki aktif EMAIL_VERIFICATION invalidate + yeni create aynı TX. Commit sonrası email provider çağrısı (ayrı durable email queue yok). |
| Concurrency | one-active partial unique |
| Idempotency | Her çağrı yeni credential; yanıt generic |
| Side effects | Email provider TX dışı; provider failure credential geçerli kalır |
| Primary domain errors | `RATE_LIMITED`, `VALIDATION_ERROR`, `INTERNAL_ERROR`, `DEPENDENCY_UNAVAILABLE` |
| Notes | User existence sızdırmaz. |

#### `AUTH-04` — `Login`

| Alan | İçerik |
| --- | --- |
| ID | `AUTH-04` |
| Exact function name | `Login` |
| Domain | IAM |
| Exposure | `PUBLIC` |
| Actor | Anonymous → session |
| Purpose | Email/password ile session açar; context’e göre FE veya BO session üretir. |
| Preconditions | Normalize email; password verify; status ACTIVE; BO için role=admin + client_context=ADMIN_BO; FE için PUBLIC_WEB/MOBILE; locked_until kontrolü. |
| Input summary | Email, password, client_context (server-validated allowlist). |
| Output summary | Access token (DB’ye yazılmaz) + refresh plaintext yalnız response; session metadata. |
| Reads | `hrd_users`, `hrd_auth_sessions` |
| Writes | `hrd_users`, `hrd_auth_sessions`; Conditional: `hrd_security_events` |
| Transaction boundary | **Başarısız (user bulundu):** kısa TX içinde `failed_login_count` (+ gerekirse `locked_until`) güncellenir; session yok; commit sonrası LOGIN_FAILURE `BEST_EFFORT_INDEPENDENT`. **User yok:** session yok; subject null LOGIN_FAILURE best-effort olabilir; enumeration yok. **Başarılı:** aynı TX içinde `failed_login_count` reset, `locked_until` clear, `hrd_auth_sessions` insert; commit sonrası LOGIN_SUCCESS (veya BO_CONTEXT_REJECTED) `BEST_EFFORT_INDEPENDENT`. |
| Concurrency | refresh hash unique; user row update race controlled |
| Idempotency | Non-idempotent |
| Side effects | LOGIN_SUCCESS / LOGIN_FAILURE / BO_CONTEXT_REJECTED → `BEST_EFFORT_INDEPENDENT` |
| Primary domain errors | `UNAUTHENTICATED`, `ACCOUNT_INACTIVE`, `FORBIDDEN`, `EMAIL_NOT_VERIFIED`, `RATE_LIMITED`, `VALIDATION_ERROR` |
| Notes | Exact rate-limit sayısı/harici store bu belgede kilitlenmez. Refresh hash saklanır; plaintext DB’de yok. |

#### `AUTH-05` — `RefreshSession`

| Alan | İçerik |
| --- | --- |
| ID | `AUTH-05` |
| Exact function name | `RefreshSession` |
| Domain | IAM |
| Exposure | `PUBLIC` |
| Actor | Refresh-token bearer |
| Purpose | Geçerli refresh ile access yeniler ve refresh rotate eder. |
| Preconditions | Hash lookup; revoked değil; absolute/idle TTL; client_context eşleşmesi; user ACTIVE. |
| Input summary | Refresh token, client_context binding. |
| Output summary | Yeni access + yeni refresh. |
| Reads | `hrd_auth_sessions`, `hrd_users` |
| Writes | `hrd_auth_sessions`; on replay: `hrd_security_events` (REQUIRED_CALLER_TRANSACTION) |
| Transaction boundary | Normal rotate: eski session revoke/replace + yeni session aynı TX. **Replay:** family revoke + `REFRESH_REPLAY_DETECTED` insert aynı TX (`REQUIRED_CALLER_TRANSACTION`); stale worker/completion yok. |
| Concurrency | Atomic rotation; unique refresh hash |
| Idempotency | Non-idempotent (rotation); replay → REFRESH_REPLAY_DETECTED |
| Side effects | Replay: AppendSecurityEvent(REFRESH_REPLAY_DETECTED) mode=`REQUIRED_CALLER_TRANSACTION` |
| Primary domain errors | `TOKEN_INVALID`, `SESSION_REVOKED`, `REFRESH_REPLAY_DETECTED`, `ACCOUNT_INACTIVE`, `FORBIDDEN` |
| Notes | Kısa access token DB’ye yazılmaz. |

#### `AUTH-06` — `LogoutCurrentSession`

| Alan | İçerik |
| --- | --- |
| ID | `AUTH-06` |
| Exact function name | `LogoutCurrentSession` |
| Domain | IAM |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user |
| Purpose | Mevcut session’ı revoke eder. |
| Preconditions | ACTIVE session. |
| Input summary | Current session identity (server). |
| Output summary | Success; zaten revoked ise success. |
| Reads | `hrd_auth_sessions` |
| Writes | `hrd_auth_sessions`; Conditional: `hrd_security_events` |
| Transaction boundary | Revoke current session TX. Commit sonrası LOGOUT/SESSION_REVOKED best-effort. |
| Concurrency | — |
| Idempotency | Idempotent |
| Side effects | AppendSecurityEvent(LOGOUT veya SESSION_REVOKED) mode=`BEST_EFFORT_INDEPENDENT` |
| Primary domain errors | `UNAUTHENTICATED`, `ACCOUNT_INACTIVE` |
| Notes | Access blacklist zorunlu değil (kısa TTL). |

#### `AUTH-07` — `LogoutAllSessions`

| Alan | İçerik |
| --- | --- |
| ID | `AUTH-07` |
| Exact function name | `LogoutAllSessions` |
| Domain | IAM |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user |
| Purpose | Kullanıcının tüm session’larını revoke eder. |
| Preconditions | ACTIVE user. |
| Input summary | None beyond auth. |
| Output summary | Success. |
| Reads | `hrd_auth_sessions` |
| Writes | `hrd_auth_sessions`; Conditional: `hrd_security_events` |
| Transaction boundary | Tüm session revoke aynı TX. Commit sonrası ALL_SESSIONS_REVOKED best-effort. |
| Concurrency | — |
| Idempotency | Idempotent |
| Side effects | AppendSecurityEvent(ALL_SESSIONS_REVOKED) mode=`BEST_EFFORT_INDEPENDENT` |
| Primary domain errors | `UNAUTHENTICATED`, `ACCOUNT_INACTIVE` |
| Notes | — |

#### `AUTH-08` — `ListMySessions`

| Alan | İçerik |
| --- | --- |
| ID | `AUTH-08` |
| Exact function name | `ListMySessions` |
| Domain | IAM |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user |
| Purpose | Kullanıcının aktif/geçmiş session özetini listeler. |
| Preconditions | ACTIVE. |
| Input summary | Pagination cursor (API aşaması). |
| Output summary | Session list (hash/token yok). |
| Reads | `hrd_auth_sessions` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `UNAUTHENTICATED`, `ACCOUNT_INACTIVE` |
| Notes | Refresh plaintext/hash döndürülmez. |
#### `AUTH-09` — `RevokeMySession`

| Alan | İçerik |
| --- | --- |
| ID | `AUTH-09` |
| Exact function name | `RevokeMySession` |
| Domain | IAM |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user |
| Purpose | Belirtilen kendi session’ını revoke eder. |
| Preconditions | ACTIVE; session owner = current user. |
| Input summary | Target session id. |
| Output summary | Success; already revoked idempotent. |
| Reads | `hrd_auth_sessions` |
| Writes | `hrd_auth_sessions`; Conditional: `hrd_security_events` |
| Transaction boundary | Revoke TX. Commit sonrası SESSION_REVOKED best-effort. |
| Concurrency | — |
| Idempotency | Idempotent |
| Side effects | AppendSecurityEvent(SESSION_REVOKED) mode=`BEST_EFFORT_INDEPENDENT` |
| Primary domain errors | `NOT_FOUND`, `OWNERSHIP_REQUIRED`, `UNAUTHENTICATED` |
| Notes | — |

#### `AUTH-10` — `RequestPasswordReset`

| Alan | İçerik |
| --- | --- |
| ID | `AUTH-10` |
| Exact function name | `RequestPasswordReset` |
| Domain | IAM |
| Exposure | `PUBLIC` |
| Actor | Anonymous |
| Purpose | PASSWORD_RESET credential üretir. |
| Preconditions | Rate limit. |
| Input summary | Email. |
| Output summary | Generic success. |
| Reads | `hrd_users`, `hrd_one_time_credentials` |
| Writes | `hrd_one_time_credentials` (user bulunduysa) |
| Transaction boundary | User bulunduysa: invalidate previous + create credential aynı TX. Commit sonrası email provider (ayrı durable email queue yok). |
| Concurrency | one-active unique |
| Idempotency | Response always generic |
| Side effects | Email provider TX dışı; failure credential geçerli kalır |
| Primary domain errors | `RATE_LIMITED`, `VALIDATION_ERROR`, `DEPENDENCY_UNAVAILABLE` |
| Notes | User existence sızdırmaz. |

#### `AUTH-11` — `ResetPassword`

| Alan | İçerik |
| --- | --- |
| ID | `AUTH-11` |
| Exact function name | `ResetPassword` |
| Domain | IAM |
| Exposure | `PUBLIC` |
| Actor | Anonymous (token) |
| Purpose | Reset credential consume + password + stamp + tüm session revoke. |
| Preconditions | Valid PASSWORD_RESET credential. |
| Input summary | Token, new password. |
| Output summary | Success; re-login required. |
| Reads | `hrd_one_time_credentials`, `hrd_users`, `hrd_auth_sessions` |
| Writes | `hrd_one_time_credentials`, `hrd_users`, `hrd_auth_sessions`, `hrd_security_events` |
| Transaction boundary | Consume + password_hash + security stamp rotate + RevokeUserSessions(all) + AppendSecurityEvent(PASSWORD_RESET) `REQUIRED_CALLER_TRANSACTION` aynı TX. |
| Concurrency | consume race |
| Idempotency | Non-idempotent after consume |
| Side effects | PASSWORD_RESET mandatory transactional event |
| Primary domain errors | `TOKEN_INVALID`, `TOKEN_EXPIRED`, `TOKEN_ALREADY_USED`, `VALIDATION_ERROR`, `INTERNAL_ERROR` |
| Notes | Admin password set fonksiyonu yok. |

#### `AUTH-12` — `ChangePassword`

| Alan | İçerik |
| --- | --- |
| ID | `AUTH-12` |
| Exact function name | `ChangePassword` |
| Domain | IAM |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user |
| Purpose | Mevcut şifre doğrulayarak password değiştirir. |
| Preconditions | ACTIVE; current password correct. |
| Input summary | Current password, new password. |
| Output summary | Success; other sessions revoked; current reauth/rotation. |
| Reads | `hrd_users`, `hrd_auth_sessions` |
| Writes | `hrd_users`, `hrd_auth_sessions`, `hrd_security_events` |
| Transaction boundary | Password+stamp update + diğer session revoke + current rotation + AppendSecurityEvent(PASSWORD_CHANGE) `REQUIRED_CALLER_TRANSACTION` aynı TX. |
| Concurrency | — |
| Idempotency | Non-idempotent |
| Side effects | PASSWORD_CHANGE mandatory transactional event |
| Primary domain errors | `UNAUTHENTICATED`, `VALIDATION_ERROR`, `ACCOUNT_INACTIVE` |
| Notes | — |

#### `AUTH-13` — `RequestEmailChange`

| Alan | İçerik |
| --- | --- |
| ID | `AUTH-13` |
| Exact function name | `RequestEmailChange` |
| Domain | IAM |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user |
| Purpose | Target email için EMAIL_CHANGE_VERIFICATION credential oluşturur. |
| Preconditions | ACTIVE; mevcut email verified; target normalize unique check (preliminary). |
| Input summary | New email. |
| Output summary | Generic/pending confirmation. |
| Reads | `hrd_users`, `hrd_one_time_credentials` |
| Writes | `hrd_one_time_credentials` |
| Transaction boundary | Invalidate previous EMAIL_CHANGE + create with target_email aynı TX. Commit sonrası email provider (ayrı durable email queue yok). |
| Concurrency | one-active |
| Idempotency | Rate-limited |
| Side effects | Email provider TX dışı; failure credential geçerli kalır |
| Primary domain errors | `EMAIL_NOT_VERIFIED`, `CONFLICT`, `RATE_LIMITED`, `VALIDATION_ERROR`, `DEPENDENCY_UNAVAILABLE` |
| Notes | Email profil update ile değişmez. |

#### `AUTH-14` — `ConfirmEmailChange`

| Alan | İçerik |
| --- | --- |
| ID | `AUTH-14` |
| Exact function name | `ConfirmEmailChange` |
| Domain | IAM |
| Exposure | `PUBLIC` |
| Actor | Anonymous (token) |
| Purpose | Email change credential consume + email update. |
| Preconditions | Valid EMAIL_CHANGE_VERIFICATION; target uniqueness re-check. |
| Input summary | Token. |
| Output summary | Success. |
| Reads | `hrd_one_time_credentials`, `hrd_users` |
| Writes | `hrd_one_time_credentials`, `hrd_users`, `hrd_security_events` |
| Transaction boundary | Consume + email/email_normalized update + AppendSecurityEvent(EMAIL_CHANGE) `REQUIRED_CALLER_TRANSACTION` aynı TX. Session/stamp etkisi açık ürün kararı. |
| Concurrency | unique email race → CONFLICT |
| Idempotency | Non-idempotent after consume |
| Side effects | EMAIL_CHANGE mandatory transactional event |
| Primary domain errors | `TOKEN_INVALID`, `TOKEN_EXPIRED`, `TOKEN_ALREADY_USED`, `CONFLICT`, `VALIDATION_ERROR` |
| Notes | Session revoke etkisi ürün/teknik açık karar. |

## 10. ACCOUNT

#### `ACCOUNT-01` — `GetMyProfile`

| Alan | İçerik |
| --- | --- |
| ID | `ACCOUNT-01` |
| Exact function name | `GetMyProfile` |
| Domain | IAM |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user |
| Purpose | Kendi profil bilgisini döndürür. |
| Preconditions | ACTIVE FE/MOBILE session. |
| Input summary | None. |
| Output summary | first_name, last_name, phone, email, email_verified, role (read-only), status. |
| Reads | `hrd_users` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `UNAUTHENTICATED`, `ACCOUNT_INACTIVE` |
| Notes | Security fields (password_hash, stamp) dönmez. |
#### `ACCOUNT-02` — `UpdateMyProfile`

| Alan | İçerik |
| --- | --- |
| ID | `ACCOUNT-02` |
| Exact function name | `UpdateMyProfile` |
| Domain | IAM |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user |
| Purpose | first_name/last_name/phone günceller. |
| Preconditions | ACTIVE. |
| Input summary | first_name, last_name, phone. |
| Output summary | Updated profile. |
| Reads | `hrd_users` |
| Writes | `hrd_users` |
| Transaction boundary | User row update TX. |
| Concurrency | — |
| Idempotency | Idempotent if same values |
| Side effects | None (role/status/email değişmez) |
| Primary domain errors | `VALIDATION_ERROR`, `UNAUTHENTICATED`, `ACCOUNT_INACTIVE` |
| Notes | Email/role/status bu fonksiyonla değişmez. |
## 11. ADMIN-USER (BO)

| ID | Name |
| --- | --- |
| ADMIN-USER-01..05 | ListUsers, GetUserAdminDetail, ChangeUserStatus, ChangeUserRole, ListUserSecurityEvents |

#### `ADMIN-USER-01` — `ListUsers`

| Alan | İçerik |
| --- | --- |
| ID | `ADMIN-USER-01` |
| Exact function name | `ListUsers` |
| Domain | IAM |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Kullanıcı listesini filtreler. |
| Preconditions | admin + ADMIN_BO. |
| Input summary | Status/role/email prefix filters; cursor. |
| Output summary | Admin user list projection. |
| Reads | `hrd_users` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `FORBIDDEN`, `UNAUTHENTICATED`, `VALIDATION_ERROR` |
| Notes | — |
#### `ADMIN-USER-02` — `GetUserAdminDetail`

| Alan | İçerik |
| --- | --- |
| ID | `ADMIN-USER-02` |
| Exact function name | `GetUserAdminDetail` |
| Domain | IAM |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Tek user admin detayı. |
| Preconditions | admin + ADMIN_BO. |
| Input summary | User id. |
| Output summary | Admin detail (hash yok). |
| Reads | `hrd_users`, `hrd_auth_sessions` (counts) |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `NOT_FOUND`, `FORBIDDEN` |
| Notes | Password set/view yok. |
#### `ADMIN-USER-03` — `ChangeUserStatus`

| Alan | İçerik |
| --- | --- |
| ID | `ADMIN-USER-03` |
| Exact function name | `ChangeUserStatus` |
| Domain | IAM |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | User status ACTIVE/DISABLED/CLOSED değiştirir. |
| Preconditions | admin+ADMIN_BO; target exists; exact status set. |
| Input summary | Target user ID; expected current status; new status. |
| Output summary | Updated status. |
| Reads | `hrd_users`, `hrd_auth_sessions` |
| Writes | `hrd_users`, `hrd_auth_sessions`, `hrd_security_events` |
| Transaction boundary | TX: target user row lock veya compare-and-set; current status ≠ expected → CONFLICT; status update; DISABLED/CLOSED ise bütün session revoke; AppendSecurityEvent(ACCOUNT_STATUS_CHANGE) `REQUIRED_CALLER_TRANSACTION`. |
| Concurrency | expected current status compare-and-set (hrd_users.version yok) |
| Idempotency | Idempotent if already same status → success |
| Side effects | ACCOUNT_STATUS_CHANGE mandatory; session revoke on DISABLED/CLOSED |
| Primary domain errors | `VALIDATION_ERROR`, `CONFLICT`, `INVALID_STATE`, `FORBIDDEN`, `NOT_FOUND` |
| Notes | Son admin kilitleme ürün kararı açık. Actor server’dan. Optimistic version kolonu yok. |

#### `ADMIN-USER-04` — `ChangeUserRole`

| Alan | İçerik |
| --- | --- |
| ID | `ADMIN-USER-04` |
| Exact function name | `ChangeUserRole` |
| Domain | IAM |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Role user/admin değiştirir. |
| Preconditions | admin+ADMIN_BO. |
| Input summary | Target user ID; expected current role; new role. |
| Output summary | Updated role. |
| Reads | `hrd_users`, `hrd_auth_sessions` |
| Writes | `hrd_users`, `hrd_auth_sessions`, `hrd_security_events` |
| Transaction boundary | TX: lock/compare-and-set current role; mismatch → CONFLICT; role update; bütün session revoke; AppendSecurityEvent(ROLE_CHANGE) `REQUIRED_CALLER_TRANSACTION`. |
| Concurrency | expected current role compare-and-set (hrd_users.version yok) |
| Idempotency | Conditional idempotent if already same role |
| Side effects | ROLE_CHANGE mandatory; full session revoke; re-login |
| Primary domain errors | `VALIDATION_ERROR`, `CONFLICT`, `FORBIDDEN`, `NOT_FOUND` |
| Notes | Son admin demote ürün kararı açık. Actor server’dan. |

#### `ADMIN-USER-05` — `ListUserSecurityEvents`

| Alan | İçerik |
| --- | --- |
| ID | `ADMIN-USER-05` |
| Exact function name | `ListUserSecurityEvents` |
| Domain | IAM |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | User subject/actor security event’lerini listeler. |
| Preconditions | admin+ADMIN_BO; target user exists. |
| Input summary | User id, cursor, type filter. |
| Output summary | Event list (metadata secret-free). |
| Reads | `hrd_users`, `hrd_security_events` |
| Writes | None |
| Transaction boundary | read-only: önce `hrd_users` existence; yoksa NOT_FOUND; varsa events (boş liste OK) |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `NOT_FOUND`, `FORBIDDEN` |
| Notes | Metadata secret içermez. User yok vs event yok ayrılır. |

## 12. GEO

| ID | Name | Exposure |
| --- | --- | --- |
| GEO-01 | ListActiveProvinces | PUBLIC |
| GEO-02 | SearchProvinces | PUBLIC |
| GEO-03 | ListDistrictsByProvince | PUBLIC |
| GEO-04 | SearchDistricts | PUBLIC |

#### `GEO-01` — `ListActiveProvinces`

| Alan | İçerik |
| --- | --- |
| ID | `GEO-01` |
| Exact function name | `ListActiveProvinces` |
| Domain | GEO |
| Exposure | `PUBLIC` |
| Actor | Anonymous |
| Purpose | Aktif illeri sıralı listeler. |
| Preconditions | Yok. |
| Input summary | None. |
| Output summary | Active provinces sorted. |
| Reads | `hrd_provinces` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | None |
| Notes | Runtime external location API yok. Mutation seed/migration-controlled. |
#### `GEO-02` — `SearchProvinces`

| Alan | İçerik |
| --- | --- |
| ID | `GEO-02` |
| Exact function name | `SearchProvinces` |
| Domain | GEO |
| Exposure | `PUBLIC` |
| Actor | Anonymous |
| Purpose | Normalize prefix ile il arar. |
| Preconditions | Yok. |
| Input summary | Prefix query; limit. |
| Output summary | Matching provinces. |
| Reads | `hrd_provinces` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `VALIDATION_ERROR` |
| Notes | Turkish normalized deterministic prefix. |
#### `GEO-03` — `ListDistrictsByProvince`

| Alan | İçerik |
| --- | --- |
| ID | `GEO-03` |
| Exact function name | `ListDistrictsByProvince` |
| Domain | GEO |
| Exposure | `PUBLIC` |
| Actor | Anonymous |
| Purpose | Province’e bağlı aktif ilçeleri listeler. |
| Preconditions | Province exists. |
| Input summary | Province id. |
| Output summary | Active districts sorted. |
| Reads | `hrd_provinces`, `hrd_districts` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `NOT_FOUND` |
| Notes | Province→district relation doğrulanır. Index: province_active_sort. |
#### `GEO-04` — `SearchDistricts`

| Alan | İçerik |
| --- | --- |
| ID | `GEO-04` |
| Exact function name | `SearchDistricts` |
| Domain | GEO |
| Exposure | `PUBLIC` |
| Actor | Anonymous |
| Purpose | Normalize prefix ile ilçe arar; province scope opsiyonel. |
| Preconditions | Opsiyonel province exists. |
| Input summary | Prefix; optional province_id. |
| Output summary | Matching districts. |
| Reads | `hrd_districts`, `hrd_provinces` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `VALIDATION_ERROR`, `NOT_FOUND` |
| Notes | — |
## 13. CATALOG (public)

#### `CATALOG-01` — `GetPublicCategoryTree`

| Alan | İçerik |
| --- | --- |
| ID | `CATALOG-01` |
| Exact function name | `GetPublicCategoryTree` |
| Domain | CATALOG |
| Exposure | `PUBLIC` |
| Actor | Anonymous |
| Purpose | Aktif kategori ağacını döndürür. |
| Preconditions | Yok. |
| Input summary | Optional root. |
| Output summary | Active tree nodes (id, slug, name, children). |
| Reads | `hrd_categories` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | None |
| Notes | Inheritance yok. |
#### `CATALOG-02` — `GetCategoryFormDefinition`

| Alan | İçerik |
| --- | --- |
| ID | `CATALOG-02` |
| Exact function name | `GetCategoryFormDefinition` |
| Domain | CATALOG |
| Exposure | `PUBLIC` |
| Actor | Anonymous |
| Purpose | Leaf kategori form/property metadata’sını döndürür. |
| Preconditions | Category active; leaf context preferred. |
| Input summary | Category id/slug. |
| Output summary | Property defs: code, title, help, order, required, visibility, filterable, options/default/safe validation/UI projection. |
| Reads | `hrd_categories`, `hrd_category_properties` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `NOT_FOUND`, `INVALID_STATE` |
| Notes | Internal validation metadata kontrolsüz dökülmez. Parent scope dynamic filter yok. |
## 14. ADMIN-CATALOG (BO)

| ID | Name |
| --- | --- |
| ADMIN-CATALOG-01 | ListCategoriesAdmin |
| ADMIN-CATALOG-02 | GetCategoryAdminDetail |
| ADMIN-CATALOG-03 | CreateCategory |
| ADMIN-CATALOG-04 | UpdateCategory |
| ADMIN-CATALOG-05 | ReparentCategory |
| ADMIN-CATALOG-06 | SetCategoryActive |
| ADMIN-CATALOG-07 | ReorderCategories |
| ADMIN-CATALOG-08 | ListCategoryPropertiesAdmin |
| ADMIN-CATALOG-09 | CreateCategoryProperty |
| ADMIN-CATALOG-10 | UpdateCategoryProperty |
| ADMIN-CATALOG-11 | SetCategoryPropertyActive |
| ADMIN-CATALOG-12 | ReorderCategoryProperties |

#### `ADMIN-CATALOG-01` — `ListCategoriesAdmin`

| Alan | İçerik |
| --- | --- |
| ID | `ADMIN-CATALOG-01` |
| Exact function name | `ListCategoriesAdmin` |
| Domain | CATALOG |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Kategori admin listesi (aktif/pasif). |
| Preconditions | admin+ADMIN_BO. |
| Input summary | Filters. |
| Output summary | Admin tree/list. |
| Reads | `hrd_categories` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `FORBIDDEN` |
| Notes | — |
#### `ADMIN-CATALOG-02` — `GetCategoryAdminDetail`

| Alan | İçerik |
| --- | --- |
| ID | `ADMIN-CATALOG-02` |
| Exact function name | `GetCategoryAdminDetail` |
| Domain | CATALOG |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Kategori detay + property özet. |
| Preconditions | admin+ADMIN_BO. |
| Input summary | Category id. |
| Output summary | Detail. |
| Reads | `hrd_categories`, `hrd_category_properties` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `NOT_FOUND` |
| Notes | — |
#### `ADMIN-CATALOG-03` — `CreateCategory`

| Alan | İçerik |
| --- | --- |
| ID | `ADMIN-CATALOG-03` |
| Exact function name | `CreateCategory` |
| Domain | CATALOG |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Yeni kategori oluşturur. |
| Preconditions | admin+ADMIN_BO; parent exists or null; slug unique; no self-parent. |
| Input summary | Parent, slug, name, description, sort. |
| Output summary | Created category + version. |
| Reads | `hrd_categories` |
| Writes | `hrd_categories` |
| Transaction boundary | Insert TX. |
| Concurrency | slug unique |
| Idempotency | Non-idempotent |
| Side effects | None |
| Primary domain errors | `CONFLICT`, `VALIDATION_ERROR` |
| Notes | Hard delete yok. |
#### `ADMIN-CATALOG-04` — `UpdateCategory`

| Alan | İçerik |
| --- | --- |
| ID | `ADMIN-CATALOG-04` |
| Exact function name | `UpdateCategory` |
| Domain | CATALOG |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Kategori alanlarını günceller (parent hariç). |
| Preconditions | admin+ADMIN_BO; expected version. |
| Input summary | Expected version; name/description/sort/slug policy. |
| Output summary | Updated. |
| Reads | `hrd_categories` |
| Writes | `hrd_categories` |
| Transaction boundary | Update where version match; version++. |
| Concurrency | `version` → STALE_VERSION |
| Idempotency | Conditional |
| Side effects | None |
| Primary domain errors | `STALE_VERSION`, `CONFLICT` |
| Notes | Reparent ayrı fonksiyon. |
#### `ADMIN-CATALOG-05` — `ReparentCategory`

| Alan | İçerik |
| --- | --- |
| ID | `ADMIN-CATALOG-05` |
| Exact function name | `ReparentCategory` |
| Domain | CATALOG |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Kategoriyi yeni parent altına taşır. |
| Preconditions | admin+ADMIN_BO; expected version; no self-parent; no cycle. |
| Input summary | Category id, new parent, expected version. |
| Output summary | Updated parent. |
| Reads | `hrd_categories` |
| Writes | `hrd_categories` |
| Transaction boundary | Cycle check + update same TX; advert FK değişmez. |
| Concurrency | `version` |
| Idempotency | Non-idempotent if same parent → idempotent |
| Side effects | None |
| Primary domain errors | `INVALID_STATE`, `STALE_VERSION`, `VALIDATION_ERROR` |
| Notes | Uzun cycle app TX. Deactivation/reparent advert status değiştirmez. |
#### `ADMIN-CATALOG-06` — `SetCategoryActive`

| Alan | İçerik |
| --- | --- |
| ID | `ADMIN-CATALOG-06` |
| Exact function name | `SetCategoryActive` |
| Domain | CATALOG |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | is_active set eder. |
| Preconditions | admin+ADMIN_BO; expected version. |
| Input summary | is_active, expected version. |
| Output summary | Updated. |
| Reads | `hrd_categories` |
| Writes | `hrd_categories` |
| Transaction boundary | Update+version++. |
| Concurrency | `version` |
| Idempotency | Idempotent if same |
| Side effects | None |
| Primary domain errors | `STALE_VERSION` |
| Notes | Published advert otomatik değişmez. |
#### `ADMIN-CATALOG-07` — `ReorderCategories`

| Alan | İçerik |
| --- | --- |
| ID | `ADMIN-CATALOG-07` |
| Exact function name | `ReorderCategories` |
| Domain | CATALOG |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Aynı parent altında sort_order günceller. |
| Preconditions | admin+ADMIN_BO. |
| Input summary | Ordered category ids + versions. |
| Output summary | New order. |
| Reads | `hrd_categories` |
| Writes | `hrd_categories` |
| Transaction boundary | Collision-safe multi-row TX. |
| Concurrency | versions |
| Idempotency | Conditional |
| Side effects | None |
| Primary domain errors | `STALE_VERSION`, `VALIDATION_ERROR` |
| Notes | — |
#### `ADMIN-CATALOG-08` — `ListCategoryPropertiesAdmin`

| Alan | İçerik |
| --- | --- |
| ID | `ADMIN-CATALOG-08` |
| Exact function name | `ListCategoryPropertiesAdmin` |
| Domain | CATALOG |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Category property admin listesi. |
| Preconditions | admin+ADMIN_BO; category exists. |
| Input summary | Category id. |
| Output summary | Properties (empty list OK if category exists). |
| Reads | `hrd_categories`, `hrd_category_properties` |
| Writes | None |
| Transaction boundary | read-only: category existence önce; yoksa NOT_FOUND |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `NOT_FOUND`, `FORBIDDEN` |
| Notes | Kategori yok ile kategori var/property yok ayrılır. |

#### `ADMIN-CATALOG-09` — `CreateCategoryProperty`

| Alan | İçerik |
| --- | --- |
| ID | `ADMIN-CATALOG-09` |
| Exact function name | `CreateCategoryProperty` |
| Domain | CATALOG |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Property definition oluşturur. |
| Preconditions | admin+ADMIN_BO; category exists; code unique in category; data_type set. |
| Input summary | Property fields + options/validation/default/ui. |
| Output summary | Created property. |
| Reads | `hrd_categories`, `hrd_category_properties` |
| Writes | `hrd_category_properties` |
| Transaction boundary | Insert TX. |
| Concurrency | category+code unique |
| Idempotency | Non-idempotent |
| Side effects | None |
| Primary domain errors | `CONFLICT`, `VALIDATION_ERROR` |
| Notes | Inheritance yok. Values advert JSONB’de. |
#### `ADMIN-CATALOG-10` — `UpdateCategoryProperty`

| Alan | İçerik |
| --- | --- |
| ID | `ADMIN-CATALOG-10` |
| Exact function name | `UpdateCategoryProperty` |
| Domain | CATALOG |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Property metadata günceller; code kullanıldıysa immutable. |
| Preconditions | admin+ADMIN_BO; expected version; code immutability rule. |
| Input summary | Expected version; mutable fields. |
| Output summary | Updated. |
| Reads | `hrd_category_properties`, `hrd_adverts` (usage check) |
| Writes | `hrd_category_properties` |
| Transaction boundary | Usage check + update+version++. |
| Concurrency | `version` |
| Idempotency | Conditional |
| Side effects | None |
| Primary domain errors | `STALE_VERSION`, `INVALID_STATE`, `VALIDATION_ERROR` |
| Notes | Historical advert JSONB silinmez. Advert values değiştirilmez. |
#### `ADMIN-CATALOG-11` — `SetCategoryPropertyActive`

| Alan | İçerik |
| --- | --- |
| ID | `ADMIN-CATALOG-11` |
| Exact function name | `SetCategoryPropertyActive` |
| Domain | CATALOG |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Property is_active. |
| Preconditions | admin+ADMIN_BO; expected version. |
| Input summary | is_active, version. |
| Output summary | Updated. |
| Reads | `hrd_category_properties` |
| Writes | `hrd_category_properties` |
| Transaction boundary | Update+version++. |
| Concurrency | `version` |
| Idempotency | Idempotent if same |
| Side effects | None |
| Primary domain errors | `STALE_VERSION` |
| Notes | Deactivation historical values silmez. |
#### `ADMIN-CATALOG-12` — `ReorderCategoryProperties`

| Alan | İçerik |
| --- | --- |
| ID | `ADMIN-CATALOG-12` |
| Exact function name | `ReorderCategoryProperties` |
| Domain | CATALOG |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Property sort_order. |
| Preconditions | admin+ADMIN_BO. |
| Input summary | Ordered property ids + versions. |
| Output summary | New order. |
| Reads | `hrd_category_properties` |
| Writes | `hrd_category_properties` |
| Transaction boundary | Multi-row TX. |
| Concurrency | versions |
| Idempotency | Conditional |
| Side effects | None |
| Primary domain errors | `STALE_VERSION` |
| Notes | — |
## 15. HORSE

#### `HORSE-01` — `SearchHorsesForSelection`

| Alan | İçerik |
| --- | --- |
| ID | `HORSE-01` |
| Exact function name | `SearchHorsesForSelection` |
| Domain | HORSE |
| Exposure | `PUBLIC` |
| Actor | Anonymous |
| Purpose | Form seçimi için local horse arar. |
| Preconditions | Yok. |
| Input summary | Normalized name prefix and/or tjk_number; limit. |
| Output summary | Safe selection list: id, original_name, tjk_number, distinguishing public fields. |
| Reads | `hrd_horses` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `VALIDATION_ERROR` |
| Notes | Canlı TJK yok; name identity değil; same-name ayrıştırılır; raw payload yok. |
#### `HORSE-02` — `GetHorsePublicDetail`

| Alan | İçerik |
| --- | --- |
| ID | `HORSE-02` |
| Exact function name | `GetHorsePublicDetail` |
| Domain | HORSE |
| Exposure | `PUBLIC` |
| Actor | Anonymous |
| Purpose | Public horse detayı. |
| Preconditions | Horse exists. |
| Input summary | Horse id. |
| Output summary | Typed public core + allowed controlled detail sections. |
| Reads | `hrd_horses` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `NOT_FOUND` |
| Notes | Raw payload/sync ops alanları public değil. Manual horse update fonksiyonu yok. |
## 16. ADVERT-PUBLIC

#### `ADVERT-PUBLIC-01` — `SearchPublishedAdverts`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-PUBLIC-01` |
| Exact function name | `SearchPublishedAdverts` |
| Domain | ADVERT |
| Exposure | `PUBLIC` |
| Actor | Anonymous; optional ACTIVE for favorite enrichment |
| Purpose | PUBLISHED ilanları filtreleyip cursor ile listeler. |
| Preconditions | Filter whitelist; parent browse yalnız fixed filters; dynamic filters yalnız tek leaf. |
| Input summary | Category scope, province/district, horse_id, photo flag, property filters (leaf), sort, opaque cursor, fingerprint. |
| Output summary | Search projection cards; optional is_favorite; next cursor; exact total zorunlu değil. |
| Reads | `hrd_adverts`, `hrd_districts`, `hrd_provinces`, `hrd_categories`, `hrd_category_properties`, `hrd_horses`, `hrd_advert_media`, `hrd_media_assets`, `hrd_media_variants`; optional `hrd_favorites` |
| Writes | None |
| Transaction boundary | read-only; N+1 yok |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `VALIDATION_ERROR`, `QUERY_TOO_COMPLEX` |
| Notes | Status PUBLISHED query-time; READY variant foto; province via district; raw JSONPath/arbitrary sort yok. ResolvePublicAdvertProjection kullanılır. |
#### `ADVERT-PUBLIC-02` — `GetPublishedAdvertDetail`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-PUBLIC-02` |
| Exact function name | `GetPublishedAdvertDetail` |
| Domain | ADVERT |
| Exposure | `PUBLIC` |
| Actor | Anonymous; optional ACTIVE favorite |
| Purpose | PUBLISHED ilan detay projection. |
| Preconditions | Advert status must be PUBLISHED at read time. |
| Input summary | Advert id. |
| Output summary | Detail projection: public fields, READY media, public properties (definition-mapped), horse, location; optional is_favorite. |
| Reads | `hrd_adverts`, `hrd_districts`, `hrd_provinces`, `hrd_categories`, `hrd_category_properties`, `hrd_horses`, `hrd_advert_media`, `hrd_media_assets`, `hrd_media_variants`; optional `hrd_favorites` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `NOT_FOUND` |
| Notes | Moderation history/internal keys/raw payload yok. Dynamic property code’ları yalnız public-visible definitions ile title/option’a çevrilir. ResolvePublicAdvertProjection kullanılır. |

## 17. ADVERT-OWNER

| ID | Name | From→To / notes |
| --- | --- | --- |
| ADVERT-OWNER-01 | CreateAdvertDraft | → DRAFT |
| ADVERT-OWNER-02 | ListMyAdverts | read |
| ADVERT-OWNER-03 | GetMyAdvert | read |
| ADVERT-OWNER-04 | UpdateAdvertDraftDetails | DRAFT/CHANGES_REQUESTED |
| ADVERT-OWNER-05 | ChangeAdvertDraftCategory | DRAFT only |
| ADVERT-OWNER-06 | ReplaceAdvertDynamicProperties | DRAFT/CHANGES_REQUESTED |
| ADVERT-OWNER-07 | SubmitAdvertForReview | DRAFT→PENDING_REVIEW |
| ADVERT-OWNER-08 | ResubmitAdvertForReview | CHANGES_REQUESTED→PENDING_REVIEW |
| ADVERT-OWNER-09 | SoftDeleteAdvertDraft | DRAFT soft-delete |
| ADVERT-OWNER-10 | MarkAdvertSold | PUBLISHED→SOLD |
| ADVERT-OWNER-11 | ArchiveAdvert | PUBLISHED→ARCHIVED |

#### `ADVERT-OWNER-01` — `CreateAdvertDraft`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-OWNER-01` |
| Exact function name | `CreateAdvertDraft` |
| Domain | ADVERT |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user (owner) |
| Purpose | Minimal/partial DRAFT oluşturur. |
| Preconditions | ACTIVE user; email-verify draft create ürün kararı açık (öneri: ACTIVE yeterli). |
| Input summary | Opsiyonel partial fields; owner/status client’tan alınmaz. |
| Output summary | Advert id, version, status=DRAFT. |
| Reads | `hrd_users` |
| Writes | `hrd_adverts`, `hrd_advert_status_history` |
| Transaction boundary | Aynı TX: (1) `hrd_adverts` DRAFT insert; (2) `hrd_advert_status_history` with `from_status=NULL`, `to_status=DRAFT`, actor=owner, `is_system=false`. Initial DRAFT history için `TransitionAdvertStatus` kullanılmaz. |
| Concurrency | `version` init |
| Idempotency | Non-idempotent |
| Side effects | Initial status history row (direct insert) |
| Primary domain errors | `UNAUTHENTICATED`, `ACCOUNT_INACTIVE`, `VALIDATION_ERROR` |
| Notes | Owner = session user. INTERNAL-02 yalnız mevcut non-null status transition’larında. |

#### `ADVERT-OWNER-02` — `ListMyAdverts`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-OWNER-02` |
| Exact function name | `ListMyAdverts` |
| Domain | ADVERT |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user (owner) |
| Purpose | Sahibin ilanlarını listeler. |
| Preconditions | ACTIVE; ownership = self. |
| Input summary | Status filter, cursor. |
| Output summary | Owner list projection. |
| Reads | `hrd_adverts` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `UNAUTHENTICATED`, `ACCOUNT_INACTIVE` |
| Notes | Soft-deleted drafts policy: excluded by default. |
#### `ADVERT-OWNER-03` — `GetMyAdvert`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-OWNER-03` |
| Exact function name | `GetMyAdvert` |
| Domain | ADVERT |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user (owner) |
| Purpose | Sahip ilan detayı (owner projection). |
| Preconditions | ACTIVE; owner. |
| Input summary | Advert id. |
| Output summary | Full owner projection including status, properties, media relation. |
| Reads | `hrd_adverts`, `hrd_categories`, `hrd_category_properties`, `hrd_districts`, `hrd_provinces`, `hrd_horses`, `hrd_advert_media`, `hrd_media_assets`, `hrd_media_variants`, `hrd_advert_status_history` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `NOT_FOUND`, `OWNERSHIP_REQUIRED` |
| Notes | Admin bypass yok. |

#### `ADVERT-OWNER-04` — `UpdateAdvertDraftDetails`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-OWNER-04` |
| Exact function name | `UpdateAdvertDraftDetails` |
| Domain | ADVERT |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user (owner) |
| Purpose | Draft/changes core alanları günceller (category hariç). |
| Preconditions | ACTIVE; owner; status DRAFT|CHANGES_REQUESTED; expected version; not deleted. |
| Input summary | Expected version; district_id; horse_id; title; description; price pair. |
| Output summary | Updated advert + version. |
| Reads | `hrd_adverts`, `hrd_districts`, `hrd_horses` |
| Writes | `hrd_adverts` |
| Transaction boundary | Update where version; version++. |
| Concurrency | `version` → STALE_VERSION |
| Idempotency | Conditional |
| Side effects | None (no status history) |
| Primary domain errors | `INVALID_STATE`, `STALE_VERSION`, `OWNERSHIP_REQUIRED`, `VALIDATION_ERROR` |
| Notes | Category bu fonksiyonla değişmez. Required checks submit’te. |
#### `ADVERT-OWNER-05` — `ChangeAdvertDraftCategory`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-OWNER-05` |
| Exact function name | `ChangeAdvertDraftCategory` |
| Domain | ADVERT |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user (owner) |
| Purpose | Yalnız DRAFT’ta category değiştirir ve properties temizler. |
| Preconditions | ACTIVE; owner; status=DRAFT; expected version. |
| Input summary | New category_id (leaf preferred), expected version. |
| Output summary | Updated category; properties={}; warning flag. |
| Reads | `hrd_adverts`, `hrd_categories` |
| Writes | `hrd_adverts` |
| Transaction boundary | Category set + properties clear + version++ aynı TX. |
| Concurrency | `version` |
| Idempotency | Non-idempotent if same category still clears? If same category: no-op or still clear — exact: same id → no property clear, idempotent |
| Side effects | None |
| Primary domain errors | `INVALID_STATE`, `STALE_VERSION`, `VALIDATION_ERROR` |
| Notes | CHANGES_REQUESTED’ta category değişmez. |
#### `ADVERT-OWNER-06` — `ReplaceAdvertDynamicProperties`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-OWNER-06` |
| Exact function name | `ReplaceAdvertDynamicProperties` |
| Domain | ADVERT |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user (owner) |
| Purpose | Dynamic properties’i leaf metadata’ya göre replace eder. |
| Preconditions | ACTIVE; owner; DRAFT|CHANGES_REQUESTED; expected version; category set. |
| Input summary | Expected version; properties map by code. |
| Output summary | Saved properties (partial OK). |
| Reads | `hrd_adverts`, `hrd_category_properties` |
| Writes | `hrd_adverts` |
| Transaction boundary | ValidateDynamicProperties (draft mode) + replace + version++. |
| Concurrency | `version` |
| Idempotency | Conditional |
| Side effects | None |
| Primary domain errors | `VALIDATION_ERROR`, `INVALID_STATE`, `STALE_VERSION` |
| Notes | Arbitrary keys reddedilir. Required submit’te. |
#### `ADVERT-OWNER-07` — `SubmitAdvertForReview`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-OWNER-07` |
| Exact function name | `SubmitAdvertForReview` |
| Domain | ADVERT |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user (owner) |
| Purpose | DRAFT→PENDING_REVIEW full validation ile. |
| Preconditions | ACTIVE; owner; email_verified; status=DRAFT; expected version. |
| Input summary | Expected version. |
| Output summary | Status PENDING_REVIEW + version. |
| Reads | `hrd_adverts`, `hrd_categories`, `hrd_category_properties`, `hrd_districts`, `hrd_horses`, `hrd_advert_media`, `hrd_media_assets`, `hrd_media_variants`, `hrd_users` |
| Writes | `hrd_adverts`, `hrd_advert_status_history` |
| Transaction boundary | ValidateAdvertForSubmission + TransitionAdvertStatus aynı TX. |
| Concurrency | `version` |
| Idempotency | Non-idempotent |
| Side effects | History; no payment wait |
| Primary domain errors | `EMAIL_NOT_VERIFIED`, `VALIDATION_ERROR`, `PROCESSING_NOT_READY`, `INVALID_STATE`, `STALE_VERSION` |
| Notes | Horse/min media/price required category metadata + açık ürün kararlarına bağlı. |
#### `ADVERT-OWNER-08` — `ResubmitAdvertForReview`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-OWNER-08` |
| Exact function name | `ResubmitAdvertForReview` |
| Domain | ADVERT |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user (owner) |
| Purpose | CHANGES_REQUESTED→PENDING_REVIEW. |
| Preconditions | ACTIVE; owner; email_verified; status=CHANGES_REQUESTED; expected version. |
| Input summary | Expected version. |
| Output summary | Status PENDING_REVIEW. |
| Reads | `hrd_users`, `hrd_adverts`, `hrd_categories`, `hrd_category_properties`, `hrd_districts`, `hrd_horses`, `hrd_advert_media`, `hrd_media_assets`, `hrd_media_variants` |
| Writes | `hrd_adverts`, `hrd_advert_status_history` |
| Transaction boundary | ValidateAdvertForSubmission + TransitionAdvertStatus aynı TX. |
| Concurrency | `version` |
| Idempotency | Non-idempotent |
| Side effects | Status history via INTERNAL-02 |
| Primary domain errors | `EMAIL_NOT_VERIFIED`, `VALIDATION_ERROR`, `PROCESSING_NOT_READY`, `INVALID_STATE`, `STALE_VERSION`, `OWNERSHIP_REQUIRED` |
| Notes | Category change hâlâ yasak. |

#### `ADVERT-OWNER-09` — `SoftDeleteAdvertDraft`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-OWNER-09` |
| Exact function name | `SoftDeleteAdvertDraft` |
| Domain | ADVERT |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user (owner) |
| Purpose | DRAFT soft-delete (`deleted_at`). |
| Preconditions | ACTIVE; owner; status=DRAFT; expected version. |
| Input summary | Expected version. |
| Output summary | Soft-deleted marker. |
| Reads | `hrd_adverts` |
| Writes | `hrd_adverts` |
| Transaction boundary | Set `deleted_at` + version++ where DRAFT. Status history yazılmaz (status transition değildir). |
| Concurrency | `version` |
| Idempotency | Idempotent if already deleted → success |
| Side effects | None |
| Primary domain errors | `INVALID_STATE`, `STALE_VERSION`, `OWNERSHIP_REQUIRED` |
| Notes | Hard delete yok. Restore phase-one dışı/açık. |

#### `ADVERT-OWNER-10` — `MarkAdvertSold`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-OWNER-10` |
| Exact function name | `MarkAdvertSold` |
| Domain | ADVERT |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user (owner) |
| Purpose | PUBLISHED→SOLD. |
| Preconditions | ACTIVE; owner; status=PUBLISHED; expected version. |
| Input summary | Expected version. |
| Output summary | Status SOLD. |
| Reads | `hrd_adverts` |
| Writes | `hrd_adverts`, `hrd_advert_status_history` |
| Transaction boundary | TransitionAdvertStatus TX. |
| Concurrency | `version` |
| Idempotency | Non-idempotent |
| Side effects | History |
| Primary domain errors | `INVALID_STATE`, `STALE_VERSION`, `OWNERSHIP_REQUIRED` |
| Notes | Finansal kayıt değil; phase-one public değil. |
#### `ADVERT-OWNER-11` — `ArchiveAdvert`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-OWNER-11` |
| Exact function name | `ArchiveAdvert` |
| Domain | ADVERT |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user (owner) |
| Purpose | PUBLISHED→ARCHIVED. |
| Preconditions | ACTIVE; owner; status=PUBLISHED; expected version. |
| Input summary | Expected version. |
| Output summary | Status ARCHIVED. |
| Reads | `hrd_adverts` |
| Writes | `hrd_adverts`, `hrd_advert_status_history` |
| Transaction boundary | TransitionAdvertStatus TX. |
| Concurrency | `version` |
| Idempotency | Non-idempotent |
| Side effects | History |
| Primary domain errors | `INVALID_STATE`, `STALE_VERSION` |
| Notes | SUSPENDED→ARCHIVED kabul edilmedi; açık karar. |
## 18. ADVERT-ADMIN (BO moderation)

Accepted transitions only: PENDING_REVIEW→PUBLISHED / CHANGES_REQUESTED / REJECTED; PUBLISHED→SUSPENDED. Resume/unsuspend phase-one fonksiyon listesinde yok (açık karar; docs/08’de geçse bile bu blueprint’te Resume eklenmedi — ürün onayı sonrası).

#### `ADVERT-ADMIN-01` — `ListAdvertModerationQueue`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-ADMIN-01` |
| Exact function name | `ListAdvertModerationQueue` |
| Domain | ADVERT |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Moderasyon kuyruğu. |
| Preconditions | admin+ADMIN_BO. |
| Input summary | Status filter default PENDING_REVIEW; cursor. |
| Output summary | Queue projection. |
| Reads | `hrd_adverts`, `hrd_users`, `hrd_categories`, `hrd_districts` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `FORBIDDEN` |
| Notes | — |

#### `ADVERT-ADMIN-02` — `GetAdvertModerationDetail`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-ADMIN-02` |
| Exact function name | `GetAdvertModerationDetail` |
| Domain | ADVERT |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Moderasyon detayı. |
| Preconditions | admin+ADMIN_BO. |
| Input summary | Advert id. |
| Output summary | Full moderation projection + history. |
| Reads | `hrd_adverts`, `hrd_advert_status_history`, `hrd_users`, `hrd_categories`, `hrd_category_properties`, `hrd_districts`, `hrd_provinces`, `hrd_horses`, `hrd_advert_media`, `hrd_media_assets`, `hrd_media_variants` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `NOT_FOUND`, `FORBIDDEN` |
| Notes | Public’e sızmaz. Admin owner değildir. |

#### `ADVERT-ADMIN-03` — `ApproveAdvert`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-ADMIN-03` |
| Exact function name | `ApproveAdvert` |
| Domain | ADVERT |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | PENDING_REVIEW→PUBLISHED; published_at set. |
| Preconditions | admin+ADMIN_BO; expected status+version; preconditions still valid. |
| Input summary | Expected version; optional note. |
| Output summary | PUBLISHED. |
| Reads | `hrd_users`, `hrd_adverts`, `hrd_categories`, `hrd_category_properties`, `hrd_districts`, `hrd_horses`, `hrd_advert_media`, `hrd_media_assets`, `hrd_media_variants` |
| Writes | `hrd_adverts`, `hrd_advert_status_history` |
| Transaction boundary | ValidateAdvertForSubmission (moderation path) + TransitionAdvertStatus + published_at aynı TX. |
| Concurrency | `version` |
| Idempotency | Non-idempotent |
| Side effects | History; actor=admin user |
| Primary domain errors | `INVALID_STATE`, `STALE_VERSION`, `VALIDATION_ERROR`, `PROCESSING_NOT_READY`, `FORBIDDEN` |
| Notes | Payment beklemez. Canonical content edit yok. |

#### `ADVERT-ADMIN-04` — `RequestAdvertChanges`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-ADMIN-04` |
| Exact function name | `RequestAdvertChanges` |
| Domain | ADVERT |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | PENDING_REVIEW→CHANGES_REQUESTED; reason required. |
| Preconditions | admin+ADMIN_BO; expected version. |
| Input summary | Expected version; reason. |
| Output summary | CHANGES_REQUESTED. |
| Reads | `hrd_adverts` |
| Writes | `hrd_adverts`, `hrd_advert_status_history` |
| Transaction boundary | Transition + reason history TX. |
| Concurrency | `version` |
| Idempotency | Non-idempotent |
| Side effects | History |
| Primary domain errors | `VALIDATION_ERROR`, `INVALID_STATE`, `STALE_VERSION` |
| Notes | Reason user-visible. |
#### `ADVERT-ADMIN-05` — `RejectAdvert`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-ADMIN-05` |
| Exact function name | `RejectAdvert` |
| Domain | ADVERT |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | PENDING_REVIEW→REJECTED; reason required. |
| Preconditions | admin+ADMIN_BO; expected version. |
| Input summary | Expected version; reason. |
| Output summary | REJECTED. |
| Reads | `hrd_adverts` |
| Writes | `hrd_adverts`, `hrd_advert_status_history` |
| Transaction boundary | Transition TX. |
| Concurrency | `version` |
| Idempotency | Non-idempotent |
| Side effects | History |
| Primary domain errors | `VALIDATION_ERROR`, `INVALID_STATE`, `STALE_VERSION` |
| Notes | Re-apply açık ürün kararı. |
#### `ADVERT-ADMIN-06` — `SuspendAdvert`

| Alan | İçerik |
| --- | --- |
| ID | `ADVERT-ADMIN-06` |
| Exact function name | `SuspendAdvert` |
| Domain | ADVERT |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | PUBLISHED→SUSPENDED; reason required. |
| Preconditions | admin+ADMIN_BO; expected version. |
| Input summary | Expected version; reason. |
| Output summary | SUSPENDED. |
| Reads | `hrd_adverts` |
| Writes | `hrd_adverts`, `hrd_advert_status_history` |
| Transaction boundary | Transition TX. |
| Concurrency | `version` |
| Idempotency | Non-idempotent |
| Side effects | History |
| Primary domain errors | `INVALID_STATE`, `STALE_VERSION`, `VALIDATION_ERROR` |
| Notes | Resume fonksiyonu bu listede yok. |
## 19. FAVORITE

#### `FAVORITE-01` — `AddFavorite`

| Alan | İçerik |
| --- | --- |
| ID | `FAVORITE-01` |
| Exact function name | `AddFavorite` |
| Domain | ADVERT |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user |
| Purpose | PUBLISHED advert’e favorite relation ekler. |
| Preconditions | ACTIVE; advert status=PUBLISHED deleted_at null. |
| Input summary | Advert id. |
| Output summary | Favorite exists confirmation. |
| Reads | `hrd_adverts`, `hrd_favorites` |
| Writes | `hrd_favorites` |
| Transaction boundary | Insert; unique violation → success (idempotent). |
| Concurrency | unique (user,advert) |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `INVALID_STATE`, `NOT_FOUND`, `UNAUTHENTICATED` |
| Notes | Own-advert favorite açık ürün kararı. Admin bypass yok. |
#### `FAVORITE-02` — `RemoveFavorite`

| Alan | İçerik |
| --- | --- |
| ID | `FAVORITE-02` |
| Exact function name | `RemoveFavorite` |
| Domain | ADVERT |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user |
| Purpose | Favorite relation siler. |
| Preconditions | ACTIVE; own relation. |
| Input summary | Advert id. |
| Output summary | Removed confirmation. |
| Reads | `hrd_favorites` |
| Writes | `hrd_favorites` |
| Transaction boundary | Delete; missing → success. |
| Concurrency | — |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `UNAUTHENTICATED` |
| Notes | Advert unpublish existing relation silmez (bu fonksiyon kullanıcı isteği). |
#### `FAVORITE-03` — `ListMyFavorites`

| Alan | İçerik |
| --- | --- |
| ID | `FAVORITE-03` |
| Exact function name | `ListMyFavorites` |
| Domain | ADVERT |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user |
| Purpose | Favorileri listeler; non-public için safe placeholder. |
| Preconditions | ACTIVE. |
| Input summary | Cursor. |
| Output summary | Items: public card (batch projection) veya güvenli placeholder (moderation reason yok; eski içerik yok; internal media/object yok). |
| Reads | `hrd_favorites`, `hrd_adverts`, `hrd_categories`, `hrd_districts`, `hrd_provinces`, `hrd_horses`, `hrd_advert_media`, `hrd_media_assets`, `hrd_media_variants` |
| Writes | None |
| Transaction boundary | read-only; batch enrichment; N+1 yok |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `UNAUTHENTICATED`, `ACCOUNT_INACTIVE` |
| Notes | Public favorite count phase-one zorunlu değil. Public kalan item’lar public card projection ile; public olmayan yalnız placeholder. |

## 20. MEDIA (user + admin upload)

User advert media: `MEDIA-01`–`MEDIA-07`. Banner asset upload ayrı BO fonksiyonları: `MEDIA-ADMIN-01`–`MEDIA-ADMIN-03`. Banner relation fonksiyonları BANNER domainindedir.

#### `MEDIA-01` — `InitiateMediaUpload`

| Alan | İçerik |
| --- | --- |
| ID | `MEDIA-01` |
| Exact function name | `InitiateMediaUpload` |
| Domain | MEDIA |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user |
| Purpose | UPLOAD_PENDING asset + short-lived direct upload authorization oluşturur. |
| Preconditions | ACTIVE. |
| Input summary | Declared intent (size/MIME hints non-canonical). Client idempotency key yok. |
| Output summary | Asset id; upload authorization (no permanent B2 credentials). |
| Reads | `hrd_users` |
| Writes | `hrd_media_assets` |
| Transaction boundary | Asset insert TX. B2 short-lived authorization TX dışı; başarısızsa asset UPLOAD_PENDING kalabilir; client yeniden initiation yapabilir; abandoned cleanup. Ayrı durable queue/idempotency key yok. |
| Concurrency | — |
| Idempotency | Non-idempotent: iki çağrı → iki UPLOAD_PENDING; rate limiting; retention+cleanup |
| Side effects | None |
| Primary domain errors | `VALIDATION_ERROR`, `RATE_LIMITED`, `DEPENDENCY_UNAVAILABLE` |
| Notes | Object key backend unique. Persist idempotency key yok. |

#### `MEDIA-02` — `ConfirmMediaUpload`

| Alan | İçerik |
| --- | --- |
| ID | `MEDIA-02` |
| Exact function name | `ConfirmMediaUpload` |
| Domain | MEDIA |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE user (asset owner) |
| Purpose | Provider object doğrular; UPLOADED; validate job enqueue. |
| Preconditions | ACTIVE; asset owner; expected lifecycle UPLOAD_PENDING (veya zaten ileri lifecycle için idempotent). |
| Input summary | Asset id. |
| Output summary | Lifecycle status. |
| Reads | `hrd_media_assets`, `hrd_background_jobs` |
| Writes | `hrd_media_assets`, `hrd_background_jobs` |
| Transaction boundary | 1) B2/provider HEAD doğrulaması DB TX dışında. 2) Başarılıysa tek kısa TX: expected lifecycle doğrula; asset→UPLOADED; `hrd_background_jobs` içine `MEDIA_VALIDATE_AND_NORMALIZE` insert (dedup key = job_type+asset_id deterministik). Asset UPLOADED olup job’suz bırakılmaz. Ayrı durable email/media queue tablosu yok. |
| Concurrency | job deduplication_key partial unique |
| Idempotency | Idempotent: tekrar completion mevcut job bulur veya dedup conflict → success; duplicate job yok |
| Side effects | Durable validate job enqueue (same TX as UPLOADED) |
| Primary domain errors | `OWNERSHIP_REQUIRED`, `INVALID_STATE`, `DEPENDENCY_UNAVAILABLE`, `CONFLICT` |
| Notes | Client MIME/size canonical değil. |

#### `MEDIA-03` — `GetMediaProcessingStatus`

| Alan | İçerik |
| --- | --- |
| ID | `MEDIA-03` |
| Exact function name | `GetMediaProcessingStatus` |
| Domain | MEDIA |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE asset owner |
| Purpose | Asset/variant readiness güvenli projection. |
| Preconditions | ACTIVE; owner. |
| Input summary | Asset id. |
| Output summary | Lifecycle + variant statuses; no raw/master object keys. |
| Reads | `hrd_media_assets`, `hrd_media_variants` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `NOT_FOUND`, `OWNERSHIP_REQUIRED` |
| Notes | — |
#### `MEDIA-04` — `AttachMediaToAdvert`

| Alan | İçerik |
| --- | --- |
| ID | `MEDIA-04` |
| Exact function name | `AttachMediaToAdvert` |
| Domain | MEDIA |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE advert owner |
| Purpose | Asset’i advert’e bağlar. |
| Preconditions | ACTIVE; advert owner; DRAFT|CHANGES_REQUESTED; asset owner; expected media_version; asset lifecycle UPLOAD_PENDING|UPLOADED|VALIDATING|MASTER_READY (VALIDATION_FAILED|CLEANUP_CANDIDATE|DELETING|PHYSICALLY_DELETED reddedilir). |
| Input summary | Advert id, asset id, display_order, expected media_version. |
| Output summary | Relation + new media_version. |
| Reads | `hrd_adverts`, `hrd_media_assets`, `hrd_advert_media` |
| Writes | `hrd_advert_media`, `hrd_adverts` |
| Transaction boundary | Aynı TX: asset row lock; lifecycle recheck; relation insert + media_version++. Processing asset bağlanabilir; submit validation READY ister. |
| Concurrency | `media_version`; unique advert+asset; asset lock vs DeleteMediaObjects |
| Idempotency | Non-idempotent duplicate → CONFLICT |
| Side effects | None |
| Primary domain errors | `STALE_VERSION`, `INVALID_STATE`, `OWNERSHIP_REQUIRED`, `CONFLICT` |
| Notes | Detach≠physical delete. |

#### `MEDIA-05` — `DetachMediaFromAdvert`

| Alan | İçerik |
| --- | --- |
| ID | `MEDIA-05` |
| Exact function name | `DetachMediaFromAdvert` |
| Domain | MEDIA |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE advert owner |
| Purpose | Relation siler; asset physical delete değildir. |
| Preconditions | ACTIVE; owner; DRAFT|CHANGES_REQUESTED; expected media_version. |
| Input summary | Advert id, asset id, expected media_version. |
| Output summary | New media_version. |
| Reads | `hrd_advert_media`, `hrd_adverts` |
| Writes | `hrd_advert_media`, `hrd_adverts` |
| Transaction boundary | Delete relation + cover fix if needed + media_version++ TX. |
| Concurrency | `media_version` |
| Idempotency | Idempotent if already detached |
| Side effects | None |
| Primary domain errors | `STALE_VERSION`, `INVALID_STATE` |
| Notes | — |
#### `MEDIA-06` — `ReorderAdvertMedia`

| Alan | İçerik |
| --- | --- |
| ID | `MEDIA-06` |
| Exact function name | `ReorderAdvertMedia` |
| Domain | MEDIA |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE advert owner |
| Purpose | Display order’ı collision-safe yeniden yazar. |
| Preconditions | ACTIVE; owner; DRAFT|CHANGES_REQUESTED; expected media_version. |
| Input summary | Ordered asset ids; expected media_version. |
| Output summary | New order + media_version. |
| Reads | `hrd_advert_media`, `hrd_adverts` |
| Writes | `hrd_advert_media`, `hrd_adverts` |
| Transaction boundary | Two-phase or temp-order update same TX + media_version++. |
| Concurrency | `media_version`; unique (advert,display_order) |
| Idempotency | Conditional |
| Side effects | None |
| Primary domain errors | `STALE_VERSION`, `VALIDATION_ERROR` |
| Notes | — |
#### `MEDIA-07` — `SetAdvertCover`

| Alan | İçerik |
| --- | --- |
| ID | `MEDIA-07` |
| Exact function name | `SetAdvertCover` |
| Domain | MEDIA |
| Exposure | `FE_AUTH` |
| Actor | ACTIVE advert owner |
| Purpose | Tek cover atar; eski cover güvenli kaldırılır. |
| Preconditions | ACTIVE; owner; DRAFT|CHANGES_REQUESTED; asset attached; expected media_version. |
| Input summary | Asset id; expected media_version. |
| Output summary | Cover set + media_version. |
| Reads | `hrd_advert_media`, `hrd_adverts` |
| Writes | `hrd_advert_media`, `hrd_adverts` |
| Transaction boundary | Clear old cover + set new + media_version++ same TX (partial unique one cover). |
| Concurrency | `media_version` |
| Idempotency | Idempotent if already cover |
| Side effects | None |
| Primary domain errors | `STALE_VERSION`, `INVALID_STATE` |
| Notes | — |
#### `MEDIA-ADMIN-01` — `InitiateAdminMediaUpload`

| Alan | İçerik |
| --- | --- |
| ID | `MEDIA-ADMIN-01` |
| Exact function name | `InitiateAdminMediaUpload` |
| Domain | MEDIA |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Banner vb. için UPLOAD_PENDING admin asset oluşturur. |
| Preconditions | admin+ADMIN_BO. |
| Input summary | Intent hints; placement target optional. Client idempotency key yok. |
| Output summary | Asset id + short-lived upload auth. |
| Reads | `hrd_users` |
| Writes | `hrd_media_assets` |
| Transaction boundary | Insert TX. B2 authorization TX dışı; failure → UPLOAD_PENDING orphan mümkün; yeniden initiation; cleanup. Persist idempotency key yok. |
| Concurrency | — |
| Idempotency | Non-idempotent; rate limiting; abandoned cleanup |
| Side effects | None |
| Primary domain errors | `FORBIDDEN`, `VALIDATION_ERROR`, `RATE_LIMITED`, `DEPENDENCY_UNAVAILABLE` |
| Notes | Kalıcı B2 credential yok. Advert relation fonksiyonlarına karışmaz. |

#### `MEDIA-ADMIN-02` — `ConfirmAdminMediaUpload`

| Alan | İçerik |
| --- | --- |
| ID | `MEDIA-ADMIN-02` |
| Exact function name | `ConfirmAdminMediaUpload` |
| Domain | MEDIA |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Admin asset upload completion. |
| Preconditions | admin+ADMIN_BO; asset created in admin flow. |
| Input summary | Asset id. |
| Output summary | Status. |
| Reads | `hrd_media_assets`, `hrd_background_jobs` |
| Writes | `hrd_media_assets`, `hrd_background_jobs` |
| Transaction boundary | Provider verify TX dışı; sonra tek TX: asset→UPLOADED + MEDIA_VALIDATE_AND_NORMALIZE job insert (dedup job_type+asset_id). Ayrı durable queue tablosu yok. |
| Concurrency | deduplication_key |
| Idempotency | Idempotent via lifecycle + job dedup |
| Side effects | Durable validate job enqueue (same TX) |
| Primary domain errors | `INVALID_STATE`, `DEPENDENCY_UNAVAILABLE`, `FORBIDDEN` |
| Notes | — |

#### `MEDIA-ADMIN-03` — `GetAdminMediaProcessingStatus`

| Alan | İçerik |
| --- | --- |
| ID | `MEDIA-ADMIN-03` |
| Exact function name | `GetAdminMediaProcessingStatus` |
| Domain | MEDIA |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Admin asset processing projection. |
| Preconditions | admin+ADMIN_BO. |
| Input summary | Asset id. |
| Output summary | Lifecycle/variants; no object keys. |
| Reads | `hrd_media_assets`, `hrd_media_variants` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `NOT_FOUND` |
| Notes | — |
## 21. BANNER

#### `BANNER-PUBLIC-01` — `ListActiveBannersByPlacement`

| Alan | İçerik |
| --- | --- |
| ID | `BANNER-PUBLIC-01` |
| Exact function name | `ListActiveBannersByPlacement` |
| Domain | BANNER |
| Exposure | `PUBLIC` |
| Actor | Anonymous |
| Purpose | Placement için ACTIVE banner’ları READY variant ile listeler. |
| Preconditions | Placement in HOMEPAGE|LISTING_DETAIL|SEARCH. |
| Input summary | Placement. |
| Output summary | Ordered public banners (title/alt/target URL + READY media URL projection). |
| Reads | `hrd_banners`, `hrd_media_assets`, `hrd_media_variants` |
| Writes | None |
| Transaction boundary | read-only; query-time banner ACTIVE + required variant READY yeniden doğrulanır |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `VALIDATION_ERROR` |
| Notes | Scheduling yok. |

#### `BANNER-ADMIN-01` — `ListBannersAdmin`

| Alan | İçerik |
| --- | --- |
| ID | `BANNER-ADMIN-01` |
| Exact function name | `ListBannersAdmin` |
| Domain | BANNER |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | BO list. |
| Preconditions | admin+ADMIN_BO. |
| Input summary | Filters. |
| Output summary | Admin list. |
| Reads | `hrd_banners` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `FORBIDDEN` |
| Notes | — |
#### `BANNER-ADMIN-02` — `GetBannerAdminDetail`

| Alan | İçerik |
| --- | --- |
| ID | `BANNER-ADMIN-02` |
| Exact function name | `GetBannerAdminDetail` |
| Domain | BANNER |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | BO detail. |
| Preconditions | admin+ADMIN_BO. |
| Input summary | Banner id. |
| Output summary | Detail + asset status. |
| Reads | `hrd_banners`, `hrd_media_assets`, `hrd_media_variants` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `NOT_FOUND`, `FORBIDDEN` |
| Notes | — |

#### `BANNER-ADMIN-03` — `CreateBanner`

| Alan | İçerik |
| --- | --- |
| ID | `BANNER-ADMIN-03` |
| Exact function name | `CreateBanner` |
| Domain | BANNER |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Banner oluşturur (tek placement); her zaman INACTIVE version 1. |
| Preconditions | admin+ADMIN_BO; asset exists; placement exact set; title/alt/target URL/sort temel validation; asset lifecycle in UPLOAD_PENDING|UPLOADED|VALIDATING|MASTER_READY. VALIDATION_FAILED|CLEANUP_CANDIDATE|DELETING|PHYSICALLY_DELETED reddedilir. |
| Input summary | Placement, asset_id, title, alt, target URL, sort. Client ACTIVE seçemez. |
| Output summary | Created banner: status=`INACTIVE`, version=`1`. |
| Reads | `hrd_media_assets` |
| Writes | `hrd_banners` |
| Transaction boundary | Aynı TX: asset row lock (SELECT FOR UPDATE eşdeğeri); lifecycle recheck; insert banner INACTIVE v1. READY/dims/MIME/byte zorunlu değil — ACTIVE yalnız SetBannerStatus’ta. |
| Concurrency | asset row lock vs cleanup |
| Idempotency | Non-idempotent |
| Side effects | None |
| Primary domain errors | `VALIDATION_ERROR`, `INVALID_STATE`, `NOT_FOUND`, `FORBIDDEN` |
| Notes | Advert-media relation kullanmaz. Processing asset INACTIVE bağlanabilir. Hard delete yok. |

#### `BANNER-ADMIN-04` — `UpdateBanner`

| Alan | İçerik |
| --- | --- |
| ID | `BANNER-ADMIN-04` |
| Exact function name | `UpdateBanner` |
| Domain | BANNER |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Banner metadata/asset günceller. |
| Preconditions | admin+ADMIN_BO; expected version. INACTIVE iken processing asset’e geçiş (allowed lifecycle) yapılabilir. ACTIVE iken yeni asset/placement required READY physical variant + dims/MIME/byte; aksi halde reddedilir. Asset değişiyorsa yeni asset row lock + lifecycle recheck. |
| Input summary | Expected version; mutable fields; asset change. |
| Output summary | Updated. |
| Reads | `hrd_banners`, `hrd_media_assets`, `hrd_media_variants` |
| Writes | `hrd_banners` |
| Transaction boundary | Same-snapshot validation + update + version++. Asset değişiminde: asset lock; allowed lifecycle UPLOAD_PENDING|UPLOADED|VALIDATING|MASTER_READY (INACTIVE banner) veya READY rules (ACTIVE banner). |
| Concurrency | `version`; asset lock when asset_id changes |
| Idempotency | Conditional |
| Side effects | None |
| Primary domain errors | `STALE_VERSION`, `VALIDATION_ERROR`, `PROCESSING_NOT_READY`, `INVALID_STATE`, `FORBIDDEN` |
| Notes | INACTIVE asset orphan sayılmaz. |

#### `BANNER-ADMIN-05` — `SetBannerStatus`

| Alan | İçerik |
| --- | --- |
| ID | `BANNER-ADMIN-05` |
| Exact function name | `SetBannerStatus` |
| Domain | BANNER |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | ACTIVE/INACTIVE. |
| Preconditions | admin+ADMIN_BO; expected version. ACTIVE geçişinde aynı kısa TX/snapshot: expected version, asset relation, placement required READY variant, dims/MIME/byte validation, sonra status ACTIVE + version++. |
| Input summary | Status, expected version. |
| Output summary | Updated status. |
| Reads | `hrd_banners`, `hrd_media_assets`, `hrd_media_variants` |
| Writes | `hrd_banners` |
| Transaction boundary | ACTIVE path: READY recheck + status + version aynı kısa TX (eski/TX-dışı stale okumaya dayanmaz). INACTIVE path: status + version. |
| Concurrency | `version` |
| Idempotency | Idempotent if same status |
| Side effects | None |
| Primary domain errors | `PROCESSING_NOT_READY`, `STALE_VERSION`, `VALIDATION_ERROR`, `FORBIDDEN` |
| Notes | Public ListActiveBannersByPlacement query-time ACTIVE + READY yeniden doğrular. |

#### `BANNER-ADMIN-06` — `ReorderBanners`

| Alan | İçerik |
| --- | --- |
| ID | `BANNER-ADMIN-06` |
| Exact function name | `ReorderBanners` |
| Domain | BANNER |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Placement içinde sort_order. |
| Preconditions | admin+ADMIN_BO. |
| Input summary | Placement; ordered ids+versions. |
| Output summary | New order. |
| Reads | `hrd_banners` |
| Writes | `hrd_banners` |
| Transaction boundary | Deterministic multi-row TX. |
| Concurrency | versions |
| Idempotency | Conditional |
| Side effects | None |
| Primary domain errors | `STALE_VERSION` |
| Notes | — |
## 22. JOB (internal)

HTTP API değildir.

#### `JOB-01` — `EnqueueBackgroundJob`

| Alan | İçerik |
| --- | --- |
| ID | `JOB-01` |
| Exact function name | `EnqueueBackgroundJob` |
| Domain | JOB |
| Exposure | `INTERNAL_SYSTEM` |
| Actor | system/orchestration |
| Purpose | Durable job satırı oluşturur. |
| Preconditions | job_type whitelist; payload object; max_attempts explicit; TJK type requires run_id; media types forbid run_id. |
| Input summary | job_type, payload, max_attempts, available_at, optional deduplication_key, optional tjk_sync_run_id. |
| Output summary | Job id or existing via dedup. |
| Reads | `hrd_tjk_sync_runs` (yalnız `TJK_SYNC_BATCH`); diğer job type’larda `None` |
| Writes | `hrd_background_jobs` |
| Transaction boundary | Insert TX; dedup conflict → return existing. |
| Concurrency | deduplication_key partial unique |
| Idempotency | Conditional idempotent |
| Side effects | None |
| Primary domain errors | `VALIDATION_ERROR`, `CONFLICT` |
| Notes | Büyük dataset payload’a gömülmez. Secret yok. |
#### `JOB-02` — `ClaimAvailableBackgroundJobs`

| Alan | İçerik |
| --- | --- |
| ID | `JOB-02` |
| Exact function name | `ClaimAvailableBackgroundJobs` |
| Domain | JOB |
| Exposure | `INTERNAL_WORKER` |
| Actor | worker |
| Purpose | QUEUED job’ları LEASED yapar; attempt_count başlatılmış attempt olarak artırır. |
| Preconditions | Worker identity. |
| Input summary | Limit, lease duration, lease_owner. |
| Output summary | Claimed jobs. |
| Reads | `hrd_background_jobs` |
| Writes | `hrd_background_jobs` |
| Transaction boundary | Claim seçimi: status=QUEUED AND available_at<=now AND attempt_count<max_attempts. Aynı TX per job: SELECT FOR UPDATE SKIP LOCKED; status→LEASED; lease_owner; leased_until; attempt_count=attempt_count+1; version++. |
| Concurrency | `version` + lease; attempt_count claim-time increment |
| Idempotency | Non-idempotent |
| Side effects | None |
| Primary domain errors | `INTERNAL_ERROR` |
| Notes | attempt_count = başlatılmış worker attempt sayısı. İki worker aynı job’ı alamaz. |

#### `JOB-03` — `RenewBackgroundJobLease`

| Alan | İçerik |
| --- | --- |
| ID | `JOB-03` |
| Exact function name | `RenewBackgroundJobLease` |
| Domain | JOB |
| Exposure | `INTERNAL_WORKER` |
| Actor | worker (lease owner) |
| Purpose | LEASED job lease süresini uzatır. |
| Preconditions | status=LEASED; lease_owner match; expected version. |
| Input summary | Job id, version, new leased_until. |
| Output summary | Renewed. |
| Reads | `hrd_background_jobs` |
| Writes | `hrd_background_jobs` |
| Transaction boundary | Update lease+version where match. |
| Concurrency | `version` |
| Idempotency | Conditional |
| Side effects | None |
| Primary domain errors | `STALE_VERSION`, `INVALID_STATE` |
| Notes | — |
#### `JOB-04` — `CompleteBackgroundJob`

| Alan | İçerik |
| --- | --- |
| ID | `JOB-04` |
| Exact function name | `CompleteBackgroundJob` |
| Domain | JOB |
| Exposure | `INTERNAL_WORKER` |
| Actor | worker |
| Purpose | Job’ı SUCCEEDED terminal yapar. |
| Preconditions | LEASED; owner; expected version; cancel not forcing cancel path. |
| Input summary | Job id, version. |
| Output summary | Terminal SUCCEEDED. |
| Reads | `hrd_background_jobs` |
| Writes | `hrd_background_jobs` |
| Transaction boundary | Status SUCCEEDED; completed_at; lease clear; version++. |
| Concurrency | `version`; stale worker rejected |
| Idempotency | Non-idempotent / conditional if already SUCCEEDED |
| Side effects | None |
| Primary domain errors | `STALE_VERSION`, `INVALID_STATE` |
| Notes | — |
#### `JOB-05` — `RetryOrFailBackgroundJob`

| Alan | İçerik |
| --- | --- |
| ID | `JOB-05` |
| Exact function name | `RetryOrFailBackgroundJob` |
| Domain | JOB |
| Exposure | `INTERNAL_WORKER` |
| Actor | worker |
| Purpose | Transient→QUEUED retry veya FAILED/DEAD terminal; attempt_count burada artırılmaz. |
| Preconditions | LEASED; owner; expected version. |
| Input summary | Job id, version, error class, last_error (sanitized). |
| Output summary | New status. |
| Reads | `hrd_background_jobs` |
| Writes | `hrd_background_jobs` |
| Transaction boundary | attempt_count artırılmaz (claim’de artırıldı). Permanent → FAILED + completed_at + lease clear. Transient ve attempt_count<max_attempts → QUEUED + available_at/backoff + lease clear + completed_at NULL. Transient ve attempt_count=max_attempts → DEAD + completed_at + lease clear. version++. |
| Concurrency | `version` |
| Idempotency | Conditional |
| Side effects | None |
| Primary domain errors | `STALE_VERSION`, `INVALID_STATE` |
| Notes | FAILED≠DEAD. last_error secret içermez. |

#### `JOB-06` — `RequestBackgroundJobCancellation`

| Alan | İçerik |
| --- | --- |
| ID | `JOB-06` |
| Exact function name | `RequestBackgroundJobCancellation` |
| Domain | JOB |
| Exposure | `INTERNAL_SYSTEM` |
| Actor | system/BO orchestration |
| Purpose | cancel_requested_at set eder (CANCELLED değil). |
| Preconditions | Job not terminal. |
| Input summary | Job id. |
| Output summary | Cancel requested. |
| Reads | `hrd_background_jobs` |
| Writes | `hrd_background_jobs` |
| Transaction boundary | Set cancel_requested_at TX. |
| Concurrency | — |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `INVALID_STATE`, `NOT_FOUND` |
| Notes | Cooperative; worker Apply path cancels. |
#### `JOB-07` — `RecoverExpiredBackgroundJobLeases`

| Alan | İçerik |
| --- | --- |
| ID | `JOB-07` |
| Exact function name | `RecoverExpiredBackgroundJobLeases` |
| Domain | JOB |
| Exposure | `INTERNAL_WORKER` |
| Actor | worker/scheduler |
| Purpose | Expired LEASED job’ları cancel/retry/dead kurallarına göre çözer. |
| Preconditions | leased_until < now; status=LEASED. |
| Input summary | Batch limit. |
| Output summary | Recovered count. |
| Reads | `hrd_background_jobs` |
| Writes | `hrd_background_jobs` |
| Transaction boundary | Her job kısa TX: expected LEASED+expired lease. (1) cancel_requested_at NOT NULL → CANCELLED + completed_at + lease clear. (2) no cancel ve attempt_count<max_attempts → QUEUED + available_at/backoff + lease clear. (3) no cancel ve attempt_count=max_attempts → DEAD + completed_at + lease clear. Her branch version++. Crashed worker attempt bütçesini tüketmiş sayılır; sonsuz recovery döngüsü yok. |
| Concurrency | — |
| Idempotency | Idempotent per expired job state |
| Side effects | None |
| Primary domain errors | None |
| Notes | attempt_count claim’de artmış olduğu için recovery’de yeniden artırılmaz. |

## 23. MEDIA-WORKER

#### `MEDIA-WORKER-01` — `ValidateAndNormalizeMediaAsset`

| Alan | İçerik |
| --- | --- |
| ID | `MEDIA-WORKER-01` |
| Exact function name | `ValidateAndNormalizeMediaAsset` |
| Domain | MEDIA |
| Exposure | `INTERNAL_WORKER` |
| Actor | worker |
| Purpose | UPLOADED→VALIDATING→MASTER_READY veya VALIDATION_FAILED; variant jobs enqueue. |
| Preconditions | Job type MEDIA_VALIDATE_AND_NORMALIZE; asset UPLOADED (veya VALIDATING retry). |
| Input summary | Job payload asset_id. |
| Output summary | Job completion outcome. |
| Reads | `hrd_media_assets`, `hrd_background_jobs` |
| Writes | `hrd_media_assets`, `hrd_background_jobs` |
| Transaction boundary | Kısa TX: status→VALIDATING. External (B2 read/decode/validate/normalize/master write) TX dışı. Başarı sonrası tek kısa TX: canonical master metadata + lifecycle MASTER_READY + tüm required transform profile’lar için MEDIA_GENERATE_VARIANT job insert’leri aynı TX. Validation fail: VALIDATION_FAILED + failure_reason; variant job yok. Domain lifecycle ≠ job lifecycle. |
| Concurrency | asset lifecycle; job dedup per asset+profile |
| Idempotency | Retry-safe |
| Side effects | Variant job enqueue only on MASTER_READY path (same TX) |
| Primary domain errors | `DEPENDENCY_UNAVAILABLE`, `VALIDATION_ERROR` |
| Notes | EXIF/GPS strip; raw metadata canonical alanlara karışmaz; secret/object content log yok. |

#### `MEDIA-WORKER-02` — `GenerateMediaVariant`

| Alan | İçerik |
| --- | --- |
| ID | `MEDIA-WORKER-02` |
| Exact function name | `GenerateMediaVariant` |
| Domain | MEDIA |
| Exposure | `INTERNAL_WORKER` |
| Actor | worker |
| Purpose | Physical transform profile variant üretir. |
| Preconditions | Asset MASTER_READY; profile whitelist. |
| Input summary | asset_id, transform_profile. |
| Output summary | Variant READY/FAILED. |
| Reads | `hrd_media_assets`, `hrd_media_variants` |
| Writes | `hrd_media_variants` |
| Transaction boundary | Upsert PENDING row TX; TinyPNG/transform TX DIŞI; READY/FAILED TX. Unique asset+profile. |
| Concurrency | unique profile |
| Idempotency | Idempotent/retry-safe |
| Side effects | None |
| Primary domain errors | `DEPENDENCY_UNAVAILABLE` |
| Notes | Uncompressed fallback otomatik public olmaz. Object key immutable. |
#### `MEDIA-WORKER-03` — `DeleteMediaObjects`

| Alan | İçerik |
| --- | --- |
| ID | `MEDIA-WORKER-03` |
| Exact function name | `DeleteMediaObjects` |
| Domain | MEDIA |
| Exposure | `INTERNAL_WORKER` |
| Actor | worker |
| Purpose | Reference-safe physical delete pipeline. |
| Preconditions | Job MEDIA_DELETE_OBJECTS; asset CLEANUP_CANDIDATE; retention elapsed. |
| Input summary | asset_id. |
| Output summary | PHYSICALLY_DELETED. |
| Reads | `hrd_media_assets`, `hrd_advert_media`, `hrd_banners`, `hrd_media_variants` |
| Writes | `hrd_media_assets`, `hrd_media_variants` |
| Transaction boundary | İlk kısa TX: (1) asset row lock; (2) status CLEANUP_CANDIDATE doğrula; (3) advert_media ref recheck; (4) banners ref recheck (INACTIVE dahil); (5) refs yoksa lifecycle DELETING; commit. Sonra B2 delete TX dışı. Ardından PHYSICALLY_DELETED kısa TX. Relation create aynı asset lock disiplinini kullandığı için yarış engellenir. |
| Concurrency | asset row lock |
| Idempotency | Retry-safe |
| Side effects | None |
| Primary domain errors | `CONFLICT`, `INVALID_STATE`, `DEPENDENCY_UNAVAILABLE` |
| Notes | Detach ≠ physical delete. |

#### `MEDIA-WORKER-04` — `ReconcileMediaStorage`

| Alan | İçerik |
| --- | --- |
| ID | `MEDIA-WORKER-04` |
| Exact function name | `ReconcileMediaStorage` |
| Domain | MEDIA |
| Exposure | `INTERNAL_WORKER` |
| Actor | worker |
| Purpose | Missing/orphan/mismatch tespit eder; otomatik veri kaybı yaratmaz. |
| Preconditions | Job MEDIA_RECONCILE. |
| Input summary | Scope payload. |
| Output summary | Reconciliation report/job outcome. |
| Reads | `hrd_media_assets`, `hrd_media_variants`, `hrd_advert_media`, `hrd_banners` |
| Writes | `hrd_media_assets` (yalnız güvenli marker/status düzeltmeleri gerektiğinde); `hrd_background_jobs` (job outcome/terminal update caller path) |
| Transaction boundary | Read-mostly; provider list TX dışı; minimal corrective writes ayrı kısa TX; otomatik destructive delete yok. |
| Concurrency | — |
| Idempotency | Idempotent report |
| Side effects | None (destructive physical delete bu fonksiyonun işi değil) |
| Primary domain errors | `DEPENDENCY_UNAVAILABLE` |
| Notes | Exact retention süreleri açık karar. |

## 24. TJK-ADMIN (BO)

#### `TJK-ADMIN-01` — `TriggerTJKSync`

| Alan | İçerik |
| --- | --- |
| ID | `TJK-ADMIN-01` |
| Exact function name | `TriggerTJKSync` |
| Domain | TJK |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | HORSES scope için MANUAL sync run QUEUED + initial batch job enqueue. |
| Preconditions | admin+ADMIN_BO; no active QUEUED/RUNNING for source+scope; mode in FULL|INCREMENTAL|RECONCILIATION. |
| Input summary | mode; controlled source_adapter; scope fixed HORSES. |
| Output summary | Run id + status QUEUED. |
| Reads | `hrd_tjk_sync_runs` |
| Writes | `hrd_tjk_sync_runs`, `hrd_background_jobs` |
| Transaction boundary | Run insert (created_by=session user, trigger MANUAL) + EnqueueBackgroundJob(TJK_SYNC_BATCH) same orchestration TX. |
| Concurrency | one-active partial unique; run version |
| Idempotency | Conflict if active run |
| Side effects | Initial job |
| Primary domain errors | `CONFLICT`, `FORBIDDEN`, `VALIDATION_ERROR` |
| Notes | Raw payload/credential yok. FE tetiklemez. Creator server actor. |
#### `TJK-ADMIN-02` — `CancelTJKSync`

| Alan | İçerik |
| --- | --- |
| ID | `TJK-ADMIN-02` |
| Exact function name | `CancelTJKSync` |
| Domain | TJK |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Cooperative cancel request on run + related jobs. |
| Preconditions | admin+ADMIN_BO; run cancellable. |
| Input summary | Run id. |
| Output summary | cancel_requested. |
| Reads | `hrd_tjk_sync_runs`, `hrd_background_jobs` |
| Writes | `hrd_tjk_sync_runs`, `hrd_background_jobs` |
| Transaction boundary | Run cancel_requested + RequestBackgroundJobCancellation for open jobs; expected run version. |
| Concurrency | `version` |
| Idempotency | Idempotent |
| Side effects | Job cancel requests |
| Primary domain errors | `INVALID_STATE`, `STALE_VERSION`, `NOT_FOUND` |
| Notes | Committed horse updates geri alınmaz. |
#### `TJK-ADMIN-03` — `ListTJKSyncRuns`

| Alan | İçerik |
| --- | --- |
| ID | `TJK-ADMIN-03` |
| Exact function name | `ListTJKSyncRuns` |
| Domain | TJK |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Sync run listesi. |
| Preconditions | admin+ADMIN_BO. |
| Input summary | Filters, cursor. |
| Output summary | Run summaries. |
| Reads | `hrd_tjk_sync_runs` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `FORBIDDEN` |
| Notes | — |
#### `TJK-ADMIN-04` — `GetTJKSyncRun`

| Alan | İçerik |
| --- | --- |
| ID | `TJK-ADMIN-04` |
| Exact function name | `GetTJKSyncRun` |
| Domain | TJK |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Run detay + counters/checkpoint summary. |
| Preconditions | admin+ADMIN_BO. |
| Input summary | Run id. |
| Output summary | Run detail (no secrets). |
| Reads | `hrd_tjk_sync_runs` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `NOT_FOUND` |
| Notes | — |
#### `TJK-ADMIN-05` — `ListTJKSyncItemErrors`

| Alan | İçerik |
| --- | --- |
| ID | `TJK-ADMIN-05` |
| Exact function name | `ListTJKSyncItemErrors` |
| Domain | TJK |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Run item errors. |
| Preconditions | admin+ADMIN_BO; run exists. |
| Input summary | Run id, status filter, cursor. |
| Output summary | Error list (empty OK if run exists). |
| Reads | `hrd_tjk_sync_runs`, `hrd_tjk_sync_item_errors` |
| Writes | None |
| Transaction boundary | read-only: run existence önce; yoksa NOT_FOUND |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `NOT_FOUND`, `FORBIDDEN` |
| Notes | Run yok ile run var/error yok ayrılır. Retry single item fonksiyonu yok (açık karar). |

#### `TJK-ADMIN-06` — `ResolveTJKSyncItemError`

| Alan | İçerik |
| --- | --- |
| ID | `TJK-ADMIN-06` |
| Exact function name | `ResolveTJKSyncItemError` |
| Domain | TJK |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Error status RESOLVED. |
| Preconditions | admin+ADMIN_BO; error OPEN. |
| Input summary | Error id. |
| Output summary | Resolved. |
| Reads | `hrd_tjk_sync_item_errors` |
| Writes | `hrd_tjk_sync_item_errors` |
| Transaction boundary | Status+resolved_at TX. |
| Concurrency | — |
| Idempotency | Idempotent if resolved |
| Side effects | None |
| Primary domain errors | `INVALID_STATE` |
| Notes | Canonical horse edit değildir. |
#### `TJK-ADMIN-07` — `IgnoreTJKSyncItemError`

| Alan | İçerik |
| --- | --- |
| ID | `TJK-ADMIN-07` |
| Exact function name | `IgnoreTJKSyncItemError` |
| Domain | TJK |
| Exposure | `BO_AUTH` |
| Actor | admin+ADMIN_BO |
| Purpose | Error status IGNORED. |
| Preconditions | admin+ADMIN_BO; error OPEN. |
| Input summary | Error id. |
| Output summary | Ignored. |
| Reads | `hrd_tjk_sync_item_errors` |
| Writes | `hrd_tjk_sync_item_errors` |
| Transaction boundary | Status TX. |
| Concurrency | — |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `INVALID_STATE` |
| Notes | — |
## 25. TJK-WORKER / scheduled

Source adapter erişim yöntemi open; API/file/scraping uydurulmaz. Normalized adapter input + field presence kuralları geçerlidir.

#### `TJK-WORKER-01` — `TriggerScheduledTJKSync`

| Alan | İçerik |
| --- | --- |
| ID | `TJK-WORKER-01` |
| Exact function name | `TriggerScheduledTJKSync` |
| Domain | TJK |
| Exposure | `INTERNAL_SYSTEM` |
| Actor | scheduler |
| Purpose | SCHEDULED trigger ile run oluşturur (creator NULL). |
| Preconditions | No active run for source+scope; schedule policy. |
| Input summary | mode, source_adapter. |
| Output summary | Run id. |
| Reads | `hrd_tjk_sync_runs` |
| Writes | `hrd_tjk_sync_runs`, `hrd_background_jobs` |
| Transaction boundary | Run insert trigger_kind=SCHEDULED created_by NULL + enqueue initial job TX. |
| Concurrency | one-active unique |
| Idempotency | Skip/conflict if active |
| Side effects | Job enqueue |
| Primary domain errors | `CONFLICT` |
| Notes | Admin role DB’de doğrulanmaz (scheduled). |
#### `TJK-WORKER-02` — `StartTJKSyncRun`

| Alan | İçerik |
| --- | --- |
| ID | `TJK-WORKER-02` |
| Exact function name | `StartTJKSyncRun` |
| Domain | TJK |
| Exposure | `INTERNAL_WORKER` |
| Actor | worker |
| Purpose | QUEUED run → RUNNING; started_at. |
| Preconditions | Run QUEUED; expected version; cancel not completed. |
| Input summary | Run id, version. |
| Output summary | RUNNING. |
| Reads | `hrd_tjk_sync_runs` |
| Writes | `hrd_tjk_sync_runs` |
| Transaction boundary | Status+started_at+version++ TX. |
| Concurrency | `version` |
| Idempotency | Conditional |
| Side effects | None |
| Primary domain errors | `STALE_VERSION`, `INVALID_STATE` |
| Notes | — |
#### `TJK-WORKER-03` — `PlanTJKSyncBatches`

| Alan | İçerik |
| --- | --- |
| ID | `TJK-WORKER-03` |
| Exact function name | `PlanTJKSyncBatches` |
| Domain | TJK |
| Exposure | `INTERNAL_WORKER` |
| Actor | worker |
| Purpose | Checkpoint’ten sonraki batch planını üretir; follow-up jobs enqueue. |
| Preconditions | Run RUNNING; adapter available. |
| Input summary | Run id. |
| Output summary | Batch plan / enqueued jobs. |
| Reads | `hrd_tjk_sync_runs`, `hrd_background_jobs` |
| Writes | `hrd_background_jobs` |
| Transaction boundary | Read checkpoint; enqueue batch jobs with small payloads. Deduplication key deterministik: run ID + batch cursor/range identity + job purpose. Aynı plan tekrarında duplicate batch job yok. Initial bootstrap TJK_SYNC_BATCH job StartTJKSyncRun + PlanTJKSyncBatches orchestration’ını tetikler. |
| Concurrency | run version when advancing; job dedup |
| Idempotency | Retry-safe / idempotent enqueue via dedup |
| Side effects | Job enqueue |
| Primary domain errors | `DEPENDENCY_UNAVAILABLE` |
| Notes | Adapter call TX dışı. Yeni job type yok. |

#### `TJK-WORKER-04` — `ProcessTJKSyncBatch`

| Alan | İçerik |
| --- | --- |
| ID | `TJK-WORKER-04` |
| Exact function name | `ProcessTJKSyncBatch` |
| Domain | TJK |
| Exposure | `INTERNAL_WORKER` |
| Actor | worker |
| Purpose | Normalized batch öğelerini horse upsert + item errors uygular. |
| Preconditions | Run RUNNING; job LEASED; cancel boundary respected. |
| Input summary | Run id, batch cursor/range, normalized items with field presence. |
| Output summary | Batch result counters; per-item errors. Checkpoint/job terminal `AdvanceTJKSyncCheckpoint` ile yapılır. |
| Reads | `hrd_tjk_sync_runs`, `hrd_horses`, `hrd_tjk_sync_item_errors` |
| Writes | `hrd_horses`, `hrd_tjk_sync_item_errors` |
| Transaction boundary | Adapter fetch TX dışı. Her horse upsert/error kısa item TX. Permanent item error ≠ technical batch failure. Technical failure → checkpoint ilerlemez. Success sonrası finalization `AdvanceTJKSyncCheckpoint` (checkpoint + job SUCCEEDED aynı TX). |
| Concurrency | horse tjk_number unique |
| Idempotency | Item-level idempotent upsert |
| Side effects | Item errors only on fail/conflict; no per-success audit |
| Primary domain errors | `DEPENDENCY_UNAVAILABLE` |
| Notes | Missing-from-source ≠ hard delete. Live TJK fallback yok. Raw payload canonical/public değil. |

#### `TJK-WORKER-05` — `AdvanceTJKSyncCheckpoint`

| Alan | İçerik |
| --- | --- |
| ID | `TJK-WORKER-05` |
| Exact function name | `AdvanceTJKSyncCheckpoint` |
| Domain | TJK |
| Exposure | `INTERNAL_WORKER` |
| Actor | worker |
| Purpose | Committed contiguous checkpoint + counters ilerletir ve başarılı batch job’ı aynı TX’de terminal SUCCEEDED yapar. |
| Preconditions | Run RUNNING; Job LEASED; lease_owner eşleşiyor; run expected version; job expected version; yeni checkpoint contiguous. |
| Input summary | Run id, expected run version, job id, expected job version, new checkpoint, delta counters. |
| Output summary | Updated checkpoint; job SUCCEEDED. |
| Reads | `hrd_tjk_sync_runs`, `hrd_background_jobs` |
| Writes | `hrd_tjk_sync_runs`, `hrd_background_jobs` |
| Transaction boundary | Item işlemleri bittikten sonra tek kısa TX: (1) run checkpoint/counters/version; (2) job SUCCEEDED + completed_at + lease clear + version (CompleteBackgroundJob primitive caller TX içinde). Technical batch failure: checkpoint ilerlemez; RetryOrFailBackgroundJob çağrılır; committed horse kayıtları korunur. |
| Concurrency | run `version` + job `version` |
| Idempotency | Conditional |
| Side effects | Batch job completion (same TX) |
| Primary domain errors | `STALE_VERSION`, `INVALID_STATE` |
| Notes | Gap atlama yok. ProcessTJKSyncBatch item TX’lerinden sonra bu finalization çağrılır. |

#### `TJK-WORKER-06` — `FinalizeTJKSyncRun`

| Alan | İçerik |
| --- | --- |
| ID | `TJK-WORKER-06` |
| Exact function name | `FinalizeTJKSyncRun` |
| Domain | TJK |
| Exposure | `INTERNAL_WORKER` |
| Actor | worker |
| Purpose | Run terminal status’unu DB job/counter durumundan türetir. |
| Preconditions | Run RUNNING; expected version; run’a bağlı hiçbir job QUEUED veya LEASED değil. |
| Input summary | Run ID; expected run version. Caller terminal status göndermez. |
| Output summary | Derived terminal status + completed_at. |
| Reads | `hrd_tjk_sync_runs`, `hrd_background_jobs` |
| Writes | `hrd_tjk_sync_runs` |
| Transaction boundary | Aynı TX: açık QUEUED/LEASED job yokluğunu yeniden doğrula; job terminal durumlarından status türet: herhangi FAILED/DEAD → run FAILED; tüm job başarılı ve (failed_count>0 veya conflict_count>0) → PARTIAL_SUCCESS; tüm başarılı ve failed_count=0 ve conflict_count=0 → SUCCEEDED. CANCELLED bu function’da türetilmez (yalnız ApplyTJKSyncCancellation). completed_at + version++ + güvenli last_error_summary. |
| Concurrency | `version` |
| Idempotency | Conditional |
| Side effects | None |
| Primary domain errors | `STALE_VERSION`, `INVALID_STATE` |
| Notes | Terminal status caller input değildir. |

#### `TJK-WORKER-07` — `ApplyTJKSyncCancellation`

| Alan | İçerik |
| --- | --- |
| ID | `TJK-WORKER-07` |
| Exact function name | `ApplyTJKSyncCancellation` |
| Domain | TJK |
| Exposure | `INTERNAL_WORKER` |
| Actor | worker |
| Purpose | Safe batch boundary’de CANCELLED finalize. |
| Preconditions | cancel_requested_at set; expected version. |
| Input summary | Run id, version. |
| Output summary | CANCELLED. |
| Reads | `hrd_tjk_sync_runs`, `hrd_background_jobs` |
| Writes | `hrd_tjk_sync_runs`, `hrd_background_jobs` |
| Transaction boundary | Cancel jobs cooperatively; run cancelled_at+completed_at+CANCELLED+version++ TX. |
| Concurrency | `version` |
| Idempotency | Idempotent if already cancelled |
| Side effects | Job CANCELLED via worker complete path |
| Primary domain errors | `STALE_VERSION` |
| Notes | Last trusted local data preserved. |
## 26. INTERNAL cross-cutting

Repository CRUD değildir. Outer use-case TX’ine katılır; kendi başına ayrı TX açmaz (belirtilmedikçe).

#### `INTERNAL-01` — `AppendSecurityEvent`

| Alan | İçerik |
| --- | --- |
| ID | `INTERNAL-01` |
| Exact function name | `AppendSecurityEvent` |
| Domain | INTERNAL |
| Exposure | `INTERNAL_SYSTEM` |
| Actor | system (caller supplies actor/subject) |
| Purpose | hrd_security_events satırı ekler. |
| Preconditions | Caller mode seçer: REQUIRED_CALLER_TRANSACTION veya BEST_EFFORT_INDEPENDENT (§5A). |
| Input summary | event_type, subject, actor, client_context, metadata (secret-free), write_mode. |
| Output summary | Event id. |
| Reads | None |
| Writes | `hrd_security_events` |
| Transaction boundary | REQUIRED_CALLER_TRANSACTION: caller TX içinde insert; failure → caller rollback. BEST_EFFORT_INDEPENDENT: ana commit sonrası ayrı kısa TX; failure ana sonucu değiştirmez; log+metric. Best-effort event, caller transaction’ıyla aynı transaction içinde yazılmaz. |
| Concurrency | — |
| Idempotency | Non-idempotent |
| Side effects | None beyond row |
| Primary domain errors | `VALIDATION_ERROR` |
| Notes | Actor nullable for system. Event type → mode mapping §5A. RegisterUser bu primitive’i çağırmaz. |

#### `INTERNAL-02` — `TransitionAdvertStatus`

| Alan | İçerik |
| --- | --- |
| ID | `INTERNAL-02` |
| Exact function name | `TransitionAdvertStatus` |
| Domain | INTERNAL |
| Exposure | `INTERNAL_SYSTEM` |
| Actor | system/owner/admin via caller |
| Purpose | Advert status + immutable history atomik geçiş. |
| Preconditions | Allowed transition graph; expected version; ownership/admin checked by caller. |
| Input summary | Advert id, from, to, actor_user_id nullable, is_system, reason, expected version. |
| Output summary | New status+version. |
| Reads | `hrd_adverts` |
| Writes | `hrd_adverts`, `hrd_advert_status_history` |
| Transaction boundary | Update status/version/published_at as needed + history insert same TX. Never split. |
| Concurrency | `version` |
| Idempotency | Non-idempotent |
| Side effects | History always on success |
| Primary domain errors | `INVALID_STATE`, `STALE_VERSION` |
| Notes | Failed attempts history yazmaz. |
#### `INTERNAL-03` — `RevokeUserSessions`

| Alan | İçerik |
| --- | --- |
| ID | `INTERNAL-03` |
| Exact function name | `RevokeUserSessions` |
| Domain | INTERNAL |
| Exposure | `INTERNAL_SYSTEM` |
| Actor | system |
| Purpose | User session’larını revoke eder. |
| Preconditions | Caller authorized; typically inside caller TX. |
| Input summary | user_id; scope all|except_session|family. |
| Output summary | Revoked count. |
| Reads | `hrd_auth_sessions` |
| Writes | `hrd_auth_sessions` |
| Transaction boundary | Bulk revoke in caller TX. Security event caller sorumluluğunda (status/role/password paths REQUIRED; logout paths BEST_EFFORT). |
| Concurrency | — |
| Idempotency | Idempotent |
| Side effects | Event yazımı caller’a aittir; bu primitive yalnız session satırlarını yazar |
| Primary domain errors | None |
| Notes | — |

#### `INTERNAL-04` — `ValidateAdvertForSubmission`

| Alan | İçerik |
| --- | --- |
| ID | `INTERNAL-04` |
| Exact function name | `ValidateAdvertForSubmission` |
| Domain | INTERNAL |
| Exposure | `INTERNAL_SYSTEM` |
| Actor | system |
| Purpose | Submit/resubmit/approve full validation (DB CHECK + category metadata rules). |
| Preconditions | Advert loaded. |
| Input summary | Advert id. |
| Output summary | Validation result errors list. |
| Reads | `hrd_adverts`, `hrd_users`, `hrd_categories`, `hrd_category_properties`, `hrd_districts`, `hrd_horses`, `hrd_advert_media`, `hrd_media_assets`, `hrd_media_variants` |
| Writes | None |
| Transaction boundary | read-only validation inside caller TX snapshot |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `VALIDATION_ERROR`, `EMAIL_NOT_VERIFIED`, `PROCESSING_NOT_READY` |
| Notes | Horse/min media/price required açık ürün kararlarına bağlı. |

#### `INTERNAL-05` — `ValidateDynamicProperties`

| Alan | İçerik |
| --- | --- |
| ID | `INTERNAL-05` |
| Exact function name | `ValidateDynamicProperties` |
| Domain | INTERNAL |
| Exposure | `INTERNAL_SYSTEM` |
| Actor | system |
| Purpose | Leaf category property map validate (draft partial veya submit full). |
| Preconditions | Category known. |
| Input summary | category_id, properties map, mode draft|submit. |
| Output summary | Normalized properties or errors. |
| Reads | `hrd_category_properties` |
| Writes | None |
| Transaction boundary | read-only |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `VALIDATION_ERROR` |
| Notes | Arbitrary keys reddedilir. Inheritance yok. |
#### `INTERNAL-06` — `ResolvePublicAdvertProjection`

| Alan | İçerik |
| --- | --- |
| ID | `INTERNAL-06` |
| Exact function name | `ResolvePublicAdvertProjection` |
| Domain | INTERNAL |
| Exposure | `INTERNAL_SYSTEM` |
| Actor | system |
| Purpose | PUBLISHED advert için public search/detail projection üretir. |
| Preconditions | Status PUBLISHED checked by caller. |
| Input summary | Advert ids; projection kind search|detail; optional user for favorite. |
| Output summary | Safe public DTOs conceptual. |
| Reads | `hrd_adverts`, `hrd_districts`, `hrd_provinces`, `hrd_categories`, `hrd_category_properties`, `hrd_horses`, `hrd_advert_media`, `hrd_media_assets`, `hrd_media_variants`; optional `hrd_favorites` |
| Writes | None |
| Transaction boundary | read-only batch |
| Concurrency | read-only |
| Idempotency | Idempotent |
| Side effects | None |
| Primary domain errors | `NOT_FOUND` |
| Notes | Internal fields sızdırmaz. Property code→public title/option yalnız public-visible `hrd_category_properties` ile. |

## 27. Transaction sınırı matrisi

| # | Owner use-case | Tables | Atomic changes | Rollback | External dependency placement |
| --- | --- | --- | --- | --- | --- |
| 1 | `AUTH-01 RegisterUser` | `hrd_users`, `hrd_one_time_credentials` | user + credential | all or nothing | Email provider **after** commit; **no** security event |
| 2 | `AUTH-02 VerifyRegistrationEmail` | `hrd_one_time_credentials`, `hrd_users` | consume + `email_verified_at` | all | `EMAIL_VERIFICATION` BEST_EFFORT_INDEPENDENT **after** commit |
| 3 | `AUTH-14 ConfirmEmailChange` | `hrd_one_time_credentials`, `hrd_users`, `hrd_security_events` | consume + email fields + EMAIL_CHANGE REQUIRED | all | None |
| 4 | `AUTH-11 ResetPassword` | `hrd_one_time_credentials`, `hrd_users`, `hrd_auth_sessions`, `hrd_security_events` | consume + password/stamp + revoke all + PASSWORD_RESET REQUIRED | all | None |
| 5 | `AUTH-12 ChangePassword` | `hrd_users`, `hrd_auth_sessions`, `hrd_security_events` | password/stamp + revoke others + rotate + PASSWORD_CHANGE REQUIRED | all | None |
| 6 | `AUTH-05 RefreshSession` | `hrd_auth_sessions`; replay also `hrd_security_events` | rotate/replace; replay → family revoke + REFRESH_REPLAY_DETECTED REQUIRED | all | None |
| 7 | `AUTH-04 Login` success | `hrd_users`, `hrd_auth_sessions` | failed_login reset + locked_until clear + session insert | all | LOGIN_SUCCESS best-effort after commit |
| 7b | `AUTH-04 Login` failure (user found) | `hrd_users` | failed_login_count (+ locked_until) | all | LOGIN_FAILURE best-effort after commit; no session |
| 8 | `ADVERT-OWNER-01 CreateAdvertDraft` | `hrd_adverts`, `hrd_advert_status_history` | DRAFT insert + direct initial history (NULL→DRAFT); not TransitionAdvertStatus | all | None |
| 9 | `INTERNAL-02 TransitionAdvertStatus` callers | `hrd_adverts`, `hrd_advert_status_history` | status/version (+published_at) + history | all | None |
| 10 | `ADVERT-OWNER-05 ChangeAdvertDraftCategory` | `hrd_adverts` | category + properties clear + version | all | None |
| 11a | `MEDIA-04 AttachMediaToAdvert` | `hrd_adverts`, `hrd_media_assets`, `hrd_advert_media` | asset lock + lifecycle recheck + insert + media_version++ | all | None |
| 11b | `MEDIA-05 DetachMediaFromAdvert` | `hrd_advert_media`, `hrd_adverts` | detach + media_version++ | all | None |
| 11c | `MEDIA-06 ReorderAdvertMedia` | `hrd_advert_media`, `hrd_adverts` | reorder + media_version++ | all | None |
| 11d | `MEDIA-07 SetAdvertCover` | `hrd_advert_media`, `hrd_adverts` | cover swap + media_version++ | all | None |
| 12 | `FAVORITE-01 AddFavorite` / `FAVORITE-02 RemoveFavorite` | `hrd_favorites` | insert/delete unique | conflict→idempotent | None |
| 13 | `BANNER-ADMIN-03 CreateBanner` | `hrd_media_assets`, `hrd_banners` | asset row lock + lifecycle recheck + insert INACTIVE v1 | all | No READY/dims required at create |
| 14 | `BANNER-ADMIN-05 SetBannerStatus` → ACTIVE | `hrd_banners`, `hrd_media_assets`, `hrd_media_variants` | READY recheck + ACTIVE + version++ same short TX | all | None |
| 15 | `BANNER-ADMIN-04 UpdateBanner` | `hrd_banners`, `hrd_media_assets`, `hrd_media_variants` | validate; asset change → asset lock + lifecycle; update + version | all | None |
| 16 | `MEDIA-02 ConfirmMediaUpload` / `MEDIA-ADMIN-02 ConfirmAdminMediaUpload` | `hrd_media_assets`, `hrd_background_jobs` | UPLOADED + MEDIA_VALIDATE_AND_NORMALIZE job same TX | all | Provider HEAD **before** TX |
| 17 | `MEDIA-WORKER-01 ValidateAndNormalizeMediaAsset` success | `hrd_media_assets`, `hrd_background_jobs` | MASTER_READY metadata + MEDIA_GENERATE_VARIANT jobs same TX | all | B2/decode/normalize/master **before** TX |
| 18 | `MEDIA-WORKER-01 ValidateAndNormalizeMediaAsset` failure | `hrd_media_assets` | VALIDATION_FAILED + reason; no variant jobs | all | Validation **before** TX |
| 19 | `MEDIA-WORKER-03 DeleteMediaObjects` DELETING transition | `hrd_media_assets`, `hrd_advert_media`, `hrd_banners` | asset lock + CLEANUP_CANDIDATE + ref recheck + DELETING | all | B2 physical delete **after** commit |
| 20 | `JOB-02 ClaimAvailableBackgroundJobs` | `hrd_background_jobs` | QUEUED→LEASED + lease + **attempt_count++** + version | per job | None |
| 21 | `JOB-04 CompleteBackgroundJob` | `hrd_background_jobs` | SUCCEEDED + completed_at + lease clear + version | all | None |
| 22 | `JOB-05 RetryOrFailBackgroundJob` | `hrd_background_jobs` | FAILED/QUEUED/DEAD branches; **no** attempt_count increment; lease clear + version | all | None |
| 23 | `JOB-07 RecoverExpiredBackgroundJobLeases` | `hrd_background_jobs` | CANCELLED / QUEUED retry / DEAD by cancel+attempt_count; lease clear + version | per job | None |
| 24 | `TJK-WORKER-04 ProcessTJKSyncBatch` items | `hrd_horses`, `hrd_tjk_sync_item_errors` | per-item upsert ± error | per-item TX | Adapter fetch **before** item TX |
| 25 | `TJK-WORKER-05 AdvanceTJKSyncCheckpoint` | `hrd_tjk_sync_runs`, `hrd_background_jobs` | checkpoint + counters + version + job SUCCEEDED/lease clear | all | None |
| 26 | Technical batch failure | `hrd_background_jobs` | checkpoint not advanced; `JOB-05 RetryOrFailBackgroundJob` | all | Prior committed horse rows preserved |
| 27 | `TJK-WORKER-06 FinalizeTJKSyncRun` | `hrd_tjk_sync_runs`, `hrd_background_jobs` | derive terminal from jobs+counters; no open QUEUED/LEASED; completed_at + version | all | Caller does not supply terminal status |
| 28 | `TJK-ADMIN-01 TriggerTJKSync` / `TJK-WORKER-01 TriggerScheduledTJKSync` | `hrd_tjk_sync_runs`, `hrd_background_jobs` | run insert + initial TJK_SYNC_BATCH job | all | None |
| 29 | `ADMIN-USER-03 ChangeUserStatus` | `hrd_users`, `hrd_auth_sessions`, `hrd_security_events` | expected status CAS + status + revoke on DISABLED/CLOSED + ACCOUNT_STATUS_CHANGE REQUIRED | all | None |
| 30 | `ADMIN-USER-04 ChangeUserRole` | `hrd_users`, `hrd_auth_sessions`, `hrd_security_events` | expected role CAS + role + revoke all + ROLE_CHANGE REQUIRED | all | None |

Media relation create (`MEDIA-04`, banner create/update asset change) ve `MEDIA-WORKER-03` aynı asset row-lock disiplinini kullanır. Best-effort security event, caller transaction’ıyla aynı transaction içinde yazılmaz.

## 28. Authorization ve ownership matrisi

| Domain operation | Anonymous | ACTIVE user | DISABLED/CLOSED | Admin FE/MOBILE | Admin ADMIN_BO | Worker/System |
| --- | --- | --- | --- | --- | --- | --- |
| Public search/detail | Allow | Allow (+favorite enrich) | Allow public only | Allow public | Allow public | — |
| Favorite add/list | Deny | Own only | Deny | Own only (not BO) | Deny as BO tool | — |
| Own advert edit/submit | Deny | Owner + status gates | Deny | Owner only if owns; no bypass | Deny ownership bypass | — |
| Media attach/reorder | Deny | Owner + DRAFT/CR | Deny | Owner only | Deny as owner bypass | — |
| Public category/geo/horse | Allow | Allow | Allow | Allow | Allow | — |
| User profile | Deny | Self | Deny | Self | Deny (use admin user ops) | — |
| BO user management | Deny | Deny | Deny | **Deny** | Allow | — |
| Category management | Deny | Deny | Deny | **Deny** | Allow | — |
| Moderation | Deny | Deny | Deny | **Deny** | Allow | — |
| Banner management | Deny | Deny | Deny | **Deny** | Allow | — |
| TJK trigger/cancel | Deny | Deny | Deny | **Deny** | Allow | Scheduled system trigger |
| Worker job claim | Deny | Deny | Deny | Deny | Deny | Allow |

## 29. Table coverage matrisi

| Table | Read functions | Write functions | Coverage |
| --- | --- | --- | --- |
| `hrd_users` | `AUTH-04 Login`; `AUTH-05 RefreshSession`; `ACCOUNT-01 GetMyProfile`; `ADMIN-USER-01 ListUsers`; `ADMIN-USER-02 GetUserAdminDetail`; `ADMIN-USER-05 ListUserSecurityEvents`; `ADVERT-OWNER-07 SubmitAdvertForReview`; `ADVERT-OWNER-08 ResubmitAdvertForReview`; `ADVERT-ADMIN-01 ListAdvertModerationQueue`; `ADVERT-ADMIN-02 GetAdvertModerationDetail`; `ADVERT-ADMIN-03 ApproveAdvert`; `INTERNAL-04 ValidateAdvertForSubmission`; `MEDIA-01 InitiateMediaUpload`; `MEDIA-ADMIN-01 InitiateAdminMediaUpload` | `AUTH-01 RegisterUser`; `AUTH-02 VerifyRegistrationEmail`; `AUTH-04 Login`; `AUTH-11 ResetPassword`; `AUTH-12 ChangePassword`; `AUTH-14 ConfirmEmailChange`; `ACCOUNT-02 UpdateMyProfile`; `ADMIN-USER-03 ChangeUserStatus`; `ADMIN-USER-04 ChangeUserRole` | Covered |
| `hrd_auth_sessions` | `AUTH-04 Login`; `AUTH-05 RefreshSession`; `AUTH-08 ListMySessions`; `AUTH-09 RevokeMySession`; `ADMIN-USER-02 GetUserAdminDetail` | `AUTH-04 Login`; `AUTH-05 RefreshSession`; `AUTH-06 LogoutCurrentSession`; `AUTH-07 LogoutAllSessions`; `AUTH-09 RevokeMySession`; `AUTH-11 ResetPassword`; `AUTH-12 ChangePassword`; `ADMIN-USER-03 ChangeUserStatus`; `ADMIN-USER-04 ChangeUserRole`; `INTERNAL-03 RevokeUserSessions` | Covered |
| `hrd_one_time_credentials` | `AUTH-01 RegisterUser`; `AUTH-02 VerifyRegistrationEmail`; `AUTH-03 ResendRegistrationEmailVerification`; `AUTH-10 RequestPasswordReset`; `AUTH-11 ResetPassword`; `AUTH-13 RequestEmailChange`; `AUTH-14 ConfirmEmailChange` | `AUTH-01 RegisterUser`; `AUTH-02 VerifyRegistrationEmail`; `AUTH-03 ResendRegistrationEmailVerification`; `AUTH-10 RequestPasswordReset`; `AUTH-11 ResetPassword`; `AUTH-13 RequestEmailChange`; `AUTH-14 ConfirmEmailChange` | Covered |
| `hrd_security_events` | `ADMIN-USER-05 ListUserSecurityEvents` | `INTERNAL-01 AppendSecurityEvent`; `AUTH-02 VerifyRegistrationEmail` (Conditional best-effort); `AUTH-04 Login` (Conditional best-effort); `AUTH-05 RefreshSession` (REQUIRED on replay); `AUTH-06 LogoutCurrentSession` (Conditional best-effort); `AUTH-07 LogoutAllSessions` (Conditional best-effort); `AUTH-09 RevokeMySession` (Conditional best-effort); `AUTH-11 ResetPassword` (REQUIRED); `AUTH-12 ChangePassword` (REQUIRED); `AUTH-14 ConfirmEmailChange` (REQUIRED); `ADMIN-USER-03 ChangeUserStatus` (REQUIRED); `ADMIN-USER-04 ChangeUserRole` (REQUIRED) | Covered |
| `hrd_provinces` | `GEO-01 ListActiveProvinces`; `GEO-02 SearchProvinces`; `GEO-03 ListDistrictsByProvince`; `GEO-04 SearchDistricts`; `ADVERT-PUBLIC-01 SearchPublishedAdverts`; `ADVERT-PUBLIC-02 GetPublishedAdvertDetail`; `ADVERT-OWNER-03 GetMyAdvert`; `ADVERT-ADMIN-02 GetAdvertModerationDetail`; `FAVORITE-03 ListMyFavorites`; `INTERNAL-06 ResolvePublicAdvertProjection` | Seed/migration-controlled; business function yok | Covered |
| `hrd_districts` | `GEO-03 ListDistrictsByProvince`; `GEO-04 SearchDistricts`; `ADVERT-PUBLIC-01 SearchPublishedAdverts`; `ADVERT-PUBLIC-02 GetPublishedAdvertDetail`; `ADVERT-OWNER-03 GetMyAdvert`; `ADVERT-OWNER-04 UpdateAdvertDraftDetails`; `ADVERT-OWNER-07 SubmitAdvertForReview`; `ADVERT-OWNER-08 ResubmitAdvertForReview`; `ADVERT-ADMIN-01 ListAdvertModerationQueue`; `ADVERT-ADMIN-02 GetAdvertModerationDetail`; `ADVERT-ADMIN-03 ApproveAdvert`; `FAVORITE-03 ListMyFavorites`; `INTERNAL-04 ValidateAdvertForSubmission`; `INTERNAL-06 ResolvePublicAdvertProjection` | Seed/migration-controlled; business function yok | Covered |
| `hrd_categories` | `CATALOG-01 GetPublicCategoryTree`; `CATALOG-02 GetCategoryFormDefinition`; `ADMIN-CATALOG-01 ListCategoriesAdmin`; `ADMIN-CATALOG-02 GetCategoryAdminDetail`; `ADMIN-CATALOG-08 ListCategoryPropertiesAdmin`; `ADVERT-PUBLIC-01 SearchPublishedAdverts`; `ADVERT-PUBLIC-02 GetPublishedAdvertDetail`; `ADVERT-OWNER-03 GetMyAdvert`; `ADVERT-OWNER-05 ChangeAdvertDraftCategory`; `ADVERT-OWNER-07 SubmitAdvertForReview`; `ADVERT-OWNER-08 ResubmitAdvertForReview`; `ADVERT-ADMIN-01 ListAdvertModerationQueue`; `ADVERT-ADMIN-02 GetAdvertModerationDetail`; `ADVERT-ADMIN-03 ApproveAdvert`; `FAVORITE-03 ListMyFavorites`; `INTERNAL-04 ValidateAdvertForSubmission`; `INTERNAL-05 ValidateDynamicProperties`; `INTERNAL-06 ResolvePublicAdvertProjection` | `ADMIN-CATALOG-03 CreateCategory`; `ADMIN-CATALOG-04 UpdateCategory`; `ADMIN-CATALOG-05 ReparentCategory`; `ADMIN-CATALOG-06 SetCategoryActive`; `ADMIN-CATALOG-07 ReorderCategories` | Covered |
| `hrd_category_properties` | `CATALOG-02 GetCategoryFormDefinition`; `ADMIN-CATALOG-02 GetCategoryAdminDetail`; `ADMIN-CATALOG-08 ListCategoryPropertiesAdmin`; `ADVERT-PUBLIC-01 SearchPublishedAdverts`; `ADVERT-PUBLIC-02 GetPublishedAdvertDetail`; `ADVERT-OWNER-03 GetMyAdvert`; `ADVERT-OWNER-06 ReplaceAdvertDynamicProperties`; `ADVERT-OWNER-07 SubmitAdvertForReview`; `ADVERT-OWNER-08 ResubmitAdvertForReview`; `ADVERT-ADMIN-02 GetAdvertModerationDetail`; `ADVERT-ADMIN-03 ApproveAdvert`; `INTERNAL-04 ValidateAdvertForSubmission`; `INTERNAL-05 ValidateDynamicProperties`; `INTERNAL-06 ResolvePublicAdvertProjection` | `ADMIN-CATALOG-09 CreateCategoryProperty`; `ADMIN-CATALOG-10 UpdateCategoryProperty`; `ADMIN-CATALOG-11 SetCategoryPropertyActive`; `ADMIN-CATALOG-12 ReorderCategoryProperties` | Covered |
| `hrd_horses` | `HORSE-01 SearchHorsesForSelection`; `HORSE-02 GetHorsePublicDetail`; `ADVERT-PUBLIC-01 SearchPublishedAdverts`; `ADVERT-PUBLIC-02 GetPublishedAdvertDetail`; `ADVERT-OWNER-03 GetMyAdvert`; `ADVERT-OWNER-04 UpdateAdvertDraftDetails`; `ADVERT-OWNER-07 SubmitAdvertForReview`; `ADVERT-OWNER-08 ResubmitAdvertForReview`; `ADVERT-ADMIN-02 GetAdvertModerationDetail`; `ADVERT-ADMIN-03 ApproveAdvert`; `FAVORITE-03 ListMyFavorites`; `TJK-WORKER-04 ProcessTJKSyncBatch`; `INTERNAL-04 ValidateAdvertForSubmission`; `INTERNAL-06 ResolvePublicAdvertProjection` | `TJK-WORKER-04 ProcessTJKSyncBatch` | Covered |
| `hrd_adverts` | `ADVERT-PUBLIC-01 SearchPublishedAdverts`; `ADVERT-PUBLIC-02 GetPublishedAdvertDetail`; `ADVERT-OWNER-02 ListMyAdverts`; `ADVERT-OWNER-03 GetMyAdvert`; `ADVERT-ADMIN-01 ListAdvertModerationQueue`; `ADVERT-ADMIN-02 GetAdvertModerationDetail`; `FAVORITE-01 AddFavorite`; `FAVORITE-03 ListMyFavorites`; `MEDIA-04 AttachMediaToAdvert`; `MEDIA-05 DetachMediaFromAdvert`; `MEDIA-06 ReorderAdvertMedia`; `MEDIA-07 SetAdvertCover`; `INTERNAL-02 TransitionAdvertStatus`; `INTERNAL-04 ValidateAdvertForSubmission`; `INTERNAL-06 ResolvePublicAdvertProjection` | `ADVERT-OWNER-01 CreateAdvertDraft`; `ADVERT-OWNER-04 UpdateAdvertDraftDetails`; `ADVERT-OWNER-05 ChangeAdvertDraftCategory`; `ADVERT-OWNER-06 ReplaceAdvertDynamicProperties`; `ADVERT-OWNER-07 SubmitAdvertForReview`; `ADVERT-OWNER-08 ResubmitAdvertForReview`; `ADVERT-OWNER-09 SoftDeleteAdvertDraft`; `ADVERT-OWNER-10 MarkAdvertSold`; `ADVERT-OWNER-11 ArchiveAdvert`; `ADVERT-ADMIN-03 ApproveAdvert`; `ADVERT-ADMIN-04 RequestAdvertChanges`; `ADVERT-ADMIN-05 RejectAdvert`; `ADVERT-ADMIN-06 SuspendAdvert`; `MEDIA-04 AttachMediaToAdvert`; `MEDIA-05 DetachMediaFromAdvert`; `MEDIA-06 ReorderAdvertMedia`; `MEDIA-07 SetAdvertCover`; `INTERNAL-02 TransitionAdvertStatus` | Covered |
| `hrd_advert_status_history` | `ADVERT-OWNER-03 GetMyAdvert`; `ADVERT-ADMIN-02 GetAdvertModerationDetail` | `ADVERT-OWNER-01 CreateAdvertDraft`; `INTERNAL-02 TransitionAdvertStatus` | Covered |
| `hrd_favorites` | `ADVERT-PUBLIC-01 SearchPublishedAdverts`; `ADVERT-PUBLIC-02 GetPublishedAdvertDetail`; `FAVORITE-01 AddFavorite`; `FAVORITE-02 RemoveFavorite`; `FAVORITE-03 ListMyFavorites`; `INTERNAL-06 ResolvePublicAdvertProjection` | `FAVORITE-01 AddFavorite`; `FAVORITE-02 RemoveFavorite` | Covered |
| `hrd_media_assets` | `ADVERT-PUBLIC-01 SearchPublishedAdverts`; `ADVERT-PUBLIC-02 GetPublishedAdvertDetail`; `ADVERT-OWNER-03 GetMyAdvert`; `ADVERT-OWNER-07 SubmitAdvertForReview`; `ADVERT-OWNER-08 ResubmitAdvertForReview`; `ADVERT-ADMIN-02 GetAdvertModerationDetail`; `ADVERT-ADMIN-03 ApproveAdvert`; `FAVORITE-03 ListMyFavorites`; `MEDIA-02 ConfirmMediaUpload`; `MEDIA-03 GetMediaProcessingStatus`; `MEDIA-04 AttachMediaToAdvert`; `MEDIA-ADMIN-02 ConfirmAdminMediaUpload`; `MEDIA-ADMIN-03 GetAdminMediaProcessingStatus`; `BANNER-PUBLIC-01 ListActiveBannersByPlacement`; `BANNER-ADMIN-02 GetBannerAdminDetail`; `BANNER-ADMIN-03 CreateBanner`; `BANNER-ADMIN-04 UpdateBanner`; `BANNER-ADMIN-05 SetBannerStatus`; `MEDIA-WORKER-01 ValidateAndNormalizeMediaAsset`; `MEDIA-WORKER-02 GenerateMediaVariant`; `MEDIA-WORKER-03 DeleteMediaObjects`; `MEDIA-WORKER-04 ReconcileMediaStorage`; `INTERNAL-04 ValidateAdvertForSubmission`; `INTERNAL-06 ResolvePublicAdvertProjection` | `MEDIA-01 InitiateMediaUpload`; `MEDIA-02 ConfirmMediaUpload`; `MEDIA-ADMIN-01 InitiateAdminMediaUpload`; `MEDIA-ADMIN-02 ConfirmAdminMediaUpload`; `MEDIA-WORKER-01 ValidateAndNormalizeMediaAsset`; `MEDIA-WORKER-03 DeleteMediaObjects`; `MEDIA-WORKER-04 ReconcileMediaStorage` | Covered |
| `hrd_media_variants` | `ADVERT-PUBLIC-01 SearchPublishedAdverts`; `ADVERT-PUBLIC-02 GetPublishedAdvertDetail`; `ADVERT-OWNER-03 GetMyAdvert`; `ADVERT-OWNER-07 SubmitAdvertForReview`; `ADVERT-OWNER-08 ResubmitAdvertForReview`; `ADVERT-ADMIN-02 GetAdvertModerationDetail`; `ADVERT-ADMIN-03 ApproveAdvert`; `FAVORITE-03 ListMyFavorites`; `MEDIA-03 GetMediaProcessingStatus`; `MEDIA-ADMIN-03 GetAdminMediaProcessingStatus`; `BANNER-PUBLIC-01 ListActiveBannersByPlacement`; `BANNER-ADMIN-02 GetBannerAdminDetail`; `BANNER-ADMIN-04 UpdateBanner`; `BANNER-ADMIN-05 SetBannerStatus`; `MEDIA-WORKER-02 GenerateMediaVariant`; `MEDIA-WORKER-03 DeleteMediaObjects`; `MEDIA-WORKER-04 ReconcileMediaStorage`; `INTERNAL-04 ValidateAdvertForSubmission`; `INTERNAL-06 ResolvePublicAdvertProjection` | `MEDIA-WORKER-02 GenerateMediaVariant`; `MEDIA-WORKER-03 DeleteMediaObjects` | Covered |
| `hrd_advert_media` | `ADVERT-PUBLIC-01 SearchPublishedAdverts`; `ADVERT-PUBLIC-02 GetPublishedAdvertDetail`; `ADVERT-OWNER-03 GetMyAdvert`; `ADVERT-OWNER-07 SubmitAdvertForReview`; `ADVERT-OWNER-08 ResubmitAdvertForReview`; `ADVERT-ADMIN-02 GetAdvertModerationDetail`; `ADVERT-ADMIN-03 ApproveAdvert`; `FAVORITE-03 ListMyFavorites`; `MEDIA-04 AttachMediaToAdvert`; `MEDIA-05 DetachMediaFromAdvert`; `MEDIA-06 ReorderAdvertMedia`; `MEDIA-07 SetAdvertCover`; `MEDIA-WORKER-03 DeleteMediaObjects`; `MEDIA-WORKER-04 ReconcileMediaStorage`; `INTERNAL-04 ValidateAdvertForSubmission`; `INTERNAL-06 ResolvePublicAdvertProjection` | `MEDIA-04 AttachMediaToAdvert`; `MEDIA-05 DetachMediaFromAdvert`; `MEDIA-06 ReorderAdvertMedia`; `MEDIA-07 SetAdvertCover` | Covered |
| `hrd_banners` | `BANNER-PUBLIC-01 ListActiveBannersByPlacement`; `BANNER-ADMIN-01 ListBannersAdmin`; `BANNER-ADMIN-02 GetBannerAdminDetail`; `MEDIA-WORKER-03 DeleteMediaObjects`; `MEDIA-WORKER-04 ReconcileMediaStorage` | `BANNER-ADMIN-03 CreateBanner`; `BANNER-ADMIN-04 UpdateBanner`; `BANNER-ADMIN-05 SetBannerStatus`; `BANNER-ADMIN-06 ReorderBanners` | Covered |
| `hrd_background_jobs` | `JOB-01 EnqueueBackgroundJob`; `JOB-02 ClaimAvailableBackgroundJobs`; `JOB-03 RenewBackgroundJobLease`; `JOB-04 CompleteBackgroundJob`; `JOB-05 RetryOrFailBackgroundJob`; `JOB-06 RequestBackgroundJobCancellation`; `JOB-07 RecoverExpiredBackgroundJobLeases`; `MEDIA-02 ConfirmMediaUpload`; `MEDIA-ADMIN-02 ConfirmAdminMediaUpload`; `MEDIA-WORKER-01 ValidateAndNormalizeMediaAsset`; `MEDIA-WORKER-04 ReconcileMediaStorage`; `TJK-ADMIN-02 CancelTJKSync`; `TJK-WORKER-03 PlanTJKSyncBatches`; `TJK-WORKER-05 AdvanceTJKSyncCheckpoint`; `TJK-WORKER-06 FinalizeTJKSyncRun` | `JOB-01 EnqueueBackgroundJob`; `JOB-02 ClaimAvailableBackgroundJobs`; `JOB-03 RenewBackgroundJobLease`; `JOB-04 CompleteBackgroundJob`; `JOB-05 RetryOrFailBackgroundJob`; `JOB-06 RequestBackgroundJobCancellation`; `JOB-07 RecoverExpiredBackgroundJobLeases`; `MEDIA-02 ConfirmMediaUpload`; `MEDIA-ADMIN-02 ConfirmAdminMediaUpload`; `MEDIA-WORKER-01 ValidateAndNormalizeMediaAsset`; `TJK-ADMIN-01 TriggerTJKSync`; `TJK-ADMIN-02 CancelTJKSync`; `TJK-WORKER-01 TriggerScheduledTJKSync`; `TJK-WORKER-03 PlanTJKSyncBatches`; `TJK-WORKER-05 AdvanceTJKSyncCheckpoint` | Covered |
| `hrd_tjk_sync_runs` | `JOB-01 EnqueueBackgroundJob`; `TJK-ADMIN-01 TriggerTJKSync`; `TJK-ADMIN-02 CancelTJKSync`; `TJK-ADMIN-03 ListTJKSyncRuns`; `TJK-ADMIN-04 GetTJKSyncRun`; `TJK-ADMIN-05 ListTJKSyncItemErrors`; `TJK-WORKER-01 TriggerScheduledTJKSync`; `TJK-WORKER-02 StartTJKSyncRun`; `TJK-WORKER-03 PlanTJKSyncBatches`; `TJK-WORKER-04 ProcessTJKSyncBatch`; `TJK-WORKER-05 AdvanceTJKSyncCheckpoint`; `TJK-WORKER-06 FinalizeTJKSyncRun`; `TJK-WORKER-07 ApplyTJKSyncCancellation` | `TJK-ADMIN-01 TriggerTJKSync`; `TJK-ADMIN-02 CancelTJKSync`; `TJK-WORKER-01 TriggerScheduledTJKSync`; `TJK-WORKER-02 StartTJKSyncRun`; `TJK-WORKER-05 AdvanceTJKSyncCheckpoint`; `TJK-WORKER-06 FinalizeTJKSyncRun`; `TJK-WORKER-07 ApplyTJKSyncCancellation` | Covered |
| `hrd_tjk_sync_item_errors` | `TJK-ADMIN-05 ListTJKSyncItemErrors`; `TJK-ADMIN-06 ResolveTJKSyncItemError`; `TJK-ADMIN-07 IgnoreTJKSyncItemError`; `TJK-WORKER-04 ProcessTJKSyncBatch` | `TJK-WORKER-04 ProcessTJKSyncBatch`; `TJK-ADMIN-06 ResolveTJKSyncItemError`; `TJK-ADMIN-07 IgnoreTJKSyncItemError` | Covered |

`hrd_schema_migrations`: business coverage dışı.

## 30. Açık ürün kararları (fonksiyon uydurma)

- Seller DISABLED/CLOSED iken PUBLISHED visibility
- Pasif category × PUBLISHED behavior
- Email verification’ın login vs publication/submit etkisi nüansları (submit’te EMAIL_NOT_VERIFIED kapısı varsayıldı)
- Unverified user draft create izni
- Telefon zorunluluğu
- Own-advert favorite
- Exact media minimum / title-description min
- Price requirement per category
- Public seller contact/profile fields
- Keyword search / similar adverts / favorite count
- Banner scheduling
- TJK retry single item
- ResumeSuspendedAdvert (SUSPENDED→PUBLISHED) — docs/08 geçse bile bu listede fonksiyon yok; onay sonrası eklenebilir
- Deleted draft restore; pending withdrawal; rejected re-apply
- SUSPENDED→ARCHIVED actor
- Exact session/token TTL; rate limits; retention süreleri
- Email-change sonrası session/stamp etkisi
- Son admin demote/disable koruması
- BO’nun user advert content/image edit’i

## 31. Phase-one dışı fonksiyonlar (red)

Payment processing; package purchase/entitlement; campaign boost/doping; article; contact; YKK; social login; MFA; notification package; view counter; similar advert engine; favorite count cache; Elasticsearch/OpenSearch management; user/admin manual TJK horse edit; generic raw SQL/JSONPath query; generic admin ownership bypass; GEO runtime mutation BO CRUD (seed-controlled); Resume/force-publish/withdraw/restore (onaysız).

## 32. Blueprint kabul kriterleri

1. Bütün phase-one domainler fonksiyonlarla kapsandı mı? **Evet**
2. Her function exact stable isim taşıyor mu? **Evet**
3. Her function exposure sınıfına sahip mi? **Evet**
4. Public/FE/BO/worker ayrımı açık mı? **Evet**
5. Admin role + ADMIN_BO birlikte zorunlu mu? **Evet** (BO_AUTH)
6. FE admin BO fonksiyonu kullanabiliyor mu? **Hayır**
7. Ownership server context’inden mi? **Evet**
8. DISABLED/CLOSED korumalı işlemlerden engelleniyor mu? **Evet**
9. Her function read/write tablolarını gösteriyor mu? **Evet**
10. Transaction sınırları tanımlı mı? **Evet** (§27)
11. Status transition + history atomik mi? **Evet** (INTERNAL-02)
12. Refresh rotation/replay atomik mi? **Evet**
13. Password reset session revoke içeriyor mu? **Evet**
14. Email change target credential’a bağlı mı? **Evet**
15. Category change property clear içeriyor mu? **Evet**
16. Advert partial draft korunuyor mu? **Evet**
17. Submission full validation yapıyor mu? **Evet**
18. Advert version ve media_version ayrılmış mı? **Evet**
19. Favorite idempotent mi? **Evet**
20. Media upload client credential sızdırmıyor mu? **Evet**
21. Raw/master/variant ayrımı korunuyor mu? **Evet**
22. Banner advert-media’dan ayrı mı? **Evet**
23. Durable job functionları var mı? **Evet**
24. FAILED/DEAD ayrımı korunuyor mu? **Evet**
25. TJK run/job/error ayrımı korunuyor mu? **Evet**
26. Checkpoint committed contiguous progress mi? **Evet**
27. Worker external çağrıları uzun DB transaction içinde mi? **Hayır** — dışında
28. Public projection internal alan sızdırıyor mu? **Hayır**
29. `hr_` tablo kullanımı var mı? **Hayır**
30. Payment/package fonksiyonu eklendi mi? **Hayır**
31. Endpoint/path/HTTP method üretildi mi? **Hayır**
32. Function listesi OpenAPI aşamasına hazır mı? **Evet**

33. Bütün function Reads/Writes exact `hrd_` tablo adları mı (wildcard/shorthand yok)? **Evet**
34. Domain error kodları exact taxonomy’den mi (wildcard error kodu yok)? **Evet**
35. Media UPLOADED/MASTER_READY + job enqueue aynı kısa TX atomik mi? **Evet**
36. TJK checkpoint + batch job SUCCEEDED aynı TX mi? **Evet** (`AdvanceTJKSyncCheckpoint`)
37. Persist edilmeyen media initiation idempotency key varsayılıyor mu? **Hayır** — Initiate* Non-idempotent
38. `CreateBanner` kesin INACTIVE mı? **Evet**
39. ACTIVE banner READY validation aynı TX snapshot içinde mi? **Evet** (`SetBannerStatus`)
40. Security event required vs best-effort politikası kesin mi? **Evet** (§5A)
41. Best-effort event caller TX ile aynı transaction’da yazılıyor mu? **Hayır** — yalnız BEST_EFFORT_INDEPENDENT ayrı kısa TX
42. Login `hrd_users` brute-force alanlarını (`failed_login_count`, `locked_until`) yazıyor mu? **Evet**
43. Initial DRAFT history normal TransitionAdvertStatus’tan ayrıldı mı? **Evet**
44. Ayrı durable email queue tablosu veya email job type varsayılıyor mu? **Hayır**

45. `AUTH-01 RegisterUser` yanlış `EMAIL_VERIFICATION` event yazıyor mu? **Hayır**
46. `FAVORITE-03 ListMyFavorites` Reads exact mi? **Evet**
47. Public advert property projection `hrd_category_properties` okuyor mu? **Evet**
48. NOT_FOUND üreten list function’lar parent entity’yi okuyor mu? **Evet** (`ADMIN-USER-05`, `ADMIN-CATALOG-08`, `TJK-ADMIN-05`)
49. `CreateBanner` processing asset’i INACTIVE bağlayabiliyor mu? **Evet**
50. `CreateBanner` READY validation’ı activation’a bırakıyor mu? **Evet**
51. Relation insert ve asset delete aynı asset lock disiplinini kullanıyor mu? **Evet**
52. Crashed worker attempt bütçesini (claim-time `attempt_count++`) tüketiyor mu? **Evet**
53. Expired lease sonsuz QUEUED döngüsü yaratıyor mu? **Hayır**
54. `FinalizeTJKSyncRun` açık QUEUED/LEASED job varken terminal olabilir mi? **Hayır**
55. Run terminal status caller input’undan mı geliyor? **Hayır** — DB’den türetilir
56. Table coverage exact function ID/name kullanıyor mu? **Evet**
57. Exact olmayan banned ifade dosyada kalmış mı? **Hayır**

## 33. Sonraki adım

Bu blueprint kabul edilirse:

1. Use-case → API operation mapping
2. Public/FE/BO endpoint listesi
3. Worker internal operation mapping
4. Request/response DTO kararları
5. Error → HTTP status mapping
6. OpenAPI 3.x sözleşmesi
7. Migration tool seçimi
8. SQL migration dosyaları
9. Go project bootstrap
10. Modül modül backend geliştirme
