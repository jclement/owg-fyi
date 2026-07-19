// Package web serves the same content tree over HTTP/HTTPS with a thin
// HTML wrapper, plus /search and an Atom feed for the gemlog.
package web

import (
	"crypto/tls"
	"embed"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"golang.org/x/crypto/acme/autocert"

	"github.com/jclement/owg-fyi/internal/config"
	"github.com/jclement/owg-fyi/internal/content"
	"github.com/jclement/owg-fyi/internal/search"
)

//go:embed assets
var assets embed.FS

var pageTpl = template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<meta name="description" content="{{.Desc}}">
<link rel="stylesheet" href="/_/style.css">
<link rel="alternate" type="application/atom+xml" title="gemlog" href="/posts/feed.xml">
<link rel="icon" href="data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 100 100%22><text y=%22.9em%22 font-size=%2290%22>🛞</text></svg>">
</head>
<body>
<main>
{{.Body}}
</main>
<footer class="site">
<p>also on gemini: <a href="gemini://{{.Host}}/">gemini://{{.Host}}/</a> · <a href="/search">search</a> · served by <a href="https://github.com/jclement/owg-fyi">capsule</a></p>
</footer>
</body>
</html>
`))

type pageData struct {
	Title string
	Desc  string
	Host  string
	Body  template.HTML
}

// Server is the web half of the capsule.
type Server struct {
	Cfg    *config.Config
	Store  *content.Store
	Search *search.Index
	Log    *log.Logger
}

// Handler builds the full HTTP handler tree.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	sub, _ := fs.Sub(assets, "assets")
	mux.Handle("/_/", http.StripPrefix("/_/", cacheControl(http.FileServer(http.FS(sub)))))

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/search", s.handleSearch)
	mux.HandleFunc("/posts/feed.xml", s.handleFeed)
	mux.HandleFunc("/", s.handlePage)

	return s.logMiddleware(mux)
}

func cacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		if strings.HasPrefix(r.URL.Path, "/_/") || r.URL.Path == "/healthz" {
			return
		}
		logFn := s.Log.Info
		if rec.status >= 400 {
			logFn = s.Log.Warn
		}
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		logFn("web",
			"status", rec.status,
			"method", r.Method,
			"path", r.URL.RequestURI(),
			"remote", host,
			"dur", time.Since(start).Round(time.Millisecond))
	})
}

func (s *Server) render(w http.ResponseWriter, status int, title, desc string, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = pageTpl.Execute(w, pageData{
		Title: title,
		Desc:  desc,
		Host:  s.Cfg.Hostname,
		Body:  template.HTML(body),
	})
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	res, err := s.Store.Open(r.URL.Path)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	switch res.Type {
	case content.RedirectResult:
		http.Redirect(w, r, res.Location, http.StatusMovedPermanently)
	case content.StaticResult:
		w.Header().Set("Cache-Control", "public, max-age=3600")
		http.ServeFile(w, r, res.FilePath)
	case content.PageResult:
		body, err := res.Page.HTML()
		if err != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
			return
		}
		title := res.Page.Title
		if title == "" {
			title = s.Cfg.Hostname
		} else if !strings.Contains(strings.ToLower(title), s.Cfg.Hostname) {
			title = title + " · " + s.Cfg.Hostname
		}
		s.render(w, http.StatusOK, title, "OneWheelGeek's capsule — self-hosting, homelab, and small-web notes", body)
	default:
		s.render(w, http.StatusNotFound, "not found · "+s.Cfg.Hostname, "not found",
			`<h1>40 — not found</h1><p>That page isn't here. It may have drifted off into gemini-space.</p><p class="lnk"><a href="/">Back home</a></p>`)
	}
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	var b strings.Builder
	b.WriteString("<h1>Search</h1>\n")
	fmt.Fprintf(&b, `<form class="search" action="/search" method="get"><input type="text" name="q" value="%s" placeholder="tailscale, gpg, tunnels…" autofocus><button type="submit">go</button></form>`+"\n", html.EscapeString(q))
	if q != "" {
		hits := s.Search.Search(q, 20)
		if len(hits) == 0 {
			b.WriteString("<p>Nothing found. Try fewer or different words.</p>\n")
		} else {
			fmt.Fprintf(&b, "<p>%d result(s):</p>\n", len(hits))
			for _, h := range hits {
				date := ""
				if h.Date != "" {
					date = fmt.Sprintf(`<span class="date">%s</span> `, html.EscapeString(h.Date))
				}
				fmt.Fprintf(&b, `<div class="hit"><p class="lnk">%s<a href="%s">%s</a></p><p class="snip">…%s…</p></div>`+"\n",
					date, h.URL, html.EscapeString(h.Title), html.EscapeString(h.Snippet))
			}
		}
	}
	b.WriteString(`<p class="lnk"><a href="/">Back home</a></p>`)
	s.render(w, http.StatusOK, "search · "+s.Cfg.Hostname, "search this capsule", b.String())
}

func (s *Server) handleFeed(w http.ResponseWriter, r *http.Request) {
	entries := s.Store.List("/posts")
	var b strings.Builder
	base := "https://" + s.Cfg.Hostname
	updated := time.Now().UTC().Format(time.RFC3339)
	if len(entries) > 0 && entries[0].Date != "" {
		if t, err := time.Parse("2006-01-02", entries[0].Date); err == nil {
			updated = t.UTC().Format(time.RFC3339)
		}
	}
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom">` + "\n")
	fmt.Fprintf(&b, "<title>%s gemlog</title>\n", s.Cfg.Hostname)
	fmt.Fprintf(&b, `<link href="%s/posts/"/>`+"\n", base)
	fmt.Fprintf(&b, `<link rel="self" href="%s/posts/feed.xml"/>`+"\n", base)
	fmt.Fprintf(&b, "<id>%s/posts/</id>\n<updated>%s</updated>\n", base, updated)
	for _, e := range entries {
		if e.IsDir || e.Date == "" {
			continue
		}
		t, err := time.Parse("2006-01-02", e.Date)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "<entry>\n<title>%s</title>\n", html.EscapeString(e.Title))
		fmt.Fprintf(&b, `<link href="%s%s"/>`+"\n", base, e.URL)
		fmt.Fprintf(&b, "<id>%s%s</id>\n<updated>%s</updated>\n</entry>\n", base, e.URL, t.UTC().Format(time.RFC3339))
	}
	b.WriteString("</feed>\n")
	w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

// Serve starts the web listeners and blocks until one fails.
func (s *Server) Serve() error {
	h := s.Handler()

	if s.Cfg.Dev || !s.Cfg.ACME {
		s.Log.Info("web listening (plain http)", "addr", s.Cfg.HTTPAddr)
		return http.ListenAndServe(s.Cfg.HTTPAddr, h)
	}

	mgr := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(s.Cfg.Hostname),
		Cache:      autocert.DirCache(s.Cfg.DataDir + "/acme"),
		Email:      s.Cfg.ACMEEmail,
	}

	redirect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" { // keep the docker healthcheck off TLS
			fmt.Fprintln(w, "ok")
			return
		}
		target := "https://" + s.Cfg.Hostname + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})

	errCh := make(chan error, 2)
	go func() {
		s.Log.Info("web listening (http, acme+redirect)", "addr", s.Cfg.HTTPAddr)
		errCh <- http.ListenAndServe(s.Cfg.HTTPAddr, mgr.HTTPHandler(redirect))
	}()
	go func() {
		srv := &http.Server{
			Addr:      s.Cfg.HTTPSAddr,
			Handler:   h,
			TLSConfig: &tls.Config{GetCertificate: mgr.GetCertificate, MinVersion: tls.VersionTLS12},
		}
		s.Log.Info("web listening (https, lets encrypt)", "addr", s.Cfg.HTTPSAddr, "host", s.Cfg.Hostname)
		errCh <- srv.ListenAndServeTLS("", "")
	}()
	return <-errCh
}
