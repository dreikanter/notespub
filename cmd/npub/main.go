package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/dreikanter/notes-cli/note"
	"github.com/dreikanter/npub"
	"github.com/dreikanter/npub/internal/build"
	"github.com/dreikanter/npub/internal/config"
	"github.com/spf13/cobra"
)

var Version = "dev"

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "npub",
	Short: "Build a static site from a local notes store",
}

var initCmd = &cobra.Command{
	Use:   "init [path]",
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
		fmt.Fprintf(cmd.OutOrStdout(), "created %s\nedit it to set required fields before running npub build\n", cfgPath)
		return nil
	},
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the static site",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, _ := cmd.Flags().GetString("config")
		cfg, err := loadConfig(cmd, cfgPath)
		if err != nil {
			return err
		}

		log.Printf("building site from %s to %s", cfg.NotesPath, cfg.BuildPath)
		store := note.NewOSStore(cfg.NotesPath)
		if err := build.Build(store, cfg, npub.TemplateFS, npub.StyleCSS); err != nil {
			return fmt.Errorf("build failed: %w", err)
		}
		log.Println("build complete")
		return nil
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve the built site locally",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		port, _ := cmd.Flags().GetString("port")

		if dir == "" {
			dir = "./dist"
		}
		addr := ":" + port
		log.Printf("serving %s on http://localhost%s", dir, addr)
		return http.ListenAndServe(addr, http.FileServer(http.Dir(dir)))
	},
}

func initConfig(path string) (string, error) {
	path = expandHome(os.ExpandEnv(path))
	if path == "" {
		path = "."
	}

	info, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("cannot inspect %s: %w", path, err)
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return "", fmt.Errorf("cannot create directory %s: %w", path, err)
		}
	} else if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", path)
	}

	cfgPath := filepath.Join(path, config.DefaultConfigFile)
	file, err := os.OpenFile(cfgPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("config file already exists: %s", cfgPath)
		}
		return "", fmt.Errorf("cannot create config file %s: %w", cfgPath, err)
	}
	if _, err := file.Write(npub.SampleConfig); err != nil {
		_ = file.Close()
		_ = os.Remove(cfgPath)
		return "", fmt.Errorf("cannot write config file %s: %w", cfgPath, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(cfgPath)
		return "", fmt.Errorf("cannot write config file %s: %w", cfgPath, err)
	}
	return cfgPath, nil
}

func resolveConfigPath(flagValue, envValue, notesPath string) string {
	if flagValue != "" {
		return expandHome(os.ExpandEnv(flagValue))
	}
	if envValue != "" {
		return expandHome(os.ExpandEnv(envValue))
	}
	if notesPath != "" {
		// Match config.Load's path handling for notes_path: expand ~/ but not $VARS.
		candidate := filepath.Join(expandHome(notesPath), config.DefaultConfigFile)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return config.DefaultConfigFile
}

func loadConfig(cmd *cobra.Command, cfgPath string) (config.Config, error) {
	// Resolve notes path here too (not only in config.Load) because config
	// discovery needs it before the yaml is read.
	notesPath, _ := cmd.Flags().GetString("notes")
	if notesPath == "" {
		notesPath = os.Getenv("NOTES_PATH")
	}
	cfgPath = resolveConfigPath(cfgPath, os.Getenv("NPUB_CONFIG"), notesPath)

	flagNames := []string{"notes", "assets", "out", "static", "url", "site-name", "author", "license-name", "license-url"}
	flagOverrides := make(map[string]string)
	for _, name := range flagNames {
		if cmd.Flags().Changed(name) {
			v, _ := cmd.Flags().GetString(name)
			flagOverrides[name] = v
		}
	}

	return config.Load(cfgPath, flagOverrides)
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func init() {
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
			Version = info.Main.Version
		}
	}
	rootCmd.Version = Version

	buildCmd.Flags().String("config", "", "config file path (default: npub.yml)")
	buildCmd.Flags().String("notes", "", "notes store path")
	buildCmd.Flags().String("assets", "", "image assets path")
	buildCmd.Flags().String("out", "", "output directory (default: ./dist)")
	buildCmd.Flags().String("static", "", "static files directory")
	buildCmd.Flags().String("url", "", "site root URL")
	buildCmd.Flags().String("site-name", "", "site name")
	buildCmd.Flags().String("author", "", "author name")
	buildCmd.Flags().String("license-name", "", "license name (default: CC BY 4.0)")
	buildCmd.Flags().String("license-url", "", "license URL (default: https://creativecommons.org/licenses/by/4.0/)")

	serveCmd.Flags().String("dir", "./dist", "directory to serve")
	serveCmd.Flags().String("port", "4000", "port to listen on")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(serveCmd)
}
