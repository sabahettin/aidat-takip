package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

//go:embed web/templates/*.html
var templatesFS embed.FS

//go:embed web/static
var staticFS embed.FS

var funcMap = template.FuncMap{
	"money": func(v float64) string {
		return fmt.Sprintf("%.2f ₺", v)
	},
	"periodLabel": periodLabelText,
}

func dbPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "aidat.db"
	}
	return filepath.Join(filepath.Dir(exe), "aidat.db")
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func main() {
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templatesFS, "web/templates/*.html")
	if err != nil {
		log.Fatalf("şablonlar yüklenemedi: %v", err)
	}

	db, err := openDB(dbPath())
	if err != nil {
		log.Fatalf("veritabanı açılamadı: %v", err)
	}
	defer db.Close()

	staticSub, err := fs.Sub(staticFS, "web/static")
	if err != nil {
		log.Fatalf("statik dosyalar yüklenemedi: %v", err)
	}
	staticHandler := http.StripPrefix("/static/", http.FileServerFS(staticSub))

	srv := newServer(db, tmpl, staticHandler)

	port := 8765
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("port %d dinlenemedi: %v (uygulama zaten çalışıyor olabilir)", port, err)
	}

	url := "http://" + addr + "/"
	fmt.Println("Aidat Takip uygulaması çalışıyor:", url)
	fmt.Println("Kapatmak için bu pencereyi kapatın veya Ctrl+C'ye basın.")
	fmt.Println("Veritabanı dosyası:", dbPath())
	openBrowser(url)

	if err := http.Serve(ln, srv.routes()); err != nil {
		log.Fatal(err)
	}
}
