package ui

import (
	"fmt"
	"os/exec"
	"strings"
)

// SendNativeNotification triggers a native macOS notification bubble / banner with sound.
// It executes asynchronously in a separate goroutine so it never blocks the Bubble Tea
// event loop or TUI rendering.
func SendNativeNotification(title, subtitle, message string) {
	go func() {
		sanitize := func(s string) string {
			s = strings.ReplaceAll(s, `\`, `\\`)
			s = strings.ReplaceAll(s, `"`, `\"`)
			s = strings.ReplaceAll(s, "\n", " ")
			s = strings.ReplaceAll(s, "\r", "")
			return s
		}

		sMsg := sanitize(message)
		sTitle := sanitize(title)
		sSub := sanitize(subtitle)

		var script string
		if sSub != "" {
			script = fmt.Sprintf(`display notification "%s" with title "%s" subtitle "%s" sound name "default"`, sMsg, sTitle, sSub)
		} else {
			script = fmt.Sprintf(`display notification "%s" with title "%s" sound name "default"`, sMsg, sTitle)
		}

		cmd := exec.Command("osascript", "-e", script)
		_ = cmd.Run()
	}()
}
