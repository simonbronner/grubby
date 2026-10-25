package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) renderManageGroupsModal() string {
	if len(m.Repos) == 0 || m.RepoCursor >= len(m.Repos) {
		return ""
	}
	repo := m.Repos[m.RepoCursor]

	title := ModalTitleStyle.Render(fmt.Sprintf("Manage Groups for '%s'", repo.Name))

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "Press [Space] to toggle, [+] to add a new group, [Esc] to exit.\n")

	groupNames := m.Config.GetAllGroupNames()
	if len(groupNames) == 0 {
		lines = append(lines, "  No groups created yet. Press [+] to create one!\n")
	}

	for i, g := range groupNames {
		checked := "[ ]"
		if m.SelectedRepoGroups[g] {
			checked = "[x]"
		}

		cursor := "  "
		if i == m.ModalCursor {
			cursor = "▶ "
		}

		line := fmt.Sprintf("%s%s %s", cursor, checked, g)
		if i == m.ModalCursor {
			lines = append(lines, SelectedRowStyle.Render(line))
		} else {
			lines = append(lines, NormalRowStyle.Render(line))
		}
	}

	if m.ModalInput.Focused() {
		lines = append(lines, "\nNew Group Name: "+m.ModalInput.View())
		lines = append(lines, MetaBarStyle.Render("(Press Enter to save group)"))
	}

	content := strings.Join(lines, "\n")
	return ModalBoxStyle.Render(content)
}

func (m Model) handleGroupsModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.Repos) == 0 || m.RepoCursor >= len(m.Repos) {
		m.ActiveModal = ModalNone
		return m, nil
	}
	repo := m.Repos[m.RepoCursor]

	if m.ModalInput.Focused() {
		switch msg.String() {
		case "enter":
			newGroupName := strings.TrimSpace(m.ModalInput.Value())
			if newGroupName != "" {
				_, _ = m.Config.ToggleRepoInGroup(repo.Name, newGroupName)
				m.SelectedRepoGroups[newGroupName] = true
				m.ModalInput.Reset()
				m.ModalInput.Blur()
				return m, func() tea.Msg { return StatusToastMsg(fmt.Sprintf("Added %s to %s", repo.Name, newGroupName)) }
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

	groupNames := m.Config.GetAllGroupNames()

	switch msg.String() {
	case "esc":
		m.ActiveModal = ModalNone
		return m, nil
	case "up", "k":
		if m.ModalCursor > 0 {
			m.ModalCursor--
		}
	case "down", "j":
		if m.ModalCursor < len(groupNames)-1 {
			m.ModalCursor++
		}
	case " ":
		// Toggle group membership
		if len(groupNames) > 0 && m.ModalCursor < len(groupNames) {
			groupName := groupNames[m.ModalCursor]
			added, err := m.Config.ToggleRepoInGroup(repo.Name, groupName)
			if err == nil {
				m.SelectedRepoGroups[groupName] = added
				action := "Removed from"
				if added {
					action = "Added to"
				}
				return m, func() tea.Msg {
					return StatusToastMsg(fmt.Sprintf("%s group '%s'", action, groupName))
				}
			}
		}
	case "+", "n":
		m.ModalInput.Placeholder = "e.g. backend, payments"
		m.ModalInput.Focus()
		return m, nil
	}

	return m, nil
}
