# Haradan Backend — Proje Kapsamı

## 1. Projenin amacı

Haradan, at ve atçılık ekosistemine yönelik sıfırdan geliştirilen bir dijital ilan platformudur. Backend, bu platformun FE (React Native) ve BO (ayrı repo) istemcilerine hizmet veren API katmanını sağlar; ilk fazda payment hariç temel ilan, kullanıcı, kategori, TJK at verisi ve moderasyon yeteneklerini kapsar.

## 2. İlk backend fazının kapsamı

Şirket talebine sadık kalarak ilk backend fazı şunları kapsar:

- Ürün kapsamının analiz edilmesi
- Veritabanı tablo yapısının belirlenmesi
- Fonksiyon listesinin belirlenmesi
- Payment hariç backend geliştirilmesi
- FE ve BO için gerekli API’lerin oluşturulması

## 3. Kesin kararlar

Yalnızca kesinleşmiş bilgiler:

- Backend: Go
- Veritabanı: PostgreSQL
- FE: React Native; web, Android ve iOS için tek kod tabanı hedefi
- BO: ayrı repo
- At verisi kaynağı: yalnızca TJK
- YKK kullanılmayacak
- Storage: Backblaze B2, S3-compatible
- Payment ilk backend fazının dışında
- Kullanıcı rolleri yalnızca normal kullanıcı ve admin
- İl ve ilçe dışında ayrıntılı adres gerekmiyor
- Legacy sistem hiçbir şekilde referans alınmayacak

## 4. İlk fazın fonksiyonel alanları

Kapsam başlıkları (tablo veya endpoint tasarımı yok):

- Kullanıcı hesabı ve kimlik doğrulama
- Yetkilendirme
- İl ve ilçe
- Kategori yönetimi
- Kategori property yönetimi
- TJK at verileri
- TJK senkronizasyon işleri
- İlan oluşturma ve yönetme
- İlan moderasyonu
- İlan arama ve filtreleme
- Favoriler
- İlan görselleri
- Banner yönetimi

## 5. Kapsam dışı işler

Aşağıdakiler ilk backend fazının dışındadır:

- Payment tabloları
- Payment API’leri
- Ödeme entegrasyonu
- At satış bedelinin yönetilmesi
- Satış komisyonu
- Paket ve ilan öne çıkarma sisteminin uygulanması
- BO kodunun şu aşamada yazılması
- FE kodunun şu aşamada yazılması
- Deploy işlemleri

Gelecekteki paket sistemi sonraki faz konusudur; mevcut model bu özelliğin ileride eklenmesini engellememelidir.

## 6. Kapsamı henüz kesinleşmemiş işler

Aşağıdakiler karar bekleyen kapsamdadır:

- Eküri/toplu ilan özelliğinin ilk faza dahil olup olmadığı
- CSV, XLSX ve JSON toplu içe aktarma
- Görsel işleme sürecinin senkron veya asenkron olması
- SEARCH, HOMEPAGE ve DETAIL görsel varyantlarının kesin yapısı
- TJK servisinin gerçek veri modeli ve erişim yöntemleri
- TJK full, incremental ve on-demand sync kapsamı

## 7. Tasarımdan önce cevaplanması gereken teknik kararlar

Aşağıdaki sorular henüz cevaplanmamıştır; yalnızca karar gündemidir:

1. Kullanıcı rolü hangi PostgreSQL yapısıyla tutulmalı?
2. İl ve ilçe verileri hangi tablo yapısında tutulmalı?
3. İl ve ilçe başlangıç verisi nasıl yüklenmeli?
4. Category, category property, advert ve property value ilişkisi nasıl kurulmalı?
5. İlan property değerleri JSONB mi, ilişkisel tablo mu, hibrit mi olmalı?
6. Horse verisi kolonlu, full JSONB veya hibrit mi olmalı?
7. İlanın sabit kolonları ile dinamik property’leri nasıl ayrılmalı?
8. Arama filtrelerinin kaynakları nasıl sınıflandırılmalı?
9. Görsel metadata ve varyant kayıtları nasıl modellenmeli?
10. TJK senkronizasyon durumunun kalıcı verileri neler olmalı?
11. Toplu ilan özelliği ilk faza dahil edilmeli mi?

## 8. Çalışma kuralları

- Main branch üzerinde doğrudan geliştirme yapılmaz.
- Her görev ayrı feature branch üzerinde yürütülür.
- Legacy sistem referans alınmaz.
- Kesin olmayan bilgiler varsayım olarak uygulanmaz.
- Gereksiz tablo, mikroservis ve soyutlama oluşturulmaz.
- Payment ilk faza eklenmez.
- Her teknik karar uygulamadan önce gerekçelendirilir.
