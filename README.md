# Aidat Takip

Spor salonu, dernek gibi üyelik/aidat toplayan işletmeler için basit, **tek dosyalık** aidat ve üye takip uygulaması.

## Özellikler

- Üye yönetimi (ekle / düzenle / aktif-pasif yap / sil)
- Aylık aidat takibi: her üye için katılım tarihinden itibaren ay ay ödeme durumu
- Ödeme kaydetme, geciken ödemelerin otomatik tespiti
- Panel: aktif/pasif üye sayısı, bu ay toplanan tutar, geciken ödemeler
- Raporlar: son 12 ayın tahsilat tutarları

## Neden tek dosya?

Uygulama [Go](https://go.dev) ile yazıldı ve tüm HTML şablonları, statik dosyalar (`web/` klasörü) binary'nin içine gömülü (`embed`). Veritabanı olarak da saf Go ile yazılmış bir SQLite sürücüsü ([`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite)) kullanılıyor — yani **cgo, .NET, Node.js, Python gibi hiçbir ek kuruluma gerek yok**. Tek bir çalıştırılabilir dosyayı indirip çift tıklamak yeterli.

## Kullanım

1. [Releases](../../releases) sayfasından (veya `dist/` klasöründen) işletim sisteminize uygun dosyayı indirin:
   - Windows: `aidat-takip-windows-amd64.exe`
   - macOS (Apple Silicon): `aidat-takip-macos-arm64`
   - macOS (Intel): `aidat-takip-macos-amd64`
   - Linux: `aidat-takip-linux-amd64`
2. Dosyayı çift tıklayarak (macOS/Linux'ta `chmod +x` sonrası) çalıştırın.
3. Program otomatik olarak tarayıcınızda `http://127.0.0.1:8765/` adresini açar.
4. Kapatmak için terminal penceresini kapatın veya `Ctrl+C` yapın.

Veriler, programın bulunduğu klasörde oluşan **`aidat.db`** dosyasında saklanır. Bu dosyayı yedeklemeniz veya başka bir bilgisayara taşımanız yeterlidir — tüm üye ve ödeme geçmişiniz onunla birlikte gider.

## Geliştirme

Gereksinim: Go 1.24+

```bash
go run .
```

### Tüm platformlar için derleme

```bash
./build.sh
```

Bu komut `dist/` klasörüne Windows, macOS (Intel/Apple Silicon) ve Linux için tek dosyalık binary'ler üretir.

## Yol haritası

- SMS/e-posta ile otomatik ödeme hatırlatması (dış servis entegrasyonu gerektirdiği için ilk sürüme dahil edilmedi; panelde geciken ödemeler zaten listeleniyor)
