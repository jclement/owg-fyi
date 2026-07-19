// Package gmi implements the gemini:// server (and optional titan://
// uploads, gated behind an allowlist of client certificate fingerprints —
// the approach recommended by the Titan spec and used by molly brown).
package gmi

import (
	"bufio"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/jclement/owg-fyi/internal/config"
	"github.com/jclement/owg-fyi/internal/content"
	"github.com/jclement/owg-fyi/internal/search"
)

const (
	maxRequestLen = 1024
	ioTimeout     = 30 * time.Second
)

// Server is a gemini protocol server.
type Server struct {
	Cfg    *config.Config
	Store  *content.Store
	Search *search.Index
	Log    *log.Logger
	TLS    *tls.Config
}

// ListenAndServe accepts gemini connections until the listener fails.
func (s *Server) ListenAndServe() error {
	ln, err := tls.Listen("tcp", s.Cfg.GeminiAddr, s.TLS)
	if err != nil {
		return err
	}
	s.Log.Info("gemini listening", "addr", s.Cfg.GeminiAddr, "host", s.Cfg.Hostname)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(ioTimeout))
	start := time.Now()

	r := bufio.NewReaderSize(conn, maxRequestLen+2)
	line, err := readRequestLine(r)
	if err != nil {
		s.respondHeader(conn, 59, "bad request")
		return
	}
	u, err := url.Parse(line)
	if err != nil || u.Host == "" || u.User != nil {
		s.respondHeader(conn, 59, "bad request")
		return
	}

	status, meta := 0, ""
	switch u.Scheme {
	case "gemini":
		status, meta = s.serveGemini(conn, u)
	case "titan":
		status, meta = s.serveTitan(conn, r, u)
	default:
		status, meta = 53, "proxy request refused"
		s.respondHeader(conn, status, meta)
	}

	logFn := s.Log.Info
	if status >= 40 {
		logFn = s.Log.Warn
	}
	logFn("gemini",
		"status", status,
		"url", line,
		"remote", remoteIP(conn),
		"dur", time.Since(start).Round(time.Millisecond),
		"meta", meta)
}

func readRequestLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	if len(line) > maxRequestLen+2 {
		return "", fmt.Errorf("request too long")
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	if line == "" {
		return "", fmt.Errorf("empty request")
	}
	return line, nil
}

func remoteIP(conn net.Conn) string {
	if h, _, err := net.SplitHostPort(conn.RemoteAddr().String()); err == nil {
		return h
	}
	return conn.RemoteAddr().String()
}

func (s *Server) respondHeader(w io.Writer, status int, meta string) {
	fmt.Fprintf(w, "%d %s\r\n", status, meta)
}

func (s *Server) serveGemini(conn net.Conn, u *url.URL) (int, string) {
	if !strings.EqualFold(u.Hostname(), s.Cfg.Hostname) && !strings.EqualFold(u.Hostname(), "localhost") {
		s.respondHeader(conn, 53, "proxy request refused")
		return 53, "proxy request refused"
	}

	if u.Path == "/search" || u.Path == "/search/" {
		return s.serveSearch(conn, u)
	}

	res, err := s.Store.Open(u.Path)
	if err != nil {
		s.respondHeader(conn, 40, "temporary failure")
		return 40, "temporary failure"
	}
	switch res.Type {
	case content.RedirectResult:
		s.respondHeader(conn, 31, res.Location)
		return 31, res.Location
	case content.StaticResult:
		f, err := os.Open(res.FilePath)
		if err != nil {
			s.respondHeader(conn, 51, "not found")
			return 51, "not found"
		}
		defer f.Close()
		s.respondHeader(conn, 20, res.Mime)
		_, _ = io.Copy(conn, f)
		return 20, res.Mime
	case content.PageResult:
		s.respondHeader(conn, 20, "text/gemini; charset=utf-8")
		_, _ = io.WriteString(conn, res.Page.Gemtext())
		return 20, "text/gemini"
	default:
		s.respondHeader(conn, 51, "not found")
		return 51, "not found"
	}
}

func (s *Server) serveSearch(conn net.Conn, u *url.URL) (int, string) {
	if u.RawQuery == "" {
		s.respondHeader(conn, 10, "Search this capsule:")
		return 10, "input"
	}
	q, err := url.QueryUnescape(u.RawQuery)
	if err != nil || strings.TrimSpace(q) == "" {
		s.respondHeader(conn, 10, "Search this capsule:")
		return 10, "input"
	}
	hits := s.Search.Search(q, 20)
	var b strings.Builder
	fmt.Fprintf(&b, "# Search: %s\n\n", q)
	if len(hits) == 0 {
		b.WriteString("Nothing found. Try fewer or different words.\n")
	} else {
		fmt.Fprintf(&b, "%d result(s):\n\n", len(hits))
		for _, h := range hits {
			label := h.Title
			if h.Date != "" {
				label = h.Date + " " + label
			}
			fmt.Fprintf(&b, "=> %s %s\n", h.URL, label)
			if h.Snippet != "" {
				fmt.Fprintf(&b, "> …%s…\n", h.Snippet)
			}
		}
	}
	b.WriteString("\n=> / Home\n")
	s.respondHeader(conn, 20, "text/gemini; charset=utf-8")
	_, _ = io.WriteString(conn, b.String())
	return 20, "search"
}

// ---- titan uploads ------------------------------------------------------

// serveTitan handles titan://host/path;mime=...;size=N uploads. Only
// clients presenting an allowlisted certificate may write, and only inside
// the content root.
func (s *Server) serveTitan(conn net.Conn, r *bufio.Reader, u *url.URL) (int, string) {
	if !s.Cfg.TitanEnabled {
		s.respondHeader(conn, 53, "titan disabled")
		return 53, "titan disabled"
	}
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		s.respondHeader(conn, 50, "tls required")
		return 50, "tls required"
	}
	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		s.respondHeader(conn, 60, "client certificate required")
		return 60, "client certificate required"
	}
	fp := sha256.Sum256(certs[0].Raw)
	fpHex := hex.EncodeToString(fp[:])
	allowed := false
	for _, want := range s.Cfg.TitanCertFingerprints {
		if want == fpHex {
			allowed = true
			break
		}
	}
	if !allowed {
		s.respondHeader(conn, 61, "certificate not authorized")
		return 61, "certificate not authorized"
	}

	// path;mime=text/gemini;size=1234[;token=x]
	segs := strings.Split(u.Path, ";")
	rawPath := segs[0]
	params := map[string]string{}
	for _, kv := range segs[1:] {
		if k, v, found := strings.Cut(kv, "="); found {
			params[k] = v
		}
	}
	size, err := strconv.ParseInt(params["size"], 10, 64)
	if err != nil || size < 0 || size > s.Cfg.TitanMaxBytes {
		s.respondHeader(conn, 59, "bad or excessive size")
		return 59, "bad size"
	}

	cleaned, ok2 := content.Clean(rawPath)
	if !ok2 || strings.HasSuffix(cleaned, "/") {
		s.respondHeader(conn, 59, "bad path")
		return 59, "bad path"
	}
	dst := filepath.Join(s.Store.Root, filepath.FromSlash(strings.TrimPrefix(cleaned, "/")))
	if !strings.HasPrefix(dst, s.Store.Root+string(filepath.Separator)) {
		s.respondHeader(conn, 59, "bad path")
		return 59, "bad path"
	}

	if size == 0 { // zero size means delete, per titan convention
		if err := os.Remove(dst); err != nil {
			s.respondHeader(conn, 51, "not found")
			return 51, "not found"
		}
		s.respondHeader(conn, 20, "text/gemini")
		fmt.Fprintf(conn, "# Deleted\n=> gemini://%s%s\n", s.Cfg.Hostname, cleaned)
		return 20, "deleted"
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		s.respondHeader(conn, 40, "cannot create directory")
		return 40, "mkdir failed"
	}
	tmp := dst + ".titan-tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		s.respondHeader(conn, 40, "cannot write")
		return 40, "open failed"
	}
	_, cpErr := io.CopyN(f, r, size)
	closeErr := f.Close()
	if cpErr != nil || closeErr != nil {
		_ = os.Remove(tmp)
		s.respondHeader(conn, 40, "short write")
		return 40, "short write"
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		s.respondHeader(conn, 40, "rename failed")
		return 40, "rename failed"
	}
	geminiURL := "gemini://" + s.Cfg.Hostname + strings.TrimSuffix(cleaned, filepath.Ext(cleaned))
	s.respondHeader(conn, 30, geminiURL)
	return 30, "uploaded " + cleaned
}
