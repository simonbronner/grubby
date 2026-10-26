package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"grubby/internal/config"
	"grubby/internal/git"
	"grubby/internal/github"
	"grubby/internal/storage"
	"grubby/internal/ui"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func main() {
	var (
		orgFlag       string
		workspaceFlag string
		configFlag    string
		proxyFlag     string
		insecureFlag  bool
		versionFlag   bool
	)

	flag.StringVar(&orgFlag, "org", "", "Target GitHub organization or user (defaults to personal account)")
	flag.StringVar(&workspaceFlag, "workspace", "", "Path to local workspace directory (defaults to ~/workspaces)")
	flag.StringVar(&configFlag, "config", "", "Path to Grubby config file (defaults to ~/.config/grubby/config.yaml)")
	flag.StringVar(&proxyFlag, "proxy", "", "HTTP, HTTPS, or SOCKS5 corporate proxy URL (e.g. http://proxy.corp.com:8080)")
	flag.BoolVar(&insecureFlag, "insecure", false, "Skip TLS certificate verification (useful for corporate SSL inspection proxies)")
	flag.BoolVar(&versionFlag, "version", false, "Print Grubby version and exit")
	flag.BoolVar(&versionFlag, "v", false, "Print Grubby version and exit (shorthand)")
	flag.Parse()

	if versionFlag {
		fmt.Printf("grubby %s (%s, %s)\n", Version, Commit, Date)
		return
	}

	// Check authentication
	token, err := github.GetGHToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "Grubby requires GitHub CLI authentication. Please run:")
		fmt.Fprintln(os.Stderr, "  gh auth login --scopes repo,read:org")
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.Load(configFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if workspaceFlag != "" {
		cfg.WorkspaceDir = workspaceFlag
	}

	// Resolve proxy settings: CLI flag takes precedence over config
	proxyURL := proxyFlag
	if proxyURL == "" {
		proxyURL = cfg.Proxy
	}
	insecure := insecureFlag || cfg.InsecureSkipVerify

	// Initialize GitHub client
	ghClient, err := github.NewClientWithOptions(github.ClientOptions{
		Token:              token,
		ProxyURL:           proxyURL,
		InsecureSkipVerify: insecure,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize GitHub client: %v\n", err)
		os.Exit(1)
	}

	// Resolve target organization / owner
	targetOrg := orgFlag
	if targetOrg == "" {
		targetOrg = cfg.DefaultOrg
	}
	if targetOrg == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		currentUser, err := ghClient.GetAuthenticatedUser(ctx)
		cancel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not determine GitHub user: %v\n", err)
			os.Exit(1)
		}
		targetOrg = currentUser
		cfg.DefaultOrg = targetOrg
		_ = cfg.Save()
	}

	// Initialize workspace manager
	gitMgr := git.NewWorkspaceManager(cfg.WorkspaceDir)

	// Initialize local SQLite database cache
	dbPath, _ := storage.DefaultDBPath()
	db, err := storage.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to open SQLite db: %v\n", err)
	} else {
		defer db.Close()
	}

	// Launch TUI
	initialModel := ui.InitialModel(cfg, ghClient, gitMgr, targetOrg, db)
	p := tea.NewProgram(initialModel, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Grubby exited with error: %v\n", err)
		os.Exit(1)
	}
}
