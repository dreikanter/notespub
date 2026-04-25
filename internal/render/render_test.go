package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderBasicMarkdown(t *testing.T) {
	md := "# Hello\n\nThis is a **test**.\n"
	html, err := Render(md, nil, nil)
	require.NoError(t, err)

	assert.Contains(t, string(html), "<h1")
	assert.Contains(t, string(html), "<strong>test</strong>")
}

func TestRenderFencedCode(t *testing.T) {
	md := "```go\nfmt.Println(\"hello\")\n```\n"
	html, err := Render(md, nil, nil)
	require.NoError(t, err)

	assert.Contains(t, string(html), "Println")
}

func TestRenderNoteLink(t *testing.T) {
	noteIndex := map[int]string{
		8823: "20260106_8823/some-slug",
	}
	md := "See [this note](8823)\n"
	html, err := Render(md, noteIndex, nil)
	require.NoError(t, err)

	assert.Contains(t, string(html), `href="/20260106_8823/some-slug"`)
}

func TestRenderNoteLinkUnresolved(t *testing.T) {
	md := "See [this note](9999)\n"
	html, err := Render(md, nil, nil)
	require.NoError(t, err)

	assert.Contains(t, string(html), `href="9999"`)
}

func TestRenderAutolink(t *testing.T) {
	md := "Visit https://example.com for info.\n"
	html, err := Render(md, nil, nil)
	require.NoError(t, err)

	assert.Contains(t, string(html), `href="https://example.com"`)
}

func TestRenderStrikethrough(t *testing.T) {
	md := "This is ~~deleted~~ text.\n"
	html, err := Render(md, nil, nil)
	require.NoError(t, err)

	assert.Contains(t, string(html), "<del>")
}

func TestRenderTable(t *testing.T) {
	md := "| A | B |\n|---|---|\n| 1 | 2 |\n"
	html, err := Render(md, nil, nil)
	require.NoError(t, err)

	assert.Contains(t, string(html), "<table>")
}

func TestRenderImageCallback(t *testing.T) {
	called := false
	processImage := func(src string) (localName string, err error) {
		called = true
		return "abc123.jpg", nil
	}
	md := "![alt text](https://example.com/img.jpg)\n"
	html, err := Render(md, nil, processImage)
	require.NoError(t, err)

	assert.True(t, called)
	assert.Contains(t, string(html), `src="abc123.jpg"`)
}
