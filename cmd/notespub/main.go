package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	notespub "github.com/dreikanter/notespub"
	"github.com/dreikanter/notespub/internal/build"
	"github.com/dreikanter/notespub/internal/config"
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
	Use:   "notespub",
	Short: "Build a static site from a local notes store",
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
		if err := build.Build(cfg, notespub.TemplateFS, notespub.StyleCSS); err != nil {
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

func resolveConfigPath(flagValue, envValue string) string {
	if flagValue != "" {
		return expandHome(os.ExpandEnv(flagValue))
	}
	if envValue != "" {
		return expandHome(os.ExpandEnv(envValue))
	}
	return config.DefaultConfigFile
}

func loadConfig(cmd *cobra.Command, cfgPath string) (config.Config, error) {
	cfgPath = resolveConfigPath(cfgPath, os.Getenv("NOTESPUB_CONFIG"))

	flagNames := []string{"notes-path", "assets-path", "out", "static", "url", "site-name", "author"}
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

	buildCmd.Flags().String("config", "", "config file path (default: notespub.yml)")
	buildCmd.Flags().String("notes-path", "", "notes store path")
	buildCmd.Flags().String("assets-path", "", "image assets path")
	buildCmd.Flags().String("out", "", "output directory (default: ./dist)")
	buildCmd.Flags().String("static", "", "static files directory")
	buildCmd.Flags().String("url", "", "site root URL")
	buildCmd.Flags().String("site-name", "", "site name")
	buildCmd.Flags().String("author", "", "author name")

	serveCmd.Flags().String("dir", "./dist", "directory to serve")
	serveCmd.Flags().String("port", "4000", "port to listen on")

	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(serveCmd)
}
