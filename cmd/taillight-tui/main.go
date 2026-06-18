// Command taillight-tui provides a terminal user interface for the Taillight
// log viewer. It connects to a running Taillight API server via HTTP/SSE and
// renders real-time log streams, dashboards, and device summaries in the
// terminal using the Charmbracelet TUI stack.
//
// Usage:
//
//	taillight-tui -s https://taillight.example.com -k tl_xxxxx
//	taillight-tui -s https://dev.local -k tl_xxxxx --insecure
//	taillight-tui -c ~/.config/taillight/tui.yml
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	tea "charm.land/bubbletea/v2"

	"github.com/lasseh/taillight-tui/internal/tui"
	"github.com/lasseh/taillight-tui/internal/tui/client"
)

// Version is set at build time via -ldflags "-X main.Version=...".
var Version = "dev"

var (
	configPath string
	serverURL  string
	apiKey     string
	insecure   bool
)

var rootCmd = &cobra.Command{
	Use:          "taillight-tui",
	Short:        "Terminal UI for the Taillight log viewer",
	Version:      Version,
	SilenceUsage: true,
	RunE:         run,
}

func init() {
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "", "path to config file")
	rootCmd.Flags().StringVarP(&serverURL, "server", "s", "", "server URL (overrides config)")
	rootCmd.Flags().StringVarP(&apiKey, "key", "k", "", "API key (overrides config)")
	rootCmd.Flags().BoolVar(&insecure, "insecure", false, "skip TLS certificate verification")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// CLI flags override config file values.
	if serverURL != "" {
		cfg.Server.URL = serverURL
	}
	if apiKey != "" {
		cfg.Server.APIKey = apiKey
	}
	if cmd.Flags().Changed("insecure") {
		cfg.Server.TLSSkipVerify = insecure
	}

	if cfg.Server.URL == "" {
		return errors.New("server URL is required (use -s flag or config file)")
	}

	// Clamp the frame rate to the supported range.
	cfg.Display.FPS = min(max(cfg.Display.FPS, 1), 60)

	c := client.New(client.Config{
		BaseURL:       cfg.Server.URL,
		APIKey:        cfg.Server.APIKey,
		TLSSkipVerify: cfg.Server.TLSSkipVerify,
	})

	app := tui.NewApp(cfg.ToAppConfig(), c)
	defer app.Cleanup()

	p := tea.NewProgram(app, tea.WithFPS(cfg.Display.FPS))
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run TUI: %w", err)
	}
	return nil
}
