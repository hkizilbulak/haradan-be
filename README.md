# Haradan Backend API

## Proje hakkında

Haradan Backend; Gin tabanlı Go API, PostgreSQL veri katmanı ve bağımsız background worker'dan oluşur. Public ve admin API'lerini, TJK at verisi entegrasyonunu, e-posta bildirimlerini, medya depolama ve görsel işleme akışlarını sağlar.

Haradan BO, Next.js arayüzünü Go runtime/session/API proxy üzerinden sunar ve admin işlemlerinin tek istemcisidir. Gelecekteki React Native Web, Android ve iOS uygulamaları public API'ye doğrudan bağlanacak; BO session proxy'sini kullanmayacaktır.

## Kullanılan teknolojiler

| Teknoloji | Kullanım amacı |
| --- | --- |
| Go + Gin | REST API ve servis katmanı |
| PostgreSQL | Kalıcı uygulama verisi |
| OpenAPI | API sözleşmesi ve server/client uyumu |
| Goose | İleri yönlü SQL migrasyonları |
| Background worker/job sistemi | Zamanlanmış ve kuyruklanmış işler |
| TJK | At verisi kaynağı |
| Resend | Transactional e-posta gönderimi |
| Backblaze B2 / S3-compatible storage | Medya nesne depolama |
| Tinify | Görsel optimizasyonu |

Provider seçimi ve yapılandırması environment üzerinden yapılır.

## Gereksinimler

- Git
- Go `1.26.5` (`go.mod`)
- PostgreSQL erişimi
- Tam local stack için Node.js ve npm; minimum Node.js/npm sürümü repository'de sabitlenmemiştir
- BO runtime için ayrıca Go `1.25`

## Local klasör yapısı

Varsayılan yerleşim:

```text
parent/
├── haradan-be/
└── haradan-bo/
```

BE farklı bir konumdaysa BO launcher komutlarında `HARADAN_BE_DIR=/path/to/haradan-be` kullanılabilir.

## Environment dosyası

Local BE yapılandırması repository kökündeki `.env` dosyasındadır. Dosya Git'e commit edilmez; gerçek environment secret ve API key değerleri README'ye yazılmaz.

| Değişken | Amaç |
| --- | --- |
| `DATABASE_URL` | PostgreSQL bağlantısı |
| `AUTH_JWT_SECRET` | Access/refresh token imzalama |
| `HTTP_ADDR` | API dinleme adresi; local launcher varsayılanı `:8080` |
| `CORS_ALLOWED_ORIGINS` | İzin verilen browser origin'leri |
| `TJK_BASE_URL`, `TJK_ENABLED`, `TJK_*_TIMEOUT`, `TJK_MAX_BODY_BYTES` | TJK provider ve timeout sınırları |
| `EMAIL_PROVIDER`, `RESEND_*`, `FROM_EMAIL`, `FROM_NAME`, `FRONTEND_URL` | E-posta provider ve link ayarları |
| `STORAGE_PROVIDER`, `S3_*`, `MEDIA_PUBLIC_BASE_URL` | S3-compatible/Backblaze B2 depolama |
| `IMAGE_PROCESSOR_PROVIDER`, `TINIFY_*` | Görsel işleme provider'ı |
| `WORKER_*`, `JOB_SCHEDULER_REFRESH_INTERVAL` | Worker, lease, retry ve scheduler davranışı |

Liste ana grupları özetler; kullanılabilir tam değişken seti `internal/config/config.go` içindedir.

## İlk kurulum ve full local stack

BE `.env` ve BO `.env.local` dosyalarını hazırladıktan sonra:

```bash
cd ../haradan-bo
npm install
npm run build
npm run start:all
```

`npm run build`, `out/` yoksa veya bilinçli olarak yenilenecekse gereklidir. `npm run start:all`; BO runtime'ı, BE API'yi ve worker'ı birlikte başlatan local developer kolaylığıdır. Tarayıcıda `http://localhost:3000`, health kontrolü için `http://localhost:8080/api/health` açılır. Production deployment bu komuta bağlı değildir.

Hot reload kullanan full stack için BO repository'sinde:

```bash
npm run dev:all
```

Tarayıcı adresi `http://localhost:3001`'dir.

## Local portlar

| Servis | Port |
| --- | ---: |
| BO runtime / Go proxy | 3000 |
| Next.js development | 3001 |
| Backend API | 8080 |
| Worker | HTTP portu yok |

## Servisleri ayrı çalıştırma

BE repository'sinde ayrı terminallerde:

```bash
go run ./cmd/dev api
go run ./cmd/dev worker
```

API `http://localhost:8080` üzerinde çalışır; worker HTTP portu açmaz. Make kurulu ortamlarda `make api` ve `make worker` opsiyonel developer kolaylıklarıdır. Birincil başlangıç yolu Make veya Bash gerektirmez.

BO ayrı olarak `npm run local` (build + `:3000`), `npm run local:start` (mevcut `out/` + `:3000`) veya `npm run dev` (Next.js `:3001`) ile çalıştırılabilir.

## Veritabanı ve migrasyonlar

Haradan PostgreSQL tabloları `hrd_` prefix'ini kullanır. Goose migrasyonları gömülüdür ve production'da ileri yönlü uygulanır. Güncel seviye `00016_campaign_email_provider_template.sql`, sürüm tablosu `hrd_schema_migrations`'dır.

```bash
go run ./cmd/migrate status
go run ./cmd/migrate version
go run ./cmd/migrate up
```

Komutlardan önce environment ile gelen `DATABASE_URL` hedefini doğrulayın. Shared/test veya production veritabanında destructive reset/drop/down uygulamayın.

## TJK senkronizasyonu

TJK, at verilerinin kaynağıdır. FULL sync background worker/job zinciri üzerinden sayfalı çalışır; retry, dedup/idempotency, checkpoint, güvenli page handoff ve terminal hata yönetimi içerir. Desteklenen at detayları, pedigree, statistics ve siblings enrichment verileri saklanır; enrichment hataları görünürdür ve gerektiğinde `PARTIAL_SUCCESS` kullanılır.

`INCREMENTAL` ve `RECONCILIATION` şu anda ayrı algoritmalar yerine FULL çalışma yolunu kullanır. Scheduled TJK sync varsayılan olarak pasiftir; etkinleştirildiğinde `Europe/Istanbul` saat diliminde Salı, Perşembe ve Cumartesi 00:10'da çalışır. İlk production FULL senkronizasyonunu uzun çalışan, kontrollü ve izlenen bir ortamda başlatın.

## OpenAPI ve gelecekteki public FE

[`api/openapi.yaml`](api/openapi.yaml) API sözleşmesinin source of truth dosyasıdır. React Native tabanlı gelecekteki Web, Android ve iOS istemcileri bu sözleşme üzerinden doğrudan BE public API'ye bearer token ile bağlanacaktır. Mevcut auth implementasyonu `PUBLIC_WEB` ve `MOBILE` audience tiplerini destekler; bu akış BO cookie/session mekanizmasından bağımsızdır. Admin işlemleri BO'da kalır.

## Production deployment

BE API, worker, BO, PostgreSQL ve gelecekteki React Native FE bağımsız deploy edilebilir birimlerdir. Production; `npm run start:all`, sibling klasör yapısı, localhost, Make/Bash veya BO `.env.local` dosyasına bağlı değildir. URL, port, database, JWT, CORS ve provider ayarları deployment environment/secret yönetimi üzerinden sağlanır; deployment secret'ları repository'de tutulmaz. Migrasyonları kontrollü biçimde API/worker rollout'undan önce uygulayın.

## Kısa troubleshooting

- Port kullanımdaysa `3000`, `3001` veya `8080` üzerindeki eski local process'i durdurun.
- Health yanıt vermiyorsa `http://localhost:8080/api/health`, BE `.env`, `DATABASE_URL` ve API loglarını kontrol edin.
- Full stack başlamıyorsa BE `.env`, BO `.env.local`, `HARADAN_BE_DIR` ve BO `out/` varlığını kontrol edin.
- Next dev proxy isteği başarısızsa Next `:3001`, BO proxy `:3000` ve kesin CORS allowlist'ini doğrulayın.
