package ui

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var (
	nonAlphaNumRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	multiDashRegex   = regexp.MustCompile(`-+`)
)

// SanitizeTmuxSessionName converts a repository name into a valid tmux session name
func SanitizeTmuxSessionName(repoName string) string {
	clean := nonAlphaNumRegex.ReplaceAllString(repoName, "-")
	clean = multiDashRegex.ReplaceAllString(clean, "-")
	clean = strings.Trim(clean, "-")
	if clean == "" {
		clean = "repo"
	}
	return "grubby-" + clean
}

// HasTmux returns true if the tmux executable is present in PATH
func HasTmux() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// OpenRepoShell opens a terminal shell for a repository.
// If tmux is installed, it attaches to an existing session or creates a dedicated session for that repository.
// If tmux is not installed, it falls back to a standard shell in iTerm or Terminal.app.
func OpenRepoShell(repoName, repoDir string) (bool, error) {
	tmuxInstalled := HasTmux()
	sessionName := SanitizeTmuxSessionName(repoName)

	var shellCommand string
	if tmuxInstalled {
		// If session exists, attach to it. If not, create and attach.
		shellCommand = fmt.Sprintf("tmux attach-session -t %q || tmux new-session -s %q -c %q", sessionName, sessionName, repoDir)
	} else {
		shellCommand = fmt.Sprintf("cd %q && clear", repoDir)
	}

	if _, err := os.Stat("/Applications/iTerm.app"); err == nil {
		script := fmt.Sprintf(`
tell application "iTerm"
	activate
	try
		tell current window
			create tab with default profile
			tell current session
				write text %q
			end tell
		end tell
	on error
		create window with default profile
		tell current session of current window
			write text %q
		end tell
	end try
end tell`, shellCommand, shellCommand)
		cmd := exec.Command("osascript", "-e", script)
		if err := cmd.Run(); err == nil {
			return tmuxInstalled, nil
		}
		// Fallback to open -a iTerm
		return tmuxInstalled, exec.Command("open", "-a", "iTerm", repoDir).Start()
	}

	// Fallback to Terminal.app
	script := fmt.Sprintf(`
tell application "Terminal"
	activate
	do script %q
end tell`, shellCommand)
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err == nil {
		return tmuxInstalled, nil
	}

	return tmuxInstalled, exec.Command("open", "-a", "Terminal", repoDir).Start()
}
