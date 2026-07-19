# owg.fyi — capsule

The small-web home of OneWheelGeek, served over **Gemini** (`gemini://owg.fyi`)
and **the web** (`https://owg.fyi`) by one little Go binary.

```
 ██████╗ ██╗    ██╗ ██████╗     ███████╗██╗   ██╗██╗
██╔═══██╗██║    ██║██╔════╝     ██╔════╝╚██╗ ██╔╝██║
██║   ██║██║ █╗ ██║██║  ███╗    █████╗   ╚████╔╝ ██║
██║   ██║██║███╗██║██║   ██║    ██╔══╝    ╚██╔╝  ██║
╚██████╔╝╚███╔███╔╝╚██████╔╝ ██╗██║        ██║   ██║
 ╚═════╝  ╚══╝╚══╝  ╚═════╝  ╚═╝╚═╝        ╚═╝   ╚═╝
```

## What it does

* **One content tree, two protocols.** Pages are gemtext (`.gmi`) or markdown
  (`.md`). Markdown renders to HTML on the web and converts to idiomatic
  gemtext on Gemini (inline links hoisted to `=>` lines, tables to preformat
  blocks). Gemtext renders natively on Gemini and to clean HTML on the web.
* **TLS everywhere.** Let's Encrypt (autocert) on :443 with :80 handling ACME
  + redirects; a persistent self-signed cert on :1965 (Gemini clients do TOFU).
* **Inherited chrome.** The nearest `_header.gmi|md` / `_footer.gmi|md` up the
  directory tree wraps every page.
* **Directives.**
  * `{{index}}` / `{{index /posts}}` / `{{index /posts 5}}` — directory
    listing, gemfeed-style (`YYYY-MM-DD Title`, newest first).
  * `{{include /path/file.md}}` — server-side include.
* **Search** on both protocols: `/search` (status 10 input on Gemini, a form
  on the web).
* **Atom feed** for the gemlog at `/posts/feed.xml`; the Gemini side is a
  standard gemfeed.
* **Titan uploads** (optional, off by default): allowlisted client-cert
  fingerprints only (`CAPSULE_TITAN=true`, `CAPSULE_TITAN_CERTS=<sha256,…>`).
* Colored logs, docker healthcheck subcommand (`capsule health`).

## Local development

```bash
mise run dev     # web on http://localhost:8080, gemini://localhost:1965
mise run test    # go test ./...
mise run docker  # build the image locally
```

Content lives in `content/`. Files/dirs starting with `_` or `.` are never
served or listed. Posts are `content/posts/YYYY-MM-DD-slug.md` — the date
prefix (or front matter `date:`) drives sorting and the feed.

## Deploying

Push to `main`. GitHub Actions builds `ghcr.io/jclement/owg-fyi:latest`;
[PullPilot](https://github.com/jclement/pullpilot) on the server notices,
soaks it for 10 minutes, rolls it out, and rolls back if the healthcheck
fails. Manual deploy: `ssh root@owg.fyi 'cd /opt/owg && docker compose pull && docker compose up -d'`.

The production stack (`docker-compose.yml`) runs at `/opt/owg` on owg.fyi.
