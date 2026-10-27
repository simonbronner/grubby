package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type RepoTabOption struct {
	Name string
	Key  string
	Path string
}

var repoTabOptions = []RepoTabOption{
	{Name: "Code (Overview)", Key: "c", Path: ""},
	{Name: "Pull Requests", Key: "p", Path: "/pulls"},
	{Name: "Issues", Key: "i", Path: "/issues"},
	{Name: "Actions (CI/CD)", Key: "a", Path: "/actions"},
	{Name: "Dependabot (Security)", Key: "d", Path: "/security/dependabot"},
	{Name: "Releases & Tags", Key: "r", Path: "/releases"},
	{Name: "Insights (Pulse)", Key: "n", Path: "/pulse"},
	{Name: "Settings", Key: "s", Path: "/settings"},
}

func (m Model) renderRepoTabsModal() string {
	repo := m.getSelectedRepo()
	if repo == nil {
		return ""
	}

	var title string
	var hint string
	if m.RepoTabsYankLeader {
		title = ModalTitleStyle.Render(fmt.Sprintf("📋 Yank URL to Clipboard: %s", repo.FullName))
		hint = lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render(
			"📋 YANK MODE: Press [c/p/i/a/d/r/n/s] or [Enter/y] to copy URL to clipboard, [Esc] to cancel.\n",
		)
	} else {
		title = ModalTitleStyle.Render(fmt.Sprintf("Open GitHub Tab: %s", repo.FullName))
		hint = "Use [↑/↓] or [c/p/i/a/d/r/n/s], [Enter] to open in browser, [y] Yank leader, [Esc] to exit.\n"
	}

	var lines []string
	lines = append(lines, title)
	lines = append(lines, hint)

	for i, opt := range repoTabOptions {
		cursor := "  "
		if i == m.ModalCursor {
			cursor = "▶ "
		}

		url := fmt.Sprintf("https://github.com/%s%s", repo.FullName, opt.Path)
		line := fmt.Sprintf("%s[%s] %-24s %s", cursor, opt.Key, opt.Name, url)
		if i == m.ModalCursor {
			lines = append(lines, SelectedRowStyle.Render(line))
		} else {
			lines = append(lines, NormalRowStyle.Render(line))
		}
	}

	return ModalBoxStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) handleRepoTabsModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	repo := m.getSelectedRepo()
	if repo == nil {
		m.ActiveModal = ModalNone
		m.RepoTabsYankLeader = false
		return m, nil
	}

	key := strings.ToLower(msg.String())

	// Handle 'y' leader key
	if key == "y" {
		if !m.RepoTabsYankLeader {
			m.RepoTabsYankLeader = true
			return m, nil
		}
		// Pressing 'y' again while in yank mode copies currently highlighted option's URL
		if m.ModalCursor >= 0 && m.ModalCursor < len(repoTabOptions) {
			opt := repoTabOptions[m.ModalCursor]
			m.ActiveModal = ModalNone
			m.RepoTabsYankLeader = false
			m = m.recordRepoInteraction(repo.Name)
			m = m.focusRepoCursor(repo.Name)
			url := fmt.Sprintf("https://github.com/%s%s", repo.FullName, opt.Path)
			_ = CopyToClipboard(url)
			return m, func() tea.Msg {
				return StatusToastMsg(fmt.Sprintf("Copied %s URL to clipboard: %s", opt.Name, url))
			}
		}
		m.RepoTabsYankLeader = false
		return m, nil
	}

	if key == "esc" {
		if m.RepoTabsYankLeader {
			m.RepoTabsYankLeader = false
			return m, nil
		}
		m.ActiveModal = ModalNone
		return m, nil
	}

	if key == "q" {
		m.ActiveModal = ModalNone
		m.RepoTabsYankLeader = false
		return m, nil
	}

	// Match mnemonic letter hotkeys
	for _, opt := range repoTabOptions {
		if key == opt.Key {
			m.ActiveModal = ModalNone
			url := fmt.Sprintf("https://github.com/%s%s", repo.FullName, opt.Path)
			if m.RepoTabsYankLeader {
				m.RepoTabsYankLeader = false
				_ = CopyToClipboard(url)
				return m, func() tea.Msg {
					return StatusToastMsg(fmt.Sprintf("Copied %s URL to clipboard: %s", opt.Name, url))
				}
			}
			m = m.recordRepoInteraction(repo.Name)
			m = m.focusRepoCursor(repo.Name)
			_ = OpenBrowser(url)
			return m, func() tea.Msg {
				return StatusToastMsg(fmt.Sprintf("Opened %s %s in dedicated browser", repo.Name, opt.Name))
			}
		}
	}

	switch msg.String() {
	case "up", "k":
		if m.ModalCursor > 0 {
			m.ModalCursor--
		}
	case "down", "j":
		if m.ModalCursor < len(repoTabOptions)-1 {
			m.ModalCursor++
		}
	case "1", "2", "3", "4", "5", "6", "7", "8":
		idx := int(msg.String()[0] - '1')
		if idx >= 0 && idx < len(repoTabOptions) {
			opt := repoTabOptions[idx]
			m.ActiveModal = ModalNone
			url := fmt.Sprintf("https://github.com/%s%s", repo.FullName, opt.Path)
			if m.RepoTabsYankLeader {
				m.RepoTabsYankLeader = false
				_ = CopyToClipboard(url)
				return m, func() tea.Msg {
					return StatusToastMsg(fmt.Sprintf("Copied %s URL to clipboard: %s", opt.Name, url))
				}
			}
			m = m.recordRepoInteraction(repo.Name)
			m = m.focusRepoCursor(repo.Name)
			_ = OpenBrowser(url)
			return m, func() tea.Msg {
				return StatusToastMsg(fmt.Sprintf("Opened %s %s in dedicated browser", repo.Name, opt.Name))
			}
		}
	case "enter":
		if m.ModalCursor >= 0 && m.ModalCursor < len(repoTabOptions) {
			opt := repoTabOptions[m.ModalCursor]
			m.ActiveModal = ModalNone
			url := fmt.Sprintf("https://github.com/%s%s", repo.FullName, opt.Path)
			if m.RepoTabsYankLeader {
				m.RepoTabsYankLeader = false
				_ = CopyToClipboard(url)
				return m, func() tea.Msg {
					return StatusToastMsg(fmt.Sprintf("Copied %s URL to clipboard: %s", opt.Name, url))
				}
			}
			m = m.recordRepoInteraction(repo.Name)
			m = m.focusRepoCursor(repo.Name)
			_ = OpenBrowser(url)
			return m, func() tea.Msg {
				return StatusToastMsg(fmt.Sprintf("Opened %s %s in dedicated browser", repo.Name, opt.Name))
			}
		}
	}

	return m, nil
}
