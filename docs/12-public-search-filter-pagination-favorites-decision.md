# Public Arama, Filtreleme, Sıralama, Pagination ve Favoriler Kararı

## 1. Problem tanımı

Aşağıdaki kavramlar birbirinden ayrılır:

- **Public advert search:** Anonim kullanılabilen, yalnız `PUBLISHED` advert listesi.
- **Public advert detail:** Anonim kullanılabilen, yalnız `PUBLISHED` advert detayı.
- **Search criteria:** Kullanıcının seçtiği filtre/sort/pagination parametreleri bütünü.
- **Structured filter:** Category, location, horse, metadata-doğrulanmış property, fotoğraf varlığı gibi kontrollü filtreler.
- **Text search:** Başlık/açıklama veya genel keyword araması; structured filter’dan ayrıdır.
- **Category scope:** Filtrenin geçerli olduğu category ve descendant kapsamı.
- **Parent/root browse:** Parent veya root seçimi; descendant leaf’lerdeki `PUBLISHED` advert’leri kapsar; yalnız ortak sabit filtreler.
- **Leaf category filter context:** Tek leaf category; dynamic property filtrelerinin tek geçerli bağlamı.
- **Dynamic property filter:** Tek leaf category metadata ile doğrulanan JSONB property filtresi.
- **Advert category edit:** Advert’in category referansının değiştirilmesi (yalnız `DRAFT`).
- **Category taxonomy/tree change:** Parent taşıma, reparent, pasifleştirme; advert referansını otomatik dönüştürmez.
- **Sort:** Backend whitelist sıralama seçeneği.
- **Pagination:** Sonuç dilimleme.
- **Cursor:** Opaque, backend üretilmiş keyset ilerleme jetonu.
- **Result summary/read model:** Liste satırı projection’ı.
- **Detail read model:** Detay projection’ı.
- **Favorite relation:** User–advert favori ilişkisi.
- **Favorite state enrichment:** Authenticated kullanıcı için `is_favorite`.
- **Favorite count:** Favori sayısı; public gösterimi ürün kararıdır.
- **Photo availability:** Public kullanıma uygun READY advert media varlığı.
- **Query authorization:** Public vs authenticated vs BO ayrımı.
- **Query performance:** N+1, index, complexity.
- **Search index / database index:** Arama motoru ≠ PostgreSQL index.

Açık ilkeler:

- Public search yalnız `PUBLISHED` advert’leri döndürür.
- Search read model, advert write/domain modelinin tamamını istemciye açmak zorunda değildir.
- Public summary ile public detail aynı projection olmak zorunda değildir.
- Favorite relation ile advert status’u aynı şey değildir.
- Search filtresi ile BO moderasyon filtresi aynı API/read model değildir.
- Database index ile search engine aynı kavram değildir.
- Parent/root browse ile leaf dynamic property filter aynı yetenek değildir.
- Property code global kimlik değildir; category context ile anlam kazanır.
- Advert category edit ile taxonomy reparent / deactivation aynı şey değildir.
- Public search category’yi kendi başına değiştirmez veya yeniden sınıflandırmaz.
- Bu dokümanda tablo, endpoint veya SQL tasarımı yapılmaz.

## 2. Fonksiyonel gereksinimler

Sistem kavramsal olarak şunları desteklemelidir:

- Public yayınlanmış advert listesi
- Public advert detail
- Parent/root category browse (descendant `PUBLISHED` advert kapsamı)
- Leaf category seçimi
- Parent/root scope’ta ortak sabit filtreler (province, district, horse, fotoğraflı ilan, kesinleşirse fiyat, whitelist sort)
- Leaf category seçildikten sonra dynamic property filtresi
- Province filtresi
- District filtresi
- Horse filtresi
- Horse autocomplete (Haradan PostgreSQL)
- Fotoğraflı ilan filtresi
- Ürün kararı varsa fiyat filtresi
- Belirli whitelist sort seçenekleri
- Deterministic pagination
- Mobil “daha fazla yükle” akışı
- Web pagination ihtiyacı (ürün)
- Kullanıcının favori durumunu görmesi
- Favori ekleme/çıkarma
- Kullanıcının favori listesi
- Public olmayan advert’in favorilerde güvenli davranışı
- Invalid filtrelerin reddedilmesi (parent scope + dynamic property dahil)
- Query abuse / çok pahalı sorgu koruması
- N+1 sorguların önlenmesi
- Ölçüme dayalı index geliştirmesi
- Shared DB’de yalnız `hrd_` alanının kullanılması

Bunlar kesin API veya fonksiyon listesi değildir.

## 3. Public görünürlük temel filtresi

Kesin kurallar:

- Yalnız `PUBLISHED` public arama ve public detail’de görünür.
- Public detail’de status tekrar doğrulanır.
- `DRAFT`, `PENDING_REVIEW`, `CHANGES_REQUESTED`, `REJECTED`, `SUSPENDED`, `SOLD`, `ARCHIVED` public değildir.
- Status history public response değildir.
- Moderasyon nedeni, admin notu veya operasyonel alanlar public değildir.
- Status değişikliği ile pagination yarışlarında sonuçlar hareket edebilir; her query anlık public status’a göre çalışır.
- Search sırasında yayından kaldırılan advert sonraki sayfada görünmez; detail’de tekrar kontrol edilir.
- Public result cache varsa status/media invalidation ihtiyacı vardır; ilk fazda zorunlu cache yoktur.
- Category pasifleştirme advert status’unu otomatik değiştirmez; pasif category altındaki mevcut `PUBLISHED` advert’lerin list/detail davranışı ayrı ürün kararıdır.

**Seller `DISABLED` / `CLOSED`:**

- Advert status ile seller hesap status’u aynı kavram değildir.
- `DISABLED` veya `CLOSED` seller’a ait `PUBLISHED` advert’lerin public davranışı henüz ürün kararıdır.
- **İlk faz önerisi:** Ürün kararı netleşene kadar güvenli varsayılan olarak yalnız advert `PUBLISHED` filtresine dayanılır; seller status ile otomatik gizleme bu dokümanda kesinleştirilmez ve açık kalır. Kesin karar sonraki ürün onayıyla verilir.

## 4. Category scope ve ağaç filtresi

### Parent category veya root browse

- Parent category seçimi descendant leaf kategorilerdeki `PUBLISHED` advert’leri kapsayabilir.
- Bu kapsamda yalnız category’ler arasında güvenli biçimde ortak olan sabit filtreler kullanılır:
  - Province
  - District
  - Horse
  - Fotoğraflı ilan
  - Kesinleşirse fiyat
  - Whitelist sort
- Parent scope altında descendant property tanımları yalnız code adına göre birleştirilmez.
- Aynı code’un descendant kategorilerde aynı anlam/type/options taşıdığı varsayılmaz.
- İlk fazda parent scope için otomatik “ortak dynamic property kesişimi” hesaplanmaz.
- İlk fazda property inheritance veya cross-category property federation tasarlanmaz.
- Parent/root seçiliyken gönderilen dynamic property filtresi sessizce yok sayılmaz; açık validation error döner.

### Leaf category seçimi

- Dynamic property filtreleri yalnız tek ve kesin leaf category seçildikten sonra sunulur ve kabul edilir.
- Backend filter’ı seçili leaf category’ye ait aktif, public ve filterable property definition üzerinden doğrular.
- Client’ın başka descendant category’ye ait aynı code’u göndermesi reddedilir.
- Property code tek başına global kimlik değildir; category context ile anlam kazanır.
- Leaf category seçilmeden dynamic property filtresi gönderilirse açık validation error döner.

### Advert category edit vs taxonomy change

**Advert’in category referansının değiştirilmesi:**

- Normal kullanıcı akışında yalnız `DRAFT`.
- Category değişince mevcut dynamic property değerleri temizlenir ve kullanıcı uyarılır (lifecycle kararı).
- `PENDING_REVIEW`, `CHANGES_REQUESTED`, `PUBLISHED`, `REJECTED`, `SUSPENDED`, `SOLD`, `ARCHIVED` durumlarında category değişikliği yapılamaz.
- Özellikle published advert’in category’si arama sırasında sessizce değiştirilmez.
- Search sistemi advert’in mevcut category referansını okur; public search category’yi kendi başına değiştirmez veya yeniden sınıflandırmaz.

**Category taxonomy/tree değişikliği:**

- Örnek: parent değiştirme, başka dal altına taşıma, pasif yapma.
- Taxonomy değişikliği advert’in tuttuğu category referansını otomatik olarak başka category’ye dönüştürmez.
- Parent değişikliği browse/descendant kapsamını etkileyebilir.
- Existing advert’in category kimliği aynı kalır.
- Category’nin pasif yapılması tarihsel advert relation’ını fiziksel olarak bozmaz.
- Pasif category nedeniyle advert otomatik `ARCHIVED`, `SUSPENDED` veya başka status’a geçirilmez.
- Public browse scope yalnız aktif category seçimine izin verebilir; tarihsel `PUBLISHED` advert’in list/detail görünürlüğü ayrı ürün kararıdır.

Karıştırılmayan kavramlar: advert category edit, category tree reparent, category deactivation, advert status değişikliği, public browse scope.

Gelecekte parent’ta ortak dynamic filtre istenirse ayrı parent-level property tanımı, doğrulanmış inheritance veya semantik mapping gerekir; ilk fazda varsayılmaz.

Tek category scope ilk faz için yeterlidir. Birden fazla category seçimi zorunlu değildir. Exact recursive SQL / taxonomy migration yazılmaz.

## 5. Province ve district filtresi

Kurallar:

- Advert yalnız district referansı taşır.
- Province, district ilişkisi üzerinden türetilir.
- Province seçimi tüm bağlı district’leri kapsar (relation/join).
- District seçilirse ait olduğu province ile tutarlı olmalıdır.
- Province ve district birlikte gelirse çelişki reddedilir.
- Pasif district/province geçmiş advert’lerde bulunabilir; display güvenli tutulur.
- FE bağımlı dropdown kullanır; backend yine doğrular.
- Runtime external location API kullanılmaz.

**İlk faz:** Province veya district filtresi; çelişkide validation error; join üzerinden uygulama.

## 6. Horse autocomplete ve horse filtresi

Akış:

1. Kullanıcı Haradan backend autocomplete’e yazar.
2. Normalize horse adı üzerinden prefix arama yapılır.
3. Sonuçlarda gerçek/orijinal ad gösterilir; aynı adlı horse’lar TJK numarası ve diğer ayırt edici bilgilerle ayrıştırılır.
4. Kullanıcı belirli horse kaydını Haradan internal/public-safe identifier ile seçer.
5. Advert search horse relation filtresi bu seçime göre uygulanır.

Kurallar:

- Canlı TJK fallback yoktur.
- Yalnız text horse name gönderilmesi nihai kimlik olarak kabul edilmez.
- Horse bulunamazsa filter validation error veya empty-safe reddi.
- TJK geçici kesintisinde son güvenilir yerel veri kullanılır.
- Pasif/stale horse ürün kararıyla sınırlanabilir; advert relation korunur.

Horse adı kimlik değildir.

## 7. Structured text search sınırı

Seçenekler: yalnız structured filters; basit normalize prefix/contains; PostgreSQL full-text/trigram; harici Elasticsearch/OpenSearch.

Bağlam:

- Advert title ve description sabit advert alanları olarak kabul edilmiştir.
- Genel keyword search ürün kapsamı hâlâ netleştirilebilir.
- Türkçe normalizasyon ihtiyacı vardır.
- Shared PostgreSQL’de extension varsayılan/izinsiz kurulmaz.
- Typo tolerance ilk fazda zorunlu değildir.

**İlk faz önerisi:** Yapılandırılmış filtrelere öncelik. Genel free-text search zorunlu değildir. Title/description üzerinde sınırlı normalize prefix/contains ürün onayıyla eklenebilir; FTS/trigram extension veya harici search engine doğrulanmış gereksinim olmadan zorunlu değildir.

## 8. Dynamic property filter modeli

Kavramsal akış:

1. Tek leaf category seçilir (parent/root yeterli değildir).
2. Backend yalnız bu leaf category’nin property metadata’sını çözer.
3. Yalnız filterable ve public olarak izin verilen property’ler kabul edilir.
4. Property data type’a göre operator whitelist belirlenir.
5. Filter value validate/normalize edilir.
6. JSONB advert values üzerinde güvenli sorguya dönüştürülür.

Değerlendirme:

- `filterable` metadata gereklidir.
- String/text, numeric, boolean, single-select desteklenir.
- Multi-select varsa davranışı açık ürün kararıdır.
- Date/year benzeri değerler type’a göre whitelist operator ile.
- Range ve exact match type’a göre.
- Option code kullanılır; display label kimlik değildir.
- Deactivated property/option yeni filtrede kabul edilmez; mevcut advert historical value korunabilir.
- Property code category-scoped unique olduğundan **tek leaf** category context zorunludur.
- Parent/root browse altında dynamic property kabul edilmez.
- Descendant’lardan code adına göre property birleştirme / inheritance / federation yoktur.
- Client arbitrary JSON path/operator gönderemez.
- Missing/null property: eşleşmez; sessiz yanlış sonuç üretilmez.
- Type mismatch reddedilir.
- Birden fazla dynamic filter ilk fazda **AND** birleşir; OR ürün kararıdır.

**Reddedilenler:** raw SQL, raw JSONPath, her JSONB key otomatik filterable, metadata olmadan operator, parent scope’ta dynamic filter, code-based cross-category merge.

Exact JSONB operator veya SQL yazılmaz.

## 9. Sabit alan ve fiyat filtresi

Ayrım:

- Sabit advert alanı (örn. title, description, category, district, status)
- Dynamic category property (JSONB)
- Horse alanı (horse kaydı)
- Derived alan (province, photo availability, age)
- Gelecekte payment/package alanı (bu faz dışı)

**Fiyat:** Canonical sabit advert alanı olup olmadığı mevcut karar belgelerinde kesin değildir; açık ürün kararıdır. Min/max, para birimi, fiyatsız/“iletişime geç”, sıfır değer, price sort bu kesinleşmeye bağlıdır. Fiyat dynamic JSONB’ye taşınmamalıdır (kesinleşirse sabit alan olur). Paket fiyatı ile advert fiyatı karıştırılmaz. Uydurma fiyat modeli oluşturulmaz.

## 10. Fotoğraflı ilan filtresi

Kesin semantik:

- Yalnız advert-media ilişkisi varlığı yeterli değildir.
- Ham upload yeterli değildir.
- Canonical master yeterli değildir.
- PROCESSING veya FAILED varyant yeterli değildir.
- İlgili public kullanım için zorunlu READY varyantı bulunan en az bir advert media gerekir.
- Removed/detached ilişki sayılmaz.
- Public list cover: explicit kapak varsa ve READY ise o; yoksa kurallara uygun ilk READY görsel.
- List summary ve detail aynı READY semantiğini kullanır.

## 11. Sorting modeli

Değerlendirme:

- En yeni / en eski: uygun.
- Fiyat artan/azalan: yalnız price modeli kesinleşirse.
- Relevance: yalnız gerçek text search varsa.
- Dynamic property sort: ilk faz varsayılan değil.
- Rastgele, popülerlik/favori sayısı, admin boost, campaign/package boost: ilk fazda yok / payment dışı.

Kesin ilkeler:

- Client arbitrary DB column veya JSONB key ile sort yapamaz.
- Backend whitelist sort seçenekleri sunar.
- Her sort deterministic tie-breaker içermelidir (stable sonuç).
- Payment/package/campaign boost mevcut faza eklenmez.

**Default sort:** En yeni (yayın/public zamanına göre) + stable tie-breaker. Exact column veya SQL yazılmaz.

## 12. Pagination seçenekleri

| Kriter | Offset/page | Cursor/keyset | Hibrit |
| --- | --- | --- | --- |
| Büyük dataset | Zayıf derin sayfa | İyi | Orta |
| Mobil scroll | Orta | İyi | İyi |
| Web page navigation | İyi | Zayıf | İyi |
| Yeni kayıt eklenmesi | Skip/dup riski | Daha iyi | Orta |
| Yayından kalkan kayıt | Hareket eder | Hareket edebilir | Orta |
| Deep page performansı | Kötü | İyi | Orta |
| Exact total | Kolay | Zorunlu değil | Mümkün |
| Uygulama maliyeti | Düşük | Orta | Yüksek |
| Bu projeye uygunluk | Orta | Yüksek | Orta |

**İlk faz:** Public search için cursor/keyset temel yaklaşım. Exact total count zorunlu değildir. Web page number ihtiyacı ürün kararıysa sonra hibrit değerlendirilir.

## 13. Cursor güvenliği ve deterministik ilerleme

- Cursor opaque ve backend tarafından üretilir.
- Sort value + stable tie-breaker mantığını temsil edebilir.
- Client cursor içeriğini güvenilir veri olarak belirleyemez.
- Cursor farklı sort/filter kombinasyonunda kontrolsüz kullanılamaz.
- Filter/sort fingerprint veya eşdeğer doğrulama değerlendirilir.
- Invalid veya eski cursor güvenli biçimde reddedilir.
- Cursor hassas veri içermemelidir.
- Exact serialization, signing veya token formatı belirlenmez.
- Pagination snapshot isolation sağlamıyorsa yayın durumu değişince sonuçlar hareket edebilir.
- Exact total count cursor için zorunlu değildir.
- Offset kullanılacak yerlerde derin offset maliyeti yüksektir; sınırsız derin offset yoktur.

## 14. Public list summary read model

Kavramsal alan adayları:

- Advert public kimliği
- Category display bilgisi (advert’in mevcut category referansından)
- Location display bilgisi
- Horse özet bilgisi
- Public sabit advert alanları (title vb.)
- Gerekli seçilmiş public dynamic property özetleri (advert’in leaf category context’ine göre)
- Cover READY varyant URL/descriptor
- Yayın zamanı
- Fiyat, yalnız kesinleşirse
- Authenticated kullanıcı için `is_favorite`
- Public favorite count, yalnız ürün kararıysa

İlkeler:

- Bütün advert aggregate liste başına yüklenmez.
- Ham JSONB’nin tamamı list response’a dökülmez.
- Canonical master veya storage internal key public değildir.
- Status history ve moderation alanları public değildir.
- N+1 category/location/horse/media/favorite sorguları önlenir.
- Taxonomy reparent advert’in category kimliğini otomatik değiştirmez; summary mevcut referansı gösterir.
- Exact DTO veya endpoint tasarlanmaz.

## 15. Public detail read model

- Yalnız `PUBLISHED`; status tekrar doğrulanır.
- Full public advert bilgisi (write modelinin tamamı değil).
- Category metadata ile display edilebilir dynamic values (advert’in mevcut leaf category referansı üzerinden).
- Horse public temel alanları; kontrollü public horse detail bölümleri (raw TJK payload yok).
- Province/district gösterimi.
- Sıralı READY media + cover.
- Seller’a ait public profil/iletişim alanları mevcut belgelerde kesin değilse açık karar.
- Favorite state (authenticated).
- Benzer ilanlar ürün kararı.
- Moderasyon/internal alanlar ve raw property validation internalleri dönmez.
- Public search/detail advert category’sini yeniden sınıflandırmaz.
- Pasif category altındaki tarihsel `PUBLISHED` detail görünürlüğü açık ürün kararıdır.

## 16. Favori ilişkisinin davranışı

Kesin ilkeler:

- `ACTIVE` authenticated user gerekir.
- User + advert uniqueness; duplicate yok.
- Add idempotent; remove idempotent.
- Başkasının favori ilişkisini değiştiremez.
- Admin rolü sahiplik kontrolünü otomatik bypass etmez.
- Advert owner’ın kendi advert’ini favorilemesi ürün kararıdır (açık).
- Public olmayan advert yeni favori yapılamaz.
- Advert favoriyken public durumdan çıkarsa ilişki fiziksel silinmez.
- Advert tekrar `PUBLISHED` olursa mevcut ilişki kullanılabilir.
- User `DISABLED` / `CLOSED` favori işlemi yapamaz.
- Hard delete zorunlu yaklaşım değildir.
- Favorite timestamps/history ürün ihtiyacıdır.
- Public favorite count zorunlu değildir.

Exact tablo veya constraint yazılmaz.

## 17. Favoriler listesinde public olmayan advert davranışı

| Kriter | İlişkiyi sil | Tamamen gizle | Güvenli placeholder |
| --- | --- | --- | --- |
| UX | Kaybolmuş hissi | Kaybolmuş | Anlaşılır |
| Gizlilik | İyi | İyi | Dikkatli tasarlanmalı |
| Tekrar yayınlanma | İlişki kaybolabilir | Korunur | Korunur |
| Veri bütünlüğü | Zayıf | İyi | İyi |
| Pagination | Basit | Dikkat | Dikkat |
| İlk faz uygunluk | Düşük | Orta | Yüksek |

**Öneri:** İlişkiyi koruyup sınırlı “artık kullanılamıyor” placeholder. Eski içerik, status detayı veya moderation nedeni gösterilmez. Suspended/sold/archived ayrımı sızdırılmaz.

## 18. Favorite state enrichment

- Public search anonim kullanılabilir.
- Authenticated kullanıcı için `is_favorite` eklenebilir.
- Her satır için ayrı sorgu/N+1 reddedilir.
- Tek query join/exists veya sonradan bulk enrichment uygundur.
- Anonymous response’ta favorite state yok veya false-equivalent.
- User-specific response ortak public cache’e karışmaz.
- Auth optional endpoint davranışı güvenli tasarlanır.

## 19. Favorite count

Seçenekler: public gösterme, yalnız BO, hiç göstermeme, exact/derived/counter cache.

**İlk faz:** Public favorite count zorunlu değildir. Ürün gereksinimi yoksa eklenmez. Manipülasyon/sosyal kanıt etkisi dikkate alınır.

## 20. Filter validation ve hata davranışı

Senaryolar: bilinmeyen category; parent/root seçiliyken dynamic property; leaf seçilmeden dynamic property; başka leaf’e ait property code; category’ye ait olmayan property; geçersiz operator; type mismatch; min > max; province/district çelişkisi; bilinmeyen horse; çok fazla filter; çok uzun text; invalid cursor; desteklenmeyen sort; deactivated property/option; pasif category ile browse seçimi.

| Yaklaşım | Risk |
| --- | --- |
| Sessizce yanlış sonuç | Yüksek; güven kırılır |
| Bilinmeyen filtreyi yok sayma | Yanıltıcı sonuç |
| Açık validation error | Güvenli; FE mesajı gerekir |

**Öneri:** Açık validation error; FE kullanıcı dostu mesaj gösterir. Sessiz yok sayma yok. Parent scope + dynamic property ve leaf’siz dynamic property açık hata üretir.

## 21. Query abuse ve güvenlik sınırı

Değerlendirme: max page size, filter sayısı, text uzunluğu, dynamic complexity, arbitrary sort/JSON path reddi, expensive count, deep offset, rate limiting, timeout, cancellation, public abuse, BO/private alan sızıntısı.

Kesin limit sayısı uydurulmaz.

**İlk faz:** Backend-enforced query complexity + whitelist. Arbitrary sort/JSONPath/SQL yok.

## 22. Query performansı ve index sınırı

Kavramsal adaylar: public status, category, district, province relation, horse relation, default sort, favorite uniqueness/lookups, READY public media existence, normalize horse prefix, dynamic JSONB filters, partial/selective/composite/GIN/expression index ihtiyacı, FTS/trigram extension, query plan ölçümü, shared DB kaynak kullanımı.

Kesin ilkeler:

- Her property için otomatik index yok.
- Tüm olası filtre kombinasyonlarına index yok.
- Sabit ve sık sorgulanan alanlar schema aşamasında değerlendirilir.
- JSONB indexleri gerçek filterable property ve trafik ölçümüne göre eklenir.
- Extension ortak DB üzerinde izinsiz/varsayılan kurulmaz.
- `hr_` nesnelerine dokunulmaz.
- Haradan index/constraint adları `hrd_` namespace ile ayrılır.
- Exact index veya SQL üretilmez.

## 23. Public read query mimarisi

| Kriter | Full aggregate hydrate | Search-specific projection | Materialized view | Harici search |
| --- | --- | --- | --- | --- |
| N+1 | Risk | Kontrollü | İyi | İyi |
| Sadelik | Zayıf | Yüksek | Orta | Düşük |
| Tutarlılık | İyi | İyi | Invalidation | Sync maliyeti |
| Performans | Orta | İyi | Çok iyi | Çok iyi |
| Operasyon | Düşük | Düşük | Orta | Yüksek |
| İlk faz | Düşük | Yüksek | Düşük | Düşük |

**Öneri:** Aynı PostgreSQL; search/public read için özel query/read repository; domain write modelinden ayrı projection/DTO; materialized view veya search engine olmadan başlama; ölçüm sonrası optimizasyon; N+1 önleme; tek source of truth.

## 24. Cache sınırı

Değerlendirme: category/location metadata cache; public search/detail cache; user-specific favorite; status/media invalidation; Redis; in-process; CDN.

**İlk faz:** Cache zorunlu değildir. Category/location metadata için hafif in-process veya uygulama içi cache değerlendirilebilir. Public search result cache zorunlu değil. Redis doğrulanmış ihtiyaç olmadan zorunlu değildir. User-specific favorite state ortak public cache’e karışmaz.

## 25. Eş zamanlılık ve tutarlılık

Yarışlar: pagination sırasında yeni yayın / yayından kalkma; favorite add/remove; iki cihazdan aynı favori; media READY/cover değişimi; category/property metadata değişimi; taxonomy reparent; category deactivation; deactivated option + mevcut advert; seller status değişimi; cursor + farklı filter; DRAFT dışı advert category edit denemesi.

**İlk faz yaklaşımı:**

- DB uniqueness (favorite)
- Idempotent favorite
- Stable sort + tie-breaker
- Cursor validation (filter/sort)
- Public status her query/detail’de doğrulanır
- Historical JSONB value korunabilir
- Metadata değişiminde display metadata ile; leaf filter yalnız aktif filterable üzerinden
- Advert category edit yalnız `DRAFT`; published advert aramada yeniden sınıflandırılmaz
- Taxonomy reparent advert category kimliğini otomatik değiştirmez; browse descendant kapsamı değişebilir
- Category deactivation advert status’unu otomatik değiştirmez
- Exact transaction/SQL yazılmaz

## 26. FE, BO ve backend sorumlulukları

### Haradan FE

- Public search/filter/sort/pagination sunar.
- Parent/root browse’da yalnız ortak sabit filtreleri gösterir.
- Dynamic property filtrelerini yalnız leaf category seçildikten sonra gösterir.
- Kullanıcıya raw property code veya JSONB operator göstermez.
- Province/district bağımlılığını yönetir.
- Horse autocomplete’i Haradan backend’den kullanır.
- Favori işlemleri için authenticated session kullanır.
- Backend validation yerine geçmez.
- Invalid filtrede (parent + dynamic dahil) anlaşılır davranış sunar.
- Admin/moderation filtresi sunmaz.
- Taxonomy yönetmez; advert category edit kurallarını lifecycle’a bırakır.

### Haradan BO

- Bu public search read model’in güvenlik sınırını bypass etmez.
- Moderasyon listeleme ihtiyaçları ayrı BO query/use-case olabilir.
- Public olmayan advert verisini public endpoint üzerinden açmaz.
- Category taxonomy (reparent/deactivation) BO operasyonudur; published advert’leri sessizce taşımamalıdır.
- Favorite count veya arama operasyon metrikleri ürün kararıysa görebilir.
- Exact ekran tasarımı yapılmaz.

### Backend

- Public visibility filtresinin güvenilir kaynağıdır.
- Parent browse ile leaf dynamic filter semantiğini ayırır.
- Category/location/horse/property filtrelerini doğrular.
- Parent/root + dynamic property veya leaf’siz dynamic property’yi reddeder.
- Raw JSONPath veya arbitrary sort kabul etmez.
- Pagination/cursor bütünlüğünü korur.
- Search summary/detail projection’larını üretir.
- READY medya seçimini uygular.
- Favorite ownership ve uniqueness uygular.
- Optional auth favorite enrichment yapabilir.
- N+1 ve pahalı query risklerini yönetir.
- Advert category’yi search sırasında değiştirmez.
- `hr_` nesnelerine dokunmaz.
- FE/BO’nun gönderdiği owner, role veya internal field’e güvenmez.

## 27. Karşılaştırma tabloları

### 27.1 Pagination yaklaşımı

(Bkz. bölüm 12 tablosu.)

### 27.2 Text search yaklaşımı

| Kriter | Yalnız structured | Prefix/contains | PG FTS/trigram | Harici engine |
| --- | --- | --- | --- | --- |
| Türkçe | Filtre odaklı | Normalize ile | Extension gerekir | Güçlü |
| Typo | Yok | Zayıf | Orta | İyi |
| Ek altyapı | Yok | Yok | Extension | Yüksek |
| Shared DB | Güvenli | Güvenli | İzin gerekir | Ayrı |
| Performans | İyi | Orta | Orta-iyi | İyi |
| İlk faz maliyeti | Düşük | Düşük | Orta | Yüksek |
| Bu projeye uygunluk | Yüksek | Orta (opsiyonel) | Düşük | Düşük |

### 27.3 Dynamic property filter yaklaşımı

| Kriter | Raw JSONPath | Her key filterable | Metadata validated (leaf) | Ayrı value table |
| --- | --- | --- | --- | --- |
| Güvenlik | Kötü | Zayıf | Yüksek | Yüksek |
| Tip doğrulama | Yok | Zayıf | Yüksek | Yüksek |
| Query performansı | Risk | Risk | Kontrollü | İyi |
| Esneklik | Yanıltıcı | Yanıltıcı | Dengeli | Yüksek |
| İlk faz maliyeti | Düşük yanıltıcı | Düşük yanıltıcı | Orta | Yüksek |
| Kabul edilmiş model | Uyumsuz | Uyumsuz | Uyumlu | Erken |

### 27.4 Public read modeli

(Bkz. bölüm 23 tablosu.)

### 27.5 Public olmayan favori davranışı

(Bkz. bölüm 17 tablosu.)

### 27.6 Index yaklaşımı

| Kriter | Her property index | Hiç plan yok | Sabit + ölçüm JSONB | Harici engine |
| --- | --- | --- | --- | --- |
| Yazma maliyeti | Yüksek | Düşük | Dengeli | Ayrı |
| Storage | Yüksek | Düşük | Dengeli | Ayrı |
| Query | Değişken | Risk | İyi | İyi |
| Bakım | Zor | Kör | Yönetilebilir | Ağır |
| Shared DB | Kötü | Risk | İyi | Ayrı |
| İlk faz | Düşük | Düşük | Yüksek | Düşük |

## 28. Önerilen karar

**Özet model:** Public yalnız `PUBLISHED`; parent/root browse descendant advert + ortak sabit filtreler; dynamic property yalnız tek leaf context; property code category-scoped; advert category edit yalnız `DRAFT`; taxonomy reparent advert referansını otomatik değiştirmez; province/district join; local horse autocomplete + ID; metadata-validated leaf filters (AND); fotoğraf = READY media; default newest + stable tie-breaker; cursor/keyset; ayrı summary/detail; idempotent favorites + güvenli placeholder; join/bulk enrichment; public favorite count zorunlu değil; açık validation; complexity whitelist; sabit + ölçüme dayalı JSONB index; search-specific read repository; cache/Redis/engine zorunlu değil; yalnız `hrd_`.

Soru cevapları:

1. **Public status?** Yalnız `PUBLISHED`.
2. **Detail status tekrar?** Evet.
3. **Seller DISABLED/CLOSED advert?** Açık ürün kararı; bu dokümanda kesinleştirilmez.
4. **Parent descendant kapsar mı?** Evet (advert browse).
5. **Parent scope’ta dynamic property filtresi?** Hayır, ilk fazda.
6. **Dynamic property filtresi leaf context gerektirir mi?** Evet; tek leaf.
7. **Category’siz / parent’ta arbitrary dynamic filter?** Hayır; validation error.
8. **Province district relation üzerinden mi?** Evet.
9. **Province/district çelişkisi reddedilir mi?** Evet.
10. **Horse autocomplete canlı TJK?** Hayır.
11. **Horse adı nihai kimlik?** Hayır.
12. **Genel text search zorunlu mu?** Hayır.
13. **PG extension zorunlu mu?** Hayır.
14. **Harici search engine?** Hayır.
15. **Property filter metadata doğrulama?** Evet.
16. **Client raw JSONPath?** Hayır.
17. **Her JSONB key filterable?** Hayır.
18. **Operator type whitelist?** Evet.
19. **AND/OR?** İlk faz AND; OR açık.
20. **Fiyat filtresi kesin mi?** Hayır; açık.
21. **Fotoğraflı ilan?** Public READY varyantı olan ≥1 advert media.
22. **Default sort?** En yeni + stable tie-breaker.
23. **Arbitrary sort?** Hayır.
24. **Dynamic property sort ilk faz?** Hayır.
25. **Pagination?** Cursor/keyset.
26. **Cursor opaque?** Evet.
27. **Cursor filter/sort doğrulama?** Evet.
28. **Exact total zorunlu mu?** Hayır.
29. **List summary ayrı projection?** Evet.
30. **Detail ayrı projection?** Evet.
31. **Liste bütün JSONB?** Hayır.
32. **Canonical master/object key public?** Hayır.
33. **Favorite add idempotent?** Evet.
34. **Favorite remove idempotent?** Evet.
35. **Duplicate favorite?** Hayır.
36. **Public olmayan yeni favori?** Hayır.
37. **Public çıkınca favori silinsin mi?** Hayır (zorunlu hard delete yok).
38. **Public olmayan favori gösterimi?** Güvenli placeholder; içerik/moderation sızmaz.
39. **Favorite state N+1?** Hayır; join/bulk.
40. **Public favorite count zorunlu mu?** Hayır.
41. **Invalid filtre sessiz yok sayılsın mı?** Hayır; validation error.
42. **Query complexity sınırlı mı?** Evet.
43. **Her property index?** Hayır.
44. **JSONB index ölçüme göre mi?** Evet.
45. **Search-specific read repository?** Evet.
46. **Materialized view ilk faz?** Hayır.
47. **Redis/cache zorunlu mu?** Hayır.
48. **Favorite state public cache’e karışır mı?** Hayır.
49. **Stable tie-breaker?** Evet.
50. **Public search `hr_` dokunur mu?** Hayır.
51. **`hrd_` prefix?** Evet.
52. **Advert category edit ne zaman?** Yalnız `DRAFT`.
53. **Taxonomy reparent advert category’yi otomatik değiştirir mi?** Hayır.
54. **Pasif category advert status’unu otomatik değiştirir mi?** Hayır.

**Gerekçe:** İleri yaş kullanıcılar için sade category-driven filtreler; leaf property güvenliği; lifecycle ile uyumlu category edit; taxonomy/browse ayrımı; web/Android/iOS; JSONB esnekliği + query güvenliği; PostgreSQL/Railway; shared test DB; N+1 önleme; mobil cursor pagination; public/moderation ayrımı; favori gizliliği; ölçüme dayalı indeksleme; gereksiz search engine/cache kurmama; gelecekte büyüme.

## 29. Reddedilen yaklaşımlar

- Public aramada `PUBLISHED` dışındaki advert’leri döndürmek
- Public detail’de status kontrol etmemek
- BO moderasyon filtresini public search’e karıştırmak
- Client’tan raw SQL / raw JSONPath kabul etmek
- Her JSONB key’i otomatik filterable saymak
- Metadata/type doğrulaması olmadan operator uygulamak
- Category context olmadan category-scoped property code filtrelemek
- Parent/root scope’ta dynamic property filtresi kabul etmek
- Parent scope’ta descendant property’leri code adına göre birleştirmek
- İlk fazda property inheritance veya cross-category federation varsaymak
- Parent + dynamic filter’ı sessizce yok saymak
- “Category değişikliği mevcut advert’leri anlık taşır” varsayımı
- Published advert’in category’sini search sırasında sessizce değiştirmek
- Taxonomy reparent’ın advert category referansını otomatik dönüştürmesi
- Category pasifleştirmenin advert’i otomatik `ARCHIVED`/`SUSPENDED` yapması
- Horse adını unique kimlik saymak
- Public autocomplete’i canlı TJK’ya bağlamak
- Shared DB’de doğrulanmadan extension kurmak
- İlk fazda zorunlu olmadan Elasticsearch/OpenSearch kurmak
- Arbitrary column veya JSONB sort kabul etmek
- Stable tie-breaker olmadan pagination
- Derin offset’i sınırsız kullanmak
- Cursor’ı filter/sort değişince aynen kullanmak
- Liste sonucunda bütün aggregate’i hydrate etmek
- Her satır için ayrı category/location/horse/media/favorite sorgusu
- Ham upload veya canonical master’ı public göstermek
- Sadece asset ilişkisi var diye fotoğraflı saymak
- Favorite duplicate’e izin vermek
- Favorite add/remove’u non-idempotent yapmak
- Advert public durumdan çıkınca favoriyi zorunlu hard delete etmek
- Public olmayan advert’in moderation nedenini favori sahibine göstermek
- User-specific favorite state’i ortak public cache’e karıştırmak
- Her property için otomatik index / ölçümsüz tüm kombinasyon index
- Redis veya search engine’i doğrulanmış ihtiyaç olmadan zorunlu yapmak
- Haradan query/migration’larının `hr_` nesnelerine dokunması
- Secret `.env` içeriğini dokümana veya Git’e yazmak

## 30. Açık kalan ürün ve teknik kararlar

- Seller `DISABLED` / `CLOSED` iken `PUBLISHED` advert public davranışı
- Kesin category taxonomy ve leaf zorunluluğu
- Birden fazla category filter ihtiyacı
- Pasif category altındaki mevcut `PUBLISHED` advert’in listede görünmesi
- Pasif category altındaki mevcut `PUBLISHED` advert’in direct detail görünürlüğü
- Category tree reparent sonrası mevcut cursor/search davranışı
- Property definition’da `filterable` niteliğinin kesin kapsamı
- Her property type için operator seti
- Multi-select property desteği
- Dynamic filter OR ihtiyacı
- Gelecekte parent-level ortak dynamic property ihtiyacı
- Advert title/description genel keyword search kapsamı
- Türkçe fuzzy search gereksinimi
- PostgreSQL extension izni
- Fiyatın canonical advert alanı olup olmadığı
- Para birimi
- Default sort’ın kesin ürün onayı
- Web page number ihtiyacı
- Exact total count ihtiyacı
- Cursor formatı ve süresi
- Page size sınırı
- Filter complexity sınırı
- Public list summary alanları
- Public detail seller iletişim/profil alanları
- Public dynamic property display kapsamı
- Public horse detail kapsamı
- Public olmayan favori placeholder UX’i
- Advert sahibinin kendi advert’ini favorileyip favorileyemeyeceği
- Public favorite count
- Benzer ilanlar
- Cache ihtiyacı
- Query timeout
- Query plan ölçüm ve index ekleme eşiği
- Dynamic JSONB index adayları
- Shared DB kaynak limitleri
- Search/favorite observability ve log retention

## 31. Sonraki adım

Bu karar kabul edilirse kapsamlı analiz aşaması tamamlanmaya çok yaklaşır.

Sonraki adım:

- Kabul edilmiş bütün karar belgelerinin çapraz tutarlılık kontrolü
- Phase-one kapsamının kesinleştirilmesi
- `hrd_` prefix’li kesin PostgreSQL tablo yapısı
- PK/FK ve relationship’ler
- Constraint ve index planı
- Migration sırası
- Fonksiyon/use-case listesi
- FE ve BO API ihtiyaçlarının ayrılması
- OpenAPI sözleşmesi
- Backend geliştirme sırası

Bir sonraki belgede kavramsal analiz yerine kesin veri modeli ve uygulama blueprint’ine geçilir.
