package build

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dreikanter/notes-cli/note"
	"github.com/dreikanter/npub/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanBuildDir(t *testing.T) {
	dir := t.TempDir()

	// Create regular files and dirs.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("hi"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "subdir", "page.html"), []byte("hi"), 0o644))

	// Create dotfiles and dotdirs that should survive.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".nojekyll"), []byte(""), 0o644))

	require.NoError(t, cleanBuildDir(dir))

	assert.NoFileExists(t, filepath.Join(dir, "index.html"))
	assert.NoDirExists(t, filepath.Join(dir, "subdir"))
	assert.DirExists(t, filepath.Join(dir, ".git", "objects"))
	assert.FileExists(t, filepath.Join(dir, ".nojekyll"))
}

func TestCleanBuildDirNonExistent(t *testing.T) {
	assert.NoError(t, cleanBuildDir("/tmp/npub-does-not-exist-"+t.Name()))
}

func TestCleanBuildDirRejectsRoot(t *testing.T) {
	require.Error(t, cleanBuildDir("/"))
}

func TestCopyStaticFiles(t *testing.T) {
	staticDir := t.TempDir()
	buildDir := t.TempDir()

	// Create static files.
	require.NoError(t, os.WriteFile(filepath.Join(staticDir, "CNAME"), []byte("example.com"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(staticDir, "README.md"), []byte("# Hello"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(staticDir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(staticDir, "sub", "file.txt"), []byte("nested"), 0o644))

	require.NoError(t, copyStaticFiles(staticDir, buildDir))

	data, err := os.ReadFile(filepath.Join(buildDir, "CNAME"))
	require.NoError(t, err)
	assert.Equal(t, "example.com", string(data))

	data, err = os.ReadFile(filepath.Join(buildDir, "sub", "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "nested", string(data))
}

func TestCleanBuildDirRejectsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	require.Error(t, cleanBuildDir(home))
}

func testConfig(t *testing.T, buildPath, assetsPath string) config.Config {
	t.Helper()
	return config.Config{
		AssetsPath:  assetsPath,
		BuildPath:   buildPath,
		SiteRootURL: "https://example.com",
		SiteName:    "Test Site",
		AuthorName:  "Test Author",
	}
}

func TestBuildPublicNote(t *testing.T) {
	buildDir := t.TempDir()
	assetsDir := t.TempDir()

	store := note.NewMemStore()
	_, err := store.Put(note.Entry{
		ID: 3961,
		Meta: note.Meta{
			Title:     "My Test Note",
			Slug:      "my-test-note",
			Tags:      []string{"golang", "testing"},
			Public:    true,
			CreatedAt: time.Date(2023, 1, 30, 0, 0, 0, 0, time.UTC),
		},
		Body: "Hello **world**.\n",
	})
	require.NoError(t, err)

	cfg := testConfig(t, buildDir, assetsDir)
	templateFS := os.DirFS("../../")
	styleCSS := []byte("/* test */")
	require.NoError(t, Build(store, cfg, templateFS, styleCSS))

	notePath := filepath.Join(buildDir, "20230130_3961", "my-test-note", "index.html")
	data, err := os.ReadFile(notePath)
	require.NoError(t, err)
	html := string(data)
	assert.Contains(t, html, "My Test Note")
	assert.Contains(t, html, "<strong>world</strong>")

	redirectPath := filepath.Join(buildDir, "20230130_3961", "index.html")
	rdata, err := os.ReadFile(redirectPath)
	require.NoError(t, err)
	assert.Contains(t, string(rdata), "my-test-note")

	indexPath := filepath.Join(buildDir, "index.html")
	idata, err := os.ReadFile(indexPath)
	require.NoError(t, err)
	assert.Contains(t, string(idata), "My Test Note")

	assert.FileExists(t, filepath.Join(buildDir, "tags", "golang", "index.html"))

	feedPath := filepath.Join(buildDir, "feed.xml")
	fdata, err := os.ReadFile(feedPath)
	require.NoError(t, err)
	assert.Contains(t, string(fdata), "My Test Note")

	assert.NoFileExists(t, filepath.Join(buildDir, "style.css"))

	indexData, err := os.ReadFile(filepath.Join(buildDir, "index.html"))
	require.NoError(t, err)
	assert.Contains(t, string(indexData), "/* test */")
	assert.NotContains(t, string(indexData), `href="/style.css"`)
}

func TestBuildSkipsPrivateNote(t *testing.T) {
	buildDir := t.TempDir()
	assetsDir := t.TempDir()

	store := note.NewMemStore()
	_, err := store.Put(note.Entry{
		ID: 3961,
		Meta: note.Meta{
			Title:     "Private Note",
			Tags:      []string{"secret"},
			Public:    false,
			CreatedAt: time.Date(2023, 1, 30, 0, 0, 0, 0, time.UTC),
		},
		Body: "This is private.\n",
	})
	require.NoError(t, err)

	cfg := testConfig(t, buildDir, assetsDir)
	templateFS := os.DirFS("../../")
	styleCSS := []byte("/* test */")
	require.NoError(t, Build(store, cfg, templateFS, styleCSS))

	assert.NoDirExists(t, filepath.Join(buildDir, "20230130_3961"))

	data, err := os.ReadFile(filepath.Join(buildDir, "index.html"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "Private Note")
}

func TestBuildNoteLinkResolution(t *testing.T) {
	buildDir := t.TempDir()
	assetsDir := t.TempDir()

	store := note.NewMemStore()
	_, err := store.Put(note.Entry{
		ID: 3961,
		Meta: note.Meta{
			Title:     "First Note",
			Slug:      "first-note",
			Tags:      []string{"test"},
			Public:    true,
			CreatedAt: time.Date(2023, 1, 30, 0, 0, 0, 0, time.UTC),
		},
		Body: "See [second note](3962).\n",
	})
	require.NoError(t, err)
	_, err = store.Put(note.Entry{
		ID: 3962,
		Meta: note.Meta{
			Title:     "Second Note",
			Slug:      "second-note",
			Tags:      []string{"test"},
			Public:    true,
			CreatedAt: time.Date(2023, 1, 31, 0, 0, 0, 0, time.UTC),
		},
		Body: "Hello from second note.\n",
	})
	require.NoError(t, err)

	cfg := testConfig(t, buildDir, assetsDir)
	templateFS := os.DirFS("../../")
	styleCSS := []byte("/* test */")
	require.NoError(t, Build(store, cfg, templateFS, styleCSS))

	data, err := os.ReadFile(filepath.Join(buildDir, "20230130_3961", "first-note", "index.html"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `href="/20230131_3962/second-note"`)
}

func TestChooseSlug(t *testing.T) {
	tests := []struct {
		name  string
		entry note.Entry
		want  string
	}{
		{
			name:  "explicit slug wins",
			entry: note.Entry{ID: 42, Meta: note.Meta{Slug: "explicit-slug", Title: "Some Title"}},
			want:  "explicit-slug",
		},
		{
			name:  "title fallback when slug empty",
			entry: note.Entry{ID: 42, Meta: note.Meta{Title: "Some Title"}},
			want:  "some-title",
		},
		{
			name:  "ID fallback when slug and title both empty",
			entry: note.Entry{ID: 42},
			want:  "42",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, chooseSlug(tt.entry))
		})
	}
}

func TestTitleOrUID(t *testing.T) {
	tests := []struct {
		name  string
		title string
		uid   string
		want  string
	}{
		{name: "non-empty title wins", title: "Hello", uid: "20230130_42", want: "Hello"},
		{name: "empty title falls back to uid", title: "", uid: "20230130_42", want: "20230130_42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, titleOrUID(tt.title, tt.uid))
		})
	}
}
