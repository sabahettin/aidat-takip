# Aidat Takip

Spor salonu, dernek gibi üyelik/aidat toplayan işletmeler için basit, **tek dosyalık** aidat ve üye takip uygulaması.

## Özellikler

### Üye yönetimi
- Üye ekle / düzenle / aktif-pasif yap
- Üye bilgileri: ad soyad, telefon, e-posta, doğum tarihi, veli adı/telefonu (çocuk üyeler için), not
- Silinen üyeler çöp kutusuna taşınır; oradan geri yüklenebilir veya kalıcı olarak silinebilir

### Üyelik dönemleri ve aidat
- Her üye için birden fazla **üyelik dönemi** tutulur (başlangıç/bitiş tarihi, aylık aidat tutarı, not) — böylece bir üyenin aidatı zam gördüğünde veya üyeliğe ara verip tekrar başladığında geçmiş kayıtlar bozulmaz
- **Aidat Güncelle**: hangi aydan itibaren geçerli olacağını seçip yeni tutarı girerek, o ay ve sonrası için aidatı tek seferde günceller
- Katılım tarihinden itibaren ay ay ödeme durumu otomatik hesaplanır, geciken aylar tespit edilir

### Ödemeler
- Ödeme kaydetme (nakit / EFT-Havale), tarih ve not ile
- Ödeme geçmişi düzenlenebilir/silinebilir
- Geciken ödemeler için tek tıkla **WhatsApp hatırlatma mesajı** linki oluşturulur

### Panel (Dashboard)
- Aktif/pasif üye sayısı
- Geciken üyelerin listesi (isim ve kaç ay geciktiği) ve hızlı ödeme/hatırlatma aksiyonları
- **Gizlilik**: panelde hiçbir kazanç/tutar bilgisi gösterilmez (panel herkese açık bir ekranda kalabileceği için) — tüm parasal veriler yalnızca Raporlar sayfasındadır

### Raporlar
- Son 12 ayın tahsilat grafiği (aylık toplam, en iyi ay, ortalama)
- Ödeme yöntemine göre dağılım (nakit / EFT)
- Aktif/toplam üye sayısı, geciken üye sayısı ve toplam gecikmiş tutar

### Sınıf/Grup yönetimi
- Sınıf/Grup oluşturma (ör. "Cumartesi 10:00 Minikler"), not ekleme
- Gruba haftalık ders programı (gün + saat aralığı) tanımlama
- Gruba üye/öğrenci atama ve gruptan çıkarma
- Her grup için bir **eğitmen** atanabilir

### Eğitmenler
- Eğitmen ekle / düzenle / sil (ad soyad, telefon, e-posta, not)
- Bir eğitmenin görevli olduğu tüm gruplar tek ekranda görülebilir
- Bir eğitmen silinirse, atandığı gruplardan otomatik olarak kaldırılır (gruplar silinmez)

### Yoklama
- Her grup için tarih bazlı yoklama alma (öğrenci listesinde katıldı/katılmadı işaretleme)
- Geçmiş yoklama oturumları ve katılım oranları (kaç kişi / toplam kaç kişi) listelenir

### Takvim
- Tüm grupların haftalık ders programı **tek bir takvimde** (FullCalendar, Türkçe arayüz) görsel olarak görüntülenir
- Her ders bloğunda grup adı ve varsa eğitmen adı gösterilir
- Bir derse tıklandığında ilgili grup sayfasına gidilir

## Neden tek dosya?

Uygulama [Go](https://go.dev) ile yazıldı ve tüm HTML şablonları, statik dosyalar (`web/` klasörü) binary'nin içine gömülü (`embed`). Veritabanı olarak da saf Go ile yazılmış bir SQLite sürücüsü ([`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite)) kullanılıyor — yani **cgo, .NET, Node.js, Python gibi hiçbir ek kuruluma gerek yok**. Tek bir çalıştırılabilir dosyayı indirip çift tıklamak yeterli. Takvim gibi ön yüz kütüphaneleri de (jQuery, DataTables, SweetAlert2, toastr, FullCalendar) internetten çekilmeden binary'nin içine gömülüdür; uygulama tamamen **çevrimdışı** çalışır.

## Kullanım

1. [Releases](../../releases) sayfasından (veya `dist/` klasöründen) işletim sisteminize uygun dosyayı indirin:
   - Windows: `aidat-takip-windows-amd64.exe`
   - macOS (Apple Silicon): `aidat-takip-macos-arm64`
   - macOS (Intel): `aidat-takip-macos-amd64`
   - Linux: `aidat-takip-linux-amd64`
2. Dosyayı çift tıklayarak (macOS/Linux'ta `chmod +x` sonrası) çalıştırın.
3. Program otomatik olarak tarayıcınızda `http://127.0.0.1:8765/` adresini açar.
4. Kapatmak için terminal penceresini kapatın veya `Ctrl+C` yapın.

Veriler, programın bulunduğu klasörde oluşan **`aidat.db`** dosyasında saklanır. Bu dosyayı yedeklemeniz veya başka bir bilgisayara taşımanız yeterlidir — tüm üye, ödeme, grup ve yoklama geçmişiniz onunla birlikte gider.

## Geliştirme

Gereksinim: Go 1.24+

```bash
go run .
```

Testleri çalıştırmak için:

```bash
go test ./...
```

### Tüm platformlar için derleme

```bash
./build.sh
```

Bu komut `dist/` klasörüne Windows, macOS (Intel/Apple Silicon) ve Linux için tek dosyalık binary'ler üretir.

## Yol haritası

- SMS/e-posta ile otomatik ödeme hatırlatması (dış servis entegrasyonu gerektirdiği için ilk sürüme dahil edilmedi; panelde ve WhatsApp linkleriyle geciken ödemeler zaten listeleniyor)
