package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"grubby/internal/github"
)

func (m Model) getSortedDoneTasks() []github.Task {
	var list []github.Task
	if m.ActiveTaskSubtab == TaskSubtabMine {
		list = append(list, m.PersonalDoneTasks...)
	} else {
		list = append(list, m.SquadDoneTasks...)
	}

	sort.SliceStable(list, func(i, j int) bool {
		timeI := list[i].UpdatedAt
		if list[i].ClosedAt != nil {
			timeI = *list[i].ClosedAt
		}
		timeJ := list[j].UpdatedAt
		if list[j].ClosedAt != nil {
			timeJ = *list[j].ClosedAt
		}
		if !timeI.Equal(timeJ) {
			return timeI.After(timeJ)
		}
		return list[i].Number > list[j].Number
	})

	return list
}

func (m Model) renderCompletedTasksModal() string {
	list := m.getSortedDoneTasks()

	subtabName := "My Tasks"
	if m.ActiveTaskSubtab == TaskSubtabSquad {
		subtabName = "Squad Tasks"
	}
	title := ModalTitleStyle.Render(fmt.Sprintf("Completed Tasks (%d) — %s", len(list), subtabName))

	if len(list) == 0 {
		emptyMsg := "  No completed tasks found.\n\n  Complete an active task with [Space] to see it here!\n"
		helpLine := MetaBarStyle.Render("  [Esc] Close")
		content := strings.Join([]string{title, "", emptyMsg, helpLine}, "\n")
		return ModalBoxStyle.Render(content)
	}

	if m.CompletedTasksCursor >= len(list) {
		m.CompletedTasksCursor = len(list) - 1
	}
	if m.CompletedTasksCursor < 0 {
		m.CompletedTasksCursor = 0
	}

	// Columns
	cursorW := 3
	prioW := 10
	notesW := 3
	dateW := 12
	repoW := 14
	titleW := 45

	cursorCol := lipgloss.NewStyle().Width(cursorW).MaxHeight(1).Inline(true)
	prioCol := lipgloss.NewStyle().Width(prioW).MaxHeight(1).Inline(true)
	titleCol := lipgloss.NewStyle().Width(titleW).MaxHeight(1).Inline(true)
	notesCol := lipgloss.NewStyle().Width(notesW).MaxHeight(1).Inline(true)
	dateCol := lipgloss.NewStyle().Width(dateW).MaxHeight(1).Inline(true)
	repoCol := lipgloss.NewStyle().Width(repoW).MaxHeight(1).Inline(true)

	tableHead := lipgloss.JoinHorizontal(lipgloss.Top,
		cursorCol.Render(""),
		prioCol.Render("PRIORITY"),
		titleCol.Render("TASK TITLE"),
		notesCol.Render("📝"),
		dateCol.Render("COMPLETED"),
		repoCol.Render("ISSUE"),
	)
	renderedTableHead := TableHeaderStyle.Render(tableHead)

	var rows []string
	rows = append(rows, title, "", renderedTableHead)

	visibleCount := 12
	start := 0
	if m.CompletedTasksCursor >= visibleCount {
		start = m.CompletedTasksCursor - visibleCount + 1
	}
	end := start + visibleCount
	if end > len(list) {
		end = len(list)
	}

	for i := start; i < end; i++ {
		t := list[i]

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
		if i == m.CompletedTasksCursor {
			displayTitle = lipgloss.NewStyle().Bold(true).Render(displayTitle)
		} else {
			displayTitle = lipgloss.NewStyle().Foreground(MutedColor).Render(displayTitle)
		}

		notesIcon := " · "
		if len(strings.TrimSpace(t.Body)) > 0 {
			notesIcon = "📝"
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
		if i == m.CompletedTasksCursor {
			cursorText = "▶ "
		}

		rowCells := lipgloss.JoinHorizontal(lipgloss.Top,
			cursorCol.Render(cursorText),
			prioCol.Render(prioBadge),
			titleCol.Render(displayTitle),
			notesCol.Render(notesIcon),
			dateCol.Render(dateText),
			repoCol.Render(issueRefText),
		)

		if i == m.CompletedTasksCursor {
			rows = append(rows, SelectedRowStyle.MaxHeight(1).Render(rowCells))
		} else {
			rows = append(rows, NormalRowStyle.MaxHeight(1).Render(rowCells))
		}
	}

	helpLineText := "  [Space] Reopen  •  [n] View Notes  •  [o] Web  •  [j/k] Navigate  •  [Esc] Close"
	if len(list) > 0 && m.CompletedTasksCursor < len(list) && list[m.CompletedTasksCursor].IsPRReview() {
		helpLineText = "  [Enter/o] Open PR  •  [Space] Reopen  •  [n] View Notes  •  [j/k] Navigate  •  [Esc] Close"
	}
	helpLine := MetaBarStyle.Render(helpLineText)
	rows = append(rows, "", helpLine)

	return ModalBoxStyle.Render(strings.Join(rows, "\n"))
}

func (m Model) handleCompletedTasksModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	list := m.getSortedDoneTasks()

	switch msg.String() {
	case "esc", "q", "d":
		m.ActiveModal = ModalNone
		return m, nil

	case "up", "k":
		if m.CompletedTasksCursor > 0 {
			m.CompletedTasksCursor--
		}

	case "down", "j":
		if m.CompletedTasksCursor < len(list)-1 {
			m.CompletedTasksCursor++
		}

	case "home", "g":
		m.CompletedTasksCursor = 0

	case "end", "G":
		if len(list) > 0 {
			m.CompletedTasksCursor = len(list) - 1
		}

	case "pgup", "ctrl+u":
		m.CompletedTasksCursor -= 10
		if m.CompletedTasksCursor < 0 {
			m.CompletedTasksCursor = 0
		}

	case "pgdown", "ctrl+d":
		m.CompletedTasksCursor += 10
		if m.CompletedTasksCursor >= len(list) {
			m.CompletedTasksCursor = len(list) - 1
		}
		if m.CompletedTasksCursor < 0 {
			m.CompletedTasksCursor = 0
		}

	case " ", "x":
		if len(list) > 0 && m.CompletedTasksCursor < len(list) {
			target := list[m.CompletedTasksCursor]
			m.StatusMsg = fmt.Sprintf("Reopening task #%d in %s...", target.Number, target.RepoName)
			return m, m.reopenTaskCmd(target)
		}

	case "n":
		if len(list) > 0 && m.CompletedTasksCursor < len(list) {
			target := list[m.CompletedTasksCursor]
			return m.openTaskNotesModal(&target, false)
		}

	case "enter":
		if len(list) > 0 && m.CompletedTasksCursor < len(list) {
			target := list[m.CompletedTasksCursor]
			if target.IsPRReview() {
				targetURL := target.PRURL()
				if targetURL == "" {
					targetURL = target.URL
				}
				_ = OpenBrowser(targetURL)
				return m, func() tea.Msg {
					return StatusToastMsg(fmt.Sprintf("Opened PR in browser: %s", targetURL))
				}
			}
		}

	case "o":
		if len(list) > 0 && m.CompletedTasksCursor < len(list) {
			target := list[m.CompletedTasksCursor]
			targetURL := target.URL
			if target.IsPRReview() {
				if prURL := target.PRURL(); prURL != "" {
					targetURL = prURL
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
