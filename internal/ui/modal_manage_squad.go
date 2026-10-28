package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	SquadModeOverview = iota
	SquadModeAdd
	SquadModeRemove
	SquadModeRename
	SquadModeDeleteConfirm
)

func (m Model) getManagedSquad() (string, []string, bool) {
	name := m.ManagedSquadName
	available := m.getAvailableSquads()
	if name == "" {
		if m.Config != nil {
			if active := m.Config.GetActiveSquad(); active != nil {
				name = active.Name
			}
		}
		if name == "" && len(available) > 0 {
			name = available[0].Name
		}
	}
	if name == "" {
		return "", nil, false
	}
	for _, s := range available {
		if strings.EqualFold(s.Name, name) {
			isActive := false
			if m.Config != nil {
				isActive = strings.EqualFold(m.Config.ActiveSquad, s.Name)
			}
			return s.Name, s.Members, isActive
		}
	}
	return name, nil, false
}

func (m Model) renderManageSquadModal() string {
	squadName, members, isActive := m.getManagedSquad()
	if squadName == "" {
		title := ModalTitleStyle.Render("Manage Squad")
		return ModalBoxStyle.Render(fmt.Sprintf("%s\n\nNo squads defined yet. Press [n] in Squad view to create one.\n\n[Esc] Exit", title))
	}

	activeBadge := ""
	if isActive {
		activeBadge = lipgloss.NewStyle().Foreground(SuccessColor).Bold(true).Render(" (Active)")
	}

	switch m.SquadManageMode {
	case SquadModeAdd:
		title := ModalTitleStyle.Render(fmt.Sprintf("Add Members to Squad: %s", squadName))
		var lines []string
		lines = append(lines, title)
		lines = append(lines, fmt.Sprintf("Search members from '%s' or type a username. [Space] to select, [Enter] to add selected, [Esc] Back.\n", m.CurrentOrg))
		lines = append(lines, "Search / Username: "+m.ModalInput.View()+"\n")

		filtered := m.getFilteredOrgMembers()
		existingMap := make(map[string]bool)
		for _, mem := range members {
			existingMap[strings.ToLower(mem)] = true
		}

		if len(filtered) == 0 {
			typed := strings.TrimSpace(m.ModalInput.Value())
			if typed != "" {
				lines = append(lines, lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(fmt.Sprintf("  ▶ Add '%s' directly (Press [Enter])", typed)))
			} else {
				lines = append(lines, "  No organization members match your search.")
			}
		} else {
			visibleLimit := 7
			if m.ModalCursor >= len(filtered) {
				m.ModalCursor = len(filtered) - 1
			}
			if m.ModalCursor < 0 {
				m.ModalCursor = 0
			}

			start := 0
			if m.ModalCursor >= visibleLimit {
				start = m.ModalCursor - visibleLimit + 1
			}
			end := start + visibleLimit
			if end > len(filtered) {
				end = len(filtered)
			}

			for i := start; i < end; i++ {
				om := filtered[i]
				cursor := "  "
				if i == m.ModalCursor {
					cursor = "▶ "
				}

				loginLower := strings.ToLower(om.Login)
				tag := "[ ]"
				suffix := ""
				if existingMap[loginLower] {
					tag = "[✓]"
					suffix = lipgloss.NewStyle().Foreground(MutedColor).Render(" (already in squad)")
				} else if m.SelectedSquadMembers[loginLower] {
					tag = "[x]"
					suffix = lipgloss.NewStyle().Foreground(SuccessColor).Bold(true).Render(" (will add)")
				}

				disp := om.Login
				if om.Name != "" && om.Name != om.Login {
					disp = fmt.Sprintf("%-16s (%s)", om.Login, om.Name)
				}
				line := fmt.Sprintf("%s%s %s%s", cursor, tag, disp, suffix)
				if i == m.ModalCursor {
					lines = append(lines, SelectedRowStyle.Render(line))
				} else {
					lines = append(lines, NormalRowStyle.Render(line))
				}
			}

			if len(filtered) > visibleLimit {
				lines = append(lines, MetaBarStyle.Render(fmt.Sprintf("  (Showing %d-%d of %d)", start+1, end, len(filtered))))
			}
		}

		selectedCount := len(m.SelectedSquadMembers)
		statusBar := fmt.Sprintf("Selected: %d new members to add | [Space] Toggle [Enter] Confirm [Esc] Back", selectedCount)
		lines = append(lines, "\n"+MetaBarStyle.Render(statusBar))
		return ModalBoxStyle.Render(strings.Join(lines, "\n"))

	case SquadModeRemove:
		title := ModalTitleStyle.Render(fmt.Sprintf("Remove Members from Squad: %s", squadName))
		var lines []string
		lines = append(lines, title)
		lines = append(lines, "Mark members to remove with [Space], then press [Enter] to delete. [Esc] Back.\n")

		if len(members) == 0 {
			lines = append(lines, "  No members currently in this squad.")
		} else {
			if m.ModalCursor >= len(members) {
				m.ModalCursor = len(members) - 1
			}
			if m.ModalCursor < 0 {
				m.ModalCursor = 0
			}

			for i, mem := range members {
				cursor := "  "
				if i == m.ModalCursor {
					cursor = "▶ "
				}
				memLower := strings.ToLower(mem)
				check := "[ ]"
				suffix := ""
				if m.SelectedSquadMembers[memLower] {
					check = "[x]"
					suffix = lipgloss.NewStyle().Foreground(DangerColor).Bold(true).Render(" (will remove)")
				}
				line := fmt.Sprintf("%s%s %s%s", cursor, check, mem, suffix)
				if i == m.ModalCursor {
					lines = append(lines, SelectedRowStyle.Render(line))
				} else {
					lines = append(lines, NormalRowStyle.Render(line))
				}
			}
		}

		toRemoveCount := len(m.SelectedSquadMembers)
		lines = append(lines, "\n"+MetaBarStyle.Render(fmt.Sprintf("Marked to remove: %d | [Space] Toggle [Enter] Confirm Deletion [Esc] Back", toRemoveCount)))
		return ModalBoxStyle.Render(strings.Join(lines, "\n"))

	case SquadModeRename:
		title := ModalTitleStyle.Render(fmt.Sprintf("Rename Squad: %s", squadName))
		var lines []string
		lines = append(lines, title)
		lines = append(lines, "Enter new name for this squad:\n")
		lines = append(lines, "Name: "+m.ModalInput.View()+"\n")
		lines = append(lines, MetaBarStyle.Render("[Enter] Save New Name  [Esc] Cancel"))
		return ModalBoxStyle.Render(strings.Join(lines, "\n"))

	case SquadModeDeleteConfirm:
		title := lipgloss.NewStyle().Bold(true).Foreground(DangerColor).Render("Delete Squad Confirmation")
		var lines []string
		lines = append(lines, title)
		lines = append(lines, fmt.Sprintf("\n⚠️  Are you sure you want to permanently delete squad '%s'?", squadName))
		lines = append(lines, fmt.Sprintf("    This squad currently has %d members.", len(members)))
		lines = append(lines, "\n"+lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render("Press [y] to permanently delete, or [n / Esc] to cancel."))
		return ModalBoxStyle.Render(strings.Join(lines, "\n"))

	default: // SquadModeOverview
		title := ModalTitleStyle.Render(fmt.Sprintf("Manage Squad: %s%s", squadName, activeBadge))
		var lines []string
		lines = append(lines, title)
		lines = append(lines, fmt.Sprintf("Total Members: %d\n", len(members)))

		// Display current members
		if len(members) == 0 {
			lines = append(lines, lipgloss.NewStyle().Foreground(MutedColor).Render("  (No members configured yet. Press [a] to add members.)"))
		} else {
			memberTags := make([]string, len(members))
			for i, mem := range members {
				memberTags[i] = lipgloss.NewStyle().Foreground(SecondaryColor).Render("@" + mem)
			}
			lines = append(lines, "  Members: "+strings.Join(memberTags, ", "))
		}
		lines = append(lines, "")

		// Action Menu
		actions := []string{
			"  [a] Add Members from Organization",
			"  [d] Delete / Remove Members",
			"  [r] Rename Squad",
			"  [x] Delete Entire Squad",
		}
		if !isActive {
			actions = append(actions, "  [*] Set as Active Squad")
		}

		for _, act := range actions {
			lines = append(lines, NormalRowStyle.Render(act))
		}

		lines = append(lines, "\n"+MetaBarStyle.Render("[Esc] Done / Close"))
		return ModalBoxStyle.Render(strings.Join(lines, "\n"))
	}
}

func (m Model) handleManageSquadModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	squadName, members, isActive := m.getManagedSquad()
	if squadName == "" {
		m.ActiveModal = ModalNone
		return m, nil
	}

	switch m.SquadManageMode {
	case SquadModeAdd:
		filtered := m.getFilteredOrgMembers()
		switch msg.String() {
		case "esc":
			m.SquadManageMode = SquadModeOverview
			m.ModalInput.Reset()
			m.ModalInput.Blur()
			m.SelectedSquadMembers = make(map[string]bool)
			return m, nil
		case "up", "k":
			if m.ModalCursor > 0 {
				m.ModalCursor--
			}
			return m, nil
		case "down", "j":
			if m.ModalCursor < len(filtered)-1 {
				m.ModalCursor++
			}
			return m, nil
		case " ":
			if len(filtered) > 0 && m.ModalCursor < len(filtered) {
				login := strings.ToLower(filtered[m.ModalCursor].Login)
				if m.SelectedSquadMembers[login] {
					delete(m.SelectedSquadMembers, login)
				} else {
					m.SelectedSquadMembers[login] = true
				}
			}
			return m, nil
		case "enter":
			// If user typed a name directly in input
			typed := strings.TrimSpace(m.ModalInput.Value())
			if typed != "" {
				m.SelectedSquadMembers[strings.ToLower(typed)] = true
			}

			// Add all selected members
			addedCount := 0
			for login := range m.SelectedSquadMembers {
				_ = m.Config.AddMemberToSquad(squadName, login)
				addedCount++
			}

			m.SquadManageMode = SquadModeOverview
			m.ModalInput.Reset()
			m.ModalInput.Blur()
			m.SelectedSquadMembers = make(map[string]bool)

			toastMsg := fmt.Sprintf("Added %d member(s) to squad '%s'", addedCount, squadName)
			return m, tea.Batch(
				func() tea.Msg { return StatusToastMsg(toastMsg) },
				m.reloadSquadActivityCmd(),
				m.syncSquadsFileCmd(),
			)
		default:
			prevVal := m.ModalInput.Value()
			var cmd tea.Cmd
			m.ModalInput, cmd = m.ModalInput.Update(msg)
			if m.ModalInput.Value() != prevVal {
				m.ModalCursor = 0
			}
			return m, cmd
		}

	case SquadModeRemove:
		switch msg.String() {
		case "esc":
			m.SquadManageMode = SquadModeOverview
			m.SelectedSquadMembers = make(map[string]bool)
			return m, nil
		case "up", "k":
			if m.ModalCursor > 0 {
				m.ModalCursor--
			}
			return m, nil
		case "down", "j":
			if m.ModalCursor < len(members)-1 {
				m.ModalCursor++
			}
			return m, nil
		case " ":
			if len(members) > 0 && m.ModalCursor < len(members) {
				mem := strings.ToLower(members[m.ModalCursor])
				if m.SelectedSquadMembers[mem] {
					delete(m.SelectedSquadMembers, mem)
				} else {
					m.SelectedSquadMembers[mem] = true
				}
			}
			return m, nil
		case "enter", "d":
			removedCount := 0
			for mem := range m.SelectedSquadMembers {
				_ = m.Config.RemoveMemberFromSquad(squadName, mem)
				removedCount++
			}

			m.SquadManageMode = SquadModeOverview
			m.SelectedSquadMembers = make(map[string]bool)

			toastMsg := fmt.Sprintf("Removed %d member(s) from squad '%s'", removedCount, squadName)
			return m, tea.Batch(
				func() tea.Msg { return StatusToastMsg(toastMsg) },
				m.reloadSquadActivityCmd(),
				m.syncSquadsFileCmd(),
			)
		}

	case SquadModeRename:
		switch msg.String() {
		case "esc":
			m.SquadManageMode = SquadModeOverview
			m.ModalInput.Reset()
			m.ModalInput.Blur()
			return m, nil
		case "enter":
			newName := strings.TrimSpace(m.ModalInput.Value())
			if newName != "" && !strings.EqualFold(newName, squadName) {
				if err := m.Config.RenameSquad(squadName, newName); err == nil {
					m.ManagedSquadName = newName
					m.SquadManageMode = SquadModeOverview
					m.ModalInput.Reset()
					m.ModalInput.Blur()
					return m, tea.Batch(
						func() tea.Msg {
							return StatusToastMsg(fmt.Sprintf("Renamed squad to '%s'", newName))
						},
						m.syncSquadsFileCmd(),
					)
				}
			}
			m.SquadManageMode = SquadModeOverview
			m.ModalInput.Reset()
			m.ModalInput.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.ModalInput, cmd = m.ModalInput.Update(msg)
			return m, cmd
		}

	case SquadModeDeleteConfirm:
		switch strings.ToLower(msg.String()) {
		case "y":
			_ = m.Config.DeleteSquad(squadName)
			m.ActiveModal = ModalNone
			m.SquadManageMode = SquadModeOverview
			m.ManagedSquadName = ""
			return m, tea.Batch(
				func() tea.Msg {
					return StatusToastMsg(fmt.Sprintf("Deleted squad '%s'", squadName))
				},
				m.syncSquadsFileCmd(),
			)
		case "n", "esc":
			m.SquadManageMode = SquadModeOverview
			return m, nil
		}

	default: // SquadModeOverview
		switch msg.String() {
		case "esc", "q":
			m.ActiveModal = ModalNone
			m.SquadManageMode = SquadModeOverview
			return m, nil
		case "a":
			m.SquadManageMode = SquadModeAdd
			m.ModalCursor = 0
			m.ModalInput.Reset()
			m.ModalInput.Placeholder = "Search or type username..."
			m.ModalInput.Focus()
			m.SelectedSquadMembers = make(map[string]bool)
			return m, textinput.Blink
		case "d":
			m.SquadManageMode = SquadModeRemove
			m.ModalCursor = 0
			m.SelectedSquadMembers = make(map[string]bool)
			return m, nil
		case "r":
			m.SquadManageMode = SquadModeRename
			m.ModalInput.Reset()
			m.ModalInput.SetValue(squadName)
			m.ModalInput.Focus()
			return m, textinput.Blink
		case "x":
			m.SquadManageMode = SquadModeDeleteConfirm
			return m, nil
		case "*":
			if !isActive {
				_ = m.Config.SetActiveSquad(squadName)
				return m, func() tea.Msg {
					return StatusToastMsg(fmt.Sprintf("'%s' is now the active squad", squadName))
				}
			}
		}
	}
	return m, nil
}

func (m Model) reloadSquadActivityCmd() tea.Cmd {
	allCommits := m.AllCommitEvents
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		squad := m.Config.GetActiveSquad()
		var members []string
		if squad != nil {
			members = squad.Members
		}
		events, err := m.GHClient.FetchSquadActivity(ctx, members, allCommits)
		return SquadActivityLoadedMsg{
			Events: events,
			Err:    err,
		}
	}
}
