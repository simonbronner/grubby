package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"grubby/internal/github"
)

func (m Model) renderCreatePRTaskModal() string {
	pr := m.PRTaskTarget
	title := ModalTitleStyle.Render(fmt.Sprintf("Create Review Task for PR #%d", pr.Number))

	// PR Summary
	prTitleDisplay := pr.Title
	if len(prTitleDisplay) > 55 {
		prTitleDisplay = prTitleDisplay[:52] + "…"
	}
	prInfo := lipgloss.NewStyle().Foreground(HighlightColor).Bold(true).Render(fmt.Sprintf("⑂ %s (%s)", prTitleDisplay, pr.Repo))
	authorInfo := lipgloss.NewStyle().Foreground(SecondaryColor).Render(fmt.Sprintf("Author: @%s", pr.Author))
	summaryLine := fmt.Sprintf("  %s  •  %s", prInfo, authorInfo)

	// 1. Assignee Field (List with selections)
	assignHeader := "Assign to:"
	if m.PRTaskFocusedField == 0 {
		assignHeader = lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render("▶ Assign to:") +
			lipgloss.NewStyle().Foreground(MutedColor).Render("  [Space/Enter] Toggle  [a] All  [m] Me  [c] Clear")
	} else {
		assignHeader = "  " + lipgloss.NewStyle().Bold(true).Render("Assign to:")
	}

	candidates := m.PRTaskCandidates
	if len(candidates) == 0 {
		candidates = m.getSquadAssigneeOptions()
	}

	var memberLines []string
	if len(candidates) == 0 {
		memberLines = append(memberLines, "    (No squad members found)")
	} else {
		maxVisible := 5
		cursor := m.PRTaskMemberCursor
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
			isCurrent := (m.PRTaskFocusedField == 0 && i == cursor)
			rowPrefix := "    "
			if isCurrent {
				rowPrefix = "  ▶ "
			}

			checked := m.PRTaskSelectedMembers != nil && m.PRTaskSelectedMembers[c]
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
			if strings.EqualFold(c, pr.Author) {
				label += " (author)"
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
		if m.PRTaskSelectedMembers != nil && m.PRTaskSelectedMembers[c] {
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

	// 2. Priority Field
	var prioLow, prioMed, prioHigh string
	switch m.PRTaskPriority {
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
	if m.PRTaskFocusedField == 1 {
		prioLine = lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render("▶ " + prioLine)
	} else {
		prioLine = "  " + prioLine
	}

	// 3. Target repo & metadata info
	repoInfo := m.getTaskTargetRepoFullName(TaskTargetSquad)
	repoLine := lipgloss.NewStyle().Foreground(lipgloss.Color("#7F8497")).Render(
		fmt.Sprintf("  Backing Repo: %s  •  Label: pr-review", repoInfo),
	)
	autoCloseNotice := lipgloss.NewStyle().Foreground(lipgloss.Color("#8888AA")).Render(
		"  Auto-Close: Links to PR; auto-closes when PR merges, or unassigns others on review start.",
	)

	// 4. Buttons
	var btnCreate, btnCancel string
	if m.PRTaskFocusedField == 2 {
		if m.PRTaskActiveButton == 0 {
			btnCreate = lipgloss.NewStyle().Bold(true).Background(PrimaryColor).Foreground(lipgloss.Color("#FFFFFF")).Render(" [ Create Task ] ")
			btnCancel = lipgloss.NewStyle().Foreground(MutedColor).Render(" [ Cancel ] ")
		} else {
			btnCreate = lipgloss.NewStyle().Foreground(MutedColor).Render(" [ Create Task ] ")
			btnCancel = lipgloss.NewStyle().Bold(true).Background(DangerColor).Foreground(lipgloss.Color("#FFFFFF")).Render(" [ Cancel ] ")
		}
	} else {
		btnCreate = lipgloss.NewStyle().Foreground(PrimaryColor).Render(" [ Create Task ] ")
		btnCancel = lipgloss.NewStyle().Foreground(MutedColor).Render(" [ Cancel ] ")
	}
	buttonLine := fmt.Sprintf("               %s   %s", btnCreate, btnCancel)

	// Help line
	helpLine := MetaBarStyle.Render("  [Tab/Shift+Tab] Move to Buttons  •  [Enter on Create Task] Confirm  •  [Esc] Cancel")

	var contentLines []string
	contentLines = append(contentLines, title, "", summaryLine, "", assignHeader)
	contentLines = append(contentLines, memberLines...)
	contentLines = append(contentLines, summaryText, "", prioLine, "", repoLine, autoCloseNotice, "", buttonLine, "", helpLine)

	return ModalBoxStyle.Render(strings.Join(contentLines, "\n"))
}

func (m Model) handleCreatePRTaskModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	candidates := m.PRTaskCandidates
	if len(candidates) == 0 {
		candidates = m.getSquadAssigneeOptions()
		m.PRTaskCandidates = candidates
	}
	if m.PRTaskSelectedMembers == nil {
		m.PRTaskSelectedMembers = make(map[string]bool)
	}

	switch msg.String() {
	case "esc":
		m.ActiveModal = ModalNone
		return m, nil

	case "tab":
		m.PRTaskFocusedField = (m.PRTaskFocusedField + 1) % 3
		return m, nil

	case "shift+tab":
		m.PRTaskFocusedField--
		if m.PRTaskFocusedField < 0 {
			m.PRTaskFocusedField = 2
		}
		return m, nil

	case "up", "k":
		if m.PRTaskFocusedField == 0 {
			if m.PRTaskMemberCursor > 0 {
				m.PRTaskMemberCursor--
			}
			return m, nil
		}
		if m.PRTaskFocusedField > 0 {
			m.PRTaskFocusedField--
		}
		return m, nil

	case "down", "j":
		if m.PRTaskFocusedField == 0 {
			if m.PRTaskMemberCursor < len(candidates)-1 {
				m.PRTaskMemberCursor++
			}
			return m, nil
		}
		if m.PRTaskFocusedField < 2 {
			m.PRTaskFocusedField++
		}
		return m, nil

	case " ":
		if m.PRTaskFocusedField == 0 {
			if len(candidates) > 0 && m.PRTaskMemberCursor < len(candidates) {
				mem := candidates[m.PRTaskMemberCursor]
				m.PRTaskSelectedMembers[mem] = !m.PRTaskSelectedMembers[mem]
			}
			return m, nil
		}
		if m.PRTaskFocusedField == 1 {
			switch m.PRTaskPriority {
			case github.PriorityMedium:
				m.PRTaskPriority = github.PriorityHigh
			case github.PriorityHigh:
				m.PRTaskPriority = github.PriorityLow
			default:
				m.PRTaskPriority = github.PriorityMedium
			}
			return m, nil
		}
		if m.PRTaskFocusedField == 2 {
			m.PRTaskActiveButton = (m.PRTaskActiveButton + 1) % 2
			return m, nil
		}

	case "a", "s":
		// Select all squad members (excluding PR author & me)
		pr := m.PRTaskTarget
		reviewCands := m.getSquadReviewCandidates(pr.Author)
		if len(reviewCands) > 0 {
			m.PRTaskSelectedMembers = make(map[string]bool)
			for _, c := range reviewCands {
				m.PRTaskSelectedMembers[c] = true
			}
		} else {
			// Select all candidates if no other squad members exist
			for _, c := range candidates {
				m.PRTaskSelectedMembers[c] = true
			}
		}
		return m, nil

	case "m":
		// Select myself only
		m.PRTaskSelectedMembers = make(map[string]bool)
		if m.AuthUser != "" {
			m.PRTaskSelectedMembers[m.AuthUser] = true
		}
		return m, nil

	case "c":
		// Clear all selections
		m.PRTaskSelectedMembers = make(map[string]bool)
		return m, nil

	case "p":
		// Cycle priority
		switch m.PRTaskPriority {
		case github.PriorityMedium:
			m.PRTaskPriority = github.PriorityHigh
		case github.PriorityHigh:
			m.PRTaskPriority = github.PriorityLow
		default:
			m.PRTaskPriority = github.PriorityMedium
		}
		return m, nil

	case "left", "h":
		if m.PRTaskFocusedField == 1 {
			switch m.PRTaskPriority {
			case github.PriorityHigh:
				m.PRTaskPriority = github.PriorityLow
			case github.PriorityLow:
				m.PRTaskPriority = github.PriorityMedium
			default:
				m.PRTaskPriority = github.PriorityHigh
			}
			return m, nil
		}
		if m.PRTaskFocusedField == 2 {
			m.PRTaskActiveButton = 0
			return m, nil
		}

	case "right", "l":
		if m.PRTaskFocusedField == 1 {
			switch m.PRTaskPriority {
			case github.PriorityMedium:
				m.PRTaskPriority = github.PriorityHigh
			case github.PriorityHigh:
				m.PRTaskPriority = github.PriorityLow
			default:
				m.PRTaskPriority = github.PriorityMedium
			}
			return m, nil
		}
		if m.PRTaskFocusedField == 2 {
			m.PRTaskActiveButton = 1
			return m, nil
		}

	case "enter":
		// Enter on members list: toggle selection (does NOT save!)
		if m.PRTaskFocusedField == 0 {
			if len(candidates) > 0 && m.PRTaskMemberCursor < len(candidates) {
				mem := candidates[m.PRTaskMemberCursor]
				m.PRTaskSelectedMembers[mem] = !m.PRTaskSelectedMembers[mem]
			}
			return m, nil
		}

		// Enter on priority: move focus to buttons (does NOT quick save!)
		if m.PRTaskFocusedField == 1 {
			m.PRTaskFocusedField = 2
			m.PRTaskActiveButton = 0
			return m, nil
		}

		// Enter on buttons: execute active button
		if m.PRTaskFocusedField == 2 {
			if m.PRTaskActiveButton == 1 {
				m.ActiveModal = ModalNone
				return m, nil
			}
			return m.submitCreatePRTaskModal()
		}
	}

	return m, nil
}

func (m Model) submitCreatePRTaskModal() (tea.Model, tea.Cmd) {
	pr := m.PRTaskTarget

	var assignees []string
	candidates := m.PRTaskCandidates
	if len(candidates) == 0 {
		candidates = m.getSquadAssigneeOptions()
	}

	for _, cand := range candidates {
		if m.PRTaskSelectedMembers != nil && m.PRTaskSelectedMembers[cand] {
			assignees = append(assignees, cand)
		}
	}

	// Task Title (informative and immutable)
	title := fmt.Sprintf("Review PR #%d: %s (%s)", pr.Number, pr.Title, pr.Repo)

	// Task Body with PR link and auto-closing note
	body := fmt.Sprintf(`Review pull request: %s

Repository: %s
Author: @%s

Note: GitHub automatically closes this task when the linked pull request is merged, or it can be completed manually in Grubby with [Space].`,
		pr.URL, pr.Repo, pr.Author,
	)

	priority := m.PRTaskPriority
	if priority == "" {
		priority = github.PriorityMedium
	}

	labels := []string{github.LabelPRReview}

	repoFullName := m.getTaskTargetRepoFullName(TaskTargetSquad)
	parts := strings.SplitN(repoFullName, "/", 2)
	owner := parts[0]
	repo := parts[1]

	m.ActiveModal = ModalNone
	m.StatusMsg = fmt.Sprintf("Creating PR review task for #%d in %s...", pr.Number, repoFullName)
	m.StatusMsgTimer = time.Now().Add(4 * time.Second)

	return m, m.createPRTaskCmd(owner, repo, title, body, priority, labels, assignees, true, pr)
}
