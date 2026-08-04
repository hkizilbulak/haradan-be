# İlan Alanları ve Veri Sorumlulukları

## 1. Amaç

Bu doküman tablo, kolon, JSONB, DBML, migration, API veya fonksiyon listesi tasarlamaz. Amacı, ilan sistemindeki verilerin hangi domain kaynağına ait olduğunu sınıflandırmak; kategori, ilan, horse ve dinamik property sorumluluklarını birbirinden ayırmak ve sonraki veri modeli kararına temel hazırlamaktır.

Veri sorumluluğu ayrımı, kullanıcıdan istenen manuel girdiyi azaltmalıdır. Güvenilir kaynaktan gelen bilgilerin otomatik doldurulmasını ve kategoriye göre sade form üretimini desteklemelidir. Kullanıcı kitlesinde ileri yaş grubu yoğun olduğu için ilan oluşturma mümkün olduğunca az alan, açık yönlendirme ve hata önleme ile yürütülmelidir.

## 2. Temel veri kaynakları

### 2.1 İlanın sabit verileri

Birçok veya bütün ilan türünde ortak olan ve ilan yaşam döngüsünü doğrudan yöneten bilgilerdir. Kategori seçimi, sahip, konum referansı, başlık/açıklama gibi ortak içerik ve durum/moderasyon/zaman damgaları bu gruba girer. Bu alanlar kategori property tanımına bağlı olmadan ilanın kimliğini ve yaşamını taşır.

### 2.2 Kategoriye bağlı dinamik property’ler

Kategoriye göre değişen, admin tarafından tanımlanabilen ilan özellikleridir. Form, liste, detay ve arama davranışları kategori property yönetimiyle ayarlanabilir. Aynı advert modeli farklı kategorilerde farklı property kümeleriyle çalışır; property tanımları kategorinin parçasıdır, değerler ise o ilana aittir.

### 2.3 Horse verileri

Horse verileri, seçilen ata ait ve atçılık domain’inde tutulan bilgilerdir. At verilerinin tek harici kaynağı TJK’dır; YKK kullanılmayacaktır. Ancak TJK servisinin gerçek veri modeli, erişim yöntemi ve hangi alanları sağladığı henüz doğrulanmamıştır. Bu nedenle belirli alanların kesin olarak TJK’dan geldiği varsayılmaz.

At adı, TJK numarası, doğum yılı, ırk, cinsiyet, don, anne ve baba gibi alanlar horse domain’ine ait güçlü adaylardır. Bu alanların gerçekten TJK’dan gelip gelmediği TJK entegrasyon analizi sonrası kesinleşir. Bir alan TJK tarafından güvenilir biçimde sağlandığı doğrulanana kadar ilan property’si veya kullanıcı fallback girişi kendiliğinden önerilmez; eksik alanlar için kullanıcı girişi, başka servis veya yeni kaynak çözümü bu aşamada üretilmez.

Haradan veritabanına önceden senkronlanmış ve TJK’dan güvenilir biçimde geldiği doğrulanmış horse alanları, at seçimi sonrası otomatik doldurulmalı veya okunur gösterilmelidir. Kullanıcı aynı bilgiyi tekrar girmemeli veya değiştirememelidir.

### 2.4 Türetilmiş veriler

Kalıcı olarak ayrıca saklanması zorunlu olmayan, diğer alanlardan hesaplanabilen bilgilerdir. Örnek: güvenilir doğum yılından yaş, medya kayıtlarından fotoğraflı ilan bilgisi, status/tarihlerden yayında olma, ilçeden il. Performans ihtiyacı ölçülmeden denormalizasyon önerilmez.

## 3. Sınıflandırma kriterleri

Bir alanın hangi kaynağa ait olduğuna karar verirken şu kriterler kullanılır:

- **Bütün kategorilerde ortak olup olmaması:** Ortaksa sabit ilan verisine adaydır; kategoriye özgüyse dinamik property adayıdır.
- **İlan yaşam döngüsünü yönetip yönetmemesi:** Durum, moderasyon, yayınlama gibi süreç alanları sabit ilan verisidir.
- **Admin tarafından kategoriye göre tanımlanıp tanımlanmaması:** Admin’in kategoriye eklediği özellikler dinamik property’dir.
- **TJK’dan gelip gelmemesi:** Yalnız TJK tarafından sağlandığı doğrulanan at alanları horse domain’inde tutulur ve ilan property’si olarak tekrar edilmez. TJK şeması doğrulanmadan herhangi bir alanın kaynağı kesin kabul edilmez. Doğrulanmış TJK alanları kullanıcı tarafından tekrar girilmez veya değiştirilemez. TJK tarafından sağlanmayan alanların davranışı ürün kararı bekler.
- **Kullanıcı tarafından girilip girilmemesi:** Kullanıcı girdisi sabit alan veya property olabilir; doğrulanmış TJK alanları kullanıcı tarafından girilmez.
- **Kullanıcıdan gereksiz tekrar giriş istenmemesi:** Aynı bilgi birden fazla kaynaktan veya form alanından sorulmamalıdır.
- **Güvenilir kaynaktan otomatik doldurulabilirlik:** Otomatik doldurulabilen alanlar manuel giriş adayı değildir.
- **Form karmaşıklığına etkisi:** Her ek alan formu büyütür; gerekmeyen alan sorulmaz.
- **İleri yaş kullanıcılar için anlaşılabilirlik:** Etiket, açıklama ve seçim akışı sade ve açık olmalıdır.
- **Hata üretme riski:** Çelişen veya belirsiz alanlar yanlış seçim ve veri tutarsızlığı riskini artırır.
- **Alanın kullanıcıya gerçekten gösterilmesinin gerekli olup olmadığı:** Teknik olarak var olan her alan forma çıkmak zorunda değildir.
- **Arama ve filtrelemede kullanılma sıklığı:** Sık kullanılan ortak filtreler sabit alan veya iyi indekslenen property olabilir; tek başına kaynak seçimini belirlemez.
- **Veri bütünlüğü ihtiyacı:** Referans bütünlüğü ve tek kaynak ilkesi sınıflandırmayı etkiler (ör. ilçe referansı, horse referansı).
- **Başka alanlardan türetilebilir olması:** Türetilmiş veri adayıdır; zorunlu kalıcı alan değildir.
- **Kategori değişikliğinden etkilenip etkilenmemesi:** Kategori değişince anlamsızlaşan alanlar property tarafına yakındır; yaşam döngüsü alanları etkilenmez.

## 4. Örnek alanların sınıflandırılması

| Alan | Önerilen veri kaynağı | Kullanıcı mı girer? | TJK’dan mı gelir? | Kategoriye bağlı mı? | Türetilmiş olabilir mi? | Kısa gerekçe | Kesin mi, karar bekliyor mu? |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Başlık | Sabit ilan verisi | Evet | Hayır | Hayır | Hayır | Ortak ilan içeriği; yaşam döngüsünden bağımsız metin | Kesin (kaynak olarak) |
| Açıklama | Sabit ilan verisi | Evet | Hayır | Hayır | Hayır | Ortak ilan içeriği | Kesin (kaynak olarak) |
| Fiyat | Sabit ilan verisi adayı / ürün kararı | Muhtemelen | Hayır | Kısmen belirsiz | Hayır | Birçok ilanda ortak görünür; her kategoride zorunluluk/anlam net değil | Ürün kararı gerekli |
| Para birimi | Sabit ilan verisi adayı / ürün kararı | Muhtemelen | Hayır | Kısmen belirsiz | Hayır | Fiyatla birlikte anlam kazanır; tek/çoklu birim net değil | Ürün kararı gerekli |
| İlçe | Sabit ilan verisi | Evet (seçim) | Hayır | Hayır | Hayır | Ortak konum referansı; il ilçeden türetilir | Kesin |
| İlan sahibi | Sabit ilan verisi | Hayır (sistem atar) | Hayır | Hayır | Hayır | Sahiplik ve yetki için ortak zorunlu referans | Kesin |
| Kategori | Sabit ilan verisi | Evet (seçim) | Hayır | — | Hayır | İlanın türünü ve property kümesini belirler | Kesin |
| İlan durumu | Sabit ilan verisi | Kısmen (süreç) | Hayır | Hayır | Kısmen | Yaşam döngüsü yönetimi | Kesin (kaynak olarak) |
| Moderasyon durumu | Sabit ilan verisi | Hayır (admin/süreç) | Hayır | Hayır | Hayır | Moderasyon yaşam döngüsü | Kesin |
| Oluşturulma tarihi | Sabit ilan verisi | Hayır (sistem) | Hayır | Hayır | Hayır | Denetim ve sıralama | Kesin |
| Yayınlanma tarihi | Sabit ilan verisi | Hayır (sistem/süreç) | Hayır | Hayır | Hayır | Yayın yaşam döngüsü | Kesin |
| At | Horse bağlantısı (ilan→horse) | Evet (seçim) | TJK şeması doğrulanmalı | Kategoriye göre zorunluluk değişebilir | Hayır | Seçilen atın referansı; horse verisinin kapısı | Zorunluluk ürün/metadata kararı; TJK erişimi analiz bekliyor |
| At adı | Horse domain adayı | Hayır (TJK sağlarsa) | TJK şeması doğrulanmalı | Hayır | Hayır | Horse domain güçlü adayı; TJK sağlarsa property olarak tekrar girilmez | Domain: güçlü aday; kaynak kullanılabilirliği TJK analizi bekliyor |
| TJK numarası | Horse domain adayı | Hayır (TJK sağlarsa) | TJK şeması doğrulanmalı | Hayır | Hayır | Kimlik adayı; varlığı ve benzersizliği TJK ile doğrulanmalı | Domain: güçlü aday; kaynak kullanılabilirliği TJK analizi bekliyor |
| Doğum yılı | Horse domain adayı | Hayır (TJK sağlarsa) | TJK şeması doğrulanmalı | Hayır | Hayır | Horse domain güçlü adayı; TJK sağlarsa property olarak tekrar girilmez | Domain: güçlü aday; kaynak kullanılabilirliği TJK analizi bekliyor |
| Irk | Horse domain adayı | Hayır (TJK sağlarsa) | TJK şeması doğrulanmalı | Hayır | Hayır | Horse domain güçlü adayı; TJK sağlarsa property olarak tekrar girilmez | Domain: güçlü aday; kaynak kullanılabilirliği TJK analizi bekliyor |
| Cinsiyet | Horse domain adayı | Hayır (TJK sağlarsa) | TJK şeması doğrulanmalı | Hayır | Hayır | Horse domain güçlü adayı; TJK sağlarsa property olarak tekrar girilmez | Domain: güçlü aday; kaynak kullanılabilirliği TJK analizi bekliyor |
| Don | Horse domain adayı | Hayır (TJK sağlarsa) | TJK şeması doğrulanmalı | Hayır | Hayır | Horse domain güçlü adayı; TJK sağlarsa property olarak tekrar girilmez | Domain: güçlü aday; kaynak kullanılabilirliği TJK analizi bekliyor |
| Baba adı | Horse domain adayı | Hayır (TJK sağlarsa) | TJK şeması doğrulanmalı | Hayır | Hayır | Horse domain güçlü adayı; TJK sağlarsa property olarak tekrar girilmez | Domain: güçlü aday; kaynak kullanılabilirliği TJK analizi bekliyor |
| Anne adı | Horse domain adayı | Hayır (TJK sağlarsa) | TJK şeması doğrulanmalı | Hayır | Hayır | Horse domain güçlü adayı; TJK sağlarsa property olarak tekrar girilmez | Domain: güçlü aday; kaynak kullanılabilirliği TJK analizi bekliyor |
| İdmanda mı? | Dinamik property adayı | Muhtemelen | Hayır | Muhtemelen | Hayır | İş anlamı ve kategori kapsamı net değil | Ürün kararı gerekli |
| Kiralık mı? | Kategori veya property / ürün kararı | Belirsiz | Hayır | Belirsiz | Hayır | Ayrı kategori mi, property mi belirsiz | Ürün kararı gerekli |
| Koşar durumda mı? | Dinamik property adayı | Muhtemelen | Hayır | Muhtemelen | Hayır | İş anlamı net değil | Ürün kararı gerekli |
| Pansiyon hizmet türü | Dinamik property | Evet | Hayır | Evet (pansiyon) | Hayır | Pansiyona özgü özellik | Kaynak olarak güçlü aday; tanım ürünle netleşir |
| Pansiyon kapasitesi | Dinamik property | Evet | Hayır | Evet (pansiyon) | Hayır | Pansiyona özgü özellik | Kaynak olarak güçlü aday |
| Aylık pansiyon ücreti | Dinamik property veya fiyat alanı / ürün kararı | Evet | Hayır | Evet (pansiyon) | Hayır | Pansiyon fiyatının genel fiyat alanından farkı net değil | Ürün kararı gerekli |
| Görsel var mı? | Türetilmiş veri | Hayır | Hayır | Hayır | Evet | İlan görsellerinden / medya ilişkisinden hesaplanabilir | Kesin (türetilmiş) |
| At yaşı | Türetilmiş veri adayı | Hayır | TJK şeması doğrulanmalı (doğum yılına bağlı) | Hayır | Evet (doğum yılı varsa) | Yalnız güvenilir doğum yılı varsa türetilir; yaş+doğum yılı birlikte sorulmaz | Türetme: koşullu; doğum yılı kaynağı TJK analizi bekliyor |

## 5. At ilanları ile at dışı ilanların ayrımı

- **Satılık at:** Horse bağlantısı tipik olarak merkezi roldedir; ilanın konusu seçilen attır. Horse öznitelikleri, TJK tarafından güvenilir biçimde sağlandığı doğrulanan ölçüde horse domain’inden okunur; bu durumda kullanıcı aynı alanları property olarak yeniden girmez. TJK şeması doğrulanmadan alan kullanılabilirliği kesin kabul edilmez.
- **Kiralık at:** Horse bağlantısı yine tipik olarak merkezi olabilir; kiralama koşulları ise kategori property’leriyle ayrışır. Satılık ile ayrımın kategori mi yoksa property mi olduğu ürün sorusudur.
- **Pansiyon:** Horse bağlantısı zorunlu görünmez; ilanın konusu tesis/hizmet olabilir. Horse gerekip gerekmediği ürün kararıdır.
- **Horse seçiminin bütün kategoriler için zorunlu olmaması:** Aynı advert modeli farklı kategorileri desteklemelidir; at bağlantısı evrensel zorunluluk olmamalıdır.
- **Kategori metadata ihtiyacı:** Kategorinin at bağlantısı gerektirip gerektirmediği kategori metadata’sı ile belirtilmelidir. Böylece validasyon kategoriye göre uygulanır; her kategoriye aynı kural dayatılmaz. Saklama modeli bu dokümanda tasarlanmaz.

**Önerilen kullanıcı akışı (at gerektiren kategoriler):**

1. Kullanıcı at adından 2–3 karakter yazar.
2. Arama canlı TJK çağrısıyla değil, Haradan veritabanında (önceden senkronlanmış horse kayıtlarında) yapılır.
3. Kullanıcı ayırt edici bilgilerle doğru atı seçer.
4. Seçim sonrası, TJK’dan güvenilir biçimde geldiği doğrulanmış alanlar otomatik doldurulur veya okunur gösterilir.
5. Kullanıcı yalnız ilana özgü ve TJK’dan gelmeyen gerekli bilgileri girer.
6. Otomatik doldurulacak kesin alan listesi TJK analizi sonrasında belirlenir; bu dokümanda kesin alan kümesi tanımlanmaz.

## 6. Sabit ilan alanı olma kriteri

Bir alanın sabit ilan verisi olması için tipik olarak şu koşullar aranır: çoğu/tüm kategorilerde ortak olması, ilan kimliği veya yaşam döngüsünü yönetmesi, kategori property tanımına bağlı olmadan var olması, referans bütünlüğü gerektirmesi.

- **Kategori:** İlanın türünü ve geçerli property kümesini belirler; sabittir.
- **İlan sahibi:** Sahiplik, yetki ve moderasyon için sabittir.
- **İlçe:** Ortak konum referansıdır; il ilçeden türetilir.
- **Başlık / açıklama:** Ortak içerik alanlarıdır; kategori property’si değildir.
- **İlan yaşam döngüsü:** Durum ve tarihler (oluşturma, yayınlama vb.) sabit süreç verisidir.
- **Moderasyon bilgisi:** Ortak yönetim sürecidir; kategoriye göre yeniden tanımlanmaz.

**Fiyat ve para birimi:** Birçok ilanda ortak görünebilir; ancak bütün kategorilerde zorunlu olup olmadığı, pansiyon ücretinin ayrı mı yoksa genel fiyat mı olduğu, para biriminin yalnız TRY mi olduğu kesinleştirilmemiştir. Bu nedenle sabit alan adayıdır; kesin sınıflandırma için ürün kararı gerekir.

## 7. Dinamik property olma kriteri

Bir alanın dinamik property olması için tipik koşullar:

- **Kategoriye özgü olması:** Yalnız bazı kategorilerde anlamlıdır. Property yalnız gerçekten kategoriye özgüyse kullanıcıya sorulur.
- **Horse veya sabit ilan alanından gelen bilgi tekrar sorulmaz:** Aynı veri property olarak yeniden üretilmez.
- **Admin tarafından yönetilmesi:** Admin kategoriye property ekleyebilir/düzenleyebilir.
- **Formda gösterilmesi:** Property’nin form davranışı yönetilebilir. Etiket, açıklama, sıra, zorunluluk ve görünürlük sade form üretimini desteklemelidir.
- **Zorunluluk bilgisi:** Property bazında zorunlu/opsiyonel tanımlanabilir.
- **Liste veya detay görünürlüğü:** Liste/detay davranışları yönetilebilir.
- **Arama ve filtre davranışı:** Property arama/filtreye açılabilir.
- **Veri tipi ihtiyacı:** Metin, sayı, boolean, seçim vb. property tanımının parçasıdır.
- **Seçenek listesi ihtiyacı:** Enum/seçim property’lerinde seçenekler admin tarafından yönetilir.
- **Güvenli varsayılan değerler:** Uygun olduğunda güvenli varsayılanlar değerlendirilebilir; kullanıcıyı gereksiz seçimden kurtarır.
- **Gereksiz property oluşturulmaması:** Teknik esneklik için “ileride lazım olur” property’si eklenmez.
- **Property pasifleştirilince eski ilanların korunması:** Yeni formlarda gizlenebilir/pasifleşebilir; mevcut ilanlardaki geçmiş değerlerin korunması ilkesi gözetilmelidir.
- **Metadata’nın kavramsal rolü:** Property metadata’sı form, liste, detay ve arama davranışını destekleyebilir. Kesin kolon, tablo veya JSON yapısı bu dokümanda üretilmez. Değer saklama modeli (JSONB / ilişkisel / hibrit) seçilmez.

## 8. Horse verisi ile ilan property’sinin çakışması

**İlke:** At adı, TJK numarası, doğum yılı, ırk, cinsiyet, don, anne ve baba horse domain’ine ait güçlü adaylardır. Bunlar TJK tarafından güvenilir biçimde sağlandığı doğrulandığında ilan property’si olarak kullanıcıya tekrar girdirilmemeli ve kullanıcı tarafından değiştirilmemelidir. TJK şeması doğrulanmadan bu alanların kesin olarak TJK’dan geldiği kabul edilmez; sağlanmayan veya eksik alanlar için bu aşamada kullanıcı fallback’i, başka servis veya yeni kaynak çözümü üretilmez.

- **Tek kaynak ilkesi:** At verilerinin tek harici kaynağı TJK’dır (YKK yok). TJK’nın sağladığı doğrulanmış öznitelikler horse domain’inde tutulur; advert property katmanında kopyalanmaz. Bu ilke aynı zamanda kullanıcı deneyimi ilkesidir: aynı alan iki kez sorulmaz.
- **Tekrar sormanın riski:** Aynı alanın tekrar sorulması veri çelişkisi ve kullanıcı hatası üretir; özellikle ileri yaş kullanıcılar için formu zorlaştırır.
- **Kullanıcının TJK verisini değiştirememesi:** Bir alan TJK’dan güvenilir geliyorsa kullanıcı override edemez.
- **Otomatik gelen alanların gösterimi:** Otomatik doldurulan doğrulanmış alanlar okunabilir fakat değiştirilemez gösterilebilir.
- **İlanda gösterilen horse verisinin okunması:** İlan, seçilen horse referansı üzerinden TJK analiziyle netleşmiş horse verisini okur; property formundan okumaz.
- **Eksik TJK alanları:** Manuel fallback, ürün kararı olmadan eklenmemelidir.
- **TJK verisi değişince eski ilanlara etki:** Snapshot mu, her zaman güncel horse mu gösterileceği bu dokümanda kesinleştirilmez; ayrıca ürün/teknik kararı gerekir.
- **Doğrulama öncesi durum:** Alan listesi ve kullanılabilirlik TJK entegrasyon analizine bağlıdır; varsayımsal kesin alan kümesi tanımlanmaz.

## 9. Türetilmiş alanlar

- **At yaşı:** Yalnız güvenilir doğum yılı varsa türetilebilir. Kullanıcıdan hem yaş hem doğum yılı istenmemelidir. Yaş filtre aralıkları (ör. 3–5 yaş) saklanan ayrı bir horse değeri değil; arama arayüzü / sorgu aralığıdır.
- **Görsel var mı? / fotoğraflı ilanlar:** Medya kayıtlarından türetilir; ayrı zorunlu kalıcı bayrak gerekmez.
- **Yayında olup olmadığı:** İlan durumu ve ilgili tarihlerden türetilebilir veya status ile ifade edilir; çelişen ayrı bayrak önerilmez.
- **İl bilgisi:** İlçenin bağlı olduğu ilden türetilir; ilan üzerinde ayrıca il referansı tutulmaz.

Türetilmiş değerler mutlaka veritabanında ayrıca saklanmak zorunda değildir. Performans ihtiyacı ölçülmeden ve belgelenmeden denormalizasyon önerilmez.

## 10. Gözlemlenen Arama Filtresi Adayları

Aşağıdaki filtreler yalnızca ürün örneği olarak sınıflandırılır; kesin filtre seti veya şema tasarımı değildir. Örnek belgelerdeki tablo/kolon önerileri buraya taşınmaz.

| Filtre | Muhtemel veri kaynağı | Kalıcı alan mı, türetilmiş mi? | TJK doğrulaması gerekiyor mu? | Ürün kararı gerekiyor mu? | Kısa açıklama |
| --- | --- | --- | --- | --- | --- |
| İl | İlçe→il ilişkisi | Türetilmiş (ilan yalnız ilçeye bağlı) | Hayır | Hayır (konum modeli kesin) | İl, ilçeden türetilerek filtrelenir |
| İlçe | Sabit ilan verisi (ilçe referansı) | Kalıcı referans | Hayır | Hayır (konum modeli kesin) | İlanın kalıcı konum referansı |
| Minimum fiyat | Sabit ilan alanı adayı | Kalıcı aday | Hayır | Evet | Fiyatın tüm kategorilerde ortak/zorunlu olup olmadığı açık |
| Maksimum fiyat | Sabit ilan alanı adayı | Kalıcı aday | Hayır | Evet | Min fiyat ile aynı ürün kararına bağlı |
| At ırkı | Horse domain adayı | Kalıcı aday (horse) | Evet | Evet (TJK sağlıyor mu) | Yalnız TJK sağlarsa horse alanıdır |
| Yaş | Doğum yılından türetilmiş | Türetilmiş / sorgu aralığı | Evet (doğum yılı için) | Evet | Güvenilir doğum yılı varsa türetilir; aralık UI/sorgu seviyesindedir |
| Cinsiyet | Horse domain adayı | Kalıcı aday (horse) | Evet | Evet (TJK sağlıyor mu) | Yalnız TJK sağlarsa horse alanıdır |
| Don | Horse domain adayı | Kalıcı aday (horse) | Evet | Evet (TJK sağlıyor mu) | Yalnız TJK sağlarsa horse alanıdır |
| İdmanda mı? | Dinamik property adayı | Kalıcı aday (property) | Hayır | Evet | İş anlamı belirsiz |
| Kiralık mı? | Kategori veya property | Belirsiz | Hayır | Evet | Kategori mi property mi kararsız |
| Koşar durumda mı? | Dinamik property adayı | Kalıcı aday (property) | Hayır | Evet | İş anlamı belirsiz |
| İlan tarihi | Sabit ilan verisi (tarih) | Kalıcı | Hayır | Evet | Oluşturulma mı yayınlanma tarihi mi kullanılacağı kararsız |
| Fotoğraflı ilanlar | Medya ilişkisi | Türetilmiş | Hayır | Hayır (kaynak olarak) | Medya kayıtlarından türetilir |
| Kelime araması | Birden fazla kaynak adayı | Karma | Kısmen (horse alanları için) | Evet | Hangi alanları kapsayacağı daha sonra belirlenir |

## 11. Açık ürün soruları

1. Fiyat bütün kategorilerde zorunlu mu?
2. Para birimi yalnız TRY mı, birden fazla mı?
3. Satılık ve kiralık at ayrı kategoriler mi, yoksa ilan türü mü?
4. “Kiralık mı?” alanı kategoriyle mi belirlenmeli, ayrıca property olarak mı tutulmalı?
5. “İdmanda mı?” ve “koşar durumda mı?” alanlarının kesin iş anlamı / tanımları nedir?
6. Pansiyon ilanında fiyatın anlamı nedir? (genel fiyat mı, aylık pansiyon ücreti mi, ikisi birden mi?)
7. Pansiyon ilanı horse bağlantısı gerektirir mi?
8. Horse verisi güncellendiğinde eski ilan güncel horse verisini mi göstermeli?
9. Bir ilan birden fazla at içerebilir mi?
10. Kategori değiştirilen bir ilanda eski property değerlerine ne olmalı?
11. TJK servisi hangi horse alanlarını gerçekten sağlıyor?
12. TJK numarası tüm seçilebilir atlar için mevcut ve benzersiz mi?
13. TJK’da bulunmayan veya eksik alanların ürün davranışı ne olmalı?
14. TJK verisinin kullanım, saklama ve güncelleme kısıtları var mı?
15. İlan oluşturma formunda hangi alanlar otomatik doldurulacak?
16. Otomatik doldurulan TJK alanları yalnız okunur mu gösterilecek?
17. TJK’da eksik alan olduğunda ürün davranışı ne olacak?
18. Kategori bazında minimum zorunlu alan kümesi nedir?
19. İlan oluşturma tek ekran mı, adımlı form mu olmalı?
20. Fiyat tüm kategoriler için ortak mı?
21. İlan tarihi filtresi yayınlanma tarihine mi dayanmalı?
22. Kelime araması hangi kaynakları kapsamalı?
23. Category property tanımı ile advert içindeki gerçek JSONB değerler nasıl versiyonlanmalı?
24. Property pasifleştirilince eski ilan değeri nasıl korunmalı?
25. Kategori değişikliğinde eski JSONB property değerleri ne olmalı?
26. JSONB içinde filtrelenecek alanlar için indeks stratejisi nasıl belirlenecek?
27. Property değerinin veri tipi ve validasyonu JSONB’ye yazılmadan önce nasıl uygulanacak?

## 12. Kullanıcı Dostu İlan Oluşturma İlkeleri

- Önce kategori seçimi yapılır; form, seçilen kategoriye göre sadeleşir.
- At gerektiren kategorilerde yerel horse autocomplete kullanılır (Haradan DB; canlı TJK çağrısı değil).
- Doğrulanmış TJK verileri otomatik doldurulur veya okunur gösterilir.
- Yalnız ilana özgü ve gerçekten gerekli bilgiler sorulur.
- Küçük ve anlaşılır adımlar tercih edilir; gerekirse adımlı form ürün kararıyla netleşir.
- Büyük ve açık etiketler kullanılır; teknik terimlerden kaçınılır.
- Zorunlu alanlar net gösterilir.
- Hatalı seçimler form seviyesinde önlenir; anlaşılır hata mesajları verilir.
- Kullanıcı girdisi kaybolmamalıdır (taslak/koruma davranışı ürün/uygulama aşamasında ele alınır).
- Aynı bilgi iki kez sorulmaz.
- Güvenli varsayılanlar kullanılır.
- Backend metadata ve validasyon modeli, FE’nin sade form üretmesini desteklemelidir.

## 13. Sonraki karar adımı

Bir sonraki aşamada şu konuların değerlendirilmesi gerekir:

- Category
- Category property
- Advert
- Advert property values
- Şirketin JSONB yönlendirmesi
- JSONB / ilişkisel / hibrit seçenekleri
- Validasyon, geçmiş veri, kategori değişikliği ve filtreleme etkileri

Bu dokümanda bu karar verilmez; yalnızca sorumluluk sınırları, UX ilkeleri ve açık sorular netleştirilir.
