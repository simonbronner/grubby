package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"grubby/internal/github"
	"grubby/internal/storage"
)

type TaskSubtab int

const (
	TaskSubtabMine TaskSubtab = iota
	TaskSubtabSquad
)

func (m Model) isSquadTasks() bool {
	if m.ActiveTab == TabSquadTasks {
		return true
	}
	return m.ActiveTaskSubtab == TaskSubtabSquad
}

func (m Model) getActiveTasks() []github.Task {
	var list []github.Task
	if !m.isSquadTasks() {
		list = append(list, m.PersonalTasks...)
	} else {
		list = append(list, m.SquadTasks...)
	}

	sort.SliceStable(list, func(i, j int) bool {
		rankI := list[i].Priority.Rank()
		rankJ := list[j].Priority.Rank()
		if rankI != rankJ {
			return rankI < rankJ
		}
		if !list[i].UpdatedAt.Equal(list[j].UpdatedAt) {
			return list[i].UpdatedAt.After(list[j].UpdatedAt)
		}
		return list[i].Number > list[j].Number
	})

	return list
}

func (m Model) getDisplayTasks() []github.Task {
	return m.getActiveTasks()
}

func (m Model) getRecentDoneTasks() []github.Task {
	allDone := m.getSortedDoneTasks()
	if len(allDone) > 5 {
		return allDone[:5]
	}
	return allDone
}

type cursorLocationType int

const (
	locInlineNew cursorLocationType = iota
	locActiveTask
	locCompletedTask
)

func (m Model) getCursorTarget() (cursorLocationType, *github.Task) {
	activeTasks := m.getActiveTasks()
	recentDone := m.getRecentDoneTasks()

	cursor := m.TaskCursor
	if m.InlineAdding {
		if cursor == 0 {
			return locInlineNew, nil
		}
		cursor--
	}

	if cursor < len(activeTasks) {
		return locActiveTask, &activeTasks[cursor]
	}

	cursor -= len(activeTasks)
	if cursor < len(recentDone) {
		return locCompletedTask, &recentDone[cursor]
	}

	return locActiveTask, nil
}

func (m Model) getTotalTaskRows() int {
	total := len(m.getActiveTasks()) + len(m.getRecentDoneTasks())
	if m.InlineAdding {
		total++
	}
	return total
}

func (m Model) renderTasksView() string {
	activeTasks := m.getActiveTasks()
	recentDone := m.getRecentDoneTasks()

	// 1. Task Board Header Banner
	mineOpen := len(m.PersonalTasks)
	squadOpen := len(m.SquadTasks)
	allDoneCount := len(m.getSortedDoneTasks())

	var activeRepo string
	var boardName string
	if m.isSquadTasks() {
		activeRepo = m.getTaskTargetRepoFullName(TaskTargetSquad)
		squadName := ""
		if m.Config != nil && m.Config.ActiveSquad != "" {
			squadName = fmt.Sprintf(" (%s)", m.Config.ActiveSquad)
		}
		boardName = fmt.Sprintf("👥 Squad Tasks%s (%d)", squadName, squadOpen)
	} else {
		activeRepo = m.getTaskTargetRepoFullName(TaskTargetPersonal)
		boardName = fmt.Sprintf("📋 My Tasks (%d)", mineOpen)
	}

	boardBadge := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Render(boardName)

	var headerBanner string
	if m.InlineEditing {
		headerBanner = "  " + boardBadge + MetaBarStyle.Render(fmt.Sprintf("  •  Repo: %s  •  [Editing Title: Enter to save, Esc to cancel]", activeRepo))
	} else if m.InlineAdding {
		headerBanner = "  " + boardBadge + MetaBarStyle.Render(fmt.Sprintf("  •  Repo: %s  •  [New Task: Enter to create, Esc to cancel]", activeRepo))
	} else {
		allDoneBadge := lipgloss.NewStyle().Foreground(lipgloss.Color("#8888AA")).Render(fmt.Sprintf("[d] All Done (%d)", allDoneCount))
		if m.isSquadTasks() {
			headerBanner = "  " + boardBadge + MetaBarStyle.Render(fmt.Sprintf("  •  Repo: %s  •  %s  •  [a] Add Task  [A] Assign  [n] Notes  [Space] Done", activeRepo, allDoneBadge))
		} else {
			headerBanner = "  " + boardBadge + MetaBarStyle.Render(fmt.Sprintf("  •  Repo: %s  •  %s  •  [a] Add Task  [n] Notes  [Space] Done", activeRepo, allDoneBadge))
		}
	}

	// 2. Table Layout
	cursorW := 3
	prioW := 10
	notesW := 3
	assigneeW := 16
	updatedW := 12
	repoW := 14

	titleW := m.Width - (cursorW + prioW + notesW + assigneeW + updatedW + repoW + 6)
	if titleW < 24 {
		titleW = 24
	}

	cursorCol := lipgloss.NewStyle().Width(cursorW).MaxHeight(1).Inline(true)
	prioCol := lipgloss.NewStyle().Width(prioW).MaxHeight(1).Inline(true)
	titleCol := lipgloss.NewStyle().Width(titleW).MaxHeight(1).Inline(true)
	notesCol := lipgloss.NewStyle().Width(notesW).MaxHeight(1).Inline(true)
	assigneeCol := lipgloss.NewStyle().Width(assigneeW).MaxHeight(1).Inline(true)
	updatedCol := lipgloss.NewStyle().Width(updatedW).MaxHeight(1).Inline(true)
	repoCol := lipgloss.NewStyle().Width(repoW).MaxHeight(1).Inline(true)

	tableHead := lipgloss.JoinHorizontal(lipgloss.Top,
		cursorCol.Render(""),
		prioCol.Render("PRIORITY"),
		titleCol.Render("TASK TITLE"),
		notesCol.Render("📝"),
		assigneeCol.Render("ASSIGNEE"),
		updatedCol.Render("UPDATED"),
		repoCol.Render("ISSUE"),
	)
	renderedTableHead := TableHeaderStyle.Render(tableHead)

	var rows []string
	rows = append(rows, headerBanner, "", renderedTableHead)

	// Available lines for active tasks
	doneLines := 0
	if len(recentDone) > 0 {
		doneLines = len(recentDone) + 2 // 2 lines for separator and blank
	}
	visibleCount := m.Height - 10 - doneLines
	if visibleCount < 3 {
		visibleCount = 3
	}

	rowIdx := 0

	// Inline adding row
	if m.InlineAdding {
		prioBadge := lipgloss.NewStyle().Bold(true).Foreground(WarningColor).Render("[🟡 MED]")
		switch m.InlineNewPriority {
		case github.PriorityHigh:
			prioBadge = lipgloss.NewStyle().Bold(true).Foreground(DangerColor).Render("[🔴 HIGH]")
		case github.PriorityLow:
			prioBadge = lipgloss.NewStyle().Foreground(SuccessColor).Render("[🟢 LOW]")
		}

		notesIcon := " · "
		if len(strings.TrimSpace(m.InlineNewNotes)) > 0 {
			notesIcon = "📝"
		}

		assigneeDisplay := "Unassigned"
		if m.InlineNewAssignee != "" {
			assigneeDisplay = "@" + m.InlineNewAssignee
		}
		assigneeText := lipgloss.NewStyle().Foreground(SecondaryColor).Render(assigneeDisplay)
		dateText := lipgloss.NewStyle().Foreground(MutedColor).Render("Draft")
		issueText := lipgloss.NewStyle().Foreground(WarningColor).Render("(new)")

		inlineCells := lipgloss.JoinHorizontal(lipgloss.Top,
			cursorCol.Render("▶ "),
			prioCol.Render(prioBadge),
			titleCol.Render(m.InlineNewTitleInput.View()),
			notesCol.Render(notesIcon),
			assigneeCol.Render(assigneeText),
			updatedCol.Render(dateText),
			repoCol.Render(issueText),
		)
		rows = append(rows, SelectedRowStyle.MaxHeight(1).Render(inlineCells))
		rowIdx++
	}

	// Active Tasks
	if len(activeTasks) == 0 && !m.InlineAdding {
		rows = append(rows, lipgloss.NewStyle().Foreground(MutedColor).Render("   No active tasks. Press [a] to add a task inline!"))
	} else {
		// Calculate scrolling for active tasks
		start := 0
		activeCursor := m.TaskCursor
		if m.InlineAdding {
			activeCursor--
		}
		if activeCursor >= visibleCount {
			start = activeCursor - visibleCount + 1
		}
		end := start + visibleCount
		if end > len(activeTasks) {
			end = len(activeTasks)
		}

		for i := start; i < end; i++ {
			t := activeTasks[i]
			isCurrent := (rowIdx == m.TaskCursor)

			var prioBadge string
			if t.IsPRReview() {
				switch t.Priority {
				case github.PriorityHigh:
					prioBadge = lipgloss.NewStyle().Bold(true).Foreground(DangerColor).Render("[⑂ HIGH]")
				case github.PriorityLow:
					prioBadge = lipgloss.NewStyle().Foreground(SuccessColor).Render("[⑂ LOW]")
				default:
					prioBadge = lipgloss.NewStyle().Foreground(WarningColor).Render("[⑂ MED]")
				}
			} else {
				switch t.Priority {
				case github.PriorityHigh:
					prioBadge = lipgloss.NewStyle().Bold(true).Foreground(DangerColor).Render("[🔴 HIGH]")
				case github.PriorityLow:
					prioBadge = lipgloss.NewStyle().Foreground(SuccessColor).Render("[🟢 LOW]")
				default:
					prioBadge = lipgloss.NewStyle().Foreground(WarningColor).Render("[🟡 MED]")
				}
			}

			var titleDisplay string
			if m.InlineEditing && t.ID == m.InlineEditTaskID {
				titleDisplay = m.InlineEditTitleInput.View()
			} else {
				titleDisplay = strings.ReplaceAll(strings.ReplaceAll(t.Title, "\r", " "), "\n", " ")
				if t.IsPRReview() {
					prPrefix := lipgloss.NewStyle().Foreground(lipgloss.Color("#A371F7")).Bold(true).Render("⑂ ")
					titleDisplay = prPrefix + titleDisplay
				}
				if len(titleDisplay) > titleW-2 && titleW > 3 {
					titleDisplay = titleDisplay[:titleW-3] + "…"
				}
				if isCurrent {
					titleDisplay = lipgloss.NewStyle().Bold(true).Render(titleDisplay)
				}
			}

			notesIcon := " · "
			if len(strings.TrimSpace(t.Body)) > 0 {
				notesIcon = "📝"
			}

			var assigneeText string
			if len(t.Assignees) > 0 {
				assigneeText = "@" + strings.Join(t.Assignees, ", @")
				if len(assigneeText) > assigneeW-2 && assigneeW > 3 {
					assigneeText = assigneeText[:assigneeW-3] + "…"
				}
				assigneeText = lipgloss.NewStyle().Foreground(SecondaryColor).Render(assigneeText)
			} else {
				assigneeText = lipgloss.NewStyle().Foreground(MutedColor).Render("Unassigned")
			}

			updatedText := formatTimeAgo(t.UpdatedAt)

			issueRef := fmt.Sprintf("%s#%d", t.RepoName, t.Number)
			if len(issueRef) > repoW-2 && repoW > 3 {
				issueRef = issueRef[:repoW-3] + "…"
			}
			issueRefText := lipgloss.NewStyle().Foreground(HighlightColor).Render(issueRef)

			cursorText := "  "
			if isCurrent {
				cursorText = "▶ "
			}

			rowCells := lipgloss.JoinHorizontal(lipgloss.Top,
				cursorCol.Render(cursorText),
				prioCol.Render(prioBadge),
				titleCol.Render(titleDisplay),
				notesCol.Render(notesIcon),
				assigneeCol.Render(assigneeText),
				updatedCol.Render(updatedText),
				repoCol.Render(issueRefText),
			)

			if isCurrent {
				rows = append(rows, SelectedRowStyle.MaxHeight(1).Render(rowCells))
			} else {
				rows = append(rows, NormalRowStyle.MaxHeight(1).Render(rowCells))
			}
			rowIdx++
		}
	}

	// Completed Tasks Section (Last 5)
	if len(recentDone) > 0 {
		sepText := fmt.Sprintf("─── COMPLETED (Last %d • [d] Show All %d) ──────────────────────────────────────────────────────────", len(recentDone), allDoneCount)
		sep := lipgloss.NewStyle().Foreground(lipgloss.Color("#555566")).Render(sepText)
		rows = append(rows, "", sep)

		// Adjust rowIdx to account for remaining active tasks if scrolled
		baseDoneIdx := len(activeTasks)
		if m.InlineAdding {
			baseDoneIdx++
		}

		for i, t := range recentDone {
			curIdx := baseDoneIdx + i
			isCurrent := (curIdx == m.TaskCursor)

			var prioBadge string
			if t.IsPRReview() {
				switch t.Priority {
				case github.PriorityHigh:
					prioBadge = lipgloss.NewStyle().Foreground(DangerColor).Render("[✓⑂ HIGH]")
				case github.PriorityLow:
					prioBadge = lipgloss.NewStyle().Foreground(SuccessColor).Render("[✓⑂ LOW]")
				default:
					prioBadge = lipgloss.NewStyle().Foreground(WarningColor).Render("[✓⑂ MED]")
				}
			} else {
				switch t.Priority {
				case github.PriorityHigh:
					prioBadge = lipgloss.NewStyle().Foreground(DangerColor).Render("[✓ HIGH]")
				case github.PriorityLow:
					prioBadge = lipgloss.NewStyle().Foreground(SuccessColor).Render("[✓ LOW]")
				default:
					prioBadge = lipgloss.NewStyle().Foreground(WarningColor).Render("[✓ MED]")
				}
			}

			displayTitle := strings.ReplaceAll(strings.ReplaceAll(t.Title, "\r", " "), "\n", " ")
			if t.IsPRReview() {
				prPrefix := lipgloss.NewStyle().Foreground(lipgloss.Color("#A371F7")).Render("⑂ ")
				displayTitle = "✓ " + prPrefix + displayTitle
			} else {
				displayTitle = "✓ " + displayTitle
			}
			if len(displayTitle) > titleW-2 && titleW > 3 {
				displayTitle = displayTitle[:titleW-3] + "…"
			}
			if isCurrent {
				displayTitle = lipgloss.NewStyle().Bold(true).Render(displayTitle)
			} else {
				displayTitle = lipgloss.NewStyle().Foreground(MutedColor).Render(displayTitle)
			}

			notesIcon := " · "
			if len(strings.TrimSpace(t.Body)) > 0 {
				notesIcon = "📝"
			}

			var assigneeText string
			if len(t.Assignees) > 0 {
				assigneeText = "@" + strings.Join(t.Assignees, ", @")
				if len(assigneeText) > assigneeW-2 && assigneeW > 3 {
					assigneeText = assigneeText[:assigneeW-3] + "…"
				}
				assigneeText = lipgloss.NewStyle().Foreground(SecondaryColor).Render(assigneeText)
			} else {
				assigneeText = lipgloss.NewStyle().Foreground(MutedColor).Render("Unassigned")
			}

			var dateText string
			if t.ClosedAt != nil {
				dateText = formatTimeAgo(*t.ClosedAt)
			} else {
				dateText = formatTimeAgo(t.UpdatedAt)
			}

			issueRef := fmt.Sprintf("%s#%d", t.RepoName, t.Number)
			if len(issueRef) > repoW-2 && repoW > 3 {
				issueRef = issueRef[:repoW-3] + "…"
			}
			issueRefText := lipgloss.NewStyle().Foreground(HighlightColor).Render(issueRef)

			cursorText := "  "
			if isCurrent {
				cursorText = "▶ "
			}

			rowCells := lipgloss.JoinHorizontal(lipgloss.Top,
				cursorCol.Render(cursorText),
				prioCol.Render(prioBadge),
				titleCol.Render(displayTitle),
				notesCol.Render(notesIcon),
				assigneeCol.Render(assigneeText),
				updatedCol.Render(dateText),
				repoCol.Render(issueRefText),
			)

			if isCurrent {
				rows = append(rows, SelectedRowStyle.MaxHeight(1).Render(rowCells))
			} else {
				rows = append(rows, NormalRowStyle.MaxHeight(1).Render(rowCells))
			}
		}
	}

	// Exactly one empty row between task table and bottom hot-key row
	rows = append(rows, "")

	return strings.Join(rows, "\n")
}

func (m Model) handleTasksKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalRows := m.getTotalTaskRows()

	// 1. Handling keys while Inline Adding
	if m.InlineAdding {
		switch msg.String() {
		case "esc", "ctrl+c":
			m.InlineAdding = false
			m.InlineNewNotes = ""
			m.InlineNewTitleInput.Reset()
			m.StatusMsg = "Cancelled adding task"
			m.StatusMsgTimer = time.Now().Add(2 * time.Second)
			return m, nil

		case "enter":
			titleText := strings.TrimSpace(m.InlineNewTitleInput.Value())
			if titleText == "" {
				m.InlineAdding = false
				return m, nil
			}
			target := TaskTargetPersonal
			isSquad := false
			if m.isSquadTasks() {
				target = TaskTargetSquad
				isSquad = true
			}
			repoFullName := m.getTaskTargetRepoFullName(target)
			parts := strings.SplitN(repoFullName, "/", 2)
			owner := parts[0]
			repo := parts[1]

			var assignees []string
			if !isSquad {
				if m.AuthUser != "" {
					assignees = append(assignees, m.AuthUser)
				}
			} else if m.InlineNewAssignee != "" {
				assignees = append(assignees, m.InlineNewAssignee)
			}

			priority := m.InlineNewPriority
			if priority == "" {
				priority = github.PriorityMedium
			}
			body := m.InlineNewNotes

			now := time.Now()
			tempTask := github.Task{
				ID:          -now.UnixNano(),
				Number:      0,
				Title:       titleText,
				Body:        body,
				State:       "open",
				Priority:    priority,
				RepoOwner:   owner,
				RepoName:    repo,
				Author:      m.AuthUser,
				Assignees:   assignees,
				Labels:      []string{priority.Label()},
				CreatedAt:   now,
				UpdatedAt:   now,
				IsSquadTask: isSquad,
			}
			if isSquad {
				m.SquadTasks = append([]github.Task{tempTask}, m.SquadTasks...)
			} else {
				m.PersonalTasks = append([]github.Task{tempTask}, m.PersonalTasks...)
			}
			if m.DB != nil {
				_ = m.DB.SaveTask(&tempTask, "pending_create")
			}
			m.InlineAdding = false
			m.InlineNewNotes = ""
			m.InlineNewTitleInput.Reset()
			m.TaskCursor = 0
			m.StatusMsg = fmt.Sprintf("✓ Created task draft in %s (syncing with GitHub...)", repoFullName)
			m.StatusMsgTimer = time.Now().Add(3 * time.Second)
			return m, m.createTaskCmd(owner, repo, titleText, body, priority, assignees, isSquad)

		case "ctrl+n":
			return m.openTaskNotesModal(nil, true)

		case "ctrl+p":
			switch m.InlineNewPriority {
			case github.PriorityMedium:
				m.InlineNewPriority = github.PriorityHigh
			case github.PriorityHigh:
				m.InlineNewPriority = github.PriorityLow
			default:
				m.InlineNewPriority = github.PriorityMedium
			}
			m.StatusMsg = fmt.Sprintf("Priority set to %s for new task", m.InlineNewPriority)
			return m, nil

		case "tab":
			if m.isSquadTasks() {
				candidates := m.getSquadTaskAssigneeCandidates()
				curIdx := 0
				for i, c := range candidates {
					if c == m.InlineNewAssignee {
						curIdx = i
						break
					}
				}
				m.InlineNewAssignee = candidates[(curIdx+1)%len(candidates)]
				return m, nil
			}
		}

		var cmd tea.Cmd
		m.InlineNewTitleInput, cmd = m.InlineNewTitleInput.Update(msg)
		return m, cmd
	}

	// 2. Handling keys while Inline Editing an existing task's title
	if m.InlineEditing {
		switch msg.String() {
		case "esc", "ctrl+c":
			m.InlineEditing = false
			m.StatusMsg = "Cancelled editing title"
			m.StatusMsgTimer = time.Now().Add(2 * time.Second)
			return m, nil

		case "enter":
			newTitle := strings.TrimSpace(m.InlineEditTitleInput.Value())
			m.InlineEditing = false
			if newTitle != "" {
				for _, t := range m.getActiveTasks() {
					if t.ID == m.InlineEditTaskID {
						target := t
						target.Title = newTitle
						target.UpdatedAt = time.Now()
						m = m.updateTaskInSlices(target)
						if m.DB != nil {
							_ = m.DB.SaveTask(&target, "pending_update")
						}
						m.StatusMsg = fmt.Sprintf("✓ Saved title for task #%d", target.Number)
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

						return m, m.updateTaskCmd(t, newTitle, t.Body, t.Priority, t.Assignees)
					}
				}
			}
			return m, nil

		case "ctrl+n":
			for _, t := range m.getActiveTasks() {
				if t.ID == m.InlineEditTaskID {
					target := t
					return m.openTaskNotesModal(&target, false)
				}
			}
		}

		var cmd tea.Cmd
		m.InlineEditTitleInput, cmd = m.InlineEditTitleInput.Update(msg)
		return m, cmd
	}

	// 3. Normal Task Navigation & Actions
	switch msg.String() {
	case "up", "k":
		if m.TaskCursor > 0 {
			m.TaskCursor--
		}

	case "down", "j":
		if m.TaskCursor < totalRows-1 {
			m.TaskCursor++
		}

	case "pgdown", "ctrl+d":
		m.TaskCursor += 8
		if m.TaskCursor >= totalRows {
			m.TaskCursor = totalRows - 1
		}
		if m.TaskCursor < 0 {
			m.TaskCursor = 0
		}

	case "pgup", "ctrl+u":
		m.TaskCursor -= 8
		if m.TaskCursor < 0 {
			m.TaskCursor = 0
		}

	case "home", "g":
		m.TaskCursor = 0

	case "end", "G":
		if totalRows > 0 {
			m.TaskCursor = totalRows - 1
		}

	case "left", "right", "h", "l", "[", "]":
		// Switch between My Tasks and Squad Tasks
		if m.isSquadTasks() {
			m.ActiveTab = TabMyTasks
			m.ActiveTaskSubtab = TaskSubtabMine
		} else {
			m.ActiveTab = TabSquadTasks
			m.ActiveTaskSubtab = TaskSubtabSquad
		}
		m.TaskCursor = 0
		return m, nil

	case "a":
		// Add new task inline
		m.InlineAdding = true
		m.InlineNewNotes = ""
		m.InlineNewPriority = github.PriorityMedium
		if m.isSquadTasks() {
			m.InlineNewAssignee = ""
		} else {
			m.InlineNewAssignee = m.AuthUser
		}
		ti := textinput.New()
		ti.Placeholder = "What needs to be done? (Enter to create, Esc to cancel)"
		ti.Prompt = ""
		ti.Focus()
		m.InlineNewTitleInput = ti
		m.TaskCursor = 0
		return m, textinput.Blink

	case "enter", "e":
		loc, task := m.getCursorTarget()
		if task != nil {
			if task.IsPRReview() {
				targetURL := task.PRURL()
				if targetURL == "" {
					targetURL = task.URL
				}
				_ = OpenBrowser(targetURL)

				// If a review task is assigned to multiples, unassign from everyone but the person launching the review
				if len(task.Assignees) > 1 && m.AuthUser != "" {
					newAssignees := []string{m.AuthUser}
					taskCopy := *task
					taskCopy.Assignees = newAssignees
					m = m.updateTaskInSlices(taskCopy)
					assignCmd := m.updateTaskAssigneesCmd(taskCopy, newAssignees)
					return m, tea.Batch(
						func() tea.Msg {
							return StatusToastMsg(fmt.Sprintf("Opened PR & claimed review task #%d (assigned to @%s)", task.Number, m.AuthUser))
						},
						assignCmd,
					)
				}

				return m, func() tea.Msg {
					return StatusToastMsg(fmt.Sprintf("Opened PR in browser: %s", targetURL))
				}
			}
			if loc == locActiveTask {
				m.InlineEditing = true
				m.InlineEditTaskID = task.ID
				ieti := textinput.New()
				ieti.SetValue(task.Title)
				ieti.Prompt = ""
				ieti.Focus()
				m.InlineEditTitleInput = ieti
				return m, textinput.Blink
			}
		}

	case " ", "x":
		loc, task := m.getCursorTarget()
		if task != nil {
			if loc == locCompletedTask {
				target := *task
				m = m.reopenTaskLocally(target)
				if m.DB != nil {
					_ = m.DB.SaveTask(&target, "pending_reopen")
				}
				m.StatusMsg = fmt.Sprintf("✓ Reopened task #%d in %s", task.Number, task.RepoName)
				m.StatusMsgTimer = time.Now().Add(3 * time.Second)
				return m, m.reopenTaskCmd(*task)
			} else if loc == locActiveTask {
				target := *task
				m = m.closeTaskLocally(target)
				if m.DB != nil {
					_ = m.DB.SaveTask(&target, "pending_close")
				}
				m.StatusMsg = fmt.Sprintf("✓ Completed task #%d in %s", task.Number, task.RepoName)
				m.StatusMsgTimer = time.Now().Add(3 * time.Second)
				return m, m.closeTaskCmd(*task)
			}
		}

	case "n":
		loc, task := m.getCursorTarget()
		if loc == locInlineNew {
			return m.openTaskNotesModal(nil, true)
		} else if task != nil {
			return m.openTaskNotesModal(task, false)
		}

	case "p":
		loc, task := m.getCursorTarget()
		if loc == locInlineNew {
			switch m.InlineNewPriority {
			case github.PriorityMedium:
				m.InlineNewPriority = github.PriorityHigh
			case github.PriorityHigh:
				m.InlineNewPriority = github.PriorityLow
			default:
				m.InlineNewPriority = github.PriorityMedium
			}
			m.StatusMsg = fmt.Sprintf("Priority set to %s for new task", m.InlineNewPriority)
			return m, nil
		} else if task != nil && loc == locActiveTask {
			var nextPriority github.TaskPriority
			switch task.Priority {
			case github.PriorityMedium:
				nextPriority = github.PriorityHigh
			case github.PriorityHigh:
				nextPriority = github.PriorityLow
			default:
				nextPriority = github.PriorityMedium
			}
			target := *task
			target.Priority = nextPriority
			target.UpdatedAt = time.Now()
			m = m.updateTaskInSlices(target)
			if m.DB != nil {
				_ = m.DB.SaveTask(&target, "pending_update")
			}
			m.StatusMsg = fmt.Sprintf("✓ Priority set to %s for task #%d", nextPriority, task.Number)
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
					Action:    storage.ActionPriorityTask,
				}, func(ctx context.Context, job *storage.TaskDebounceJob) error {
					var err error
					if m.GHClient != nil {
						err = m.GHClient.UpdateTaskPriority(ctx, job.RepoOwner, job.RepoName, job.Number, job.Priority, job.Labels)
					}
					if err == nil && m.DB != nil {
						_ = m.DB.UpdateTaskSyncStatus(job.TaskID, "synced")
					}
					return err
				})
				return m, nil
			}

			return m, m.updateTaskPriorityCmd(*task, nextPriority)
		}

	case "A":
		if m.isSquadTasks() {
			loc, task := m.getCursorTarget()
			if task != nil && loc == locActiveTask {
				m.ActiveModal = ModalAssignSquadTask
				m.AssignTaskTarget = task
				m.AssignTaskCandidates = m.getSquadAssigneeOptions()
				m.AssignTaskMemberCursor = 0
				m.AssignTaskFocusedField = 0
				m.AssignTaskActiveButton = 0
				m.AssignTaskSelectedMembers = make(map[string]bool)
				for _, a := range task.Assignees {
					m.AssignTaskSelectedMembers[a] = true
				}
				return m, nil
			}
		}

	case "c":
		loc, task := m.getCursorTarget()
		candidates := m.getSquadTaskAssigneeCandidates()
		if loc == locInlineNew && m.isSquadTasks() {
			currentIdx := 0
			for i, c := range candidates {
				if c == m.InlineNewAssignee {
					currentIdx = i
					break
				}
			}
			m.InlineNewAssignee = candidates[(currentIdx+1)%len(candidates)]
			return m, nil
		} else if task != nil && task.IsSquadTask && loc == locActiveTask {
			currentAssignee := ""
			if len(task.Assignees) > 0 {
				currentAssignee = task.Assignees[0]
			}
			currentIdx := 0
			for i, c := range candidates {
				if c == currentAssignee {
					currentIdx = i
					break
				}
			}
			nextAssignee := candidates[(currentIdx+1)%len(candidates)]
			var newAssignees []string
			if nextAssignee != "" {
				newAssignees = append(newAssignees, nextAssignee)
			}
			target := *task
			target.Assignees = newAssignees
			target.UpdatedAt = time.Now()
			m = m.updateTaskInSlices(target)
			if m.DB != nil {
				_ = m.DB.SaveTask(&target, "pending_update")
			}
			m.StatusMsg = fmt.Sprintf("✓ Assigned task #%d to %s", target.Number, nextAssignee)
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
					Action:    storage.ActionAssignTask,
				}, func(ctx context.Context, job *storage.TaskDebounceJob) error {
					var err error
					if m.GHClient != nil {
						err = m.GHClient.UpdateTaskAssignees(ctx, job.RepoOwner, job.RepoName, job.Number, job.Assignees)
					}
					if err == nil && m.DB != nil {
						_ = m.DB.UpdateTaskSyncStatus(job.TaskID, "synced")
					}
					return err
				})
				return m, nil
			}

			return m, m.updateTaskAssigneesCmd(*task, newAssignees)
		}

	case "d":
		// Open Completed Tasks modal
		m.ActiveModal = ModalCompletedTasks
		m.CompletedTasksCursor = 0
		return m, nil

	case "i":
		// Always open the underlying GitHub issue in browser
		_, task := m.getCursorTarget()
		if task != nil && task.URL != "" {
			_ = OpenBrowser(task.URL)
			return m, func() tea.Msg {
				return StatusToastMsg(fmt.Sprintf("Opened issue #%d in browser", task.Number))
			}
		}

	case "o":
		// Open in browser
		_, task := m.getCursorTarget()
		if task != nil {
			targetURL := task.URL
			if task.IsPRReview() {
				if prURL := task.PRURL(); prURL != "" {
					targetURL = prURL
				}
				_ = OpenBrowser(targetURL)

				if len(task.Assignees) > 1 && m.AuthUser != "" {
					newAssignees := []string{m.AuthUser}
					taskCopy := *task
					taskCopy.Assignees = newAssignees
					m = m.updateTaskInSlices(taskCopy)
					assignCmd := m.updateTaskAssigneesCmd(taskCopy, newAssignees)
					return m, tea.Batch(
						func() tea.Msg {
							return StatusToastMsg(fmt.Sprintf("Opened PR & claimed review task #%d (assigned to @%s)", task.Number, m.AuthUser))
						},
						assignCmd,
					)
				}

				return m, func() tea.Msg {
					return StatusToastMsg(fmt.Sprintf("Opened in browser: %s", targetURL))
				}
			}
			if targetURL != "" {
				_ = OpenBrowser(targetURL)
				return m, func() tea.Msg {
					return StatusToastMsg(fmt.Sprintf("Opened in browser: %s", targetURL))
				}
			}
		}
	}

	return m, nil
}
