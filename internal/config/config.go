// Package config holds runtime configuration for the capsule server.
package config

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// Config is the full runtime configuration, populated from flags with
// environment-variable defaults (flag wins over env, env wins over default).
type Config struct {
	// Hostname is the canonical hostname (used for TLS certs and URL checks).
	Hostname string
	// Onion is the site's Tor hidden-service hostname (x…x.onion). When set,
	// requests for it are served: plain HTTP on the web listener (Tor is
	// already end-to-end encrypted) and gemini://<onion>/ on the gemini one.
	Onion string
	// ContentDir is the root of the site content tree.
	ContentDir string
	// DataDir holds persistent state: ACME cache, generated gemini certs.
	DataDir string

	// Dev disables ACME / port-443 serving and runs plain HTTP on HTTPAddr.
	Dev bool

	HTTPAddr   string // e.g. ":80" (prod) or ":8080" (dev)
	HTTPSAddr  string // e.g. ":443"
	GeminiAddr string // e.g. ":1965"

	// ACME enables Let's Encrypt certificates for the web listener.
	ACME      bool
	ACMEEmail string

	// TitanEnabled turns on titan:// uploads (requires client certs).
	TitanEnabled bool
	// TitanCertFingerprints is the allowlist of SHA-256 client-cert
	// fingerprints (hex, lowercase, no colons) permitted to upload.
	TitanCertFingerprints []string
	// TitanMaxBytes caps a single titan upload.
	TitanMaxBytes int64
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := strings.ToLower(os.Getenv(key))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

// Load parses flags/env into a Config.
func Load(args []string) (*Config, error) {
	fs := flag.NewFlagSet("capsule", flag.ContinueOnError)
	c := &Config{}

	fs.StringVar(&c.Hostname, "hostname", envStr("CAPSULE_HOSTNAME", "localhost"), "canonical hostname (env CAPSULE_HOSTNAME)")
	fs.StringVar(&c.Onion, "onion", envStr("CAPSULE_ONION", ""), "tor hidden service hostname (env CAPSULE_ONION)")
	fs.StringVar(&c.ContentDir, "content", envStr("CAPSULE_CONTENT", "content"), "content directory (env CAPSULE_CONTENT)")
	fs.StringVar(&c.DataDir, "data", envStr("CAPSULE_DATA", "data"), "data directory for certs/state (env CAPSULE_DATA)")
	fs.BoolVar(&c.Dev, "dev", envBool("CAPSULE_DEV", false), "dev mode: plain HTTP, no ACME (env CAPSULE_DEV)")
	fs.StringVar(&c.HTTPAddr, "http", envStr("CAPSULE_HTTP", ""), "HTTP listen address (default :80, or :8080 in dev) (env CAPSULE_HTTP)")
	fs.StringVar(&c.HTTPSAddr, "https", envStr("CAPSULE_HTTPS", ":443"), "HTTPS listen address (env CAPSULE_HTTPS)")
	fs.StringVar(&c.GeminiAddr, "gemini", envStr("CAPSULE_GEMINI", ":1965"), "gemini listen address (env CAPSULE_GEMINI)")
	fs.BoolVar(&c.ACME, "acme", envBool("CAPSULE_ACME", true), "enable Let's Encrypt for web (env CAPSULE_ACME)")
	fs.StringVar(&c.ACMEEmail, "acme-email", envStr("CAPSULE_ACME_EMAIL", ""), "ACME account email (env CAPSULE_ACME_EMAIL)")
	fs.BoolVar(&c.TitanEnabled, "titan", envBool("CAPSULE_TITAN", false), "enable titan:// uploads (env CAPSULE_TITAN)")
	titanFP := fs.String("titan-certs", envStr("CAPSULE_TITAN_CERTS", ""), "comma-separated sha256 client cert fingerprints allowed to titan upload (env CAPSULE_TITAN_CERTS)")
	fs.Int64Var(&c.TitanMaxBytes, "titan-max-bytes", 10<<20, "max titan upload size in bytes")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if c.HTTPAddr == "" {
		if c.Dev {
			c.HTTPAddr = ":8080"
		} else {
			c.HTTPAddr = ":80"
		}
	}
	if c.Dev {
		c.ACME = false
	}

	for _, fp := range strings.Split(*titanFP, ",") {
		fp = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(fp, ":", "")))
		if fp != "" {
			c.TitanCertFingerprints = append(c.TitanCertFingerprints, fp)
		}
	}
	if c.TitanEnabled && len(c.TitanCertFingerprints) == 0 {
		return nil, fmt.Errorf("titan enabled but no client cert fingerprints configured (CAPSULE_TITAN_CERTS)")
	}

	if st, err := os.Stat(c.ContentDir); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("content directory %q not usable: %v", c.ContentDir, err)
	}
	if err := os.MkdirAll(c.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating data dir: %w", err)
	}
	return c, nil
}
