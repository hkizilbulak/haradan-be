# Kullanıcı Rolü Saklama Kararı

## 1. Bağlam ve gereksinimler

Sistemde yalnızca iki rol vardır: normal kullanıcı ve admin. Yeni rol eklenmesi beklenmemektedir. Admin sayısı çok az olacaktır. Şirket gereksiz rol ve yetki tabloları istememektedir; `roles`, `permissions`, `role_permissions` ve `user_roles` gibi tablolar otomatik olarak oluşturulmamalıdır. Güvenlik, veri bütünlüğü ve bakım kolaylığı korunmalıdır. Bu karar aşamasında users tablosu, migration veya backend kodu oluşturulmayacaktır.

## 2. Değerlendirilen seçenekler

### 2.1 PostgreSQL ENUM

PostgreSQL ENUM, kapalı bir değer kümesini veritabanı tip düzeyinde tanımlar. Bu projede küme `normal kullanıcı` ve `admin` ile sınırlıdır.

**Veri bütünlüğü.** Tip düzeyinde kısıt vardır; izin verilmeyen değerler yazılamaz. Bütünlük güçlüdür.

**Okunabilirlik.** Şema ve sorgularda rol alanı kapalı bir tip olarak okunur; anlamı nettir.

**Migration ve rol değişikliği.** Değer ekleme `ALTER TYPE` ile mümkündür ancak yeniden adlandırma, silme ve sıralama değişiklikleri zahmetlidir. Küme gerçekten sabit kalacaksa bu maliyet düşüktür; ileride küçük bir değişiklik gerekirse ENUM maliyeti yükselir.

**Go koduyla eşleştirme.** Sabit string veya özel tip ile eşleştirilir. Tarama ve doğrulama elle veya küçük bir yardımcı katmanda yapılır; karmaşıklık düşüktür.

**Avantajları.**
- Kapalı küme için güçlü tip güvenliği
- Okunabilir şema
- İki sabit rol senaryosuna doğal uyum

**Dezavantajları.**
- Rol kümesi değişince migration sürtünmesi
- VARCHAR + CHECK’e göre değişiklik esnekliği daha düşük
- Uygulama katmanında yine sabitlerle eşleştirme gerekir; ENUM tek başına yetkilendirme sağlamaz

### 2.2 VARCHAR + CHECK constraint

Rol alanı metin olarak tutulur; izin verilen değerler CHECK constraint ile sınırlanır.

**Veri bütünlüğü.** CHECK, yalnızca tanımlı değerlerin yazılmasına izin verir. ENUM kadar tip düzeyinde değildir ama pratikte yeterli bütünlük sağlar.

**Okunabilirlik.** Değerler düz metindir; şema ve sorgular açıktır. Constraint tanımı izinli kümeyi belgeler.

**Migration ve rol değişikliği.** Constraint düşürülüp yeniden eklenerek küme güncellenebilir. ENUM’a göre değişiklik genellikle daha basittir. Bu projede değişiklik beklenmese de bakım yolu daha düzdür.

**Go koduyla eşleştirme.** String sabitleri ile doğrudan eşleşir. Doğrulama uygulama ve veritabanı katmanında aynı değer kümesine bağlanabilir.

**Avantajları.**
- İki rol için yeterli bütünlük
- Go ile doğal eşleştirme
- Migration ve bakım kolaylığı
- Ek tablo gerektirmez

**Dezavantajları.**
- Tip düzeyinde ENUM kadar katı değildir
- Constraint ve uygulama sabitlerinin senkron tutulması gerekir
- Serbest metin alanı gibi durduğu için disiplinli kullanım şarttır

### 2.3 Ayrı role tablosu

Rol kayıtları ayrı bir tabloda tutulur; kullanıcı kaydı bu tabloya referans verir. Çoktan çoğa modellerde `user_roles` ile genişletilir.

**Bu proje için sağlayacağı fayda.** İki sabit rol ve az admin için anlamlı bir fayda sağlamaz. Dinamik rol yönetimi, çoklu rol veya zengin permission modeli yoktur.

**Getireceği tablo ve ilişki yükü.** En az bir role tablosu ve kullanıcıdan role referansı gerekir. Permission modeli eklenirse `permissions`, `role_permissions`, `user_roles` gibi ek tablolar ve join’ler gelir. Bu, şirketin istediği sade modele aykırıdır.

**Hangi koşullarda gerekli olabileceği.** Çoklu rol, kullanıcı başına birden fazla rol, dinamik permission atama, BO üzerinden rol/permission yönetimi veya üçüncü taraf yetki entegrasyonu gerektiğinde yeniden değerlendirilir.

**Neden şu an gereksiz olabileceği.** Rol kümesi kapalıdır, yeni rol beklenmemektedir, admin azdır ve gereksiz rol/yetki tabloları istenmemektedir.

**Avantajları.**
- Dinamik rol ve permission modellerine açılım
- Merkezi rol tanımı ve zengin metadata

**Dezavantajları.**
- Bu kapsam için aşırı model
- Ek tablo, join ve bakım maliyeti
- Yetkilendirme karmaşıklığını erken büyütür

## 3. Karşılaştırma

| Kriter | PostgreSQL ENUM | VARCHAR + CHECK | Ayrı role tablosu |
| --- | --- | --- | --- |
| Veri bütünlüğü | Çok yüksek | Yüksek | Yüksek (FK ile) |
| Basitlik | Yüksek | Çok yüksek | Düşük |
| Değişiklik esnekliği | Düşük | Orta-yüksek | Yüksek |
| Migration kolaylığı | Orta-düşük | Yüksek | Orta |
| Sorgu kolaylığı | Yüksek | Yüksek | Orta (join gerekir) |
| Bu projeye uygunluk | Yüksek | Çok yüksek | Düşük |

## 4. Önerilen karar

**Önerilen yaklaşım:** Rolü doğrudan kullanıcı kaydında tutmak; değer kümesini `VARCHAR + CHECK constraint` ile sınırlamak.

**Gerekçe.**
- Yalnız iki rol vardır; ayrı role tablosu fayda sağlamaz.
- Yeni rol beklenmediği için permission/RBAC şeması gereksizdir.
- Admin sayısı azdır; merkezi rol kataloğu veya çoktan çoğa ilişki gerekmez.
- Gereksiz tablo istenmemektedir; rol alanı kullanıcı kaydında yeterlidir.
- CHECK constraint ile geçersiz rol yazımı engellenir; yetkilendirme uygulama katmanında bu alana dayanır.
- ENUM’a göre migration ve bakım yolu daha sadedir; Go tarafında string sabitlerle eşleştirme doğaldır.

**Önerilen yaklaşımın uygulama ilkeleri (kavramsal).**
- Rol, doğrudan kullanıcı kaydında tutulur; ayrı rol veya yetki tablosu kullanılmaz.
- Kavramsal değerler: `user` (normal kullanıcı) ve `admin`.
- Varsayılan rol normal kullanıcı (`user`) olmalıdır. Yeni kayıtlar admin olmamalıdır.
- Admin rolü, kayıt veya profil güncelleme istekleriyle kullanıcı tarafından değiştirilememelidir. İstemciden gelen rol alanı yok sayılmalı veya reddedilmelidir.
- Rol değişikliği yalnızca güvenilir yönetim süreciyle yapılmalıdır (kontrollü admin işlemi veya kontrollü operasyonel süreç); sıradan API akışlarıyla yapılmamalıdır.

Bu dokümanda SQL, migration, tablo şeması veya Go kodu üretilmez.

## 5. Reddedilen yaklaşımlar

- **roles:** İki sabit rol için katalog tablosu gereksizdir; değer kümesi kullanıcı kaydındaki constraint ile karşılanır.
- **permissions:** İnce taneli yetki matrisi talep edilmemiştir; admin/normal ayrımı uygulama kurallarıyla yönetilebilir.
- **role_permissions:** Permission tablosu olmadığı için ara tablo da gereksizdir.
- **user_roles:** Kullanıcı başına tek rol vardır; çoktan çoğa ilişki yoktur.

Bu yapılar yeniden değerlendirilebilir; örneğin birden fazla eşzamanlı rol, dinamik permission yönetimi, BO’dan rol/permission tanımlama veya dış yetki sistemine uyum zorunlu hale gelirse.

## 6. Sonraki uygulama etkileri

Yalnızca kavramsal etkiler:

- **users tablosu:** Kullanıcı kaydında tek bir rol alanı bulunur; ayrı rol tabloları eklenmez.
- **kayıt süreci:** Yeni kullanıcı varsayılan olarak normal kullanıcı rolüyle oluşur; istemci rol seçemez.
- **authentication claim veya kullanıcı context’i:** Oturum/context içinde rol bilgisi taşınır; yetki kontrolleri buna dayanır.
- **admin middleware veya authorization kontrolü:** Admin gerektiren işlemler context’teki rolün `admin` olmasına göre korunur.
- **admin rolü değişikliği:** Kullanıcı self-service akışlarında yapılamaz; yalnızca güvenilir yönetim süreciyle yapılır.
- **test senaryoları:** Varsayılan rolün `user` olması, istemciden rol yükseltme denemelerinin başarısız olması, admin-only uçların `user` ile reddedilmesi ve `admin` ile geçmesi doğrulanmalıdır.

Bu aşamada teknik uygulama veya fonksiyon listesi üretilmez.
