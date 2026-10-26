package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"grubby/internal/github"
	"grubby/internal/storage"
)

func (m Model) openEditTaskModal(task github.Task) (tea.Model, tea.Cmd) {
	m.ActiveModal = ModalEditTask
	m.TaskEditTarget = task

	ti := textinput.New()
	ti.SetValue(task.Title)
	ti.Prompt = ""
	ti.Focus()
	m.TaskEditTitleInput = ti

	ta := textarea.New()
	ta.SetValue(task.Body)
	ta.Placeholder = "Task notes/details..."
	ta.ShowLineNumbers = false
	ta.SetHeight(5)
	ta.SetWidth(60)
	m.TaskEditBodyInput = ta

	m.TaskEditPriority = task.Priority
	if len(task.Assignees) > 0 {
		m.TaskEditAssignee = task.Assignees[0]
	} else {
		m.TaskEditAssignee = ""
	}
	m.TaskEditFocusedField = 0 // 0: Title, 1: Notes, 2: Priority, 3: Assignee (if squad), 4: Save, 5: Cancel

	return m, textinput.Blink
}

func (m Model) renderEditTaskModal() string {
	task := m.TaskEditTarget
	title := ModalTitleStyle.Render(fmt.Sprintf("Task #%d: Details & Edit", task.Number))

	// Meta info row
	var statusBadge string
	if task.State == "closed" {
		statusBadge = lipgloss.NewStyle().Bold(true).Foreground(SuccessColor).Render("[✓ Done / Completed]")
	} else {
		statusBadge = lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render("[● Active / Open]")
	}

	repoFullName := fmt.Sprintf("%s/%s", task.RepoOwner, task.RepoName)
	authorText := lipgloss.NewStyle().Foreground(lipgloss.Color("#7F8497")).Render("@" + task.Author)
	createdText := lipgloss.NewStyle().Foreground(lipgloss.Color("#7F8497")).Render(formatTimeAgo(task.CreatedAt))

	metaRow := fmt.Sprintf("  Status: %s  •  Repo: %s  •  Author: %s  •  Created: %s",
		statusBadge, repoFullName, authorText, createdText)

	// Field 0: Title
	titleLine := "Title:         " + m.TaskEditTitleInput.View()
	if m.TaskEditFocusedField == 0 {
		titleLine = lipgloss.NewStyle().Bold(true).Render("▶ ") + titleLine
	} else {
		titleLine = "  " + titleLine
	}

	// Field 1: Notes (multiline textarea)
	notesHeader := "Notes (multiline):"
	if m.TaskEditFocusedField == 1 {
		notesHeader = lipgloss.NewStyle().Bold(true).Render("▶ " + notesHeader)
	} else {
		notesHeader = "  " + notesHeader
	}
	notesBox := lipgloss.NewStyle().MarginLeft(4).Render(m.TaskEditBodyInput.View())

	// Field 2: Priority Selector
	var prioLow, prioMed, prioHigh string
	switch m.TaskEditPriority {
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
		prioMed = lipgloss.NewStyle().Bold(true).Foreground(WarningColor).Render("[● Medium]")
		prioLow = lipgloss.NewStyle().Foreground(MutedColor).Render("[○ Low]")
	}
	prioLine := fmt.Sprintf("Priority:      %s   %s   %s", prioHigh, prioMed, prioLow)
	if m.TaskEditFocusedField == 2 {
		prioLine = lipgloss.NewStyle().Bold(true).Render("▶ " + prioLine)
	} else {
		prioLine = "  " + prioLine
	}

	// Field 3: Assignee Selector
	var assigneeDisplay string
	if !task.IsSquadTask {
		assigneeDisplay = lipgloss.NewStyle().Foreground(SecondaryColor).Render(fmt.Sprintf("@%s (Personal Task)", m.AuthUser))
	} else {
		if m.TaskEditAssignee == "" {
			assigneeDisplay = lipgloss.NewStyle().Foreground(MutedColor).Render("Unassigned • [Space/←/→] Change")
		} else {
			assigneeDisplay = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(fmt.Sprintf("@%s • [Space/←/→] Change", m.TaskEditAssignee))
		}
	}
	assigneeLine := "Assignee:      " + assigneeDisplay
	if m.TaskEditFocusedField == 3 {
		assigneeLine = lipgloss.NewStyle().Bold(true).Render("▶ ") + assigneeLine
	} else {
		assigneeLine = "  " + assigneeLine
	}

	// Action buttons
	var saveBtn, cancelBtn string
	if m.TaskEditFocusedField == 4 {
		saveBtn = lipgloss.NewStyle().Bold(true).Background(PrimaryColor).Foreground(lipgloss.Color("#FFFFFF")).Render(" [ Save Changes ] ")
	} else {
		saveBtn = lipgloss.NewStyle().Background(DarkBgColor).Foreground(lipgloss.Color("#CCCCCC")).Render(" [ Save Changes ] ")
	}
	if m.TaskEditFocusedField == 5 {
		cancelBtn = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("#444455")).Foreground(lipgloss.Color("#FFFFFF")).Render(" [ Cancel ] ")
	} else {
		cancelBtn = lipgloss.NewStyle().Foreground(MutedColor).Render(" [ Cancel ] ")
	}
	buttonsLine := fmt.Sprintf("  %s   %s", saveBtn, cancelBtn)

	// Help line
	helpLine := MetaBarStyle.Render("  [Tab] Move  •  [Enter on Notes] Newline  •  [Ctrl+S] Save  •  [o] Web  •  [Esc] Cancel")

	content := strings.Join([]string{
		title,
		"",
		metaRow,
		"",
		titleLine,
		notesHeader,
		notesBox,
		prioLine,
		assigneeLine,
		"",
		buttonsLine,
		"",
		helpLine,
	}, "\n")

	return ModalBoxStyle.Render(content)
}

func (m Model) handleEditTaskModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	task := m.TaskEditTarget

	switch msg.String() {
	case "esc":
		m.ActiveModal = ModalNone
		return m, nil

	case "ctrl+s":
		return m.submitEditTaskModal()

	case "o":
		// Only open in browser if not typing in title or notes
		if m.TaskEditFocusedField != 0 && m.TaskEditFocusedField != 1 {
			if task.URL != "" {
				_ = OpenBrowser(task.URL)
				return m, func() tea.Msg {
					return StatusToastMsg(fmt.Sprintf("Opened task #%d in browser", task.Number))
				}
			}
		}

	case "tab":
		m.TaskEditFocusedField = (m.TaskEditFocusedField + 1) % 6
		m = m.syncEditTaskInputsFocus()
		return m, nil

	case "shift+tab":
		m.TaskEditFocusedField = (m.TaskEditFocusedField + 5) % 6
		m = m.syncEditTaskInputsFocus()
		return m, nil

	case "down":
		if m.TaskEditFocusedField != 1 {
			m.TaskEditFocusedField = (m.TaskEditFocusedField + 1) % 6
			m = m.syncEditTaskInputsFocus()
			return m, nil
		}

	case "up":
		if m.TaskEditFocusedField != 1 {
			m.TaskEditFocusedField = (m.TaskEditFocusedField + 5) % 6
			m = m.syncEditTaskInputsFocus()
			return m, nil
		}

	case "left", "right", "h", "l", " ":
		// If in title or notes, don't intercept typing
		if m.TaskEditFocusedField == 0 || m.TaskEditFocusedField == 1 {
			break
		}

		if m.TaskEditFocusedField == 2 {
			// Cycle Priority: Medium -> High -> Low -> Medium
			switch m.TaskEditPriority {
			case github.PriorityMedium:
				m.TaskEditPriority = github.PriorityHigh
			case github.PriorityHigh:
				m.TaskEditPriority = github.PriorityLow
			default:
				m.TaskEditPriority = github.PriorityMedium
			}
			return m, nil
		}

		if m.TaskEditFocusedField == 3 && task.IsSquadTask {
			// Cycle Assignees for Squad Task
			candidates := m.getSquadTaskAssigneeCandidates()
			currentIdx := 0
			for i, c := range candidates {
				if c == m.TaskEditAssignee {
					currentIdx = i
					break
				}
			}
			nextIdx := (currentIdx + 1) % len(candidates)
			m.TaskEditAssignee = candidates[nextIdx]
			return m, nil
		}

		if m.TaskEditFocusedField == 4 && msg.String() == " " {
			return m.submitEditTaskModal()
		}

		if m.TaskEditFocusedField == 5 && msg.String() == " " {
			m.ActiveModal = ModalNone
			return m, nil
		}

	case "enter":
		if m.TaskEditFocusedField == 0 {
			// Advance to notes
			m.TaskEditFocusedField = 1
			m = m.syncEditTaskInputsFocus()
			return m, nil
		}
		if m.TaskEditFocusedField == 4 || m.TaskEditFocusedField == 2 || m.TaskEditFocusedField == 3 {
			return m.submitEditTaskModal()
		}
		if m.TaskEditFocusedField == 5 {
			m.ActiveModal = ModalNone
			return m, nil
		}
	}

	// Forward key to active input if focused
	if m.TaskEditFocusedField == 0 {
		var cmd tea.Cmd
		m.TaskEditTitleInput, cmd = m.TaskEditTitleInput.Update(msg)
		return m, cmd
	}
	if m.TaskEditFocusedField == 1 {
		var cmd tea.Cmd
		m.TaskEditBodyInput, cmd = m.TaskEditBodyInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) syncEditTaskInputsFocus() Model {
	if m.TaskEditFocusedField == 0 {
		m.TaskEditTitleInput.Focus()
		m.TaskEditBodyInput.Blur()
	} else if m.TaskEditFocusedField == 1 {
		m.TaskEditTitleInput.Blur()
		m.TaskEditBodyInput.Focus()
	} else {
		m.TaskEditTitleInput.Blur()
		m.TaskEditBodyInput.Blur()
	}
	return m
}

func (m Model) submitEditTaskModal() (tea.Model, tea.Cmd) {
	titleText := strings.TrimSpace(m.TaskEditTitleInput.Value())
	if titleText == "" {
		return m, func() tea.Msg { return StatusToastMsg("Task title cannot be empty!") }
	}

	bodyText := strings.TrimSpace(m.TaskEditBodyInput.Value())
	priority := m.TaskEditPriority
	if priority == "" {
		priority = github.PriorityMedium
	}

	task := m.TaskEditTarget
	var assignees []string
	if !task.IsSquadTask {
		if m.AuthUser != "" {
			assignees = append(assignees, m.AuthUser)
		}
	} else if m.TaskEditAssignee != "" {
		assignees = append(assignees, m.TaskEditAssignee)
	}

	task.Title = titleText
	task.Body = bodyText
	task.Priority = priority
	task.Assignees = assignees
	task.UpdatedAt = time.Now()

	m.ActiveModal = ModalNone
	m = m.updateTaskInSlices(task)
	if m.DB != nil {
		_ = m.DB.SaveTask(&task, "pending_update")
	}
	m.StatusMsg = fmt.Sprintf("✓ Saved changes to task #%d", task.Number)
	m.StatusMsgTimer = time.Now().Add(3 * time.Second)

	if m.Debouncer != nil {
		m.Debouncer.Enqueue(storage.TaskDebounceJob{
			TaskID:    task.ID,
			Number:    task.Number,
			RepoOwner: task.RepoOwner,
			RepoName:  task.RepoName,
			Title:     task.Title,
			Body:      task.Body,
			Priority:  task.Priority,
			Assignees: task.Assignees,
			Labels:    task.Labels,
			IsSquad:   task.IsSquadTask,
			Action:    storage.ActionUpdateTask,
		}, func(ctx context.Context, job *storage.TaskDebounceJob) error {
			var err error
			if m.GHClient != nil {
				_, err = m.GHClient.UpdateTask(ctx, job.RepoOwner, job.RepoName, job.Number, job.Title, job.Body, job.Priority, job.Assignees, job.Labels, job.IsSquad)
			}
			if err == nil && m.DB != nil {
				_ = m.DB.UpdateTaskSyncStatus(job.TaskID, "synced")
			}
			return err
		})
		return m, nil
	}

	return m, m.updateTaskCmd(task, titleText, bodyText, priority, assignees)
}
