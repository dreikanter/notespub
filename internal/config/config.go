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
	StaticPath  string `yaml:"static_path"`
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

// Load reads configuration from a YAML file, then overlays flag values.
func Load(yamlPath string, flagOverrides map[string]string) (Config, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return Config{}, fmt.Errorf("cannot read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("cannot parse config: %w", err)
	}

	flagMap := map[string]*string{
		"notes-path":  &cfg.NotesPath,
		"assets-path": &cfg.AssetsPath,
		"out":         &cfg.BuildPath,
		"static":      &cfg.StaticPath,
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
	cfg.StaticPath = expandHome(cfg.StaticPath)
	if cfg.StaticPath == "" && cfg.NotesPath != "" {
		cfg.StaticPath = filepath.Join(cfg.NotesPath, "static")
	}

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
