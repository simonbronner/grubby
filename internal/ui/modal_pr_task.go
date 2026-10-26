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

	// 1. Assignee Field (Mode selector)
	var squadRadio, indRadio, meRadio string
	switch m.PRTaskAssignMode {
	case 0:
		squadRadio = lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render("[● Entire Squad (excl. me)]")
		indRadio = lipgloss.NewStyle().Foreground(MutedColor).Render("[○ Individual]")
		meRadio = lipgloss.NewStyle().Foreground(MutedColor).Render("[○ Myself]")
	case 1:
		squadRadio = lipgloss.NewStyle().Foreground(MutedColor).Render("[○ Entire Squad (excl. me)]")
		indRadio = lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render("[● Individual]")
		meRadio = lipgloss.NewStyle().Foreground(MutedColor).Render("[○ Myself]")
	case 2:
		squadRadio = lipgloss.NewStyle().Foreground(MutedColor).Render("[○ Entire Squad (excl. me)]")
		indRadio = lipgloss.NewStyle().Foreground(MutedColor).Render("[○ Individual]")
		meRadio = lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render("[● Myself]")
	}

	modeLine := fmt.Sprintf("Assign to:     %s   %s   %s", squadRadio, indRadio, meRadio)
	if m.PRTaskFocusedField == 0 {
		modeLine = lipgloss.NewStyle().Bold(true).Render("▶ " + modeLine)
	} else {
		modeLine = "  " + modeLine
	}

	// Details for the chosen assignment mode
	var assignDetail string
	switch m.PRTaskAssignMode {
	case 0:
		candidates := m.getSquadReviewCandidates(pr.Author)
		if len(candidates) == 0 {
			assignDetail = lipgloss.NewStyle().Foreground(WarningColor).Render("               (No other squad members in active squad • will create unassigned)")
		} else {
			assignDetail = lipgloss.NewStyle().Foreground(SecondaryColor).Render(fmt.Sprintf("               Squad Members: @%s", strings.Join(candidates, ", @")))
		}
	case 1:
		candidates := m.getSquadIndividualCandidates()
		if len(candidates) == 0 {
			assignDetail = lipgloss.NewStyle().Foreground(WarningColor).Render("               (No squad members configured)")
		} else {
			currentMember := m.PRTaskAssignee
			if currentMember == "" && len(candidates) > 0 {
				currentMember = candidates[0]
			}
			assignDetail = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(fmt.Sprintf("               Selected Member: @%s", currentMember)) +
				lipgloss.NewStyle().Foreground(MutedColor).Render("  [Space to cycle member]")
		}
	case 2:
		me := m.AuthUser
		if me == "" {
			me = "me"
		}
		assignDetail = lipgloss.NewStyle().Foreground(SecondaryColor).Render(fmt.Sprintf("               Assignee: @%s", me))
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
		prioLine = lipgloss.NewStyle().Bold(true).Render("▶ " + prioLine)
	} else {
		prioLine = "  " + prioLine
	}

	// 3. Target repo & metadata info
	repoInfo := m.getTaskTargetRepoFullName(TaskTargetSquad)
	repoLine := lipgloss.NewStyle().Foreground(lipgloss.Color("#7F8497")).Render(
		fmt.Sprintf("  Backing Repo: %s  •  Label: pr-review", repoInfo),
	)
	autoCloseNotice := lipgloss.NewStyle().Foreground(lipgloss.Color("#8888AA")).Render(
		"  Auto-Close: Links to PR; auto-closes when PR merges, or complete with [Space].",
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
	helpLine := MetaBarStyle.Render("  [Tab/Shift+Tab] Move  •  [Enter] Confirm  •  [p] Priority  •  [a] Mode  •  [Esc] Cancel")

	content := strings.Join([]string{
		title,
		"",
		summaryLine,
		"",
		modeLine,
		assignDetail,
		"",
		prioLine,
		"",
		repoLine,
		autoCloseNotice,
		"",
		buttonLine,
		"",
		helpLine,
	}, "\n")

	return ModalBoxStyle.Render(content)
}

func (m Model) handleCreatePRTaskModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		if m.PRTaskFocusedField > 0 {
			m.PRTaskFocusedField--
		}
		return m, nil

	case "down", "j":
		if m.PRTaskFocusedField < 2 {
			m.PRTaskFocusedField++
		}
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

	case "a":
		// Cycle assign mode
		m.PRTaskAssignMode = (m.PRTaskAssignMode + 1) % 3
		if m.PRTaskAssignMode == 1 {
			m = m.initPRTaskIndividualAssignee()
		}
		return m, nil

	case "1":
		m.PRTaskAssignMode = 0
		return m, nil

	case "2":
		m.PRTaskAssignMode = 1
		m = m.initPRTaskIndividualAssignee()
		return m, nil

	case "3":
		m.PRTaskAssignMode = 2
		return m, nil

	case "left", "h":
		if m.PRTaskFocusedField == 0 {
			m.PRTaskAssignMode--
			if m.PRTaskAssignMode < 0 {
				m.PRTaskAssignMode = 2
			}
			if m.PRTaskAssignMode == 1 {
				m = m.initPRTaskIndividualAssignee()
			}
			return m, nil
		}
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
		if m.PRTaskFocusedField == 0 {
			m.PRTaskAssignMode = (m.PRTaskAssignMode + 1) % 3
			if m.PRTaskAssignMode == 1 {
				m = m.initPRTaskIndividualAssignee()
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
			m.PRTaskActiveButton = 1
			return m, nil
		}

	case "m":
		if m.PRTaskFocusedField == 0 && m.PRTaskAssignMode == 1 {
			m = m.cyclePRTaskIndividualAssignee(1)
			return m, nil
		}

	case " ":
		if m.PRTaskFocusedField == 0 {
			if m.PRTaskAssignMode == 1 {
				m = m.cyclePRTaskIndividualAssignee(1)
			} else {
				m.PRTaskAssignMode = (m.PRTaskAssignMode + 1) % 3
				if m.PRTaskAssignMode == 1 {
					m = m.initPRTaskIndividualAssignee()
				}
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
			if m.PRTaskActiveButton == 1 {
				m.ActiveModal = ModalNone
				return m, nil
			}
			return m.submitCreatePRTaskModal()
		}

	case "enter":
		if m.PRTaskFocusedField == 2 && m.PRTaskActiveButton == 1 {
			m.ActiveModal = ModalNone
			return m, nil
		}
		return m.submitCreatePRTaskModal()
	}

	return m, nil
}

func (m Model) initPRTaskIndividualAssignee() Model {
	candidates := m.getSquadIndividualCandidates()
	if len(candidates) > 0 && m.PRTaskAssignee == "" {
		m.PRTaskAssignee = candidates[0]
		m.PRTaskAssigneeIndex = 0
	}
	return m
}

func (m Model) cyclePRTaskIndividualAssignee(delta int) Model {
	candidates := m.getSquadIndividualCandidates()
	if len(candidates) == 0 {
		return m
	}
	curIdx := 0
	for i, c := range candidates {
		if c == m.PRTaskAssignee {
			curIdx = i
			break
		}
	}
	newIdx := (curIdx + delta) % len(candidates)
	if newIdx < 0 {
		newIdx += len(candidates)
	}
	m.PRTaskAssignee = candidates[newIdx]
	m.PRTaskAssigneeIndex = newIdx
	return m
}

func (m Model) submitCreatePRTaskModal() (tea.Model, tea.Cmd) {
	pr := m.PRTaskTarget

	var assignees []string
	switch m.PRTaskAssignMode {
	case 0: // Entire Squad (excl. me)
		assignees = m.getSquadReviewCandidates(pr.Author)
		// If squad has no other members, assignees is empty slice, creating an unassigned squad task.
	case 1: // Individual
		if m.PRTaskAssignee == "" {
			candidates := m.getSquadIndividualCandidates()
			if len(candidates) > 0 {
				m.PRTaskAssignee = candidates[0]
			}
		}
		if m.PRTaskAssignee != "" {
			assignees = []string{m.PRTaskAssignee}
		}
	case 2: // Myself
		if m.AuthUser != "" {
			assignees = []string{m.AuthUser}
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
