package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderAssignSquadTaskModal() string {
	task := m.AssignTaskTarget
	if task == nil {
		return ""
	}

	title := ModalTitleStyle.Render(fmt.Sprintf("Assign Squad Task #%d", task.Number))

	taskTitleDisplay := task.Title
	if len(taskTitleDisplay) > 55 {
		taskTitleDisplay = taskTitleDisplay[:52] + "…"
	}
	taskInfo := lipgloss.NewStyle().Foreground(HighlightColor).Bold(true).Render(taskTitleDisplay)
	repoInfo := lipgloss.NewStyle().Foreground(SecondaryColor).Render(fmt.Sprintf("(%s)", task.RepoName))
	summaryLine := fmt.Sprintf("  %s  %s", taskInfo, repoInfo)

	// Members List
	assignHeader := "Assign to:"
	if m.AssignTaskFocusedField == 0 {
		assignHeader = lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render("▶ Assign to:") +
			lipgloss.NewStyle().Foreground(MutedColor).Render("  [Space/Enter] Toggle  [a] All  [m] Me  [c] Clear")
	} else {
		assignHeader = "  " + lipgloss.NewStyle().Bold(true).Render("Assign to:")
	}

	candidates := m.AssignTaskCandidates
	if len(candidates) == 0 {
		candidates = m.getSquadAssigneeOptions()
	}

	var memberLines []string
	if len(candidates) == 0 {
		memberLines = append(memberLines, "    (No squad members found)")
	} else {
		maxVisible := 5
		cursor := m.AssignTaskMemberCursor
		if cursor >= len(candidates) {
			cursor = len(candidates) - 1
		}
		if cursor < 0 {
			cursor = 0
		}

		start := 0
		if cursor >= maxVisible {
			start = cursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(candidates) {
			end = len(candidates)
		}

		for i := start; i < end; i++ {
			c := candidates[i]
			isCurrent := (m.AssignTaskFocusedField == 0 && i == cursor)
			rowPrefix := "    "
			if isCurrent {
				rowPrefix = "  ▶ "
			}

			checked := m.AssignTaskSelectedMembers != nil && m.AssignTaskSelectedMembers[c]
			box := "[ ]"
			if checked {
				box = lipgloss.NewStyle().Foreground(SuccessColor).Bold(true).Render("[x]")
			} else {
				box = lipgloss.NewStyle().Foreground(MutedColor).Render("[ ]")
			}

			label := "@" + c
			if strings.EqualFold(c, m.AuthUser) {
				label += " (me)"
			}

			rowText := fmt.Sprintf("%s%s %s", rowPrefix, box, label)
			if isCurrent {
				memberLines = append(memberLines, SelectedRowStyle.Render(rowText))
			} else {
				memberLines = append(memberLines, NormalRowStyle.Render(rowText))
			}
		}

		if len(candidates) > maxVisible {
			scrollHint := lipgloss.NewStyle().Foreground(MutedColor).Render(
				fmt.Sprintf("    (Showing %d-%d of %d members - use ↑/↓ to scroll)", start+1, end, len(candidates)),
			)
			memberLines = append(memberLines, scrollHint)
		}
	}

	// Selection Summary Line
	var selectedList []string
	for _, c := range candidates {
		if m.AssignTaskSelectedMembers != nil && m.AssignTaskSelectedMembers[c] {
			selectedList = append(selectedList, "@"+c)
		}
	}
	var summaryText string
	if len(selectedList) == 0 {
		summaryText = lipgloss.NewStyle().Foreground(WarningColor).Render("    Selected: (Unassigned squad task)")
	} else if len(selectedList) <= 3 {
		summaryText = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render("    Selected: " + strings.Join(selectedList, ", "))
	} else {
		summaryText = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(
			fmt.Sprintf("    Selected: %s (+%d more)", strings.Join(selectedList[:3], ", "), len(selectedList)-3),
		)
	}

	// Buttons
	var btnSave, btnCancel string
	if m.AssignTaskFocusedField == 1 {
		if m.AssignTaskActiveButton == 0 {
			btnSave = lipgloss.NewStyle().Bold(true).Background(PrimaryColor).Foreground(lipgloss.Color("#FFFFFF")).Render(" [ Save Assignees ] ")
			btnCancel = lipgloss.NewStyle().Foreground(MutedColor).Render(" [ Cancel ] ")
		} else {
			btnSave = lipgloss.NewStyle().Foreground(MutedColor).Render(" [ Save Assignees ] ")
			btnCancel = lipgloss.NewStyle().Bold(true).Background(DangerColor).Foreground(lipgloss.Color("#FFFFFF")).Render(" [ Cancel ] ")
		}
	} else {
		btnSave = lipgloss.NewStyle().Foreground(PrimaryColor).Render(" [ Save Assignees ] ")
		btnCancel = lipgloss.NewStyle().Foreground(MutedColor).Render(" [ Cancel ] ")
	}
	buttonLine := fmt.Sprintf("               %s   %s", btnSave, btnCancel)

	helpLine := MetaBarStyle.Render("  [Tab/Shift+Tab] Move to Buttons  •  [Enter on Save] Confirm  •  [Esc] Cancel")

	var contentLines []string
	contentLines = append(contentLines, title, "", summaryLine, "", assignHeader)
	contentLines = append(contentLines, memberLines...)
	contentLines = append(contentLines, summaryText, "", buttonLine, "", helpLine)

	return ModalBoxStyle.Render(strings.Join(contentLines, "\n"))
}

func (m Model) handleAssignSquadTaskModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	candidates := m.AssignTaskCandidates
	if len(candidates) == 0 {
		candidates = m.getSquadAssigneeOptions()
		m.AssignTaskCandidates = candidates
	}
	if m.AssignTaskSelectedMembers == nil {
		m.AssignTaskSelectedMembers = make(map[string]bool)
	}

	switch msg.String() {
	case "esc":
		m.ActiveModal = ModalNone
		return m, nil

	case "tab", "shift+tab":
		m.AssignTaskFocusedField = (m.AssignTaskFocusedField + 1) % 2
		return m, nil

	case "up", "k":
		if m.AssignTaskFocusedField == 0 {
			if m.AssignTaskMemberCursor > 0 {
				m.AssignTaskMemberCursor--
			}
			return m, nil
		}
		m.AssignTaskFocusedField = 0
		return m, nil

	case "down", "j":
		if m.AssignTaskFocusedField == 0 {
			if m.AssignTaskMemberCursor < len(candidates)-1 {
				m.AssignTaskMemberCursor++
			}
			return m, nil
		}
		return m, nil

	case " ":
		if m.AssignTaskFocusedField == 0 {
			if len(candidates) > 0 && m.AssignTaskMemberCursor < len(candidates) {
				mem := candidates[m.AssignTaskMemberCursor]
				m.AssignTaskSelectedMembers[mem] = !m.AssignTaskSelectedMembers[mem]
			}
			return m, nil
		}
		if m.AssignTaskFocusedField == 1 {
			m.AssignTaskActiveButton = (m.AssignTaskActiveButton + 1) % 2
			return m, nil
		}

	case "a", "s":
		// Select all squad candidates
		for _, c := range candidates {
			m.AssignTaskSelectedMembers[c] = true
		}
		return m, nil

	case "m":
		// Select myself only
		m.AssignTaskSelectedMembers = make(map[string]bool)
		if m.AuthUser != "" {
			m.AssignTaskSelectedMembers[m.AuthUser] = true
		}
		return m, nil

	case "c":
		// Clear all selections
		m.AssignTaskSelectedMembers = make(map[string]bool)
		return m, nil

	case "left", "h":
		if m.AssignTaskFocusedField == 1 {
			m.AssignTaskActiveButton = 0
			return m, nil
		}

	case "right", "l":
		if m.AssignTaskFocusedField == 1 {
			m.AssignTaskActiveButton = 1
			return m, nil
		}

	case "enter":
		if m.AssignTaskFocusedField == 0 {
			if len(candidates) > 0 && m.AssignTaskMemberCursor < len(candidates) {
				mem := candidates[m.AssignTaskMemberCursor]
				m.AssignTaskSelectedMembers[mem] = !m.AssignTaskSelectedMembers[mem]
			}
			return m, nil
		}

		if m.AssignTaskFocusedField == 1 {
			if m.AssignTaskActiveButton == 1 {
				m.ActiveModal = ModalNone
				return m, nil
			}

			if m.AssignTaskTarget != nil {
				var newAssignees []string
				for _, cand := range candidates {
					if m.AssignTaskSelectedMembers[cand] {
						newAssignees = append(newAssignees, cand)
					}
				}

				task := *m.AssignTaskTarget
				task.Assignees = newAssignees
				m = m.updateTaskInSlices(task)
				m.ActiveModal = ModalNone
				m.StatusMsg = fmt.Sprintf("Updating assignees for task #%d...", task.Number)
				m.StatusMsgTimer = time.Now().Add(4 * time.Second)

				return m, m.updateTaskAssigneesCmd(task, newAssignees)
			}
			m.ActiveModal = ModalNone
			return m, nil
		}
	}

	return m, nil
}
