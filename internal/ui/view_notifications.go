package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) handleNotificationsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.NotifCursor > 0 {
			m.NotifCursor--
		}
	case "down", "j":
		if m.NotifCursor < len(m.Notifications)-1 {
			m.NotifCursor++
		}
	case "enter", "o":
		// Open notification in dedicated browser and mark as read
		if len(m.Notifications) > 0 && m.NotifCursor < len(m.Notifications) {
			n := m.Notifications[m.NotifCursor]
			_ = OpenBrowser(n.URL)

			// Mark as read locally
			m.Notifications[m.NotifCursor].Unread = false

			// Mark as read on GitHub
			threadID := n.ID
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = m.GHClient.MarkNotificationRead(ctx, threadID)
				return StatusToastMsg("Opened and marked as read: " + n.Title)
			}
		}
	case "m":
		// Mark current notification as read
		if len(m.Notifications) > 0 && m.NotifCursor < len(m.Notifications) {
			n := m.Notifications[m.NotifCursor]
			if !n.Unread {
				return m, func() tea.Msg { return StatusToastMsg("Notification already marked as read") }
			}
			m.Notifications[m.NotifCursor].Unread = false
			threadID := n.ID
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				err := m.GHClient.MarkNotificationRead(ctx, threadID)
				if err != nil {
					return StatusToastMsg("Failed to mark as read: " + err.Error())
				}
				return StatusToastMsg("Marked as read: " + n.Title)
			}
		}
	case "M":
		// Mark all notifications as read
		if len(m.Notifications) > 0 {
			for i := range m.Notifications {
				m.Notifications[i].Unread = false
			}
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				err := m.GHClient.MarkAllNotificationsRead(ctx)
				if err != nil {
					return StatusToastMsg("Failed to mark all as read: " + err.Error())
				}
				return StatusToastMsg("Marked all notifications as read")
			}
		}
	}
	return m, nil
}

func (m Model) renderNotificationsView() string {
	if m.Loading && len(m.Notifications) == 0 {
		return "\n  Fetching notifications from GitHub...\n"
	}
	if len(m.Notifications) == 0 {
		return "\n  No notifications found.\n  Inbox is clear! ✓\n"
	}

	cols := computeNotificationColumns(m.Width)

	unreadCount := 0
	for _, n := range m.Notifications {
		if n.Unread {
			unreadCount++
		}
	}

	unreadBadge := lipgloss.NewStyle().Foreground(SuccessColor).Render("0 unread")
	if unreadCount > 0 {
		unreadBadge = lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render(fmt.Sprintf("%d unread", unreadCount))
	}

	banner := MetaBarStyle.Render(fmt.Sprintf("  Inbox: %s  |  Total: %d notifications  |  [Enter/o] Open  [m] Mark Read  [M] Mark All Read", unreadBadge, len(m.Notifications))) + "\n"

	headerRow := lipgloss.JoinHorizontal(lipgloss.Top,
		cols.Cursor.Render(""),
		cols.Status.Render("STATUS"),
		cols.Type.Render("TYPE"),
		cols.Reason.Render("REASON"),
		cols.Repo.Render("REPOSITORY"),
		cols.Title.Render("TITLE"),
		cols.Time.Render("UPDATED"),
	)
	renderedHeader := TableHeaderStyle.Render(headerRow)

	var rows []string
	rows = append(rows, banner, renderedHeader)

	visibleCount := m.Height - 10
	if visibleCount < 5 {
		visibleCount = 10
	}

	start := 0
	if m.NotifCursor >= visibleCount {
		start = m.NotifCursor - visibleCount + 1
	}
	end := start + visibleCount
	if end > len(m.Notifications) {
		end = len(m.Notifications)
	}

	for i := start; i < end; i++ {
		n := m.Notifications[i]

		prefix := "  "
		if i == m.NotifCursor {
			prefix = "▶ "
		}

		var statusText string
		if n.Unread {
			statusText = lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render("● NEW")
		} else {
			statusText = lipgloss.NewStyle().Foreground(MutedColor).Render("○ read")
		}

		typeText := n.Type
		if len(typeText) > 11 {
			typeText = typeText[:10] + "…"
		}

		reasonText := n.Reason
		if len(reasonText) > 12 {
			reasonText = reasonText[:11] + "…"
		}

		repoText := n.Repository
		if len(repoText) > cols.RepoWidth-2 && cols.RepoWidth > 3 {
			repoText = repoText[:cols.RepoWidth-3] + "…"
		}

		titleText := n.Title
		if len(titleText) > cols.TitleWidth-2 && cols.TitleWidth > 3 {
			titleText = titleText[:cols.TitleWidth-3] + "…"
		}

		timeText := formatTimeAgo(n.UpdatedAt)

		rowCells := lipgloss.JoinHorizontal(lipgloss.Top,
			cols.Cursor.Render(prefix),
			cols.Status.Render(statusText),
			cols.Type.Render(typeText),
			cols.Reason.Render(reasonText),
			cols.Repo.Render(repoText),
			cols.Title.Render(titleText),
			cols.Time.Render(timeText),
		)

		if i == m.NotifCursor {
			rows = append(rows, SelectedRowStyle.Render(rowCells))
		} else {
			rows = append(rows, NormalRowStyle.Render(rowCells))
		}
	}

	return strings.Join(rows, "\n")
}
