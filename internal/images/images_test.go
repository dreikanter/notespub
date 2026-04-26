package images

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheHit(t *testing.T) {
	dir := t.TempDir()
	idx := map[string]Entry{
		"https://example.com/img.jpg": {FileName: "abc.jpg", PageUID: "20230130_3961"},
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.json"), data, 0o644))

	// Create the cached file so CopyTo won't fail.
	uidDir := filepath.Join(dir, "20230130_3961")
	require.NoError(t, os.MkdirAll(uidDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(uidDir, "abc.jpg"), []byte("fake image"), 0o644))

	cache := NewCache(dir)
	entry, err := cache.Get("https://example.com/img.jpg", "20230130_3961")
	require.NoError(t, err)

	assert.Equal(t, "abc.jpg", entry.FileName)
}

func TestCacheMissDownloads(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake jpeg data"))
	}))
	defer ts.Close()

	dir := t.TempDir()
	cache := NewCache(dir)
	entry, err := cache.Get(ts.URL+"/photo.jpg", "20230130_3961")
	require.NoError(t, err)

	assert.NotEmpty(t, entry.FileName)
	assert.Equal(t, "20230130_3961", entry.PageUID)
	assert.FileExists(t, filepath.Join(dir, "20230130_3961", entry.FileName))

	indexData, err := os.ReadFile(filepath.Join(dir, "index.json"))
	require.NoError(t, err)
	var idx map[string]Entry
	require.NoError(t, json.Unmarshal(indexData, &idx))
	assert.Contains(t, idx, ts.URL+"/photo.jpg")
}

func TestCleanshotRedirect(t *testing.T) {
	directURL := ""
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/share/abc+":
			w.Header().Set("Location", directURL+"/direct/photo.jpg?response-content-disposition=attachment%3Bfilename%3Dscreenshot.png")
			w.WriteHeader(http.StatusFound)
		case "/direct/photo.jpg":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png data"))
		}
	}))
	defer ts.Close()
	directURL = ts.URL

	dir := t.TempDir()
	cache := NewCache(dir)
	entry, err := cache.Get(ts.URL+"/share/abc", "20230130_3961")
	require.NoError(t, err)

	assert.NotEmpty(t, entry.FileName)
}

func TestCopyTo(t *testing.T) {
	dir := t.TempDir()
	uidDir := filepath.Join(dir, "20230130_3961")
	require.NoError(t, os.MkdirAll(uidDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(uidDir, "abc.jpg"), []byte("image data"), 0o644))

	cache := NewCache(dir)
	destDir := t.TempDir()
	require.NoError(t, cache.CopyTo(Entry{FileName: "abc.jpg", PageUID: "20230130_3961"}, destDir))

	data, err := os.ReadFile(filepath.Join(destDir, "abc.jpg"))
	require.NoError(t, err)
	assert.Equal(t, "image data", string(data))
}
