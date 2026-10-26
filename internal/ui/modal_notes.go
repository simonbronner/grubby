package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"grubby/internal/github"
)

func (m Model) openTaskNotesModal(target *github.Task, isNew bool) (tea.Model, tea.Cmd) {
	m.ActiveModal = ModalTaskNotes
	m.TaskNotesTarget = target
	m.TaskNotesIsNewTask = isNew

	ta := textarea.New()
	ta.Placeholder = "Task notes/details (Enter for newline, Esc to save)..."
	ta.ShowLineNumbers = false
	ta.SetHeight(8)
	ta.SetWidth(65)
	if isNew {
		ta.SetValue(m.InlineNewNotes)
	} else if target != nil {
		ta.SetValue(target.Body)
	}
	ta.Focus()
	m.TaskNotesInput = ta
	return m, m.TaskNotesInput.Focus()
}

func (m Model) renderTaskNotesModal() string {
	var titleStr string
	if m.TaskNotesIsNewTask {
		titleStr = "Task Notes (New Task)"
	} else if m.TaskNotesTarget != nil {
		titleStr = fmt.Sprintf("Task Notes: #%d %s", m.TaskNotesTarget.Number, m.TaskNotesTarget.Title)
		if len(titleStr) > 55 {
			titleStr = titleStr[:52] + "…"
		}
	} else {
		titleStr = "Task Notes"
	}
	title := ModalTitleStyle.Render(titleStr)

	helpLine := MetaBarStyle.Render("  [Enter] Newline  •  [Esc] Save Notes  •  [Ctrl+C] Discard")

	content := strings.Join([]string{
		title,
		"",
		m.TaskNotesInput.View(),
		"",
		helpLine,
	}, "\n")

	return ModalBoxStyle.Render(content)
}

func (m Model) handleTaskNotesModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.ActiveModal = ModalNone
		return m, nil

	case "esc":
		if m.TaskNotesIsNewTask {
			m.InlineNewNotes = m.TaskNotesInput.Value()
			m.ActiveModal = ModalNone
			m.StatusMsg = "✓ Saved notes for new task draft"
			m.StatusMsgTimer = time.Now().Add(3 * time.Second)
			return m, nil
		}
		if m.TaskNotesTarget != nil {
			target := *m.TaskNotesTarget
			newBody := m.TaskNotesInput.Value()
			m.ActiveModal = ModalNone
			m.StatusMsg = fmt.Sprintf("Saving notes for task #%d...", target.Number)
			return m, m.updateTaskCmd(target, target.Title, newBody, target.Priority, target.Assignees)
		}
		m.ActiveModal = ModalNone
		return m, nil
	}

	var cmd tea.Cmd
	m.TaskNotesInput, cmd = m.TaskNotesInput.Update(msg)
	return m, cmd
}
