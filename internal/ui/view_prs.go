package ui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"grubby/internal/github"
)

func (m Model) getDisplayPRs() []github.PullRequest {
	if m.PRFilter == "" {
		return m.PRs
	}
	f := strings.ToLower(m.PRFilter)
	var result []github.PullRequest
	for _, pr := range m.PRs {
		if strings.Contains(strings.ToLower(pr.Title), f) ||
			strings.Contains(strings.ToLower(pr.Repo), f) ||
			strings.Contains(strings.ToLower(pr.Author), f) {
			result = append(result, pr)
		}
	}
	return result
}

func (m Model) handlePRsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	prs := m.getDisplayPRs()
	visibleCount := m.Height - 8
	if visibleCount < 3 {
		visibleCount = 3
	}

	switch msg.String() {
	case "up", "k":
		if m.PRCursor > 0 {
			m.PRCursor--
		}
	case "down", "j":
		if m.PRCursor < len(prs)-1 {
			m.PRCursor++
		}
	case "pgdown", "ctrl+d":
		m.PRCursor += visibleCount
		if m.PRCursor >= len(prs) {
			m.PRCursor = len(prs) - 1
		}
	case "pgup", "ctrl+u":
		m.PRCursor -= visibleCount
		if m.PRCursor < 0 {
			m.PRCursor = 0
		}
	case "home", "g":
		m.PRCursor = 0
	case "end", "G":
		if len(prs) > 0 {
			m.PRCursor = len(prs) - 1
		}
	case "f":
		m.ActiveModal = ModalFilterPR
		m.PRFilterInput.SetValue(m.PRFilter)
		m.PRFilterInput.Focus()
		return m, nil
	case "esc":
		if m.PRFilter != "" {
			m.PRFilter = ""
			m.PRCursor = 0
			return m, func() tea.Msg { return StatusToastMsg("Cleared pull request filter") }
		}
	case "o":
		if len(prs) > 0 && m.PRCursor < len(prs) {
			pr := prs[m.PRCursor]
			_ = OpenBrowser(pr.URL)
			return m, func() tea.Msg { return StatusToastMsg(fmt.Sprintf("Opened PR #%d in browser", pr.Number)) }
		}
	case "p":
		// Trigger Copilot review
		if len(prs) > 0 && m.PRCursor < len(prs) {
			pr := prs[m.PRCursor]
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
		if len(prs) > 0 && m.PRCursor < len(prs) {
			pr := prs[m.PRCursor]
			m = m.recordRepoInteraction(pr.Repo)
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

	displayPRs := m.getDisplayPRs()

	if len(m.PRs) == 0 {
		return "\n  No open pull requests found for " + m.CurrentOrg + ".\n"
	}

	if len(displayPRs) == 0 {
		return fmt.Sprintf("\n  No open pull requests match filter %q.\n  Press [Esc] to clear filter.\n", m.PRFilter)
	}

	if m.PRCursor >= len(displayPRs) {
		m.PRCursor = len(displayPRs) - 1
	}

	cols := computePRColumns(m.Width)

	headerRow := lipgloss.JoinHorizontal(lipgloss.Top,
		cols.Cursor.Render(""),
		cols.Num.Render("PR"),
		cols.Repo.Render("REPOSITORY"),
		cols.Title.Render("TITLE"),
		cols.Author.Render("AUTHOR"),
		cols.Age.Render("AGE"),
		cols.Review.Render("REVIEW"),
		cols.Comments.Render("COMMENTS"),
		cols.Labels.Render("LABELS"),
	)
	renderedHeader := TableHeaderStyle.Render(headerRow)

	var rows []string

	// Banner summary & filter indicator
	var bannerText string
	if m.PRFilter != "" {
		bannerText = MetaBarStyle.Render(fmt.Sprintf("  ⑂ %d Open Pull Requests  ", len(m.PRs))) +
			lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(fmt.Sprintf("🔍 Filter: %q (%d matched)", m.PRFilter, len(displayPRs))) +
			MetaBarStyle.Render(" • [Esc] Clear")
	} else {
		bannerText = MetaBarStyle.Render(fmt.Sprintf("  ⑂ %d Open Pull Requests across %s", len(m.PRs), m.CurrentOrg))
	}
	rows = append(rows, bannerText, renderedHeader)

	visibleCount := m.Height - 8
	if m.PRFilter != "" {
		visibleCount = m.Height - 9
	}
	if visibleCount < 3 {
		visibleCount = 3
	}
	if visibleCount > m.Height-6 {
		visibleCount = m.Height - 6
	}

	start := 0
	if m.PRCursor >= visibleCount {
		start = m.PRCursor - visibleCount + 1
	}
	end := start + visibleCount
	if end > len(displayPRs) {
		end = len(displayPRs)
	}

	for i := start; i < end; i++ {
		pr := displayPRs[i]

		repoText := pr.Repo
		if repoText == "" {
			repoText = "-"
		} else if len(repoText) > cols.RepoWidth-2 && cols.RepoWidth > 3 {
			repoText = repoText[:cols.RepoWidth-3] + "…"
		}

		title := strings.ReplaceAll(strings.ReplaceAll(pr.Title, "\r", " "), "\n", " ")
		if title == "" {
			title = "(no title)"
		} else if len(title) > cols.TitleWidth-2 && cols.TitleWidth > 3 {
			title = title[:cols.TitleWidth-3] + "…"
		}

		authorText := pr.Author
		if authorText == "" {
			authorText = "-"
		} else if len(authorText) > cols.AuthorWidth-1 && cols.AuthorWidth > 2 {
			authorText = authorText[:cols.AuthorWidth-2] + "…"
		}

		// Calculate raw unstyled age string first
		var rawAge string
		if pr.CreatedAt.IsZero() {
			rawAge = "-"
		} else if pr.DaysStale() >= 3 {
			rawAge = fmt.Sprintf("%dd [!]", pr.DaysStale())
		} else {
			rawAge = formatTimeAgo(pr.CreatedAt)
		}
		if rawAge == "" {
			rawAge = "-"
		}

		// Truncate plain text string BEFORE applying ANSI styling
		if len(rawAge) > cols.AgeWidth && cols.AgeWidth > 2 {
			rawAge = rawAge[:cols.AgeWidth-1]
		}

		var ageText string
		if pr.DaysStale() >= 3 && !pr.CreatedAt.IsZero() {
			ageText = StaleBadge.Render(rawAge)
		} else {
			ageText = rawAge
		}

		// Review Decision Badge
		var reviewBadge string
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
		if reviewBadge == "" {
			reviewBadge = "-"
		}

		// Comments & Unresolved
		commentsText := fmt.Sprintf("💬 %d/%d", pr.UnresolvedComments, pr.TotalComments)
		if pr.UnresolvedComments > 0 {
			commentsText = lipgloss.NewStyle().Foreground(WarningColor).Render(commentsText)
		}

		labelsText := "-"
		if len(pr.Labels) > 0 {
			labelsText = strings.Join(pr.Labels, ", ")
		}
		if len(labelsText) > cols.LabelsWidth-2 && cols.LabelsWidth > 3 {
			labelsText = labelsText[:cols.LabelsWidth-3] + "…"
		}

		prefix := "  "
		if i == m.PRCursor {
			prefix = "▶ "
		}

		numText := fmt.Sprintf("#%-5d", pr.Number)

		rowCells := lipgloss.JoinHorizontal(lipgloss.Top,
			cols.Cursor.Render(prefix),
			cols.Num.Render(numText),
			cols.Repo.Render(repoText),
			cols.Title.Render(title),
			cols.Author.Render(authorText),
			cols.Age.Render(ageText),
			cols.Review.Render(reviewBadge),
			cols.Comments.Render(commentsText),
			cols.Labels.Render(labelsText),
		)

		if i == m.PRCursor {
			rows = append(rows, SelectedRowStyle.MaxHeight(1).Render(rowCells))
		} else {
			rows = append(rows, NormalRowStyle.MaxHeight(1).Render(rowCells))
		}
	}

	return strings.Join(rows, "\n")
}
