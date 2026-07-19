// Package search implements a small full-site search over the content tree.
// The corpus is tiny (a personal site), so it keeps every page in memory and
// scores with a straightforward term match; the index refreshes lazily.
package search

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jclement/owg-fyi/internal/content"
)

type doc struct {
	URL   string
	Title string
	Date  string
	Text  string // lowercased plain-ish text
}

// Hit is one search result.
type Hit struct {
	URL     string
	Title   string
	Date    string
	Snippet string
	Score   int
}

// Index is a lazily-refreshed in-memory search index.
type Index struct {
	store *content.Store

	mu      sync.Mutex
	docs    []doc
	builtAt time.Time
	ttl     time.Duration
}

func New(store *content.Store, ttl time.Duration) *Index {
	return &Index{store: store, ttl: ttl}
}

var markupRe = regexp.MustCompile("(?m)^(=>\\s*\\S+|```.*|[#>*]+\\s*)|[`*_\\[\\]()!]")

func (ix *Index) build() {
	var docs []doc
	root := ix.store.Root
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".gmi" && ext != ".md" {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		url := "/" + filepath.ToSlash(rel)
		url = strings.TrimSuffix(url, ext)
		if strings.HasSuffix(url, "/index") {
			url = strings.TrimSuffix(url, "index")
		}
		res, err := ix.store.Open(url)
		if err != nil || res.Type != content.PageResult {
			return nil
		}
		// index only the body chunk text (skip shared header/footer noise)
		var body string
		if n := len(res.Page.Chunks); n == 3 {
			body = res.Page.Chunks[1].Src
		} else if n > 0 {
			body = res.Page.Chunks[0].Src
			for _, c := range res.Page.Chunks[1:] {
				body += "\n" + c.Src
			}
		}
		plain := markupRe.ReplaceAllString(body, " ")
		docs = append(docs, doc{
			URL:   url,
			Title: res.Page.Title,
			Date:  res.Page.Date,
			Text:  strings.ToLower(plain),
		})
		return nil
	})
	ix.docs = docs
	ix.builtAt = time.Now()
}

// Search returns scored hits for a free-text query.
func (ix *Index) Search(query string, limit int) []Hit {
	ix.mu.Lock()
	if time.Since(ix.builtAt) > ix.ttl || ix.docs == nil {
		ix.build()
	}
	docs := ix.docs
	ix.mu.Unlock()

	terms := tokenize(query)
	if len(terms) == 0 {
		return nil
	}
	var hits []Hit
	for _, d := range docs {
		score := 0
		lt := strings.ToLower(d.Title)
		matchedAll := true
		firstPos := -1
		for _, t := range terms {
			tScore := 0
			if strings.Contains(lt, t) {
				tScore += 10
			}
			if n := strings.Count(d.Text, t); n > 0 {
				if c := n; c > 5 {
					tScore += 5
				} else {
					tScore += c
				}
				if firstPos < 0 {
					firstPos = strings.Index(d.Text, t)
				}
			}
			if tScore == 0 {
				matchedAll = false
				break
			}
			score += tScore
		}
		if !matchedAll || score == 0 {
			continue
		}
		hits = append(hits, Hit{
			URL:     d.URL,
			Title:   titleOr(d.Title, d.URL),
			Date:    d.Date,
			Snippet: snippet(d.Text, firstPos),
			Score:   score,
		})
	}
	sortHits(hits)
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

func titleOr(t, fallback string) string {
	if t != "" {
		return t
	}
	return fallback
}

func tokenize(q string) []string {
	var out []string
	for _, f := range strings.Fields(strings.ToLower(q)) {
		f = strings.Trim(f, `"'.,;:!?`)
		if len(f) >= 2 {
			out = append(out, f)
		}
	}
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

func snippet(text string, pos int) string {
	if pos < 0 {
		pos = 0
	}
	start := pos - 60
	if start < 0 {
		start = 0
	}
	end := pos + 100
	if end > len(text) {
		end = len(text)
	}
	s := strings.Join(strings.Fields(text[start:end]), " ")
	if len(s) > 140 {
		s = s[:140]
	}
	return strings.TrimSpace(s)
}

func sortHits(hits []Hit) {
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].Score > hits[j-1].Score; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
}
