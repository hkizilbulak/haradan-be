# Medya, Görsel İşleme ve Banner Modeli Kararı

## 1. Problem tanımı

Aşağıdaki kavramlar birbirinden ayrılır:

- **Asset:** Fiziksel dosya ve teknik metadata sorumluluğu.
- **Ham upload nesnesi:** Kullanıcının veya admin’in cihazından doğrudan yüklediği ilk dosya; henüz güvenilir veya domain kullanımına hazır değildir; canonical yeniden işleme kaynağı değildir.
- **Güvenli canonical kaynak/master görsel:** Ham upload doğrulanıp normalize edildikten sonra backend/worker tarafından oluşturulan, yeniden varyant üretmeye uygun master kaynak; public response’ta kullanılmaz.
- **İşlenmiş görsel varyantı:** Transform profili uygulanmış türetilmiş dosya; her fiziksel varyantın hazırlık durumu canonical master durumundan ayrıdır.
- **Transform profili:** Resize/crop/sıkıştırma ve output kurallarını tanımlayan yapılandırılabilir profil.
- **Advert media ilişkisi:** Asset’in belirli bir advert içindeki kullanım ilişkisi (sıra, kapak).
- **Kapak görseli:** Bir advert’te aynı anda yalnız bir olan ana görsel ilişkisi.
- **Banner:** Admin tarafından yönetilen, belirli placement’ta gösterilen ayrı domain kaydı.
- **Banner placement:** Banner’ın gösterim yeri (`HOMEPAGE`, `LISTING_DETAIL`, `SEARCH`).
- **Upload işlemi:** Yetkilendirme, yükleme ve completion doğrulaması.
- **Görsel işleme işi:** Decode, normalize, master üretimi, resize, sıkıştırma, varyant yazma.
- **Storage nesnesi:** Backblaze B2 üzerindeki object; object key kullanıcı dosya adından bağımsızdır.

Açık ilkeler:

- Asset fiziksel dosya ve teknik metadata’dır.
- Advert media, asset’in advert içindeki kullanım ilişkisidir.
- Banner ayrı domain kaydıdır; advert media ile aynı şey değildir.
- Aynı asset altyapısı advert media ve banner tarafından kullanılabilir.
- Advert sahipliği, sıralama, kapak ve banner placement kuralları birbirine karıştırılmaz.
- Storage object key ile kullanıcıya görünen dosya adı aynı amaçta kullanılmaz.
- Ham upload ile güvenli canonical master aynı kavram değildir; doğru yeniden işleme kaynağı güvenli master’dır.
- Canonical master’ın hazır olması bütün transform varyantlarının hazır olduğu anlamına gelmez; varyant hazırlığı profil bazında ayrıdır.
- Domain kullanımına hazır olma, zorunlu transform profillerinin READY olmasına bakılarak değerlendirilir; tek belirsiz asset `READY` varsayımı yeterli değildir.
- Domain kaydının public/aktif olması ile asset’in referanslı olması aynı kavram değildir.

Bu dokümanda tablo veya kolon tasarımı yapılmaz.

## 2. Fonksiyonel gereksinimler

Medya altyapısı kavramsal olarak şu ihtiyaçları desteklemelidir:

- Web, Android ve iOS’tan görsel yükleme
- Büyük mobil fotoğraflarında kullanıcı deneyiminin korunması
- Yükleme ilerlemesi ve yeniden deneme
- Advert sahiplik kontrolü
- Birden fazla advert görseli
- Görsel sıralama
- Tek kapak görseli
- Güvenli canonical master’ın saklanması
- DETAIL, HOMEPAGE ve SEARCH kullanımları
- Aynı transform profilinin birden fazla kullanım tarafından paylaşılması
- TinyPNG sıkıştırması
- İşleme hatası ve retry
- Yarım kalmış yüklemelerin temizlenmesi
- Public gösterimde yalnız hazır görseller
- Banner placement doğrulaması
- Banner aktif/pasif yönetimi
- Storage dosyalarının referanssız kalmaması
- Yeniden işleme veya yeni varyant üretme imkânı
- Kullanıcıya teknik storage ayrıntısı göstermeme

Bunlar kesin kolon veya endpoint listesi değildir.

## 3. Ortak asset modeli ile domain ilişkilerinin ayrımı

Değerlendirilen yaklaşım:

- Fiziksel dosya metadata’sı ortak asset sorumluluğunda tutulur.
- Ham upload, güvenli canonical master ve işlenmiş varyant metadata’sı kavramsal olarak ayrılır.
- Advert–asset ilişkisi ayrı advert media sorumluluğudur.
- Banner–asset ilişkisi ayrı banner sorumluluğudur.
- Sıra ve kapak asset’e değil advert media ilişkisine aittir.
- Placement asset’e değil banner kaydına aittir.
- Aynı fiziksel dosyanın farklı domain kayıtlarında kontrolsüz paylaşımı engellenir.
- Referans sayısı ve güvenli silme ortak asset katmanında ele alınır.
- Domain ilişkisinin kaldırılması (`REMOVED` benzeri ilişki koparma) asset processing durumu değildir; asset’i yalnızca korunması gereken referans kalmadığında orphan adayı haline getirebilir.
- Bir advert media veya banner ilişkisi DB’de hâlâ mevcutsa asset referanslıdır; domain kaydının public veya ACTIVE olması zorunlu değildir.
- Banner’ı INACTIVE yapmak veya advert’i public görünürlükten çıkarmak asset’i orphan yapmaz.
- Advert veya banner status’u ile asset referans durumu aynı kavram değildir.
- Asset başka bir korunması gereken domain referansı tarafından kullanılıyorsa fiziksel silinemez.
- DB’de silme adayı işaretlenmesi ile B2 object’in gerçekten silinmesi aynı olay değildir.

### Karşılaştırma

| Kriter | Tamamen ayrı altyapılar | Ortak asset + ayrı domain ilişkileri |
| --- | --- | --- |
| Tekrar | Yüksek (çift işleme/storage kodu) | Düşük |
| Güvenlik | Ayrı kontrol, daha fazla yüzey | Merkezi upload/validasyon |
| Domain ayrımı | Güçlü ama pahalı | İlişki kurallarıyla korunabilir |
| İşleme | Çift TinyPNG/B2 yolu | Tek adapter/pipeline |
| Bakım | Daha zor | Daha kolay |
| Bu projeye uygunluk | Düşük | Yüksek |

**İlk faz önerisi:** Ortak asset altyapısı; advert media ve banner için ayrı domain ilişkileri ve kuralları.

## 4. Kaynak görselin saklanması

Ham upload ile güvenli canonical master ayrılır.

### Ham upload nesnesi

- Kullanıcının veya admin’in cihazından doğrudan yüklediği ilk dosyadır.
- Henüz güvenilir veya domain kullanımına hazır değildir.
- Private/quarantine alanında tutulmalıdır.
- Dosya uzantısı, istemci MIME bilgisi ve istemci metadata’sı güvenilir kabul edilmez.
- Decode, gerçek content/MIME, byte ve piksel kontrolleri yapılmadan kullanılmamalıdır.
- EXIF GPS ve diğer gizlilik riski taşıyan metadata içerebilir.
- Public API veya public URL üzerinden sunulmamalıdır.
- Domain için canonical yeniden işleme kaynağı olmamalıdır.
- Güvenli canonical master başarıyla oluşturulduktan sonra ayrı retention ihtiyacı yoksa cleanup adayı olabilir.
- Ham upload’ın saklama süresi bu dokümanda belirlenmez.

### Güvenli canonical kaynak/master görsel

- Ham upload doğrulandıktan sonra backend/worker tarafından oluşturulur.
- Orientation düzeltilmiş olmalıdır.
- EXIF GPS ve gereksiz metadata temizlenmiş olmalıdır.
- Yeniden varyant üretmeye uygun yüksek kaliteli ve mümkün olduğunca kayıpsız master kaynaktır.
- Public response’ta doğrudan kullanılmamalıdır.
- DETAIL, HOMEPAGE, SEARCH ve gelecekteki transform profilleri bu güvenli master kaynaktan üretilmelidir.
- TinyPNG varyant sıkıştırması canonical master’ı zorunlu olarak kayıplı biçimde ezmemelidir.
- Canonical master, korunması gereken herhangi bir domain referansı bulunduğu sürece saklanmalıdır.
- Canonical master ile kullanıcının yüklediği ham dosyanın aynı fiziksel object olması zorunlu değildir.

| Seçenek | Avantaj | Risk |
| --- | --- | --- |
| Ham upload’ı canonical saymak | Basit görünür | Güvensiz metadata; yeniden işleme kalitesi zayıf |
| Master’ı silip yalnız varyant bırakmak | Storage tasarrufu | Yeni profil/retry zorlaşır |
| Güvenli master’ı referanslı sürece saklamak; orphan sonrası cleanup | Güvenli yeniden işleme | Maliyet; retention ürün kararı |

**İlk faz önerisi:**

- Doğru yeniden işleme kaynağı güvenli canonical master’dır; ham upload değildir.
- Canonical master public response’ta kullanılmaz.
- Canonical master’ın hazır olması bütün varyantların READY olduğu anlamına gelmez.
- Herhangi bir korunması gereken advert media veya banner referansı varken canonical master ve gerekli varyantlar cleanup ile silinmez.
- Domain kaydının ACTIVE/public olması zorunlu değildir; ilişki DB’de mevcutsa asset referanslıdır.
- Advert media ilişkisinin kaldırılması tek başına hemen fiziksel silme anlamına gelmez.
- Orphan adaylığı, ilişkilerin açıkça kaldırılması, domain kaydının kalıcı kullanım dışı bırakılması veya ayrı retention politikasının tamamlanması sonrası değerlendirilir.
- Kesin retention süresi bu dokümanda belirlenmez; sonsuz saklama zorunlu değildir.

Public yalnız ilgili kullanım için READY işlenmiş varyantları kullanır.

**Soru:** Canonical master public response’ta kullanılmalı mı? **Hayır.** Ham upload public’te kullanılmalı mı? **Hayır.**

## 5. Görsel varyant stratejileri

### 5.1 Yalnız kaynak görsel

Bütün ekranlarda aynı büyük dosya. Mobil veri ve performans açısından zayıf.

### 5.2 Tam olarak üç sabit fiziksel varyant

DETAIL, HOMEPAGE ve SEARCH için daima üç farklı dosya. Legacy’ye bağımlı; aynı profilde duplicate üretir.

### 5.3 Dinamik / on-demand dönüşüm

İstemci talebinde boyutlandırma veya image CDN. İlk fazda ek karmaşıklık; B2+worker modeliyle zorunlu değil.

### 5.4 Yapılandırılabilir, önceden üretilmiş transform profilleri

- DETAIL, HOMEPAGE, SEARCH mantıksal kullanımlardır.
- Her kullanım bir transform profiline bağlanır.
- Aynı profile sahip kullanımlar aynı fiziksel varyantı paylaşır; o ortak fiziksel varyantın tek hazırlık durumu bulunur.
- Farklı profil ayrı varyant üretir; profiller ayrı ayrı başarılı veya başarısız olabilir.
- Bir varyantın READY olması diğer bütün varyantların READY olduğu anlamına gelmez.
- Ölçü/kalite config’ten yönetilir.
- Yeni kullanım için güvenli canonical master’dan yeniden üretim mümkündür.
- Retry yalnız başarısız veya eksik varyantı yeniden üretebilmelidir; aynı master + profil için duplicate varyant üretilmez.
- Asset için tek belirsiz `READY` değeri ile bütün varyantların hazır olduğu varsayılmaz.

| Kriter | Yalnız kaynak | Daima 3 dosya | Dinamik/on-demand | Yapılandırılabilir profiller |
| --- | --- | --- | --- | --- |
| Performans | Zayıf | İyi | Değişken | İyi |
| Mobil veri | Zayıf | İyi | İyi | İyi |
| Storage | Düşük | Yüksek risk | Düşük-orta | Orta, kontrollü |
| İşleme maliyeti | Düşük | Yüksek | Dağınık | Orta |
| FE sadeliği | Yüksek | Orta | Düşük | Yüksek |
| Cache | Zayıf | İyi | Karmaşık | İyi |
| B2 uygunluğu | Orta | Orta | Zayıf (CDN ihtiyacı) | Yüksek |
| TinyPNG | Tek dosya | 3× maliyet | Zor | Profil başına |
| Yeniden işleme | Zayıf | Orta | — | Güçlü (master var) |
| Bakım | Basit ama yetersiz | Legacy kilit | Ağır | Dengeli |
| Bu projeye uygunluk | Düşük | Düşük | Düşük (ilk faz) | Yüksek |

**Öneri:** Yapılandırılabilir, önceden üretilmiş transform profilleri. Legacy “mutlaka üç dosya” varsayımı reddedilir. Exact piksel ölçüleri bu dokümanda belirlenmez.

## 6. Görsel işleme sırası

Kavramsal güvenli hat:

1. Upload yetkisi ve advert sahiplik kontrolü
2. Beklenen dosya metadata’sının alınması
3. Ham upload’ın private/quarantine alana yüklenmesi
4. Ham upload üzerinde görsel decode edilebilirlik
5. Gerçek MIME/content kontrolü
6. Dosya byte boyutu kontrolü
7. Piksel boyutu ve aşırı büyük görsel kontrolü
8. Orientation düzeltme
9. Gizlilik riskli metadata temizliği (EXIF GPS vb.)
10. Güvenli canonical master’ın oluşturulması ve saklanması
11. Canonical master’dan gerekli resize/crop (transform profili başına)
12. TinyPNG adapter üzerinden varyant sıkıştırması (master’ı kayıplı ezmeden)
13. Varyantların Backblaze B2’ye yazılması
14. İlgili varyantın READY yapılması (canonical master durumundan ayrı)
15. Başarısızlıkta yalnız ilgili varyant için hata kaydı ve retry
16. Public response’ta yalnız ilgili kullanımın READY varyantı
17. Ayrı retention ihtiyacı yoksa ham upload’ın cleanup adayı olması

Ek değerlendirmeler:

- Ham upload canonical yeniden işleme kaynağı değildir; doğru kaynak güvenli master’dır.
- **TinyPNG sırası:** Master’dan türetilen varyant üzerinde resize/crop sonrası sıkıştırma tercih edilir; TinyPNG master’ı zorunlu kayıplı ezmez.
- Exact image processing library/SDK bu dokümanda seçilmez.
- TinyPNG geçici erişilemezse ilgili varyant FAILED kalır; sıkıştırılmamış dosya otomatik public edilmez; kontrollü retry.
- Retry’de duplicate object oluşmaması için idempotent processing / immutable key; aynı master + transform profili tekrar üretilmez.
- Canonical master hazır olsa bile zorunlu varyantlar hazır olmadan public görsel kullanılmaz.
- Sonradan eklenen opsiyonel yeni bir transform profilinin başarısız olması, mevcut zorunlu public varyantlar hâlâ hazırsa asset’i otomatik olarak tamamen kullanılamaz hale getirmez.
- Hangi profillerin zorunlu olduğu yapılandırmayla belirlenir.

Kesin queue teknolojisi veya kod yazılmaz.

## 7. Upload mimarisi

### 7.1 Backend proxy upload

İstemci → backend → Backblaze B2. Basit ama mobil büyük dosyalarda backend bant genişliği ve timeout riski yüksektir.

### 7.2 Kısa ömürlü yetkilendirilmiş doğrudan upload

- İstemci backend’den upload izni/session ister.
- Backend sahiplik ve advert durumunu kontrol eder.
- Backend kısa ömürlü, sınırlı upload yetkisi üretir.
- İstemci ham dosyayı doğrudan storage’a (private/quarantine) yükler.
- Backend completion doğrular.
- Worker doğrulama, master üretimi ve varyant işlemeyi başlatır.
- Kalıcı B2 credential istemciye verilmez.

| Kriter | Proxy upload | Direct upload (kısa ömürlü yetki) |
| --- | --- | --- |
| Mobil performans | Zayıf-orta | Yüksek |
| Backend bant genişliği | Yüksek yük | Düşük |
| Güvenlik | Merkezi | Yetki sınırlıysa yeterli |
| Sahiplik | Kolay | Backend pre-auth ile |
| Retry | Backend yükü | İstemci storage retry |
| Uygulama karmaşıklığı | Düşük | Orta |
| Bu projeye uygunluk | Düşük (mobil) | Yüksek |

**İlk faz önerisi:** Kısa ömürlü yetkilendirilmiş direct upload. Exact presigned URL/SDK/imza kodu yazılmaz.

## 8. Dosya güvenliği ve validasyon

Kavramsal kurallar:

- Ham upload üzerinde dosya uzantısına güvenilmez.
- İstemci MIME tek başına yeterli değildir.
- Gerçek content/decode doğrulaması gerekir.
- MIME allowlist kullanılır (kesin liste sonra).
- Maksimum byte, min/max piksel yapılandırılır (değer uydurulmaz).
- Decompression bomb / aşırı piksel riski dikkate alınır.
- Bozuk veya yarım dosya reddedilir.
- EXIF orientation düzeltilir; GPS ve gereksiz metadata temizlenerek güvenli canonical master üretilir.
- Ham upload public API/URL ile sunulmaz.
- Kullanıcı dosya adı object key olmaz; backend güvenli key üretir.
- Path traversal ve key çakışması engellenir.
- SVG gibi aktif içerik taşıyabilen formatlar varsayılan kabul edilmez.
- HEIC/HEIF mobil UX açısından değerlendirilir; kesin destek ürün/teknik doğrulama sonrası.
- Input format ≠ zorunlu output format; master ve varyant formatları ayrı karar olabilir.
- Kesin MIME/format listesi teknik destek doğrulandıktan sonra belirlenir.

Kullanıcı kitlesi ileri yaş ve mobil ağırlıklı olabilir; yükleme hataları anlaşılır olmalıdır.

## 9. Asset ve işleme durumları

`REMOVED`, ortak asset’in processing durumu değildir. Advert’ten görsel kaldırma veya banner–asset ilişkisinin açıkça kaldırılması domain ilişkisi işlemidir; asset processing durumu değildir. Banner’ı yalnız INACTIVE yapmak asset processing durumu değildir ve orphan üretmez.

### Kaynak tarafı lifecycle (kavramsal gruplar)

| Kavramsal grup | Anlam |
| --- | --- |
| Upload bekliyor | Upload yetkisi verildi; ham dosya henüz tamamlanmadı |
| Ham upload doğrulama bekliyor | Ham upload storage’da; decode/content kontrolü bekliyor |
| Doğrulama/normalizasyon yapılıyor | Decode, MIME, boyut, orientation, EXIF temizliği |
| Güvenli canonical master hazır | Master oluşturuldu; varyantlar henüz ayrı durumdadır |
| Kaynak doğrulama başarısız | Ham upload veya master üretimi başarısız; retry/cleanup adayı |
| Cleanup adayı | Korunması gereken domain referansı yok; cleanup sürecine alındı (DB işaretleme ≠ B2 silindi) |
| Fiziksel olarak silindi | B2 object(ler) başarıyla silindi |

Canonical master’ın hazır olması bütün transform varyantlarının hazır olduğu anlamına gelmez. Varyant hazırlığı profil bazında ayrı kalır. Domain moderasyon/public hazırlığı zorunlu varyant durumlarına göre belirlenir.

### Varyant durumu

- Her transform profili için ayrı işleme sonucu bulunabilir.
- DETAIL, HOMEPAGE ve SEARCH aynı profile bağlanıyorsa ortak fiziksel varyantın tek durumu bulunur.
- Farklı profiller ayrı ayrı başarılı veya başarısız olabilir.
- Bir varyantın READY olması diğer bütün varyantların READY olduğu anlamına gelmez.
- Retry yalnız başarısız veya eksik varyantı yeniden üretebilmelidir.
- Aynı canonical master ve transform profili için duplicate varyant üretilmemelidir.
- Asset için tek belirsiz `READY` değerine güvenerek bütün varyantların hazır olduğu varsayılmaz.

### Referanslı asset vs orphan

- Advert media veya banner ilişkisi DB’de mevcutsa asset referanslıdır.
- Banner INACTIVE olsa bile banner–asset ilişkisi korunuyorsa asset orphan değildir.
- Advert status değişikliği (`SOLD`, `ARCHIVED`, `SUSPENDED`, `REJECTED` vb.) medya ilişkisini otomatik kaldırmaz ve tek başına B2 cleanup başlatmaz.
- Orphan adaylığı için ilişkinin açıkça kaldırılması, domain kaydının kalıcı kullanım dışı bırakılması veya ayrı retention sürecinin tamamlanması gerekir.
- Fiziksel silmeden önce başka korunması gereken referans yeniden doğrulanır.
- Silme adayı işaretleme ile B2 fiziksel silme ayrı olaylardır; B2 silme başarısız olursa retry edilir.

**İlk faz önerisi:** Yukarıdaki kavramsal gruplar; kesin enum/kolon tasarımı yapılmaz.

## 10. Advert media ilişkisi

Değerlendirme ve öneriler:

- Advert ile asset arasında ayrı ilişki vardır.
- Sıralama advert media üzerindedir.
- Bir advert’te aynı anda yalnız bir kapak vardır.
- Kapak kaldırılırsa: kalan, ilgili zorunlu varyantı READY olan görsellerden ilk sıradaki otomatik kapak olabilir; hiç uygun READY yoksa kapaksız kalır.
- İlk eklenen uygun READY görsel otomatik kapak olabilir; kullanıcı manuel değiştirebilir.
- Aynı asset aynı advert’e iki kez eklenemez.
- Başka kullanıcının asset’i bağlanamaz.
- İlk fazda aynı asset’in başka advert’e bağlanması varsayılmaz (advert’e özel upload).
- Advert’ten görsel kaldırmak yalnız advert media ilişkisini koparır; asset processing durumunu `REMOVED` yapmaz ve anında fiziksel silme tetiklemez.
- Advert status değişikliği (`SOLD`, `ARCHIVED`, `SUSPENDED`, `REJECTED` vb.) medya ilişkisini otomatik kaldırmaz; ilişki ayrıca kaldırılmadığı sürece asset referanslıdır.
- Public olmayan advert’in media ilişkileri geçmiş, operasyon veya tekrar görüntüleme için korunabilir.
- Reorder atomik ve version korumalı olmalıdır.
- İşlenmemiş asset listeye eklenebilir ama public’te gösterilmez.
- Public response yalnız ilgili kullanım için READY varyant, sıra ve kapak kurallarına göre.
- Fotoğraflı ilan: en az bir, zorunlu public varyantı READY olan advert media ilişkisinden türetilir.

## 11. Advert status ve medya düzenleme sınırı

Çekirdek içerik düzenleme kararıyla uyumlu medya politikası:

| Durum | Ekle | Kaldır | Sırala | Kapak | FAILED retry |
| --- | --- | --- | --- | --- | --- |
| DRAFT | Evet | Evet | Evet | Evet | Evet |
| PENDING_REVIEW | Hayır | Hayır | Hayır | Hayır | Hayır |
| CHANGES_REQUESTED | Evet | Evet | Evet | Evet | Evet |
| PUBLISHED | Hayır | Hayır | Hayır | Hayır | Hayır |
| REJECTED | Hayır | Hayır | Hayır | Hayır | Hayır |
| SUSPENDED | Hayır | Hayır | Hayır | Hayır | Hayır |
| SOLD | Hayır | Hayır | Hayır | Hayır | Hayır |
| ARCHIVED | Hayır | Hayır | Hayır | Hayır | Hayır |

Gerekçe:

- Moderatör incelerken görsel listesi değişmez.
- Yayında moderasyonsuz medya değişikliği yok (revision yok).
- PUBLISHED’da “düşük riskli” sıralama/kapak ilk fazda açılmaz; içerik bütünlüğü riski vardır.
- CHANGES_REQUESTED’ta kullanıcı görsel düzeltebilir.
- SUSPENDED’ta kullanıcı sessizce yayına hazırlık yapamaz.

## 12. Moderasyona gönderme ve yayınlama medya kuralları

- Moderasyona gönderirken bağlı görsellerin mevcut fazda ihtiyaç duyulan zorunlu medya profilleri READY olmalıdır.
- Canonical master hazır olsa bile tek belirsiz asset `READY` varsayımı yeterli değildir; zorunlu varyant durumları kontrol edilir.
- Zorunlu bir varyant PROCESSING veya FAILED ise gönderim reddedilir (veya kullanıcıya net hata).
- En az bir görsel zorunluluğu **ürün kararıdır**; bu dokümanda sayı uydurulmaz.
- Kapak zorunluluğu: en az bir uygun READY görsel varsa kapak yoksa ilk uygun READY otomatik kapak önerilir.
- Moderatör incelemede güvenli master veya DETAIL profili kullanılabilir; kesin UI kararı BO’ya bırakılır. Ham upload ve canonical master public response değildir.
- Admin onayında zorunlu varyantlar hâlâ READY olmalıdır.
- Public’te yalnız ilgili kullanım için READY varyant döner.
- Yeniden işleme ve yeni profil üretimi güvenli canonical master’dan yapılır.
- Sonradan eklenen opsiyonel profil başarısız olsa bile mevcut zorunlu public varyantlar hazırsa advert medya açısından kullanılamaz hale gelmez.

## 13. Silme, ayırma ve orphan temizliği

Kavram ayrımı:

- Advert media ilişkisini kaldırmak ≠ asset’i silme adayı yapmak ≠ B2 object’i fiziksel silmek.
- Banner’ı INACTIVE yapmak asset’i kaldırmaz, silmez ve orphan yapmaz; banner–asset ilişkisi korunuyorsa asset referanslıdır.
- Advert’in public görünürlükten kalkması veya `SOLD` / `ARCHIVED` / `SUSPENDED` / `REJECTED` olması asset’i orphan yapmaz.
- Domain ilişkisinin açıkça kaldırılması, domain kaydının kalıcı kullanım dışı bırakılması veya ayrı retention politikasının tamamlanması sonrası orphan adaylığı değerlendirilir.
- Asset’in fiziksel silinme sürecine alınması ayrı bir cleanup/deletion durumudur.
- Canonical master ve gerekli varyantlar, korunması gereken herhangi bir referans varken cleanup ile silinmez.
- Fiziksel silmeden önce başka domain referansı yeniden doğrulanır.
- Soft delete edilmiş veya tarihsel korunması gereken ilişkilerin fiziksel silme davranışı retention politikasıyla belirlenir.
- Ham upload, güvenli master başarıyla oluştuktan sonra ayrı retention ihtiyacı yoksa cleanup adayı olabilir.
- Yarım upload, başarısız doğrulama, referanssız asset orphan/cleanup adayıdır.
- DB’de silme adayı işaretleme ile B2 fiziksel silme aynı olay değildir; B2 silme başarısız olursa retry edilir.

**İlk faz önerisi:**

- Kullanıcı görseli kaldırınca object anında silinmez.
- Korunması gereken referans varken canonical master ve gerekli varyantlar silinmez.
- Cleanup, orphan koşulları oluştuktan sonra güvenli bekleme ile başlar (süre uydurulmaz).
- B2 silme başarısızlığında retry.
- DB–storage reconciliation gerekir.
- Kesin retention süresi bu dokümanda belirlenmez; sonsuz saklama zorunlu değildir.

## 14. TinyPNG adapter kararı

İlkeler:

- Domain/use-case TinyPNG SDK/API detayına doğrudan bağımlı olmaz.
- Compression/processing port veya adapter arkasında kullanılır.
- Credential yalnız backend/worker secret ortamındadır.
- FE/BO TinyPNG credential almaz.
- Timeout, retry, rate limit/quota, geçici kesinti, kalıcı geçersiz dosya ayrılır.
- Processing idempotent olmalıdır.
- Provider değişikliği adapter değişimiyle mümkün olmalıdır.
- Testlerde fake adapter kullanılabilir.

**TinyPNG başarısızlığı:** Sıkıştırılmamış varyant otomatik public edilmez. İlgili varyant FAILED kalır; kontrollü retry. İlk fazda ayrı fallback processor zorunlu değildir. TinyPNG canonical master’ı zorunlu kayıplı ezmez.

## 15. Backblaze B2 object key ve erişim ilkeleri

Kavramsal ilkeler:

- Object key backend üretir; kullanıcı dosya adından bağımsızdır.
- Ortam, ham upload / canonical master / varyant, advert media / banner ayrımı key stratejisinde kavramsal olarak yansıtılabilir.
- Tahmin edilmesi zor / çakışmasız kimlik kullanılır.
- Object metadata’ya körü körüne güvenilmez.
- Public bucket vs private + signed URL ürün/teknik karardır; kalıcı credential istemciye verilmez.
- Ham upload private/quarantine erişimde tutulur; public sunulmaz.
- Kısa ömürlü upload yetkisi.
- Aynı key üzerine sürekli overwrite yerine immutable/versioned yaklaşım.
- Yeni işleme yeni object üretir; eski object cleanup ile gider.
- Canonical master, korunması gereken domain referansı varken cleanup ile silinmez.
- Ham upload ve canonical master public response’ta kullanılmaz; public yalnız READY varyant object’lerini kullanır.

Kesin bucket adı, klasör yolu veya key formatı tasarlanmaz.

## 16. Banner domain modeli

Banner sorumlulukları:

- Placement (`HOMEPAGE`, `LISTING_DETAIL`, `SEARCH`)
- Görsel asset
- Aktif/pasif
- Sıralama/öncelik ihtiyacı (ürün)
- Başlangıç/bitiş zamanı ihtiyacı (ürün)
- Başlık / erişilebilirlik metni (ürün)
- Hedef bağlantı (ürün)
- BO oluşturma/güncelleme
- Public gösterim
- Geçersiz placement ve ölçü reddi
- Görsel değiştirme
- Audit ihtiyacı (ürün)

Kesin kabul:

- Placement değerleri: HOMEPAGE, LISTING_DETAIL, SEARCH.
- Banner yalnız BO’dan yönetilir.
- Placement’a özel görsel validasyonu gerekir.
- Banner TinyPNG adapter + B2 kullanır.
- Normal kullanıcı banner oluşturamaz/değiştiremez.
- Banner, advert media ilişkisini kullanmaz.

Açık bırakılanlar: kesin ölçüler, MIME, byte limit, hedef URL, sıralama, scheduling, placement başına gösterim sayısı.

**İlk faz minimum lifecycle:** Banner kaydı + asset işleme + ACTIVE/INACTIVE; placement için gerekli varyant READY olmadan banner aktif olamaz.

## 17. Banner görsel doğrulama ve işleme

Her placement için yapılandırılabilir profil:

- Beklenen en/boy veya oran
- MIME allowlist
- Maksimum byte
- Resize/crop davranışı
- Kalite/sıkıştırma
- Public output formatı
- Mobil ve web ihtiyaçları

**İlk faz önerisi:** Yanlış ölçülü banner sessiz otomatik crop ile “düzeltilmez”; BO’ya açık validasyon hatasıyla reddedilir (veya ürün onaylı kontrollü transform — varsayılan: reddet). Placement değişince mevcut asset yeniden doğrulanır. Aynı görselin farklı placement’larda kontrolsüz paylaşımı varsayılmaz. Exact profil değerleri config’ten yönetilir. Banner ACTIVE olabilmesi için kendi placement profilinin gerekli varyantı READY olmalıdır; tek belirsiz asset `READY` varsayımı yeterli değildir. Placement’a özel varyant durumu, canonical master durumundan ayrı değerlendirilir.

## 18. Banner aktiflik ve public görünürlük

| Model | İlk faz sadeliği | READY güvenliği | Scheduling | Uygunluk |
| --- | --- | --- | --- | --- |
| ACTIVE/INACTIVE | Yüksek | Placement varyant READY kontrolüyle yeterli | Sonra eklenebilir | Yüksek |
| DRAFT/ACTIVE/INACTIVE | Orta | İyi | Orta | Orta |
| Zamanlanmış geniş lifecycle | Düşük | İyi | Yerleşik | Düşük (ilk faz) |

**Öneri:** İlk fazda ACTIVE/INACTIVE. Placement için gerekli varyant READY değilse veya FAILED ise banner ACTIVE olamaz. Public’te yalnız aktif + placement varyantı READY + geçerli placement. BO önizleme master/işleme/varyant durumunu görebilir. Banner’ı INACTIVE yapmak asset’i kaldırmaz veya silmez; banner–asset ilişkisi korunuyorsa asset orphan değildir ve tekrar aktif hale getirilebilir. Orphan olabilmesi için banner–asset ilişkisinin açıkça kaldırılmış olması veya banner için geçerli retention sürecinin tamamlanmış olması gerekir. Advert status makinesi ile karıştırılmaz.

## 19. FE, BO ve backend sorumlulukları

### Haradan FE

- Yalnız kendi advert görsellerini yönetir.
- Kalıcı B2 veya TinyPNG credential taşımaz.
- Upload ilerlemesi ve hataları anlaşılır gösterir.
- Yalnız izinli advert durumlarında medya işlemi sunar.
- READY olmayan görseli public hazır gibi göstermez.
- Admin banner ekranı içermez.
- Public advert/banner’da backend’in seçtiği URL/varyantları kullanır.
- Object key üretmez; güvenlik kararı vermez.

### Haradan BO

- Banner oluşturma/güncelleme/aktif-pasif.
- Placement validasyon hatalarını gösterir.
- İşleme durumunu görüntüler; retry sunabilir.
- Moderasyonda advert görsellerini görüntüler.
- Kullanıcı advert görsellerini sessizce değiştirmeyi ana yaklaşım yapmaz.
- Exact ekran tasarımı bu dokümanda yok.

### Backend/worker

- Upload authorization ve sahiplik
- Güvenli object key
- Completion doğrulaması
- Dosya güvenlik validasyonu
- Decode/normalize/master üretimi/resize/compress
- TinyPNG adapter
- B2 yazma/silme
- Varyant üretme
- Processing state ve retry
- Advert media sıra/kapak bütünlüğü
- Banner placement validasyonu
- Public varyant seçimi
- Orphan cleanup
- İstemci storage metadata’sına körü körüne güvenmeme

## 20. Eş zamanlılık ve idempotency

Riskler: çift completion, çift worker işleme, iki cihazdan reorder, çift kapak seçimi, kaldırma sırasında processing, eski worker sonucunun yeni kaydı ezmesi, retry duplicate, upload–DB tutarsızlığı, ilişki kaldırılırken cleanup yarışı, master hazırken varyant yarışı.

**İlk faz kavramsal yaklaşımlar:**

- Reorder beklenen version ile
- Kapak değişimi transaction içinde tek kapak garantisi
- Ham upload doğrulama, master üretimi ve varyant processing idempotent
- Aynı transform profili aynı master için tekrar üretilmez (veya aynı logical key)
- Retry yalnız başarısız/eksik varyantı hedefler
- Güncel olmayan worker sonucu reddedilir
- Domain ilişki kaldırma ile silme adayı/cleanup ayrı adımlardır; korunması gereken referans varken fiziksel silme yapılmaz
- Status değişikliği (INACTIVE, SOLD, ARCHIVED vb.) otomatik cleanup tetiklemez
- Reconciliation/cleanup
- Immutable object key

Kesin kolon veya idempotency key formatı yazılmaz.

## 21. Karşılaştırma tabloları

### 21.1 Asset altyapısı

(Bkz. bölüm 3 tablosu.)

### 21.2 Görsel varyant stratejisi

(Bkz. bölüm 5 tablosu.)

### 21.3 Upload yaklaşımı

(Bkz. bölüm 7 tablosu.)

### 21.4 Banner durum modeli

(Bkz. bölüm 18 tablosu.)

## 22. Önerilen karar

**Özet model:** Ortak asset altyapısı + ayrı advert media / banner domain ilişkileri; ham upload ≠ güvenli canonical master; master’dan yapılandırılabilir önceden üretilmiş transform profilleri ve varyant bazlı READY; kısa ömürlü direct upload; TinyPNG adapter (master’ı ezmeden); medya düzenleme yalnız DRAFT ve CHANGES_REQUESTED; banner ACTIVE/INACTIVE + placement varyant READY; referanslı asset ≠ orphan; domain ilişki kaldırma ≠ asset silme.

Soru cevapları:

1. **Tamamen ayrı asset altyapısı?** Hayır.
2. **Ortak asset + ayrı domain ilişkileri?** Evet.
3. **Canonical master saklanmalı mı?** Evet; korunması gereken referans varken saklanır; orphan sonrası kontrollü cleanup; sonsuz saklama zorunlu değil. Ham upload ayrıdır.
4. **Ham upload veya master public response’ta mı?** Hayır.
5. **Daima üç fiziksel varyant?** Hayır.
6. **DETAIL/HOMEPAGE/SEARCH?** Mantıksal kullanım → transform profili; master’dan üretilir.
7. **Aynı profilde dosya paylaşımı?** Evet; ortak fiziksel varyantın tek durumu vardır.
8. **Varyantlar önceden mi?** Evet.
9. **Dinamik image CDN ilk faz?** Hayır.
10. **TinyPNG doğrudan domain’e mi?** Hayır; adapter.
11. **TinyPNG fail → sıkıştırmasız public?** Hayır.
12. **Upload?** Kısa ömürlü direct upload (ham → quarantine).
13. **FE/BO kalıcı B2 credential?** Hayır.
14. **Gerçek MIME/decode?** Evet (ham upload üzerinde).
15. **EXIF/GPS temizliği?** Evet; master üretiminde.
16. **Kullanıcı dosya adı = object key?** Hayır.
17. **Sıra nerede?** Advert media ilişkisinde.
18. **Kaç kapak?** Bir.
19. **Otomatik kapak?** Evet, ilk uygun READY (kapak yoksa).
20. **READY olmayan moderasyona?** Hayır; zorunlu medya profilleri READY olmalı.
21. **Minimum görsel sayısı bu dokümanda?** Hayır; ürün kararı.
22. **Medya hangi durumlarda?** DRAFT, CHANGES_REQUESTED.
23. **PUBLISHED medya değişir mi?** İlk fazda hayır.
24. **Kaldırınca anında B2 silme?** Hayır; ilişki kaldırma ≠ fiziksel silme.
25. **Orphan cleanup?** Evet; korunacak referans kalmadığında / retention sonrası.
26. **Banner placement?** HOMEPAGE, LISTING_DETAIL, SEARCH.
27. **Placement doğrulaması?** Backend.
28. **Banner advert media kullanır mı?** Hayır.
29. **Banner exact ölçüler bu dokümanda?** Hayır.
30. **Yanlış ölçü?** Reddet (sessiz crop yok).
31. **Banner lifecycle?** ACTIVE/INACTIVE (+ placement varyant READY).
32. **READY olmayan aktif?** Hayır (placement varyant READY şart).
33. **Public banner?** Aktif + placement varyant READY + geçerli placement.
34. **Processing idempotent?** Evet; varyant bazında.
35. **Reorder/kapak concurrency?** Evet.

**Gerekçe:** Mobil UX, kalite, performans, güvenlik, moderasyon bütünlüğü, B2 + TinyPNG, sade MVP, gelecekte yeni boyutlar, gereksiz dosya üretmeme, FE–BO ayrımı, ham–master–varyant–referans ayrımı.

## 23. Reddedilen yaklaşımlar

- Bütün ekranlarda kaynak görsel
- Legacy nedeniyle koşulsuz daima üç fiziksel dosya
- Aynı profilde HOMEPAGE/SEARCH duplicate dosya
- Canonical master’ı silip yeniden işleme imkânını kaybetmek
- Ham upload’ı canonical yeniden işleme kaynağı saymak
- TinyPNG’yi handler/domain’e gömmek
- TinyPNG ile canonical master’ı zorunlu kayıplı ezmek
- TinyPNG hatasında doğrulanmamış/sıkıştırmasız public
- FE/BO’ya kalıcı B2 credential
- Yalnız uzantı veya client MIME’a güvenmek
- Kullanıcı dosya adını object key yapmak
- READY olmayan public gösterim
- PENDING_REVIEW sırasında medya değişikliği
- PUBLISHED medyayı moderasyonsuz değiştirmek
- Birden fazla kapak
- Referans kontrolü olmadan anında object silme
- Banner placement doğrulamasını yalnız BO UI’ye bırakmak
- Yanlış ölçülü banner’ı sessizce bozacak otomatik crop
- Advert media ile banner domain’ini tek ilişkide birleştirmek
- Object key üzerine sürekli overwrite
- Retry’de duplicate varyant
- İlk fazda zorunlu olmayan dinamik image CDN
- Exact gereksinim olmadan bütün formatları kabul etmek
- Domain ilişki kaldırmayı asset processing durumu (`REMOVED`) saymak
- Tek belirsiz asset `READY` ile bütün varyantların hazır olduğunu varsaymak
- Bütün master görselleri koşulsuz sonsuza kadar saklamak
- Banner INACTIVE veya advert public değil diye asset’i orphan saymak

## 24. Açık kalan ürün ve teknik kararlar

- Advert başına min/max görsel sayısı
- Moderasyona gönderimde en az bir görsel zorunluluğu
- Kesin DETAIL / HOMEPAGE / SEARCH transform profilleri
- HOMEPAGE ve SEARCH profillerinin aynı olup olmayacağı
- Hangi transform profillerinin zorunlu / opsiyonel olduğu
- Exact input MIME allowlist
- HEIC/HEIF desteği
- Exact output formatları (master ve varyant)
- Maximum upload byte; min/max piksel
- Crop mı fit mi
- TinyPNG quota ve hata politikası
- Ham upload retention süresi (master oluştuktan sonra; süre uydurulmaz)
- Orphan olduktan sonra master/varyant cleanup bekleme süresi (korunan referans varken silinmez; süre uydurulmaz)
- Soft delete / tarihsel ilişki retention politikası
- Bucket/public erişim politikası
- CDN veya custom domain
- Banner placement kesin ölçüleri
- Banner MIME ve byte limitleri
- Banner hedef bağlantı modeli
- Banner sıralama/öncelik
- Banner scheduling
- Placement başına gösterilecek banner sayısı
- Banner audit ihtiyacı
- BO’nun advert görseli kaldırma yetkisi
- Görsel moderasyonu / uygunsuz içerik kontrolü

## 25. Sonraki adım

Bu karar kabul edilirse sonraki teknik karar kullanıcı, authentication ve session modelidir:

- User çekirdek alanları
- `user` / `admin` rolü
- Kayıt
- Giriş
- Parola saklama
- Access ve refresh session yaklaşımı
- Logout
- Session iptali
- E-posta doğrulama
- Parola sıfırlama
- Hesap pasifleştirme
- FE ve BO authentication ayrımı
- Rate limiting ve brute-force koruması

Bu dokümanda user tablosu, auth API’si, token formatı veya kod üretilmez.
