package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"grubby/internal/github"
)

func (m Model) handleSecurityKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	sortedRepos := m.getSecuritySortedRepos()
	switch msg.String() {
	case "up", "k":
		if m.RepoCursor > 0 {
			m.RepoCursor--
		}
	case "down", "j":
		if m.RepoCursor < len(sortedRepos)-1 {
			m.RepoCursor++
		}
	case "o", "enter":
		if len(sortedRepos) > 0 && m.RepoCursor < len(sortedRepos) {
			repo := sortedRepos[m.RepoCursor]
			url := fmt.Sprintf("https://github.com/%s/security/dependabot", repo.FullName)
			_ = OpenBrowser(url)
			SendNativeNotification("Grubby • Browser", "Dependabot Alerts", repo.FullName)
			return m, func() tea.Msg { return StatusToastMsg("Opened Dependabot alerts for " + repo.Name) }
		}
	}
	return m, nil
}

func (m Model) getSecuritySortedRepos() []github.Repository {
	sorted := make([]github.Repository, len(m.Repos))
	copy(sorted, m.Repos)

	sort.Slice(sorted, func(i, j int) bool {
		sumI := m.SecuritySummaries[sorted[i].Name]
		sumJ := m.SecuritySummaries[sorted[j].Name]

		if sumI.Critical != sumJ.Critical {
			return sumI.Critical > sumJ.Critical
		}
		if sumI.High != sumJ.High {
			return sumI.High > sumJ.High
		}
		if sumI.Total != sumJ.Total {
			return sumI.Total > sumJ.Total
		}
		return sorted[i].Name < sorted[j].Name
	})
	return sorted
}

func (m Model) renderSecurityView() string {
	if m.Loading {
		return "\n  Fetching security alerts from GitHub...\n"
	}
	if len(m.Repos) == 0 {
		return "\n  No repositories loaded.\n"
	}

	cols := computeSecurityColumns(m.Width)

	headerRow := lipgloss.JoinHorizontal(lipgloss.Top,
		cols.Cursor.Render(""),
		cols.Repo.Render("REPOSITORY"),
		cols.Crit.Render("CRITICAL"),
		cols.High.Render("HIGH"),
		cols.Med.Render("MEDIUM"),
		cols.Low.Render("LOW"),
		cols.Total.Render("TOTAL"),
		cols.Status.Render("PACKAGES / STATUS"),
	)
	renderedHeader := TableHeaderStyle.Render(headerRow)

	var rows []string
	rows = append(rows, renderedHeader)

	sortedRepos := m.getSecuritySortedRepos()

	visibleCount := m.Height - 8
	if visibleCount < 5 {
		visibleCount = 10
	}

	start := 0
	if m.RepoCursor >= visibleCount {
		start = m.RepoCursor - visibleCount + 1
	}
	end := start + visibleCount
	if end > len(sortedRepos) {
		end = len(sortedRepos)
	}

	for i := start; i < end; i++ {
		r := sortedRepos[i]
		sum := m.SecuritySummaries[r.Name]

		prefix := "  "
		if i == m.RepoCursor {
			prefix = "▶ "
		}

		critText := "0"
		if sum.Critical > 0 {
			critText = lipgloss.NewStyle().Foreground(DangerColor).Bold(true).Render(fmt.Sprintf("%d [CRIT]", sum.Critical))
		}

		highText := "0"
		if sum.High > 0 {
			highText = lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render(fmt.Sprintf("%d [HIGH]", sum.High))
		}

		medText := fmt.Sprintf("%d", sum.Medium)
		if sum.Medium > 0 {
			medText = lipgloss.NewStyle().Foreground(WarningColor).Render(medText)
		}

		lowText := fmt.Sprintf("%d", sum.Low)

		totalText := fmt.Sprintf("%d", sum.Total)
		if sum.Total == 0 {
			totalText = lipgloss.NewStyle().Foreground(SuccessColor).Render("0 [✓ clean]")
		} else {
			totalText = lipgloss.NewStyle().Foreground(DangerColor).Bold(true).Render(totalText)
		}

		statusText := "No alerts"
		if len(sum.Packages) > 0 {
			statusText = strings.Join(sum.Packages, ", ")
			if len(statusText) > cols.StatusWidth-2 && cols.StatusWidth > 3 {
				statusText = statusText[:cols.StatusWidth-3] + "…"
			}
		}

		repoName := r.Name
		if len(repoName) > cols.RepoWidth-2 && cols.RepoWidth > 3 {
			repoName = repoName[:cols.RepoWidth-3] + "…"
		}

		rowCells := lipgloss.JoinHorizontal(lipgloss.Top,
			cols.Cursor.Render(prefix),
			cols.Repo.Render(repoName),
			cols.Crit.Render(critText),
			cols.High.Render(highText),
			cols.Med.Render(medText),
			cols.Low.Render(lowText),
			cols.Total.Render(totalText),
			cols.Status.Render(statusText),
		)

		if i == m.RepoCursor {
			rows = append(rows, SelectedRowStyle.Render(rowCells))
		} else {
			rows = append(rows, NormalRowStyle.Render(rowCells))
		}
	}

	return strings.Join(rows, "\n")
}
