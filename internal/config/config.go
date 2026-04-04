package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultConfigFile is the conventional config file name.
const DefaultConfigFile = "notespub.yml"

// Config holds all configuration for a notespub build.
type Config struct {
	NotesPath   string `yaml:"notes_path"`
	AssetsPath  string `yaml:"assets_path"`
	BuildPath   string `yaml:"build_path"`
	SiteRootURL string `yaml:"site_root_url"`
	SiteName    string `yaml:"site_name"`
	AuthorName  string `yaml:"author_name"`
}

// SiteRootPath returns the URL path component of SiteRootURL.
func (c Config) SiteRootPath() string {
	idx := strings.Index(c.SiteRootURL, "://")
	if idx < 0 {
		return "/"
	}
	rest := c.SiteRootURL[idx+3:]
	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		return "/"
	}
	p := rest[slash:]
	p = strings.TrimRight(p, "/")
	if p == "" {
		return "/"
	}
	return p
}

// FeedURL returns the full feed URL.
func (c Config) FeedURL() string {
	return strings.TrimRight(c.SiteRootURL, "/") + c.FeedPath()
}

// FeedPath returns the path to the feed relative to site root.
func (c Config) FeedPath() string {
	root := c.SiteRootPath()
	if root == "/" {
		return "/feed.xml"
	}
	return root + "/feed.xml"
}

// Load reads configuration from a YAML file, then overlays environment variables,
// then overlays flag values.
func Load(yamlPath string, envOverrides map[string]string, flagOverrides map[string]string) (Config, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return Config{}, fmt.Errorf("cannot read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("cannot parse config: %w", err)
	}

	envMap := map[string]*string{
		"NOTES_PATH":             &cfg.NotesPath,
		"NOTESPUB_ASSETS_PATH":   &cfg.AssetsPath,
		"NOTESPUB_BUILD_PATH":    &cfg.BuildPath,
		"NOTESPUB_SITE_ROOT_URL": &cfg.SiteRootURL,
		"NOTESPUB_SITE_NAME":     &cfg.SiteName,
		"NOTESPUB_AUTHOR_NAME":   &cfg.AuthorName,
	}
	for envKey, ptr := range envMap {
		if v, ok := envOverrides[envKey]; ok && v != "" {
			*ptr = v
		}
	}

	flagMap := map[string]*string{
		"notes-path":  &cfg.NotesPath,
		"assets-path": &cfg.AssetsPath,
		"out":         &cfg.BuildPath,
		"url":         &cfg.SiteRootURL,
		"site-name":   &cfg.SiteName,
		"author":      &cfg.AuthorName,
	}
	for flagKey, ptr := range flagMap {
		if v, ok := flagOverrides[flagKey]; ok && v != "" {
			*ptr = v
		}
	}

	cfg.NotesPath = expandHome(cfg.NotesPath)
	cfg.AssetsPath = expandHome(cfg.AssetsPath)
	cfg.BuildPath = expandHome(cfg.BuildPath)

	if cfg.SiteRootURL == "" {
		return Config{}, fmt.Errorf("site_root_url is required")
	}
	if cfg.SiteName == "" {
		return Config{}, fmt.Errorf("site_name is required")
	}
	if cfg.AuthorName == "" {
		return Config{}, fmt.Errorf("author_name is required")
	}

	return cfg, nil
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}
