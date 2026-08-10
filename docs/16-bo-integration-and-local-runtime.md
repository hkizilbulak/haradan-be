# 16 — BO Integration and Local Runtime

Current integration notes for Haradan API + worker + Back Office (BO).
Decision history remains in docs `01`–`15`; this file documents **how to run** the current stack locally.

## Architecture boundary

- **OpenAPI source of truth:** `api/openapi.yaml`
- **BO** talks only to the HTTP API (`BACKEND_API_URL`). It must not connect to PostgreSQL, TJK, B2/S3, or Resend directly.
- **Browser** must not see provider secrets or raw private object URLs. Media is served via `/api/v1/media/{assetId}/{profile}`.
- Roles: `user` | `admin`. Packages are dynamic (no fixed STARTER/MIDDLE/ADVANCED business equality).
- Payment is out of scope for this phase.

## Processes

| Process | Entry | Role |
|---|---|---|
| API | `go run ./cmd/api` | HTTP, auth, admin + public ops |
| Worker | `go run ./cmd/worker` | Background jobs, TJK claim loop, schedulers |
| BO | `npm run build` then `go run main.go` | Static `out/` embedded; proxies/CORS to API |

API and worker share the same environment variables (see `.env.example`). Config is loaded from the **process environment** (`internal/config`); the Go processes do not auto-load `.env` files unless your shell/tooling injects them.

## Local API

1. Export env from a sanitized copy of `.env.example` (never commit real secrets).
2. Ensure `DATABASE_URL` points at a dedicated Haradan DB (`hrd_*` tables). Do not point local experiments at shared production without an explicit ops plan.
3. Apply migrations with the repo’s goose workflow (forward only against non-prod).
4. Start API:

```bash
export HTTP_ADDR=:8080
go run ./cmd/api
```

Health: `GET /health` (or the OpenAPI health operation) — dependency failures return 503.

## Local worker

Same env as API. Start:

```bash
go run ./cmd/worker
```

Notes:

- Scheduled definitions live in `hrd_job_definitions` (three seeds: `TJK_SYNC`, `PACKAGE_EXPIRY_SCAN`, `MEDIA_RECONCILE`).
- The worker is capability-aware and stays running with partial configuration. Package/fan-out jobs need no external provider; email jobs are claimed only with Resend; media jobs are claimed only with both B2 and Tinify; TJK runs independently of media/email.
- `TJK_ENABLED=true` explicitly enables TJK in the API, scheduler, and worker. `TJK_BASE_URL` alone does not enable it. When disabled, manual/scheduled TJK triggers are rejected instead of leaving unclaimable work queued.
- `JOB_SCHEDULER_REFRESH_INTERVAL` reloads active job definitions into the cron scheduler.

## Local BO

BO env (process serving `main.go`, not Next runtime):

| Variable | Purpose |
|---|---|
| `BACKEND_API_URL` | Absolute API base, e.g. `http://127.0.0.1:8080` |
| `PORT` | BO Go server listen port |

Build + serve:

```bash
npm run build
go run main.go
```

`main.go` embeds `out/` (`//go:embed all:out`). Treat `out/` as a generated artifact; restore it after validation builds unless preparing a production commit.

Session cookies are HttpOnly; BO must not store access tokens in `localStorage` / `sessionStorage`.

## Optional integrations (no live calls required for BO UI work)

### Email (Resend)

Set `EMAIL_PROVIDER=resend` and fill:

- `RESEND_API_KEY`
- `FROM_EMAIL` / `FROM_NAME`
- `FRONTEND_URL` (links in invitation / reset flows)
- `RESEND_WELCOME_TEMPLATE_ID` (and legacy alias `RESEND_REGISTRATION_VERIFICATION_TEMPLATE_ID`)
- `RESEND_RESET_PASSWORD_TEMPLATE_ID`

If `EMAIL_PROVIDER` is empty/`unconfigured`, the API uses a noop sender. Admin user create still succeeds; `invitationEmailSent` is `false`. Provider template discovery endpoints return **503** `DEPENDENCY_UNAVAILABLE` with a Turkish “sağlayıcı yapılandırılmamış” message — BO should treat that as configuration state, not a generic crash.

SMTP is not implemented. Only `resend` and `unconfigured` are accepted `EMAIL_PROVIDER` values.

### Object storage (B2 via S3 API)

Set `STORAGE_PROVIDER=b2` and all required `S3_*` fields. Empty/`unconfigured` keeps uploads unavailable without requiring credentials.

### Image processor (Tinify)

Set `IMAGE_PROCESSOR_PROVIDER=tinify`, `TINIFY_API_KEY`, and profile width/height env vars. Unconfigured keeps the stub processor.

### TJK

- `TJK_ENABLED=true` to allow the API/scheduler to enqueue and workers to execute TJK jobs.
- `TJK_BASE_URL` optional (defaults to the public TJK host when enabled).
- Tunables: `TJK_HTTP_TIMEOUT`, `TJK_BATCH_SIZE`, `TJK_MAX_BODY_BYTES`.

Do not run real TJK sync against production hosts from disposable local environments unless intentionally testing the adapter.

## Admin create-user invitation

`POST /v1/admin/users` creates an ACTIVE account with a random hashed password (never returned) and a one-time `PASSWORD_RESET` invitation credential. Email is attempted after commit when the email provider is configured; otherwise the response reports `invitationEmailSent: false` and the account remains recoverable.

`POST /v1/admin/users/{userId}/invitation/resend` (`ResendAdminUserInvitation`) re-issues the invitation for an ACTIVE user: previous unused invitation credentials are invalidated, a new hashed OTC is stored, and email is attempted. Plaintext tokens are never returned in API responses or logs. When the email provider is unconfigured, the endpoint returns **503** `DEPENDENCY_UNAVAILABLE` without rotating credentials. Delivery failure after rotation also returns **503** so the admin can retry.

Public forgot-password remains enumeration-safe (generic success even when noop/unconfigured). Admin invitation delivery must not treat noop/unconfigured as a successful send — use `invitationEmailSent` and/or 503 accordingly.

Last active admin demotion/disable is rejected with **409** and serialized with a transaction-scoped advisory lock so concurrent cross-demote cannot leave zero ACTIVE admins.

## CORS / cookies

BO Go server CORS allowlist must match the local/prod BO origin. Session cookies require compatible `SameSite` / HTTPS settings for the deployment topology.

## BO → Go proxy → BE auth topology

Browser calls relative `/api/...` with `withCredentials` (HttpOnly session cookies).
BO Go `main.go` handles `/api/session*` locally and proxies other `/api/*` to Haradan BE,
attaching `Authorization: Bearer <access_token>` from the session cookie.

BE `authn.Selective` must include every used BO_AUTH route (admin categories, media,
user profile/email-change, package reorder, etc.). Unlisted routes skip Bearer auth →
handlers see no Principal → `401 Kimlik doğrulama gerekli.`

Do not call `BACKEND_API_URL` from the browser except signed media PUT URLs.

## Safety

- Do not put secrets in docs, commits, or BO UI copy.
- Do not mutate migrations `00001`–`00015` after apply; add `00016+` only.
- Haradan tables use `hrd_*`. Never touch `hr_*` (Kartezya HR) or prefixless legacy tables from Haradan migrations/code.

## Campaign provider email templates

Campaign rows may store a nullable `email_provider_template_id` (`emailProviderTemplateId` in API). The name is provider-neutral (today’s adapter may be Resend). Rich layout/image/button design lives in the provider template; BO selects an id from discovery endpoints and keeps optional subject/heading/body fallbacks. Fallback `emailBody` may be sanitized HTML from CKEditor Classic (same family as Kartezya FAQ). Provider templates are **not** replaced by arbitrary BO HTML. When the email provider is unconfigured, discovery returns 503 and selection UI stays empty without blocking other campaign fields.

## Rich text (BO)

- Editor: `@ckeditor/ckeditor5-build-classic` + `@ckeditor/ckeditor5-react` (Kartezya FAQ pattern).
- Storage: sanitized HTML (plain legacy text remains valid).
- Client sanitize: `sanitize-html` allowlist (`helpers/sanitizeHtml.ts`) + `SafeRichText` renderer.
- Server sanitize: `internal/platform/security/richhtml` on package `description` and campaign `emailBody`.
- Image/media toolbar plugins disabled until a secure Haradan media upload path exists for that field.
- Notification `inAppBodyTemplate` stays plain template text (domain rejects HTML).

## Kartezya template reuse strategy

Reuse responsive layout/CTA patterns from the company provider dashboard by **duplicating** templates into Haradan-specific template IDs. Do not share live template IDs or copy secrets/HR branding into source. Haradan env uses its own `FROM_*`, `FRONTEND_URL`, and template id configuration.
