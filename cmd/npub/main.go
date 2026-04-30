package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"

	"github.com/dreikanter/notesctl/note"
	"github.com/dreikanter/npub"
	"github.com/dreikanter/npub/internal/build"
	"github.com/dreikanter/npub/internal/config"
	"github.com/spf13/cobra"
)

var Version = "dev"

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "npub",
	Short: "Build a static site from a local notes store",
	Long: `npub builds a static site from a directory of Markdown notes.

Configuration is layered: command-line flags (and positional arguments) override
values from the YAML config file.

Config file discovery order:
  1. --config flag (if set)
  2. npub.yml inside $NOTES_PATH (or the --path value, for build)
  3. npub.yml in the current directory

NOTES_PATH does double duty:
  1. Source for notes_path when neither --path nor the YAML sets it.
  2. Hint location for finding npub.yml during config discovery.`,
	SilenceUsage: true,
}

var initCmd = &cobra.Command{
	Use:   "init [dir]",
	Short: "Create a sample npub configuration",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		cfgPath, err := initConfig(path)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "created %s\nedit it to set required fields before running npub build\n", cfgPath)
		return err
	},
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the static site",
	Long: `Build the static site from notes.

Reads notes from notes_path, renders them to HTML, and writes the result to
build_path.

Examples:
  # Use npub.yml from $NOTES_PATH or the current directory
  npub build

  # Override the config file
  npub --config ./prod.yml build

  # Override individual settings via flags
  npub build --path ~/notes --out ./public --url https://example.com`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, _ := cmd.Flags().GetString("config")
		cfg, err := loadConfig(cmd, cfgPath)
		if err != nil {
			return err
		}
		if err := validateNotesPath(cfg.NotesPath); err != nil {
			return err
		}

		log.Printf("building site from %s to %s", cfg.NotesPath, cfg.BuildPath)
		store := note.NewOSStore(cfg.NotesPath)
		if err := build.Build(store, cfg, npub.Assets); err != nil {
			return fmt.Errorf("build failed: %w", err)
		}
		log.Println("build complete")
		return nil
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve the built site locally",
	Long: `Serve the built site over HTTP from a local directory.

Without --dir, serve resolves the directory from build_path in the config
(falling back to ./dist when no config is found). With --dir, the config is
not consulted.

Examples:
  # Serve build_path from the discovered config (or ./dist)
  npub serve

  # Serve a specific directory
  npub serve --dir ./public

  # Bind to all interfaces on a custom port
  npub serve --host 0.0.0.0 --port 8080`,
	RunE: func(cmd *cobra.Command, args []string) error {
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		if err := validatePort(port); err != nil {
			return err
		}
		dir, _ := cmd.Flags().GetString("dir")
		explicitDir := cmd.Flags().Changed("dir")
		if !explicitDir {
			cfgPath, _ := cmd.Flags().GetString("config")
			cfg, err := loadConfigOpt(cmd, cfgPath)
			if err != nil {
				return err
			}
			dir = cfg.BuildPath
		}
		dir = config.ExpandPath(dir)
		info, err := os.Stat(dir)
		if err != nil {
			if explicitDir {
				return fmt.Errorf("cannot serve %q: %w", dir, err)
			}
			return fmt.Errorf("cannot serve %q: %w (run npub build first)", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("cannot serve %q: not a directory", dir)
		}
		addr := host + ":" + strconv.Itoa(port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		log.Printf("serving %s on http://%s", dir, ln.Addr().String())
		return http.Serve(ln, http.FileServer(http.Dir(dir)))
	},
}

func validatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port %d: must be between 1 and 65535", port)
	}
	return nil
}

func validateNotesPath(path string) error {
	if path == "" {
		return fmt.Errorf("notes path is not set: pass --path or set NOTES_PATH")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("invalid notes path %q: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("invalid notes path %q: not a directory", path)
	}
	return nil
}

func initConfig(path string) (string, error) {
	path = config.ExpandPath(path)
	if path == "" {
		path = "."
	}

	info, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return "", fmt.Errorf("cannot create directory %q: %w", path, err)
		}
	} else if !info.IsDir() {
		return "", fmt.Errorf("%q is not a directory", path)
	}

	cfgPath := filepath.Join(path, config.DefaultConfigFile)
	file, err := os.OpenFile(cfgPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("config file already exists: %q", cfgPath)
		}
		return "", fmt.Errorf("cannot create config file %q: %w", cfgPath, err)
	}
	if _, err := file.Write(npub.SampleConfig); err != nil {
		_ = file.Close()
		_ = os.Remove(cfgPath)
		return "", fmt.Errorf("cannot write config file %q: %w", cfgPath, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(cfgPath)
		return "", fmt.Errorf("cannot write config file %q: %w", cfgPath, err)
	}
	return cfgPath, nil
}

func resolveConfigPath(flagValue, notesPath string) string {
	if flagValue != "" {
		return config.ExpandPath(flagValue)
	}
	if notesPath != "" {
		candidate := filepath.Join(config.ExpandPath(notesPath), config.DefaultConfigFile)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return config.DefaultConfigFile
}

func loadConfig(cmd *cobra.Command, cfgPath string) (config.Config, error) {
	// Resolve notes path here too (not only in config.Load) because config
	// discovery needs it before the yaml is read.
	var notesPath string
	if cmd.Flags().Lookup("path") != nil {
		notesPath, _ = cmd.Flags().GetString("path")
	}
	if notesPath == "" {
		notesPath = os.Getenv("NOTES_PATH")
	}
	cfgPath = resolveConfigPath(cfgPath, notesPath)

	flagNames := []string{"path", "assets", "out", "static", "url", "site-name", "author", "license-name", "license-url"}
	flagOverrides := make(map[string]string)
	for _, name := range flagNames {
		if f := cmd.Flags().Lookup(name); f != nil && f.Changed {
			flagOverrides[name] = f.Value.String()
		}
	}

	return config.Load(cfgPath, flagOverrides)
}

// loadConfigOpt loads config like loadConfig but treats a missing/invalid
// config as non-fatal when --config wasn't set explicitly, returning a
// minimal default instead.
func loadConfigOpt(cmd *cobra.Command, cfgPath string) (config.Config, error) {
	cfg, err := loadConfig(cmd, cfgPath)
	if err != nil && !cmd.Flags().Changed("config") {
		return config.Config{BuildPath: "./dist"}, nil
	}
	return cfg, err
}

func init() {
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
			Version = info.Main.Version
		}
	}
	rootCmd.Version = Version

	rootCmd.PersistentFlags().String("config", "", "config file path (default: npub.yml)")

	buildCmd.Flags().String("path", "", "notes path (default: NOTES_PATH)")
	buildCmd.Flags().String("assets", "", "image assets path")
	buildCmd.Flags().String("out", "", "output directory (default: ./dist)")
	buildCmd.Flags().String("static", "", "static files directory")
	buildCmd.Flags().String("url", "", "site root URL")
	buildCmd.Flags().String("site-name", "", "site name")
	buildCmd.Flags().String("author", "", "author name")
	buildCmd.Flags().String("license-name", "", "license name (default: CC BY 4.0)")
	buildCmd.Flags().String("license-url", "", "license URL (default: https://creativecommons.org/licenses/by/4.0/)")

	serveCmd.Flags().String("dir", "", "directory to serve (default: build_path from config, or ./dist)")
	serveCmd.Flags().String("host", "localhost", "interface to bind")
	serveCmd.Flags().Int("port", 4000, "port to listen on")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(serveCmd)
}
