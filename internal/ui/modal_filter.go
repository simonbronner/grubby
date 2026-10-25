package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// View by Group Modal

func (m Model) renderFilterGroupModal() string {
	title := ModalTitleStyle.Render("View by Group")

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "Select a group to filter repositories, or [All Groups] to view all. [Esc] to exit.\n")

	groups := m.Config.Groups

	// Option 0: All Groups
	cursor := "  "
	if m.ModalCursor == 0 {
		cursor = "▶ "
	}
	allPrefix := "[ ]"
	if m.RepoGroupFilter == "" {
		allPrefix = "[✓]"
	}
	allLine := fmt.Sprintf("%s%s All Groups (%d total repositories)", cursor, allPrefix, len(m.Repos))
	if m.ModalCursor == 0 {
		lines = append(lines, SelectedRowStyle.Render(allLine))
	} else {
		lines = append(lines, NormalRowStyle.Render(allLine))
	}

	for i, g := range groups {
		cursor = "  "
		if m.ModalCursor == i+1 {
			cursor = "▶ "
		}
		checked := "[ ]"
		if strings.EqualFold(m.RepoGroupFilter, g.Name) {
			checked = "[✓]"
		}

		line := fmt.Sprintf("%s%s %-20s (%d repos)", cursor, checked, g.DisplayName, len(g.Repos))
		if m.ModalCursor == i+1 {
			lines = append(lines, SelectedRowStyle.Render(line))
		} else {
			lines = append(lines, NormalRowStyle.Render(line))
		}
	}

	if len(groups) == 0 {
		lines = append(lines, "\n  No groups created yet. Press [g] on any repository to create one!")
	}

	return ModalBoxStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) handleFilterGroupModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	groups := m.Config.Groups
	totalOptions := 1 + len(groups)

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
			return m, func() tea.Msg { return StatusToastMsg("Showing all repository groups") }
		}
		selected := groups[m.ModalCursor-1]
		m.RepoGroupFilter = selected.Name
		return m, func() tea.Msg {
			return StatusToastMsg(fmt.Sprintf("Filtered by group: %s (%d repos)", selected.DisplayName, len(selected.Repos)))
		}
	}
	return m, nil
}

// Filter by Name Modal

func (m Model) renderFilterNameModal() string {
	title := ModalTitleStyle.Render("Filter Repositories by Name")

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "Type repository name to filter. Press [Enter] to apply, [Esc] to exit.\n")
	lines = append(lines, m.RepoFilterInput.View())

	// Live match preview
	query := strings.TrimSpace(strings.ToLower(m.RepoFilterInput.Value()))
	matchCount := 0
	for _, r := range m.Repos {
		if query == "" || strings.Contains(strings.ToLower(r.Name), query) || strings.Contains(strings.ToLower(r.FullName), query) {
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
				return StatusToastMsg(fmt.Sprintf("Filtered by name: %q", m.RepoFilter))
			}
		}
		return m, func() tea.Msg { return StatusToastMsg("Cleared name filter") }
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
