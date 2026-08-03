# FE, BO ve Admin Sorumluluk Sınırı

## 1. Kararın amacı

Admin operasyonlarının son kullanıcı uygulamasından ayrılmasının amaçları:

- Son kullanıcı deneyimini sade tutmak
- İleri yaş kullanıcılar için gereksiz yönetim karmaşıklığını kaldırmak
- Yönetim operasyonlarını tek uygulamada (`haradan-bo`) toplamak
- Güvenlik sınırını netleştirmek; UI gizlemenin yetki sayılmamasını sağlamak
- FE ve BO geliştirme sorumluluklarını ayırmak
- Aynı backend iş kurallarının bütün istemcilerde tutarlı uygulanmasını sağlamak

Kesin şirket kararı: Admin ekranları ayrı `haradan-bo` reposunda geliştirilir. Tüm admin ve operasyon işlemleri BO üzerinden yönetilir. Public ve normal kullanıcı ekranları `haradan-fe` reposundadır. `haradan-fe` içinde mümkün olduğunca admin’e özel ekran, menü, kontrol ve işlem bulunmamalıdır. Admin bir işlem yapacaksa bunu BO üzerinden yapabilmelidir.

## 2. Haradan FE sorumlulukları

`haradan-fe`, public ve normal kullanıcı uygulamasıdır (React Native; web, Android ve iOS için tek kod tabanı hedefi). Admin yönetim uygulaması değildir.

Yalnız public ve normal kullanıcı akışları (kavramsal):

- Kayıt
- Giriş ve çıkış
- Profil işlemleri
- İlan arama ve filtreleme
- İlan detayını görüntüleme
- Favoriye ekleme ve favoriden çıkarma
- Kullanıcının favorilerini görüntülemesi
- İlan oluşturma
- Kullanıcının kendi ilanlarını görüntülemesi
- Kullanıcının kendi ilanında izin verilen güncelleme işlemleri
- Kullanıcının kendi ilanında izin verilen yaşam döngüsü işlemleri
- Görsel yükleme
- Görsel sıralama
- Kapak görseli seçme
- Kategori ağacını ve aktif kategorileri okuma
- Kategori form metadata’sını okuyarak sade ilan formu oluşturma
- Haradan veritabanında horse autocomplete kullanma
- Atı seçtikten sonra TJK verilerini otomatik veya salt okunur görüntüleme

Normal kullanıcı yalnızca kendi kaynakları üzerinde işlem yapabilir. Bu dokümanda ekran, endpoint veya fonksiyon listesi üretilmez.

## 3. Haradan FE içinde bulunmaması gereken admin işlemleri

Aşağıdakiler `haradan-fe` içinde bulunmamalıdır:

- İlan moderasyon kuyruğu
- Başka kullanıcıların ilanlarını yönetme
- İlan onaylama
- İlan reddetme
- İlanda değişiklik isteme
- İlan askıya alma
- Moderasyon gerekçesi yönetme
- Kategori oluşturma
- Kategori güncelleme
- Kategori ağacında taşıma ve sıralama
- Kategori aktif/pasif yönetimi
- Category property oluşturma veya güncelleme
- Property seçenekleri yönetimi
- Property validasyon kuralları yönetimi
- Property görünürlük ve filtre metadata’sı yönetimi
- Banner oluşturma veya güncelleme
- TJK senkronizasyon operasyonlarını yönetme
- TJK hata, retry ve checkpoint operasyonlarını yönetme
- Worker hata ve retry operasyonlarını yönetme
- Kullanıcı rolü veya kullanıcı durumu yönetme
- Sistemsel operasyon ve denetim ekranları

FE’nin admin rolünü algılayarak admin menüsü veya moderasyon ekranı üretmesi ana yaklaşım değildir. Admin işlemleri BO üzerinden yapılır.

## 4. Haradan BO sorumlulukları

`haradan-bo`, admin ve operasyon backoffice uygulamasıdır (ayrı repo; teknolojisi henüz kesinleşmemiştir).

Kavramsal sorumluluklar:

- İlan moderasyon kuyruğu
- İlan detay incelemesi
- İlanı onaylama
- İlanı reddetme
- İlanda değişiklik isteme
- İlanı askıya alma
- Moderasyon gerekçesi kaydetme
- Kategori oluşturma ve güncelleme
- Kategori hiyerarşisini yönetme
- Kategori sıralama
- Kategori aktif/pasif yönetimi
- Category property yönetimi
- Property sırası
- Property zorunluluğu
- Property form görünürlüğü
- Property liste ve detay görünürlüğü
- Property filtre ve arama metadata’sı
- Property seçenekleri
- Property validasyon kuralları
- Banner yönetimi
- TJK senkronizasyon durumunu görüntüleme
- TJK senkronizasyon hatalarını görüntüleme
- Gerekli retry ve operasyon işlemleri
- Worker operasyonlarını izleme
- Kullanıcılarla ilgili gerekli admin işlemleri
- Operasyonel kayıt ve geçmiş görüntüleme

Kesin BO ekran tasarımı, sayfa listesi veya teknoloji seçimi bu dokümanda yapılmaz.

## 5. Backend sorumluluğu

`haradan-be` backend iş kurallarını, PostgreSQL erişimini, FE ve BO API’lerini, authentication/authorization’ı, sahiplik kontrollerini, TJK entegrasyonunu, worker işlemlerini, medya ve diğer backend operasyonlarını sağlar.

İlkeler:

- Aynı backend FE ve BO’ya API hizmeti verebilir.
- Public, normal kullanıcı, sahiplik gerektiren ve admin işlemleri mantıksal olarak ayrılmalıdır.
- FE’nin kullandığı normal kullanıcı işlemleri admin yetkisine bağımlı olmamalıdır.
- BO işlemleri backend’de açık admin authorization kontrolü gerektirir.
- UI’de buton gizlemek güvenlik değildir.
- Backend her admin işleminde kimlik ve rol kontrolü yapmalıdır.
- Normal kullanıcı işlemlerinde kaynak sahipliği kontrol edilmelidir.
- Normal kullanıcı başka kullanıcının ilanını değiştirememelidir.
- Normal kullanıcı admin operasyonlarını çağıramamalıdır.
- Admin API’sine normal kullanıcı erişimi reddedilmelidir.
- Admin’in FE içinden işlem yapabilmesi zorunlu değildir.
- BO, bütün gerekli admin operasyonlarını karşılayabilmelidir.
- FE ve BO için çelişen iş kuralları geliştirilmemelidir.
- Kritik kurallar yalnız istemcide değil backend’de uygulanmalıdır.

Bu dokümanda endpoint adı, URL veya request/response modeli tasarlanmaz.

## 6. Category ve property yönetimine etkisi

- Kategori yapısı hiyerarşiyi destekler; bir kategorinin isteğe bağlı üst kategorisi bulunabilir.
- Kategori ağacı ve category property tanımlarını BO yönetir.
- Admin kategori oluşturma, güncelleme, sıralama ve aktif/pasif işlemlerini BO üzerinden yapar.
- FE kategori veya property tanımlarını değiştirmez.
- FE yalnız aktif ve kendisine sunulan kategori ağacını ve form metadata’sını okur.
- FE seçilen kategoriye göre sade ilan formu üretir.
- Property oluşturma, güncelleme, pasifleştirme ve sıralama BO operasyonudur.
- Property seçenekleri, zorunluluk, validasyon, filtre ve görünürlük değişiklikleri BO operasyonudur.
- Advert üzerindeki gerçek kullanıcı property değerleri FE ilan oluşturma/güncelleme akışından gelir.
- BO, moderasyon amacıyla bu değerleri görüntüleyebilir.
- BO’nun kullanıcı tarafından girilmiş property değerlerini değiştirip değiştiremeyeceği daha sonra ayrı bir iş kuralı olarak belirlenmelidir.
- TJK’dan gelen (doğrulanmış) horse alanları category property olarak kullanıcıya tekrar sorulmamalıdır.
- Category property yapısı ileri yaş kullanıcılar için sade form üretimini desteklemelidir.
- Başlangıçtaki kesin üst ve alt kategori isimleri bu dokümanda belirlenmez.
- Kategori property mirası, maksimum derinlik ve üst kategoriye doğrudan ilan verilip verilemeyeceği bu dokümanda kesinleştirilmez.
- Bu dokümanda JSONB veya ilişkisel değer saklama kararı verilmez.

## 7. TJK operasyonlarına etkisi

Belgelerde TJK’dan alınabildiği belirtilen temel horse alanları kullanılabilir TJK verisi olarak kabul edilir:

- TJK numarası
- At adı
- Normalize edilmiş at adı
- Doğum yılı
- Baba adı
- Anne adı
- Irk
- Cinsiyet
- Don

Belgelerde değişken TJK detayları olarak geçen veri grupları da TJK domain kapsamındadır (kesin JSON yapısı bu dokümanda tasarlanmaz):

- Pedigri
- Yarış istatistikleri
- Kardeşler
- Yavrular
- Antrenör geçmişi

Operasyon sınırları:

- FE canlı TJK servisini çağırmaz.
- FE TJK senkronizasyonu başlatmaz.
- İlan oluşturma sırasında canlı TJK çağrısı yapılmaz.
- TJK verisi önceden Haradan PostgreSQL veritabanına senkronlanır.
- FE Haradan PostgreSQL veritabanına önceden alınmış horse kayıtlarında arama yapar.
- Kullanıcı at adından 2–3 karakter yazarak horse autocomplete kullanabilir.
- Arama sonucunda doğru atı ayırt etmeye yardımcı TJK bilgileri gösterilebilir.
- At seçildikten sonra temel TJK alanları otomatik veya salt okunur gösterilebilir.
- Kullanıcı TJK verisini değiştiremez; normal kullanıcı TJK horse kaydını düzenleyemez.
- YKK veya manuel horse kaynağı kullanılmaz.
- TJK sync başlatma, durum görüntüleme, hata, retry ve checkpoint operasyonları gerekiyorsa BO’ya aittir.
- Belgelerde tanımlı olmayan ek TJK alanlarına ihtiyaç duyulursa yalnız bu ek alanlar ayrıca doğrulanır.
- TJK JSON yapısı veya kesin tablo modeli bu dokümanda tasarlanmaz.

## 8. Yetkilendirme ilkeleri

- Rol yalnızca `user` ve `admin` değerlerinden oluşur.
- Rol doğrudan kullanıcı kaydında tutulacaktır.
- Rol saklama yaklaşımı `VARCHAR + CHECK constraint` olacaktır.
- Ayrı roles, permissions, role_permissions veya user_roles tabloları oluşturulmayacaktır.
- Rol backend tarafından güvenilir authentication context’inden okunur.
- İstemcinin request içinde gönderdiği role güvenilmez.
- FE içinde admin butonu gizlemek tek başına güvenlik sağlamaz.
- BO uygulamasına erişebilmek tek başına operasyon yetkisi sağlamaz.
- Her admin operasyonu backend tarafından yeniden kontrol edilir.
- Normal kullanıcı işlemlerinde sahiplik kontrolü uygulanır.
- Admin rolü kullanıcı kayıt veya profil güncelleme isteğiyle değiştirilemez.
- Admin işlemlerinde audit, actor ve gerekçe ihtiyacı daha sonra ayrıca değerlendirilir.

## 9. PostgreSQL ve Railway etkisi

- FE ve BO kalıcı veritabanına doğrudan bağlanmaz.
- Bütün kalıcı veri erişimi backend API üzerinden gerçekleştirilir.
- Güvenilir kayıt kaynağı PostgreSQL’dir.
- PostgreSQL Railway üzerinde barındırılacaktır.
- FE ve BO Railway PostgreSQL bağlantı bilgilerini taşımaz.
- Veritabanı credential ve secret bilgileri yalnız backend/worker ortamında yönetilir.
- PostgreSQL major sürümü gerçek Railway ortamı incelenerek belirlenecektir; bu dokümanda sabitlenmez.
- Storage: Backblaze B2, S3-compatible (istemci doğrudan DB’ye değil; medya/backend akışı üzerinden).
- Bu dokümanda ORM, SQL üretim aracı veya migration aracı seçilmez.

## 10. Gelecekteki paket, yükseltme ve iletişim sınırı

Payment ve paket sistemi mevcut backend geliştirme fazının dışındadır. Gelecekteki sorumluluk sınırı kavramsal olarak şöyledir.

**Normal kullanıcı FE tarafında ileride:**

- Mevcut ilan paketini görüntüleyebilir.
- Paket süresini görüntüleyebilir.
- Paket avantajlarını karşılaştırabilir.
- Standart paketten üst pakete geçebilir.
- Mevcut paketini yenileyebilir.
- Kendisine sunulan yükseltme veya yenileme tekliflerini görüntüleyebilir.

**BO tarafında ileride:**

- Paket tanımlarını yönetebilir.
- Paket avantajlarını yönetebilir.
- Paket fiyatlarını ve geçerlilik dönemlerini yönetebilir.
- Paket yükseltme kurallarını yönetebilir.
- Yenileme ve kampanya koşullarını yönetebilir.
- E-posta teklif ve hatırlatma kurallarını yönetebilir.
- Gönderim sonuçlarını ve operasyonları izleyebilir.

**Backend tarafında ileride:**

- İlanın aktif paket hakkını takip eder.
- Paket başlangıç ve bitiş zamanını takip eder.
- Paket yükseltme uygunluğunu hesaplar.
- Standart paketten üst pakete geçişi destekler.
- Yenileme ve yükseltme geçmişini korur.
- Paket bitişine yaklaşma veya yükseltme fırsatı gibi tetikleyicileri üretir.
- E-posta gönderim süreçlerini güvenli biçimde başlatır.

Açık sınırlar:

- Bu yapı kategori ağacı değildir.
- Kategori, ilanın türünü belirler.
- Paket, ilanın görünürlük ve tanıtım seviyesini belirler.
- Kullanıcı ilk aldığı standart paketten daha üst pakete sonradan geçebilmelidir.
- Paket yükseltme fiyatı, kalan değer, süre ve kampanya hesapları daha sonra ayrıntılandırılacaktır.
- Payment, paket, bildirim veya e-posta tabloları bu aşamada tasarlanmayacaktır.
- Mevcut advert modeline şimdiden package_level gibi geçici alanlar eklenmeyecektir.
- Gelecekteki paket sistemi mevcut veri modelinin genişletilmesini imkânsız hale getirmemelidir.

## 11. API sözleşmesine sonraki etkiler

Henüz API üretilmeden, ileride şu grupların ayrıştırılması gerekir:

- Public işlemler
- Kimliği doğrulanmış normal kullanıcı işlemleri
- Kullanıcının kendi kaynakları üzerindeki işlemleri
- Admin/BO operasyonları
- Internal worker ve entegrasyon işlemleri
- Gelecekteki payment, paket ve bildirim işlemleri

Bu ayrımın URL yapısı, OpenAPI tag yapısı veya ayrı sözleşme dosyaları biçiminde nasıl uygulanacağı bu dokümanda kesinleştirilmez.

## 12. Reddedilen yaklaşımlar

- **Admin ekranlarını haradan-fe içinde tutmak:** Son kullanıcı deneyimini karmaşıklaştırır; yönetim operasyonlarını tek uygulamada toplama hedefine aykırıdır.
- **Aynı FE ekranında role göre çok sayıda admin butonu göstermek:** Ana yaklaşım değildir; admin BO’dadır.
- **Yalnız UI kontrolüne güvenmek:** Buton veya ekran gizlemek güvenlik değildir; yetki backend’de uygulanmalıdır.
- **Category/property yönetimini normal kullanıcı uygulamasına koymak:** Yönetim operasyonudur; BO’ya aittir.
- **Kategori ağacı yönetimini FE’ye koymak:** Ağaç yönetimi BO sorumluluğudur; FE yalnız okur.
- **BO’nun ihtiyaç duyduğu bir operasyonu yalnız FE üzerinden yapılabilir bırakmak:** Admin her gerekli işlemi BO’dan yapabilmelidir.
- **FE ve BO için farklı ve çelişen backend iş kuralları geliştirmek:** Aynı backend kuralları tutarlı uygulanmalıdır.
- **FE veya BO’nun PostgreSQL’e doğrudan bağlanması:** Kalıcı veri erişimi yalnız backend API üzerinden yapılır.
- **TJK verisinin normal kullanıcı tarafından değiştirilmesi:** Tek kaynak TJK’dır; kullanıcı override edemez.
- **İlan oluştururken canlı TJK servisini beklemek:** FE yerel (önceden senkronlanmış) horse araması kullanır.
- **Paket yükseltme işlemini kategori değişikliği olarak modellemek:** Kategori türü, paket görünürlük/tanıtım seviyesidir; karıştırılmaz.
- **Gelecekteki paket seviyesini şimdiden advert içine sabit alan olarak eklemek:** Payment/paket bu faz dışındadır; geçici alan eklenmez.

## 13. Sonraki adımlara etkisi

- Category/property saklama kararı BO yönetilebilirliğini dikkate almalıdır.
- Kategori hiyerarşisi veri modelinde desteklenmelidir.
- Category property metadata’sı FE’nin sade form üretmesini desteklemelidir.
- Fonksiyon listesi hazırlanırken public, normal kullanıcı ve admin fonksiyonları ayrılmalıdır.
- API sözleşmesinde FE ve BO tüketicileri açıkça düşünülmelidir.
- TJK fonksiyonları kullanıcı araması ve BO operasyonları olarak ayrılmalıdır.
- BO kodu; backend tablo yapısı, iş kuralları ve API sözleşmesi netleşmeden başlatılmamalıdır.
- haradan-fe yalnız public ve normal kullanıcı deneyimine göre planlanmalıdır.
- Gelecekteki paket ve bildirim sistemi mevcut fazdan ayrı tutulmalıdır.
