// Command carohelper runs the local web UI. Double-click the exe; it opens
// the browser at http://localhost:<port>. Close the console window to stop.
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"

	"carohelper/internal/config"
	"carohelper/internal/server"
)

//go:embed all:dist
var distFS embed.FS

func main() {
	noBrowser := flag.Bool("no-browser", false, "do not open the browser on start")
	portFlag := flag.Int("port", 0, "override port from config")
	cfgPath := flag.String("config", "", "config file path (default: %APPDATA%\\CaroHelper\\config.json)")
	flag.Parse()

	log.SetFlags(log.Ltime)
	log.Println("Caro Helper starting")

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	log.Printf("config: %s", cfg.Path())
	if !cfg.HasToken() {
		log.Println("no Smartsheet token yet - enter it in the settings page")
	}

	static, err := fs.Sub(distFS, "dist")
	if err != nil {
		log.Fatal(err)
	}

	port := cfg.Port()
	if *portFlag != 0 {
		port = *portFlag
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("cannot listen on %s: %v (is Caro Helper already running?)", addr, err)
	}
	url := "http://" + addr
	log.Printf("listening on %s", url)

	if !*noBrowser {
		if err := openBrowser(url); err != nil {
			log.Printf("could not open browser: %v - open %s manually", err, url)
		}
	}

	srv := server.New(cfg, static)
	if err := http.Serve(ln, srv); err != nil {
		log.Fatal(err)
	}
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
