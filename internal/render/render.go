package render

import (
	"bytes"
	"regexp"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

// ProcessImageFunc is called for each external image URL during rendering.
type ProcessImageFunc func(src string) (localName string, err error)

// Render converts Markdown to HTML.
// noteIndex maps note IDs to their public paths (for link resolution).
// processImage is called for external image URLs (may be nil).
func Render(source []byte, noteIndex map[string]string, processImage ProcessImageFunc) ([]byte, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("github"),
				highlighting.WithCSSWriter(nil),
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(true),
				),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)

	reader := text.NewReader(source)
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
				if resolved, ok := noteIndex[dest]; ok {
					node.Destination = []byte("/" + resolved)
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
	if err := md.Renderer().Render(&buf, source, doc); err != nil {
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

// HighlightCSS returns the Chroma CSS for the github style, scoped to .chroma.
func HighlightCSS() string {
	var buf bytes.Buffer
	formatter := chromahtml.New(chromahtml.WithClasses(true))
	style := styles.Get("github")
	_ = formatter.WriteCSS(&buf, style)
	return buf.String()
}
