# İl ve İlçe Veri Modeli Kararı

## 1. Bağlam ve gereksinimler

İlan konumu için yalnızca il ve ilçe yeterlidir; ayrıntılı adres tutulmayacaktır. İl ve ilçe verileri sık değişmez. İlçenin yanlış bir ile bağlanması engellenmelidir. İl ve ilçe alanlarında arama ve autocomplete yapılacaktır; kullanıcı `ist` yazdığında İstanbul gibi sonuçları bulabilmelidir. Türkçe karakter ve büyük/küçük harf farklılıkları aramayı bozmamalıdır. Filtreleme performansı korunmalıdır. Gereksiz dış servis bağımlılığı ve gereksiz tablo oluşturulmamalıdır. Bu karar aşamasında tablo, migration veya seed dosyası oluşturulmayacaktır.

## 2. Değerlendirilen seçenekler

### 2.1 Ayrı il ve ilçe tabloları

İl ve ilçe ayrı varlıklardır; ilçe kaydı bir ile bağlanır.

**Veri bütünlüğü.** İlçe–il ilişkisi yabancı anahtar ile güvence altına alınır. Yanlış ile bağlı ilçe yazımı engellenir.

**İl–ilçe ilişkisi.** İlişki açık ve tek yönlüdür: her ilçe zorunlu olarak tek bir ile aittir. İlan yalnızca seçilen ilçeye bağlanır; il bilgisi ilçenin bağlı olduğu il üzerinden elde edilir. İlan üzerinde ayrıca il referansı tutulmaz; böylece aynı bilginin tekrarı ve yanlış il–ilçe kombinasyonu riski oluşmaz.

**Sorgu ve filtreleme.** İl listesi ve bir ile bağlı ilçe listesi sade sorgularla yapılır. İl bazlı ilan filtreleme, ilçe ile il arasındaki ilişki üzerinden (join ile) yürütülür; join ihtiyacı öngörülebilir ve sınırlıdır.

**Autocomplete.** İl adında prefix arama ve seçilen ile göre ilçe arama ayrı uçlarda netleşir. Performans için alan ve indeks stratejisi sade kalır.

**Tablo sayısı.** İki referans tablosu yeterlidir; ek hiyerarşi veya konum seviyesi yoktur.

**Seed ve bakım yükü.** Türkiye il/ilçe kümesi sınırlı ve seyrek değişir. Seed ile kurulur; bakım düşük kalır.

**Avantajları.**
- Güçlü il–ilçe bütünlüğü
- Basit ve öngörülebilir sorgular
- Autocomplete ve filtrelemeye uygun
- Gereksiz genelleme yok

**Dezavantajları.**
- İki tablo yönetimi (sınırlı maliyet)
- Seed’in her iki tablo için tutarlı yüklenmesi gerekir

### 2.2 Tek hiyerarşik location tablosu

İl ve ilçe aynı tabloda tutulur; ilçe kaydı üst (parent) il kaydına bağlanır. Tip alanı ile seviye ayırt edilir.

**Parent-child bütünlüğü.** Parent ilişkisi ile hiyerarşi kurulabilir. Ancak tip kurallarının (ilçenin parent’ının il olması, ilin parent’sız olması) ayrıca doğrulanması gerekir.

**İl ve ilçe tiplerinin doğrulanması.** Aynı tabloda tip karışıklığı riski vardır. Constraint veya uygulama kuralları olmadan geçersiz ağaç oluşabilir.

**Sorgu karmaşıklığı.** Listeleme ve autocomplete tip filtresi + parent filtresi ister. Ayrı tablolara göre sorgular daha dolaylıdır.

**Gelecekte başka konum seviyeleri eklenmeyecek olması.** Mahalle, cadde vb. yok. Tek tablolu hiyerarşi genellemesi bu projede karşılıksızdır.

**Avantajları.**
- Tek fiziksel tablo
- İleride ek seviye gerekirse genişlemeye açık

**Dezavantajları.**
- Bu kapsam için aşırı genelleme
- Tip ve parent kurallarının ek doğrulama yükü
- Sorgu ve autocomplete’in daha karmaşık olması
- Ayrıntılı adres zaten kapsam dışı olduğu için genişleme değeri yok

### 2.3 Dış API kullanımı

İl/ilçe listesi ve autocomplete çalışma zamanında dış servisten alınır.

**Çalışma zamanı bağımlılığı.** İlan oluşturma, filtreleme ve autocomplete dış servise bağlanır.

**Servis kesintileri.** Dış servis kesintisi veya kota sorunları kritik kullanıcı akışını bozar.

**Veri tutarlılığı.** İlanın kaydettiği konum ile dış servisin güncel cevabı zamanla sapabilir. Tarihsel ilan bütünlüğü zayıflar.

**Autocomplete gecikmesi.** Ağ çağrısı gecikme ekler; yerel tabloya göre daha yavaştır.

**Cache ihtiyacı.** Performans ve kesinti toleransı için cache zorunlu hale gelir; karmaşıklık artar.

**Bakım yükü.** Sağlayıcı sözleşmesi, sürüm değişimi, rate limit ve cache invalidation bakımı gerekir.

**Avantajları.**
- Yerel seed güncelleme ihtiyacını azaltabilir
- Güncel resmi listelere bağlanma potansiyeli

**Dezavantajları.**
- Gereksiz dış bağımlılık
- Kesinti ve gecikme riski
- İlan referans bütünlüğünün zayıflaması
- Proje gereksinimiyle çelişmesi

### 2.4 Sabit verinin veritabanına seed edilmesi

Seed, tablo modelinden bağımsız bir veri yükleme yöntemidir. Ayrı il/ilçe tabloları veya tek location tablosu fark etmeksizin başlangıç verisi kontrollü şekilde yüklenebilir.

**İlk kurulum.** Ortam ayağa kalkarken bilinen il/ilçe kümesi yüklenir.

**Tekrarlanabilir ortam kurulumu.** Geliştirme, test ve staging aynı veri kümesiyle kurulabilir.

**Veri sürümleme.** Seed içeriği repo veya migration süreciyle versiyonlanır; değişiklikler gözden geçirilebilir.

**Güncelleme ihtiyacı.** İl/ilçe seyrek değişir. Nadir güncellemeler kontrollü seed/migration ile yapılır.

**Avantajları.**
- Dış API’siz çalışma
- Tekrarlanabilir kurulum
- Filtreleme ve autocomplete için yerel, hızlı veri
- İlan referanslarının kalıcı tutulması

**Dezavantajları.**
- Nadir resmi değişikliklerde manuel/kontrollü güncelleme gerekir
- Seed içeriğinin doğru ve tutarlı hazırlanması gerekir

## 3. Karşılaştırma

| Kriter | Ayrı il / ilçe tabloları | Tek hiyerarşik location | Dış API | Seed (yükleme yöntemi) |
| --- | --- | --- | --- | --- |
| Veri bütünlüğü | Yüksek | Orta-yüksek | Düşük-orta | Modele bağlı; yerel referansı güçlendirir |
| Model sadeliği | Yüksek | Orta | Yüksek (lokal model yok) | Modele ek yükleme kararı |
| Sorgu kolaylığı | Yüksek | Orta | Dış çağrıya bağlı | Yerel sorguları mümkün kılar |
| Autocomplete uygunluğu | Yüksek | Orta | Gecikme/cache riski | Yüksek (yerel veri) |
| Dış bağımlılık | Yok | Yok | Var | Yok |
| Bakım yükü | Düşük | Orta | Yüksek | Düşük-orta (seyrek güncelleme) |
| Bu projeye uygunluk | Çok yüksek | Düşük | Düşük | Çok yüksek (ayrı karar) |

Seed yaklaşımı tablo modelinden ayrı bir karardır: önce sade tablo modeli seçilir, başlangıç verisi seed veya migration ile yüklenir.

## 4. Önerilen karar

**Önerilen tablo modeli:** Ayrı il ve ilçe tabloları.

**Önerilen veri yükleme yöntemi:** Sabit verinin veritabanına seed / kontrollü migration süreciyle yüklenmesi.

**Karar ayrıntıları.**
- İl ve ilçe ayrı tablolarda tutulur; tek hiyerarşik location tablosu kullanılmaz.
- İlçenin bağlı olduğu il, ilçe kaydındaki zorunlu il referansı ile güvence altına alınır; geçersiz il bağlantısı kabul edilmez.
- Dış API çalışma zamanında kullanılmaz; autocomplete, listeleme ve filtreleme yerel veriden yapılır.
- Başlangıç il/ilçe verisi seed veya migration süreciyle yüklenir; ortamlar tekrarlanabilir olur.
- İl ve ilçe kayıtları günlük admin akışında serbestçe eklenip silinmemelidir. Küme referans veridir; değişiklikler kontrollü veri güncellemesiyle yapılır. Gerekirse aktif/pasif ile kullanım dışı bırakma tercih edilir.
- İlan yalnızca seçilen ilçeye bağlanır. İl, ilçenin zorunlu il referansı üzerinden türetilir; ilan üzerinde ayrıca il referansı tutulmaz. Ayrıntılı adres tutulmaz.
- Kullanıcı arayüzünde önce il, sonra ilçe seçilebilir; backend’e kalıcı konum referansı olarak yalnızca ilçe kimliği gönderilmesi önerilir. Backend, gönderilen ilçenin mevcut ve aktif olduğunu doğrular. Seçilen ilçenin seçilen ile ait olup olmadığını ilan kaydında ayrıca doğrulama ihtiyacı ortadan kalkar; çünkü il bilgisi tek kaynaktan (ilçe→il) gelir.
- İlan üzerinde ayrıca il referansı ancak ileride ölçülmüş ve belgelenmiş ciddi bir performans ihtiyacı oluşursa kontrollü denormalizasyon olarak yeniden değerlendirilebilir. Böyle bir denormalizasyon şu anda önerilmez.

Bu dokümanda SQL, kolon listesi, migration veya tablo şeması üretilmez.

## 5. Arama ve normalizasyon yaklaşımı

**Türkçe büyük/küçük harf davranışı.** Türkçede `I/ı` ve `İ/i` dönüşümleri İngilizce kurallarından farklıdır. Arama, istemci veya sunucu tarafında Türkçe’ye duyarlı bir case-folding ile yapılmalıdır.

**`İ`, `I`, `ı`, `i` farklılıkları.** Kullanıcı girdisi ve kayıtlı adlar bu karakterlerde sapma gösterebilir. Karşılaştırma ham metin eşitliğine bırakılmamalıdır.

**Aksan veya Türkçe karakter duyarsız arama.** Autocomplete senaryosunda `ist` → İstanbul beklenir. Bu, en azından Türkçe case-insensitive ve pratikte karakter-normalize edilmiş karşılaştırma gerektirir. Aşırı agresif aksan silme kuralları anlamı bozmayacak şekilde sınırlandırılmalıdır.

**Normalize edilmiş arama değeri gerekip gerekmediği.** Evet, kavramsal olarak önerilir. Görünen ad ile arama için kullanılan normalize değer ayrılmalıdır. Normalize değer, tutarlı case-folding ve arama amaçlı sadeleştirme sonrası üretilir; hem yazımda hesaplanır hem sorguda aynı kuralla üretilen girdiyle karşılaştırılır.

**`ist` ile İstanbul bulunması.** Prefix eşleşmesi normalize alan üzerinden yapılmalıdır (`ist` normalize girdi, `istanbul` normalize ad).

**Prefix arama.** Autocomplete için prefix arama birincil modeldir. Tam metin arama bu fazda gerekli değildir.

**İleride indeks ihtiyacı.** Normalize arama alanı üzerinde prefix aramayı destekleyen indeks ihtiyacı doğacaktır. Spesifik SQL fonksiyonu, extension veya indeks komutu bu dokümanda kesinleştirilmez; uygulama aşamasında ölçülerek seçilir.

**Öneri.** Görünen ad saklanır; arama, Türkçe-aware normalizasyonla üretilmiş ayrı bir arama değeri üzerinden prefix olarak yapılır. Böylece büyük/küçük harf ve `İ/I/ı/i` farkları autocomplete’i bozmaz; filtreleme yerel indekslenebilir alanla performanslı kalır.

## 6. Veri bütünlüğü kuralları

- Aynı il adı tekrar etmemelidir.
- Aynı il içinde aynı ilçe adı tekrar etmemelidir; farklı illerde aynı ilçe adı olabilir.
- İlçe zorunlu olarak geçerli bir ile bağlı olmalıdır.
- İlan oluşturma/güncellemede kalıcı konum referansı yalnızca ilçedir; gönderilen ilçe mevcut ve aktif olmalıdır. İlan üzerinde ayrıca il tutulmadığı için seçilen ilçenin seçilen ile ait olup olmadığını ilan kaydında ayrıca doğrulamaya gerek kalmaz.
- Fiziksel silme yerine aktif/pasif yaklaşımı tercih edilmelidir. Referans verinin sert silinmesi mevcut ilan bağlarını bozar. Kullanımdan kalkacak kayıtlar pasifleştirilir; yeni seçimlerde gösterilmez, geçmiş ilanlar çözümlenmeye devam eder.

## 7. Reddedilen yaklaşımlar

- **Tek hiyerarşik location tablosu:** Yalnız il/ilçe varken tip+parent genellemesi gereksiz karmaşıklık üretir; ek konum seviyesi yoktur.
- **Çalışma zamanı dış API:** Gereksiz bağımlılık, kesinti/gecikme riski ve ilan referans tutarsızlığı nedeniyle uygun değildir.
- **Adres / mahalle / koordinat genişlemesi:** Kapsam dışı; ayrıntılı adres tutulmayacaktır.
- **Admin’in günlük serbest CRUD ile referans kümesini değiştirmesi:** Seyrek değişen resmi küme kontrollü seed/güncelleme ile yönetilmelidir.
- **Normalize arama olmadan ham string karşılaştırma:** Türkçe karakter ve case farkları `ist` → İstanbul senaryosunu bozar.

## 8. Sonraki uygulama etkileri

Yalnızca kavramsal etkiler:

- **Veritabanı tabloları:** Ayrı il ve ilçe referans tabloları; ilan yalnızca ilçeye bağlanır, il ilçe üzerinden elde edilir.
- **Başlangıç verisi:** Seed veya migration ile yüklenen sabit il/ilçe kümesi.
- **İlan oluşturma validasyonu:** Kalıcı konum olarak yalnızca ilçe kimliği; ilçe mevcut ve aktif olmalı; ayrıntılı adres yok. UI’de il seçimi yardımcı adımdır, ilan kaydında saklanmaz.
- **İl autocomplete:** Normalize arama değeri üzerinde prefix arama.
- **İlçe listeleme:** Seçilen ile bağlı aktif ilçeler; isteğe bağlı autocomplete aynı il içinde.
- **İlan filtreleme:** İlçe referansı ve ilçe→il ilişkisi üzerinden yerel filtreleme; il filtresi join ile uygulanır.
- **Test senaryoları:** Seed tutarlılığı, mükerrer ad engeli, geçersiz/pasif ilçe reddi, ilan kaydında yalnızca ilçe referansı, il bilgisinin ilçeden türetilmesi, `ist` → İstanbul, Türkçe case farkları, pasif kaydın yeni seçimde görünmemesi.

Bu aşamada tablo, migration, API, fonksiyon listesi veya backend kodu üretilmez.
