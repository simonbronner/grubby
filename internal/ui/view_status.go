package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"grubby/internal/github"
)

func (m Model) getDisplayComponents() []github.StatusComponent {
	if m.GitHubStatus == nil {
		return nil
	}
	var res []github.StatusComponent
	for _, c := range m.GitHubStatus.Components {
		// Filter out purely decorative or web link components
		if c.Group || c.Name == "Visit www.githubstatus.com for more information" {
			continue
		}
		res = append(res, c)
	}
	return res
}

func (m Model) handleStatusKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	components := m.getDisplayComponents()
	visibleCount := m.Height - 12
	if visibleCount < 4 {
		visibleCount = 4
	}

	switch msg.String() {
	case "up", "k":
		if m.StatusCursor > 0 {
			m.StatusCursor--
		}
	case "down", "j":
		if m.StatusCursor < len(components)-1 {
			m.StatusCursor++
		}
	case "pgup":
		m.StatusCursor -= visibleCount
		if m.StatusCursor < 0 {
			m.StatusCursor = 0
		}
	case "pgdown":
		m.StatusCursor += visibleCount
		if m.StatusCursor >= len(components) {
			m.StatusCursor = len(components) - 1
		}
	case "home", "g":
		m.StatusCursor = 0
	case "end", "G":
		if len(components) > 0 {
			m.StatusCursor = len(components) - 1
		}
	case "o", "enter":
		url := "https://www.githubstatus.com"
		if m.GitHubStatus != nil && len(m.GitHubStatus.Incidents) > 0 && m.GitHubStatus.Incidents[0].Shortlink != "" {
			url = m.GitHubStatus.Incidents[0].Shortlink
		}
		_ = OpenBrowser(url)
		return m, func() tea.Msg { return StatusToastMsg("Opened githubstatus.com in browser") }
	case "r":
		m.StatusMsg = "Checking GitHub status..."
		m.StatusMsgTimer = time.Now().Add(3 * time.Second)
		m.LastStatusCheck = time.Now()
		return m, m.fetchGitHubStatusCmd()
	}
	return m, nil
}

func (m Model) renderStatusView() string {
	if m.GitHubStatus == nil {
		return "\n  Checking GitHub System Status... (Press [r] to refresh, [o] to open statuspage)\n"
	}

	var sections []string

	// 1. Overall Status Card / Banner
	indicator := m.GitHubStatus.Status.Indicator
	description := m.GitHubStatus.Status.Description
	if description == "" {
		description = "All Systems Operational"
	}

	var statusBanner string
	timeSinceCheck := time.Since(m.LastStatusCheck)
	checkedAgo := fmt.Sprintf("%ds ago", int(timeSinceCheck.Seconds()))
	if timeSinceCheck >= time.Minute {
		checkedAgo = fmt.Sprintf("%dm ago", int(timeSinceCheck.Minutes()))
	}

	if indicator == "none" || indicator == "" {
		badge := lipgloss.NewStyle().
			Bold(true).
			Background(SuccessColor).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 2).
			Render("● ALL SYSTEMS OPERATIONAL")
		info := lipgloss.NewStyle().Foreground(MutedColor).Render(fmt.Sprintf("  Checked: %s • Polled every 60s • https://www.githubstatus.com", checkedAgo))
		statusBanner = "  " + badge + info
	} else {
		alertTitle := "SERVICE OUTAGE / DEGRADATION"
		if indicator == "minor" {
			alertTitle = "MINOR SERVICE OUTAGE"
		} else if indicator == "critical" || indicator == "major" {
			alertTitle = "MAJOR SERVICE OUTAGE"
		}
		badge := lipgloss.NewStyle().
			Bold(true).
			Background(DangerColor).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 2).
			Render("▲ " + alertTitle)
		desc := lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render("  " + description)
		info := lipgloss.NewStyle().Foreground(MutedColor).Render(fmt.Sprintf(" (Checked %s)", checkedAgo))
		statusBanner = "  " + badge + desc + info
	}
	sections = append(sections, statusBanner)

	// 2. Active Incidents (if any)
	if len(m.GitHubStatus.Incidents) > 0 {
		var incLines []string
		incHeader := lipgloss.NewStyle().Foreground(DangerColor).Bold(true).Render("  ⚡ Active Incidents:")
		incLines = append(incLines, incHeader)
		for _, inc := range m.GitHubStatus.Incidents {
			incStatus := strings.ToUpper(inc.Status)
			incImpact := strings.ToUpper(inc.Impact)
			title := fmt.Sprintf("    • [%s | %s] %s", incStatus, incImpact, inc.Name)
			incLines = append(incLines, lipgloss.NewStyle().Bold(true).Render(title))
			if len(inc.IncidentUpdates) > 0 {
				latest := inc.IncidentUpdates[0]
				updateTime := latest.CreatedAt.Format("15:04 MST")
				updateBody := strings.ReplaceAll(latest.Body, "\n", " ")
				if len(updateBody) > m.Width-15 && m.Width > 20 {
					updateBody = updateBody[:m.Width-18] + "…"
				}
				incLines = append(incLines, lipgloss.NewStyle().Foreground(MutedColor).Render(fmt.Sprintf("      Latest (%s): %s", updateTime, updateBody)))
			}
		}
		sections = append(sections, strings.Join(incLines, "\n"))
	}

	// 3. Components / Services Table
	components := m.getDisplayComponents()
	if len(components) == 0 {
		sections = append(sections, "  No service components found.")
		return strings.Join(sections, "\n\n")
	}

	// Compute columns
	// Cursor (3) + Status (20) + Updated (14) = 37 fixed
	nameWidth := 26
	descWidth := m.Width - nameWidth - 37
	if descWidth < 20 {
		descWidth = 20
	}

	nameStyle := lipgloss.NewStyle().Width(nameWidth).MaxWidth(nameWidth).MaxHeight(1).Inline(true)
	statusColStyle := lipgloss.NewStyle().Width(20).MaxWidth(20).MaxHeight(1).Inline(true)
	descStyle := lipgloss.NewStyle().Width(descWidth).MaxWidth(descWidth).MaxHeight(1).Inline(true)
	updatedStyle := lipgloss.NewStyle().Width(12).MaxWidth(12).MaxHeight(1).Inline(true)

	tableHeader := lipgloss.JoinHorizontal(
		lipgloss.Top,
		"   ",
		TableHeaderStyle.Render(nameStyle.Render("SERVICE COMPONENT")),
		TableHeaderStyle.Render(statusColStyle.Render("STATUS")),
		TableHeaderStyle.Render(descStyle.Render("DESCRIPTION")),
		TableHeaderStyle.Render(updatedStyle.Render("LAST UPDATED")),
	)
	sections = append(sections, tableHeader)

	// Scroll windowing
	visibleCount := m.Height - 12
	if len(m.GitHubStatus.Incidents) > 0 {
		visibleCount -= (len(m.GitHubStatus.Incidents)*2 + 1)
	}
	if visibleCount < 3 {
		visibleCount = 3
	}

	start := 0
	if m.StatusCursor >= visibleCount {
		start = m.StatusCursor - visibleCount + 1
	}
	end := start + visibleCount
	if end > len(components) {
		end = len(components)
	}

	var rows []string
	for i := start; i < end; i++ {
		comp := components[i]
		cursor := "   "
		if i == m.StatusCursor {
			cursor = " ▶ "
		}

		nameText := comp.Name
		if len(nameText) > nameWidth-2 {
			nameText = nameText[:nameWidth-3] + "…"
		}

		var statusBadge string
		switch comp.Status {
		case "operational":
			statusBadge = ApprovedBadge.Render("OPERATIONAL")
		case "degraded_performance":
			statusBadge = lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render("DEGRADED")
		case "partial_outage":
			statusBadge = lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render("PARTIAL OUTAGE")
		case "major_outage":
			statusBadge = DangerBadge.Render("MAJOR OUTAGE")
		default:
			statusBadge = comp.Status
		}

		descText := comp.Description
		if descText == "" {
			descText = "-"
		} else if len(descText) > descWidth-2 {
			descText = descText[:descWidth-3] + "…"
		}

		updatedText := "-"
		if !comp.UpdatedAt.IsZero() {
			updatedText = formatTimeAgo(comp.UpdatedAt)
		}

		rowContent := lipgloss.JoinHorizontal(
			lipgloss.Top,
			cursor,
			nameStyle.Render(nameText),
			statusColStyle.Render(statusBadge),
			descStyle.Render(descText),
			updatedStyle.Render(updatedText),
		)

		if i == m.StatusCursor {
			rows = append(rows, SelectedRowStyle.Render(rowContent))
		} else {
			rows = append(rows, NormalRowStyle.Render(rowContent))
		}
	}

	sections = append(sections, strings.Join(rows, "\n"))
	return strings.Join(sections, "\n\n")
}
