package build

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	texttemplate "text/template"

	"github.com/dreikanter/notescli/note"
	"github.com/dreikanter/notespub/internal/config"
	"github.com/dreikanter/notespub/internal/images"
	"github.com/dreikanter/notespub/internal/page"
	"github.com/dreikanter/notespub/internal/render"
)

// layoutData is the top-level data passed to layout.html.
type layoutData struct {
	Config configData
	Page   pageData
	// Content is the rendered inner template (safe HTML).
	Content template.HTML
}

// configData is the subset of config passed to templates.
type configData struct {
	SiteName     string
	SiteDomain   string
	SiteRootURL  string
	SiteRootPath string
	AuthorName   string
	FeedURL      string
	FeedPath     string
	LicenseName  string
	LicenseURL   string
	HighlightCSS template.CSS
}

// pageData holds metadata for the layout template.
type pageData struct {
	Title           string
	MetaDescription string
	CanonicalPath   string
}

// noteData is the data passed to note.html inner template.
type noteData struct {
	Note    noteViewData
	Related []noteViewData
}

// noteViewData represents a note for template rendering.
type noteViewData struct {
	UID         string
	Slug        string
	Title       string
	Tags        []string
	Body        template.HTML
	PublicPath  string
	PublishedAt time.Time
}

// indexData is the data passed to index.html inner template.
type indexData struct {
	Tags      []string
	NotePages []noteViewData
}

// tagData is the data passed to tag.html inner template.
type tagData struct {
	Tags        []string
	CurrentTag  string
	TaggedPages []noteViewData
}

// feedData is the data passed to feed.xml.
type feedData struct {
	Config    configData
	NotePages []feedNoteData
}

// feedNoteData is a note for the feed template.
type feedNoteData struct {
	Title string
	URL   string
	UID   string
	Body  string
}

// cleanBuildDir removes all non-dotfile entries from the build directory,
// preserving dotfiles and dotdirs (e.g. .git, .nojekyll).
func cleanBuildDir(buildPath string) error {
	abs, err := filepath.Abs(buildPath)
	if err != nil {
		return fmt.Errorf("resolving build path: %w", err)
	}

	home, _ := os.UserHomeDir()
	if abs == "/" || abs == home {
		return fmt.Errorf("refusing to clean dangerous build path: %s", abs)
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading build dir: %w", err)
	}

	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if err := os.RemoveAll(filepath.Join(abs, e.Name())); err != nil {
			return fmt.Errorf("removing %s: %w", e.Name(), err)
		}
	}
	return nil
}

// copyStaticFiles copies all files from staticPath to buildPath, preserving
// directory structure. Returns nil if staticPath does not exist.
func copyStaticFiles(staticPath, buildPath string) error {
	if _, err := os.Stat(staticPath); os.IsNotExist(err) {
		return nil
	}
	return filepath.WalkDir(staticPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(staticPath, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(buildPath, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		dst, err := os.Create(dest)
		if err != nil {
			return err
		}
		defer dst.Close()
		_, err = io.Copy(dst, src)
		return err
	})
}

// Build generates the static site from notes.
func Build(cfg config.Config, templateFS fs.FS, styleCSS []byte) error {
	// 0. Clean build directory.
	if err := cleanBuildDir(cfg.BuildPath); err != nil {
		return err
	}

	// 0b. Copy static files.
	if cfg.StaticPath != "" {
		if err := copyStaticFiles(cfg.StaticPath, cfg.BuildPath); err != nil {
			return fmt.Errorf("copying static files: %w", err)
		}
	}

	// 1. Scan notes.
	notes, err := note.Scan(cfg.NotesPath)
	if err != nil {
		return fmt.Errorf("scanning notes: %w", err)
	}

	// 2. Parse frontmatter and filter to public notes.
	type parsedNote struct {
		note note.Note
		fm   note.FrontmatterFields
		body []byte
	}

	var publicNotes []parsedNote
	for _, n := range notes {
		data, err := os.ReadFile(filepath.Join(cfg.NotesPath, n.RelPath))
		if err != nil {
			return fmt.Errorf("reading note %s: %w", n.RelPath, err)
		}
		fm := note.ParseFrontmatterFields(data)
		if !fm.Public {
			continue
		}
		body := note.StripFrontmatter(data)
		publicNotes = append(publicNotes, parsedNote{note: n, fm: fm, body: body})
	}

	// 3. Build note pages and note index (for link resolution).
	noteIndex := make(map[string]string) // note ID -> public path
	var notePages []page.NotePage
	imgCache := images.NewCache(cfg.AssetsPath)

	for _, pn := range publicNotes {
		uid := pn.note.Date + "_" + pn.note.ID
		slug := slugify(trimQuotes(pn.fm.Slug))
		if slug == "" {
			slug = slugify(trimQuotes(pn.fm.Title))
		}
		if slug == "" {
			slug = pn.note.ID
		}

		np := page.NotePage{
			UID:         uid,
			ShortUID:    pn.note.ID,
			Slug:        slug,
			Title:       cleanTitle(trimQuotes(pn.fm.Title), uid),
			Description: trimQuotes(pn.fm.Description),
			Tags:        trimQuotesList(pn.fm.Tags),
			SiteRootURL: cfg.SiteRootURL,
			PublishedAt: parseDate(pn.note.Date),
		}
		noteIndex[pn.note.ID] = np.PublicPath()
		notePages = append(notePages, np)
	}

	// 4. Render Markdown for each note (now that noteIndex is complete).
	for i, pn := range publicNotes {
		uid := notePages[i].UID
		processImage := func(src string) (string, error) {
			entry, err := imgCache.Get(src, uid)
			if err != nil {
				return "", err
			}
			notePages[i].Attachments = append(notePages[i].Attachments, page.Attachment{
				FileName: entry.FileName,
				PageUID:  entry.PageUID,
			})
			return entry.FileName, nil
		}
		rendered, err := render.Render(pn.body, noteIndex, processImage)
		if err != nil {
			return fmt.Errorf("rendering note %s: %w", pn.note.RelPath, err)
		}
		notePages[i].Body = string(rendered)
	}

	// 5. Sort pages newest first.
	page.SortNotePages(notePages)

	// 6. Load templates.
	tmpl, err := template.New("").ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return fmt.Errorf("parsing HTML templates: %w", err)
	}

	// Parse feed.xml with text/template (no HTML escaping).
	feedTmpl, err := texttemplate.New("").ParseFS(templateFS, "templates/feed.xml")
	if err != nil {
		return fmt.Errorf("parsing feed template: %w", err)
	}

	cfgData := configData{
		SiteName:     cfg.SiteName,
		SiteDomain:   cfg.SiteDomain(),
		SiteRootURL:  cfg.SiteRootURL,
		SiteRootPath: cfg.SiteRootPath(),
		AuthorName:   cfg.AuthorName,
		FeedURL:      cfg.FeedURL(),
		FeedPath:     cfg.FeedPath(),
		LicenseName:  cfg.LicenseName,
		LicenseURL:   cfg.LicenseURL,
		HighlightCSS: template.CSS(render.HighlightCSS()),
	}

	// 7. Write note pages and redirects.
	for _, np := range notePages {
		nvd := toNoteViewData(np)

		// Find related notes.
		relatedPages := page.RelatedTo(notePages, np)
		var relatedViews []noteViewData
		for _, rp := range relatedPages {
			relatedViews = append(relatedViews, toNoteViewData(rp))
		}

		inner := noteData{
			Note:    nvd,
			Related: relatedViews,
		}

		pd := pageData{
			Title:           np.Title,
			MetaDescription: np.Description,
			CanonicalPath:   np.CanonicalPath(),
		}

		if err := writeHTMLPage(tmpl, cfg.BuildPath, np.LocalPath(), "note.html", inner, cfgData, pd); err != nil {
			return fmt.Errorf("writing note page %s: %w", np.UID, err)
		}

		// Copy attachments.
		for _, att := range np.Attachments {
			destDir := filepath.Join(cfg.BuildPath, np.UID, np.Slug)
			if err := imgCache.CopyTo(images.Entry{FileName: att.FileName, PageUID: att.PageUID}, destDir); err != nil {
				return fmt.Errorf("copying attachment %s: %w", att.FileName, err)
			}
		}

		// Write redirect page.
		rp := page.RedirectPage{
			UID:        np.UID,
			RedirectTo: "/" + np.PublicPath(),
		}
		if err := writeRedirectPage(tmpl, cfg.BuildPath, rp); err != nil {
			return fmt.Errorf("writing redirect page %s: %w", np.UID, err)
		}
	}

	// 8. Write index page.
	allTags := page.AllTags(notePages)
	var noteViews []noteViewData
	for _, np := range notePages {
		noteViews = append(noteViews, toNoteViewData(np))
	}

	indexInner := indexData{
		Tags:      allTags,
		NotePages: noteViews,
	}
	indexPD := pageData{
		Title:           "",
		MetaDescription: cfg.SiteName,
	}
	if err := writeHTMLPage(tmpl, cfg.BuildPath, "index.html", "index.html", indexInner, cfgData, indexPD); err != nil {
		return fmt.Errorf("writing index page: %w", err)
	}

	// 9. Write tag pages.
	for _, tag := range allTags {
		tagged := page.TaggedPages(notePages, tag)
		var taggedViews []noteViewData
		for _, tp := range tagged {
			taggedViews = append(taggedViews, toNoteViewData(tp))
		}
		tagInner := tagData{
			Tags:        allTags,
			CurrentTag:  tag,
			TaggedPages: taggedViews,
		}
		tp := page.TagPage{Tag: tag}
		tagPD := pageData{
			Title:           tag,
			MetaDescription: fmt.Sprintf("Notes tagged with %s", tag),
			CanonicalPath:   tp.CanonicalPath(),
		}
		if err := writeHTMLPage(tmpl, cfg.BuildPath, tp.LocalPath(), "tag.html", tagInner, cfgData, tagPD); err != nil {
			return fmt.Errorf("writing tag page %s: %w", tag, err)
		}
	}

	// 10. Write feed.
	var feedNotes []feedNoteData
	for _, np := range notePages {
		feedNotes = append(feedNotes, feedNoteData{
			Title: np.Title,
			URL:   np.URL(),
			UID:   np.UID,
			Body:  np.Body,
		})
	}
	fd := feedData{
		Config:    cfgData,
		NotePages: feedNotes,
	}
	if err := writeFile(cfg.BuildPath, "feed.xml", func() ([]byte, error) {
		var buf strings.Builder
		if err := feedTmpl.ExecuteTemplate(&buf, "feed.xml", fd); err != nil {
			return nil, err
		}
		return []byte(buf.String()), nil
	}); err != nil {
		return fmt.Errorf("writing feed: %w", err)
	}

	// 11. Write style.css.
	stylePath := filepath.Join(cfg.BuildPath, "style.css")
	if err := os.WriteFile(stylePath, styleCSS, 0o644); err != nil {
		return fmt.Errorf("writing style.css: %w", err)
	}

	return nil
}

func toNoteViewData(np page.NotePage) noteViewData {
	return noteViewData{
		UID:         np.UID,
		Slug:        np.Slug,
		Title:       np.Title,
		Tags:        np.Tags,
		Body:        template.HTML(np.Body),
		PublicPath:  np.PublicPath(),
		PublishedAt: np.PublishedAt,
	}
}

// writeHTMLPage renders an inner template then wraps it in the layout.
func writeHTMLPage(tmpl *template.Template, buildPath, localPath, innerName string, innerData any, cfgData configData, pd pageData) error {
	// Render inner template.
	var innerBuf strings.Builder
	if err := tmpl.ExecuteTemplate(&innerBuf, innerName, innerData); err != nil {
		return fmt.Errorf("executing %s: %w", innerName, err)
	}

	ld := layoutData{
		Config:  cfgData,
		Page:    pd,
		Content: template.HTML(innerBuf.String()),
	}

	var outBuf strings.Builder
	if err := tmpl.ExecuteTemplate(&outBuf, "layout.html", ld); err != nil {
		return fmt.Errorf("executing layout: %w", err)
	}

	fullPath := filepath.Join(buildPath, localPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, []byte(outBuf.String()), 0o644)
}

// writeRedirectPage renders the redirect template (no layout).
func writeRedirectPage(tmpl *template.Template, buildPath string, rp page.RedirectPage) error {
	var buf strings.Builder
	if err := tmpl.ExecuteTemplate(&buf, "redirect.html", rp); err != nil {
		return fmt.Errorf("executing redirect template: %w", err)
	}

	fullPath := filepath.Join(buildPath, rp.LocalPath())
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, []byte(buf.String()), 0o644)
}

func writeFile(buildPath, relPath string, generate func() ([]byte, error)) error {
	data, err := generate()
	if err != nil {
		return err
	}
	fullPath := filepath.Join(buildPath, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, data, 0o644)
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "`", "")
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func cleanTitle(title, fallback string) string {
	t := strings.ReplaceAll(title, "`", "")
	t = strings.Trim(t, `"`)
	if t == "" {
		return fallback
	}
	return t
}

// trimQuotes removes surrounding double quotes from a YAML value.
// The notescli hand-rolled parser doesn't strip YAML string quotes.
func trimQuotes(s string) string {
	return strings.Trim(s, `"`)
}

// trimQuotesList removes surrounding double quotes from each value in a list.
func trimQuotesList(vals []string) []string {
	result := make([]string, len(vals))
	for i, v := range vals {
		result[i] = trimQuotes(v)
	}
	return result
}

func parseDate(dateStr string) time.Time {
	if len(dateStr) < 8 {
		return time.Time{}
	}
	t, err := time.Parse("20060102", dateStr[:8])
	if err != nil {
		return time.Time{}
	}
	return t
}
