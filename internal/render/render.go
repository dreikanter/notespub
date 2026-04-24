package render

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

type chromaPreWrapper struct{}

func (w chromaPreWrapper) Start(code bool, styleAttr string) string {
	if code {
		return fmt.Sprintf(`<pre class="chroma"%s><code>`, styleAttr)
	}
	return fmt.Sprintf(`<pre class="chroma"%s>`, styleAttr)
}

func (w chromaPreWrapper) End(code bool) string {
	if code {
		return `</code></pre>`
	}
	return `</pre>`
}

// ProcessImageFunc is called for each external image URL during rendering.
type ProcessImageFunc func(src string) (localName string, err error)

// Render converts Markdown to HTML.
// noteIndex maps note IDs to their public paths (for link resolution).
// processImage is called for external image URLs (may be nil).
func Render(source string, noteIndex map[int]string, processImage ProcessImageFunc) ([]byte, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("monokailight"),
				highlighting.WithGuessLanguage(true),
				highlighting.WithCSSWriter(nil),
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(true),
					chromahtml.WithPreWrapper(chromaPreWrapper{}),
				),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)

	sourceBytes := []byte(source)
	reader := text.NewReader(sourceBytes)
	doc := md.Parser().Parse(reader)

	// Walk AST and transform links and images.
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := n.(type) {
		case *ast.Link:
			dest := string(node.Destination)
			if isNoteID(dest) && noteIndex != nil {
				id, err := strconv.Atoi(dest)
				if err == nil {
					if resolved, ok := noteIndex[id]; ok {
						node.Destination = []byte("/" + resolved)
					}
				}
			}

		case *ast.Image:
			src := string(node.Destination)
			if processImage != nil && isExternalURL(src) {
				localName, err := processImage(src)
				if err != nil {
					return ast.WalkContinue, nil
				}
				node.Destination = []byte(localName)
			}
		}

		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, sourceBytes, doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var noteIDPattern = regexp.MustCompile(`^\d+$`)

func isNoteID(s string) bool {
	return noteIDPattern.MatchString(s)
}

func isExternalURL(s string) bool {
	return len(s) > 8 && (s[:8] == "https://" || s[:7] == "http://")
}

// HighlightCSS returns the Chroma CSS for the friendly style, scoped to .chroma.
func HighlightCSS() string {
	var buf bytes.Buffer
	formatter := chromahtml.New(chromahtml.WithClasses(true), chromahtml.WithCSSComments(false))
	style := styles.Get("monokailight")
	_ = formatter.WriteCSS(&buf, style)
	return buf.String()
}
