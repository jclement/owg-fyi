package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"_header.gmi":              "=> / home\n",
		"_footer.md":               "---\n\nfooter here\n",
		"index.gmi":                "# Welcome\n\nhello\n\n{{index /posts 2}}\n",
		"about.md":                 "# About\n\nSome *markdown*.\n",
		"posts/index.gmi":          "# Gemlog\n\n{{index}}\n",
		"posts/_header.gmi":        "=> /posts/ gemlog\n",
		"posts/2024-01-02-b.md":    "# Beta Post\n\nbody b\n",
		"posts/2024-03-04-a.md":    "---\ntitle: Alpha Post\ndate: 2024-03-04\n---\n\n# ignored\n\nbody a\n",
		"posts/2023-01-01-old.gmi": "# Old One\n\nold\n",
		"stuff/readme.txt":         "plain text\n",
		"inc.md":                   "# Inc\n\n{{include /_snip.md}}\n",
		"_snip.md":                 "snippet-content\n",
		"tagged.gmi":               "# Tagged\n\n{{random /_tags.txt}}\n\nbuild {{version}}\n",
		"_tags.txt":                "only-tagline\n",
	}
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return NewStore(root)
}

func open(t *testing.T, s *Store, path string) *Result {
	t.Helper()
	res, err := s.Open(path)
	if err != nil {
		t.Fatalf("Open(%s): %v", path, err)
	}
	return res
}

func TestResolveAndAssemble(t *testing.T) {
	s := testStore(t)

	res := open(t, s, "/")
	if res.Type != PageResult {
		t.Fatalf("/ not a page: %v", res.Type)
	}
	gt := res.Page.Gemtext()
	if !strings.Contains(gt, "# Welcome") || !strings.Contains(gt, "=> / home") || !strings.Contains(gt, "footer here") {
		t.Errorf("header/body/footer assembly wrong:\n%s", gt)
	}
	// {{index /posts 2}} → two newest dated posts only
	if !strings.Contains(gt, "=> /posts/2024-03-04-a 2024-03-04 Alpha Post") {
		t.Errorf("missing newest post link:\n%s", gt)
	}
	if !strings.Contains(gt, "2024-01-02 Beta Post") {
		t.Errorf("missing second post:\n%s", gt)
	}
	if strings.Contains(gt, "Old One") {
		t.Errorf("limit 2 not applied:\n%s", gt)
	}
	if res.Page.Title != "Welcome" {
		t.Errorf("title = %q", res.Page.Title)
	}
}

func TestExtensionlessAndRedirect(t *testing.T) {
	s := testStore(t)
	if res := open(t, s, "/about"); res.Type != PageResult {
		t.Errorf("/about should resolve .md page")
	}
	res := open(t, s, "/posts")
	if res.Type != RedirectResult || res.Location != "/posts/" {
		t.Errorf("dir without slash should redirect, got %+v", res)
	}
	if res := open(t, s, "/nope"); res.Type != NotFound {
		t.Errorf("missing page should be NotFound")
	}
}

func TestHiddenFilesRejected(t *testing.T) {
	s := testStore(t)
	for _, p := range []string{"/_header.gmi", "/_snip.md", "/../etc/passwd", "/posts/_header"} {
		if res := open(t, s, p); res.Type != NotFound {
			t.Errorf("%s should be NotFound, got %v", p, res.Type)
		}
	}
}

func TestNearestHeaderWins(t *testing.T) {
	s := testStore(t)
	res := open(t, s, "/posts/2024-01-02-b")
	gt := res.Page.Gemtext()
	if !strings.Contains(gt, "=> /posts/ gemlog") {
		t.Errorf("posts header should win:\n%s", gt)
	}
	if strings.Contains(gt, "=> / home") {
		t.Errorf("root header should be shadowed:\n%s", gt)
	}
	if res.Page.Date != "2024-01-02" {
		t.Errorf("date from filename = %q", res.Page.Date)
	}
}

func TestIndexDirectiveInDirIndex(t *testing.T) {
	s := testStore(t)
	// {{index}} inside posts/index.gmi must list /posts, not the root
	res := open(t, s, "/posts/")
	gt := res.Page.Gemtext()
	if !strings.Contains(gt, "=> /posts/2024-03-04-a") {
		t.Errorf("posts index missing own entries:\n%s", gt)
	}
	if strings.Contains(gt, "=> /about") {
		t.Errorf("posts index leaked root entries:\n%s", gt)
	}
}

func TestInclude(t *testing.T) {
	s := testStore(t)
	res := open(t, s, "/inc")
	if gt := res.Page.Gemtext(); !strings.Contains(gt, "snippet-content") {
		t.Errorf("include failed:\n%s", gt)
	}
}

func TestRandomAndVersion(t *testing.T) {
	s := testStore(t)
	res := open(t, s, "/tagged")
	gt := res.Page.Gemtext()
	if !strings.Contains(gt, "only-tagline") {
		t.Errorf("random directive failed:\n%s", gt)
	}
	if !strings.Contains(gt, "build dev") {
		t.Errorf("inline version directive failed:\n%s", gt)
	}
}

type fakeBumper struct{ calls map[string]uint64 }

func (f *fakeBumper) Bump(p string) uint64 { f.calls[p]++; return f.calls[p] }

func TestCounterKeyedByPage(t *testing.T) {
	s := testStore(t)
	fb := &fakeBumper{calls: map[string]uint64{}}
	s.Counter = fb
	if err := os.WriteFile(filepath.Join(s.Root, "_footer.md"), []byte("views: {{counter}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := open(t, s, "/about")
	if gt := res.Page.Gemtext(); !strings.Contains(gt, "views: 000001") {
		t.Errorf("counter not rendered:\n%s", gt)
	}
	open(t, s, "/about")
	if fb.calls["/about"] != 2 {
		t.Errorf("counter keyed wrong: %v", fb.calls)
	}
	open(t, s, "/posts/")
	if fb.calls["/posts"] != 1 {
		t.Errorf("dir page key wrong: %v", fb.calls)
	}
}

func TestStatic(t *testing.T) {
	s := testStore(t)
	res := open(t, s, "/stuff/readme.txt")
	if res.Type != StaticResult || !strings.HasPrefix(res.Mime, "text/plain") {
		t.Errorf("static resolve failed: %+v", res)
	}
}

func TestHTMLRender(t *testing.T) {
	s := testStore(t)
	res := open(t, s, "/about")
	h, err := res.Page.HTML()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h, "<em>markdown</em>") {
		t.Errorf("markdown not rendered:\n%s", h)
	}
	if !strings.Contains(h, `href="/"`) {
		t.Errorf("gemtext header not rendered to html:\n%s", h)
	}
}
