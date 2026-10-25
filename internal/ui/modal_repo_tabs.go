package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type RepoTabOption struct {
	Name string
	Path string
}

var repoTabOptions = []RepoTabOption{
	{Name: "Code (Overview)", Path: ""},
	{Name: "Pull Requests", Path: "/pulls"},
	{Name: "Issues", Path: "/issues"},
	{Name: "Actions (CI/CD)", Path: "/actions"},
	{Name: "Security (Dependabot)", Path: "/security/dependabot"},
	{Name: "Releases & Tags", Path: "/releases"},
	{Name: "Insights (Pulse)", Path: "/pulse"},
	{Name: "Settings", Path: "/settings"},
}

func (m Model) renderRepoTabsModal() string {
	if len(m.Repos) == 0 || m.RepoCursor >= len(m.Repos) {
		return ""
	}
	repo := m.Repos[m.RepoCursor]

	title := ModalTitleStyle.Render(fmt.Sprintf("Open GitHub Tab: %s", repo.FullName))

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "Use [↑/↓] or [1-8] to choose tab, [Enter] to open in dedicated browser, [Esc] to exit.\n")

	for i, opt := range repoTabOptions {
		cursor := "  "
		if i == m.ModalCursor {
			cursor = "▶ "
		}

		url := fmt.Sprintf("https://github.com/%s%s", repo.FullName, opt.Path)
		line := fmt.Sprintf("%s[%d] %-24s %s", cursor, i+1, opt.Name, url)
		if i == m.ModalCursor {
			lines = append(lines, SelectedRowStyle.Render(line))
		} else {
			lines = append(lines, NormalRowStyle.Render(line))
		}
	}

	return ModalBoxStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) handleRepoTabsModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.Repos) == 0 || m.RepoCursor >= len(m.Repos) {
		m.ActiveModal = ModalNone
		return m, nil
	}
	repo := m.Repos[m.RepoCursor]

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
			url := fmt.Sprintf("https://github.com/%s%s", repo.FullName, opt.Path)
			_ = OpenBrowser(url)
			return m, func() tea.Msg {
				return StatusToastMsg(fmt.Sprintf("Opened %s %s in dedicated browser", repo.Name, opt.Name))
			}
		}
	}

	return m, nil
}
