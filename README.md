# Haradan Backend (BE)

Haradan BE, Haradan uygulamalarının HTTP API'sini ve kuyruklanmış arka plan işlerini yürüten Go servisidir.

## Yerel gereksinimler

- Go
- Erişilebilir PostgreSQL veritabanı
- Depo kökünde yerel `.env`

`.env` Git tarafından yok sayılır. Veritabanı bağlantısı, JWT anahtarı ve sağlayıcı kimlik bilgileri gibi secret değerleri hiçbir zaman commit etmeyin veya dokümana yazmayın.

Production API ve worker komutları `.env` dosyasını kendileri yüklemez. Yerel geliştirme için aşağıdaki platformlar arası launcher `.env` dosyasını okur; mevcut process environment değerleri dosya değerlerinden önceliklidir.

## Platformlar arası API başlangıcı

```bash
go run ./cmd/dev api
```

Komut Windows, macOS ve Linux üzerinde çalışır. Process environment içinde `HTTP_ADDR` tanımlı değilse veya boşsa API'yi yerelde `:3001` üzerinde başlatır.

Health kontrolü:

```text
http://localhost:3001/api/health
```

## Platformlar arası worker başlangıcı

Ayrı bir terminalde:

```bash
go run ./cmd/dev worker
```

Worker; TJK senkronizasyonu, medya işleme ve zamanlanmış/kuyruklanmış diğer arka plan işlerini yürütür. Tam yerel davranış için API ile birlikte çalışmalıdır.

## İsteğe bağlı Make komutları

Make kurulu macOS/Linux ortamlarında mevcut kolaylık hedefleri kullanılmaya devam edebilir:

```bash
make api
make worker
```

Make, Bash, `.sh`, `source`, Node.js veya npm birincil BE komutları için gerekli değildir.

## BO ile tam stack

`haradan-bo` deposu da mevcutsa tam yerel stack BO deposundan başlatılabilir:

```bash
npm run start:all
```

Bu BO komutu BE API'yi, BE worker'ı ve BO sunucusunu birlikte başlatır. BE deposu tek başına BO'ya bağımlı değildir.

## Ortam güvenliği

- Yerel `.env` yalnızca geliştirici launcher'ı tarafından okunur; production ve Railway yapılandırması yerel `.env` dosyasından ayrıdır.
- Railway TEST ve production yapılandırmalarını birbirine karıştırmayın.
- Başlatmadan önce veritabanı ve sağlayıcı ayarlarının amaçlanan ortama ait olduğunu doğrulayın.
- Production secret'larını yerel test değerleriyle değiştirmeyin; yerel secret'ları da production'a taşımayın.
- `.env` dosyasını commit etmeyin, terminal çıktısına dökmeyin ve paylaşmayın.
