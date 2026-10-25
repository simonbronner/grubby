package ui

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"grubby/internal/config"
)

var (
	nonAlphaNumRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	multiDashRegex   = regexp.MustCompile(`-+`)
)

// SanitizeTmuxSessionName converts a repository name into a clean, valid tmux session name.
// It uses the repository name directly (without unnecessary prefixes) for easy identification.
func SanitizeTmuxSessionName(repoName string) string {
	clean := nonAlphaNumRegex.ReplaceAllString(repoName, "-")
	clean = multiDashRegex.ReplaceAllString(clean, "-")
	clean = strings.Trim(clean, "-")
	if clean == "" {
		clean = "repo"
	}
	return clean
}

// HasTmux returns true if the tmux executable is present in PATH.
func HasTmux() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// TmuxSessionExists returns true if a tmux session with the given name currently exists.
func TmuxSessionExists(sessionName string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	return cmd.Run() == nil
}

// TmuxHasAttachedClients returns true if any client is currently attached to tmux.
func TmuxHasAttachedClients() bool {
	out, err := exec.Command("tmux", "list-clients").Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// GetTmuxSessionsMap returns a map of sessionName -> windowCount for all active tmux sessions.
func GetTmuxSessionsMap() map[string]int {
	sessions := make(map[string]int)
	if !HasTmux() {
		return sessions
	}
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}:#{session_windows}").Output()
	if err != nil {
		return sessions
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			var count int
			_, _ = fmt.Sscanf(parts[1], "%d", &count)
			sessions[parts[0]] = count
		}
	}
	return sessions
}

// activateOrOpenGrubbyITermWindow ensures a single dedicated iTerm2 window (tagged as "Grubby")
// is either activated if it exists or created if it does not.
// If writeCmd is non-empty, it sends that command string into the window's current session.
func activateOrOpenGrubbyITermWindow(writeCmd string) error {
	if _, err := os.Stat("/Applications/iTerm.app"); err != nil {
		return fmt.Errorf("iTerm not installed")
	}

	script := fmt.Sprintf(`
tell application "iTerm"
	activate
	set targetWin to missing value
	repeat with w in windows
		tell current session of w
			try
				if (variable named "user.grubby_shell") is "true" then
					set targetWin to w
					exit repeat
				end if
			end try
		end tell
	end repeat
	if targetWin is not missing value then
		tell targetWin to select
		if %q is not "" then
			tell current session of targetWin to write text %q
		end if
	else
		set targetWin to (create window with default profile)
		tell current session of targetWin
			try
				set variable named "user.grubby_shell" to "true"
				set name to "Grubby"
			end try
			if %q is not "" then
				write text %q
			end if
		end tell
	end if
end tell`, writeCmd, writeCmd, writeCmd, writeCmd)

	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

// activateOrOpenTerminalApp fallback for Terminal.app when iTerm is unavailable.
func activateOrOpenTerminalApp(shellCommand string) error {
	script := fmt.Sprintf(`
tell application "Terminal"
	activate
	do script %q
end tell`, shellCommand)
	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

// OpenRepoShell opens a terminal shell for a repository:
// - If the session exists, it emits a command to create a new window in that session at repoDir.
// - If the session does not exist, it creates the session in repoDir.
// - Ensures the session is displayed in the dedicated Grubby iTerm window.
func OpenRepoShell(repoName, repoDir string) (string, error) {
	sessionName := SanitizeTmuxSessionName(repoName)
	tmuxInstalled := HasTmux()

	if !tmuxInstalled {
		shellCmd := fmt.Sprintf("cd %q && clear", repoDir)
		err := activateOrOpenGrubbyITermWindow(shellCmd)
		if err != nil {
			err = activateOrOpenTerminalApp(shellCmd)
		}
		collapsed := config.CollapseHome(repoDir)
		return fmt.Sprintf("Opened shell at %s (tip: install tmux for persistent sessions)", collapsed), err
	}

	sessionAlreadyExisted := TmuxSessionExists(sessionName)

	if !sessionAlreadyExisted {
		// Create session directly via Go exec.Command to guarantee exact repoDir and unquoted session name
		cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-c", repoDir)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to create tmux session: %w", err)
		}
	} else {
		// Session already exists: create a new window inside this session at repoDir
		_ = exec.Command("tmux", "new-window", "-t", sessionName, "-c", repoDir).Run()
	}

	// Connect client in Grubby iTerm window
	if TmuxHasAttachedClients() {
		_ = exec.Command("tmux", "switch-client", "-t", sessionName).Run()
		_ = activateOrOpenGrubbyITermWindow("")
	} else {
		attachCmd := fmt.Sprintf("tmux attach-session -t %s", sessionName)
		err := activateOrOpenGrubbyITermWindow(attachCmd)
		if err != nil {
			_ = activateOrOpenTerminalApp(attachCmd)
		}
	}

	if sessionAlreadyExisted {
		return fmt.Sprintf("Created new window in tmux session '%s'", sessionName), nil
	}
	return fmt.Sprintf("Created tmux session '%s'", sessionName), nil
}

// GoToRepoSession switches to the existing tmux session for a repository:
// - If the session exists, it switches to it without creating a new window.
// - If the session does not exist, it creates it in repoDir and switches to it.
// - Focuses the dedicated Grubby iTerm window.
func GoToRepoSession(repoName, repoDir string) (string, error) {
	sessionName := SanitizeTmuxSessionName(repoName)
	tmuxInstalled := HasTmux()

	if !tmuxInstalled {
		shellCmd := fmt.Sprintf("cd %q && clear", repoDir)
		err := activateOrOpenGrubbyITermWindow(shellCmd)
		if err != nil {
			err = activateOrOpenTerminalApp(shellCmd)
		}
		collapsed := config.CollapseHome(repoDir)
		return fmt.Sprintf("Opened shell at %s", collapsed), err
	}

	sessionAlreadyExisted := TmuxSessionExists(sessionName)

	if !sessionAlreadyExisted {
		// Create session directly via Go exec.Command to guarantee exact repoDir
		cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-c", repoDir)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to create tmux session: %w", err)
		}
	}

	// Connect client in Grubby iTerm window
	if TmuxHasAttachedClients() {
		_ = exec.Command("tmux", "switch-client", "-t", sessionName).Run()
		_ = activateOrOpenGrubbyITermWindow("")
	} else {
		attachCmd := fmt.Sprintf("tmux attach-session -t %s", sessionName)
		err := activateOrOpenGrubbyITermWindow(attachCmd)
		if err != nil {
			_ = activateOrOpenTerminalApp(attachCmd)
		}
	}

	if sessionAlreadyExisted {
		return fmt.Sprintf("Switched to session '%s'", sessionName), nil
	}
	return fmt.Sprintf("Created and switched to session '%s'", sessionName), nil
}
