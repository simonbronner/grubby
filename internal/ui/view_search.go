package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"grubby/internal/config"
	"grubby/internal/search"
)

func (m Model) handleSearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.SearchInput.Focused() {
		switch msg.String() {
		case "enter":
			query := strings.TrimSpace(m.SearchInput.Value())
			if query != "" {
				m.SearchInput.Blur()
				m.Searching = true
				m.StatusMsg = "Searching cloned repositories..."
				return m, m.runSearchCmd(query)
			}
		case "esc":
			m.SearchInput.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.SearchInput, cmd = m.SearchInput.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "/":
		m.SearchInput.Focus()
		return m, textinput.Blink
	case "up", "k":
		if m.SearchFocusedPane == 0 {
			if m.SearchCursor > 0 {
				m.SearchCursor--
				m.updateSearchPreview()
			}
		} else {
			m.SearchPreview.LineUp(1)
		}
	case "down", "j":
		if m.SearchFocusedPane == 0 {
			if m.SearchCursor < len(m.SearchFlatNodes)-1 {
				m.SearchCursor++
				m.updateSearchPreview()
			}
		} else {
			m.SearchPreview.LineDown(1)
		}
	case "h", "left":
		// Collapse current node
		if len(m.SearchFlatNodes) > 0 && m.SearchCursor < len(m.SearchFlatNodes) {
			node := m.SearchFlatNodes[m.SearchCursor]
			if node.Kind != search.NodeMatch && node.Expanded {
				node.Expanded = false
				m.SearchFlatNodes = search.FlattenVisibleNodes(m.SearchTree)
			}
		}
	case "l", "right", " ":
		// Expand current node
		if len(m.SearchFlatNodes) > 0 && m.SearchCursor < len(m.SearchFlatNodes) {
			node := m.SearchFlatNodes[m.SearchCursor]
			if node.Kind != search.NodeMatch {
				node.Expanded = !node.Expanded
				m.SearchFlatNodes = search.FlattenVisibleNodes(m.SearchTree)
			}
		}
	case "tab":
		m.SearchFocusedPane = (m.SearchFocusedPane + 1) % 2
	case "enter":
		// Open in nvim via iTerm2
		if len(m.SearchFlatNodes) > 0 && m.SearchCursor < len(m.SearchFlatNodes) {
			node := m.SearchFlatNodes[m.SearchCursor]
			if node.RepoName != "" {
				m = m.recordRepoInteraction(node.RepoName)
			}
			lineNum := 1
			if node.Match != nil {
				lineNum = node.Match.LineNumber
			}
			repoDir := node.RepoPath
			if repoDir == "" {
				repoDir = filepath.Dir(node.AbsPath)
			}
			err := OpenInITermNvim(repoDir, node.AbsPath, lineNum)
			if err != nil {
				return m, func() tea.Msg { return StatusToastMsg("Failed to open nvim: " + err.Error()) }
			}
			collapsed := config.CollapseHome(node.AbsPath)
			return m, func() tea.Msg {
				return StatusToastMsg(fmt.Sprintf("Opened nvim at %s:%d", collapsed, lineNum))
			}
		}
	}
	return m, nil
}

func (m *Model) updateSearchPreview() {
	if len(m.SearchFlatNodes) == 0 || m.SearchCursor >= len(m.SearchFlatNodes) {
		m.SearchPreview.SetContent("(No match selected)")
		m.SearchPreview.SetYOffset(0)
		return
	}

	node := m.SearchFlatNodes[m.SearchCursor]
	targetLine := 1
	filePath := node.AbsPath

	if node.Kind == search.NodeMatch && node.Match != nil {
		targetLine = node.Match.LineNumber
	} else if node.Kind == search.NodeFile {
		targetLine = 1
	} else {
		m.SearchPreview.SetContent(fmt.Sprintf("Repository: %s\nSelect a match below to preview code.", node.Title))
		m.SearchPreview.SetYOffset(0)
		return
	}

	rendered, err := search.RenderPreview(filePath, targetLine)
	if err != nil {
		m.SearchPreview.SetContent(fmt.Sprintf("Could not preview %s: %v", config.CollapseHome(filePath), err))
		m.SearchPreview.SetYOffset(0)
		return
	}
	m.SearchPreview.SetContent(rendered)
	offset := targetLine - 5
	if offset < 0 {
		offset = 0
	}
	m.SearchPreview.SetYOffset(offset)
}

func (m Model) renderSearchView() string {
	inputView := m.SearchInput.View()
	totalHits := 0
	for _, r := range m.SearchResults {
		totalHits += r.TotalHits
	}

	summary := MetaBarStyle.Render(fmt.Sprintf("Results: %d matches across %d repositories", totalHits, len(m.SearchResults)))
	topBar := lipgloss.JoinHorizontal(lipgloss.Top, "  ", inputView, "  ", summary)

	leftWidth := (m.Width / 2) - 2
	rightWidth := m.Width - leftWidth - 4
	paneHeight := m.Height - 9
	if paneHeight < 5 {
		paneHeight = 10
	}

	// Render Left Pane (Tree) with windowed scrolling
	var treeLines []string
	if m.Searching {
		treeLines = append(treeLines, "\n  Searching repositories...")
	} else if len(m.SearchFlatNodes) == 0 {
		treeLines = append(treeLines, "\n  Type a pattern above and press Enter to search all cloned workspaces.")
	} else {
		start := 0
		if m.SearchCursor >= paneHeight {
			start = m.SearchCursor - paneHeight + 1
		}
		end := start + paneHeight
		if end > len(m.SearchFlatNodes) {
			end = len(m.SearchFlatNodes)
		}

		for i := start; i < end; i++ {
			node := m.SearchFlatNodes[i]
			cursor := "  "
			if i == m.SearchCursor {
				cursor = "▶ "
			}

			indent := strings.Repeat("  ", node.Depth)
			icon := "• "
			if node.Kind == search.NodeRepo {
				if node.Expanded {
					icon = "▼ "
				} else {
					icon = "▶ "
				}
			} else if node.Kind == search.NodeFile {
				if node.Expanded {
					icon = "▼ "
				} else {
					icon = "▶ "
				}
			} else if node.Kind == search.NodeMatch && node.Match != nil {
				icon = fmt.Sprintf("L%-3d │ ", node.Match.LineNumber)
			}

			title := node.Title
			if node.Kind == search.NodeMatch && node.Match != nil {
				title = strings.TrimSpace(node.Match.LineText)
			}
			maxLen := leftWidth - len(indent) - len(icon) - 4
			if maxLen > 0 && len(title) > maxLen {
				title = title[:maxLen-3] + "..."
			}

			line := fmt.Sprintf("%s%s%s%s", cursor, indent, icon, title)
			if i == m.SearchCursor {
				treeLines = append(treeLines, SelectedRowStyle.Render(line))
			} else {
				treeLines = append(treeLines, NormalRowStyle.Render(line))
			}
		}
	}

	leftBox := lipgloss.NewStyle().
		Width(leftWidth).
		Height(paneHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Render(strings.Join(treeLines, "\n"))

	// Render Right Pane (Code Preview)
	m.SearchPreview.Height = paneHeight - 2
	m.SearchPreview.Width = rightWidth - 2
	previewBox := lipgloss.NewStyle().
		Width(rightWidth).
		Height(paneHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(SecondaryColor).
		Render(m.SearchPreview.View())

	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, " ", previewBox)
	return lipgloss.JoinVertical(lipgloss.Left, topBar, panes)
}
