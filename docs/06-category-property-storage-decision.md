# Category Property ve Advert Property Saklama Kararı

## 1. Problem tanımı

Dört kavram açıkça ayrılır:

- **Category:** İlanın sınıflandırmasını ve geçerli form yapısını belirler. Hiyerarşiyi destekler; isteğe bağlı üst kategorisi olabilir. Ağaç BO üzerinden yönetilir.
- **Category property tanımı:** Seçilen kategoriye göre form, liste, detay, arama ve filtre davranışını yöneten metadata tanımıdır. BO yönetir; FE değiştirmez. Her category property kullanıcı girdisi olmak zorunda değildir.
  - Kaynağı kullanıcı girdisi olan property’ler FE formunda kullanıcıya sorulabilir ve gerçek değerleri advert properties JSONB içinde tutulabilir.
  - Kaynağı sabit advert alanı, horse/TJK alanı veya türetilmiş değer olan tanımlar yalnız görüntüleme, filtreleme veya form davranışını açıklamak için kullanılabilir.
  - Horse, sabit advert ve türetilmiş değerler kullanıcıya tekrar sorulmaz ve advert properties JSONB içine kopyalanmaz.
  - Property tanımı ile advert içinde saklanan gerçek dinamik kullanıcı değeri aynı şey değildir.
- **Advert:** Belirli bir kullanıcının ilan kaydıdır. Sabit advert alanları, sahiplik, kategori, ilçe, horse bağlantısı ve yaşam döngüsü burada tutulur.
- **Advert üzerindeki gerçek dinamik property değerleri:** Belirli bir advert’e ait, kaynağı kullanıcı girdisi olan kategoriye özgü değerlerdir. Property tanımı ile property değeri aynı şey değildir.

Sınırlar:

- Horse alanları (TJK) dinamik kullanıcı değeri olarak advert properties JSONB içinde tekrar edilmez.
- Sabit advert alanları advert properties JSONB içine taşınmaz.
- Türetilmiş alanlar (il, yaş, fotoğraflı ilan, yayında olma) gereksiz yere advert properties JSONB içinde saklanmaz.

Bu dokümanda tablo veya kolon tasarımı yapılmaz.

## 2. Fonksiyonel gereksinimler

Category property tanımı kavramsal olarak şu davranışları desteklemelidir:

- Teknik property kodu
- Kullanıcıya gösterilen başlık
- Açıklama ve yardım metni
- Veri tipi
- Form input tipi
- Zorunlu veya opsiyonel olma
- Form sırası
- Formda görünürlük
- Liste görünürlüğü
- Detay görünürlüğü
- Arama ve filtre görünürlüğü
- Filtrelenebilirlik
- Aranabilirlik
- Sıralanabilirlik ihtiyacı
- Seçenekli alanların seçenekleri
- Varsayılan değer
- Minimum ve maksimum değer
- Metin uzunluğu
- Aktif/pasif durumu
- Kaynağın kullanıcı girdisi, sabit advert alanı, horse/TJK alanı veya türetilmiş değer olması (yalnız kullanıcı girdisi kaynaklı property’lerin forma sorulması ve JSONB’ye yazılması için)
- İleri yaş kullanıcılar için açık etiket ve sade form üretimi

Bu maddeler kesin kolon isimleri değildir; ihtiyaç listesidir.

## 3. Category property tanımı için seçenekler

Şirket yönlendirmesi (“Category attribute JSONB olsun”) iki farklı anlama gelebilir:

1. Bir kategorinin bütün property tanımlarının tek JSONB dokümanında tutulması
2. Property tanımlarının ayrı yönetilebilir kayıtlar olması; options, validation rules, default value ve benzeri esnek metadata alanlarının JSONB tutulması

Bu bölüm her iki yorumu karşılaştırır.

### 3.1 Kategori içinde tek JSONB dokümanı

Bir kategorinin bütün property tanımları kategori kaydı içindeki tek JSONB dokümanında tutulur.

**İlk geliştirme sadeliği.** Tek alan güncellemesiyle tüm şema yazılabilir; başlangıçta hızlı görünebilir.

**Tek property güncelleme.** Bir property değiştirmek için tüm doküman okunur, değiştirilir, yazılır. Kısmi güncelleme riskli ve hataya açıktır.

**BO yönetimi.** BO formları tek büyük dokümanı düzenlemek zorunda kalır; UI ve API karmaşıklaşır.

**Property sıralama / aktif-pasif.** Doküman içi dizi/nesne kurallarıyla yapılabilir; ancak atomik tek property işlemleri zayıftır.

**Property kodu benzersizliği.** Uygulama katmanında doğrulanır; DB düzeyinde tek property kaydı kadar doğal değildir.

**Eş zamanlı admin güncellemeleri.** İki admin aynı anda farklı property’leri güncellerse last-write-wins ile birbirinin değişikliğini ezebilir.

**Audit.** Tek property değişikliğini izlemek zorlaşır; tüm doküman diff’i gerekir.

**Geçmiş ilanlarla ilişki.** Property kimliği doküman içinde yaşar; pasifleştirme ve code sabitliği disiplin ister.

**Validasyon.** Tanım şemasının kendisi de validasyon gerektirir; büyük JSONB içinde hata lokalizasyonu zayıftır.

**Bakım maliyeti.** Doküman büyüdükçe okuma/yazma, çakışma ve hata riski artar.

**Avantajlar.** Az fiziksel varlık; şirket “JSONB” ifadesine yüzeysel uyum.

**Dezavantajlar.** BO tekil yönetimi zayıf; eşzamanlılık ve audit zayıf; bakım maliyeti yüksek; bu proje için uygun değil.

### 3.2 Ayrı yönetilebilir category property kayıtları

Her property tanımı ayrı yönetilebilir kayıttır. Esnek parçalar JSONB olabilir:

- Options
- Validation rules
- Default value
- UI’ye yardımcı esnek metadata

**BO’dan tek property güncelleme.** Atomik ve sadedir.

**Sıralama / aktif-pasif.** Ayrı alanlar veya kontrollü metadata ile yönetilir.

**Kategori içinde benzersiz teknik code.** Kayıt düzeyinde doğal olarak uygulanabilir.

**Audit ve değişiklik takibi.** Tek property değişikliği izlenebilir.

**Geçmiş ilanlarla ilişki.** Property code sabit kalır; pasif kayıtlar eski değerleri yorumlamak için kalır.

**Form / filtre metadata’sı üretme.** Aktif property kayıtlarından FE’ye sade metadata üretilir.

**Avantajlar.** BO yönetimi güçlü; şirket JSONB yönlendirmesi esnek metadata’da karşılanır; bütünlük ve bakım dengeli.

**Dezavantajlar.** Tanım tarafında birden fazla kayıt vardır (kabul edilebilir maliyet).

## 4. Kategori hiyerarşisinin property modeline etkisi

- **Property yalnız belirli kategoriye mi atanmalı?** İlk fazda evet; her property açıkça bir kategoriye bağlanır.
- **Üst kategoriden otomatik miras?** İlk fazda önerilmez. Gizli miras BO yönetimini ve kullanıcı formunu zorlaştırır.
- **Override?** İlk fazda yok.
- **Aynı property birden fazla kategoride?** İlk fazda kopya tanım veya paylaşımlı kütüphane zorunlu değildir; ihtiyaç doğarsa sonra değerlendirilir.
- **Üst kategoriye doğrudan ilan?** Ürün kararı açıktır. Verilirse o kategorinin kendi property kümesi kullanılır.
- **Yalnız yaprak kategorilere ilan?** Model sadeleşir; önerilen ilk faz yönü budur (ürün onayıyla).
- **Parent taşıma:** Property’ler kategori kaydına bağlı kaldığı için taşınan kategorinin property’leri birlikte gider; otomatik miras olmadığı için sürpriz azalır.

**İlk faz önerisi:** Property’ler kategoriye açık atansın; otomatik/gizli miras uygulanmasın; mümkünse ilan yalnız yaprak kategoriye bağlansın. Başlangıç isimleri ve maksimum derinlik bu dokümanda belirlenmez.

## 5. Advert property değerleri için seçenekler

### 5.1 Advert üzerinde tek JSONB nesnesi

Kategoriye özgü gerçek kullanıcı değerleri advert kaydında tek JSONB nesnesi olarak tutulur.

**Sade MVP / form kaydetme-okuma / Go request-response.** Tek nesne ile okuma-yazma sadedir; şirket “ilanın property’leri JSONB olacak” yönlendirmesine uyar.

**Yeni property ekleme.** Tanıma eklenen property, yeni ilanlarda JSONB’ye yazılabilir; eski ilanlarda anahtar yoksa zorunluluk kurallarına göre ele alınır.

**Validasyon.** JSONB esnekliği nedeniyle backend uygulama katmanında zorunludur (tip, zorunluluk, seçenek, bilinmeyen anahtar).

**Filtreleme / range / boolean / seçim.** JSONB path sorguları ile mümkündür; indeks kontrollü ve ölçüme dayalı olmalıdır.

**Property code değişikliği / pasifleştirme / kategori değişikliği / geçmiş değer.** Code sabit kalmalı; pasifte değer silinmemeli; kategori değişimi kontrollü olmalıdır.

**Avantajlar.** Sadelik, şirket yönlendirmesi, az tablo, FE form kaydı kolaylığı.

**Dezavantajlar.** Doğrulamasız kullanılırsa bütünlük bozulur; filtrede ölçülmemiş indeks maliyeti oluşabilir.

### 5.2 Ayrı advert property value kayıtları

Her advert-property değeri ayrı ilişkisel kayıttır.

**FK bütünlüğü** güçlüdür. Ancak farklı veri tipleri tek kolon veya çok tip kolonu gerektirir; çoklu seçim ek karmaşıklık üretir.

**Kayıt ve join sayısı** ilan oluşturma/güncellemede yükselir. Filtreleme/raporlama bazı senaryolarda avantajlıdır; BO moderasyonu çok satırlı hale gelir.

**Avantajlar.** İlişkisel bütünlük, bazı raporlama modelleri.

**Dezavantajlar.** İlk faz için aşırı; şirket JSONB yönlendirmesine aykırı ağırlık; tip modelleme maliyeti yüksek.

### 5.3 Hibrit yaklaşım

- Ortak ve kritik alanlar sabit advert verisi
- Horse verileri horse domain’inde
- Türetilmiş değerler sorgu veya uygulama katmanında
- Yalnız kaynağı kullanıcı girdisi olan kategoriye özgü gerçek değerler advert JSONB içinde
- Category property tanımları ayrı yönetilebilir kayıtlar (kullanıcı girdisi olmak zorunda değil)
- Options, validation ve default gibi esnek metadata JSONB
- Sık kullanılan filtrelerde kontrollü JSONB indeksleri
- İleride yalnız ölçülmüş ihtiyaç oluşursa projection veya ayrı arama yapısı

Bu, şirket JSONB yönlendirmesini korurken BO yönetilebilirliğini ve domain sınırlarını sağlar. 5.1’in sadeliği ile tanım tarafındaki 3.2’nin yönetilebilirliğini birleştirir; 5.2’nin ilk faz maliyetini taşımaz.

## 6. Property kimliği ve teknik code

- JSONB anahtarı property code olmalıdır (yalnız kaynağı kullanıcı girdisi olup advert JSONB’ye yazılan değerler için).
- Kullanıcıya gösterilen title ile teknik code ayrılmalıdır.
- Property code kategori içinde benzersiz olmalıdır.
- Code için tutarlı büyük/küçük harf ve karakter kuralları uygulanmalıdır (kesin regex bu dokümanda yazılmaz).
- Teknik property code oluşturulduktan ve kullanılmaya başladıktan sonra normal BO işlemiyle değiştirilememelidir.
- Admin title, açıklama, yardım metni, sıralama, görünürlük, zorunluluk, seçenekler, varsayılan değer ve validasyon metadata’sını yönetebilir.
- Ancak kullanılmış bir property’nin veri tipi, seçenekleri, zorunluluğu veya validasyon kuralları değiştirilirken mevcut ilanlara etkisi kontrol edilmelidir.
- Geçmiş ilanların anlamını bozacak değişiklikler engellenmeli veya güvenli geçiş gerektirmelidir.
- Teknik code değişikliği yalnız kontrollü migration/veri dönüşümü ile yapılabilir.
- Kullanılmış property’nin veri tipi serbestçe değiştirilemez.
- Kullanılmış seçenek fiziksel olarak silinmemeli; yeni seçimlere kapatılarak geçmiş değerler korunmalıdır.
- Property fiziksel silinmez; pasifleştirilir.

## 7. Advert property validasyonu

Advert properties JSONB kabul edilmeden önce backend kavramsal olarak şunları kontrol eder:

- Seçilen kategori mevcut ve aktif mi?
- İlan verilebilir bir kategori mi?
- Gönderilen property seçilen kategori için geçerli mi?
- Property aktif mi?
- Property kullanıcı formundan değer kabul ediyor mu?
- Bilinmeyen JSON anahtarları var mı?
- Zorunlu property’ler mevcut mu? (moderasyona gönderme seviyesinde)
- Veri tipi doğru mu?
- Sayısal minimum ve maksimum kuralları sağlanıyor mu?
- Metin uzunluğu doğru mu?
- Tek seçim değeri tanımlı seçenekler içinde mi?
- Çoklu seçim değerlerinin tamamı geçerli mi?
- Varsayılan değer uygulanabilir mi?
- Horse alanı kullanıcı tarafından property olarak gönderilmeye çalışılıyor mu?
- Sabit advert alanı JSONB içine kopyalanmaya çalışılıyor mu?
- Türetilmiş alan gönderiliyor mu?

Açık ilkeler:

- JSONB esnek olması nedeniyle doğrulamasız kabul edilmemelidir.
- Dinamik property kurallarının tamamı yalnız PostgreSQL CHECK ile uygulanamaz.
- İş kuralları ve tip validasyonu backend uygulama katmanında uygulanmalıdır.
- PostgreSQL temel JSON türü ve kayıt bütünlüğünü korur.
- Request’te bilinmeyen property anahtarları varsayılan olarak reddedilmelidir.
- FE’den gelen property metadata’ya güvenilmemelidir; backend kendi tanımlarını kullanarak doğrulamalıdır.

## 8. Taslak ve moderasyona gönderme validasyonu

### Taslak kaydetme

- Kullanıcı formu tamamlamadan ilerleyebilir.
- Kısmi property değerleri kaydedilebilir.
- Girilmiş değerlerin veri tipi yine doğru olmalıdır.
- Bilinmeyen veya izinsiz alanlar kabul edilmez.
- Kullanıcı girdisi kaybolmamalıdır.

### Moderasyona gönderme

- Bütün zorunlu sabit alanlar tamamlanmış olmalıdır.
- Bütün zorunlu category property değerleri bulunmalıdır.
- Category-horse gereksinimi sağlanmış olmalıdır.
- Geçersiz veya pasif property değerleri reddedilmelidir.
- Tam iş kuralı validasyonu uygulanır.

Bu ayrım ileri yaş kullanıcılar için adımlı ve kesintiye dayanıklı ilan oluşturmayı destekler.

## 9. Property pasifleştirme ve seçenek değişikliği

- Property pasif yapılınca yeni ilan formlarında gösterilmez.
- Mevcut advert JSONB değeri silinmez.
- Eski ilan detayında tarihsel değer okunabilir kalır.
- Property fiziksel olarak silinmez.
- Property başlığı / açıklama değişmesi eski değeri silmez (code sabit).
- Admin görünürlük, zorunluluk, seçenek, varsayılan ve validasyon metadata’sını yönetebilir; kullanılmış property’de bu değişiklikler mevcut ilan etkisine göre kontrol edilir.
- Seçenek listesinden seçenek kaldırılırsa eski ilanlarda değer korunur; seçenek fiziksel silinmez, yeni seçimlere kapatılır.
- Kaldırılmış seçenek yeni ilanlarda seçilemez.
- BO, geçmiş veriyi bozabilecek değişikliklere karşı uyarılmalıdır.
- Kullanılmış property’nin veri tipi serbestçe değiştirilemez.

## 10. Kategori değişikliği

Seçenekler:

1. Her durumda serbest bırakmak — yüksek risk
2. Yalnız taslak aşamasında izin vermek — sade ve güvenli
3. Moderasyona gönderilmiş veya yayınlanmış ilanda engellemek — 2 ile uyumlu
4. Kategori değişince bütün eski property değerlerini silmek — net ama veri kaybı
5. Eski değerleri arşivlemek — erken karmaşıklık
6. Aynı code + uyumlu tip taşımak — gizli eşleştirme riski
7. Kullanıcıya form alanlarının sıfırlanacağı uyarısı — UX için gerekli

**İlk faz önerisi:** Kategori değişikliği yalnız taslakta serbest olsun. Moderasyona gönderilmiş / yayınlanmış / askıdaki ilanlarda engellensin. Taslakta kategori değişince eski dinamik property değerleri temizlensin ve kullanıcıya açık uyarı gösterilsin. Otomatik code eşleştirme veya ağır arşivleme ilk fazda uygulanmasın.

## 11. Filtreleme ve indeksleme

- Bütün JSONB’ye tek genel indeks her sorguyu çözmez.
- Her property için otomatik indeks oluşturulmaz.
- Önce kategori filtresi uygulanır.
- Yalnız aktif ve gerçekten filtrelenen property’ler dikkate alınır.
- Boolean, sayısal range, tek/çoklu seçim filtreleri property tipine uygun JSON tipi ile çalışır.
- JSONB içinde sayılar metin olarak saklanmamalıdır.
- Dinamik expression index maliyeti operasyonel olarak yönetilir.
- İlk fazda bütün olası property’lere indeks açılmaz.
- İndeks kararı gerçek sorgu ve performans ölçümüyle verilir.
- İl/ilçe advert properties JSONB’den gelmez.
- Horse ırkı, cinsiyet, don ve yaş advert properties JSONB’den gelmez.
- Fotoğraflı ilan filtresi medya ilişkisinden türetilir.

Spesifik SQL veya indeks komutu yazılmaz.

## 12. Geçmiş veri ve versiyonlama

Seçenekler: yalnız güncel tanımla okumak; ilan anında tam metadata snapshot; property version; label/options etkileri; audit; aşırı versiyonlama maliyeti.

**İlk faz minimum yaklaşım:**

- Eski advert JSONB değeri kaybolmaz.
- Teknik property code sabit kalır; normal BO işlemiyle değiştirilmez.
- Property fiziksel silinmez; pasifleşir.
- Title / açıklama değişikliği değer anahtarını bozmaz.
- Zorunluluk, seçenek ve validasyon metadata değişiklikleri geçmiş ilan anlamını bozmayacak şekilde kontrol edilir.
- Kullanılmış seçenekler geçmiş ilanlarda korunur (en azından ham değer + mümkünse seçenek kaydı/pasif seçenek); fiziksel silinmez.
- Kullanılmış veri tipi serbestçe değiştirilmez.
- Her ilan için tüm form şemasının ağır snapshot’ı ilk fazda zorunlu değildir.
- Gerçek ihtiyaç olmadan karmaşık schema-version sistemi kurulmaz.
- Audit ihtiyacı sonra değerlendirilir.

## 13. FE ve BO etkisi

### FE

- Aktif kategori ağacını okur.
- Seçilen kategori için form, liste, detay, arama ve filtre metadata’sını okur.
- Forma yalnız kaynağı kullanıcı girdisi olan aktif property’leri sorar.
- Horse, sabit advert veya türetilmiş kaynaklı tanımları kullanıcıya tekrar sormaz ve JSONB’ye yazmaz.
- Property sırasını uygular.
- Zorunlu alanları açık gösterir.
- Yardım metinlerini gösterir.
- Güvenli varsayılanları kullanabilir.
- Taslak sırasında kısmi değer gönderebilir.
- Moderasyona gönderirken tam form kontrolü gösterir.
- Backend validasyonunun yerine geçmez.

### BO

- Kategori ağacını yönetir.
- Property tanımlarını yönetir.
- Title, açıklama, yardım metni, sıralama, görünürlük, zorunluluk, seçenekler, varsayılan değer ve validasyon metadata’sını yönetebilir.
- Property’yi pasifleştirebilir.
- Kullanılmış property code normal işlemle değiştirilemez; code yalnız kontrollü migration ile değişir.
- Kullanılmış veri tipi serbestçe değiştirilemez; seçenekler fiziksel silinmez.
- Kullanılmış property’de zorunluluk, seçenek ve validasyon değişiklikleri mevcut ilan etkisine göre sınırlandırılır veya güvenli geçiş gerektirir.
- Moderasyonda kullanıcının gönderdiği property değerlerini okuyabilir.
- BO’nun gerçek advert property değerlerini değiştirip değiştiremeyeceği ayrı iş kuralı olarak kalır.

## 14. Karşılaştırma tabloları

### 14.1 Category property tanımları

| Kriter | Kategori içinde tek JSONB | Ayrı kayıtlar + esnek metadata JSONB |
| --- | --- | --- |
| BO yönetimi | Zayıf | Güçlü |
| Tek property güncelleme | Zayıf | Güçlü |
| Sıralama | Orta | Güçlü |
| Aktif/pasif | Orta | Güçlü |
| Benzersiz code | Zayıf-orta | Güçlü |
| Eş zamanlı güncelleme | Zayıf | Güçlü |
| Audit | Zayıf | Güçlü |
| Geçmiş ilanlarla ilişki | Orta | Güçlü |
| Bakım | Zayıf (büyük doküman) | İyi |
| Bu projeye uygunluk | Düşük | Çok yüksek |

### 14.2 Advert property değerleri

| Kriter | Advert JSONB | Ayrı value kayıtları | Hibrit (önerilen bütün) |
| --- | --- | --- | --- |
| Sadelik | Yüksek | Düşük | Yüksek |
| Veri bütünlüğü | Orta (validasyonla yüksek) | Yüksek | Yüksek |
| Backend validasyonu | Zorunlu | Zorunlu | Zorunlu |
| Form kaydetme | Yüksek | Orta | Yüksek |
| Filtreleme | Orta-yüksek (kontrollü) | Yüksek | Orta-yüksek |
| İndeksleme | Kontrollü | Doğal | Kontrollü |
| Raporlama | Orta | Yüksek | Orta |
| Geçmiş veri | İyi (silinmezse) | İyi | İyi |
| BO moderasyonu | İyi | Orta (çok satır) | İyi |
| Bakım | İyi | Orta-yüksek maliyet | İyi |
| Bu projeye uygunluk | Yüksek | Düşük (ilk faz) | Çok yüksek |

## 15. Önerilen karar

**Önerilen model (hibrit, şirket JSONB yönlendirmesiyle uyumlu):**

- Category property tanımları ayrı yönetilebilir kayıtlardır; form/liste/detay/arama/filtre metadata’sını yönetir ve her biri kullanıcı girdisi olmak zorunda değildir.
- Options, validation rules, default value ve esnek UI metadata JSONB olabilir.
- Yalnız kaynağı kullanıcı girdisi olan gerçek dinamik değerler advert üzerindeki JSONB içinde tutulur.
- Sabit advert alanları, horse/TJK alanları ve türetilmiş değerler bu JSONB’ye girmez; ilgili category property tanımları varsa yalnız görüntüleme/filtre/form davranışı için kullanılır.
- BO title, açıklama, yardım, sıra, görünürlük, zorunluluk, seçenek, varsayılan ve validasyon metadata’sını yönetebilir; kullanılmış code/veri tipi/seçenek silme ve geçmişi bozan değişiklikler kontrollüdür.

Soru cevapları:

1. **Kategori kayıtları bütün property tanımlarını tek JSONB içinde mi tutmalı?** Hayır.
2. **Category property tanımları ayrı yönetilebilir kayıtlar mı olmalı?** Evet.
3. **Options, validation rules, default value ve esnek UI metadata JSONB olabilir mi?** Evet.
4. **Gerçek dinamik advert property değerleri advert üzerindeki JSONB içinde mi tutulmalı?** Evet.
5. **İlk fazda ayrı advert_property_values yapısı gerekli mi?** Hayır.
6. **Sabit advert alanları properties JSONB içine taşınmalı mı?** Hayır.
7. **Horse/TJK alanları properties JSONB içine kopyalanmalı mı?** Hayır.
8. **Türetilmiş değerler properties JSONB içine yazılmalı mı?** Hayır.
9. **Advert properties JSONB anahtarları property code mu olmalı?** Evet.
10. **Property code oluşturulduktan sonra değiştirilebilmeli mi?** Hayır; kullanılmaya başladıktan sonra normal BO işlemiyle değiştirilemez (zorunlu istisna = kontrollü migration).
11. **Property pasifleşince eski advert değeri korunmalı mı?** Evet.
12. **Kullanılmış seçenek kaldırılınca eski değer korunmalı mı?** Evet; seçenek fiziksel silinmez, yeni ilanlarda seçilemez.
13. **İlk fazda category property mirası uygulanmalı mı?** Hayır.
14. **İlk fazda ilan hangi kategori seviyesine bağlanmalı?** Tercihen yalnız yaprak kategori (ürün onayıyla).
15. **İlk fazda kategori değişikliğine hangi durumlarda izin?** Yalnız taslak; sonrası engelli; taslakta değerler temizlenir + uyarı.
16. **Taslak ve moderasyona gönderme validasyonları ayrılmalı mı?** Evet.
17. **JSONB filtre indeksleri nasıl ele alınmalı?** Kontrollü; ölçüme dayalı; her property’ye otomatik indeks yok.
18. **Geçmiş ilan anlamını korumak için minimum yaklaşım?** Code sabit, fiziksel silme yok, JSONB değer korunur, seçenekler pasif/korunur; ağır snapshot yok.

**Gerekçe:** Şirket JSONB’yi değer ve esnek metadata için ister; BO ise tekil property yönetimi ister. Ayrı tanım kayıtları + advert JSONB değerleri bu dengeyi sağlar. Sade MVP, ileri yaş kullanıcı formları, PostgreSQL JSONB kabiliyeti ve Railway bakım kolaylığı korunur; gereksiz value tablosu ve gizli miras ilk fazda yok.

## 16. Reddedilen yaklaşımlar

- **Bütün category property tanımlarını tek büyük JSONB’de yönetmek:** BO, audit ve eşzamanlılık zayıf.
- **Doğrulamasız JSONB değer yazmak:** Bütünlük ve güvenlik bozulur.
- **Bütün sabit advert alanlarını JSONB’ye taşımak:** Yaşam döngüsü ve sahiplik modelini bozar.
- **Horse alanlarını advert JSONB’ye kopyalamak:** Tek kaynak ve UX ilkelerine aykırı.
- **Türetilmiş filtreleri JSONB’de saklamak:** Gereksiz denormalizasyon.
- **İlk fazda ayrı tipli advert_property_values zorunlu kabul etmek:** Aşırı model; şirket yönlendirmesine aykırı ağırlık.
- **Her property için otomatik indeks:** Operasyonel maliyet ve erken optimizasyon.
- **Kullanılmış property’yi fiziksel silmek:** Geçmiş ilanları bozar.
- **Kullanılmış teknik code’u serbestçe değiştirmek:** JSONB anahtarlarını kırar.
- **Yayınlanmış ilanın kategorisini serbestçe değiştirmek:** Form/değer tutarsızlığı.
- **Karmaşık gizli property mirasını ilk fazda otomatik uygulamak:** BO ve kullanıcı formu karmaşası.

## 17. Açık kalan ürün kararları

- Başlangıç kategori ağacı
- Maksimum kategori derinliği
- Üst kategoriye doğrudan ilan verilip verilemeyeceği
- Fiyatın bütün kategorilerde ortak olup olmadığı
- Para birimi
- Satılık ve kiralık at kategori ayrımı
- Pansiyon fiyat anlamı
- İdmanda ve koşar durumda alanlarının tanımı
- BO’nun kullanıcı property değerlerini düzenleme yetkisi
- Kategori değişiminde kullanıcıya gösterilecek UX metinleri
- Hangi property’lerin ilk fazda filtreleneceği

## 18. Sonraki adım

Bu karar kabul edilirse sonraki teknik karar horse veri modelidir:

- Temel TJK alanlarının normal kolonlarda tutulması
- Değişken TJK detaylarının JSONB tutulması
- Full JSONB, tam kolonlu ve hibrit horse modeli karşılaştırması
- Horse autocomplete
- TJK sync
- Duplicate ve benzersizlik
- Horse filtreleme

Bu dokümanda horse tablo modeli oluşturulmaz veya kesinleştirilmez.
