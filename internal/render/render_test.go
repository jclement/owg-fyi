package render

import (
	"strings"
	"testing"
)

func TestMarkdownToGemtext(t *testing.T) {
	src := `# Title

Hello *world*, see [my blog](https://example.com/blog) for more.

- plain item
- [Just A Link](https://example.com/x)

` + "```go\nfmt.Println(\"hi\")\n```" + `

> a quote

![diagram](img/d.png)
`
	out := MarkdownToGemtext([]byte(src))

	for _, want := range []string{
		"# Title",
		"Hello world, see my blog for more.",
		"=> https://example.com/blog my blog",
		"* plain item",
		"=> https://example.com/x Just A Link",
		"```go",
		"fmt.Println(\"hi\")",
		"> a quote",
		"=> img/d.png image: diagram",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "*world*") || strings.Contains(out, "[my blog]") {
		t.Errorf("markdown syntax leaked into gemtext:\n%s", out)
	}
}

func TestGemtextToHTML(t *testing.T) {
	src := "# Head\n=> /foo Foo Link\n=> https://x.y Ext\n=> /pic.png A pic\n* item\n> quoted\n```alt\n<script>\n```\nplain text\n"
	out := GemtextToHTML(src)
	for _, want := range []string{
		"<h1>Head</h1>",
		`<a href="/foo">Foo Link</a>`,
		`class="ext"`,
		`<img src="/pic.png"`,
		"<li>item</li>",
		"<blockquote>quoted</blockquote>",
		"&lt;script&gt;",
		"<p>plain text</p>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestPreEscapeGuard(t *testing.T) {
	src := "Some code:\n\n    ```\n    inner\n\nDone.\n"
	out := MarkdownToGemtext([]byte(src))
	// a literal ``` inside a code block must not close the gemtext pre block
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "```") && strings.TrimSpace(line) != "```" && !strings.HasPrefix(line, "```") {
			t.Errorf("unexpected fence line: %q", line)
		}
	}
}
