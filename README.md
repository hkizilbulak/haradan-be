# Haradan Backend (BE)

Haradan BE, Haradan uygulamalarının HTTP API'sini ve kuyruklanmış arka plan işlerini yürüten Go servisidir.

## Yerel gereksinimler

- Go
- Erişilebilir PostgreSQL veritabanı
- Depo kökünde yerel `.env`

`.env` Git tarafından yok sayılır. Veritabanı bağlantısı, JWT anahtarı ve sağlayıcı kimlik bilgileri gibi secret değerleri hiçbir zaman commit etmeyin veya dokümana yazmayın.

Doğrudan `go run ./cmd/api` çalıştırmak `.env` dosyasını otomatik yüklemez. Aşağıdaki Makefile hedefleri `.env` dosyasını sürece aktarır.

## API

```bash
make api
```

Bu hedef API'yi yerelde açıkça `:3001` üzerinde başlatır.

Health kontrolü:

```text
http://localhost:3001/api/health
```

## Worker

Ayrı bir terminalde:

```bash
make worker
```

Worker; TJK senkronizasyonu, medya işleme, e-posta ve diğer kuyruklanmış arka plan işlerini yürütür. Tam yerel davranış için API ile birlikte çalışmalıdır.

## Ortam güvenliği

- Railway TEST ve production yapılandırmalarını birbirine karıştırmayın.
- Başlatmadan önce veritabanı ve sağlayıcı ayarlarının amaçlanan ortama ait olduğunu doğrulayın.
- Production secret'larını yerel test değerleriyle değiştirmeyin; yerel secret'ları da production'a taşımayın.
- `.env` dosyasını commit etmeyin, terminal çıktısına dökmeyin ve paylaşmayın.
