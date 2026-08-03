# Kullanıcı, Authentication ve Session Modeli Kararı

## 1. Problem tanımı

Aşağıdaki kavramlar birbirinden ayrılır:

- **User domain kaydı:** Platformdaki kişi kimliğinin ana kaydı.
- **Kullanıcı profili:** Ad, soyad gibi görünen/kişisel alanlar.
- **Credential:** Giriş veya doğrulama için kullanılan gizli bilgi (parola, reset/doğrulama credential’ı).
- **Parola hash’i:** Parolanın geri çözülemez saklanmış hali; parola değildir.
- **Authentication:** Kullanıcının kim olduğunu belirleme.
- **Authorization:** Belirli işlemi yapıp yapamayacağını belirleme.
- **Role:** `user` veya `admin`; kullanıcı kaydında tutulur.
- **Session:** Oturum durumu; user kaydından ayrı sorumluluk.
- **Access token:** Kısa ömürlü erişim kimlik bilgisi.
- **Refresh credential/token:** Daha uzun ömürlü, server-side takip edilen yenileme kimliği.
- **E-posta doğrulama:** E-posta sahipliğinin kanıtlanması.
- **Parola sıfırlama:** Unutulan parola için güvenli yenileme akışı.
- **Hesap durumu:** Kalıcı hesap kullanılabilirliği (`ACTIVE`, `DISABLED`, `CLOSED`).
- **Login attempt / güvenlik olayı:** Giriş denemesi, replay, iptal gibi güvenlik izleri.

Açık ilkeler:

- Authentication kimliği; authorization yetkiyi belirler.
- Role ile resource ownership aynı şey değildir.
- Session ile user kaydı aynı sorumluluk değildir.
- Parolanın kendisi hiçbir zaman kalıcı tutulmaz.
- Access token, uzun ömürlü oturumun tek kaynağı olmamalıdır.
- FE ve BO istemci davranışları farklı olsa bile backend güvenlik kuralları ortaktır.

Bu dokümanda tablo veya kolon tasarımı yapılmaz.

## 2. User çekirdek sorumlulukları

User domain kaydının kavramsal olarak yönetmesi gerekenler:

- Teknik kullanıcı kimliği
- E-posta (giriş kimliği)
- Ad
- Soyad
- Telefon ihtiyacı (ürün)
- Rol (`user` / `admin`)
- Hesap durumu (`ACTIVE`, `DISABLED`, `CLOSED`)
- E-posta doğrulama durumu (ayrı nitelik; kalıcı status içine gömülmez)
- Oluşturulma ve güncellenme zamanı

Değerlendirme ve ilk faz önerisi:

- **Giriş kimliği:** E-posta.
- **Telefon:** İlk fazda giriş kimliği olmamalı; zorunluluk ürün kararı (öneri: opsiyonel veya sonraya bırakılabilir).
- **Kullanıcı adı:** Gerekli değil; e-posta yeterlidir.
- **Görünen ad vs giriş kimliği:** Ad/soyad profil; e-posta giriş kimliği.
- **Admin hesapları:** Aynı user domain’inde; role ile ayrılır.
- **Profil vs güvenlik alanları:** Ad/soyad profil güncellemesi; e-posta değişikliği, parola ve role ayrı güvenli akışlardır.
- **E-posta değişikliği:** Sıradan profil güncellemesi değildir; yeniden doğrulama gerekir.
- **Role:** Sıradan profil alanı değildir; public/kullanıcı isteğiyle değişmez.
- **Hesap durumu:** `DISABLED` (admin/güvenlik) ile `CLOSED` (kullanıcı kapatma) ayrı iş olaylarıdır.

Kesin alan listesi veya kolon adı oluşturulmaz.

## 3. E-posta normalizasyonu ve benzersizlik

Değerlendirme:

- Baş/son boşluklar temizlenir.
- Domain kısmı case-insensitive karşılaştırılır.
- Local-part için de karşılaştırma değeri düşük harfe indirgenebilir (basit, tutarlı yaklaşım).
- Kullanıcıya gösterilen orijinal yazım ile normalize karşılaştırma değeri ayrılabilir.
- Aynı normalize e-posta ile ikinci hesap engellenir.
- E-posta değişikliğinde benzersizlik yeniden kontrol edilir.
- Unicode/internationalized adresler: aşırı karmaşık normalizasyon ilk fazda zorunlu değildir; geçerli format + normalize karşılaştırma yeterlidir.

**Reddedilen:** Gmail nokta veya `+tag` gibi sağlayıcıya özel dönüşümler uygulanmaz.

**İlk faz önerisi:** Trim + case-insensitive normalize karşılaştırma; sağlayıcıya özel akıllı dönüşüm yok. Kesin constraint/SQL yazılmaz; benzersizlik DB seviyesinde de korunmalıdır.

## 4. Hesap durum modeli

| Kalıcı durum | Anlam | İlk faz |
| --- | --- | --- |
| ACTIVE | Hesap normal biçimde kullanılabilir | Evet |
| DISABLED | Admin veya güvenlik operasyonu tarafından kullanıma kapatılmıştır; yetkili admin süreciyle tekrar açılabilir | Evet |
| CLOSED | Kullanıcının hesap kapatma talebi uygulanmıştır; yeniden açma ayrı ürün kararıdır | Evet |

Kalıcı status olmayanlar:

| Kavram | Konum |
| --- | --- |
| E-posta doğrulama | Ayrı nitelik; kalıcı status’a gömülmez |
| Geçici brute-force lock | Ayrı login güvenlik durumu; kalıcı user status değildir |
| Session iptali | Session sorumluluğu; user status değildir |
| PENDING_VERIFICATION | Kalıcı status olarak kullanılmaz |
| Fiziksel silme | İlk fazda yok |

Açık ilkeler:

- `DISABLED` ile `CLOSED` aynı iş olayı değildir.
- `CLOSED` fiziksel kullanıcı silme anlamına gelmez.
- Advert geçmişi ve operasyonel ilişkiler korunur.
- Kişisel veri anonimleştirme ve hukuki silme ayrı KVKK/retention kararıdır.
- Advert status’u user status’una karıştırılmaz.
- Anonim public advert arama/detay account status’a bağlı değildir.
- Authentication gerektiren bütün korumalı işlemler yalnız `ACTIVE` kullanıcı tarafından yapılır.

Her şeyi tek status’a yığmak (doğrulama + lock + disable + close) reddedilir.

## 5. Parola saklama kararı

| Seçenek | Uygunluk |
| --- | --- |
| Düz metin | Red |
| Geri çözülebilir encryption | Red |
| Hızlı genel amaçlı hash | Red |
| Adaptif parola hash (bcrypt / Argon2id) | Evet |

Açık kurallar:

- Düz metin / geri çözülebilir parola tutulmaz.
- Parola loglanmaz; API response’ta dönmez; hash istemciye gönderilmez.
- Her parola için salt gerekir (algoritmanın doğal salt’ı).
- Cost parametresi zamanla yükseltilebilir; login’de eski parametreyle hash rehash edilebilir.
- Pepper opsiyonel; kullanılırsa yalnız secret ortamında; ilk fazda zorunlu değil.

| Kriter | Düz/encrypt | Hızlı hash | bcrypt | Argon2id |
| --- | --- | --- | --- | --- |
| Parola güvenliği | Yok | Zayıf | İyi | Çok iyi |
| Salt | — | Yetersiz | Var | Var |
| Adaptif cost | — | Hayır | Evet | Evet |
| Go desteği | — | — | Olgun | Olgun / kütüphane |
| Operasyon | — | — | Basit | İyi |
| Rehash | — | — | Evet | Evet |
| Bu projeye uygunluk | Red | Red | Yüksek | Tercih |

**İlk faz önerisi:** Argon2id tercih; Go ekosisteminde uygulanabilirlik sorununda bcrypt kabul edilebilir. Exact cost değeri uydurulmaz.

## 6. Authentication/session seçenekleri

### 6.1 Yalnız uzun ömürlü JWT

Server-side session yok; logout/iptal zayıf.

### 6.2 Tamamen stateful opaque session

Her istekte server-side doğrulama; mobil/web için çalışır ama access/refresh ayrımı yok.

### 6.3 Kısa ömürlü access token + server-side refresh session

- Kısa access token
- Daha uzun refresh credential
- Refresh session server-side kayıt
- Client/session context (public web FE, mobil, admin BO) ile ayrılmış session’lar
- Device/session bazlı iptal
- Refresh rotation
- Logout / `DISABLED` / `CLOSED` / hassas role değişiminde session iptali + korumalı işlemlerde güncel authorization

| Kriter | Uzun JWT | Stateful opaque | Access + refresh session |
| --- | --- | --- | --- |
| Web | Orta | İyi | İyi |
| Mobil | Orta | İyi | İyi |
| Logout | Zayıf | Güçlü | Güçlü |
| Session iptali | Zayıf | Güçlü | Güçlü |
| Çoklu cihaz | Zayıf | İyi | İyi |
| Admin BO | Zayıf | İyi | İyi (role + BO context) |
| Güvenlik | Düşük-orta | Yüksek | Yüksek |
| PostgreSQL/Railway | Kolay | Kolay | Kolay |
| İlk faz maliyeti | Düşük | Orta | Orta |
| Bu projeye uygunluk | Düşük | Orta | Yüksek |

**Öneri:** Kısa access token + server-side takip edilen refresh session; aynı identity, ayrı istemci bağlamları. Redis/Kafka/NATS zorunlu değildir; session kaydı PostgreSQL’de tutulabilir.

BO operasyonu için canonical `admin` rolü ve BO session context birlikte zorunludur. `DISABLED` / `CLOSED` kullanıcı korumalı işlem yapamaz (bölüm 7).

## 7. Access token sorumluluğu

Access token:

- Kısa ömürlüdür; süre gerçekten kısa tutulmalıdır (kesin süre bu dokümanda belirlenmez).
- Kullanıcı kimliğini taşır.
- Minimum authorization context taşıyabilir (ör. role snapshot).
- Backend tarafından doğrulanır.
- Client’tan gelen role bilgisinden bağımsızdır.
- Uzun süreli iptal için tek başına kullanılmaz.

### Role / status kaynak ayrımı

- **İstemcinin request ile gönderdiği role:** Güvenilmez.
- **Backend tarafından imzalanmış access token role claim’i:** Doğrulanabilir fakat zaman içinde eskiyebilen snapshot olabilir.
- **Canonical güncel role ve account status:** Backend user/session güvenlik durumudur.

### Anonim public vs korumalı işlem

- Anonim public advert arama/detay işlemleri account status kontrolüne bağlı değildir.
- Authentication gerektiren bütün korumalı işlemler yalnız `ACTIVE` kullanıcı tarafından yapılabilir.
- Korumalı işlem örnekleri: profil görüntüleme/güncelleme, kendi advert’lerini görüntüleme, taslak oluşturma/düzenleme, moderasyona gönderme, favori, medya, hesap/session/güvenlik işlemleri, BO işlemleri.

### Normal access token davranışı

- Normal logout yalnız ilgili refresh session’ı iptal eder.
- İlk fazda global access token blacklist zorunlu değildir.
- Mevcut kısa access token süresi dolana kadar teorik olarak çalışabilir.
- Bu nedenle access süresi kısa tutulur.

### `DISABLED` / `CLOSED` davranışı

- Bütün refresh session’lar iptal edilir.
- Yeni access token üretilemez.
- Kullanıcı eski access token taşısa bile hiçbir korumalı işlem yapamaz.
- Önceden üretilmiş access token, korumalı endpoint/use-case’lerde canonical account status veya eşdeğer server-side revocation ile reddedilir.
- Bu kontrol anonim public isteklerde gereksiz DB sorgusu anlamına gelmez.
- Kesin cache, security-version, kolon veya implementation yöntemi belirlenmez.

### Hassas role değişikliği

- `admin` rolü kaldırılırsa eski access token BO işlemi yapamaz.
- `user` → `admin` değişiminde eski token otomatik admin yetkisi kazanamaz.
- Hassas role değişiminde bütün refresh session’lar iptal edilir; yeniden login gerekir.
- BO işlemi için yalnız role yetmez: canonical `admin` + BO session context birlikte zorunludur.
- FE/mobil context’te üretilmiş token/session BO endpoint’inde kabul edilmez.
- Client context request body/header’dan serbestçe belirlenmez; `client_context=bo` göndererek yetki kazanılamaz.
- Session context güvenilir token/session metadata’sına bağlıdır.

Kesin claim listesi, blacklist tablosu, format veya algoritma belirlenmez.

**İlk faz:** Minimum kimlik + gerekli minimum context; kısa ömür; `DISABLED`/`CLOSED` ve BO için güncel revocation.

## 8. Refresh session ve rotation

Öneri:

- Refresh credential yüksek entropili ve tahmin edilemez.
- DB’de düz metin tutulmaz; hash’li saklanır.
- Her refresh’te rotation.
- Eski credential yeniden kullanımı replay; session ailesi iptal edilir.
- Her refresh session bir istemci bağlamına aittir: public web FE, Android/iOS mobil veya admin BO.
- Bir bağlamın refresh credential’ı başka bağlamda kontrolsüz kullanılmaz.
- BO session yalnız başarılı admin role kontrolünden sonra oluşturulur; normal kullanıcı için BO session oluşturulmaz.
- Tek session logout (bağlama özel) ve bütün session’ları iptal desteklenir.
- Absolute timeout + idle timeout kavramsal olarak ayrılabilir; BO için ileride daha kısa süre / sıkı idle / MFA uygulanabilir; kesin süre uydurulmaz.
- Device adı / user-agent metadata opsiyonel; IP saklama gizlilik dengesiyle ürün kararı.
- Refresh, access token gibi public profil payload taşımaz.
- Concurrent refresh atomik rotation ile yönetilir.
- `DISABLED` / `CLOSED` / hassas role değişiminde tüm refresh’ler iptal + yeni access üretimi engellenir.
- Password reset sonrası bütün refresh session’lar iptal edilir.
- Password change sonrası diğer bütün cihaz ve client context session’ları iptal edilir; mevcut güvenilir session yeniden authentication veya session rotation sonrasında devam edebilir.

## 9. Web ve mobil istemcilerde credential saklama

### Web

| Seçenek | XSS | CSRF | Devamlılık | Logout | Karmaşıklık | Uygunluk |
| --- | --- | --- | --- | --- | --- | --- |
| LocalStorage | Zayıf | Düşük CSRF | İyi | Orta | Düşük | Red |
| SessionStorage | Zayıf | Düşük CSRF | Zayıf | Orta | Düşük | Red |
| Yalnız HttpOnly cookie (tümü) | İyi | CSRF dikkat | İyi | İyi | Orta | Alternatif |
| Memory access + HttpOnly Secure SameSite refresh cookie | Daha iyi | CSRF dikkat | İyi | İyi | Orta | Tercih |

**Web önerisi:** Access token memory’de; refresh credential HttpOnly + Secure + uygun SameSite cookie. LocalStorage’da refresh yok.

Cookie / CSRF sınırları:

- FE refresh cookie’si ile BO refresh cookie’sinin uygulama kapsamı karıştırılmaz.
- Cookie domain/path/SameSite kararı gerçek deployment domain yapısına göre sonra kesinleşir.
- FE ve BO farklı origin/domain’deyse CSRF ve cookie kapsamı ayrıca doğrulanır.
- Credential’ın HttpOnly olması tek başına CSRF koruması değildir.
- Gerekli durumda SameSite, Origin kontrolü ve/veya CSRF koruması uygulanır.
- Kesin mekanizma domain mimarisi netleşmeden uydurulmaz.
- FE cookie/session BO endpoint yetkisi vermez; BO cookie/session ayrı bağlamdır.

### Android ve iOS

- Refresh credential platform secure storage’da.
- Access token secure storage veya kısa ömürlü bellek; sıradan AsyncStorage reddedilir.
- Uygulama kapanıp açılınca refresh ile yenileme.
- Mobil session ayrı istemci bağlamıdır; BO session’ı değildir ve BO endpoint’inde kabul edilmez.
- Root/jailbreak riski tamamen çözülemez; yine de secure storage kullanılır.

Kesin framework paketi seçilmez.

## 10. FE ve BO authentication sınırı

Öneri:

- FE ve BO aynı identity/user sistemini kullanabilir; ayrı identity sistemi kurulmaz.
- Session istemci bağlamları: public web FE, Android/iOS mobil, admin BO.
- Aynı kullanıcı aynı anda farklı bağlamlarda ayrı session’lara sahip olabilir.
- Admin FE’de normal kullanıcı deneyimini kullanabilir; FE/mobil session BO session’ı değildir.
- Admin kullanıcının FE veya mobil session’ında role snapshot `admin` olabilir; bu tek başına BO endpoint erişimi vermez.
- Bir BO operasyonu için iki koşul birlikte gerekir: (1) canonical güncel rol `admin`, (2) session/access context admin BO için oluşturulmuş olmalı.
- BO session yalnız başarılı admin role kontrolünden sonra oluşturulur; normal kullanıcı için BO session oluşturulmaz.
- FE veya mobil session’ın yalnız BO login ekranına taşınması admin erişimi sağlamaz.
- Public web FE veya mobil context için üretilmiş token/session BO endpoint’inde kabul edilmez.
- Client context request body/header’dan serbestçe belirlenmez; `client_context=bo` ile yetki kazanılamaz.
- Session context güvenilir token/session metadata’sına bağlıdır.
- BO endpoint authorization yalnız role kontrolünden oluşmaz; role + BO context birlikte doğrulanır.
- `DISABLED` / `CLOSED` kullanıcı BO dahil hiçbir korumalı işlem yapamaz.
- BO için ileride daha kısa session, sıkı idle veya MFA uygulanabilir; MFA ilk fazda zorunlu değildir.
- Exact audience claim, cookie adı, domain veya token formatı belirlenmez.

## 11. Kayıt akışı

Kavramsal akış:

1. Kullanıcı temel bilgiler ve parolayı gönderir.
2. Backend input doğrular.
3. Normalize e-posta benzersizliği kontrol edilir.
4. Parola güvenli hash’lenir.
5. Rol sistem tarafından `user` atanır.
6. Hesap oluşturulur (`ACTIVE`; e-posta doğrulama ayrı nitelik).
7. E-posta doğrulama gereksinimi uygulanır.
8. Session hemen açılıp açılmayacağı belirlenir.

**İlk faz önerisi:**

- E-posta doğrulama gerekir.
- Doğrulanmamış kullanıcı sınırlı giriş yapabilir (ör. doğrulama hatırlatması); moderasyona advert göndermemelidir.
- Advert taslağı oluşturma ürün kararıdır; öneri: taslak izinlenebilir, gönderim doğrulama sonrası.
- Aynı e-posta tekrar kayıtta genel/hesap keşfini azaltan davranış.
- Public kayıt `role` kabul etmez.
- Şifre politikası ürün/teknik (min uzunluk vb. uydurulmaz).
- Terms/privacy ürün kapsamı.

## 12. Giriş akışı

Akış:

- Normalize e-posta ile kullanıcı bulma
- Parola hash doğrulama
- Hesap status kontrolü (`ACTIVE` / `DISABLED` / `CLOSED`); yalnız `ACTIVE` için korumalı session açılır
- E-posta doğrulama kontrolü (kısmi izin politikasına göre)
- Başarısız giriş: genel hata, rate limit, geçici lock/backoff
- Başarılı login: ilgili istemci bağlamında session oluşturma; diğer cihaz/bağlam oturumları varsayılan korunur
- BO login: canonical role = admin kontrolü + BO session context oluşturma; normal kullanıcıya BO session yok
- FE/mobil login admin BO yetkisi vermez; FE/mobil token BO endpoint’inde kabul edilmez
- Security event kaydı (bağlam bilgisiyle)

Kurallar:

- “E-posta yok” / “parola yanlış” ayrı mesajlarla hesap keşfi yapılmaz; genel hata.
- Zaman farkı ile bilgi sızdırma mümkün olduğunca azaltılır.
- Başarılı login loguna parola/token yazılmaz.

## 13. Logout ve session iptali

| Olay | Davranış |
| --- | --- |
| Tek cihaz/bağlam logout | İlgili refresh session iptal (FE logout yalnız FE; BO logout yalnız BO) |
| Bütün cihazlardan logout | Kullanıcının tüm istemci bağlamlarındaki refresh session’lar iptal |
| Access token (normal logout) | Kısa ömürle doğal sona erer; global blacklist zorunlu değil; teorik olarak süre dolana kadar çalışabilir |
| Admin hedef session iptali | Hedef kullanıcının session’ları iptal edilebilir |
| DISABLED / CLOSED | Tüm refresh session’lar iptal; yeni access üretilemez; eski access ile hiçbir korumalı işlem yapılamaz |
| Unutulan parola sıfırlama | Bütün refresh session’lar iptal; korumalı işlemlerde access revocation; yeniden login gerekir |
| Girişliyken parola değiştirme | Diğer bütün cihaz ve client context session’ları iptal; mevcut güvenilir session yeniden authentication veya session rotation sonrası devam edebilir |
| Hassas role değişikliği | Tüm refresh’ler iptal; yeniden login; eski token otomatik admin yetkisi kazanamaz / kaybedilen admin ile BO yapamaz |
| Refresh replay şüphesi | Session ailesi iptal + korumalı işlem revocation |

**Normal logout vs `DISABLED` / `CLOSED`:** Normal logout yalnız ilgili refresh’i iptal eder; kısa access süresi dolana kadar teorik çalışabilir. `DISABLED` / `CLOSED` durumunda mevcut access token korumalı işlemlerde çalışamaz.

## 14. E-posta doğrulama

Öneri:

- Tek kullanımlık, süreli doğrulama credential’ı.
- Hash’li saklanır; kullanınca geçersizleşir.
- Yeniden gönderme rate limited; eski credential’lar geçersizleşebilir.
- E-posta değişikliğinde yeni adres doğrulanır.
- Hesap enumeration azaltılır.
- Doğrulanmış e-posta başka hesaba atanamaz.
- Link yönlendirmesi FE/web/mobile ürün/teknik kararı.
- Doğrulama sonrası kısıtlar kalkar.
- Doğrulama kalıcı user status’una gömülmez.

Kesin süre, URL, template yazılmaz.

## 15. Parola sıfırlama ve parola değiştirme

### Unutulan parola sıfırlama

- Kullanıcı mevcut parolasını bilmeden reset credential ile yeni parola belirler.
- E-posta girilir; hesap var/yok fark etmeksizin genel response.
- Reset credential hash’li, süreli ve tek kullanımlıktır; kullanınca geçersizleşir.
- Yeni parola güvenli hash’lenir.
- Başarılı reset sonrasında bütün refresh session’lar iptal edilir.
- Mevcut access token’lar korumalı işlemlerde revocation/security kontrolüyle reddedilir.
- Kullanıcı tekrar login olmalıdır.
- Eski veya çalınmış hiçbir session açık kalmamalıdır.
- Rate limiting uygulanır.

### Giriş yapılmışken parola değiştirme

- Mevcut parola tekrar doğrulanır.
- Yeni parola politikası uygulanır.
- Diğer bütün cihaz ve client context session’ları iptal edilir.
- Mevcut güvenilir session, yeniden authentication veya session rotation sonrasında devam edebilir.
- Mevcut session’ın hiçbir kontrol olmadan aynı credential ile devam etmesi önerilmez.
- Kullanıcıya diğer cihazlardan çıkış yapıldığı anlaşılır şekilde bildirilebilir; kesin UX metni yazılmaz.

### Admin

- Kullanıcının parolasını göremez.
- Kullanıcı adına doğrudan yeni parola belirlemez.
- Güvenilir reset akışı başlatır.

## 16. Hesap ve e-posta değişikliği

Öneri:

- Ad/soyad profil güncellemesi ayrı sade akış; yalnız `ACTIVE` kullanıcı.
- E-posta değişikliği yeniden doğrulama; yeni e-posta doğrulanana kadar eski e-posta giriş kimliği olarak kalabilir (ürün netleştirme).
- Role yalnız güvenilir admin/operasyon akışı; hassas role değişiminde bütün refresh iptali + yeniden login.
- Admin disable → `DISABLED` (yeniden açılabilir admin süreci).
- Kullanıcı hesap kapatma → `CLOSED` (yeniden açma ürün kararı; fiziksel silme değil).
- Advert geçmişi ve operasyonel ilişkiler korunur.
- Kişisel veri anonimleştirme hukuki/ürün kararı; süre uydurulmaz.
- Pasif/kapalı kullanıcının advert public davranışı ayrı ürün kararı; anonim public arama/detay account status’a bağlı değildir.
- `DISABLED` / `CLOSED`: tüm refresh iptali + yeni access engeli + eski access ile hiçbir korumalı işlem yok.

## 17. Rate limiting ve brute-force koruması

Limit gerektiren işlemler: kayıt, login, refresh, doğrulama gönderimi, reset talebi, reset doğrulama, BO login, hassas admin operasyonları.

Değerlendirme:

- IP + hesap/e-posta bazlı limit kavramsal olarak yararlı.
- Progressive backoff / geçici lock ayrı sorumluluk (kalıcı user status değil).
- BO login ve hassas admin operasyonları ayrı / daha sıkı limit adayıdır.
- Reverse proxy IP bilgisinin güvenilirliği dikkatli ele alınır.
- Redis ilk fazda zorunlu değil.
- İlk faz: uygulama seviyesi + PostgreSQL destekli sayaç/lock mümkün; çoklu instance’da sınırlar gevşeyebilir.
- Büyümede distributed rate limit (ör. Redis veya edge) eşiği ürün/trafik ile belirlenir.

Kesin sayı/süre uydurulmaz.

## 18. Güvenlik logları ve gizlilik

Loglanabilecek olaylar: başarılı/başarısız login, refresh replay, logout, toplu iptal, parola değişimi, parola sıfırlama (bütün session iptali), e-posta değişimi, role değişimi, `DISABLED` / `CLOSED`, BO login (istemci bağlamı ile), BO context uyumsuz erişim denemesi.

Kurallar:

- Parola, hash, access/refresh credential loglanmaz.
- E-posta tam metin gereksiz yere loglanmamalıdır (maskeleme/hash tercih).
- IP/user-agent saklama gizlilik–operasyon dengesi ürün kararı.
- Güvenlik logu ≠ genel uygulama logu; status history değildir.
- Retention bu dokümanda belirlenmez.

## 19. Eş zamanlılık ve idempotency

Riskler: eş zamanlı aynı e-posta kayıt, çift refresh, rotation yarışı, çift reset/doğrulama kullanımı, parola değişirken refresh, `DISABLED`/`CLOSED`–login yarışı, role değişimi + eski access token, çapraz client context refresh / BO endpoint, çift e-posta değişikliği.

Yaklaşımlar:

- Benzersizlik yalnız uygulama kontrolüne bırakılmaz (DB seviyesinde de).
- Tek kullanımlık credential tüketimi transaction içinde.
- Refresh rotation atomik.
- Replay → session ailesi iptali.
- Stale session reddedilir.
- `DISABLED` / `CLOSED`: refresh iptali + yeni access engeli + korumalı işlemlerde access reddi.
- Hassas role değişimi: bütün refresh iptali + yeniden login; eski token otomatik yetki değişmez.
- Password reset: bütün refresh iptali + korumalı access reddi.
- Password change: diğer bütün context session’ları iptal; mevcut session yeniden auth/rotation sonrası.
- Client context uyumsuz refresh veya BO endpoint erişimi reddedilir.
- Resend idempotent/rate limited.

Kesin kolon/SQL yazılmaz.

## 20. FE, BO ve backend sorumlulukları

### Haradan FE

- Kayıt, login, logout, doğrulama, parola sıfırlama sunar.
- Role seçtirmez; admin menüsü ana yaklaşım değildir.
- FE session’ı BO session’ı değildir; BO endpoint erişimi vermez.
- Parola/token loglamaz.
- Credential’ları platforma uygun güvenli yerde tutar.
- Backend authorization yerine geçmez.
- Session bitince anlaşılır yeniden giriş.
- Parola değişiminde diğer cihazlardan çıkış bildirilebilir.

### Haradan BO

- Yalnız admin operasyonları; BO session ayrı istemci bağlamıdır.
- Login ekranı role bypass değildir; FE/mobil session BO erişimi vermez.
- Her BO operasyonu için canonical `admin` + BO context birlikte doğrulanır; başarısızsa operasyon sunmaz.
- Admin parolasını veya kullanıcı parolasını görüntülemez; doğrudan parola belirlemez; reset akışı başlatır.
- Hassas role değişimi / disable / close ayrı BO iş kurallarıdır.
- Exact ekran tasarımı yok.

### Backend

- User kimliği, rolü ve hesap durumunun güvenilir kaynağıdır.
- Parolayı güvenli hash’ler; authn/authz ayırır.
- Session’ı istemci bağlamıyla oluşturur, yeniler, iptal eder; rotation/replay korur.
- Anonim public ve korumalı işlem ayrımını uygular; korumalı işlem yalnız `ACTIVE`.
- BO için role + BO context birlikte doğrular; ownership kontrolü uygular.
- `DISABLED` / `CLOSED` / hassas role / password reset-change etkilerini session ve korumalı access’e yansıtır.
- Rate limiting ve güvenlik logları; token/parola loglamaz.
- FE/BO’nun gönderdiği role, owner veya client_context’e güvenmez.

## 21. Karşılaştırma tabloları

### 21.1 Authentication/session yaklaşımı

(Bkz. bölüm 6 tablosu.)

### 21.2 Parola hash yaklaşımı

(Bkz. bölüm 5 tablosu.)

### 21.3 Web credential saklama

(Bkz. bölüm 9 tablosu.)

### 21.4 Hesap durumu yaklaşımı

| Kriter | Her şeyi tek status | ACTIVE + DISABLED + CLOSED + ayrı doğrulama/lock/session |
| --- | --- | --- |
| Sadelik | Yanıltıcı basit | Daha net |
| Geçersiz kombinasyon | Yüksek | Düşük |
| E-posta doğrulama | Karışır | Ayrı nitelik |
| Brute-force lock | Yanlış kalıcılaşır | Ayrı |
| Admin disable | Kullanıcı kapatmayla karışır | `DISABLED` |
| Hesap kapatma | Admin disable ile karışır | `CLOSED` |
| Bakım | Zor | Daha kolay |
| Bu projeye uygunluk | Düşük | Yüksek |

## 22. Önerilen karar

**Özet model:** E-posta giriş kimliği; aynı user domain + `user`/`admin` role (`VARCHAR + CHECK`); kalıcı status `ACTIVE` / `DISABLED` / `CLOSED`; Argon2id (veya bcrypt); kısa access token + server-side hashed refresh session + rotation + client context (FE / mobil / BO); web’de memory access + HttpOnly refresh (FE/BO cookie kapsamı ayrı); mobilde secure storage; korumalı işlem yalnız `ACTIVE`; BO için `admin` + BO context; password reset bütün session iptali; password change diğer session iptali + mevcut session rotation/re-auth; e-posta doğrulama ayrı nitelik; Redis zorunlu değil.

Soru cevapları:

1. **Giriş kimliği e-posta mı?** Evet.
2. **Telefon giriş kimliği?** Hayır (ilk faz).
3. **Kullanıcı adı?** Hayır.
4. **Admin aynı user domain?** Evet.
5. **Public kayıt role?** Hayır.
6. **Profil güncelleme role?** Hayır.
7. **E-posta normalize?** Trim + case-insensitive.
8. **Gmail dönüşümü?** Hayır.
9. **Parola saklama?** Adaptif hash.
10. **Argon2id mi bcrypt mi?** Argon2id tercih; bcrypt alternatif.
11. **Pepper zorunlu mu?** Hayır.
12. **Session modeli?** Access + server-side refresh + client context.
13. **Uzun tek JWT?** Hayır.
14. **Refresh server-side?** Evet.
15. **Refresh düz metin DB?** Hayır; hash.
16. **Rotation?** Evet.
17. **Replay tespit?** Evet; aile iptali.
18. **Access kısa ömür?** Evet.
19. **Role token’da?** Minimum snapshot olabilir; canonical kaynak backend; BO için role + BO context.
20. **Role/status → refresh iptal?** Evet; `DISABLED`/`CLOSED` ve hassas role’de yeni access engeli + korumalı işlem reddi; role’de yeniden login.
21. **Web refresh?** HttpOnly Secure uygun SameSite cookie; FE/BO kapsamı ayrı.
22. **Web access?** Memory.
23. **Mobil refresh?** Platform secure storage.
24. **BO/FE aynı identity?** Evet; session context ayrı.
25. **BO login = admin erişim?** Hayır; her operasyonda `admin` + BO context.
26. **İleride MFA?** Evet, özellikle admin/BO.
27. **E-posta doğrulama?** Evet (ayrı nitelik).
28. **Doğrulanmamış login?** Sınırlı/evet (ürün detayı).
29. **Taslak oluşturma?** Öneri: evet (ürün); yalnız `ACTIVE`.
30. **Moderasyona gönderme?** Hayır (doğrulama sonrası); yalnız `ACTIVE`.
31. **Reset hesap var/yok açıklar mı?** Hayır.
32. **Reset/doğrulama hash’li mi?** Evet.
33. **Parola değişince diğer session iptal?** Evet; mevcut session re-auth/rotation sonrası devam edebilir.
34. **Logout access’i her yerde anında?** Normal logout’ta zorunlu değil; `DISABLED`/`CLOSED`/reset’te korumalı işlem reddedilir.
35. **Disable/Closed → session iptal?** Evet + yeni access engeli + korumalı işlem yok.
36. **Brute-force lock user status’ta mı?** Hayır.
37. **Login hata genel mi?** Evet.
38. **Rate limiting?** Evet.
39. **Redis zorunlu mu?** Hayır.
40. **User fiziksel silme?** Hayır (ilk faz); `CLOSED` fiziksel silme değildir.
41. **Role = ownership?** Hayır.
42. **FE/BO owner/role/client_context belirleyebilir mi?** Hayır.
43. **Security log’da token/parola?** Hayır.
44. **Refresh rotation transaction?** Evet.
45. **Aynı e-posta eş zamanlı kayıt DB engeli?** Evet.

**Gerekçe:** Sade UX, ileri yaş, çoklu istemci, BO role+context güvenliği, `DISABLED`/`CLOSED` tam koruma, password reset/change ayrımı, XSS/CSRF dengesi, parola güvenliği, PostgreSQL/Railway, MFA’ya açık, gereksiz RBAC yok, FE–BO ayrımı.

## 23. Reddedilen yaklaşımlar

- Düz metin veya geri çözülebilir parola
- Hızlı genel hash
- Parola veya token loglamak
- Uzun ömürlü tek JWT ile session yönetimi
- Refresh’i DB’de düz metin tutmak
- Rotation yapmamak / replay görmezden gelmek
- Access token’a bütün profili gömmek
- Web refresh’i localStorage’da tutmak
- Mobil refresh’i sıradan AsyncStorage’da tutmak
- Public kayıt veya profil’den role kabul etmek
- BO login’i admin saymak / yalnız UI ile role korumak
- FE/mobil session’ı BO erişimi saymak
- Yalnız role kontrolüyle BO endpoint açmak (BO context olmadan)
- Request’ten `client_context=bo` kabul ederek yetki vermek
- Authn ile ownership’i aynı saymak
- Login/reset’te hesap keşfi
- Reset/doğrulama credential düz metin
- Disable/Closed sonrası session açık bırakmak
- `DISABLED`/`CLOSED` kullanıcının eski access ile korumalı işlem yapmasına izin vermek
- Hassas role değişiminde yalnız refresh iptaline güvenip eski token ile otomatik yetki değişimi kabul etmek
- Password reset sonrası “bazı session’lar açık kalsın” belirsizliği
- Password change’de mevcut session’ın kontrolden/re-auth olmadan devam etmesi
- Geçici lock’u kalıcı status yapmak
- `DISABLED` ile `CLOSED`’ı aynı olay saymak
- Gmail nokta/plus normalizasyonu
- User’ı advert geçmişini bozacak fiziksel silmek
- İlk fazda çoklu rol/permission tabloları
- Redis’i doğrulanmış ihtiyaç olmadan zorunlu kılmak
- HttpOnly’nin tek başına CSRF koruması olduğunu varsaymak

## 24. Açık kalan ürün ve teknik kararlar

- Telefon zorunluluğu / doğrulama
- Doğrulama öncesi FE özellik sınırları (taslak kesinliği)
- Parola min uzunluk ve detay politikası
- Kesin hash algoritması ve cost
- Pepper kullanımı
- Access token süresi (kısa tutulmalı; değer uydurulmaz)
- Refresh idle/absolute süreleri (BO için ayrı sıkılık dahil)
- Web FE ve backend domain/cookie mimarisi
- FE vs BO cookie domain/path/SameSite ve CSRF mekanizması
- BO session süresi ve idle politikası
- Admin MFA zamanı
- `CLOSED` hesabın yeniden açılma politikası
- Eşzamanlı session sayısı / session listesi UI
- IP ve user-agent saklama kapsamı
- Rate limit değerleri
- Distributed rate limit eşiği
- E-posta sağlayıcısı
- Deep-link davranışı
- Hesap kapatma ve anonimleştirme
- Pasif/kapalı kullanıcı advert public davranışı
- Role değişikliği BO iş akışı
- Kritik revocation’ın kesin implementation biçimi (security version vb.)
- Security log retention
- KVKK / veri saklama gereksinimleri

## 25. Sonraki adım

Bu karar kabul edilirse sonraki teknik karar TJK sync, worker ve operasyon modelinin ayrıntılandırılmasıdır:

- İlk toplu veri alma
- Artımlı sync
- Worker/job sorumluluğu
- Retry
- Checkpoint
- Idempotency
- Kısmi merge
- Sync run geçmişi
- BO operasyon görünürlüğü
- Medya processing ve TJK sync worker sınırı
- Railway deployment modeli

Bu dokümanda worker tablosu, queue teknolojisi, API veya kod üretilmez.
