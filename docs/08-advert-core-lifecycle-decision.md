# Advert Çekirdek Modeli ve Yaşam Döngüsü Kararı

## 1. Problem tanımı

Aşağıdaki kavramlar birbirinden ayrılır:

- **Advert çekirdek kaydı:** İlanın platformdaki ana domain kaydıdır.
- **Advert içeriği:** Başlık, açıklama ve sabit çekirdek alanlar.
- **Category property değerleri:** Yalnız kullanıcı girdisi kaynaklı dinamik değerler (advert JSONB).
- **Horse ilişkisi:** Seçilen horse’a referans; horse verisinin kopyası değil.
- **Konum ilişkisi:** Yalnız ilçe referansı; il ilçeden türetilir.
- **Medya ilişkisi:** Görseller ayrı domain; properties JSONB içinde tutulmaz.
- **Moderasyon:** Admin inceleme, onay, ret, değişiklik isteme, askıya alma.
- **Yayın yaşam döngüsü:** Public görünürlük ve kullanıcı tarafı yaşam olayları (satıldı, arşiv).
- **Gelecekteki paket/görünürlük hakkı:** Advert status’undan ayrı entitlement sorumluluğu.

Açık ilkeler:

- Advert bir horse kaydı veya kategori tanımı değildir.
- Property tanımı ile gerçek property değeri aynı şey değildir.
- Paket seviyesi kategori veya advert status değildir.
- Gelecekteki paket hakkı advert’e bağlı ayrı sorumluluk olarak ele alınmalıdır; çekirdeğe geçici alanlarla gömülmez.

Bu dokümanda tablo veya kolon tasarımı yapılmaz.

## 2. Advert çekirdeğinin sorumlulukları

Advert çekirdeğinin kavramsal olarak taşıdığı bilgiler:

- Sahiplik
- Kategori
- Horse bağlantısı (kategori gerektiriyorsa)
- İlçe bağlantısı
- Başlık
- Açıklama
- Dinamik kullanıcı property değerleri (JSONB)
- Yaşam döngüsü / moderasyon durumu
- Yayınlanma zamanı
- Oluşturulma ve güncellenme zamanı

Advert çekirdeğine doğrudan gömülmemesi gerekenler:

- Horse/TJK alan kopyaları
- İl adı / il referansı
- Görsel dosya metadata’sı
- Favori kullanıcı listeleri
- Paket tanımları
- Payment kayıtları
- E-posta gönderim geçmişi
- Worker job payload’ları
- Kontrollü TJK detail JSONB

**Fiyat ve para birimi:** Sabit advert alanı adayıdır; kesin model ürün kararı bekler. Bu dokümanda kesin kolon kararı verilmez.

## 3. Advert ilişkileri ve zorunlulukları

### Advert–owner

- Her advert’in tek sahibi vardır.
- Sahip, authentication context’teki kullanıcıdan sistem tarafından belirlenir.
- Request içinden gönderilen owner bilgisine güvenilmez.
- Normal kullanıcı sahipliği değiştiremez.
- Admin sahiplik transferi ayrı ürün kararıdır; ilk fazda varsayılmaz.
- Kullanıcı silinse/pasifleşse bile advert geçmişi bozulmamalıdır (soft/pasif hesap davranışı ürün kararı).

### Advert–category

- Her advert geçerli bir kategoriye bağlı olmalıdır (moderasyona gönderme seviyesinde zorunlu).
- Kategori aktif olmalıdır.
- Kategorinin ilan verilebilir olup olduğu kontrol edilmelidir.
- **Üst kategoriye doğrudan ilan:** Ürün kararı açıktır.
- **İlk faz önerisi:** Yalnız ilan verilebilir / yaprak kategoriye bağlama tercih edilir.
- Kategori pasifleşince mevcut yayınlanmış advert’ler otomatik silinmez; public davranış ürün/operasyon kuralıyla belirlenir (açık karar).

### Advert–horse

- Horse bütün kategoriler için zorunlu değildir; kategori metadata’sı gereksinimi belirler.
- Horse gerektiren kategoride geçerli horse zorunludur (moderasyona gönderme).
- Horse gerektirmeyen kategoride horse gönderimi varsayılan olarak reddedilmelidir (tutarsızlığı önlemek için).
- Advert horse verisini kopyalamaz; güncel horse kaydından okur.
- Horse sync sorunu yaşasa bile advert ilişkisi korunur; otomatik yayından kaldırma/silme yapılmaz.

### Advert–district

- Advert yalnız ilçeye bağlanır; il ilçeden türetilir.
- İlçe aktif ve geçerli olmalıdır (moderasyona gönderme).
- İl ve ilçe ayrı ayrı request edilerek tutarsızlık üretilmemelidir; kalıcı referans yalnız ilçedir.

## 4. Tek status makinesi ile ayrık durum modellerinin karşılaştırılması

### 4.1 Tek advert status alanı

Moderasyon ve yayın yaşam döngüsü tek durum makinesinde yönetilir.

### 4.2 Ayrı moderasyon ve yayın durumları

Moderasyon sonucu ile public görünürlük ayrı kavramlar olur.

| Kriter | Tek status | Ayrık durumlar |
| --- | --- | --- |
| İlk faz sadeliği | Yüksek | Orta-düşük |
| Geçersiz kombinasyon riski | Düşük (tek değer) | Yüksek |
| Moderasyon | Yeterli | Esnek ama karmaşık |
| Public görünürlük | Status’tan türetilir | Ayrı alan |
| Askıya alma / satıldı / arşiv | Tek makinede modellenebilir | Kombinasyon patlaması riski |
| Gelecekte paket | Status’a karıştırmadan ayrı entitlement ile uyumlu | Yine ayrı entitlement gerekir |
| State explosion | Kontrollü tutulabilir | Yüksek risk |
| Bakım | Daha kolay | Daha zor |
| Bu projeye uygunluk | Yüksek | Düşük (ilk faz) |

**Öneri:** İlk fazda tek advert status makinesi. Gelecekte paket hakkı ayrı entitlement olarak eklenir; status makinesi ödeme durumlarına dönüştürülmez.

## 5. Değerlendirilecek advert durumları

| Durum | Anlam | Public? | Kullanıcı düzenler? | Kullanıcı işlemleri | Admin işlemleri | Terminal? | İlk faz? |
| --- | --- | --- | --- | --- | --- | --- | --- |
| DRAFT | Taslak | Hayır | Evet | Kaydet, gönder, sil (ürün) | — | Hayır | Evet |
| PENDING_REVIEW | Moderasyon bekliyor | Hayır | Hayır | Görüntüle | Onayla, ret, değişiklik iste | Hayır | Evet |
| CHANGES_REQUESTED | Düzeltme isteniyor | Hayır | Evet | Düzenle, tekrar gönder | — | Hayır | Evet |
| APPROVED | Onaylandı ama henüz yayın değil | Hayır | Hayır | — | — | Hayır | Hayır (payment yokken gereksiz) |
| PUBLISHED | Yayında | Evet | Hayır (ilk faz) | Satıldı, arşiv/çek | Askıya al | Hayır | Evet |
| REJECTED | Reddedildi | Hayır | Hayır (yeniden başvuru ürün kararı) | Görüntüle | — | Kısmen | Evet |
| SUSPENDED | Admin askıya aldı | Hayır | Hayır | Görüntüle | Yeniden yayına al | Hayır | Evet |
| SOLD | Kullanıcı satıldı işaretledi (finansal kayıt değil) | Hayır (ilk faz önerisi) | Hayır | — | — | Kısmen | Evet |
| ARCHIVED | Yayından çekildi / arşiv | Hayır | Hayır | — | — | Kısmen | Evet |
| EXPIRED | Süre doldu | Hayır | — | — | — | — | Hayır (paket/süre sonrası) |

Özel değerlendirmeler:

- **APPROVED vs PUBLISHED:** Payment olmadığı için ayrı `APPROVED` ilk fazda gerekmez; onay doğrudan `PUBLISHED` olur. Payment/entitlement gelirse bekleme ayrı entitlement kontrolüyle modellenebilir.
- **REJECTED vs CHANGES_REQUESTED:** Değişiklik isteme düzeltme şansı verir; ret daha kesin sonuçtur (yeniden başvuru ürün kararı).
- **SOLD:** Finansal satış kaydı değildir; kullanıcı işaretidir.
- **EXPIRED:** İlk fazda gerekmez; paket/yayın süresiyle gelir.
- **ARCHIVED vs yayından çekme:** İlk fazda aynı `ARCHIVED` durumu yeterlidir.
- **SUSPENDED:** Yalnız admin işlemi.

## 6. Önerilen durum makinesi

**İlk faz durumları:** `DRAFT`, `PENDING_REVIEW`, `CHANGES_REQUESTED`, `PUBLISHED`, `REJECTED`, `SUSPENDED`, `SOLD`, `ARCHIVED`

### Aktörler

- Normal kullanıcı / advert sahibi
- Admin
- Sistem/worker (ilk fazda sınırlı; gelecekte expiry vb.)

### Kritik geçişler

| Kaynak | Hedef | Aktör | Validasyon | Gerekçe | Public etkisi |
| --- | --- | --- | --- | --- | --- |
| (yeni) | DRAFT | Kullanıcı | Taslak kuralları | Hayır | Görünmez |
| DRAFT | PENDING_REVIEW | Kullanıcı | Tam validasyon | Hayır | Görünmez |
| PENDING_REVIEW | CHANGES_REQUESTED | Admin | — | Evet (açıklama) | Görünmez |
| PENDING_REVIEW | REJECTED | Admin | — | Evet | Görünmez |
| PENDING_REVIEW | PUBLISHED | Admin | Ön koşullar hâlâ geçerli | Hayır | Görünür |
| CHANGES_REQUESTED | PENDING_REVIEW | Kullanıcı | Tam validasyon | Hayır | Görünmez |
| PUBLISHED | SUSPENDED | Admin | — | Evet | Gizlenir |
| SUSPENDED | PUBLISHED | Admin | Ön koşullar | Opsiyonel | Görünür |
| PUBLISHED | SOLD | Sahip | — | Hayır | Gizlenir (ilk faz) |
| PUBLISHED | ARCHIVED | Sahip | — | Hayır | Gizlenir |
| SUSPENDED | ARCHIVED | Admin/sahip (ürün) | — | Opsiyonel | Görünmez kalır |

Akış özeti:

- Yeni advert → DRAFT
- Taslak moderasyona gönderilir → PENDING_REVIEW
- Admin değişiklik ister → CHANGES_REQUESTED; kullanıcı düzenleyip tekrar gönderir
- Admin onaylar → PUBLISHED (payment yok)
- Admin reddeder → REJECTED
- Yayındaki advert askıya alınır → SUSPENDED; admin yeniden yayına alabilir
- Kullanıcı satıldı işaretler → SOLD
- Kullanıcı yayından çeker → ARCHIVED
- Gelecekte expiry / yenileme / paket yükseltme ayrı entitlement ile ele alınır; status’a payment gömülmez

## 7. Düzenleme kuralları

| Durum | İçerik düzenleme (ilk faz) |
| --- | --- |
| DRAFT | Evet (kategori değişimi dahil; property temizliği + uyarı) |
| PENDING_REVIEW | Hayır |
| CHANGES_REQUESTED | Evet (kategori değişimi hayır — yalnız DRAFT) |
| PUBLISHED | Hayır (çekirdek içerik) |
| SUSPENDED | Hayır |
| SOLD | Hayır |
| ARCHIVED | Hayır |
| REJECTED | Hayır (yeniden başvuru ürün kararı) |

Riskler ve politika:

- Moderatör incelerken içerik değişmesin → PENDING_REVIEW kilitli.
- Yayında moderasyonsuz tam değişiklik olmasın → PUBLISHED kilitli.
- Kategori yalnız DRAFT’ta değişir (kesin karar).
- Horse değişimi CHANGES_REQUESTED/DRAFT’ta mümkündür; PUBLISHED’da yok.
- Görsel sırası gibi düşük riskli işlemler medya kararında ayrıca ele alınır; bu dokümanda yayın içinde serbest çekirdek içerik düzenlemesi önerilmez.
- İlk fazda ağır revision/draft-copy sistemi kurulmaz.

## 8. Moderasyon kararı ve gerekçe

- **Onay:** PENDING_REVIEW → PUBLISHED; gerekçe zorunlu değil.
- **Değişiklik isteme:** PENDING_REVIEW → CHANGES_REQUESTED; kullanıcıya gösterilecek açıklama zorunlu.
- **Ret:** PENDING_REVIEW → REJECTED; gerekçe zorunlu.
- **Askıya alma:** PUBLISHED → SUSPENDED; gerekçe zorunlu; public kalkar.
- **Askıdan çıkarma:** SUSPENDED → PUBLISHED; ön koşullar kontrol edilir.
- **Arşivleme:** Sahip veya admin (ürün); public kalkar.

İlkeler:

- Değişiklik isteme düzeltme şansı verir; ret daha kesindir.
- Admin kararı actor + zaman ile izlenir.
- BO işlemleri backend’de yeniden yetkilendirilir.
- Admin’in kullanıcı içeriğini sessizce değiştirmesi ana yaklaşım değildir.
- BO’nun property değerlerini değiştirip değiştiremeyeceği açık ürün kararıdır.

## 9. Status history ve audit

Status history gerekir:

- Önceki durum
- Yeni durum
- Aktör
- Zaman
- Gerekçe (varsa)
- Sistem işlemi bayrağı (kavramsal)

İlkeler:

- Mevcut status “şimdi”; history “geçmiş olaylar”dır.
- History sonradan değiştirilmez.
- İlk oluşturma (→ DRAFT) history’ye yazılır.
- Başarısız geçiş denemeleri history’ye yazılmaz; hata döner.
- İçerik değişiklik audit’i status history değildir; ilk fazda ayrı ağır audit zorunlu değildir.
- Admin kararları açıklanabilir olmalıdır.

## 10. Public görünürlük kuralları

**İlk faz:** Yalnız `PUBLISHED` public arama ve detayda görünür.

- SUSPENDED, SOLD, ARCHIVED, REJECTED, CHANGES_REQUESTED, PENDING_REVIEW, DRAFT public değildir.
- Pasif kullanıcı / pasif kategori / pasif district davranışı ürün kararıdır; otomatik fiziksel silme yok.
- Horse sync hatası advert’i otomatik yayından kaldırmaz.
- Public URL davranışı (404/410/mesaj) ürün/FE kararıdır; bu dokümanda URL tasarlanmaz.

## 11. Silme, arşivleme ve veri saklama

- **Yayınlanmış / moderasyona girmiş advert:** Fiziksel silme yok; arşiv veya terminal durumlarla korunur (history, medya, favori ilişkileri bozulmasın).
- **Taslak:** Kullanıcı kalıcı silebilir (ürün onayıyla); history az ise soft delete veya fiziksel silme kabul edilebilir. Öneri: taslak için kullanıcı silme izni; soft delete tercih (geri dönüş/operasyon).
- **Arşivleme ≠ fiziksel silme:** Public kalkar, kayıt kalır.
- Hesap kapanması ve hukuki saklama süreleri uydurulmaz; açık ürün/uyumluluk kararıdır.

**İlk faz önerisi:** Moderasyona girmiş veya yayınlanmış advert fiziksel silinmez. Taslak silinebilir (soft delete). Arşiv/sold/suspended geçmişi korur.

## 12. Sahiplik ve yetkilendirme

- Owner authentication context’ten gelir.
- Özel kullanıcı işlemlerinde sahiplik kontrolü zorunludur.
- Public published görüntüleme sahiplik gerektirmez.
- Admin işlemleri rol kontrolü gerektirir; admin “her kullanıcı işlemini sahiplenmez”, ayrı BO operasyonları kullanır.
- Request ile owner/admin rolü kazanılamaz.
- Sahiplik transferi ilk fazda yok.
- Pasif hesap advert public davranışı ürün kararıdır.
- Yetkisiz geçişler reddedilir.

## 13. Eş zamanlı güncelleme ve moderasyon yarışı

Riskler: kullanıcı düzenlerken admin kararı; iki admin çakışması; iki cihaz; stale FE verisi; çift geçiş; retry ile duplicate history.

**İlk faz önerisi:**

- Optimistic concurrency / version kontrolü (kavramsal).
- Geçişlerde beklenen mevcut status doğrulanır.
- Status + history aynı transaction içinde güncellenir.
- Başarılı geçiş idempotent hale getirilebilir; stale istek açık hata ile reddedilir.
- Kesin kolon/SQL/idempotency key tasarımı yapılmaz.

## 14. Taslak ve moderasyona gönderme davranışı

**Taslak:**

- Minimum başlangıç: en azından sahip + boş/kısmi taslak kaydı; kategori seçilmeden açılıp açılamayacağı ürün kararı (öneri: kategori erken seçilsin ama zorunluluk gönderimde).
- Kısmi başlık/açıklama/property JSONB izinli (tip güvenli; bilinmeyen anahtar yok).
- Horse/district sonra seçilebilir.
- Otomatik taslak kaydetme FE tercihidir; backend kısmi kaydı destekler.
- Taslak public değildir.

**Moderasyona gönderme:**

- Kategori geçerli, aktif, ilan verilebilir
- Horse gereksinimi
- District
- Başlık/açıklama (minimum kurallar ürün)
- Dinamik property tam validasyonu
- Medya zorunluluğu ürün kararı
- Fiyat zorunluluğu ürün kararı
- Status → PENDING_REVIEW + history

## 15. Gelecekteki paket ve yayın hakkı etkisi

Ayrılacak kavramlar: moderasyon durumu, public yayın durumu, paket/görünürlük hakkı, payment sonucu, yükseltme/yenileme, bitiş, kampanya/e-posta.

İlkeler:

- Kategori ≠ paket; paket seviyesi ≠ status; payment ≠ moderasyon onayı.
- Entitlement ayrı kontrol edilir; status makinesi payment’a dönüşmez.
- Paket yükseltme kategori/içeriği değiştirmez.
- `EXPIRED` ile entitlement bitişi zorunlu aynı kavram değildir.
- Bu fazda payment/entitlement tablosu tasarlanmaz; çekirdek model ayrı entitlement ilişkisini engellemez.

**Payment olmayan ilk faz:** Admin onayı → doğrudan `PUBLISHED`.

## 16. FE, BO ve backend etkisi

### FE

- Kullanıcı durumunu anlaşılır metinlerle gösterir.
- Yalnız izin verilen işlemleri sunar.
- DRAFT ve CHANGES_REQUESTED düzenlemeyi destekler.
- PENDING_REVIEW’da kontrolsüz düzenleme sunmaz.
- Admin/moderasyon ekranı içermez.
- Backend kurallarının yerine geçmez.
- Public’te yalnız backend’in görünür kabul ettiği advert’leri gösterir.

### BO

- Moderasyon kuyruğu ve detay.
- İzin verilen admin geçişleri; gerekçeli işlemlerde gerekçe.
- Stale karar hatalarını gösterir.
- Status history görüntüler.
- Sessiz içerik değiştirmeyi ana yaklaşım yapmaz.
- Ekran tasarımı bu dokümanda yok.

### Backend

- Durum makinesinin tek güvenilir uygulayıcısıdır.
- Sahiplik ve admin rolünü kontrol eder.
- Geçiş ön koşullarını doğrular; status+history tutarlı günceller.
- Public görünürlüğü merkezi belirler.
- Taslak/tam validasyonu ayırır; concurrent/stale engeller.
- İstemcinin gönderdiği status’a körü körüne güvenmez.
- Paket hakkını status içine karıştırmaz.

## 17. Karşılaştırma tabloları

### 17.1 Tek status ve ayrık durum modeli

(Bkz. bölüm 4 tablosu.)

### 17.2 Durum adayları

(Bkz. bölüm 5 tablosu.)

### 17.3 Kritik durum geçişleri

(Bkz. bölüm 6 tablosu.)

## 18. Önerilen karar

**Önerilen model:** Tek advert status makinesi; ilk faz durumları `DRAFT`, `PENDING_REVIEW`, `CHANGES_REQUESTED`, `PUBLISHED`, `REJECTED`, `SUSPENDED`, `SOLD`, `ARCHIVED`.

Soru cevapları:

1. **Tek status makinesi mi?** Evet.
2. **Ayrı moderation/publication status?** İlk fazda hayır.
3. **İlk faz listesi?** Yukarıdaki 8 durum.
4. **APPROVED ayrı mı?** Hayır.
5. **Onay → PUBLISHED mi?** Evet (payment yok).
6. **CHANGES_REQUESTED?** Evet.
7. **REJECTED?** Evet; değişiklik istemeden daha kesin.
8. **SUSPENDED yalnız admin mi?** Evet.
9. **Askıdan yayına dönüş?** Evet (admin).
10. **SOLD finansal mı?** Hayır.
11. **ARCHIVED?** Evet.
12. **EXPIRED ilk faz?** Hayır.
13. **Kullanıcı ne zaman düzenler?** DRAFT, CHANGES_REQUESTED.
14. **Yayındaki içerik doğrudan düzenlenir mi?** Hayır (ilk faz).
15. **Kategori ne zaman değişir?** Yalnız DRAFT.
16. **Moderasyona gönderme?** DRAFT veya CHANGES_REQUESTED → PENDING_REVIEW (tam validasyon).
17. **Gerekçe?** Ret, değişiklik isteme, askıya almada zorunlu.
18. **Status history?** Evet.
19. **İlk oluşturma history’de?** Evet.
20. **Aynı transaction?** Evet.
21. **Public?** Yalnız PUBLISHED.
22. **Fiziksel silme?** Moderasyona girmiş/yayında hayır; taslak soft delete.
23. **Taslak silme?** Kullanıcı soft delete (ürün onayıyla).
24. **Owner request’ten?** Hayır.
25. **Optimistic concurrency?** Evet (kavramsal).
26. **Paket status içine?** Hayır.
27. **Payment yokken yayın?** Onay → PUBLISHED.
28. **Horse sync hatası yayından kaldırmalı mı?** Hayır.
29. **Pasif kategori/district?** Otomatik silme yok; ürün kararı.
30. **BO içerik değiştirir mi?** Ana yaklaşım değil; ayrı ürün kararı.

**Gerekçe:** Sade MVP, güvenli moderasyon, FE–BO ayrımı, sahiplik, açıklanabilir history, concurrency, gelecekte ayrı entitlement; gereksiz state/payment gömme yok.

## 19. Reddedilen yaklaşımlar

- Status’u yalnız FE/BO’ya bırakmak
- Kullanıcının istediği status’u doğrudan yazmak
- Başkasının advert’ini değiştirmek
- Moderasyondayken kontrolsüz içerik değişikliği
- Yayında yeniden moderasyonsuz tam değişiklik
- Boolean ormanı ile durum yönetimi
- Payment/paketi advert status yapmak
- Status history tutmamak
- Gerekçesiz ret/askı
- History’yi değiştirilebilir yapmak
- Yayınlanmış/moderasyonlu advert’i fiziksel silmek
- Horse sync hatasında advert silmek
- İl+ilçe çift referansı
- Horse snapshot kopyası
- İlk fazda ağır revision
- Her gelecek ihtimal için şimdiden çok status (`APPROVED`, `EXPIRED` vb.)

## 20. Açık kalan ürün kararları

- Başlangıç kategori ağacı
- Üst kategoriye doğrudan ilan
- Minimum başlık/açıklama kuralları
- En az bir görsel zorunluluğu
- Fiyat zorunluluğu / para birimi
- Yayındaki advert düzenleme beklentisi (gelecek)
- Ret sonrası yeniden başvuru
- Satılmış advert’in public gösterimi (şu an hayır önerildi)
- Arşiv URL davranışı
- Pasif kullanıcı / kategori / district advert davranışı
- BO içerik düzenleme yetkisi
- Yayın süresi / expiry
- Paket/entitlement yayın koşulları
- Veri saklama süresi
- Taslak kalıcı silme beklentisi

## 21. Sonraki adım

Bu karar kabul edilirse sonraki teknik karar medya ve banner modelidir:

- Backblaze B2
- Asset metadata
- Advert–media ilişkisi
- Görsel sıralama
- Kapak görseli
- Search/HOMEPAGE/DETAIL varyantları
- TinyPNG adapter
- Upload validasyonu
- Banner placement
- Banner yaşam döngüsü
- Worker işleme akışı

Bu dokümanda medya/banner tablosu, API veya worker kodu oluşturulmaz.
