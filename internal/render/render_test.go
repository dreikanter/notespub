package render

import (
	"strings"
	"testing"
)

func TestRenderBasicMarkdown(t *testing.T) {
	md := "# Hello\n\nThis is a **test**.\n"
	html, err := Render(md, nil, nil)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if !strings.Contains(string(html), "<h1") {
		t.Errorf("expected <h1>, got: %s", html)
	}
	if !strings.Contains(string(html), "<strong>test</strong>") {
		t.Errorf("expected <strong>, got: %s", html)
	}
}

func TestRenderFencedCode(t *testing.T) {
	md := "```go\nfmt.Println(\"hello\")\n```\n"
	html, err := Render(md, nil, nil)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if !strings.Contains(string(html), "Println") {
		t.Errorf("expected code content, got: %s", html)
	}
}

func TestRenderNoteLink(t *testing.T) {
	noteIndex := map[int]string{
		8823: "20260106_8823/some-slug",
	}
	md := "See [this note](8823)\n"
	html, err := Render(md, noteIndex, nil)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if !strings.Contains(string(html), `href="/20260106_8823/some-slug"`) {
		t.Errorf("expected resolved link, got: %s", html)
	}
}

func TestRenderNoteLinkUnresolved(t *testing.T) {
	md := "See [this note](9999)\n"
	html, err := Render(md, nil, nil)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if !strings.Contains(string(html), `href="9999"`) {
		t.Errorf("expected unresolved link left as-is, got: %s", html)
	}
}

func TestRenderAutolink(t *testing.T) {
	md := "Visit https://example.com for info.\n"
	html, err := Render(md, nil, nil)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if !strings.Contains(string(html), `href="https://example.com"`) {
		t.Errorf("expected autolink, got: %s", html)
	}
}

func TestRenderStrikethrough(t *testing.T) {
	md := "This is ~~deleted~~ text.\n"
	html, err := Render(md, nil, nil)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if !strings.Contains(string(html), "<del>") {
		t.Errorf("expected <del>, got: %s", html)
	}
}

func TestRenderTable(t *testing.T) {
	md := "| A | B |\n|---|---|\n| 1 | 2 |\n"
	html, err := Render(md, nil, nil)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if !strings.Contains(string(html), "<table>") {
		t.Errorf("expected <table>, got: %s", html)
	}
}

func TestRenderImageCallback(t *testing.T) {
	called := false
	processImage := func(src string) (localName string, err error) {
		called = true
		return "abc123.jpg", nil
	}
	md := "![alt text](https://example.com/img.jpg)\n"
	html, err := Render(md, nil, processImage)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if !called {
		t.Error("processImage was not called")
	}
	if !strings.Contains(string(html), `src="abc123.jpg"`) {
		t.Errorf("expected rewritten image src, got: %s", html)
	}
}
