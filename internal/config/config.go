package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultConfigFile is the conventional config file name.
const DefaultConfigFile = "npub.yml"

// Config holds all configuration for a npub build.
type Config struct {
	NotesPath   string `yaml:"notes_path"`
	AssetsPath  string `yaml:"assets_path"`
	BuildPath   string `yaml:"build_path"`
	StaticPath  string `yaml:"static_path"`
	SiteRootURL string `yaml:"site_root_url"`
	SiteName    string `yaml:"site_name"`
	AuthorName  string `yaml:"author_name"`
	LicenseName string `yaml:"license_name"`
	LicenseURL  string `yaml:"license_url"`
	Intro       string `yaml:"intro"`
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

// SiteDomain returns the domain (host) component of SiteRootURL.
func (c Config) SiteDomain() string {
	idx := strings.Index(c.SiteRootURL, "://")
	if idx < 0 {
		return c.SiteRootURL
	}
	rest := c.SiteRootURL[idx+3:]
	if slash := strings.IndexByte(rest, '/'); slash >= 0 {
		return rest[:slash]
	}
	return rest
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
		return Config{}, fmt.Errorf("cannot read config %q: %w", yamlPath, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("cannot parse config %q: %w", yamlPath, err)
	}

	flagMap := map[string]*string{
		"path":         &cfg.NotesPath,
		"assets":       &cfg.AssetsPath,
		"out":          &cfg.BuildPath,
		"static":       &cfg.StaticPath,
		"url":          &cfg.SiteRootURL,
		"site-name":    &cfg.SiteName,
		"author":       &cfg.AuthorName,
		"license-name": &cfg.LicenseName,
		"license-url":  &cfg.LicenseURL,
	}
	for flagKey, ptr := range flagMap {
		if v, ok := flagOverrides[flagKey]; ok && v != "" {
			*ptr = v
		}
	}

	if cfg.NotesPath == "" {
		cfg.NotesPath = os.Getenv("NOTES_PATH")
	}
	cfg.NotesPath = ExpandPath(cfg.NotesPath)
	cfg.AssetsPath = ExpandPath(cfg.AssetsPath)
	if cfg.BuildPath == "" {
		cfg.BuildPath = "./dist"
	}
	cfg.BuildPath = ExpandPath(cfg.BuildPath)
	cfg.StaticPath = ExpandPath(cfg.StaticPath)
	if cfg.StaticPath == "" && cfg.NotesPath != "" {
		cfg.StaticPath = filepath.Join(cfg.NotesPath, "static")
	}
	if cfg.AssetsPath == "" && cfg.NotesPath != "" {
		cfg.AssetsPath = filepath.Join(cfg.NotesPath, "images")
	}

	if cfg.LicenseName == "" {
		cfg.LicenseName = "CC BY 4.0"
	}
	if cfg.LicenseURL == "" {
		cfg.LicenseURL = "https://creativecommons.org/licenses/by/4.0/"
	}

	var missing []string
	if cfg.SiteRootURL == "" {
		missing = append(missing, "site_root_url")
	}
	if cfg.SiteName == "" {
		missing = append(missing, "site_name")
	}
	if cfg.AuthorName == "" {
		missing = append(missing, "author_name")
	}
	if len(missing) > 0 {
		return cfg, fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

// ExpandPath expands environment variables ($VAR, ${VAR}) and a leading ~/
// in path strings. Apply it at every boundary that accepts a filesystem path
// (CLI flags, positionals, YAML fields) so users see one consistent rule.
func ExpandPath(path string) string {
	path = os.ExpandEnv(path)
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}
