# TJK Senkronizasyonu, Worker ve Operasyon Modeli Kararı

## 1. Problem tanımı

Aşağıdaki kavramlar birbirinden ayrılır:

- **TJK source adapter:** TJK’ya nasıl erişildiğini soyutlayan katman.
- **Sync run:** Belirli bir senkronizasyon çalışmasının operasyonel kaydı.
- **Sync job:** Worker’ın claim edip işleyebileceği dayanıklı iş birimi.
- **Batch:** Job içindeki sayfa/cursor/dosya dilimi.
- **Checkpoint:** Son başarıyla kalıcı işlenen ilerleme işareti.
- **Horse upsert:** TJK numarasına göre idempotent oluşturma/güncelleme.
- **Item sonucu:** Tek horse kaydının created/updated/unchanged/skipped/failed/conflict sonucu.
- **Run sonucu:** Sync run’ın genel başarı/kısmi başarı/başarısızlık özeti.
- **Retry:** Geçici hata sonrası kontrollü yeniden deneme.
- **Reconciliation:** Güvenilirlik için full/doğrulama taraması.
- **Schedule:** Planlı run/job oluşturma.
- **Manual trigger:** Yetkili admin’in BO üzerinden run başlatması.
- **Worker process:** Uzun işleri çalıştıran ayrı process/service.
- **API process:** İstekleri karşılayan process; uzun sync’i request süresince taşımaz.
- **Operasyon/audit görünürlüğü:** BO’nun run/job/hata görünümü.
- **Geçici işleme payload’ı:** Adapter/worker’ın geçici olarak kullandığı kaynak verisi.
- **Olası kalıcı ham TJK payload’ı:** Canonical olmayan, saklanıp saklanmayacağı açık audit adayı.
- **Field presence:** Normalized input’ta alanın Not provided / Explicitly empty-null / Provided with value durumları.
- **Teknik batch tamamlanması:** Batch sonucunun kalıcı commit edilip checkpoint’in ilerleyebilmesi.
- **Item-level partial batch:** Item hata/conflict içerse de teknik olarak tamamlanmış batch.
- **Cooperative cancellation:** Worker’ın güvenli sınırda durması; commit edilmiş güncellemelerin geri alınmaması.

Açık ilkeler:

- Source adapter erişim yöntemini soyutlar.
- Source adapter normalizasyon sırasında field-presence bilgisini kaybetmez; “alan yok” ile “explicit null” aynı zero-value’ya indirgenmez.
- Sync run operasyon kaydıdır; horse domain kaydı değildir.
- Worker job ile horse domain kaydı aynı şey değildir.
- Checkpoint ile canonical horse verisi aynı şey değildir.
- Checkpoint, batch’in teknik olarak eksiksiz işlenip sonucunun kalıcı commit edilmesine bağlıdır; yalnız sıfır item hatası demek değildir.
- Retry yeni horse kimliği üretmez.
- Job başarısı, bütün kaynak verisinin değişmiş olması demek değildir.
- Ham payload canonical horse kaynağı değildir.

Bu dokümanda tablo veya kolon tasarımı yapılmaz.

## 2. Fonksiyonel gereksinimler

TJK sync sistemi kavramsal olarak şu ihtiyaçları desteklemelidir:

- İlk toplu veri alma
- Sonraki düzenli senkronizasyonlar
- TJK numarasına göre idempotent upsert
- Yeni horse oluşturma
- Mevcut horse alanlarını kontrollü güncelleme
- Gerçek ad değişince normalize adı yeniden üretme
- Kontrollü detail JSONB güncelleme
- Field-presence koruyan kısmi merge
- Kısmi kaynak cevabında mevcut doğru veriyi koruma
- Duplicate veya kimlik çelişkisini engelleme
- Batch işleme
- Checkpoint ile kaldığı yerden devam
- Retry
- Geçici ve kalıcı hata ayrımı
- Kaynak rate limitine saygı
- Timeout ve backoff
- Aynı sync’in eş zamanlı iki kez çalışmasını engelleme
- Full/reconciliation ile incremental’ın aynı anda çakışmaması
- BO’dan sync durumunu görüntüleme
- Yetkili admin tarafından manual trigger (`admin` + BO session context)
- Yetkili admin + BO context ile cooperative cancellation
- Başarısız item veya batch retry
- Run geçmişi
- Restart/redeploy sonrası güvenli devam
- Veri tazeliğinin izlenmesi
- Mevcut advert ilişkilerinin korunması
- Fiziksel horse silmeme
- Kaynak mekanizması değişirse domain’i yeniden yazmama
- Geliştirme/test ortamında ortak PostgreSQL kullanılırken yalnız `hrd_` Haradan alanına dokunma

Bunlar kesin kolon, endpoint veya fonksiyon listesi değildir.

## 3. TJK source adapter sınırı

| Seçenek | Avantaj | Risk |
| --- | --- | --- |
| Resmî veya doğrulanmış API | Stabil şema, auth, pagination | Henüz doğrulanmadı |
| Dosya tabanlı aktarım | Batch/offline uygun | Gecikme; format değişimi |
| Kontrollü scraping | API yoksa alternatif | Kırılganlık; şartlar; operasyon riski |
| Başka doğrulanmış kanal | Esneklik | Doğrulama gerekir |

Değerlendirme boyutları: kimlik doğrulama, pagination, incremental destek, kaynak updated-at, rate limit, şema değişimi, retry, kaynak şartları, test edilebilirlik, adapter değiştirilebilirliği, field-presence korunumu.

**Kesin karar:**

- Horse sync use-case belirli HTTP endpoint, HTML selector, dosya formatı veya SDK’ya doğrudan bağlanmaz.
- Source adapter ortak bir normalized input üretir.
- Adapter ham dış kaynak verisi ile canonical horse modelini karıştırmaz.
- TJK erişim yöntemi doğrulanana kadar API/scraping/dosya yöntemlerinden biri kesin kabul edilmez.
- Source adapter normalizasyon sırasında field-presence bilgisini kaybetmemelidir.
- “Alan yok” ile “alan explicit null” aynı zero-value veya null değerine indirgenmemelidir.
- Normalized input, kavramsal olarak her alan için şu ayrımı taşıyabilmelidir:
  - Not provided
  - Explicitly empty/null
  - Provided with value
- Exact Go type, generic wrapper, pointer modeli veya struct tasarlanmaz.
- Adapter varsayılan değer üreterek kaynakta olmayan alanı gelmiş gibi göstermemelidir.
- Boş string normalizasyonu alanın domain anlamına göre yapılmalıdır.
- Adapter katmanı kaynak formatını normalize eder; hangi değerin canonical horse verisini temizleyebileceğine tek başına karar vermez.
- Explicit null’ın canonical değeri temizlemesine yalnız doğrulanmış source semantics ve merge policy karar verir.
- Kaynak schema değişikliği bu üç durumun yanlış yorumlanmasına neden olursa item güvenli biçimde fail/conflict olmalıdır; mevcut doğru veri sessizce silinmemelidir.
- Detail JSONB bölümlerinde de “bölüm gelmedi” ile “bölüm authoritative olarak boş geldi” ayrımı korunmalıdır.

Exact interface veya Go kodu yazılmaz.

## 4. İlk toplu import stratejileri

| Kriter | Tek transaction | Item-by-item | Batch + checkpoint |
| --- | --- | --- | --- |
| Hata izolasyonu | Zayıf | Yüksek | Yüksek |
| Performans | Riskli uzun TX | Düşük-orta | Dengeli |
| Restart | Kötü | Orta | İyi |
| Kısmi başarı | Zayıf | İyi | İyi |
| Gözlemlenebilirlik | Zayıf | Orta | İyi |
| Bu projeye uygunluk | Düşük | Orta | Yüksek |

**Öneri:** Batch + checkpoint. Bütün kaynak tek transaction’da işlenmez. Batch boyutu kesin sayı olarak belirlenmez.

Batch sonucu ile checkpoint ayrımı:

- **İşlenmiş fakat item hatası bulunan batch:** Bütün item’lar ele alınmıştır; geçerli güncellemeler kalıcı commit edilmiştir; kalıcı validation/conflict sonuçları kaydedilmiştir; batch partial olabilir ama teknik olarak tamamlanmış kabul edilebilir. Hata/conflict item’ları retry veya operasyon kuyruğuna bırakılarak checkpoint ilerleyebilir. Tek kalıcı bozuk item bütün run’ı sonsuza kadar aynı batch’te kilitlememelidir.
- **Teknik olarak tamamlanmamış batch:** Commit edilmedi, bağlantı kesildi, worker öldü, kaynak sayfa tam alınamadı, hangi item’ların kalıcı olduğu güvenilir değil veya batch sonucu/sayaçlar atomik kaydedilemedi. Checkpoint ilerlemez; batch tekrar alınabilir; idempotent upsert güvenli yeniden işlemeyi sağlar; run yanlış ilerlemiş gösterilmez.

“Batch başarılı” yalnız sıfır item hatası anlamına gelmez. Checkpoint, batch’in teknik olarak eksiksiz işlenip sonucunun kalıcı commit edilmesine bağlıdır.

## 5. Artımlı senkronizasyon stratejileri

Kaynak şunları sağlayabilir veya sağlamayabilir: updated-at, cursor, sayfa, değişiklik listesi, tam dataset, dosya versiyonu.

| Strateji | Kaynak bağımlılığı | Doğruluk | Maliyet | Eksik değişiklik | Reconciliation |
| --- | --- | --- | --- | --- | --- |
| Updated-at/cursor | Yüksek | İyi (güvenilirse) | Düşük | Orta | Gerekli |
| Full scan | Düşük | Yüksek | Yüksek | Düşük | Yerleşik |
| Fingerprint/snapshot | Orta | İyi | Orta | Orta | Destekler |
| Hibrit | Orta | Yüksek | Dengeli | Düşük | Yerleşik |

**İlk faz ilkeleri:**

- Kaynak incremental imkân sağlıyorsa kullanılabilir.
- Sağlamıyorsa uydurma updated-at üretilmez.
- Full scan güvenli ve idempotent olmalıdır.
- Exact sync sıklığı ürün/operasyon kararı olarak açık kalır.
- Hibrit: mümkünse incremental + zaman zaman full reconciliation.
- Aynı TJK source/scope üzerinde full/reconciliation run ile incremental run aynı anda çalışmamalıdır.
- Full/reconciliation bittiğinde incremental checkpoint’in geçerliliği yeniden doğrulanmalıdır; snapshot/version varsa onunla, yoksa güvenli yeni incremental başlangıç veya reconciliation kararı gerekir.

## 6. Idempotent horse upsert

Kavramsal davranış:

- Eşleştirme anahtarı TJK numarasıdır.
- Haradan teknik horse kimliği korunur; sync’te değişmez.
- Aynı TJK numarasıyla duplicate horse oluşturulmaz.
- Aynı kayıt tekrar işlendiğinde yeni kayıt üretilmez.
- Değişmeyen veri gereksiz güncelleme üretmemelidir.
- Gerçek ad değişirse normalize ad aynı işlem içinde yeniden hesaplanır.
- Advert foreign key ilişkileri korunur.
- Aynı batch içinde duplicate TJK numarası çelişki olarak ele alınır.
- Ciddi kimlik çelişkisi sessiz overwrite edilmez; operasyonel hata olur.
- Kullanıcı/admin override uygulanmaz.

Exact SQL / `ON CONFLICT` yazılmaz.

## 7. Eksik alan, null ve kısmi merge semantiği

Üç durum ayrılır ve adapter bu ayrımı normalized input’ta korur:

1. Alan kaynak cevabında hiç bulunmuyor (**Not provided**) → mevcut doğru değer korunur; otomatik silinmez.
2. Alan açıkça null/boş geliyor (**Explicitly empty/null**) → yalnız TJK’nın bu anlamı güvenilir biçimde verdiği doğrulanırsa temizlenebilir; aksi halde mevcut değer korunur.
3. Alan geçerli yeni değerle geliyor (**Provided with value**) → kontrollü güncelleme.

Ek kurallar:

- Adapter “alan yok” ile “explicit null”ı aynı zero-value’ya indirgemez.
- Adapter varsayılan değer üreterek kaynakta olmayan alanı gelmiş gibi göstermez.
- Boş string ile “bilgi yok” aynı kabul edilmeden önce alanın domain anlamına göre normalize edilir.
- Adapter hangi explicit null’ın canonical değeri temizleyeceğine tek başına karar vermez; bunu doğrulanmış source semantics + merge policy belirler.
- Geçersiz veri mevcut geçerli veriyi overwrite etmez.
- Temel tipli alanlar ayrı doğrulanır.
- Bir alan hatalı diye bütün horse güncellemesi zorunlu kaybedilmez; kimlik ve kritik bütünlük ihlalleri kaydı durdurabilir.
- Kaynak schema değişikliği üç durumu yanlış yorumlatırsa item fail/conflict olur; mevcut doğru veri sessizce silinmez.
- Kritik alan adayları: TJK numarası, Haradan horse kimliğinin korunması, gerçek adın geçerli güncellemesi.

**İlk faz:** Güvenli kısmi merge; eksik alan silmez; explicit null yalnız doğrulanmış anlamdaysa; field-presence adapter’dan merge’e taşınır.

## 8. Kontrollü detail JSONB güncelleme stratejisi

| Kriter | Kör replace | Kontrolsüz deep merge | Bölüm bazlı doğrulama + replace |
| --- | --- | --- | --- |
| Eski veri kalıntısı | Düşük | Yüksek risk | Düşük |
| Kısmi kaynak | Riskli | Riskli | Kontrollü |
| Şema değişimi | Orta | Zayıf | İyi |
| Veri güvenliği | Orta | Düşük | Yüksek |
| Bakım | Orta | Zor | Dengeli |
| Bu projeye uygunluk | Orta | Düşük | Yüksek |

Detail bölümleri: pedigri, yarış istatistikleri, kardeşler, yavrular, antrenör geçmişi.

**Öneri:** Bölüm bazlı doğrulama + replace.

- “Bölüm gelmedi” (**Not provided**) ile “bölüm authoritative olarak boş geldi” (**Explicitly empty**) ayrımı korunmalıdır.
- Kaynak bölüm eksikse (not provided) o bölüm silinmez.
- Bölüm authoritative olarak boş geldiği doğrulanırsa ve merge policy izin verirse o bölüm temizlenebilir.
- Parse edilemeyen bölüm diğer geçerli bölümleri bozmaz; item fail/conflict olabilir.
- Tipli temel alanlar detail JSONB’ye tekrar eklenmez.
- Public API JSONB’ye doğrudan bağlı olmaz.

## 9. Kaynakta görünmeyen horse davranışı

Durumlar: sync’te görünmeme, eksik sayfa/batch, geçici erişilemezlik, filtre değişimi, gerçekten kaynakta yok, bağlı advert var.

**Kesin ilkeler:**

- Tek sync’te görünmemek fiziksel silme nedeni değildir.
- Horse fiziksel silinmez.
- Advert ilişkisi bozulmaz.
- Geçici kaynak hatası public horse verisini otomatik silmez.
- “Kaynakta artık aktif değil” işareti ayrı operasyon kararıdır; tekrarlı ve güvenilir kanıt gerekir.
- Exact eşik veya gün sayısı uydurulmaz.

**İlk faz:** Görünmeyen horse korunur; silme/pasifleştirme otomatik yapılmaz.

## 10. Worker çalıştırma modelleri

| Kriter | API içi goroutine | Ayrı worker process | Worker + harici broker |
| --- | --- | --- | --- |
| İş dayanıklılığı | Zayıf | Yüksek | Yüksek |
| API performansı | Risk | İyi | İyi |
| Restart recovery | Zayıf | İyi | İyi |
| Operasyon | Yanıltıcı basit | Dengeli | Daha ağır |
| İlk faz maliyeti | Düşük | Orta | Yüksek |
| Gelecekte ölçekleme | Zayıf | İyi | Çok iyi |
| Bu projeye uygunluk | Düşük | Yüksek | Düşük (ilk faz) |

**Öneri:** Aynı Go codebase; ayrı API process; ayrı worker process; ortak domain/use-case paketleri; PostgreSQL üzerinden dayanıklı job/run koordinasyonu; harici broker olmadan başlama; ileride broker’a geçişe uygun adapter sınırı.

API process içinde güvenilmez, kaybolabilir tek background goroutine ana worker modeli değildir.

Exact command, Dockerfile veya Railway service config üretilmez.

## 11. PostgreSQL tabanlı job koordinasyonu

Broker olmadan ilk fazda PostgreSQL destekli dayanıklı yaklaşım:

- Job kalıcı kaydı
- Worker’ın işi claim etmesi
- Aynı işi iki worker’ın aynı anda işlememesi
- Lease/heartbeat ihtiyacı (worker ölürse yeniden claim)
- Retry zamanı ve attempt sayısı
- Job payload sınırı; büyük TJK dataset job payload’ına gömülmez
- Checkpoint ayrı yönetilir
- Transaction sınırı
- Poison job / terminal failure
- Reconciliation

**Ortak test veritabanı sınırı:**

- Geliştirme/test ortamında başka şirket projesiyle ortak PostgreSQL veritabanı kullanılacaktır.
- Mevcut diğer proje tabloları ve DB nesneleri `hr_` prefix’iyle başlamaktadır.
- Haradan’a ait gelecekteki bütün kalıcı tablolar `hrd_` prefix’iyle başlamalıdır.
- Haradan migration veya worker mekanizması `hr_` ile başlayan tablolara dokunmamalıdır.
- TJK sync run, job, checkpoint, horse ve operasyonel kayıtlarının exact tablo adları bu dokümanda tasarlanmaz; fakat gelecekte `hrd_` namespace/prefix kuralına uymalıdır.
- Index, sequence, constraint, migration metadata veya benzeri global isimler de çakışmayacak Haradan’a özel adlandırma kullanmalıdır.
- Ortak PostgreSQL kullanımı iki projenin domain tablolarının ilişkilendirileceği anlamına gelmez.
- Haradan worker yalnız Haradan’ın `hrd_` veri alanını yönetmelidir.
- Gerçek `.env` secret içerir; dokümana, loga veya Git’e yazılmamalıdır.
- Bu bölümde kesin credential, connection string veya tablo adı üretilmez.

Exact tablo, `FOR UPDATE SKIP LOCKED`, advisory lock veya SQL yazılmaz; uygulama aşamasında mekanizma seçilir.

| Kriter | Memory-only queue | PostgreSQL dayanıklı | Harici broker |
| --- | --- | --- | --- |
| Restart dayanıklılığı | Yok | Var | Var |
| Ek altyapı | Yok | Yok | Var |
| Çoklu worker | Zayıf | Mümkün | Güçlü |
| Retry | Zayıf | İyi | İyi |
| Operasyon | Zayıf | Dengeli | Daha karmaşık |
| İlk faz maliyeti | Düşük yanıltıcı | Orta | Yüksek |
| Bu projeye uygunluk | Düşük | Yüksek | Düşük (ilk faz) |

## 12. Sync run ve job ayrımı

### Sync run

- Kullanıcı/planlayıcı açısından bir TJK senkronizasyon çalışması
- Başlangıç/bitiş zamanı
- Kaynak türü
- Çalışma modu (full / incremental / reconciliation)
- Checkpoint
- Toplam/başarılı/güncellenen/değişmeyen/başarısız/conflict sayaçları
- Genel sonuç
- Operasyon görünürlüğü
- Cooperative cancellation isteği / tamamlanması

### Job

- Worker’ın işleyebildiği dayanıklı iş birimi
- Belirli batch/page/cursor
- Attempt/retry
- Claim/lease
- Teknik hata

### Item sonucu

- Created / Updated / Unchanged / Skipped / Failed / Conflict

**İlk faz önerisi:** Run-level counter’lar tutulur. Her başarılı item için sonsuza kadar ayrıntılı kayıt tutulmaz. Hata ve conflict detayları kalıcı tutulur; BO örnek hata görünürlüğü yeterlidir. Job status ile sync run status aynı enum olmak zorunda değildir.

Batch sonucu ayrımı:

- Item hatalı fakat teknik tamamlanmış batch → counters/hata detayları kaydedilir; checkpoint ilerleyebilir; run sonunda `PARTIAL_SUCCESS` olabilir.
- Teknik tamamlanmamış batch → checkpoint ilerlemez; counters yanlış başarı göstermez.
- Counter, item sonucu, batch sonucu ve checkpoint birbirini yanlış başarı gösterecek şekilde ayrılmamalıdır.

## 13. Sync run durumları

| Durum | Anlam | Terminal? | İlk faz |
| --- | --- | --- | --- |
| QUEUED | Çalışma kuyruğa alındı | Hayır | Evet |
| RUNNING | Worker işliyor | Hayır | Evet |
| SUCCEEDED | Tamamlandı, kritik hata yok | Evet | Evet |
| PARTIAL_SUCCESS | Kalıcı item hata/conflict’leriyle tamamlanmış run | Evet | Evet |
| FAILED | Teknik/ilerlenemez hata veya geçici hata retry sınırını aşma | Evet | Evet |
| CANCELLED | Yetkili iptal; cooperative olarak uygulandı | Evet | Evet |
| PAUSED | Duraklatıldı | Hayır | Hayır (ilk faz gereksiz) |

Checkpoint RUNNING sırasında teknik olarak tamamlanmış batch’lerin kalıcı commit’i ile ilerler. `PARTIAL_SUCCESS`, kalıcı item hata/conflict’leriyle tamamlanmış run anlamına gelebilir. Geçici teknik hata retry sınırını aşarsa run `FAILED` olabilir.

**Cancellation tutarlılığı (ilk faz):**

- Yetkili admin + BO session context cancellation isteyebilir.
- Cancellation cooperative olmalıdır.
- Worker mevcut güvenli transaction/batch sınırını tamamlayıp sonraki batch’e başlamadan durabilir.
- Kalıcı commit edilmiş horse güncellemeleri geri alınmaz.
- İşlenmemiş batch’ler tamamlandı gibi gösterilmez.
- Checkpoint son kalıcı tamamlanan batch’te kalır.
- Run, worker cancellation isteğini güvenli biçimde uyguladıktan sonra `CANCELLED` olur.
- Cancellation isteği ile cancellation’ın tamamlanması kavramsal olarak ayrılabilir.
- Exact cancel flag, status geçişi, kolon veya endpoint tasarlanmaz.

BO bu durumları gösterir.

## 14. Checkpoint ve kaldığı yerden devam

Değerlendirme: page/cursor, dosya offset/batch index, son başarılı batch, restart, kaynak snapshot değişimi, aynı run devam vs yeni run, paralel batch sıralaması.

**Kesin ilkeler:**

- Checkpoint yalnız ilgili batch başarıyla ve kalıcı işlendikten sonra ilerletilir.
- “Batch başarılı / checkpoint ilerler” = teknik olarak eksiksiz işlenmiş + sonucu kalıcı commit edilmiş batch; yalnız sıfır item hatası demek değildir.
- İşlenmiş fakat item hatası bulunan batch’te geçerli güncellemeler commit edilmişse ve batch sonucu kaydedilmişse checkpoint ilerleyebilir; hata/conflict item’ları retry/operasyon kuyruğuna bırakılabilir.
- Teknik olarak tamamlanmamış batch’te checkpoint ilerlemez; batch tekrar alınabilir; idempotent upsert güvenli yeniden işlemeyi sağlar.
- İşlenmemiş batch atlanmış gibi işaretlenmez.
- Restart sonrası aynı horse kayıtlarını yeniden işlemek güvenli olmalıdır (idempotent upsert).
- Kaynak cursor geçersizleşirse kontrollü yeni run/reconciliation gerekebilir.
- Checkpoint kullanıcı tarafından serbestçe değiştirilmez.
- Exact checkpoint formatı source adapter’a bağlıdır.

**Checkpoint sıralaması:**

- Bir run içinde batch’ler paralel işlenebiliyorsa checkpoint yalnız kesintisiz tamamlanmış aralığın sonuna ilerleyebilir.
- Örneğin batch 5 tamamlanmış fakat batch 4 tamamlanmamışsa checkpoint batch 5’e atlamamalıdır.
- Kaynak adapter bağımsız cursor’lar sağlıyorsa her cursor ayrı takip edilebilir; exact format adapter’a bağlıdır.
- Kaynak yalnız doğrusal page/cursor sağlıyorsa ilk fazda sequential batch veya contiguous high-water-mark yaklaşımı kullanılmalıdır.
- İlk faz önerisi: source traversal/checkpoint ilerlemesi sıralı olabilir; bir batch içindeki item işlemleri kontrollü paralelleştirilebilir; ya da paralel batch kullanılırsa yalnız kesintisiz tamamlanan batch zinciri checkpoint’i ilerletir.
- Kesin concurrency sayısı belirlenmez.

## 15. Retry ve hata sınıflandırması

### Geçici hatalar

Timeout, TJK geçici erişilemezlik, rate limit, network, geçici PostgreSQL bağlantısı, worker ölümü, commit/sayaç kaydının atomik tamamlanamaması. (TinyPNG vb. medya hataları TJK job’ı değildir.)

### Kalıcı veya veri kaynaklı hatalar

Geçersiz TJK numarası, parse edilemeyen zorunlu kimlik, aynı TJK ile çelişkili kayıt, desteklenmeyen şema, geçersiz tip, field-presence’ın güvenilir yorumlanamaması.

**Ayrım:**

- Kalıcı item hatası / conflict → batch teknik tamamlanmış sayılabilir; checkpoint ilerleyebilir; run `PARTIAL_SUCCESS` ile bitebilir; tek bozuk item run’ı aynı batch’te kilitlemez.
- Teknik batch/altyapı hatası → checkpoint ilerlemez; batch retry edilir; retry sınırını aşarsa run `FAILED` olabilir.

Yaklaşım: exponential backoff + jitter; maksimum attempt; kalıcı hatayı sonsuza kadar retry etmeme; manual retry; retry duplicate üretmez; BO’da anlaşılır hata nedeni.

Kesin süre veya attempt sayısı uydurulmaz.

## 16. Concurrency ve tekil sync koruması

Yarışlar: iki scheduled full sync, manual + scheduled, iki worker aynı batch, aynı TJK iki batch’te, retry + normal job, full + incremental, paralel batch checkpoint boşluğu, horse update sırasında advert okuma.

**İlk faz yaklaşımı:**

- Aynı source/scope için tek aktif full sync.
- Aynı TJK source/scope üzerinde full/reconciliation ile incremental aynı anda çalışmaz.
- Scheduled incremental, aktif full/reconciliation varken yeni çakışan run başlatmaz veya güvenli biçimde sıraya alınır.
- Manual full sync de aktif incremental ile doğrudan çakışmaz.
- Aynı run içindeki teknik batch/job concurrency ile iki bağımsız sync run’ın çakışması aynı şey değildir.
- Source traversal/checkpoint ilk fazda sıralı veya contiguous high-water-mark ile ilerler; batch içi item’lar kontrollü paralelleştirilebilir.
- Paralel batch kullanılırsa checkpoint yalnız kesintisiz tamamlanan aralığa ilerler.
- Horse upsert DB bütünlüğüyle korunur.
- Job claim/lease.
- Beklenen run/checkpoint doğrulaması.
- Canonical horse update tutarlı transaction sınırında.
- Stale worker sonucu reddedilir.

Kesin locking SQL’i veya kolon tasarlanmaz.

## 17. Scheduling ve manual trigger

Ayrım: planlı sync, admin manual sync, başarısız run/batch retry, tek horse operasyonel resync (ürün), full reconciliation, cooperative cancellation.

Kurallar:

- Exact sync sıklığı uydurulmaz.
- Scheduling mekanizması Railway özelliği doğrulanmadan kesin kabul edilmez.
- Scheduler yalnız job/run oluşturur; ağır sync’i kendi process’inde çalıştırmaz.
- Manual trigger: admin role + BO session context.
- Manual trigger eş zamanlı duplicate full sync üretmez.
- Scheduled incremental, aktif full/reconciliation varken çakışan run başlatmaz veya sıraya alır.
- Manual full, aktif incremental ile doğrudan çakışmaz.
- FE kullanıcısı TJK sync başlatamaz.
- Admin TJK alanını düzenleyemez; yalnız operasyon başlatır/izler/iptal eder.
- Cancellation: admin + BO context; cooperative; commit edilmiş güncellemeler geri alınmaz.

## 18. BO operasyon görünürlüğü

BO kavramsal olarak görebilir: son başarılı sync zamanı, çalışan run, run modu, başlangıç/bitiş, checkpoint/progress, created/updated/unchanged/skipped/failed/conflict sayıları, geçici/kalıcı hata ayrımı, son hata, retry durumu, kaynak bağlantı problemi, duplicate/kimlik çelişkileri, manual trigger, retry, cooperative cancel isteği ve tamamlanması, raw payload görünürlük sınırı.

Kurallar:

- BO TJK temel alanlarını manuel düzenlemez.
- Yalnız yetkili admin + BO context.
- Cancellation isteyebilir; cooperative tamamlanmayı izleyebilir.
- Raw payload saklanmıyorsa varmış gibi davranılmaz; saklansa bile public/canonical değildir.
- Secret, credential veya hassas header gösterilmez.
- Exact ekran tasarımı yok.

## 19. Observability ve güvenlik

Structured log, run/job correlation identifier, source adapter adı, batch/checkpoint, duration, sayaçlar, retry sayısı, worker heartbeat, veri tazeliği, alarm ihtiyacı.

Kurallar:

- TJK credential/secret loglanmaz.
- Ham response kontrolsüz loglanmaz.
- Büyük payload loglanmaz.
- Public kullanıcı verisiyle TJK operasyon logu karıştırılmaz.
- Exact log/monitoring ürünü seçilmez.

## 20. Rate limit, timeout ve kaynak dostu davranış

Kontrollü concurrency, request timeout, retry backoff + jitter, rate-limit response, kaynak kesintisi, circuit-breaker ihtiyacı (ürün/operasyon), uzun süreli başarısızlıkta BO uyarısı, manual trigger spam engeli.

Exact request sayısı, timeout veya kütüphane belirlenmez.

## 21. Medya worker ile TJK worker sınırı

| Yaklaşım | Değerlendirme |
| --- | --- |
| Aynı process bütün job türleri | Mümkün; concurrency ayrılmalı |
| Ayrı worker process/pool | İleride bölünebilir |
| Aynı dayanıklı altyapı + ayrı handler | İlk faz önerisi |
| Tamamen ayrı sistemler | İlk fazda gereksiz |

Kurallar:

- TJK sync ile medya processing domain durumları ayrıdır.
- TJK bulk işi medya yüklemelerini süresiz bekletmez.
- İş türü bazlı concurrency/öncelik uygulanabilir.
- Bir handler hatası diğer job türünü bozmaz.
- Ortak worker framework kullanılabilir; gereksiz mikroservis çoğaltılmaz.
- Gelecekte ayrı worker service/pool’a bölünebilir.

**İlk faz:** Aynı dayanıklı job altyapısı + ayrı handler ve concurrency politikaları; gerekirse aynı veya ayrı worker process.

## 22. Railway deployment yaklaşımı

Kavramsal yapı:

- Aynı repo/codebase
- API process/service
- Worker process/service
- Aynı PostgreSQL
- Ayrı process health/observability
- Worker restart sonrası job recovery
- Scheduler job/run oluşturur
- API uzun sync’i request süresince çalıştırmaz
- Deployment’ta in-flight job lease/retry ile geri alınır
- Graceful shutdown / cooperative cancellation sınırı
- Birden fazla worker instance mümkün

Geliştirme/test ortamında ortak PostgreSQL kullanılırken Haradan yalnız `hrd_` alanını yönetir; diğer projenin `hr_` nesnelerine dokunmaz. Exact credential, connection string veya tablo adı üretilmez.

Exact Railway service ayarı, Dockerfile, command veya cron tanımı üretilmez.

**İlk faz:** API ve worker ayrı process; PostgreSQL ortak; broker zorunlu değil.

## 23. Veri tazeliği ve public davranış

- Son başarılı sync zamanı BO’da görünür.
- Uzun süredir sync olmaması BO uyarısı olabilir; exact eşik uydurulmaz.
- TJK geçici kesintisi public autocomplete’i canlı TJK’ya yönlendirmez.
- Haradan DB’deki son güvenilir veri kullanılmaya devam eder.
- Sync başarısızlığı horse veya advert’i otomatik silmez.
- Horse’ın son sync zamanı operasyonel metrik olabilir.

## 24. Eş zamanlılık ve transaction sınırları

- Bütün dataset tek transaction değildir.
- Canonical horse güncellemesi (tipli alanlar + normalize ad + ilgili detail bölümü) kendi tutarlı transaction sınırına sahiptir.
- Batch/checkpoint ancak teknik olarak eksiksiz işlenmiş batch’in kalıcı commit’i sonrasında ilerler.
- Item hatalı fakat teknik tamamlanmış batch checkpoint’i kilitlemez; hata/conflict sonuçları kaydedilir.
- Teknik tamamlanmamış batch’te checkpoint ve counters yanlış başarı göstermez.
- Run counter, item sonucu, batch sonucu ve checkpoint tutarlı olmalıdır.
- Retry aynı item’i güvenli yeniden işleyebilmelidir.
- Source traversal/checkpoint ilk fazda sıralı veya contiguous high-water-mark ile ilerler; batch içi item paralelliği mümkün; paralel batch’te gap’li checkpoint atlaması yoktur.
- Full/reconciliation ile incremental aynı anda çalışmaz.

Exact isolation level belirlenmez.

## 25. Karşılaştırma tabloları

### 25.1 Worker modeli

(Bkz. bölüm 10 tablosu.)

### 25.2 İlk import stratejisi

(Bkz. bölüm 4 tablosu.)

### 25.3 Artımlı sync stratejisi

(Bkz. bölüm 5 tablosu.)

### 25.4 Detail JSONB güncellemesi

(Bkz. bölüm 8 tablosu.)

### 25.5 Job koordinasyonu

(Bkz. bölüm 11 tablosu.)

## 26. Önerilen karar

**Özet model:** Source adapter ile belirsiz TJK erişimi soyutlanır ve field-presence korunur; batch + checkpoint import; mümkünse incremental + ara sıra full reconciliation; full/incremental aynı anda çalışmaz; TJK numarasına idempotent upsert; güvenli kısmi merge; detail’de bölüm bazlı replace + bölüm presence ayrımı; görünmeyen horse silinmez; ayrı worker process + PostgreSQL dayanıklı job (`hrd_` alanı); sync run ≠ job; kısa run durum seti (CANCELLED cooperative); tek aktif full sync; sıralı/contiguous checkpoint; BO admin+context ile trigger/izleme/iptal; medya ile ortak altyapı/ayrı handler; API/worker ayrı Railway process; public canlı TJK fallback yok.

Soru cevapları:

1. **TJK erişim yöntemi kesinleştirilmeli mi?** Hayır (bu dokümanda).
2. **Source adapter?** Evet.
3. **Canonical input normalize?** Evet, adapter sonrası; field-presence korunarak.
4. **İlk import tek TX?** Hayır.
5. **Batch + checkpoint?** Evet.
6. **Idempotent TJK numarası?** Evet.
7. **Haradan horse id değişir mi?** Hayır.
8. **Duplicate TJK horse?** Hayır.
9. **Ad → normalize yeniden?** Evet, aynı işlemde.
10. **Eksik alan siler mi?** Hayır.
11. **Explicit null?** Yalnız güvenilir anlam doğrulanırsa; adapter tek başına karar vermez.
12. **Detail kör replace?** Hayır.
13. **Kontrolsüz deep merge?** Hayır.
14. **Bölüm bazlı replace?** Evet; “bölüm gelmedi” / “authoritative boş” ayrımıyla.
15. **Tek sync’te görünmeyen silinir mi?** Hayır.
16. **Horse fiziksel silinir mi?** Hayır.
17. **API goroutine ana worker?** Hayır.
18. **Ayrı worker process?** Evet.
19. **Harici broker ilk faz?** Hayır.
20. **PostgreSQL job koordinasyonu?** Evet; `hrd_` prefix alanı.
21. **Dataset job payload’a gömülür mü?** Hayır.
22. **Sync run = job?** Hayır.
23. **Her başarılı item sonsuz audit?** Hayır.
24. **Run-level counters?** Evet.
25. **Hata/conflict detayı?** Evet.
26. **Run durumları?** QUEUED, RUNNING, SUCCEEDED, PARTIAL_SUCCESS, FAILED, CANCELLED.
27. **Checkpoint ne zaman?** Teknik olarak eksiksiz işlenmiş batch’in kalıcı commit’i sonrası; item hatası tek başına kilitlemez; gap’li paralel atlama yok.
28. **Restart sonrası devam?** Evet.
29. **Geçici/kalıcı hata ayrımı?** Evet; kalıcı item vs teknik batch ayrımıyla.
30. **Kalıcı hata sonsuz retry?** Hayır.
31. **İki full sync aynı anda?** Hayır.
32. **Manual trigger kim?** Admin + BO context.
33. **FE sync başlatır mı?** Hayır.
34. **BO TJK alanını düzenler mi?** Hayır.
35. **Exact sync sıklığı?** Hayır (açık).
36. **Scheduler ağır sync çalıştırır mı?** Hayır; yalnız run/job oluşturur.
37. **TJK ve medya aynı domain model?** Hayır.
38. **Ortak worker altyapısı?** Evet.
39. **TJK bulk medyayı bloke eder mi?** Hayır.
40. **API/worker ayrı process?** Evet.
41. **Worker restart sonrası job recovery?** Evet.
42. **TJK geçici kesintisinde public veri silinmeli mi?** Hayır.
43. **Public autocomplete canlı TJK’ya fallback yapmalı mı?** Hayır.
44. **Stale data BO’da görünür olmalı mı?** Evet (uyarı).
45. **Ham TJK payload canonical kaynak olmalı mı?** Hayır.
46. **Adapter field-presence korusun mu?** Evet.
47. **Full ile incremental aynı anda?** Hayır.
48. **Cancellation cooperative mi?** Evet; commit edilmiş güncellemeler geri alınmaz.
49. **Haradan tabloları `hrd_` mi?** Evet; `hr_` alanına dokunulmaz.

**Gerekçe:** Veri bütünlüğü, idempotency, belirsiz TJK erişimi, field-presence güvenliği, Railway restart, sade MVP, PostgreSQL ile başlama, broker kurmama, ölçekleme yolu, operasyon görünürlüğü, hata izolasyonu, advert ilişkilerini koruma, medya sınırı, ortak DB namespace güvenliği, gereksiz altyapı üretmeme.

## 27. Reddedilen yaklaşımlar

- Doğrulanmadan doğrudan scraping veya belirli API endpoint varsaymak
- Source adapter olmadan TJK kodunu horse domain’ine gömmek
- Adapter’da “alan yok” ile “explicit null”ı aynı zero-value’ya indirgemek
- Adapter’ın varsayılan değer üreterek kaynakta olmayan alanı gelmiş gibi göstermesi
- Adapter’ın tek başına canonical temizleme kararı vermesi
- Bütün toplu import’u tek transaction yapmak
- Checkpoint tutmamak / commit öncesi ilerletmek
- Teknik tamamlanmamış batch’te checkpoint ilerletmek
- Tek kalıcı item hatasıyla bütün run’ı aynı batch’te sonsuza kilitlemek
- Gap’li paralel batch’te checkpoint’i ileri atlatmak
- Full/reconciliation ile incremental’ı aynı anda çalıştırmak
- Retry’de yeni horse kaydı oluşturmak
- Horse adını kimlik olarak kullanmak
- Normalize adı TJK kaynak alanı saymak
- Eksik kaynak alanında mevcut doğru değeri kör silmek
- Detail JSONB’yi kontrolsüz deep merge etmek
- “Bölüm gelmedi” ile “authoritative boş bölüm”ü aynı saymak
- Tek sync’te görünmeyen horse’u silmek
- API process içinde kaybolabilir goroutine’i ana worker yapmak
- Büyük TJK dataset’ini tek job payload’ına gömmek
- Memory-only queue ile restart dayanıklılığı varsaymak
- İlk fazda zorunlu olmadan Kafka/NATS/Redis kurmak
- Aynı anda duplicate full sync
- Kalıcı validasyon hatasını sonsuza kadar retry etmek
- Her başarılı item için sınırsız audit
- BO’nun TJK alanlarını manuel düzenlemesi
- FE kullanıcısının sync başlatması
- TJK geçici kesintisinde horse/advert silmek
- Public autocomplete’i canlı TJK’ya fallback etmek
- TJK bulk nedeniyle medya upload’larını süresiz bekletmek
- Ham payload’ı canonical/public kaynak yapmak
- Job status ile horse status’unu aynı domain modeli yapmak
- Cancellation’da commit edilmiş horse güncellemelerini geri almak
- İşlenmemiş batch’leri cancel sonrası tamamlanmış göstermek
- Haradan’ın `hr_` prefix’li diğer proje tablolarına dokunması
- Ortak DB’de iki proje domain tablolarını ilişkilendirmek

## 28. Açık kalan ürün ve teknik kararlar

- Gerçek TJK erişim yöntemi
- TJK kullanım şartları ve yetkilendirme
- API/dosya/scraping detayları
- Pagination/cursor desteği
- Güvenilir updated-at alanı
- İlk import veri hacmi
- Exact batch boyutu
- Exact sync sıklığı
- Retry attempt ve backoff değerleri
- Timeout değerleri
- Rate limit/concurrency değerleri
- Exact cancellation implementation ve BO UX ayrıntıları
- Tek horse manual resync ihtiyacı
- Raw TJK payload retention
- Ham payload audit/debug görünürlüğü
- Detail JSONB alt şemaları
- Horse pasiflik koşulu
- Stale data uyarı eşiği
- Worker instance sayısı
- Job claim/lease mekanizması
- Scheduling mekanizması
- Railway API/worker deployment detayları
- Monitoring ve alert ürünü
- Run/job log retention süresi
- TJK ile medya worker pool ayrım seviyesi
- Exact `hrd_` tablo/index/constraint adları (prefix kuralı sabit; adlar uygulama aşamasında)

## 29. Sonraki adım

Bu karar kabul edilirse sonraki teknik karar public arama, filtreleme, sıralama, pagination ve favoriler modelidir:

- Public advert search
- Category filtresi
- Province/district filtresi
- Horse filtresi
- Dinamik property filtreleri
- Fiyat filtresi ürün bağımlılığı
- JSONB filtreleme
- Sorting
- Cursor veya offset pagination
- Favoriler
- Fotoğraflı ilan filtresi
- Public/detail read modeli
- Query performansı ve indeksleme sınırı

Bu dokümanda arama API’si, SQL, indeks, tablo veya kod üretilmez.
