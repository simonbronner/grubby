package ui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	prColCursor   = lipgloss.NewStyle().Width(3)
	prColNum      = lipgloss.NewStyle().Width(7)
	prColRepo     = lipgloss.NewStyle().Width(18)
	prColTitle    = lipgloss.NewStyle().Width(32)
	prColAuthor   = lipgloss.NewStyle().Width(12)
	prColAge      = lipgloss.NewStyle().Width(12)
	prColReview   = lipgloss.NewStyle().Width(16)
	prColComments = lipgloss.NewStyle().Width(12)
	prColLabels   = lipgloss.NewStyle().Width(20)
)

func (m Model) handlePRsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.PRCursor > 0 {
			m.PRCursor--
		}
	case "down", "j":
		if m.PRCursor < len(m.PRs)-1 {
			m.PRCursor++
		}
	case "o":
		if len(m.PRs) > 0 && m.PRCursor < len(m.PRs) {
			pr := m.PRs[m.PRCursor]
			_ = OpenBrowser(pr.URL)
			return m, func() tea.Msg { return StatusToastMsg(fmt.Sprintf("Opened PR #%d in browser", pr.Number)) }
		}
	case "p":
		// Trigger Copilot review
		if len(m.PRs) > 0 && m.PRCursor < len(m.PRs) {
			pr := m.PRs[m.PRCursor]
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				err := m.GHClient.TriggerCopilotReview(ctx, m.CurrentOrg, pr.Repo, pr.Number)
				if err != nil {
					return StatusToastMsg(fmt.Sprintf("Failed to request Copilot review: %v", err))
				}
				return StatusToastMsg(fmt.Sprintf("Requested Copilot review for #%d in %s", pr.Number, pr.Repo))
			}
		}
	case "c":
		// Checkout PR branch locally
		if len(m.PRs) > 0 && m.PRCursor < len(m.PRs) {
			pr := m.PRs[m.PRCursor]
			st := m.GitStatuses[pr.Repo]
			if !st.IsCloned {
				return m, func() tea.Msg { return StatusToastMsg("Repo " + pr.Repo + " is not cloned yet!") }
			}
			return m, func() tea.Msg {
				cmd := exec.Command("gh", "pr", "checkout", fmt.Sprintf("%d", pr.Number))
				cmd.Dir = st.LocalPath
				err := cmd.Run()
				if err != nil {
					return StatusToastMsg(fmt.Sprintf("Failed to checkout PR #%d: %v", pr.Number, err))
				}
				return StatusToastMsg(fmt.Sprintf("Checked out PR #%d in %s", pr.Number, st.LocalPath))
			}
		}
	}
	return m, nil
}

func (m Model) renderPRsView() string {
	if m.Loading {
		return "\n  Fetching pull requests from GitHub...\n"
	}
	if len(m.PRs) == 0 {
		return "\n  No open pull requests found.\n"
	}

	headerRow := lipgloss.JoinHorizontal(lipgloss.Top,
		prColCursor.Render(""),
		prColNum.Render("PR"),
		prColRepo.Render("REPOSITORY"),
		prColTitle.Render("TITLE"),
		prColAuthor.Render("AUTHOR"),
		prColAge.Render("AGE"),
		prColReview.Render("REVIEW"),
		prColComments.Render("COMMENTS"),
		prColLabels.Render("LABELS"),
	)
	renderedHeader := TableHeaderStyle.Render(headerRow)

	var rows []string
	rows = append(rows, renderedHeader)

	visibleCount := m.Height - 8
	if visibleCount < 5 {
		visibleCount = 10
	}

	start := 0
	if m.PRCursor >= visibleCount {
		start = m.PRCursor - visibleCount + 1
	}
	end := start + visibleCount
	if end > len(m.PRs) {
		end = len(m.PRs)
	}

	for i := start; i < end; i++ {
		pr := m.PRs[i]

		title := pr.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}

		ageText := formatTimeAgo(pr.CreatedAt)
		if pr.DaysStale() >= 3 {
			ageText = StaleBadge.Render(fmt.Sprintf("%dd [!]", pr.DaysStale()))
		}

		// Review Decision Badge
		reviewBadge := pr.ReviewDecision
		switch pr.ReviewDecision {
		case "APPROVED":
			reviewBadge = ApprovedBadge.Render("APPROVED")
		case "CHANGES_REQUESTED":
			reviewBadge = ChangesReqBadge.Render("CHANGES_REQ")
		case "REVIEW_REQUIRED":
			reviewBadge = ReviewReqBadge.Render("REVIEW_REQ")
		default:
			reviewBadge = "-"
		}

		// Comments & Unresolved
		commentsText := fmt.Sprintf("💬 %d/%d", pr.UnresolvedComments, pr.TotalComments)
		if pr.UnresolvedComments > 0 {
			commentsText = lipgloss.NewStyle().Foreground(WarningColor).Render(commentsText)
		}

		labelsText := strings.Join(pr.Labels, ", ")
		if len(labelsText) > 18 {
			labelsText = labelsText[:15] + "..."
		}

		prefix := "  "
		if i == m.PRCursor {
			prefix = "▶ "
		}

		numText := fmt.Sprintf("#%-5d", pr.Number)

		rowCells := lipgloss.JoinHorizontal(lipgloss.Top,
			prColCursor.Render(prefix),
			prColNum.Render(numText),
			prColRepo.Render(pr.Repo),
			prColTitle.Render(title),
			prColAuthor.Render(pr.Author),
			prColAge.Render(ageText),
			prColReview.Render(reviewBadge),
			prColComments.Render(commentsText),
			prColLabels.Render(labelsText),
		)

		if i == m.PRCursor {
			rows = append(rows, SelectedRowStyle.Render(rowCells))
		} else {
			rows = append(rows, NormalRowStyle.Render(rowCells))
		}
	}

	return strings.Join(rows, "\n")
}
