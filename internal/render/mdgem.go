package render

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// mdPlain parses without the Typographer so no HTML entities (&rsquo; etc.)
// leak into plain gemtext output.
var mdPlain = goldmark.New(goldmark.WithExtensions(extension.GFM))

// MarkdownToGemtext converts markdown to idiomatic gemtext: inline links are
// kept as their label text and hoisted to `=>` link lines after the block,
// images become link lines, tables become preformatted blocks.
func MarkdownToGemtext(src []byte) string {
	doc := mdPlain.Parser().Parse(text.NewReader(src))
	var b strings.Builder
	renderBlockChildren(&b, doc, src, 0)
	out := strings.TrimRight(b.String(), "\n") + "\n"
	// collapse 3+ blank lines
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	return out
}

type pendingLink struct{ url, label string }

func renderBlockChildren(b *strings.Builder, n ast.Node, src []byte, depth int) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		renderBlock(b, c, src, depth)
	}
}

func renderBlock(b *strings.Builder, n ast.Node, src []byte, depth int) {
	switch v := n.(type) {
	case *ast.Heading:
		txt, links := inlineText(v, src)
		level := v.Level
		if level > 3 {
			level = 3
		}
		fmt.Fprintf(b, "%s %s\n\n", strings.Repeat("#", level), txt)
		writeLinks(b, links)
	case *ast.Paragraph, *ast.TextBlock:
		txt, links := inlineText(n, src)
		if strings.TrimSpace(txt) != "" {
			b.WriteString(txt + "\n")
		}
		writeLinks(b, links)
		b.WriteString("\n")
	case *ast.Blockquote:
		var inner strings.Builder
		renderBlockChildren(&inner, v, src, depth)
		for _, line := range strings.Split(strings.TrimRight(inner.String(), "\n"), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			b.WriteString("> " + line + "\n")
		}
		b.WriteString("\n")
	case *ast.List:
		for li := v.FirstChild(); li != nil; li = li.NextSibling() {
			txt, links := listItemText(li, src)
			// an item that is nothing but a single link becomes a link line
			if txt == "" && len(links) == 1 {
				fmt.Fprintf(b, "=> %s %s\n", links[0].url, links[0].label)
				continue
			}
			fmt.Fprintf(b, "* %s\n", txt)
			writeLinks(b, links)
		}
		b.WriteString("\n")
	case *ast.FencedCodeBlock:
		lang := string(v.Language(src))
		fmt.Fprintf(b, "```%s\n", lang)
		writeCodeLines(b, v, src)
		b.WriteString("```\n\n")
	case *ast.CodeBlock:
		b.WriteString("```\n")
		writeCodeLines(b, v, src)
		b.WriteString("```\n\n")
	case *ast.ThematicBreak:
		b.WriteString("\n")
	case *ast.HTMLBlock:
		// drop raw HTML on gemini
	case *east.Table:
		b.WriteString("```table\n")
		for row := v.FirstChild(); row != nil; row = row.NextSibling() {
			var cells []string
			for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
				t, _ := inlineText(cell, src)
				cells = append(cells, t)
			}
			b.WriteString(strings.Join(cells, " | ") + "\n")
		}
		b.WriteString("```\n\n")
	default:
		if n.Type() == ast.TypeBlock {
			renderBlockChildren(b, n, src, depth)
		}
	}
}

func writeCodeLines(b *strings.Builder, n ast.Node, src []byte) {
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		line := string(seg.Value(src))
		// guard: a ``` inside a pre block would break framing
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			line = " " + line
		}
		b.WriteString(strings.TrimRight(line, "\n") + "\n")
	}
}

func writeLinks(b *strings.Builder, links []pendingLink) {
	for _, l := range links {
		fmt.Fprintf(b, "=> %s %s\n", l.url, l.label)
	}
}

// inlineText flattens the inline content of a block into plain text and
// collects links/images encountered along the way.
func inlineText(n ast.Node, src []byte) (string, []pendingLink) {
	var b strings.Builder
	var links []pendingLink
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			switch v := c.(type) {
			case *ast.Text:
				b.Write(v.Segment.Value(src))
				if v.SoftLineBreak() || v.HardLineBreak() {
					b.WriteString(" ")
				}
			case *ast.String:
				b.Write(v.Value)
			case *ast.CodeSpan:
				var cs strings.Builder
				for cc := v.FirstChild(); cc != nil; cc = cc.NextSibling() {
					if t, ok := cc.(*ast.Text); ok {
						cs.Write(t.Segment.Value(src))
					}
				}
				b.WriteString(cs.String())
			case *ast.Link:
				label, sub := inlineText(v, src)
				label = strings.TrimSpace(label)
				if label == "" {
					label = string(v.Destination)
				}
				b.WriteString(label)
				links = append(links, pendingLink{url: string(v.Destination), label: label})
				links = append(links, sub...)
			case *ast.Image:
				alt, _ := inlineText(v, src)
				alt = strings.TrimSpace(alt)
				if alt == "" {
					alt = "image: " + string(v.Destination)
				} else {
					alt = "image: " + alt
				}
				links = append(links, pendingLink{url: string(v.Destination), label: alt})
			case *ast.AutoLink:
				url := string(v.URL(src))
				b.WriteString(url)
				links = append(links, pendingLink{url: url, label: url})
			case *ast.RawHTML:
				// skip
			default:
				walk(c)
			}
		}
	}
	walk(n)
	return strings.TrimSpace(b.String()), links
}

// listItemText renders a list item's first-level content as a single line.
func listItemText(li ast.Node, src []byte) (string, []pendingLink) {
	var parts []string
	var links []pendingLink
	for c := li.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.(type) {
		case *ast.List:
			// nested list: flatten one level with a dash prefix
			for nli := c.FirstChild(); nli != nil; nli = nli.NextSibling() {
				t, l := listItemText(nli, src)
				parts = append(parts, "— "+t)
				links = append(links, l...)
			}
		default:
			t, l := inlineText(c, src)
			if t != "" {
				parts = append(parts, t)
			}
			links = append(links, l...)
		}
	}
	return strings.Join(parts, " "), links
}
