package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const browserWinIDFile = "/tmp/grubby-chrome-win.id"

// OpenInDedicatedBrowser opens a URL in a dedicated, minimal Chrome app window without toolbars/bookmarks,
// and re-uses that exact same window for all subsequent URL opens.
func OpenInDedicatedBrowser(url string) error {
	chromeApp := "/Applications/Google Chrome.app"
	if _, err := os.Stat(chromeApp); err != nil {
		// Fallback to standard open command
		return exec.Command("open", url).Start()
	}

	// 1. Check if we have a saved window ID
	if data, err := os.ReadFile(browserWinIDFile); err == nil {
		winIDStr := strings.TrimSpace(string(data))
		if winID, err := strconv.Atoi(winIDStr); err == nil && winID > 0 {
			script := fmt.Sprintf(`
tell application "Google Chrome"
	try
		set w to window id %d
		set URL of active tab of w to %q
		set index of w to 1
		activate
		return "REUSED"
	on error
		return "NOT_FOUND"
	end try
end tell`, winID, url)

			out, err := exec.Command("osascript", "-e", script).Output()
			if err == nil && strings.TrimSpace(string(out)) == "REUSED" {
				return nil
			}
		}
	}

	// 2. Window not found or not created yet. Query existing window IDs before launching
	beforeScript := `
tell application "Google Chrome"
	try
		set idList to id of every window
		return idList
	on error
		return ""
	end try
end tell`
	beforeOut, _ := exec.Command("osascript", "-e", beforeScript).Output()
	beforeIDs := parseIDList(string(beforeOut))

	// 3. Launch Chrome in app mode (minimal dedicated frame, no bookmarks/toolbar)
	cmd := exec.Command("open", "-na", "Google Chrome", "--args", fmt.Sprintf("--app=%s", url))
	if err := cmd.Run(); err != nil {
		return exec.Command("open", url).Start()
	}

	// Wait briefly for the window to spawn
	time.Sleep(250 * time.Millisecond)

	// 4. Find the newly created window ID
	afterOut, _ := exec.Command("osascript", "-e", beforeScript).Output()
	afterIDs := parseIDList(string(afterOut))

	newWinID := 0
	for id := range afterIDs {
		if !beforeIDs[id] {
			newWinID = id
			break
		}
	}

	if newWinID == 0 {
		for id := range afterIDs {
			newWinID = id
			break
		}
	}

	if newWinID > 0 {
		_ = os.WriteFile(browserWinIDFile, []byte(strconv.Itoa(newWinID)), 0o644)
	}

	return nil
}

func parseIDList(raw string) map[int]bool {
	result := make(map[int]bool)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return result
	}
	parts := strings.Split(raw, ",")
	for _, p := range parts {
		if val, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
			result[val] = true
		}
	}
	return result
}
