package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"grubby/internal/github"
)

type TaskTarget int

const (
	TaskTargetPersonal TaskTarget = iota
	TaskTargetSquad
)

func (m Model) renderCreateTaskModal() string {
	title := ModalTitleStyle.Render("Create New Task")

	// Target Selector
	var targetPersonal, targetSquad string
	if m.TaskCreateTarget == TaskTargetPersonal {
		targetPersonal = lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render("[● Personal Tasks]")
		targetSquad = lipgloss.NewStyle().Foreground(MutedColor).Render("[○ Squad Tasks]")
	} else {
		targetPersonal = lipgloss.NewStyle().Foreground(MutedColor).Render("[○ Personal Tasks]")
		targetSquad = lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render("[● Squad Tasks]")
	}
	targetLine := fmt.Sprintf("Target Repo:   %s   %s", targetPersonal, targetSquad)
	if m.TaskCreateFocusedField == 0 {
		targetLine = lipgloss.NewStyle().Bold(true).Render("▶ " + targetLine)
	} else {
		targetLine = "  " + targetLine
	}

	// Title Input
	titleLine := "Title:         " + m.TaskCreateTitleInput.View()
	if m.TaskCreateFocusedField == 1 {
		titleLine = lipgloss.NewStyle().Bold(true).Render("▶ ") + titleLine
	} else {
		titleLine = "  " + titleLine
	}

	// Notes / Body Input (multiline)
	bodyHeader := "Notes (opt, multiline):"
	if m.TaskCreateFocusedField == 2 {
		bodyHeader = lipgloss.NewStyle().Bold(true).Render("▶ " + bodyHeader)
	} else {
		bodyHeader = "  " + bodyHeader
	}
	bodyBox := lipgloss.NewStyle().MarginLeft(4).Render(m.TaskCreateBodyInput.View())

	// Priority Selector: Low / Medium / High
	var prioLow, prioMed, prioHigh string
	switch m.TaskCreatePriority {
	case github.PriorityHigh:
		prioHigh = lipgloss.NewStyle().Bold(true).Foreground(DangerColor).Render("[● High]")
		prioMed = lipgloss.NewStyle().Foreground(MutedColor).Render("[○ Medium]")
		prioLow = lipgloss.NewStyle().Foreground(MutedColor).Render("[○ Low]")
	case github.PriorityLow:
		prioHigh = lipgloss.NewStyle().Foreground(MutedColor).Render("[○ High]")
		prioMed = lipgloss.NewStyle().Foreground(MutedColor).Render("[○ Medium]")
		prioLow = lipgloss.NewStyle().Bold(true).Foreground(SuccessColor).Render("[● Low]")
	default: // Medium
		prioHigh = lipgloss.NewStyle().Foreground(MutedColor).Render("[○ High]")
		prioMed = lipgloss.NewStyle().Bold(true).Foreground(WarningColor).Render("[● Medium (Default)]")
		prioLow = lipgloss.NewStyle().Foreground(MutedColor).Render("[○ Low]")
	}
	prioLine := fmt.Sprintf("Priority:      %s   %s   %s", prioHigh, prioMed, prioLow)
	if m.TaskCreateFocusedField == 3 {
		prioLine = lipgloss.NewStyle().Bold(true).Render("▶ " + prioLine)
	} else {
		prioLine = "  " + prioLine
	}

	// Assignee Selector
	var assigneeDisplay string
	if m.TaskCreateTarget == TaskTargetPersonal {
		assigneeDisplay = lipgloss.NewStyle().Foreground(SecondaryColor).Render(fmt.Sprintf("@%s (Personal: automatically assigned)", m.AuthUser))
	} else {
		if m.TaskCreateAssignee == "" {
			assigneeDisplay = lipgloss.NewStyle().Foreground(MutedColor).Render("Unassigned (Default) • [Space/h/l] Change")
		} else {
			assigneeDisplay = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(fmt.Sprintf("@%s • [Space/h/l] Change", m.TaskCreateAssignee))
		}
	}
	assigneeLine := "Assignee:      " + assigneeDisplay
	if m.TaskCreateFocusedField == 4 {
		assigneeLine = lipgloss.NewStyle().Bold(true).Render("▶ ") + assigneeLine
	} else {
		assigneeLine = "  " + assigneeLine
	}

	// Target Repo Info
	repoInfo := m.getTaskTargetRepoFullName(m.TaskCreateTarget)
	infoLine := lipgloss.NewStyle().Foreground(lipgloss.Color("#7F8497")).Render(fmt.Sprintf("  Backing Repo: %s", repoInfo))

	// Instructions
	helpLine := MetaBarStyle.Render("  [Tab] Move  •  [Enter on Notes] Newline  •  [Ctrl+S / Enter] Create  •  [Esc] Cancel")

	content := strings.Join([]string{
		title,
		"",
		targetLine,
		titleLine,
		bodyHeader,
		bodyBox,
		prioLine,
		assigneeLine,
		"",
		infoLine,
		"",
		helpLine,
	}, "\n")

	return ModalBoxStyle.Render(content)
}

func (m Model) getTaskTargetRepoFullName(target TaskTarget) string {
	if target == TaskTargetPersonal {
		user := m.AuthUser
		if user == "" {
			user = m.CurrentOrg
		}
		return fmt.Sprintf("%s/tasks", user)
	}

	squadName := "default-squad"
	if m.Config != nil && m.Config.ActiveSquad != "" {
		squadName = m.Config.ActiveSquad
	}
	org := m.CurrentOrg
	if org == "" {
		org = m.AuthUser
	}
	return fmt.Sprintf("%s/%s-tasks", org, squadName)
}

func (m Model) handleCreateTaskModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.ActiveModal = ModalNone
		return m, nil

	case "ctrl+s":
		return m.submitCreateTaskModal()

	case "tab":
		m.TaskCreateFocusedField = (m.TaskCreateFocusedField + 1) % 5
		m = m.syncCreateTaskInputsFocus()
		return m, nil

	case "shift+tab":
		m.TaskCreateFocusedField = (m.TaskCreateFocusedField + 4) % 5
		m = m.syncCreateTaskInputsFocus()
		return m, nil

	case "down":
		if m.TaskCreateFocusedField != 2 {
			m.TaskCreateFocusedField = (m.TaskCreateFocusedField + 1) % 5
			m = m.syncCreateTaskInputsFocus()
			return m, nil
		}

	case "up":
		if m.TaskCreateFocusedField != 2 {
			m.TaskCreateFocusedField = (m.TaskCreateFocusedField + 4) % 5
			m = m.syncCreateTaskInputsFocus()
			return m, nil
		}

	case "left", "right", "h", "l", " ":
		// If in title or body input, don't intercept space/arrows
		if m.TaskCreateFocusedField == 1 || m.TaskCreateFocusedField == 2 {
			break
		}

		if m.TaskCreateFocusedField == 0 {
			// Toggle Target
			if m.TaskCreateTarget == TaskTargetPersonal {
				m.TaskCreateTarget = TaskTargetSquad
				m.TaskCreateAssignee = "" // default unassigned for squad
			} else {
				m.TaskCreateTarget = TaskTargetPersonal
				m.TaskCreateAssignee = m.AuthUser // always assigned to user for personal
			}
			return m, nil
		}

		if m.TaskCreateFocusedField == 3 {
			// Cycle Priority: Medium -> High -> Low -> Medium
			switch m.TaskCreatePriority {
			case github.PriorityMedium:
				m.TaskCreatePriority = github.PriorityHigh
			case github.PriorityHigh:
				m.TaskCreatePriority = github.PriorityLow
			default:
				m.TaskCreatePriority = github.PriorityMedium
			}
			return m, nil
		}

		if m.TaskCreateFocusedField == 4 && m.TaskCreateTarget == TaskTargetSquad {
			// Cycle Assignees for Squad Task
			candidates := m.getSquadTaskAssigneeCandidates()
			currentIdx := 0
			for i, c := range candidates {
				if c == m.TaskCreateAssignee {
					currentIdx = i
					break
				}
			}
			nextIdx := (currentIdx + 1) % len(candidates)
			m.TaskCreateAssignee = candidates[nextIdx]
			return m, nil
		}

	case "enter":
		if m.TaskCreateFocusedField == 1 {
			// Advance to notes
			m.TaskCreateFocusedField = 2
			m = m.syncCreateTaskInputsFocus()
			return m, nil
		}
		if m.TaskCreateFocusedField != 2 {
			return m.submitCreateTaskModal()
		}
	}

	// Forward key to active input if focused
	if m.TaskCreateFocusedField == 1 {
		var cmd tea.Cmd
		m.TaskCreateTitleInput, cmd = m.TaskCreateTitleInput.Update(msg)
		return m, cmd
	}
	if m.TaskCreateFocusedField == 2 {
		var cmd tea.Cmd
		m.TaskCreateBodyInput, cmd = m.TaskCreateBodyInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) submitCreateTaskModal() (tea.Model, tea.Cmd) {
	titleText := strings.TrimSpace(m.TaskCreateTitleInput.Value())
	if titleText == "" {
		return m, func() tea.Msg { return StatusToastMsg("Task title cannot be empty!") }
	}

	bodyText := strings.TrimSpace(m.TaskCreateBodyInput.Value())
	priority := m.TaskCreatePriority
	if priority == "" {
		priority = github.PriorityMedium
	}

	target := m.TaskCreateTarget
	isSquad := (target == TaskTargetSquad)
	repoFullName := m.getTaskTargetRepoFullName(target)
	parts := strings.SplitN(repoFullName, "/", 2)
	owner := parts[0]
	repo := parts[1]

	var assignees []string
	if target == TaskTargetPersonal {
		if m.AuthUser != "" {
			assignees = append(assignees, m.AuthUser)
		}
	} else if m.TaskCreateAssignee != "" {
		assignees = append(assignees, m.TaskCreateAssignee)
	}

	m.ActiveModal = ModalNone
	m.StatusMsg = fmt.Sprintf("Creating task in %s...", repoFullName)
	return m, m.createTaskCmd(owner, repo, titleText, bodyText, priority, assignees, isSquad)
}

func (m Model) syncCreateTaskInputsFocus() Model {
	if m.TaskCreateFocusedField == 1 {
		m.TaskCreateTitleInput.Focus()
		m.TaskCreateBodyInput.Blur()
	} else if m.TaskCreateFocusedField == 2 {
		m.TaskCreateTitleInput.Blur()
		m.TaskCreateBodyInput.Focus()
	} else {
		m.TaskCreateTitleInput.Blur()
		m.TaskCreateBodyInput.Blur()
	}
	return m
}

func (m Model) getSquadTaskAssigneeCandidates() []string {
	var list []string
	list = append(list, "") // "" represents Unassigned
	list = append(list, m.getSquadAssigneeOptions()...)
	return list
}
