package ui

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"grubby/internal/config"
)

var (
	nonAlphaNumRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	multiDashRegex   = regexp.MustCompile(`-+`)

	hasTmuxOnce  sync.Once
	hasTmuxCache bool

	tmuxSessionsMu    sync.RWMutex
	cachedTmuxMap     map[string]TmuxSessionInfo
	cachedTmuxExpires time.Time
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
	hasTmuxOnce.Do(func() {
		_, err := exec.LookPath("tmux")
		hasTmuxCache = (err == nil)
	})
	return hasTmuxCache
}

// InvalidateTmuxCache forces the next GetTmuxSessions call to re-query tmux.
func InvalidateTmuxCache() {
	tmuxSessionsMu.Lock()
	cachedTmuxExpires = time.Time{}
	tmuxSessionsMu.Unlock()
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

type TmuxSessionInfo struct {
	Windows      int
	LastActivity time.Time
}

// GetTmuxSessions returns a map of sessionName -> TmuxSessionInfo with window counts and activity timestamps.
// Uses a 5-second TTL in-memory cache to avoid spawning expensive subprocesses on rapid UI ticks.
func GetTmuxSessions() map[string]TmuxSessionInfo {
	if !HasTmux() {
		return make(map[string]TmuxSessionInfo)
	}

	tmuxSessionsMu.RLock()
	if cachedTmuxMap != nil && time.Now().Before(cachedTmuxExpires) {
		res := make(map[string]TmuxSessionInfo, len(cachedTmuxMap))
		for k, v := range cachedTmuxMap {
			res[k] = v
		}
		tmuxSessionsMu.RUnlock()
		return res
	}
	tmuxSessionsMu.RUnlock()

	tmuxSessionsMu.Lock()
	defer tmuxSessionsMu.Unlock()

	// Double check after acquiring write lock
	if cachedTmuxMap != nil && time.Now().Before(cachedTmuxExpires) {
		res := make(map[string]TmuxSessionInfo, len(cachedTmuxMap))
		for k, v := range cachedTmuxMap {
			res[k] = v
		}
		return res
	}

	sessions := make(map[string]TmuxSessionInfo)
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}:#{session_windows}:#{session_activity}").Output()
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			parts := strings.SplitN(line, ":", 3)
			if len(parts) >= 2 {
				var count int
				_, _ = fmt.Sscanf(parts[1], "%d", &count)
				var actTime time.Time
				if len(parts) == 3 {
					var unixSec int64
					if _, err := fmt.Sscanf(parts[2], "%d", &unixSec); err == nil && unixSec > 0 {
						actTime = time.Unix(unixSec, 0)
					}
				}
				sessions[strings.ToLower(parts[0])] = TmuxSessionInfo{
					Windows:      count,
					LastActivity: actTime,
				}
			}
		}
	}

	cachedTmuxMap = sessions
	cachedTmuxExpires = time.Now().Add(5 * time.Second)

	res := make(map[string]TmuxSessionInfo, len(sessions))
	for k, v := range sessions {
		res[k] = v
	}
	return res
}

// FindTmuxSession finds a session by repo name or sanitized tmux session name.
func FindTmuxSession(repoName string, tmuxSessions map[string]TmuxSessionInfo) (TmuxSessionInfo, bool) {
	sanitized := SanitizeTmuxSessionName(repoName)
	for name, info := range tmuxSessions {
		if strings.EqualFold(name, sanitized) || strings.EqualFold(name, repoName) {
			return info, true
		}
	}
	return TmuxSessionInfo{}, false
}

// GetTmuxSessionsMap returns a map of sessionName -> windowCount for all active tmux sessions.
func GetTmuxSessionsMap() map[string]int {
	sess := GetTmuxSessions()
	res := make(map[string]int, len(sess))
	for name, info := range sess {
		res[name] = info.Windows
	}
	return res
}

// getAttachedTmuxClientTTY returns the tty of the attached tmux client (e.g. /dev/ttys002) if any.
func getAttachedTmuxClientTTY() string {
	if !HasTmux() {
		return ""
	}
	out, err := exec.Command("tmux", "list-clients", "-F", "#{client_tty}").Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 0 && lines[0] != "" {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

// activateOrOpenGrubbyITermWindow ensures an iTerm2 tab is focused or created.
// If a tab matching tabTitle or the attached tmux client tty exists, it switches to that tab.
// Otherwise, it creates a new tab with default profile, names the session tabTitle, and runs writeCmd.
func activateOrOpenGrubbyITermWindow(writeCmd string, tabTitle string) error {
	if _, err := os.Stat("/Applications/iTerm.app"); err != nil {
		return fmt.Errorf("iTerm not installed")
	}

	clientTTY := ""
	if TmuxHasAttachedClients() {
		clientTTY = getAttachedTmuxClientTTY()
	}

	script := fmt.Sprintf(`
tell application "iTerm"
	activate
	set targetTTY to %q
	set targetName to %q
	set cmdToRun to %q
	set found to false
	repeat with w in windows
		repeat with t in tabs of w
			repeat with s in sessions of t
				if targetTTY is not "" and (tty of s) is equal to targetTTY then
					tell t to select
					tell w to select
					set found to true
					exit repeat
				else if targetName is not "" and ((name of s) contains targetName) then
					tell t to select
					tell w to select
					set found to true
					exit repeat
				end if
			end repeat
			if found then exit repeat
		end repeat
		if found then exit repeat
	end repeat
	if not found and cmdToRun is not "" then
		try
			tell current window
				create tab with default profile
				tell current session
					if targetName is not "" then set name to targetName
					write text cmdToRun
				end tell
			end tell
		on error
			create window with default profile
			tell current session of current window
				if targetName is not "" then set name to targetName
				write text cmdToRun
			end tell
		end try
	end if
end tell`, clientTTY, tabTitle, writeCmd)

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
		err := activateOrOpenGrubbyITermWindow(shellCmd, repoName)
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
		_ = activateOrOpenGrubbyITermWindow("", sessionName)
	} else {
		attachCmd := fmt.Sprintf("tmux attach-session -t %s", sessionName)
		err := activateOrOpenGrubbyITermWindow(attachCmd, sessionName)
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
		err := activateOrOpenGrubbyITermWindow(shellCmd, repoName)
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
		InvalidateTmuxCache()
	}

	// Connect client in Grubby iTerm window
	if TmuxHasAttachedClients() {
		_ = exec.Command("tmux", "switch-client", "-t", sessionName).Run()
		_ = activateOrOpenGrubbyITermWindow("", sessionName)
	} else {
		attachCmd := fmt.Sprintf("tmux attach-session -t %s", sessionName)
		err := activateOrOpenGrubbyITermWindow(attachCmd, sessionName)
		if err != nil {
			_ = activateOrOpenTerminalApp(attachCmd)
		}
	}

	if sessionAlreadyExisted {
		return fmt.Sprintf("Switched to session '%s'", sessionName), nil
	}
	return fmt.Sprintf("Created and switched to session '%s'", sessionName), nil
}

// OpenRepoEditor opens the editor in a new window in the repository's tmux session:
// - If the session exists, creates a new window running the editor at repoDir.
// - If the session does not exist, creates the session with a window running the editor at repoDir.
// - Ensures the session is displayed in the dedicated Grubby iTerm window.
func OpenRepoEditor(repoName, repoDir string) (string, error) {
	sessionName := SanitizeTmuxSessionName(repoName)
	tmuxInstalled := HasTmux()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nvim"
	}

	if !tmuxInstalled {
		err := OpenInITermNvim(repoDir, "", 0)
		collapsed := config.CollapseHome(repoDir)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Opened %s at %s", editor, collapsed), nil
	}

	sessionAlreadyExisted := TmuxSessionExists(sessionName)
	editorParts := strings.Fields(editor)
	bin := editorParts[0]
	args := append(editorParts[1:], ".")

	if !sessionAlreadyExisted {
		// Create session directly with editor as initial command
		cmdArgs := append([]string{"new-session", "-d", "-s", sessionName, "-n", "editor", "-c", repoDir, bin}, args...)
		cmd := exec.Command("tmux", cmdArgs...)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to create tmux session: %w", err)
		}
	} else {
		// Create a new window running editor inside existing session
		cmdArgs := append([]string{"new-window", "-t", sessionName, "-n", "editor", "-c", repoDir, bin}, args...)
		cmd := exec.Command("tmux", cmdArgs...)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to create editor window in tmux: %w", err)
		}
	}

	// Connect client in Grubby iTerm window
	if TmuxHasAttachedClients() {
		_ = exec.Command("tmux", "switch-client", "-t", sessionName).Run()
		_ = activateOrOpenGrubbyITermWindow("", sessionName)
	} else {
		attachCmd := fmt.Sprintf("tmux attach-session -t %s", sessionName)
		err := activateOrOpenGrubbyITermWindow(attachCmd, sessionName)
		if err != nil {
			_ = activateOrOpenTerminalApp(attachCmd)
		}
	}

	if sessionAlreadyExisted {
		return fmt.Sprintf("Opened %s in new window in session '%s'", bin, sessionName), nil
	}
	return fmt.Sprintf("Created session '%s' and opened %s", sessionName, bin), nil
}
