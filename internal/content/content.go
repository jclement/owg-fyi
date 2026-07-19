// Package content resolves URL paths against the content tree, assembles
// pages (inherited _header/_footer, {{index}}/{{include}} directives) and
// renders them to gemtext or HTML.
package content

import (
	"fmt"
	"math/rand/v2"
	"mime"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jclement/owg-fyi/internal/gemtext"
	"github.com/jclement/owg-fyi/internal/render"
)

// Format identifies a source document format.
type Format int

const (
	FormatGem Format = iota
	FormatMD
)

// ResultType describes what a URL path resolved to.
type ResultType int

const (
	NotFound ResultType = iota
	PageResult
	StaticResult
	RedirectResult
)

// Chunk is a piece of page source in a known format.
type Chunk struct {
	Src string
	Fmt Format
}

// Page is an assembled page: inherited header, body, inherited footer.
type Page struct {
	URLPath string
	Title   string
	Date    string // YYYY-MM-DD if known
	Chunks  []Chunk
}

// Result is the outcome of resolving a URL path.
type Result struct {
	Type     ResultType
	Page     *Page
	FilePath string // StaticResult: file to stream
	Mime     string // StaticResult
	Location string // RedirectResult
}

// Bumper increments and returns a per-page hit count ({{counter}}).
type Bumper interface {
	Bump(page string) uint64
}

// Store serves content from a root directory.
type Store struct {
	Root    string
	Counter Bumper // nil renders {{counter}} as 000000
}

func NewStore(root string) *Store {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return &Store{Root: abs}
}

// hidden reports whether a name must never be served or listed.
func hidden(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

// Clean validates and normalizes a URL path; ok=false means reject.
func Clean(urlPath string) (string, bool) {
	if urlPath == "" {
		urlPath = "/"
	}
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}
	cleaned := path.Clean(urlPath)
	if strings.Contains(cleaned, "..") {
		return "", false
	}
	for _, seg := range strings.Split(cleaned, "/") {
		if hidden(seg) && seg != "" {
			return "", false
		}
	}
	// preserve trailing slash (path.Clean strips it)
	if strings.HasSuffix(urlPath, "/") && cleaned != "/" {
		cleaned += "/"
	}
	return cleaned, true
}

// Open resolves a URL path to a page, static file, or redirect.
func (s *Store) Open(urlPath string) (*Result, error) {
	cleaned, ok := Clean(urlPath)
	if !ok {
		return &Result{Type: NotFound}, nil
	}
	rel := strings.TrimPrefix(cleaned, "/")
	fsPath := filepath.Join(s.Root, filepath.FromSlash(strings.TrimSuffix(rel, "/")))

	if st, err := os.Stat(fsPath); err == nil {
		if st.IsDir() {
			if !strings.HasSuffix(cleaned, "/") && cleaned != "/" {
				return &Result{Type: RedirectResult, Location: cleaned + "/"}, nil
			}
			for _, idx := range []string{"index.gmi", "index.md"} {
				if p := filepath.Join(fsPath, idx); exists(p) {
					return s.page(cleaned, p)
				}
			}
			// directory with no index: synthesize a listing page
			return s.syntheticIndex(cleaned, fsPath)
		}
		switch strings.ToLower(filepath.Ext(fsPath)) {
		case ".gmi", ".md":
			return s.page(cleaned, fsPath)
		}
		return &Result{Type: StaticResult, FilePath: fsPath, Mime: mimeFor(fsPath)}, nil
	}

	// extensionless page: try .gmi then .md
	if !strings.HasSuffix(cleaned, "/") {
		for _, ext := range []string{".gmi", ".md"} {
			if p := fsPath + ext; exists(p) {
				return s.page(cleaned, p)
			}
		}
	}
	return &Result{Type: NotFound}, nil
}

func exists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func mimeFor(p string) string {
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".gmi":
		return "text/gemini; charset=utf-8"
	case ".txt", ".asc", ".key":
		return "text/plain; charset=utf-8"
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return "application/octet-stream"
}

func formatOf(p string) Format {
	if strings.HasSuffix(strings.ToLower(p), ".md") {
		return FormatMD
	}
	return FormatGem
}

// ---- front matter -------------------------------------------------------

var fmKeyRe = regexp.MustCompile(`(?m)^(title|date)\s*[:=]\s*(.+)$`)

// stripFrontMatter removes a leading --- ... --- block, returning body and
// any title/date it declared.
func stripFrontMatter(src string) (body, title, date string) {
	body = src
	if !strings.HasPrefix(src, "---\n") && !strings.HasPrefix(src, "---\r\n") {
		return body, "", ""
	}
	rest := src[strings.Index(src, "\n")+1:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return body, "", ""
	}
	fm := rest[:end]
	body = rest[end+4:]
	body = strings.TrimPrefix(strings.TrimPrefix(body, "\r"), "\n")
	for _, m := range fmKeyRe.FindAllStringSubmatch(fm, -1) {
		val := strings.Trim(strings.TrimSpace(m[2]), `"'`)
		switch m[1] {
		case "title":
			title = val
		case "date":
			if len(val) >= 10 {
				val = val[:10]
			}
			date = val
		}
	}
	return body, title, date
}

// ---- page assembly ------------------------------------------------------

func (s *Store) page(urlPath, fsPath string) (*Result, error) {
	raw, err := os.ReadFile(fsPath)
	if err != nil {
		return nil, err
	}
	body, fmTitle, fmDate := stripFrontMatter(string(raw))
	fmt2 := formatOf(fsPath)
	// base dir for relative directives: the page's own directory (a
	// trailing-slash URL is a directory index, so that IS the directory)
	baseDir := path.Dir(urlPath)
	if strings.HasSuffix(urlPath, "/") {
		baseDir = path.Clean(urlPath)
	}
	body = s.expandDirectives(body, fmt2, baseDir, 0)

	pg := &Page{URLPath: urlPath, Chunks: []Chunk{}}
	if h := s.nearestAffix(fsPath, "_header"); h != nil {
		pg.Chunks = append(pg.Chunks, *h)
	}
	pg.Chunks = append(pg.Chunks, Chunk{Src: body, Fmt: fmt2})
	if f := s.nearestAffix(fsPath, "_footer"); f != nil {
		pg.Chunks = append(pg.Chunks, *f)
	}

	// {{counter}}: one hit counter per page, shown wherever the token sits
	// (typically the inherited footer) — keyed by the page, not the footer
	const counterToken = "{{counter}}"
	for i := range pg.Chunks {
		if !strings.Contains(pg.Chunks[i].Src, counterToken) {
			continue
		}
		key := strings.TrimSuffix(urlPath, "/")
		if key == "" {
			key = "/"
		}
		val := "000000"
		if s.Counter != nil {
			val = fmt.Sprintf("%06d", s.Counter.Bump(key))
		}
		pg.Chunks[i].Src = strings.ReplaceAll(pg.Chunks[i].Src, counterToken, val)
	}

	pg.Title = fmTitle
	if pg.Title == "" {
		pg.Title = titleOf(body, fmt2)
	}
	pg.Date = fmDate
	if pg.Date == "" {
		pg.Date = dateFromName(filepath.Base(fsPath))
	}
	return &Result{Type: PageResult, Page: pg}, nil
}

// nearestAffix finds the closest _header/_footer (.gmi or .md) at or above
// the file's directory, stopping at the content root.
func (s *Store) nearestAffix(fsPath, base string) *Chunk {
	dir := filepath.Dir(fsPath)
	for {
		for _, ext := range []string{".gmi", ".md"} {
			p := filepath.Join(dir, base+ext)
			if exists(p) {
				raw, err := os.ReadFile(p)
				if err == nil {
					body, _, _ := stripFrontMatter(string(raw))
					urlDir := "/" + filepath.ToSlash(mustRel(s.Root, dir))
					if urlDir == "/." {
						urlDir = "/"
					}
					body = s.expandDirectives(body, formatOf(p), urlDir, 0)
					return &Chunk{Src: body, Fmt: formatOf(p)}
				}
			}
		}
		if dir == s.Root || len(dir) <= len(s.Root) {
			return nil
		}
		dir = filepath.Dir(dir)
	}
}

func mustRel(base, target string) string {
	r, err := filepath.Rel(base, target)
	if err != nil {
		return "."
	}
	return r
}

func titleOf(body string, f Format) string {
	if f == FormatGem {
		return gemtext.FirstHeading(body)
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:])
		}
	}
	return ""
}

var dateNameRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})[-_]`)

func dateFromName(name string) string {
	if m := dateNameRe.FindStringSubmatch(name); m != nil {
		return m[1]
	}
	return ""
}

// ---- directives ---------------------------------------------------------

// BuildSHA / BuildDate are stamped by the Dockerfile via -ldflags -X; the
// {{version}} directive renders them (e.g. in a footer).
var (
	BuildSHA  = ""
	BuildDate = ""
)

func versionString() string {
	sha := BuildSHA
	if len(sha) > 7 {
		sha = sha[:7]
	}
	switch {
	case sha != "" && BuildDate != "":
		return sha + " · " + BuildDate
	case sha != "":
		return sha
	default:
		return "dev"
	}
}

var directiveRe = regexp.MustCompile(`(?m)^\{\{\s*(index|include|random)(?:\s+([^\s}]+))?(?:\s+(\d+))?\s*\}\}\s*$`)

const maxIncludeDepth = 4

// expandDirectives replaces {{index [path] [limit]}} and {{include path}}
// lines. baseDir is the URL directory of the containing document.
func (s *Store) expandDirectives(body string, f Format, baseDir string, depth int) string {
	if depth > maxIncludeDepth {
		return body
	}
	// {{version}} works inline (mid-sentence), unlike the line directives
	body = strings.ReplaceAll(body, "{{version}}", versionString())
	return directiveRe.ReplaceAllStringFunc(body, func(m string) string {
		parts := directiveRe.FindStringSubmatch(m)
		verb, arg, limStr := parts[1], parts[2], parts[3]
		switch verb {
		case "index":
			dir := baseDir
			if arg != "" {
				dir = resolveRef(baseDir, arg)
			}
			limit := 0
			if limStr != "" {
				limit, _ = strconv.Atoi(limStr)
			}
			return s.renderIndex(dir, f, limit)
		case "random":
			// pick one non-empty line from a file: {{random /_taglines.txt}}
			ref := resolveRef(baseDir, arg)
			raw, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(strings.TrimPrefix(ref, "/"))))
			if err != nil {
				return ""
			}
			var lines []string
			for _, l := range strings.Split(string(raw), "\n") {
				if l = strings.TrimSpace(l); l != "" {
					lines = append(lines, l)
				}
			}
			if len(lines) == 0 {
				return ""
			}
			return lines[rand.IntN(len(lines))]
		case "include":
			ref := resolveRef(baseDir, arg)
			rel := strings.TrimPrefix(ref, "/")
			p := filepath.Join(s.Root, filepath.FromSlash(rel))
			raw, err := os.ReadFile(p)
			if err != nil {
				return fmt.Sprintf("(include %s: not found)", arg)
			}
			inner, _, _ := stripFrontMatter(string(raw))
			return s.expandDirectives(inner, f, path.Dir(ref), depth+1)
		}
		return m
	})
}

func resolveRef(baseDir, ref string) string {
	if strings.HasPrefix(ref, "/") {
		return path.Clean(ref)
	}
	return path.Clean(path.Join(baseDir, ref))
}

// Entry is one row of a directory listing.
type Entry struct {
	URL   string
	Title string
	Date  string
	IsDir bool
}

// List returns the visible entries of a content directory (non-recursive),
// dated entries first (newest first), then alphabetical.
func (s *Store) List(urlDir string) []Entry {
	rel := strings.TrimPrefix(path.Clean(urlDir), "/")
	fsDir := filepath.Join(s.Root, filepath.FromSlash(rel))
	des, err := os.ReadDir(fsDir)
	if err != nil {
		return nil
	}
	base := path.Clean(urlDir)
	if base != "/" {
		base += "/"
	} else {
		base = "/"
	}
	var out []Entry
	for _, de := range des {
		name := de.Name()
		if hidden(name) {
			continue
		}
		if de.IsDir() {
			e := Entry{URL: base + name + "/", Title: name + "/", IsDir: true}
			// use the sub-index title when there is one
			for _, idx := range []string{"index.gmi", "index.md"} {
				p := filepath.Join(fsDir, name, idx)
				if exists(p) {
					if raw, err := os.ReadFile(p); err == nil {
						body, t, _ := stripFrontMatter(string(raw))
						if t == "" {
							t = titleOf(body, formatOf(p))
						}
						if t != "" {
							e.Title = t
						}
					}
					break
				}
			}
			out = append(out, e)
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".gmi" && ext != ".md" {
			continue
		}
		stem := strings.TrimSuffix(name, ext)
		if stem == "index" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(fsDir, name))
		if err != nil {
			continue
		}
		body, t, d := stripFrontMatter(string(raw))
		if t == "" {
			t = titleOf(body, formatOf(name))
		}
		if t == "" {
			t = stem
		}
		if d == "" {
			d = dateFromName(name)
		}
		out = append(out, Entry{URL: base + stem, Title: t, Date: d})
	}
	sort.SliceStable(out, func(i, j int) bool {
		di, dj := out[i].Date, out[j].Date
		if (di != "") != (dj != "") {
			return di != "" // dated entries first
		}
		if di != dj {
			return di > dj // newest first
		}
		return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
	})
	return out
}

func (s *Store) renderIndex(urlDir string, f Format, limit int) string {
	entries := s.List(urlDir)
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	var b strings.Builder
	for _, e := range entries {
		label := e.Title
		if e.Date != "" {
			label = e.Date + " " + e.Title
		}
		if f == FormatMD {
			fmt.Fprintf(&b, "* [%s](%s)\n", label, e.URL)
		} else {
			fmt.Fprintf(&b, "=> %s %s\n", e.URL, label)
		}
	}
	if b.Len() == 0 {
		return "(nothing here yet)"
	}
	return strings.TrimRight(b.String(), "\n")
}

func (s *Store) syntheticIndex(urlPath, fsDir string) (*Result, error) {
	name := path.Base(strings.TrimSuffix(urlPath, "/"))
	if name == "/" || name == "." {
		name = "index"
	}
	src := fmt.Sprintf("# %s\n\n{{index}}\n", name)
	src = s.expandDirectives(src, FormatGem, strings.TrimSuffix(urlPath, "/"), 0)
	pg := &Page{URLPath: urlPath, Title: name, Chunks: []Chunk{}}
	if h := s.nearestAffix(filepath.Join(fsDir, "index.gmi"), "_header"); h != nil {
		pg.Chunks = append(pg.Chunks, *h)
	}
	pg.Chunks = append(pg.Chunks, Chunk{Src: src, Fmt: FormatGem})
	if f := s.nearestAffix(filepath.Join(fsDir, "index.gmi"), "_footer"); f != nil {
		pg.Chunks = append(pg.Chunks, *f)
	}
	return &Result{Type: PageResult, Page: pg}, nil
}

// ---- rendering ----------------------------------------------------------

// Gemtext renders the assembled page as a text/gemini document.
func (p *Page) Gemtext() string {
	var b strings.Builder
	for i, c := range p.Chunks {
		if i > 0 {
			b.WriteString("\n")
		}
		switch c.Fmt {
		case FormatMD:
			b.WriteString(render.MarkdownToGemtext([]byte(c.Src)))
		default:
			b.WriteString(strings.TrimRight(c.Src, "\n") + "\n")
		}
	}
	return b.String()
}

// HTML renders the assembled page as an HTML fragment.
func (p *Page) HTML() (string, error) {
	var b strings.Builder
	for _, c := range p.Chunks {
		switch c.Fmt {
		case FormatMD:
			h, err := render.MarkdownToHTML([]byte(c.Src))
			if err != nil {
				return "", err
			}
			b.WriteString(h)
		default:
			b.WriteString(render.GemtextToHTML(c.Src))
		}
	}
	return b.String(), nil
}
