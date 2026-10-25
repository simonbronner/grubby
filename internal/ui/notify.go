package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SendNativeNotification triggers a silent native macOS notification bubble / banner.
// If terminal-notifier is available, it sends with the custom Grubby hobbit icon.
// Otherwise, it seamlessly falls back to macOS native osascript display notification.
func SendNativeNotification(title, subtitle, message string) {
	go func() {
		// Check for terminal-notifier for custom icon support
		if tnPath, err := exec.LookPath("terminal-notifier"); err == nil {
			args := []string{"-title", title, "-message", message}
			if subtitle != "" {
				args = append(args, "-subtitle", subtitle)
			}
			// Search for Grubby custom icon
			for _, candidate := range []string{
				"internal/assets/grubby-icon.png",
				filepath.Join(os.Getenv("HOME"), ".config/grubby/grubby-icon.png"),
			} {
				if abs, err := filepath.Abs(candidate); err == nil {
					if _, err := os.Stat(abs); err == nil {
						args = append(args, "-appIcon", abs)
						break
					}
				}
			}
			cmd := exec.Command(tnPath, args...)
			if err := cmd.Run(); err == nil {
				return
			}
		}

		// Fallback to silent AppleScript display notification
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
			script = fmt.Sprintf(`display notification "%s" with title "%s" subtitle "%s"`, sMsg, sTitle, sSub)
		} else {
			script = fmt.Sprintf(`display notification "%s" with title "%s"`, sMsg, sTitle)
		}

		cmd := exec.Command("osascript", "-e", script)
		_ = cmd.Run()
	}()
}
