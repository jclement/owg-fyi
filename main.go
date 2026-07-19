// capsule — the owg.fyi gemini + web server.
//
// One binary, three listeners: gemini (:1965, self-signed TOFU cert),
// HTTP (:80, ACME challenges + redirect) and HTTPS (:443, Let's Encrypt).
// Both protocols serve the same content tree of gemtext and markdown.
package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/log"

	"github.com/jclement/owg-fyi/internal/certutil"
	"github.com/jclement/owg-fyi/internal/config"
	"github.com/jclement/owg-fyi/internal/content"
	"github.com/jclement/owg-fyi/internal/counter"
	"github.com/jclement/owg-fyi/internal/gmi"
	"github.com/jclement/owg-fyi/internal/search"
	"github.com/jclement/owg-fyi/internal/web"
)

const banner = `
   ██████╗ ██╗    ██╗ ██████╗    ███████╗██╗   ██╗██╗
  ██╔═══██╗██║    ██║██╔════╝    ██╔════╝╚██╗ ██╔╝██║
  ██║   ██║██║ █╗ ██║██║  ███╗   █████╗   ╚████╔╝ ██║
  ██║   ██║██║███╗██║██║   ██║   ██╔══╝    ╚██╔╝  ██║
  ╚██████╔╝╚███╔███╔╝╚██████╔╝██╗██║        ██║   ██║
   ╚═════╝  ╚══╝╚══╝  ╚═════╝ ╚═╝╚═╝        ╚═╝   ╚═╝
        capsule · gemini + web · onewheelgeek`

func main() {
	if len(os.Args) > 1 && os.Args[1] == "health" {
		os.Exit(healthCheck())
	}

	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: true,
		TimeFormat:      "15:04:05",
	})

	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		logger.Fatal("config", "err", err)
	}

	fmt.Fprintln(os.Stderr, "\x1b[38;5;215m"+banner+"\x1b[0m")
	logger.Info("starting capsule",
		"hostname", cfg.Hostname,
		"content", cfg.ContentDir,
		"data", cfg.DataDir,
		"dev", cfg.Dev,
		"acme", cfg.ACME,
		"titan", cfg.TitanEnabled)

	store := content.NewStore(cfg.ContentDir)
	store.Counter = counter.New(cfg.DataDir)
	idx := search.New(store, searchTTL(cfg))

	geminiCert, err := certutil.LoadOrCreate(cfg.DataDir, cfg.Hostname)
	if err != nil {
		logger.Fatal("gemini cert", "err", err)
	}

	tlsCfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{geminiCert},
		// request (never require) a client cert so titan can identify users
		ClientAuth: tls.RequestClientCert,
	}

	gemSrv := &gmi.Server{
		Cfg:    cfg,
		Store:  store,
		Search: idx,
		Log:    logger.With("proto", "gemini"),
		TLS:    tlsCfg,
	}
	webSrv := &web.Server{
		Cfg:    cfg,
		Store:  store,
		Search: idx,
		Log:    logger.With("proto", "web"),
	}

	errCh := make(chan error, 2)
	go func() { errCh <- gemSrv.ListenAndServe() }()
	go func() { errCh <- webSrv.Serve() }()
	logger.Fatal("listener died", "err", <-errCh)
}

func searchTTL(cfg *config.Config) time.Duration {
	if cfg.Dev {
		return 2 * time.Second
	}
	return 5 * time.Minute
}

// healthCheck is used as the docker HEALTHCHECK: hits the local web
// listener's /healthz.
func healthCheck() int {
	url := os.Getenv("CAPSULE_HEALTH_URL")
	if url == "" {
		url = "http://127.0.0.1:80/healthz"
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "unhealthy:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		fmt.Fprintln(os.Stderr, "unhealthy: status", resp.StatusCode)
		return 1
	}
	fmt.Println("ok")
	return 0
}
