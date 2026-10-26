package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) getAllAvailableLabels() []string {
	seen := make(map[string]bool)
	var result []string

	add := func(l string) {
		clean := strings.TrimPrefix(strings.TrimSpace(l), "#")
		clean = strings.ReplaceAll(clean, " ", "-")
		clean = strings.ToLower(clean)
		if clean != "" && !seen[clean] {
			seen[clean] = true
			result = append(result, clean)
		}
	}

	if m.DB != nil && m.Config != nil {
		if dbLabels, err := m.DB.GetAllUniqueLabels(m.Config.ActiveSquad); err == nil {
			for _, l := range dbLabels {
				add(l)
			}
		}
	}
	for _, list := range m.RepoLabels {
		for _, l := range list {
			add(l)
		}
	}
	for _, list := range m.RepoLabelsByName {
		for _, l := range list {
			add(l)
		}
	}
	if m.Config != nil {
		for _, g := range m.Config.GetAllGroupNames() {
			add(g)
		}
	}
	for l, selected := range m.SelectedRepoGroups {
		if selected {
			add(l)
		}
	}

	sort.Strings(result)
	return result
}

func (m Model) renderManageGroupsModal() string {
	repo := m.getSelectedRepo()
	if repo == nil {
		return ""
	}

	title := ModalTitleStyle.Render(fmt.Sprintf("Manage Labels for '%s'", repo.Name))

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "Press [Space] to toggle, [+] to add a new label, [Esc] to exit.\n")

	labels := m.getAllAvailableLabels()
	if len(labels) == 0 {
		lines = append(lines, "  No labels created yet. Press [+] to create one!\n")
	}

	for i, l := range labels {
		checked := "[ ]"
		if m.SelectedRepoGroups[l] {
			checked = "[x]"
		}

		cursor := "  "
		if i == m.ModalCursor {
			cursor = "▶ "
		}

		colorHex := m.getLabelColor(l)
		styledLbl := lipgloss.NewStyle().Foreground(lipgloss.Color("#" + strings.TrimPrefix(colorHex, "#"))).Render("#" + l)
		line := fmt.Sprintf("%s%s %s", cursor, checked, styledLbl)
		if i == m.ModalCursor {
			lines = append(lines, SelectedRowStyle.Render(line))
		} else {
			lines = append(lines, NormalRowStyle.Render(line))
		}
	}

	if m.ModalInput.Focused() {
		lines = append(lines, "\nNew Label Name: "+m.ModalInput.View())
		lines = append(lines, MetaBarStyle.Render("(No spaces allowed. Press Enter to save)"))
	}

	content := strings.Join(lines, "\n")
	return ModalBoxStyle.Render(content)
}

func (m Model) handleGroupsModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	repo := m.getSelectedRepo()
	if repo == nil {
		m.ActiveModal = ModalNone
		return m, nil
	}

	if m.ModalInput.Focused() {
		switch msg.String() {
		case "enter":
			val := strings.TrimSpace(m.ModalInput.Value())
			val = strings.TrimPrefix(val, "#")
			val = strings.ReplaceAll(val, " ", "-")
			val = strings.ToLower(val)
			if val != "" {
				if m.SelectedRepoGroups == nil {
					m.SelectedRepoGroups = make(map[string]bool)
				}
				m.SelectedRepoGroups[val] = true

				var active []string
				for k, v := range m.SelectedRepoGroups {
					if v {
						active = append(active, k)
					}
				}
				sort.Strings(active)

				if m.Config != nil {
					_, _ = m.Config.ToggleRepoInGroup(repo.Name, val)
				}
				if m.DB != nil && m.Config != nil {
					_ = m.DB.SaveRepoLabels(repo.ID, repo.Name, active, m.Config.ActiveSquad)
				}
				if m.RepoLabels == nil {
					m.RepoLabels = make(map[int64][]string)
				}
				if repo.ID > 0 {
					m.RepoLabels[repo.ID] = active
				}
				if m.RepoLabelsByName == nil {
					m.RepoLabelsByName = make(map[string][]string)
				}
				m.RepoLabelsByName[strings.ToLower(repo.Name)] = active

				m.ModalInput.Reset()
				m.ModalInput.Blur()
				cmd := m.syncSquadRepoLabelsCmd(repo.ID, repo.Name, active)
				return m, tea.Batch(
					cmd,
					func() tea.Msg { return StatusToastMsg(fmt.Sprintf("Added #%s to %s", val, repo.Name)) },
				)
			}
		case "esc":
			m.ModalInput.Reset()
			m.ModalInput.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.ModalInput, cmd = m.ModalInput.Update(msg)
		return m, cmd
	}

	labels := m.getAllAvailableLabels()

	switch msg.String() {
	case "esc":
		m.ActiveModal = ModalNone
		return m, nil
	case "up", "k":
		if m.ModalCursor > 0 {
			m.ModalCursor--
		}
	case "down", "j":
		if m.ModalCursor < len(labels)-1 {
			m.ModalCursor++
		}
	case " ":
		// Toggle label membership
		if len(labels) > 0 && m.ModalCursor < len(labels) {
			targetLabel := labels[m.ModalCursor]
			if m.SelectedRepoGroups == nil {
				m.SelectedRepoGroups = make(map[string]bool)
			}
			isCurrentlySelected := m.SelectedRepoGroups[targetLabel]
			if isCurrentlySelected {
				delete(m.SelectedRepoGroups, targetLabel)
			} else {
				m.SelectedRepoGroups[targetLabel] = true
			}
			isAdded := !isCurrentlySelected

			var active []string
			for k, v := range m.SelectedRepoGroups {
				if v {
					active = append(active, k)
				}
			}
			sort.Strings(active)

			if m.Config != nil {
				_, _ = m.Config.ToggleRepoInGroup(repo.Name, targetLabel)
			}
			if m.DB != nil && m.Config != nil {
				_ = m.DB.SaveRepoLabels(repo.ID, repo.Name, active, m.Config.ActiveSquad)
			}
			if m.RepoLabels == nil {
				m.RepoLabels = make(map[int64][]string)
			}
			if repo.ID > 0 {
				m.RepoLabels[repo.ID] = active
			}
			if m.RepoLabelsByName == nil {
				m.RepoLabelsByName = make(map[string][]string)
			}
			m.RepoLabelsByName[strings.ToLower(repo.Name)] = active

			action := "Removed"
			if isAdded {
				action = "Added"
			}
			cmd := m.syncSquadRepoLabelsCmd(repo.ID, repo.Name, active)
			return m, tea.Batch(
				cmd,
				func() tea.Msg {
					return StatusToastMsg(fmt.Sprintf("%s #%s on '%s'", action, targetLabel, repo.Name))
				},
			)
		}
	case "+", "n":
		m.ModalInput.Placeholder = "e.g. backend, payments"
		m.ModalInput.Focus()
		return m, nil
	}

	return m, nil
}
