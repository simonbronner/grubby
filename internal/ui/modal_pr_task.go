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
	title := ModalTitleStyle.Render(fmt.Sprintf("Request Review: PR #%d", pr.Number))

	// PR Summary
	prTitleDisplay := pr.Title
	if len(prTitleDisplay) > 55 {
		prTitleDisplay = prTitleDisplay[:52] + "…"
	}
	prInfo := lipgloss.NewStyle().Foreground(HighlightColor).Bold(true).Render(fmt.Sprintf("⑂ %s (%s)", prTitleDisplay, pr.Repo))
	authorInfo := lipgloss.NewStyle().Foreground(SecondaryColor).Render(fmt.Sprintf("Author: @%s", pr.Author))
	summaryLine := fmt.Sprintf("  %s  •  %s", prInfo, authorInfo)

	// Reviewer Selection Header
	squadDisplayName := "Squad"
	if m.Config != nil && m.Config.GetActiveSquad() != nil {
		squadDisplayName = m.Config.GetActiveSquad().DisplayName
	}

	headerText := "Select Reviewer (Squad or Individual):"
	if m.PRTaskFocusedField == 0 {
		headerText = lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render("▶ " + headerText)
	} else {
		headerText = "  " + lipgloss.NewStyle().Bold(true).Render(headerText)
	}

	candidates := m.PRTaskCandidates
	if len(candidates) == 0 {
		candidates = m.getSquadMembersForReview(pr.Author)
	}

	totalOptions := 1 + len(candidates) // Option 0 = Squad, Options 1..N = Candidates
	cursor := m.PRTaskMemberCursor
	if cursor >= totalOptions {
		cursor = totalOptions - 1
	}
	if cursor < 0 {
		cursor = 0
	}

	var optionLines []string

	// Option 0: Whole Squad (Unassigned)
	isSquadCurrent := (m.PRTaskFocusedField == 0 && cursor == 0)
	squadPrefix := "    "
	if isSquadCurrent {
		squadPrefix = "  ▶ "
	}
	squadLabel := fmt.Sprintf("%s[0/s] 👥 %s (Unassigned Review)", squadPrefix, squadDisplayName)
	if isSquadCurrent {
		optionLines = append(optionLines, SelectedRowStyle.Render(squadLabel))
	} else {
		optionLines = append(optionLines, NormalRowStyle.Render(squadLabel))
	}

	// Options 1..N: Individual squad members
	maxVisible := 5
	memberCursor := cursor - 1 // -1 when on squad
	start := 0
	if memberCursor >= maxVisible {
		start = memberCursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(candidates) {
		end = len(candidates)
	}

	for i := start; i < end; i++ {
		cand := candidates[i]
		candIdx := i + 1
		isCandCurrent := (m.PRTaskFocusedField == 0 && cursor == candIdx)
		candPrefix := "    "
		if isCandCurrent {
			candPrefix = "  ▶ "
		}

		keyHint := fmt.Sprintf("[%d]", candIdx)
		if candIdx > 9 {
			keyHint = "   "
		}

		label := "@" + cand
		if strings.EqualFold(cand, m.AuthUser) {
			label += " (me)"
		}
		if strings.EqualFold(cand, pr.Author) {
			label += " (author)"
		}

		rowText := fmt.Sprintf("%s%-5s %s", candPrefix, keyHint, label)
		if isCandCurrent {
			optionLines = append(optionLines, SelectedRowStyle.Render(rowText))
		} else {
			optionLines = append(optionLines, NormalRowStyle.Render(rowText))
		}
	}

	if len(candidates) > maxVisible {
		scrollHint := lipgloss.NewStyle().Foreground(MutedColor).Render(
			fmt.Sprintf("    (Showing %d-%d of %d members - use ↑/↓ to scroll)", start+1, end, len(candidates)),
		)
		optionLines = append(optionLines, scrollHint)
	}

	// Priority Field
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
	prioLine := fmt.Sprintf("Priority:      %s   %s   %s   (press 'p' to change)", prioHigh, prioMed, prioLow)
	if m.PRTaskFocusedField == 1 {
		prioLine = lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render("▶ " + prioLine)
	} else {
		prioLine = "  " + prioLine
	}

	// Backing Repo Info
	repoInfo := m.getTaskTargetRepoFullName(TaskTargetSquad)
	repoLine := lipgloss.NewStyle().Foreground(lipgloss.Color("#7F8497")).Render(
		fmt.Sprintf("  Backing Repo: %s  •  Task Board: Squad Tasks  •  Label: pr-review", repoInfo),
	)

	// Buttons
	var btnCreate, btnCancel string
	if m.PRTaskFocusedField == 2 {
		if m.PRTaskActiveButton == 0 {
			btnCreate = lipgloss.NewStyle().Bold(true).Background(PrimaryColor).Foreground(lipgloss.Color("#FFFFFF")).Render(" [ Request Review ] ")
			btnCancel = lipgloss.NewStyle().Foreground(MutedColor).Render(" [ Cancel ] ")
		} else {
			btnCreate = lipgloss.NewStyle().Foreground(MutedColor).Render(" [ Request Review ] ")
			btnCancel = lipgloss.NewStyle().Bold(true).Background(DangerColor).Foreground(lipgloss.Color("#FFFFFF")).Render(" [ Cancel ] ")
		}
	} else {
		btnCreate = lipgloss.NewStyle().Foreground(PrimaryColor).Render(" [ Request Review ] ")
		btnCancel = lipgloss.NewStyle().Foreground(MutedColor).Render(" [ Cancel ] ")
	}
	buttonLine := fmt.Sprintf("               %s   %s", btnCreate, btnCancel)

	// Help line
	helpLine := MetaBarStyle.Render("  [Enter] Confirm  •  [0-9/s] Quick Pick  •  [p] Priority  •  [Tab] Buttons  •  [Esc] Cancel")

	var contentLines []string
	contentLines = append(contentLines, title, "", summaryLine, "", headerText)
	contentLines = append(contentLines, optionLines...)
	contentLines = append(contentLines, "", prioLine, repoLine, "", buttonLine, "", helpLine)

	return ModalBoxStyle.Render(strings.Join(contentLines, "\n"))
}

func (m Model) handleCreatePRTaskModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	candidates := m.PRTaskCandidates
	if len(candidates) == 0 {
		candidates = m.getSquadMembersForReview(m.PRTaskTarget.Author)
		m.PRTaskCandidates = candidates
	}
	totalOptions := 1 + len(candidates)

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
			if m.PRTaskMemberCursor < totalOptions-1 {
				m.PRTaskMemberCursor++
			}
			return m, nil
		}
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

	case "left", "h":
		if m.PRTaskFocusedField == 2 {
			m.PRTaskActiveButton = 0
			return m, nil
		}

	case "right", "l":
		if m.PRTaskFocusedField == 2 {
			m.PRTaskActiveButton = 1
			return m, nil
		}

	case "0", "s", "S":
		m.PRTaskMemberCursor = 0
		return m.submitCreatePRTaskModal()

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(msg.String()[0] - '0')
		if idx <= len(candidates) {
			m.PRTaskMemberCursor = idx
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

func (m Model) submitCreatePRTaskModal() (tea.Model, tea.Cmd) {
	pr := m.PRTaskTarget

	var assignees []string
	var toastDesc string
	candidates := m.PRTaskCandidates
	if len(candidates) == 0 {
		candidates = m.getSquadMembersForReview(pr.Author)
	}

	activeSquadName := ""
	activeSquadDisplay := "Squad"
	if m.Config != nil && m.Config.GetActiveSquad() != nil {
		activeSquadName = m.Config.GetActiveSquad().Name
		activeSquadDisplay = m.Config.GetActiveSquad().DisplayName
	}

	if m.PRTaskMemberCursor == 0 {
		// Option 0: Squad review (unassigned)
		assignees = []string{}
		toastDesc = fmt.Sprintf("Requested %s review (unassigned) for PR #%d", activeSquadDisplay, pr.Number)
	} else if m.PRTaskMemberCursor <= len(candidates) {
		// Option 1..N: Individual person in the squad
		chosen := candidates[m.PRTaskMemberCursor-1]
		assignees = []string{chosen}
		toastDesc = fmt.Sprintf("Requested review from @%s for PR #%d", chosen, pr.Number)
	} else {
		assignees = []string{}
		toastDesc = fmt.Sprintf("Requested review for PR #%d", pr.Number)
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
	if activeSquadName != "" {
		labels = append(labels, "squad:"+strings.ToLower(strings.TrimSpace(activeSquadName)))
	}

	repoFullName := m.getTaskTargetRepoFullName(TaskTargetSquad)
	parts := strings.SplitN(repoFullName, "/", 2)
	owner := parts[0]
	repo := parts[1]

	now := time.Now()
	tempTask := github.Task{
		ID:          -now.UnixNano(),
		Number:      0,
		Title:       title,
		Body:        body,
		State:       "open",
		Priority:    priority,
		RepoOwner:   owner,
		RepoName:    repo,
		Author:      m.AuthUser,
		Assignees:   assignees,
		Labels:      labels,
		CreatedAt:   now,
		UpdatedAt:   now,
		IsSquadTask: true,
	}
	m.SquadTasks = append([]github.Task{tempTask}, m.SquadTasks...)
	if len(assignees) == 1 && strings.EqualFold(assignees[0], m.AuthUser) {
		m.PersonalTasks = append([]github.Task{tempTask}, m.PersonalTasks...)
	}
	if m.DB != nil {
		_ = m.DB.SaveTask(&tempTask, "pending_create")
	}

	m.ActiveModal = ModalNone
	m.TaskCursor = 0
	m.StatusMsg = fmt.Sprintf("✓ %s", toastDesc)
	m.StatusMsgTimer = time.Now().Add(4 * time.Second)

	return m, m.createPRTaskCmd(owner, repo, title, body, priority, labels, assignees, true, pr)
}
