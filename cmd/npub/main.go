package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/dreikanter/notesctl/note"
	"github.com/dreikanter/npub"
	"github.com/dreikanter/npub/internal/build"
	"github.com/dreikanter/npub/internal/config"
	"github.com/dreikanter/npub/internal/deploy"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var Version = "dev"

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:          "npub",
	Short:        "Build a static site from a local notes store",
	Long:         `npub builds a static site from a directory of Markdown notes. Run "npub config" to see how flags, environment, and YAML are resolved.`,
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
	Long: `Read notes from notes_path, render to HTML, write to build_path. Flags below
override the corresponding YAML keys.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, _ := cmd.Flags().GetString("config")
		cfg, _, err := loadConfig(cmd, cfgPath)
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

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Build and push the site to a git remote",
	Long: `Build the site into a local working copy of deploy_repo and push it.

The working copy lives in ~/.cache/npub/<repo>; npub clones into it on first
use and hard-resets it to the remote default branch on subsequent runs. With
--dry-run, npub still clones, fetches, and builds, but skips the push.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, _ := cmd.Flags().GetString("config")
		cfg, _, err := loadConfig(cmd, cfgPath)
		if err != nil {
			return err
		}
		if err := validateNotesPath(cfg.NotesPath); err != nil {
			return err
		}
		if strings.TrimSpace(cfg.DeployRepo) == "" {
			return fmt.Errorf("deploy_repo is not set; add it to %s", config.DefaultConfigFile)
		}
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		workDir, err := deploy.WorkDir(cfg.DeployRepo)
		if err != nil {
			return err
		}
		log.Printf("preparing %s", workDir)
		if err := deploy.Prepare(cfg.DeployRepo, workDir, deploy.Options{}); err != nil {
			return err
		}

		cfg.BuildPath = workDir
		log.Printf("building site from %s to %s", cfg.NotesPath, cfg.BuildPath)
		store := note.NewOSStore(cfg.NotesPath)
		if err := build.Build(store, cfg, npub.Assets); err != nil {
			return err
		}

		message := fmt.Sprintf("Deploy %s", time.Now().UTC().Format(time.RFC3339))
		committed, err := deploy.Commit(workDir, message, deploy.Options{})
		if err != nil {
			return err
		}
		if !committed {
			log.Println("no changes to deploy")
			return nil
		}
		if dryRun {
			log.Println("dry-run: skipping push")
			return nil
		}
		log.Println("pushing")
		if err := deploy.Push(workDir, deploy.Options{}); err != nil {
			return err
		}
		log.Println("deploy complete")
		return nil
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Print resolved configuration",
	Long: `Print the resolved config path and final values after merging YAML, CLI flags,
environment, and defaults. Accepts the same overrides as build, so you can
preview a build's configuration without running it. If required fields are
missing, the partial config is still printed and the command exits non-zero.

Config discovery order:
  1. --config
  2. $NOTES_PATH/npub.yml (or --path/npub.yml, for build)
  3. ./npub.yml

NOTES_PATH supplies notes_path when --path and YAML don't set it, and hints
config discovery.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgFlag, _ := cmd.Flags().GetString("config")
		cfg, cfgPath, loadErr := loadConfig(cmd, cfgFlag)
		if abs, err := filepath.Abs(cfgPath); err == nil {
			cfgPath = abs
		}
		if err := printConfig(cmd.OutOrStdout(), cfgPath, cfg); err != nil {
			return err
		}
		return loadErr
	},
}

func printConfig(w io.Writer, cfgPath string, cfg config.Config) error {
	if _, err := fmt.Fprintf(w, "config: %s\n\n", cfgPath); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}
	_, err = w.Write(data)
	return err
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve the built site locally",
	Long: `Serve the built site over HTTP. Without --dir, uses build_path from the
config (falling back to ./dist when no config is found).`,
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

func loadConfig(cmd *cobra.Command, cfgPath string) (config.Config, string, error) {
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

	cfg, err := config.Load(cfgPath, flagOverrides)
	return cfg, cfgPath, err
}

// loadConfigOpt loads config like loadConfig but treats a missing/invalid
// config as non-fatal when --config wasn't set explicitly, returning a
// minimal default instead.
func loadConfigOpt(cmd *cobra.Command, cfgPath string) (config.Config, error) {
	cfg, _, err := loadConfig(cmd, cfgPath)
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

	addConfigFlags(buildCmd)
	addConfigFlags(configCmd)
	addConfigFlags(deployCmd)
	deployCmd.Flags().Bool("dry-run", false, "build and commit but skip git push")

	serveCmd.Flags().String("dir", "", "directory to serve (default: build_path from config, or ./dist)")
	serveCmd.Flags().String("host", "localhost", "interface to bind")
	serveCmd.Flags().Int("port", 4000, "port to listen on")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(serveCmd)
}

func addConfigFlags(cmd *cobra.Command) {
	cmd.Flags().String("path", "", "notes path (default: NOTES_PATH)")
	cmd.Flags().String("assets", "", "image assets path")
	cmd.Flags().String("out", "", "output directory (default: ./dist)")
	cmd.Flags().String("static", "", "static files directory")
	cmd.Flags().String("url", "", "site root URL")
	cmd.Flags().String("site-name", "", "site name")
	cmd.Flags().String("author", "", "author name")
	cmd.Flags().String("license-name", "", "license name (default: CC BY 4.0)")
	cmd.Flags().String("license-url", "", "license URL (default: https://creativecommons.org/licenses/by/4.0/)")
}
