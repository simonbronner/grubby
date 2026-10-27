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

	// 1. Check if we have a saved window ID and if it is still open
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

	// 2. Window not found or closed. Record existing window IDs before launching
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

	// 3. Launch Chrome binary directly with --app=<url> so it creates a clean, dedicated chromeless app window.
	chromeBin := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	cmd := exec.Command(chromeBin, fmt.Sprintf("--app=%s", url))
	_ = cmd.Start()

	// 4. Poll for the newly created window ID
	newWinID := 0
	for attempt := 0; attempt < 10; attempt++ {
		time.Sleep(150 * time.Millisecond)
		afterOut, _ := exec.Command("osascript", "-e", beforeScript).Output()
		afterIDs := parseIDList(string(afterOut))

		for id := range afterIDs {
			if !beforeIDs[id] {
				newWinID = id
				break
			}
		}
		if newWinID > 0 {
			break
		}
	}

	if newWinID > 0 {
		_ = os.WriteFile(browserWinIDFile, []byte(strconv.Itoa(newWinID)), 0o644)
		// Ensure the new window is brought to front
		activateScript := fmt.Sprintf(`
tell application "Google Chrome"
	try
		set index of (window id %d) to 1
		activate
	end try
end tell`, newWinID)
		_ = exec.Command("osascript", "-e", activateScript).Run()
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
