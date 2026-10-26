package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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

	title := ModalTitleStyle.Render(fmt.Sprintf("Open GitHub Tab: %s", repo.FullName))

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "Use [↑/↓] or [c/p/i/a/d/r/n/s] to choose tab, [Enter] to open in dedicated browser, [Esc] to exit.\n")

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
		return m, nil
	}

	key := strings.ToLower(msg.String())

	// Match mnemonic letter hotkeys
	for _, opt := range repoTabOptions {
		if key == opt.Key {
			m.ActiveModal = ModalNone
			m = m.recordRepoInteraction(repo.Name)
			m = m.focusRepoCursor(repo.Name)
			url := fmt.Sprintf("https://github.com/%s%s", repo.FullName, opt.Path)
			_ = OpenBrowser(url)
			return m, func() tea.Msg {
				return StatusToastMsg(fmt.Sprintf("Opened %s %s in dedicated browser", repo.Name, opt.Name))
			}
		}
	}

	switch msg.String() {
	case "esc", "q":
		m.ActiveModal = ModalNone
		return m, nil
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
			m = m.recordRepoInteraction(repo.Name)
			m = m.focusRepoCursor(repo.Name)
			url := fmt.Sprintf("https://github.com/%s%s", repo.FullName, opt.Path)
			_ = OpenBrowser(url)
			return m, func() tea.Msg {
				return StatusToastMsg(fmt.Sprintf("Opened %s %s in dedicated browser", repo.Name, opt.Name))
			}
		}
	case "enter":
		if m.ModalCursor >= 0 && m.ModalCursor < len(repoTabOptions) {
			opt := repoTabOptions[m.ModalCursor]
			m.ActiveModal = ModalNone
			m = m.recordRepoInteraction(repo.Name)
			m = m.focusRepoCursor(repo.Name)
			url := fmt.Sprintf("https://github.com/%s%s", repo.FullName, opt.Path)
			_ = OpenBrowser(url)
			return m, func() tea.Msg {
				return StatusToastMsg(fmt.Sprintf("Opened %s %s in dedicated browser", repo.Name, opt.Name))
			}
		}
	}

	return m, nil
}
