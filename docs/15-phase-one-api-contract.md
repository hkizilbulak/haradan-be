# Haradan Phase-One API Contract

## 1. Amaç ve source of truth

- HTTP sözleşmesi: `api/openapi.yaml` (OpenAPI 3.0.3)
- Use-case kaynağı: `docs/14-phase-one-function-use-case-blueprint.md`
- Veri modeli: `docs/13-phase-one-database-blueprint.md`
- Bu belge 88 dış HTTP operation ile 24 internal non-HTTP function eşlemesini kilitler.
- Endpoint/path burada özetlenir; makine sözleşmesi OpenAPI’dedir.

## 2. API versioning / naming

| Kural | Değer |
| --- | --- |
| OpenAPI | 3.0.3 |
| Title | Haradan API |
| Version | 0.1.0 |
| Server | `/api` |
| Business paths | `/v1/...` |
| Health | `/health` |
| JSON fields | lowerCamelCase |
| Schemas | PascalCase |
| UUID | string/uuid |
| Timestamp | string/date-time |
| Money | `Money` pair: required non-null `amountMinor` + `currency` (`^[A-Z]{3}$`); parent `price` may be `null` |

## 3. Auth / exposure modeli

| Exposure | Count | Security |
| --- | ---: | --- |
| PUBLIC | 20 | Yok veya optional Bearer (favorite enrichment) |
| FE_AUTH | 29 | BearerAuth zorunlu; context PUBLIC_WEB/MOBILE |
| BO_AUTH | 39 | BearerAuth + role=`admin` + context=`ADMIN_BO` |
| INTERNAL_* | 24 | HTTP yok |

- Access token kısa ömürlüdür; refresh plaintext yalnız response’tadır.
- Client role/owner/actor güvenilir değildir.
- FE/MOBILE admin session BO path kullanamaz.
- Optional auth: `SearchPublishedAdverts`, `GetPublishedAdvertDetail`.

## 4. Shared request/response conventions

- Response objects: always-present fields are `required`; empty-but-present values use `required` + `nullable: true`; omit truly optional fields from `required`
- Typed response objects use `additionalProperties: false` except controlled maps (advert properties, category validation/UI metadata, safe security metadata)
- OpenAPI 3.0 nullable `$ref`: never put siblings next to `$ref`; use `nullable: true` + `allOf: [{ $ref }]` (defaults similarly via `allOf` + `default`)
- `ErrorResponse` required: `code` (`DomainErrorCode`), `message`, `traceId` (minLength 1); optional `details` / `fieldErrors`; `additionalProperties: false`
- Cursor lists: `items`, `nextCursor`, `hasMore`; limit default 20 max 100
- Success mutations return updated resource projection where applicable
- Confirm media uploads: `202 Accepted`
- Trigger TJK: `202 Accepted`
- Public detail: `category` → `PublicCategorySummary`, `location` → `PublicLocationSummary`
- Upload initiate: `UploadAuthorization` + `UploadConstraints`; single canonical expiry at `upload.expiresAt`
- Status/eventType query filters are exact enums (`AdvertStatus`, `UserStatus`, `BannerStatus`, `TJKSyncRunStatus`, `TJKSyncItemErrorStatus`, `SecurityEventType`)

## 5. Error → HTTP mapping

| Domain error | HTTP |
| --- | --- |
| VALIDATION_ERROR | 422 |
| UNAUTHENTICATED | 401 |
| FORBIDDEN | 403 |
| ACCOUNT_INACTIVE | 403 |
| EMAIL_NOT_VERIFIED | 403 |
| OWNERSHIP_REQUIRED | 403 |
| NOT_FOUND | 404 |
| CONFLICT | 409 |
| STALE_VERSION | 409 |
| INVALID_STATE | 409 |
| DUPLICATE | 409 (idempotent paths may return success) |
| TOKEN_INVALID / TOKEN_EXPIRED | 400 for one-time tokens; 401 for refresh credential |
| TOKEN_ALREADY_USED | 409 |
| SESSION_REVOKED | 401 |
| REFRESH_REPLAY_DETECTED | 401 |
| RATE_LIMITED | 429 |
| DEPENDENCY_UNAVAILABLE | 503 |
| PROCESSING_NOT_READY | 409 |
| QUERY_TOO_COMPLEX | 422 |
| INTERNAL_ERROR | 500 |

- Operation’lar yalnız üretebileceği HTTP response’ları tanımlar.
- Her operation machine-readable `x-error-codes` taşır: HTTP status → exact `DomainErrorCode` listesi (yalnız o status’ta üretilebilen kodlar).
- `500` → `INTERNAL_ERROR`; BO/FE auth `401`/`403` baseline docs/14 + exposure ile hizalıdır.

## 6. Pagination / cursor

- Opaque cursor; filter/sort fingerprint-bound for public advert search
- Unsupported cursor → 422
- Exact total count zorunlu değil
- Default advert sort: publishedAt DESC, id DESC
- Dynamic property filters yalnız leaf category

## 7. Concurrency / version modeli

- No ETag/If-Match
- Body: `expectedVersion` / `expectedMediaVersion`
- Admin user CAS: `expectedCurrentStatus` / `expectedCurrentRole` (no user.version)
- Stale → 409 `STALE_VERSION`
- Responses expose `version` and `mediaVersion` where relevant

## 8. 88-operation traceability

| Function ID | Operation ID | Exposure | Method | Path | Auth | Request schema | Success | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `SYS-01` | `GetHealth` | `PUBLIC` | `GET` | `/health` | None | `—` | `200 HealthResponse` | No secrets; `503 DEPENDENCY_UNAVAILABLE` |
| `AUTH-01` | `RegisterUser` | `PUBLIC` | `POST` | `/v1/auth/register` | None | `RegisterUserRequest` | `201 GenericAuthMessageResponse` | Enumeration-safe; no HTTP 409 for duplicate email |
| `AUTH-02` | `VerifyRegistrationEmail` | `PUBLIC` | `POST` | `/v1/auth/verify-email` | None | `TokenRequest` | `200 GenericAuthMessageResponse` | Token: 400 invalid/expired, 409 already used; no 401/404 |
| `AUTH-03` | `ResendRegistrationEmailVerification` | `PUBLIC` | `POST` | `/v1/auth/resend-verification` | None | `EmailRequest` | `200 GenericAuthMessageResponse` | Enumeration-safe |
| `AUTH-04` | `Login` | `PUBLIC` | `POST` | `/v1/auth/login` | None | `LoginRequest` | `200 AuthTokenResponse` | clientContext server-validated |
| `AUTH-05` | `RefreshSession` | `PUBLIC` | `POST` | `/v1/auth/refresh` | None | `RefreshSessionRequest` | `200 AuthTokenResponse` | Rotation; replay→401 |
| `AUTH-10` | `RequestPasswordReset` | `PUBLIC` | `POST` | `/v1/auth/password/forgot` | None | `EmailRequest` | `200 GenericAuthMessageResponse` | Enumeration-safe |
| `AUTH-11` | `ResetPassword` | `PUBLIC` | `POST` | `/v1/auth/password/reset` | None | `ResetPasswordRequest` | `200 GenericAuthMessageResponse` | Token 400/409; revokes all sessions |
| `AUTH-14` | `ConfirmEmailChange` | `PUBLIC` | `POST` | `/v1/auth/email/confirm` | None | `TokenRequest` | `200 GenericAuthMessageResponse` | Token 400/409; no access-session 401 |
| `GEO-01` | `ListActiveProvinces` | `PUBLIC` | `GET` | `/v1/provinces` | None | `—` | `200 ProvinceListResponse` | — |
| `GEO-02` | `SearchProvinces` | `PUBLIC` | `GET` | `/v1/provinces/search` | None | `—` | `200 ProvinceListResponse` | q prefix |
| `GEO-03` | `ListDistrictsByProvince` | `PUBLIC` | `GET` | `/v1/provinces/{provinceId}/districts` | None | `—` | `200 DistrictListResponse` | — |
| `GEO-04` | `SearchDistricts` | `PUBLIC` | `GET` | `/v1/districts/search` | None | `—` | `200 DistrictListResponse` | optional provinceId |
| `CATALOG-01` | `GetPublicCategoryTree` | `PUBLIC` | `GET` | `/v1/categories` | None | `—` | `200 CategoryTreeResponse` | — |
| `CATALOG-02` | `GetCategoryFormDefinition` | `PUBLIC` | `GET` | `/v1/categories/{categoryId}/form` | None | `—` | `200 CategoryFormDefinitionResponse` | — |
| `HORSE-01` | `SearchHorsesForSelection` | `PUBLIC` | `GET` | `/v1/horses` | None | `—` | `200 HorseSelectionListResponse` | prefix/tjkNumber |
| `HORSE-02` | `GetHorsePublicDetail` | `PUBLIC` | `GET` | `/v1/horses/{horseId}` | None | `—` | `200 HorsePublicDetailResponse` | No raw payload |
| `ADVERT-PUBLIC-01` | `SearchPublishedAdverts` | `PUBLIC` | `GET` | `/v1/adverts` | Optional Bearer | `—` | `200 PublishedAdvertSearchResponse` | Optional auth favorite |
| `ADVERT-PUBLIC-02` | `GetPublishedAdvertDetail` | `PUBLIC` | `GET` | `/v1/adverts/{advertId}` | Optional Bearer | `—` | `200 PublishedAdvertDetailResponse` | Typed category/location; optional favorite |
| `BANNER-PUBLIC-01` | `ListActiveBannersByPlacement` | `PUBLIC` | `GET` | `/v1/banners` | None | `—` | `200 ActiveBannerListResponse` | placement required |
| `AUTH-06` | `LogoutCurrentSession` | `FE_AUTH` | `POST` | `/v1/auth/logout` | Bearer | `—` | `200 GenericAuthMessageResponse` | Idempotent |
| `AUTH-07` | `LogoutAllSessions` | `FE_AUTH` | `POST` | `/v1/auth/logout-all` | Bearer | `—` | `200 GenericAuthMessageResponse` | — |
| `AUTH-08` | `ListMySessions` | `FE_AUTH` | `GET` | `/v1/me/sessions` | Bearer | `—` | `200 SessionListResponse` | No hashes |
| `AUTH-09` | `RevokeMySession` | `FE_AUTH` | `DELETE` | `/v1/me/sessions/{sessionId}` | Bearer | `—` | `200 GenericAuthMessageResponse` | Idempotent |
| `AUTH-12` | `ChangePassword` | `FE_AUTH` | `POST` | `/v1/me/password` | Bearer | `ChangePasswordRequest` | `200 GenericAuthMessageResponse` | — |
| `AUTH-13` | `RequestEmailChange` | `FE_AUTH` | `POST` | `/v1/me/email/change-request` | Bearer | `RequestEmailChangeRequest` | `200 GenericAuthMessageResponse` | — |
| `ACCOUNT-01` | `GetMyProfile` | `FE_AUTH` | `GET` | `/v1/me` | Bearer | `—` | `200 MyProfileResponse` | — |
| `ACCOUNT-02` | `UpdateMyProfile` | `FE_AUTH` | `PATCH` | `/v1/me` | Bearer | `UpdateMyProfileRequest` | `200 MyProfileResponse` | No email/role/status |
| `ADVERT-OWNER-01` | `CreateAdvertDraft` | `FE_AUTH` | `POST` | `/v1/me/adverts` | Bearer | `CreateAdvertDraftRequest` | `201 OwnerAdvertResponse` | Partial OK |
| `ADVERT-OWNER-02` | `ListMyAdverts` | `FE_AUTH` | `GET` | `/v1/me/adverts` | Bearer | `—` | `200 OwnerAdvertListResponse` | Soft-deleted excluded by default |
| `ADVERT-OWNER-03` | `GetMyAdvert` | `FE_AUTH` | `GET` | `/v1/me/adverts/{advertId}` | Bearer | `—` | `200 OwnerAdvertResponse` | — |
| `ADVERT-OWNER-04` | `UpdateAdvertDraftDetails` | `FE_AUTH` | `PATCH` | `/v1/me/adverts/{advertId}` | Bearer | `UpdateAdvertDraftDetailsRequest` | `200 OwnerAdvertResponse` | expectedVersion |
| `ADVERT-OWNER-05` | `ChangeAdvertDraftCategory` | `FE_AUTH` | `PUT` | `/v1/me/adverts/{advertId}/category` | Bearer | `ChangeAdvertDraftCategoryRequest` | `200 OwnerAdvertResponse` | DRAFT only; clears properties |
| `ADVERT-OWNER-06` | `ReplaceAdvertDynamicProperties` | `FE_AUTH` | `PUT` | `/v1/me/adverts/{advertId}/properties` | Bearer | `ReplaceAdvertDynamicPropertiesRequest` | `200 OwnerAdvertResponse` | expectedVersion |
| `ADVERT-OWNER-07` | `SubmitAdvertForReview` | `FE_AUTH` | `POST` | `/v1/me/adverts/{advertId}/submit` | Bearer | `ExpectedVersionRequest` | `200 OwnerAdvertResponse` | DRAFT→PENDING_REVIEW |
| `ADVERT-OWNER-08` | `ResubmitAdvertForReview` | `FE_AUTH` | `POST` | `/v1/me/adverts/{advertId}/resubmit` | Bearer | `ExpectedVersionRequest` | `200 OwnerAdvertResponse` | CHANGES_REQUESTED→PENDING_REVIEW |
| `ADVERT-OWNER-09` | `SoftDeleteAdvertDraft` | `FE_AUTH` | `DELETE` | `/v1/me/adverts/{advertId}` | Bearer | `—` | `200 OwnerAdvertResponse` | expectedVersion query |
| `ADVERT-OWNER-10` | `MarkAdvertSold` | `FE_AUTH` | `POST` | `/v1/me/adverts/{advertId}/sold` | Bearer | `ExpectedVersionRequest` | `200 OwnerAdvertResponse` | PUBLISHED→SOLD |
| `ADVERT-OWNER-11` | `ArchiveAdvert` | `FE_AUTH` | `POST` | `/v1/me/adverts/{advertId}/archive` | Bearer | `ExpectedVersionRequest` | `200 OwnerAdvertResponse` | PUBLISHED→ARCHIVED |
| `FAVORITE-01` | `AddFavorite` | `FE_AUTH` | `PUT` | `/v1/me/favorites/{advertId}` | Bearer | `—` | `200 FavoriteMutationResponse` | Idempotent |
| `FAVORITE-02` | `RemoveFavorite` | `FE_AUTH` | `DELETE` | `/v1/me/favorites/{advertId}` | Bearer | `—` | `200 FavoriteMutationResponse` | Idempotent |
| `FAVORITE-03` | `ListMyFavorites` | `FE_AUTH` | `GET` | `/v1/me/favorites` | Bearer | `—` | `200 FavoriteListResponse` | Placeholder for non-public |
| `MEDIA-01` | `InitiateMediaUpload` | `FE_AUTH` | `POST` | `/v1/media/uploads` | Bearer | `InitiateMediaUploadRequest` | `201 InitiateMediaUploadResponse` | Typed upload+constraints; no permanent credentials |
| `MEDIA-02` | `ConfirmMediaUpload` | `FE_AUTH` | `POST` | `/v1/media/assets/{assetId}/confirm` | Bearer | `—` | `202 MediaProcessingStatusResponse` | Idempotent; durable validate enqueued |
| `MEDIA-03` | `GetMediaProcessingStatus` | `FE_AUTH` | `GET` | `/v1/media/assets/{assetId}` | Bearer | `—` | `200 MediaProcessingStatusResponse` | No object keys |
| `MEDIA-04` | `AttachMediaToAdvert` | `FE_AUTH` | `POST` | `/v1/me/adverts/{advertId}/media` | Bearer | `AttachMediaToAdvertRequest` | `200 AdvertMediaCollectionResponse` | expectedMediaVersion |
| `MEDIA-05` | `DetachMediaFromAdvert` | `FE_AUTH` | `DELETE` | `/v1/me/adverts/{advertId}/media/{assetId}` | Bearer | `—` | `200 AdvertMediaCollectionResponse` | expectedMediaVersion query |
| `MEDIA-06` | `ReorderAdvertMedia` | `FE_AUTH` | `PUT` | `/v1/me/adverts/{advertId}/media/order` | Bearer | `ReorderAdvertMediaRequest` | `200 AdvertMediaCollectionResponse` | — |
| `MEDIA-07` | `SetAdvertCover` | `FE_AUTH` | `PUT` | `/v1/me/adverts/{advertId}/media/cover` | Bearer | `SetAdvertCoverRequest` | `200 AdvertMediaCollectionResponse` | — |
| `ADMIN-USER-01` | `ListUsers` | `BO_AUTH` | `GET` | `/v1/admin/users` | Bearer | `—` | `200 AdminUserListResponse` | — |
| `ADMIN-USER-02` | `GetUserAdminDetail` | `BO_AUTH` | `GET` | `/v1/admin/users/{userId}` | Bearer | `—` | `200 AdminUserDetailResponse` | No password |
| `ADMIN-USER-03` | `ChangeUserStatus` | `BO_AUTH` | `POST` | `/v1/admin/users/{userId}/status` | Bearer | `ChangeUserStatusRequest` | `200 AdminUserDetailResponse` | expectedCurrentStatus CAS |
| `ADMIN-USER-04` | `ChangeUserRole` | `BO_AUTH` | `POST` | `/v1/admin/users/{userId}/role` | Bearer | `ChangeUserRoleRequest` | `200 AdminUserDetailResponse` | expectedCurrentRole CAS |
| `ADMIN-USER-05` | `ListUserSecurityEvents` | `BO_AUTH` | `GET` | `/v1/admin/users/{userId}/security-events` | Bearer | `—` | `200 SecurityEventListResponse` | — |
| `ADMIN-CATALOG-01` | `ListCategoriesAdmin` | `BO_AUTH` | `GET` | `/v1/admin/categories` | Bearer | `—` | `200 AdminCategoryListResponse` | — |
| `ADMIN-CATALOG-02` | `GetCategoryAdminDetail` | `BO_AUTH` | `GET` | `/v1/admin/categories/{categoryId}` | Bearer | `—` | `200 AdminCategoryDetailResponse` | — |
| `ADMIN-CATALOG-03` | `CreateCategory` | `BO_AUTH` | `POST` | `/v1/admin/categories` | Bearer | `CreateCategoryRequest` | `201 AdminCategoryDetailResponse` | — |
| `ADMIN-CATALOG-04` | `UpdateCategory` | `BO_AUTH` | `PATCH` | `/v1/admin/categories/{categoryId}` | Bearer | `UpdateCategoryRequest` | `200 AdminCategoryDetailResponse` | expectedVersion |
| `ADMIN-CATALOG-05` | `ReparentCategory` | `BO_AUTH` | `POST` | `/v1/admin/categories/{categoryId}/reparent` | Bearer | `ReparentCategoryRequest` | `200 AdminCategoryDetailResponse` | — |
| `ADMIN-CATALOG-06` | `SetCategoryActive` | `BO_AUTH` | `POST` | `/v1/admin/categories/{categoryId}/active` | Bearer | `SetActiveRequest` | `200 AdminCategoryDetailResponse` | — |
| `ADMIN-CATALOG-07` | `ReorderCategories` | `BO_AUTH` | `PUT` | `/v1/admin/categories/reorder` | Bearer | `ReorderCategoriesRequest` | `200 SuccessMessageResponse` | — |
| `ADMIN-CATALOG-08` | `ListCategoryPropertiesAdmin` | `BO_AUTH` | `GET` | `/v1/admin/categories/{categoryId}/properties` | Bearer | `—` | `200 AdminCategoryPropertyListResponse` | — |
| `ADMIN-CATALOG-09` | `CreateCategoryProperty` | `BO_AUTH` | `POST` | `/v1/admin/categories/{categoryId}/properties` | Bearer | `CreateCategoryPropertyRequest` | `201 AdminCategoryPropertyResponse` | — |
| `ADMIN-CATALOG-10` | `UpdateCategoryProperty` | `BO_AUTH` | `PATCH` | `/v1/admin/categories/{categoryId}/properties/{propertyId}` | Bearer | `UpdateCategoryPropertyRequest` | `200 AdminCategoryPropertyResponse` | — |
| `ADMIN-CATALOG-11` | `SetCategoryPropertyActive` | `BO_AUTH` | `POST` | `/v1/admin/categories/{categoryId}/properties/{propertyId}/active` | Bearer | `SetActiveRequest` | `200 AdminCategoryPropertyResponse` | — |
| `ADMIN-CATALOG-12` | `ReorderCategoryProperties` | `BO_AUTH` | `PUT` | `/v1/admin/categories/{categoryId}/properties/reorder` | Bearer | `ReorderCategoryPropertiesRequest` | `200 SuccessMessageResponse` | — |
| `ADVERT-ADMIN-01` | `ListAdvertModerationQueue` | `BO_AUTH` | `GET` | `/v1/admin/adverts/moderation` | Bearer | `—` | `200 ModerationQueueResponse` | — |
| `ADVERT-ADMIN-02` | `GetAdvertModerationDetail` | `BO_AUTH` | `GET` | `/v1/admin/adverts/{advertId}` | Bearer | `—` | `200 ModerationAdvertDetailResponse` | — |
| `ADVERT-ADMIN-03` | `ApproveAdvert` | `BO_AUTH` | `POST` | `/v1/admin/adverts/{advertId}/approve` | Bearer | `ExpectedVersionRequest` | `200 ModerationAdvertDetailResponse` | — |
| `ADVERT-ADMIN-04` | `RequestAdvertChanges` | `BO_AUTH` | `POST` | `/v1/admin/adverts/{advertId}/request-changes` | Bearer | `ModerationReasonRequest` | `200 ModerationAdvertDetailResponse` | reason required |
| `ADVERT-ADMIN-05` | `RejectAdvert` | `BO_AUTH` | `POST` | `/v1/admin/adverts/{advertId}/reject` | Bearer | `ModerationReasonRequest` | `200 ModerationAdvertDetailResponse` | reason required |
| `ADVERT-ADMIN-06` | `SuspendAdvert` | `BO_AUTH` | `POST` | `/v1/admin/adverts/{advertId}/suspend` | Bearer | `ModerationReasonRequest` | `200 ModerationAdvertDetailResponse` | reason required |
| `MEDIA-ADMIN-01` | `InitiateAdminMediaUpload` | `BO_AUTH` | `POST` | `/v1/admin/media/uploads` | Bearer | `InitiateMediaUploadRequest` | `201 InitiateMediaUploadResponse` | — |
| `MEDIA-ADMIN-02` | `ConfirmAdminMediaUpload` | `BO_AUTH` | `POST` | `/v1/admin/media/assets/{assetId}/confirm` | Bearer | `—` | `202 MediaProcessingStatusResponse` | — |
| `MEDIA-ADMIN-03` | `GetAdminMediaProcessingStatus` | `BO_AUTH` | `GET` | `/v1/admin/media/assets/{assetId}` | Bearer | `—` | `200 MediaProcessingStatusResponse` | — |
| `BANNER-ADMIN-01` | `ListBannersAdmin` | `BO_AUTH` | `GET` | `/v1/admin/banners` | Bearer | `—` | `200 AdminBannerListResponse` | placement optional filter |
| `BANNER-ADMIN-02` | `GetBannerAdminDetail` | `BO_AUTH` | `GET` | `/v1/admin/banners/{bannerId}` | Bearer | `—` | `200 AdminBannerDetailResponse` | — |
| `BANNER-ADMIN-03` | `CreateBanner` | `BO_AUTH` | `POST` | `/v1/admin/banners` | Bearer | `CreateBannerRequest` | `201 AdminBannerDetailResponse` | Always INACTIVE v1; no status in request |
| `BANNER-ADMIN-04` | `UpdateBanner` | `BO_AUTH` | `PATCH` | `/v1/admin/banners/{bannerId}` | Bearer | `UpdateBannerRequest` | `200 AdminBannerDetailResponse` | expectedVersion |
| `BANNER-ADMIN-05` | `SetBannerStatus` | `BO_AUTH` | `POST` | `/v1/admin/banners/{bannerId}/status` | Bearer | `SetBannerStatusRequest` | `200 AdminBannerDetailResponse` | ACTIVE requires READY |
| `BANNER-ADMIN-06` | `ReorderBanners` | `BO_AUTH` | `PUT` | `/v1/admin/banners/reorder` | Bearer | `ReorderBannersRequest` | `200 SuccessMessageResponse` | — |
| `TJK-ADMIN-01` | `TriggerTJKSync` | `BO_AUTH` | `POST` | `/v1/admin/tjk/sync-runs` | Bearer | `TriggerTJKSyncRequest` | `202 TJKSyncRunResponse` | scope HORSES; queued |
| `TJK-ADMIN-02` | `CancelTJKSync` | `BO_AUTH` | `POST` | `/v1/admin/tjk/sync-runs/{runId}/cancel` | Bearer | `ExpectedVersionRequest` | `200 TJKSyncRunResponse` | Cooperative |
| `TJK-ADMIN-03` | `ListTJKSyncRuns` | `BO_AUTH` | `GET` | `/v1/admin/tjk/sync-runs` | Bearer | `—` | `200 TJKSyncRunListResponse` | — |
| `TJK-ADMIN-04` | `GetTJKSyncRun` | `BO_AUTH` | `GET` | `/v1/admin/tjk/sync-runs/{runId}` | Bearer | `—` | `200 TJKSyncRunResponse` | No raw payload |
| `TJK-ADMIN-05` | `ListTJKSyncItemErrors` | `BO_AUTH` | `GET` | `/v1/admin/tjk/sync-runs/{runId}/item-errors` | Bearer | `—` | `200 TJKSyncItemErrorListResponse` | — |
| `TJK-ADMIN-06` | `ResolveTJKSyncItemError` | `BO_AUTH` | `POST` | `/v1/admin/tjk/item-errors/{errorId}/resolve` | Bearer | `—` | `200 TJKSyncItemErrorResponse` | — |
| `TJK-ADMIN-07` | `IgnoreTJKSyncItemError` | `BO_AUTH` | `POST` | `/v1/admin/tjk/item-errors/{errorId}/ignore` | Bearer | `—` | `200 TJKSyncItemErrorResponse` | — |

## 9. 24 internal non-HTTP functions

| Function ID | Function name | Exposure | HTTP exposure | Called by |
| --- | --- | --- | --- | --- |
| `JOB-01` | `EnqueueBackgroundJob` | `INTERNAL_SYSTEM` | `None` | Callers: TriggerTJKSync, ConfirmMediaUpload, PlanTJKSyncBatches, media workers |
| `JOB-02` | `ClaimAvailableBackgroundJobs` | `INTERNAL_WORKER` | `None` | Worker process |
| `JOB-03` | `RenewBackgroundJobLease` | `INTERNAL_WORKER` | `None` | Worker process |
| `JOB-04` | `CompleteBackgroundJob` | `INTERNAL_WORKER` | `None` | Worker / AdvanceTJKSyncCheckpoint |
| `JOB-05` | `RetryOrFailBackgroundJob` | `INTERNAL_WORKER` | `None` | Worker process |
| `JOB-06` | `RequestBackgroundJobCancellation` | `INTERNAL_SYSTEM` | `None` | CancelTJKSync orchestration |
| `JOB-07` | `RecoverExpiredBackgroundJobLeases` | `INTERNAL_WORKER` | `None` | Worker/scheduler |
| `MEDIA-WORKER-01` | `ValidateAndNormalizeMediaAsset` | `INTERNAL_WORKER` | `None` | Job handler |
| `MEDIA-WORKER-02` | `GenerateMediaVariant` | `INTERNAL_WORKER` | `None` | Job handler |
| `MEDIA-WORKER-03` | `DeleteMediaObjects` | `INTERNAL_WORKER` | `None` | Job handler |
| `MEDIA-WORKER-04` | `ReconcileMediaStorage` | `INTERNAL_WORKER` | `None` | Job handler |
| `TJK-WORKER-01` | `TriggerScheduledTJKSync` | `INTERNAL_SYSTEM` | `None` | Scheduler |
| `TJK-WORKER-02` | `StartTJKSyncRun` | `INTERNAL_WORKER` | `None` | TJK worker |
| `TJK-WORKER-03` | `PlanTJKSyncBatches` | `INTERNAL_WORKER` | `None` | TJK worker |
| `TJK-WORKER-04` | `ProcessTJKSyncBatch` | `INTERNAL_WORKER` | `None` | TJK worker |
| `TJK-WORKER-05` | `AdvanceTJKSyncCheckpoint` | `INTERNAL_WORKER` | `None` | TJK worker |
| `TJK-WORKER-06` | `FinalizeTJKSyncRun` | `INTERNAL_WORKER` | `None` | TJK worker |
| `TJK-WORKER-07` | `ApplyTJKSyncCancellation` | `INTERNAL_WORKER` | `None` | TJK worker |
| `INTERNAL-01` | `AppendSecurityEvent` | `INTERNAL_SYSTEM` | `None` | Auth/admin callers |
| `INTERNAL-02` | `TransitionAdvertStatus` | `INTERNAL_SYSTEM` | `None` | Owner/admin lifecycle callers |
| `INTERNAL-03` | `RevokeUserSessions` | `INTERNAL_SYSTEM` | `None` | Password/status/role callers |
| `INTERNAL-04` | `ValidateAdvertForSubmission` | `INTERNAL_SYSTEM` | `None` | Submit/resubmit/approve |
| `INTERNAL-05` | `ValidateDynamicProperties` | `INTERNAL_SYSTEM` | `None` | Property replace/submit |
| `INTERNAL-06` | `ResolvePublicAdvertProjection` | `INTERNAL_SYSTEM` | `None` | Public search/detail |

## 10. DTO / schema envanteri (OpenAPI components)

Ortak: `DomainErrorCode`, `ErrorResponse`, CursorPageMeta, strict `Money` pair, VersionedResource, MediaProcessingState, `SecurityEventType`, `TJKTriggerKind`, enums (AdvertStatus, UserRole, UserStatus, ClientContext, BannerPlacement/Status, TJK*, JobStatus, Media lifecycles).

Typed public: `PublicCategorySummary`, `PublicLocationSummary`. Upload: `UploadAuthorization`, `UploadConstraints`.

Domain groups: AuthToken*, MyProfile*, Session*, Province/District*, Category*, Horse*, PublishedAdvert*, OwnerAdvert*, Favorite*, Media upload/status/collection*, AdminUser*, AdminCategory/Property*, Moderation*, Banner*, TJKSync*.

## 11. Public veri sızıntısı sınırları

Public/FE responses asla içermez: password/refresh hashes, security stamp, raw/master object keys, provider credentials, full technical metadata, raw TJK payload, checkpoints, worker leases, public moderation reasons, security-event secrets.

`RegisterUser` duplicate email dışarıda generic success ile ayrılmaz (DB unique + internal log korunur). Token verify/reset/confirm existence sızdırmaz (401/404 yok).

Status history yalnız BO moderation detail’dedir.

## 12. OpenAPI validation sonucu

- YAML parse (Ruby `YAML.load_file`): **OK** (77 paths, 88 operations)
- `operationId` count: **88**; duplicate: **yok**
- Exposure: PUBLIC 20 / FE_AUTH 29 / BO_AUTH 39
- Her operation `x-function-id` + `x-exposure` + `x-actor` + `x-error-codes`: **OK**
- `$ref` sibling violation: **0**
- Response-side object schemas without `required` (excl. PATCH request): **0**
- Non-2xx ↔ `x-error-codes` eksik mapping: **0**
- Contract leak property names: **yok** (`refreshToken` yalnız `AuthTokenResponse` / request)
- Internal operationId/path sızıntısı: **yok**
- Payment/package/campaign/article/contact/YKK path-schema: **yok**
- `oapi-codegen` (`types,gin` → `/tmp/haradan-openapi-check.gen.go`): **OK** (repo’ya generated dosya yazılmadı)

## 13. Açık ürün kararlarının API’ye etkisi

Resume/restore/withdraw/republish, payment/package, keyword search, favorite count, banner scheduling, single-item TJK retry endpoint’leri phase-one sözleşmesinde yoktur.

## 14. Kabul kriterleri

1. 88 HTTP operation? **Evet**
2. operationId = docs/14 function name? **Evet**
3. Her operation `x-function-id` + `x-exposure`? **Evet**
4. Internal 24 HTTP path değil? **Evet**
5. BO yalnız `/v1/admin`? **Evet**
6. Self-service `/v1/me`? **Evet**
7. Optional auth yalnız public advert search/detail? **Evet**
8. CreateBanner status almaz / INACTIVE? **Evet** (schema description)
9. expectedVersion/mediaVersion model? **Evet**
10. Payment/package yok? **Evet**
11. Object key/hash sızıntısı yok? **Evet**
12. OpenAPI makine tarafından parse edilebilir? **Evet** (validation raporunda)

## Appendix: tag counts

- Account: 6
- AdminBanners: 6
- AdminCatalog: 12
- AdminMedia: 3
- AdminModeration: 6
- AdminTjk: 7
- AdminUsers: 5
- AdvertsOwner: 11
- AdvertsPublic: 2
- Auth: 10
- BannersPublic: 1
- Catalog: 2
- Favorites: 3
- Geo: 4
- Horse: 2
- Media: 7
- System: 1
