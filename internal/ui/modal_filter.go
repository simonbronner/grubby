package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// View by Group/Label Modal

func (m Model) renderFilterGroupModal() string {
	title := ModalTitleStyle.Render("Filter by Label [#]")

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "Select a label to filter repositories, or [All Repositories] to view all. [Esc] to exit.\n")

	labels := m.getAllAvailableLabels()

	// Option 0: All Repositories
	cursor := "  "
	if m.ModalCursor == 0 {
		cursor = "▶ "
	}
	allPrefix := "[ ]"
	if m.RepoGroupFilter == "" {
		allPrefix = "[✓]"
	}
	allLine := fmt.Sprintf("%s%s All Repositories (%d total)", cursor, allPrefix, len(m.Repos))
	if m.ModalCursor == 0 {
		lines = append(lines, SelectedRowStyle.Render(allLine))
	} else {
		lines = append(lines, NormalRowStyle.Render(allLine))
	}

	for i, l := range labels {
		cursor = "  "
		if m.ModalCursor == i+1 {
			cursor = "▶ "
		}
		checked := "[ ]"
		cleanFilter := strings.ToLower(strings.TrimPrefix(m.RepoGroupFilter, "#"))
		if cleanFilter != "" && strings.EqualFold(cleanFilter, l) {
			checked = "[✓]"
		}

		cnt := 0
		for _, r := range m.Repos {
			for _, repoLbl := range m.getRepoLabels(&r) {
				if strings.EqualFold(repoLbl, l) {
					cnt++
					break
				}
			}
		}

		colorHex := m.getLabelColor(l)
		styledLbl := lipgloss.NewStyle().Foreground(lipgloss.Color("#" + strings.TrimPrefix(colorHex, "#"))).Render(fmt.Sprintf("#%-20s", l))
		line := fmt.Sprintf("%s%s %s (%d repos)", cursor, checked, styledLbl, cnt)
		if m.ModalCursor == i+1 {
			lines = append(lines, SelectedRowStyle.Render(line))
		} else {
			lines = append(lines, NormalRowStyle.Render(line))
		}
	}

	if len(labels) == 0 {
		lines = append(lines, "\n  No labels created yet. Press [g] on any repository to create one!")
	}

	return ModalBoxStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) handleFilterGroupModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	labels := m.getAllAvailableLabels()
	totalOptions := 1 + len(labels)

	switch msg.String() {
	case "esc", "q":
		m.ActiveModal = ModalNone
		return m, nil
	case "up", "k":
		if m.ModalCursor > 0 {
			m.ModalCursor--
		}
		return m, nil
	case "down", "j":
		if m.ModalCursor < totalOptions-1 {
			m.ModalCursor++
		}
		return m, nil
	case "enter":
		m.ActiveModal = ModalNone
		m.RepoCursor = 0
		if m.ModalCursor == 0 {
			m.RepoGroupFilter = ""
			return m, func() tea.Msg { return StatusToastMsg("Showing all repositories") }
		}
		selected := labels[m.ModalCursor-1]
		m.RepoGroupFilter = selected
		cnt := 0
		for _, r := range m.Repos {
			for _, repoLbl := range m.getRepoLabels(&r) {
				if strings.EqualFold(repoLbl, selected) {
					cnt++
					break
				}
			}
		}
		return m, func() tea.Msg {
			return StatusToastMsg(fmt.Sprintf("Filtered by label: #%s (%d repos)", selected, cnt))
		}
	}
	return m, nil
}

// Filter Repositories Modal (Name or Group)

func (m Model) renderFilterNameModal() string {
	title := ModalTitleStyle.Render("Filter Repositories")

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "Type keyword to filter by repository name or group name.\nPress [Enter] to apply, [Esc] to exit.\n")
	lines = append(lines, m.RepoFilterInput.View())

	// Live match preview
	query := strings.TrimSpace(strings.ToLower(m.RepoFilterInput.Value()))
	matchCount := 0
	for _, r := range m.Repos {
		if query == "" {
			matchCount++
			continue
		}
		matchesName := strings.Contains(strings.ToLower(r.Name), query) || strings.Contains(strings.ToLower(r.FullName), query)
		matchesGroup := false
		if m.Config != nil {
			for _, grp := range m.Config.GetGroupsForRepo(r.Name) {
				if strings.Contains(strings.ToLower(grp), query) {
					matchesGroup = true
					break
				}
			}
		}
		if matchesName || matchesGroup {
			matchCount++
		}
	}

	lines = append(lines, "")
	lines = append(lines, MetaBarStyle.Render(fmt.Sprintf("Matches: %d of %d repositories", matchCount, len(m.Repos))))

	return ModalBoxStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) handleFilterNameModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.ActiveModal = ModalNone
		return m, nil
	case "enter":
		m.RepoFilter = strings.TrimSpace(m.RepoFilterInput.Value())
		m.RepoCursor = 0
		m.ActiveModal = ModalNone
		if m.RepoFilter != "" {
			return m, func() tea.Msg {
				return StatusToastMsg(fmt.Sprintf("Filtered by: %q", m.RepoFilter))
			}
		}
		return m, func() tea.Msg { return StatusToastMsg("Cleared filter") }
	default:
		var cmd tea.Cmd
		m.RepoFilterInput, cmd = m.RepoFilterInput.Update(msg)
		return m, cmd
	}
}

// Filter PRs Modal

func (m Model) renderFilterPRModal() string {
	title := ModalTitleStyle.Render("Filter Pull Requests")

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "Type keyword to filter PRs by title, repository, or author.\nPress [Enter] to apply, [Esc] to exit.\n")
	lines = append(lines, m.PRFilterInput.View())

	// Live match preview
	query := strings.TrimSpace(strings.ToLower(m.PRFilterInput.Value()))
	matchCount := 0
	for _, pr := range m.PRs {
		if query == "" || strings.Contains(strings.ToLower(pr.Title), query) ||
			strings.Contains(strings.ToLower(pr.Repo), query) ||
			strings.Contains(strings.ToLower(pr.Author), query) {
			matchCount++
		}
	}

	lines = append(lines, "")
	lines = append(lines, MetaBarStyle.Render(fmt.Sprintf("Matches: %d of %d pull requests", matchCount, len(m.PRs))))

	return ModalBoxStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) handleFilterPRModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.ActiveModal = ModalNone
		return m, nil
	case "enter":
		m.PRFilter = strings.TrimSpace(m.PRFilterInput.Value())
		m.PRCursor = 0
		m.ActiveModal = ModalNone
		if m.PRFilter != "" {
			return m, func() tea.Msg {
				return StatusToastMsg(fmt.Sprintf("Filtered PRs by: %q", m.PRFilter))
			}
		}
		return m, func() tea.Msg { return StatusToastMsg("Cleared pull request filter") }
	default:
		var cmd tea.Cmd
		m.PRFilterInput, cmd = m.PRFilterInput.Update(msg)
		return m, cmd
	}
}
