# Horse ve TJK Veri Modeli Kararı

## 1. Problem tanımı

Aşağıdaki kavramlar birbirinden ayrılır:

- **Haradan içindeki horse kaydı:** Atın Haradan’daki kalıcı domain kaydıdır. Haradan’ın kendi teknik horse kimliği vardır.
- **TJK harici kimliği:** TJK numarasıdır. Aynı atı senkronizasyonlar arasında eşleştirmek için temel harici kimliktir.
- **Sık kullanılan ve filtrelenen temel horse alanları (TJK kaynaklı):** TJK numarası, gerçek/orijinal at adı, doğum yılı, anne/baba adı, ırk, cinsiyet, don.
- **Haradan türev arama değeri:** Normalize edilmiş at adı; gerçek at adından arama amacıyla türetilir. TJK’nın bağımsız kaynak alanı değildir; kullanıcı girmez; kimlik değildir.
- **Değişken ve geniş TJK detayları:** Pedigri, yarış istatistikleri, kardeşler, yavrular, antrenör geçmişi gibi iç içe/değişken kaynak verisi (kontrollü domain-detail JSONB).
- **TJK senkronizasyon metadata’sı:** Sync durumu, zaman, hata ve operasyonel izleme bilgisi.
- **Advert ile horse ilişkisi:** Advert, horse kaydının kendisi değildir; seçilen horse’a referans verir. Aynı horse zaman içinde birden fazla advert ile ilişkili olabilir.

Açık ilkeler:

- Horse, atın kalıcı domain kaydıdır.
- Advert horse verisini kopyalamaz; horse kaydına referans verir.
- Kullanıcıya gösterilen değer her zaman gerçek/orijinal at adıdır.
- Horse adı ve normalize edilmiş ad tek başına kimlik değildir.
- TJK numarası temel harici kimliktir.
- Haradan teknik horse kimliği ile TJK numarası aynı amaçta kullanılmaz.

Bu dokümanda tablo veya kolon tasarımı yapılmaz.

## 2. Fonksiyonel gereksinimler

Horse modelinin desteklemesi gereken ihtiyaçlar (kesin kolon listesi değildir):

- Kullanıcının at adından birkaç karakterle arama yapması (gerçek ad gösterilir; arama Haradan’ın türettiği normalize değer üzerinden yapılır)
- Aynı veya benzer isimli atların ayırt edilebilmesi
- TJK numarasıyla doğrudan horse bulma
- İlan oluştururken horse seçme
- TJK alanlarının otomatik veya salt okunur gösterilmesi
- Doğum yılından yaş filtresi
- Irk / cinsiyet / don filtresi
- Anne ve baba bilgisinin gösterilmesi
- Pedigri ve yarış detaylarının gerektiğinde gösterilebilmesi
- Düzenli TJK senkronizasyonu (ad güncellenince normalize arama değerinin yeniden üretilmesi)
- Aynı verinin tekrar oluşturulmaması
- Mevcut advert ilişkilerinin senkronizasyonda bozulmaması
- TJK kaydı eksik veya geçici erişilemezken mevcut ilanların korunması
- BO’dan senkronizasyon durumu ve hata takibi
- Kullanıcıya gereksiz manuel alan sorulmaması

## 3. Değerlendirilen horse saklama seçenekleri

### 3.1 Horse verisinin tamamını tek JSONB içinde tutmak

Temel kimlik dışında bütün TJK verisi tek JSONB dokümanında tutulur.

**İlk geliştirme sadeliği / şema uyumu.** Hızlı başlar; TJK yapısı değişince kolon migration’ı azalır.

**Autocomplete / TJK numarası / filtreler.** Ad, doğum yılı, ırk, cinsiyet, don JSONB path’lerine kayarsa tip güvenliği ve indeksleme zayıflar; sorgu maliyeti yükselir.

**Duplicate kontrolü.** TJK numarası benzersizliği JSONB içinde daha kırılgandır.

**BO / raporlama.** Operasyonel ve analitik kullanım zorlaşır.

**Avantajlar.** Esneklik, az kolon.

**Dezavantajlar.** Autocomplete ve filtre performansı zayıf; tip güvenliği düşük; bu proje için uygun değil.

### 3.2 Bütün horse verisini normal kolon ve ilişkisel yapılarla tutmak

Temel alanlarla birlikte pedigree, yarış istatistikleri, kardeşler, yavrular ve antrenör geçmişi tamamen ilişkisel modellenir.

**Veri bütünlüğü / sorgulama.** Güçlüdür.

**TJK şeması değişimi / tablo sayısı / sync.** Migration ve sync karmaşıklığı yüksek; ilk faz süresi uzar; değişken dış kaynağa uyum zayıftır.

**Avantajlar.** İlişkisel netlik, bazı raporlar.

**Dezavantajlar.** Erken aşırı model; gereksiz tablo/migration; ilk faz için uygun değil.

### 3.3 Hibrit horse modeli

- Haradan teknik horse kimliği ayrı tutulur.
- TJK numarası benzersiz harici kimlik olarak kullanılır.
- Sık aranan/gösterilen/filtrelenen temel TJK alanları normal ve tipli tutulur.
- Geniş, iç içe, değişken ve ilk fazda doğrudan filtrelenmeyen TJK detayları kontrollü domain-detail JSONB içinde tutulur.
- Sync ve kaynak durumu için gerekli operasyonel metadata horse ile ilişkili tutulur.
- Advert yalnız horse referansı taşır.
- Horse alanları advert JSONB’ye kopyalanmaz.
- Normalize arama değeri Haradan tarafından gerçek addan türetilir.

Bu yaklaşım autocomplete/filtre ihtiyaçlarını tipli alanlarla karşılar; değişken TJK detaylarını aşırı normalize etmeden taşır.

## 4. Temel alanlar ile değişken detayların ayrımı

### Temel alanlar (normal ve tipli)

| Alan | Kaynak | Gerekçe özeti |
| --- | --- | --- |
| TJK numarası | TJK | Benzersiz harici kimlik, doğrudan bulma, idempotent sync |
| At adı (gerçek/orijinal) | TJK | Gösterim, ayırt etme; kullanıcıya gösterilen ad |
| Normalize edilmiş at adı | Haradan türev | Gerçek addan türetilen arama değeri; autocomplete / prefix; kimlik değil |
| Doğum yılı | TJK | Yaş filtresi kaynağı, ayırt etme, tip güvenliği |
| Baba adı | TJK | Gösterim ve ayırt etme |
| Anne adı | TJK | Gösterim ve ayırt etme |
| Irk | TJK | Filtre ve gösterim |
| Cinsiyet | TJK | Filtre ve gösterim |
| Don | TJK | Filtre ve gösterim |

TJK kaynaklı temel alanlar autocomplete destekli gösterim, filtreleme, sıralama, tip güvenliği, benzersizlik (yalnız TJK numarası) ve sync güncellemesi için tipli tutulmalıdır. Normalize edilmiş ad tipli tutulabilir; kaynağı Haradan’dır, kullanıcı girmez, TJK bağımsız alanı değildir. At adı değişince veya TJK’dan düzeltilmiş ad gelince normalize değer aynı güncellemede yeniden üretilir.

### Kontrollü domain-detail JSONB

Seçilmiş geniş/değişken TJK detayları:

- Pedigri
- Yarış istatistikleri
- Kardeşler
- Yavrular
- Antrenör geçmişi

**Uygunluk.** İlk fazda doğrudan filtrelenmeyen, iç içe ve değişken kaynak detayları için ayrı tablo açmak maliyetlidir. Yoğun sorgu veya bağımsız domain ihtiyacı doğarsa ilgili detail bölümü sonra normalize edilebilir.

**Disiplin.** Kontrollü detail JSONB:

- Normal ve tipli tutulmuş temel horse alanlarını ikinci kez içermez.
- Uygulamanın kontrollü biçimde okuyacağı detail yapısıdır.
- Public API’ye doğrudan verilmez.
- İlk faz public filtrelerinin kaynağı değildir.
- Kontrolsüz veri çöplüğü olmamalıdır.

**Canonical/doğru kaynak:** Uygulama için canonical horse verisi normal ve tipli temel alanlar ile kontrollü domain-detail verisidir.

### Olası ham TJK kaynak payload’ı

- Entegrasyon, audit veya hata inceleme ihtiyacı doğrulanırsa ileride ayrıca saklanması değerlendirilebilir.
- Kaynak payload doğal olarak temel alanları da içerebilir; bu, domain verisinin iki ayrı doğruluk kaynağı olduğu anlamına gelmez.
- Ham payload filtreleme, autocomplete, normal public response veya kullanıcı gösterimi için kullanılmaz.
- Ham payload kullanıcı veya admin tarafından düzenlenmez.
- Ham payload’ın saklanıp saklanmayacağı bu dokümanda kesinleştirilmez.
- Bu aşamada ham payload için tablo, kolon veya ikinci JSONB alanı tasarlanmaz.

## 5. Horse kimliği ve benzersizlik

- **Haradan teknik horse kimliği:** İç referans ve advert FK için kalıcıdır; sync’te değişmez.
- **TJK numarası:** Temel benzersiz harici kimliktir.
- **Gerçek/orijinal horse adı:** TJK kaynaklıdır; benzersiz değildir; kullanıcıya gösterilir.
- **Normalize edilmiş at adı:** Haradan’ın gerçek addan türettiği arama değeridir; benzersiz değildir; kimlik değildir; TJK bağımsız alanı değildir.
- Sync, TJK numarasına göre idempotent upsert yapar.
- Aynı TJK numarasıyla ikinci kayıt oluşturulmaz.
- TJK numarası hatalı/çelişkili gelirse sessizce yeni horse oluşturulmaz; operasyonel hata olarak görünür.
- Advert foreign key ilişkileri teknik kimlik üzerinden korunur.

İlkeler:

- Gerçek at adı benzersiz değildir.
- Normalize edilmiş ad benzersiz değildir ve horse kimliği olarak kullanılmaz.
- TJK numarası temel benzersiz harici kimliktir.
- Aynı TJK numarasıyla çelişen veri sessizce yeni horse oluşturmamalıdır.
- Duplicate veya kimlik çelişkisi operasyonel hata olarak görünür olmalıdır.

Kesin constraint veya SQL yazılmaz.

## 6. Anne, baba ve pedigree ilişkileri

Seçenekler:

1. Anne/baba yalnız TJK’dan gelen ad metni olarak tutulur.
2. Anne/baba doğrudan başka horse kayıtlarına self-reference ile bağlanır.
3. Hem kaynak adı korunur hem güvenilir kimlik varsa horse referansı oluşturulur.

Bağlam: Anne ve baba adı alınabilir; her zaman güvenilir TJK kimliği geldiği kesin değildir. Yalnız isimle otomatik eşleştirme hatalıdır; aynı isimli horse’lar vardır; eksik kimlikte sahte referans kurulmamalıdır.

**İlk faz önerisi:** Anne ve baba bilgisi yalnız ad metni olarak temel alanlarda tutulur. Self-reference kurulmaz. Pedigri, kardeşler, yavrular ve antrenör geçmişi ilk fazda ayrı ilişki tablolarına dönüştürülmez; kontrollü domain-detail JSONB’de tutulur. Güvenilir anne/baba TJK kimliği doğrulanırsa ileride kontrollü referans değerlendirilebilir.

## 7. Horse autocomplete ve arama

Kullanıcı akışı:

1. Kullanıcı at adından birkaç karakter yazar.
2. FE backend’e arama isteği gönderir.
3. Backend Haradan PostgreSQL’de arar.
4. Canlı TJK çağrısı yapılmaz.
5. Sonuçlar ayırt edici sınırlı bilgilerle gösterilir.
6. Kullanıcı horse kaydını seçer.
7. Advert seçilen horse’a bağlanır.
8. TJK temel bilgileri otomatik veya salt okunur gösterilir.

Değerlendirme:

- TJK’dan gelen kaynak ad gerçek/orijinal addır; kullanıcıya bu ad gösterilir.
- Normalize edilmiş at adı Haradan tarafından gerçek addan türetilir; kullanıcı girmez; TJK bağımsız alanı değildir.
- Türkçe karakter ve büyük/küçük harf davranışı normalize değerde ele alınır; prefix arama bu değer üzerinden yapılır.
- Tam eşleşme ve TJK numarasıyla arama desteklenir.
- Yazım toleranslı (fuzzy) arama ilk fazda zorunlu değildir.
- 2–3 karakterlik arama sonuçları limit/pagination ile sınırlandırılır.
- Aynı isimli atlar doğum yılı, TJK numarası, anne/baba adıyla ayırt edilir.
- Normalization, kullanıcıya gösterilen gerçek adı değiştirmez.
- At adı sync ile değişince veya düzeltilince normalize değer aynı süreçte yeniden üretilir.

**İlk faz önerisi:** Haradan türev normalize ad üzerinde prefix arama + TJK numarası ile doğrudan bulma; sonuç limiti; ayırt edici alanlarla listeleme. Fuzzy arama sonraya bırakılır. Kesin SQL/extension/indeks komutu yazılmaz.

## 8. Horse filtreleri

| Filtre | Kaynak / davranış |
| --- | --- |
| At adı | Temel horse alanı / arama |
| TJK numarası | Temel horse alanı / doğrudan bulma |
| Doğum yılı | Temel horse alanı |
| Yaş | Kalıcı tutulmaz; güvenilir doğum yılından sorgu anında doğum yılı aralığına dönüştürülür |
| Irk | Temel horse alanı; advert properties JSONB’den okunmaz |
| Cinsiyet | Temel horse alanı; advert properties JSONB’den okunmaz |
| Don | Temel horse alanı; advert properties JSONB’den okunmaz |

Ek ilkeler:

- Horse filtreleri advert–horse ilişkisi üzerinden uygulanır.
- İl ve ilçe horse alanı değildir.
- İdmanda, kiralık, koşar durumda gibi özellikler horse temel TJK alanı olarak otomatik kabul edilmez.
- Pedigri/yarış detayları için ilk fazda filtre varsayılmaz.
- Değişken JSONB detaylarına ölçülmemiş genel indeks açılmaz.

## 9. TJK senkronizasyon modeli

Kavramsal sync:

- İlk toplu veri alma
- Sonraki güncellemeler
- TJK numarasına göre idempotent upsert
- Yeni horse oluşturma; mevcut TJK kaynaklı temel alanları güncelleme
- Gerçek at adı güncellenince normalize arama değerinin aynı işlemde yeniden üretilmesi
- Haradan teknik kimliğini koruma
- Temel alanlar ile kontrollü domain-detail JSONB’nin tutarlı güncellenmesi
- Ham TJK payload saklama kararı açık ise, domain verisinden ayrı ve uygulama canonical kaynağı olmayacak biçimde ele alınması
- Başarılı/başarısız sync izleme, son başarılı sync zamanı
- Kaynak güncelleme zamanı TJK tarafından veriliyorsa kullanılması
- Hata mesajı ve retry
- BO’dan durum görüntüleme ve gerekli retry
- Checkpoint / kaldığı yerden devam ihtiyacı operasyonel olarak değerlendirilir
- Aynı kayıt tekrarında duplicate oluşturmama
- Kısmi cevabın mevcut doğru alanları yanlışlıkla silmemesi
- Kaynakta geçici görünmeyen kaydın doğrudan silinmemesi

Bu dokümanda kesinleştirilmez: sync sıklığı; TJK erişim yöntemi (API/dosya/scraping); worker queue teknolojisi; Redis/Kafka/NATS; kesin job tablo modeli; ham payload’ın saklanıp saklanmayacağı.

## 10. Eksik, hatalı ve değişen TJK verisi

Durumlar: null/eksik temel alanlar; geçici erişilemezlik; çelişkili TJK numarası; ad/doğum yılı/ırk/cinsiyet/don güncellemesi; JSONB şekil değişimi; sonraki sync’te görünmeme; mevcut advert etkisi.

İlkeler:

- Eksik alan için normal kullanıcıdan manuel düzeltme istenmez.
- Admin TJK kaynak alanını doğrudan manuel değiştirmez.
- BO sync durumu, hata ve retry yönetebilir.
- Tek geçici eksiklik nedeniyle horse fiziksel silinmez.
- Existing advert ilişkisi korunur.
- Kaynak eksikse FE yalnız mevcut güvenilir alanları gösterir.
- Kısmi cevapta mevcut doğru alanların silinmesi kontrollü merge ile engellenir.

**İlk faz önerisi:** Upsert + kontrollü merge; eksik gelen alanları kör silme; fiziksel silme yok; kimlik çelişkisini operasyonel hata olarak işaretle; kullanıcı/admin override yok.

## 11. Güncel horse verisi ve advert geçmişi

Seçenekler:

1. Advert her zaman güncel horse verisini gösterir.
2. Advert oluşturma anında tam horse snapshot saklar.
3. Yalnız bazı alanlar snapshot alınır.
4. Ayrı horse değişiklik geçmişi tutulur.

Denge: Horse alanlarının advert JSONB’ye kopyalanmaması; sade ilk faz; tekrar azaltma; TJK düzeltmelerinin yansıması; tarihsel anlam; audit; zamanla değişen yarış detayları.

**İlk faz önerisi:** Advert her zaman ilişkili horse kaydının güncel verisini gösterir. Tam/kısmi snapshot ve ayrı değişiklik geçmişi ilk fazda kurulmaz. İleride ölçülmüş ihtiyaç olursa değerlendirilir.

## 12. JSONB yönetim ilkeleri

### Kontrollü domain-detail JSONB

- Pedigri, yarış istatistikleri, kardeşler, yavrular ve antrenör geçmişi gibi seçilmiş geniş/değişken detayları taşır.
- Normal ve tipli tutulmuş temel horse alanlarını ikinci kez içermez.
- Uygulamanın kontrollü biçimde okuyacağı detail yapısıdır.
- Public API’ye doğrudan verilmez.
- İlk faz public filtrelerinin kaynağı değildir.
- Bilinmeyen alanlarla sınırsız büyütülmez; boyut izlenir.
- Gerçek sorgu ihtiyacı doğarsa seçili detail alanları sonra normalize/project edilebilir.

### Olası ham TJK kaynak payload’ı

- Entegrasyon, audit veya hata inceleme ihtiyacı doğrulanırsa ileride ayrıca saklanması değerlendirilebilir.
- Kaynak payload doğal olarak temel alanları da içerebilir; bu tekrar domain için ikinci doğruluk kaynağı oluşturmaz.
- Canonical/doğru kaynak: tipli temel alanlar + kontrollü domain-detail.
- Ham payload filtreleme, autocomplete, normal public response veya kullanıcı gösterimi için kullanılmaz.
- Ham payload kullanıcı veya admin tarafından düzenlenmez.
- Saklanıp saklanmayacağı bu dokümanda kesinleştirilmez; şimdi tablo/kolon/ikinci JSONB alanı tasarlanmaz.

Kesin JSON şeması üretilmez.

## 13. FE, BO ve backend etkisi

### FE

- Canlı TJK çağrısı yapmaz.
- Haradan backend üzerinden horse arar.
- At adından autocomplete sağlar; kullanıcıya gerçek/orijinal at adını gösterir.
- Aynı isimli atların ayırt edici bilgilerini gösterir.
- Horse seçildikten sonra temel TJK alanlarını otomatik veya salt okunur gösterir.
- Normalize arama değerini kullanıcıya sormaz veya göstermez (teknik alandır).
- TJK alanlarını tekrar istemez; düzenleme ekranı içermez.
- Eksik alanlarda hayali değer veya kullanıcı override üretmez.
- Ham TJK payload veya kontrolsüz detail JSONB tüketmez.

### BO

- Horse ve TJK sync durumunu / hatalarını görüntüleyebilir.
- Gerekli retry operasyonlarını başlatabilir.
- Duplicate veya kimlik çelişkilerini görüntüleyebilir.
- TJK kaynak alanlarını normal admin işlemiyle düzenlemez.
- Normalize arama değerini manuel düzenlemez.
- Horse’u fiziksel silerek advert ilişkilerini bozmaz.
- Ham payload saklanıyorsa bunu filtre/autocomplete/public gösterim kaynağı olarak kullanmaz.
- Kesin BO ekran tasarımı bu dokümanda yapılmaz.

### Backend

- TJK verisini alır; gerçek at adından normalize arama değerini üretir/günceller.
- TJK kaynaklı temel alanları tipli yönetir; seçilmiş değişken detayları kontrollü domain-detail JSONB’de yönetir.
- Canonical kaynak tipli temel alanlar + kontrollü detail’dir; ham payload varsa ayrı ve non-canonical’dır.
- Idempotent sync uygular; duplicate/kimlik çelişkisini engeller.
- Autocomplete ve horse filtrelerini sağlar.
- Advert–horse ilişkisini korur.
- Public API’ye kontrolsüz detail veya ham JSONB döndürmez.
- FE/BO’dan TJK alan değişikliğini kabul etmez.

## 14. Karşılaştırma tablosu

| Kriter | Full JSONB | Tamamen ilişkisel/kolonlu | Hibrit |
| --- | --- | --- | --- |
| Autocomplete | Zayıf-orta | Yüksek | Yüksek |
| TJK numarası benzersizliği | Zayıf-orta | Yüksek | Yüksek |
| Yaş/ırk/cinsiyet/don filtreleri | Zayıf-orta | Yüksek | Yüksek |
| Veri tipi güvenliği | Düşük | Yüksek | Yüksek (temel) |
| Değişken TJK şemasına uyum | Yüksek | Düşük | Yüksek |
| Sync karmaşıklığı | Orta | Yüksek | Orta |
| Duplicate kontrolü | Zayıf-orta | Yüksek | Yüksek |
| Raporlama | Düşük-orta | Yüksek | Orta-yüksek |
| İlk faz geliştirme maliyeti | Düşük | Yüksek | Orta |
| Bakım | Orta (kontrolsüzse kötü) | Yüksek | İyi |
| PostgreSQL/Railway uygunluğu | Orta | Orta | Yüksek |
| Bu projeye uygunluk | Düşük | Düşük | Çok yüksek |

## 15. Önerilen karar

**Önerilen model: Hibrit horse modeli.**

Soru cevapları:

1. **Tamamı tek JSONB mi?** Hayır.
2. **Tamamı kolon/tablolara mı?** Hayır.
3. **Hibrit mi?** Evet.
4. **Tipli temel alanlar:** TJK numarası, gerçek/orijinal at adı, doğum yılı, baba adı, anne adı, ırk, cinsiyet, don; ayrıca Haradan türev normalize arama değeri.
5. **Kontrollü domain-detail JSONB:** Pedigri, yarış istatistikleri, kardeşler, yavrular, antrenör geçmişi.
6. **Temel alanlar kontrollü detail JSONB’de tekrar mı?** Hayır. Ham payload ileride saklanırsa doğal tekrar içerebilir; canonical kaynak değildir.
7. **TJK numarası benzersiz harici kimlik mi?** Evet.
8. **Gerçek ad / normalize ad benzersiz mi?** Hayır; normalize ad ayrıca kimlik değildir.
9. **Advert horse kopyalar mı?** Hayır.
10. **Advert yalnız horse referansı mı?** Evet.
11. **Yaş ayrı kalıcı alan mı?** Hayır; doğum yılından türetilir.
12. **Anne/baba ilk fazda self-reference mi?** Hayır; ad metni.
13. **Pedigri vb. ayrı tablolar mı?** Hayır; ilk fazda kontrollü domain-detail JSONB.
14. **Kullanıcı/admin TJK’yı manuel değiştirir mi?** Hayır.
15. **Autocomplete canlı TJK mi?** Hayır; Haradan DB; gösterim gerçek ad, arama normalize türev.
16. **Sync kimliği?** TJK numarası üzerinden idempotent upsert; ad değişince normalize yeniden üretilir.
17. **Geçici görünmeyen horse fiziksel silinir mi?** Hayır.
18. **Advert horse snapshot saklar mı?** İlk fazda hayır; güncel horse gösterilir.
19. **Detail/ham JSONB public API’ye doğrudan mı?** Hayır.
20. **Detail JSONB’ye genel indeks mi?** İlk fazda hayır.

**Gerekçe:** TJK değişkenliği kontrollü detail JSONB’yi gerektirir; kullanıcı dostu seçim ve filtreler tipli temel alanları gerektirir. Normalize ad Haradan türev arama değeridir. Hibrit model sade MVP, duplicate engeli, PostgreSQL/Railway bakımı ve ileride genişleme dengesini sağlar; gereksiz tablo ve migration üretmez.

## 16. Reddedilen yaklaşımlar

- **Kontrolsüz full JSONB:** Autocomplete/filtre/tip güvenliği zayıf.
- **Sık filtrelenen alanları yalnız JSONB’de bırakmak:** Performans ve bütünlük riski.
- **Bütün detayları ilk günden çok tabloya bölmek:** Erken aşırı model.
- **Horse adını / normalize adı benzersiz yapmak:** Gerçek dünyaya aykırı.
- **Aynı TJK numarası için yeni kayıt:** Duplicate ve advert FK riski.
- **Advert’e horse alanlarını kopyalamak:** Tek kaynak ve önceki kararlara aykırı.
- **Yaşı hem doğum yılı hem ayrı alan tutmak:** Çelişki riski.
- **Yalnız isimle anne/baba referansı:** Hatalı eşleşme.
- **Kullanıcı/admin TJK override:** Tek kaynak bozulur.
- **Canlı TJK ile form araması:** Gecikme/kesinti; yerel DB kararına aykırı.
- **Bir kez görünmeyen horse’u silmek:** Mevcut ilanları bozar.
- **TJK JSONB’yi public API’ye vermek:** Kontrolsüz sızıntı/şekil bağımlılığı.
- **Ölçülmemiş genel JSONB indeksi:** Erken maliyet.
- **İlk fazda ağır advert snapshot:** Gereksiz tekrar ve karmaşıklık.

## 17. Açık kalan kararlar

- TJK erişim yöntemi
- İlk toplu veri alma yöntemi
- Artımlı güncellemenin desteklenip desteklenmediği
- Sync sıklığı
- TJK’nın kaynak güncelleme zamanı sağlayıp sağlamadığı
- Anne ve babanın güvenilir TJK kimliklerinin gelip gelmediği
- Ham TJK kaynak payload’ının saklanıp saklanmayacağı (saklanırsa non-canonical; filtre/autocomplete/public gösterim için kullanılmaz)
- Kontrollü domain-detail içinde hangi alt alanların public ekranda gösterileceği
- Horse kaydının hangi koşulda pasif kabul edileceği
- BO’da duplicate/çelişki çözüm akışı
- Hangi yarış veya pedigree detaylarının public ekranda gösterileceği
- Horse değişiklik geçmişine ileride ihtiyaç olup olmadığı

## 18. Sonraki adım

Bu karar kabul edilirse sonraki teknik karar Advert çekirdek modeli ve yaşam döngüsüdür:

- Advert sabit alanları
- Advert–category ilişkisi
- Advert–horse ilişkisi
- Advert–district ilişkisi
- Draft ve moderasyon
- Status geçişleri
- Status history
- Sahiplik
- Yayınlanma ve arşivleme
- Fiyat ürün kararı bağımlılığı

Bu dokümanda advert tablo modeli, status listesi veya API tasarlanmaz.
