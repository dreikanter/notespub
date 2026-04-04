package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	notespub "github.com/dreikanter/notespub"
	"github.com/dreikanter/notespub/internal/build"
	"github.com/dreikanter/notespub/internal/config"
	"github.com/spf13/cobra"
)

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

func loadConfig(cmd *cobra.Command, cfgPath string) (config.Config, error) {
	if cfgPath == "" {
		cfgPath = os.Getenv("NOTESPUB_CONFIG")
	}
	if cfgPath == "" {
		cfgPath = "notespub.yml"
	}

	envKeys := []string{
		"NOTES_PATH", "NOTESPUB_ASSETS_PATH", "NOTESPUB_BUILD_PATH",
		"NOTESPUB_SITE_ROOT_URL", "NOTESPUB_SITE_NAME", "NOTESPUB_AUTHOR_NAME",
	}
	envOverrides := make(map[string]string)
	for _, key := range envKeys {
		if v := os.Getenv(key); v != "" {
			envOverrides[key] = v
		}
	}

	flagNames := []string{"notes-path", "assets-path", "out", "url", "site-name", "author"}
	flagOverrides := make(map[string]string)
	for _, name := range flagNames {
		if cmd.Flags().Changed(name) {
			v, _ := cmd.Flags().GetString(name)
			flagOverrides[name] = v
		}
	}

	return config.Load(cfgPath, envOverrides, flagOverrides)
}

func init() {
	buildCmd.Flags().String("config", "", "config file path (default: notespub.yml)")
	buildCmd.Flags().String("notes-path", "", "notes store path")
	buildCmd.Flags().String("assets-path", "", "image assets path")
	buildCmd.Flags().String("out", "", "output directory (default: ./dist)")
	buildCmd.Flags().String("url", "", "site root URL")
	buildCmd.Flags().String("site-name", "", "site name")
	buildCmd.Flags().String("author", "", "author name")

	serveCmd.Flags().String("dir", "./dist", "directory to serve")
	serveCmd.Flags().String("port", "4000", "port to listen on")

	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(serveCmd)
}
