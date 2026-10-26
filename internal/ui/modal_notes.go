package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"grubby/internal/github"
	"grubby/internal/storage"
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
			target.Body = newBody
			target.UpdatedAt = time.Now()

			m.ActiveModal = ModalNone
			m = m.updateTaskInSlices(target)
			if m.DB != nil {
				_ = m.DB.SaveTask(&target, "pending_update")
			}
			m.StatusMsg = fmt.Sprintf("✓ Saved notes for task #%d", target.Number)
			m.StatusMsgTimer = time.Now().Add(3 * time.Second)

			if m.Debouncer != nil {
				m.Debouncer.Enqueue(storage.TaskDebounceJob{
					TaskID:    target.ID,
					Number:    target.Number,
					RepoOwner: target.RepoOwner,
					RepoName:  target.RepoName,
					Title:     target.Title,
					Body:      target.Body,
					Priority:  target.Priority,
					Assignees: target.Assignees,
					Labels:    target.Labels,
					IsSquad:   target.IsSquadTask,
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

			return m, m.updateTaskCmd(target, target.Title, newBody, target.Priority, target.Assignees)
		}
		m.ActiveModal = ModalNone
		return m, nil
	}

	var cmd tea.Cmd
	m.TaskNotesInput, cmd = m.TaskNotesInput.Update(msg)
	return m, cmd
}
