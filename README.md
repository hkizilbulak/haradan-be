# Haradan Backend (BE)

Haradan BE; yönetim ve son kullanıcı uygulamalarının REST API'sini sunan Go servisi ile kuyruklanmış işleri yürüten bağımsız worker'dan oluşur. Kalıcı veri PostgreSQL'dedir; Haradan tabloları `hrd_` önekini kullanır.

## Mimari ve sözleşme

- `cmd/api`: Public ve admin HTTP API. Health endpoint'i `/api/health`'tir.
- `cmd/worker`: TJK senkronizasyonu, medya ve bildirim gibi kuyruklanmış/zamanlanmış işler; HTTP portu açmaz.
- `api/openapi.yaml`: API'nin kaynak sözleşmesi. Hedef, FE geliştiricisinin BE veya BO source kodunu değiştirmeden bu sözleşme üzerinden ilerleyebilmesidir.
- BO: Next.js arayüzünü sunan Go runtime/session proxy üzerinden `ADMIN_BO` audience ile BE'ye erişir; admin API yalnızca BO içindir.
- Gelecekteki React Native Web/Android/iOS istemcileri public API'ye doğrudan bearer token ile, `PUBLIC_WEB` veya `MOBILE` audience kullanarak bağlanmalıdır.
- Sağlayıcılar environment ile seçilir: TJK veri kaynağı, Resend e-posta, S3 uyumlu/Backblaze B2 nesne depolama ve Tinify görsel işleme.

## Yerel kurulum ve portlar

Gereksinimler: Go, Node.js/npm, erişilebilir PostgreSQL, bu depoda Git tarafından yok sayılan `.env` ve BO deposunda `.env.local`. Secret değerleri dokümana veya Git'e yazılmamalıdır. Temel BE ayarları `DATABASE_URL`, `AUTH_JWT_SECRET`, `HTTP_ADDR`; sağlayıcı ayarları `TJK_*`, `EMAIL_PROVIDER`/`RESEND_*`, `STORAGE_PROVIDER`/`S3_*` ve `IMAGE_PROCESSOR_PROVIDER`/`TINIFY_API_KEY` gruplarıdır.

| Bileşen | Yerel port | Amaç |
| --- | ---: | --- |
| BO runtime | 3000 | Normal BO UI + Go session/API proxy |
| Next.js dev | 3001 | BO hot reload |
| BE API | 8080 | REST API |
| Worker | yok | Arka plan işleri |

Varsayılan depo yerleşimi kardeş `haradan-be/` ve `haradan-bo/` klasörleridir. BE başka konumdaysa BO komutuna `HARADAN_BE_DIR=/path/to/haradan-be` verilebilir.

İlk kurulumdan sonra normal tam stack BO deposundan başlatılır:

```bash
npm install
npm run start:all
```

Bu komut BE API'yi `:8080`, worker'ı ve BO runtime'ını `:3000` üzerinde çalıştırır; `http://localhost:3000` açılır. Hot reload için `npm run dev:all` kullanılır ve arayüz `http://localhost:3001` üzerinden açılır. Bunlar platformlar arası yerel geliştirme kolaylıklarıdır; production süreç modeli değildir.

## Servisleri ayrı çalıştırma

BE deposunda ayrı terminallerde:

```bash
go run ./cmd/dev api
go run ./cmd/dev worker
```

Yerel launcher `.env` dosyasını okur ve mevcut process environment değerlerine öncelik verir. Make kurulu ortamlarda `make api` ve `make worker` isteğe bağlı kolaylıklardır; Bash/Make birincil gereksinim değildir. BO tek başına `npm run local`, mevcut build ile `npm run local:start`, yalnız Next.js geliştirme sunucusu olarak `npm run dev` ile çalıştırılabilir.

## Veritabanı ve migrasyonlar

Migrasyonlar gömülü Goose SQL dosyalarıdır ve production'da ileri yönlü uygulanır. Mevcut şema seviyesi `00016_campaign_email_provider_template.sql`'dir; sürüm tablosu `hrd_schema_migrations`'dır.

```bash
go run ./cmd/migrate status
go run ./cmd/migrate version
go run ./cmd/migrate up
```

`DATABASE_URL` hedefini çalıştırmadan önce doğrulayın. Down migrasyonu kasıtlı olarak ek koruma gerektirir; ortak test veya production veritabanında yıkıcı komut çalıştırmayın. Release sırasında migrasyonu API/worker rollout'undan önce tek kontrollü adım olarak uygulayın.

## TJK FULL senkronizasyon durumu

- Production-hardened yaklaşım, background worker/job zinciriyle tüm sayfaları dolaşan, checkpoint'li ve tekrar çalıştırılabilir FULL senkronizasyondur. Sayfa kalıcılığı, checkpoint/job tamamlama ve sonraki sayfanın kuyruğa alınması atomiktir; manuel ilk çalıştırma da atomik oluşturulur.
- Çalıştırma ve sayfa dedup anahtarları deterministiktir. Retry tükenmesi veya lease kaybı parent run'ı terminal duruma taşır; sayaçlar gerçekten işlenen/kaydedilen kayıtları gösterir.
- EOF yalnızca sağlayıcının doğrulanmış `Toplam 0` yanıtıyla kabul edilir. Tanınmayan/malformed HTTP 200 yanıtı başarı sayılmaz; transient hata olarak retry edilir.
- Detay, pedigree ve sibling enrichment desteklenir. Doğum tarihi, handikap, sahip, yetiştirici, kazanç, maiden sire, don/cinsiyet, istatistikler ve dinamik pedigree alanları korunur. Yetkili boş veri ile erişilemeyen/parse edilemeyen veri ayrıdır; enrichment eksikleri `PARTIAL_SUCCESS` üretebilir.
- `INCREMENTAL` ve `RECONCILIATION` bugün ayrı algoritmalar değildir; gerçeğe uygun biçimde FULL davranışına yönlenir.
- Seed job varsayılan olarak pasiftir; etkinleştirilirse `Europe/Istanbul` saat diliminde Salı/Perşembe/Cumartesi 00:10 çalışır.
- 11 Ağustos 2026 tarihli sınırlı gerçek TJK kabulünde ilk sayfa 50/50 benzersiz kayıtla ve detay/pedigree/sibling ayrıştırmalarıyla geçti; kaynak yaklaşık 72.674 kayıt bildirdi. Yaklaşık 1.454 sayfa ve 218 binin üzerinde enrichment isteği gerektirebilecek tam gerçek koşu yapılmadı. Bu koşu kontrollü, uzun ömürlü, izole bir operasyon ortamında ayrıca yürütülüp izlenmelidir; verilerin tamamının bugün veritabanında olduğu varsayılmamalıdır.

## Production dağıtımı

API, worker, BO, PostgreSQL ve gelecekteki public React Native/Web istemcileri ayrı deploy edilebilir birimlerdir. Production'da `start:all`, kardeş repo yerleşimi, localhost, `.env.local`, Make veya Bash'e güvenmeyin. Her birime yalnız gerekli environment/secret'ları verin; API adresi/portu, BO backend URL ve kesin CORS allowlist'i, veritabanı/JWT ve provider kimlik bilgilerini ortam bazında yönetin. Migrasyon sonrası API health kontrolünü, worker kuyruk/lease davranışını ve provider erişimini doğrulayın.
