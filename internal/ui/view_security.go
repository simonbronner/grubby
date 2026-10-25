package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleSecurityKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.RepoCursor > 0 {
			m.RepoCursor--
		}
	case "down", "j":
		if m.RepoCursor < len(m.Repos)-1 {
			m.RepoCursor++
		}
	case "o":
		if len(m.Repos) > 0 && m.RepoCursor < len(m.Repos) {
			repo := m.Repos[m.RepoCursor]
			url := fmt.Sprintf("https://github.com/%s/security", repo.FullName)
			_ = OpenBrowser(url)
			return m, func() tea.Msg { return StatusToastMsg("Opened security alerts for " + repo.Name) }
		}
	}
	return m, nil
}

func (m Model) renderSecurityView() string {
	header := fmt.Sprintf("  %-25s %-12s %-10s %-10s %-10s %-10s %s",
		"REPOSITORY", "CRITICAL", "HIGH", "MEDIUM", "LOW", "TOTAL", "STATUS")
	renderedHeader := TableHeaderStyle.Render(header)

	var rows []string
	rows = append(rows, renderedHeader)

	if len(m.Repos) == 0 {
		return "\n  No repositories loaded.\n"
	}

	visibleCount := m.Height - 8
	if visibleCount < 5 {
		visibleCount = 10
	}

	start := 0
	if m.RepoCursor >= visibleCount {
		start = m.RepoCursor - visibleCount + 1
	}
	end := start + visibleCount
	if end > len(m.Repos) {
		end = len(m.Repos)
	}

	for i := start; i < end; i++ {
		r := m.Repos[i]

		prefix := "  "
		if i == m.RepoCursor {
			prefix = "▶ "
		}

		// Security status placeholder / summary
		crit := "-"
		high := "-"
		med := "-"
		low := "-"
		total := "-"
		status := "Ready (press 'o' for web)"

		rowContent := fmt.Sprintf("%s%-23s %-12s %-10s %-10s %-10s %-10s %s",
			prefix, r.Name, crit, high, med, low, total, status)

		if i == m.RepoCursor {
			rows = append(rows, SelectedRowStyle.Render(rowContent))
		} else {
			rows = append(rows, NormalRowStyle.Render(rowContent))
		}
	}

	return strings.Join(rows, "\n")
}
