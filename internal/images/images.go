package images

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Entry is a single cached image record, matching the Ruby index.json format.
type Entry struct {
	FileName string `json:"file_name"`
	PageUID  string `json:"page_uid"`
}

// Cache manages downloaded images on disk.
type Cache struct {
	assetsPath string
}

func NewCache(assetsPath string) *Cache {
	return &Cache{assetsPath: assetsPath}
}

// Get returns the cached entry for a URL, downloading it if not present.
func (c *Cache) Get(imageURL, pageUID string) (Entry, error) {
	idx := c.loadIndex()
	if entry, ok := idx[imageURL]; ok {
		return entry, nil
	}

	body, originalName, err := c.download(imageURL)
	if err != nil {
		return Entry{}, fmt.Errorf("downloading %s: %w", imageURL, err)
	}

	fileName := randomName(extFrom(originalName))
	entry := Entry{FileName: fileName, PageUID: pageUID}

	dir := filepath.Join(c.assetsPath, pageUID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Entry{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, fileName), body, 0o644); err != nil {
		return Entry{}, err
	}

	idx[imageURL] = entry
	if err := c.saveIndex(idx); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// CopyTo copies a cached image to a destination directory.
func (c *Cache) CopyTo(entry Entry, destDir string) error {
	src := filepath.Join(c.assetsPath, entry.PageUID, entry.FileName)
	dst := filepath.Join(destDir, entry.FileName)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func (c *Cache) download(imageURL string) ([]byte, string, error) {
	if isCleanShotURL(imageURL) {
		return c.downloadCleanShot(imageURL)
	}
	return c.downloadDirect(imageURL)
}

func (c *Cache) downloadCleanShot(imageURL string) ([]byte, string, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(imageURL + "+")
	if err != nil {
		return nil, "", err
	}
	if err := resp.Body.Close(); err != nil {
		return nil, "", err
	}

	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return c.downloadDirect(imageURL)
	}

	directURL := resp.Header.Get("Location")
	if directURL == "" {
		return c.downloadDirect(imageURL)
	}

	originalName := nameFromDisposition(directURL)
	body, _, err := c.downloadDirect(directURL)
	if err != nil {
		return nil, "", err
	}
	if originalName != "" {
		return body, originalName, nil
	}
	return body, nameFromURL(directURL), nil
}

func (c *Cache) downloadDirect(imageURL string) ([]byte, string, error) {
	resp, err := http.Get(imageURL)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return body, nameFromURL(imageURL), nil
}

func isCleanShotURL(u string) bool {
	return strings.Contains(u, "cleanshot.com")
}

func nameFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "image"
	}
	base := filepath.Base(parsed.Path)
	if base == "" || base == "." || base == "/" {
		return "image"
	}
	return base
}

func nameFromDisposition(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	disp := parsed.Query().Get("response-content-disposition")
	if disp == "" {
		return ""
	}
	parts := strings.SplitN(disp, "filename=", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func (c *Cache) indexPath() string {
	return filepath.Join(c.assetsPath, "index.json")
}

func (c *Cache) loadIndex() map[string]Entry {
	data, err := os.ReadFile(c.indexPath())
	if err != nil {
		return make(map[string]Entry)
	}
	var idx map[string]Entry
	if err := json.Unmarshal(data, &idx); err != nil {
		return make(map[string]Entry)
	}
	return idx
}

func (c *Cache) saveIndex(idx map[string]Entry) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.indexPath(), data, 0o644)
}

func randomName(ext string) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b) + ext
}

func extFrom(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".jpeg" {
		return ".jpg"
	}
	if ext == "" {
		return ".jpg"
	}
	return ext
}
