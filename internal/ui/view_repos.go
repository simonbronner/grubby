package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"grubby/internal/config"
	"grubby/internal/git"
	"grubby/internal/github"
	"grubby/internal/storage"
)

type RepoItemKind int

const (
	ItemTierHeader RepoItemKind = iota
	ItemRepo
)

type RepoTableItem struct {
	Kind       RepoItemKind
	Tier       storage.RepoTier
	Repo       *github.Repository
	ChildCount int
	Expanded   bool
}

func (m Model) getRepoLabels(r *github.Repository) []string {
	if r == nil {
		return nil
	}
	seen := make(map[string]bool)
	var result []string

	add := func(l string) {
		clean := strings.TrimPrefix(strings.TrimSpace(l), "#")
		clean = strings.ReplaceAll(clean, " ", "-")
		clean = strings.ToLower(clean)
		if clean != "" && !seen[clean] {
			seen[clean] = true
			result = append(result, clean)
		}
	}

	if r.ID > 0 && m.RepoLabels != nil {
		if labels, ok := m.RepoLabels[r.ID]; ok {
			for _, l := range labels {
				add(l)
			}
		}
	}
	if m.RepoLabelsByName != nil {
		if labels, ok := m.RepoLabelsByName[strings.ToLower(r.Name)]; ok {
			for _, l := range labels {
				add(l)
			}
		}
	}
	if m.Config != nil {
		for _, grp := range m.Config.GetGroupsForRepo(r.Name) {
			add(grp)
		}
	}

	return result
}

func (m Model) focusRepoCursor(repoName string) Model {
	items := m.getRepoTableItems()
	for idx, it := range items {
		if it.Kind == ItemRepo && it.Repo != nil && strings.EqualFold(it.Repo.Name, repoName) {
			m.RepoCursor = idx
			break
		}
	}
	return m
}

func (m Model) getSelectedRepo() *github.Repository {
	items := m.getRepoTableItems()
	if len(items) > 0 && m.RepoCursor >= 0 && m.RepoCursor < len(items) {
		item := items[m.RepoCursor]
		if item.Kind == ItemRepo && item.Repo != nil {
			return item.Repo
		}
	}
	// Fallback to getDisplayRepos
	repos := m.getDisplayRepos()
	if len(repos) > 0 && m.RepoCursor >= 0 && m.RepoCursor < len(repos) {
		return &repos[m.RepoCursor]
	}
	return nil
}

func (m Model) isTierCollapsed(tier storage.RepoTier) bool {
	// Auto-expand all tiers when a search or label filter is active
	if strings.TrimSpace(m.RepoFilter) != "" || strings.TrimSpace(m.RepoGroupFilter) != "" {
		return false
	}
	if m.CollapsedTiers == nil {
		return tier == storage.TierDead
	}
	collapsed, ok := m.CollapsedTiers[tier]
	if !ok {
		return tier == storage.TierDead
	}
	return collapsed
}

func (m Model) getRepoTableItems() []RepoTableItem {
	repos := m.getDisplayRepos()
	if len(repos) == 0 {
		return nil
	}

	tierOrder := []storage.RepoTier{
		storage.TierHot,
		storage.TierWarm,
		storage.TierCold,
		storage.TierDead,
	}

	tierRepos := make(map[storage.RepoTier][]github.Repository)
	for _, r := range repos {
		tier := m.getRepoTier(r.Name)
		tierRepos[tier] = append(tierRepos[tier], r)
	}

	var items []RepoTableItem
	for _, tier := range tierOrder {
		list := tierRepos[tier]
		if len(list) == 0 {
			continue
		}
		collapsed := m.isTierCollapsed(tier)
		items = append(items, RepoTableItem{
			Kind:       ItemTierHeader,
			Tier:       tier,
			ChildCount: len(list),
			Expanded:   !collapsed,
		})
		if !collapsed {
			for i := range list {
				items = append(items, RepoTableItem{
					Kind: ItemRepo,
					Tier: tier,
					Repo: &list[i],
				})
			}
		}
	}

	return items
}

func (m Model) getDisplayRepos() []github.Repository {
	var result []github.Repository

	filter := strings.ToLower(strings.TrimSpace(m.RepoFilter))
	filter = strings.TrimPrefix(filter, "#")

	groupFilter := strings.ToLower(strings.TrimSpace(m.RepoGroupFilter))
	groupFilter = strings.TrimPrefix(groupFilter, "#")

	for _, r := range m.Repos {
		repoLabels := m.getRepoLabels(&r)

		// 1. Group / Label filter (from '#' modal)
		if groupFilter != "" {
			matchesGroup := false
			for _, lbl := range repoLabels {
				if strings.EqualFold(lbl, groupFilter) {
					matchesGroup = true
					break
				}
			}
			if !matchesGroup {
				continue
			}
		}

		// 2. Name or Label substring filter (from '/' modal)
		if filter != "" {
			matchesName := strings.Contains(strings.ToLower(r.Name), filter) ||
				strings.Contains(strings.ToLower(r.FullName), filter)

			matchesLabel := false
			for _, lbl := range repoLabels {
				if strings.Contains(strings.ToLower(lbl), filter) {
					matchesLabel = true
					break
				}
			}

			if !matchesName && !matchesLabel {
				continue
			}
		}

		result = append(result, r)
	}

	if m.RepoSortVulnerabilities {
		sort.SliceStable(result, func(i, j int) bool {
			secA := m.SecuritySummaries[result[i].Name]
			secB := m.SecuritySummaries[result[j].Name]

			if secA.Total != secB.Total {
				return secA.Total > secB.Total
			}
			if secA.Critical != secB.Critical {
				return secA.Critical > secB.Critical
			}
			if secA.High != secB.High {
				return secA.High > secB.High
			}
			if secA.Medium != secB.Medium {
				return secA.Medium > secB.Medium
			}
			return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
		})
	} else {
		tmuxSessions := m.TmuxSessions
		if tmuxSessions == nil {
			tmuxSessions = GetTmuxSessions()
		}

		sort.SliceStable(result, func(i, j int) bool {
			// 0. Group by Tier priority (Hot=0, Warm=1, Cold=2, Dead=3)
			tierA := storage.TierPriority(m.getRepoTier(result[i].Name))
			tierB := storage.TierPriority(m.getRepoTier(result[j].Name))
			if tierA != tierB {
				return tierA < tierB
			}

			actA := m.getRepoLastInteractedTime(result[i].Name, tmuxSessions)
			actB := m.getRepoLastInteractedTime(result[j].Name, tmuxSessions)

			hasActA := !actA.IsZero()
			hasActB := !actB.IsZero()

			// 1. Repositories opened / interacted with come first, sorted by last interaction descending
			if hasActA && hasActB {
				if !actA.Equal(actB) {
					return actA.After(actB)
				}
			} else if hasActA != hasActB {
				return hasActA
			}

			// 2. Then by last updated / pushed descending
			updA := result[i].LastUpdated()
			updB := result[j].LastUpdated()
			if !updA.Equal(updB) {
				return updA.After(updB)
			}

			// 3. Alphabetical tiebreaker
			return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
		})
	}

	return result
}

func (m Model) handleReposKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.getRepoTableItems()

	switch msg.String() {
	case "up", "k":
		if m.RepoCursor > 0 {
			m.RepoCursor--
		}
		return m, nil
	case "down", "j":
		if m.RepoCursor < len(items)-1 {
			m.RepoCursor++
		}
		return m, nil
	case "pgdown", "ctrl+d":
		visibleCount := m.Height - 8
		if visibleCount < 5 {
			visibleCount = 10
		}
		m.RepoCursor += visibleCount
		if m.RepoCursor >= len(items) {
			m.RepoCursor = len(items) - 1
		}
		if m.RepoCursor < 0 {
			m.RepoCursor = 0
		}
		return m, nil
	case "pgup", "ctrl+u":
		visibleCount := m.Height - 8
		if visibleCount < 5 {
			visibleCount = 10
		}
		m.RepoCursor -= visibleCount
		if m.RepoCursor < 0 {
			m.RepoCursor = 0
		}
		return m, nil
	case "home":
		m.RepoCursor = 0
		return m, nil
	case "end", "G":
		if len(items) > 0 {
			m.RepoCursor = len(items) - 1
		}
		return m, nil
	case "/":
		// '/' filter: filter repositories by name or label
		m.ActiveModal = ModalFilterRepoName
		m.RepoFilterInput.SetValue(m.RepoFilter)
		m.RepoFilterInput.Focus()
		return m, textinput.Blink
	case "#":
		// '#' filter: filter repositories by label
		m.ActiveModal = ModalFilterGroup
		m.ModalCursor = 0
		return m, nil
	case "v":
		// 'v' sort by vulnerabilities: toggle sorting from most to least vulnerabilities
		m.RepoSortVulnerabilities = !m.RepoSortVulnerabilities
		m.RepoCursor = 0
		if m.RepoSortVulnerabilities {
			return m, func() tea.Msg { return StatusToastMsg("Sorted repositories by vulnerabilities (most to least)") }
		}
		return m, func() tea.Msg { return StatusToastMsg("Reset repository sort order to default") }
	case "esc":
		// 'esc' clears active filters and sorting
		if m.RepoFilter != "" || m.RepoGroupFilter != "" || m.RepoSortVulnerabilities {
			m.RepoFilter = ""
			m.RepoGroupFilter = ""
			m.RepoSortVulnerabilities = false
			m.RepoCursor = 0
			return m, func() tea.Msg { return StatusToastMsg("Cleared all repository filters and sorting") }
		}
		return m, nil
	}

	if len(items) == 0 || m.RepoCursor < 0 || m.RepoCursor >= len(items) {
		return m, nil
	}

	currentItem := items[m.RepoCursor]

	// 1. Key handling when cursor is on Tier Header
	if currentItem.Kind == ItemTierHeader {
		switch msg.String() {
		case "enter", " ", "l", "right":
			if m.CollapsedTiers == nil {
				m.CollapsedTiers = make(map[storage.RepoTier]bool)
			}
			isCollapsed := m.isTierCollapsed(currentItem.Tier)
			m.CollapsedTiers[currentItem.Tier] = !isCollapsed
			return m, nil
		case "h", "left":
			if m.CollapsedTiers == nil {
				m.CollapsedTiers = make(map[storage.RepoTier]bool)
			}
			m.CollapsedTiers[currentItem.Tier] = true
			return m, nil
		default:
			return m, nil
		}
	}

	// 2. Key handling when cursor is on a Repository row
	repo := *currentItem.Repo

	switch msg.String() {
	case "h", "[":
		// Collapse parent tier and land cursor on header
		if m.CollapsedTiers == nil {
			m.CollapsedTiers = make(map[storage.RepoTier]bool)
		}
		m.CollapsedTiers[currentItem.Tier] = true
		newItems := m.getRepoTableItems()
		for idx, it := range newItems {
			if it.Kind == ItemTierHeader && it.Tier == currentItem.Tier {
				m.RepoCursor = idx
				break
			}
		}
		return m, nil
	case "p":
		// 'p' pull requests: open Pull Requests tab filtered by the selected repository
		m = m.recordRepoInteraction(repo.Name)
		m.ActiveTab = TabPRs
		m.PRFilter = repo.Name
		m.PRCursor = 0
		return m, func() tea.Msg {
			return StatusToastMsg(fmt.Sprintf("Filtered pull requests for %s", repo.Name))
		}
	case "s":
		// 's' session capability: switch to repository tmux session in Grubby window
		m = m.recordRepoInteraction(repo.Name)
		m = m.focusRepoCursor(repo.Name)
		st := m.GitStatuses[repo.Name]
		if st.IsCloned {
			msg, err := GoToRepoSession(repo.Name, st.LocalPath)
			if err != nil {
				return m, func() tea.Msg { return StatusToastMsg("Failed to switch session: " + err.Error()) }
			}
			return m, tea.Batch(
				func() tea.Msg { return StatusToastMsg(msg) },
				m.fetchSingleRepoCmd(m.CurrentOrg, repo.Name),
			)
		}
		return m.startCloneWithAction(repo, RepoActionSession)
	case "e", "E":
		// 'e' editor: open editor in a new window in the tmux session for that repo
		m = m.recordRepoInteraction(repo.Name)
		m = m.focusRepoCursor(repo.Name)
		st := m.GitStatuses[repo.Name]
		if st.IsCloned {
			msg, err := OpenRepoEditor(repo.Name, st.LocalPath)
			if err != nil {
				return m, func() tea.Msg { return StatusToastMsg("Failed to open editor: " + err.Error()) }
			}
			return m, tea.Batch(
				func() tea.Msg { return StatusToastMsg(msg) },
				m.fetchSingleRepoCmd(m.CurrentOrg, repo.Name),
			)
		}
		return m.startCloneWithAction(repo, RepoActionEditor)
	case "t":
		// 't' open repo shell (creates new window if session exists, or creates session)
		m = m.recordRepoInteraction(repo.Name)
		m = m.focusRepoCursor(repo.Name)
		st := m.GitStatuses[repo.Name]
		if st.IsCloned {
			msg, err := OpenRepoShell(repo.Name, st.LocalPath)
			if err != nil {
				return m, func() tea.Msg { return StatusToastMsg("Failed to open shell: " + err.Error()) }
			}
			return m, tea.Batch(
				func() tea.Msg { return StatusToastMsg(msg) },
				m.fetchSingleRepoCmd(m.CurrentOrg, repo.Name),
			)
		}
		return m.startCloneWithAction(repo, RepoActionShell)
	case "c":
		// 'c' clone repo
		m = m.recordRepoInteraction(repo.Name)
		m = m.focusRepoCursor(repo.Name)
		st := m.GitStatuses[repo.Name]
		if st.IsCloned {
			return m, func() tea.Msg { return StatusToastMsg(repo.Name + " is already cloned!") }
		}
		return m.startCloneWithAction(repo, RepoActionNone)
	case "enter":
		// Enter key opens repository root in dedicated browser
		m = m.recordRepoInteraction(repo.Name)
		m = m.focusRepoCursor(repo.Name)
		_ = OpenBrowser(repo.URL)
		return m, tea.Batch(
			func() tea.Msg { return StatusToastMsg("Opened " + repo.Name + " in dedicated browser") },
			m.fetchSingleRepoCmd(m.CurrentOrg, repo.Name),
		)
	case "o":
		// 'o' opens the repo tabs submenu
		m = m.recordRepoInteraction(repo.Name)
		m = m.focusRepoCursor(repo.Name)
		m.ActiveModal = ModalRepoTabs
		m.ModalCursor = 0
		return m, m.fetchSingleRepoCmd(m.CurrentOrg, repo.Name)
	case "S":
		// 'S' opens repository Security (Dependabot) directly in dedicated browser
		m = m.recordRepoInteraction(repo.Name)
		m = m.focusRepoCursor(repo.Name)
		url := fmt.Sprintf("https://github.com/%s/security/dependabot", repo.FullName)
		_ = OpenBrowser(url)
		return m, tea.Batch(
			func() tea.Msg { return StatusToastMsg("Opened Security tab for " + repo.Name) },
			m.fetchSingleRepoCmd(m.CurrentOrg, repo.Name),
		)
	case "g":
		// Open Groups/Labels modal for current repo
		m.ActiveModal = ModalManageGroups
		m.ModalCursor = 0
		m.SelectedRepoGroups = make(map[string]bool)
		for _, lbl := range m.getRepoLabels(&repo) {
			m.SelectedRepoGroups[lbl] = true
		}
		return m, nil
	case "u":
		// Fast-forward repo
		m = m.recordRepoInteraction(repo.Name)
		m = m.focusRepoCursor(repo.Name)
		st := m.GitStatuses[repo.Name]
		if !st.IsCloned {
			return m, func() tea.Msg { return StatusToastMsg("Repo is not cloned yet.") }
		}
		if st.IsDirty {
			return m, func() tea.Msg { return StatusToastMsg("Cannot fast-forward: working tree has uncommitted changes!") }
		}
		if st.Behind == 0 {
			return m, func() tea.Msg { return StatusToastMsg("Repo is already up to date.") }
		}
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := git.FastForward(ctx, st.LocalPath)
			if err != nil {
				return StatusToastMsg("Fast-forward error: " + err.Error())
			}
			return StatusToastMsg("Successfully fast-forwarded " + repo.Name)
		}
	}
	return m, nil
}

func (m Model) startCloneWithAction(repo github.Repository, postAction RepoActionType) (tea.Model, tea.Cmd) {
	m = m.recordRepoInteraction(repo.Name)
	m = m.focusRepoCursor(repo.Name)
	var targetDir string
	if m.GitMgr != nil {
		targetDir = m.GitMgr.GetRepoPath(repo.Name)
	} else {
		targetDir = filepath.Join(os.Getenv("HOME"), "workspaces", repo.Name)
	}
	actionDesc := "session"
	if postAction == RepoActionEditor {
		actionDesc = "editor"
	} else if postAction == RepoActionShell {
		actionDesc = "shell"
	}

	if postAction != RepoActionNone {
		m.StatusMsg = fmt.Sprintf("Cloning %s to open %s...", repo.FullName, actionDesc)
	} else {
		m.StatusMsg = "Cloning " + repo.FullName + "..."
	}
	m.StatusMsgTimer = time.Now().Add(3 * time.Minute)

	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		err := github.CloneRepo(ctx, repo.FullName, targetDir)
		if err != nil {
			return RepoClonedMsg{RepoName: repo.Name, TargetDir: targetDir, Err: err, PostAction: postAction}
		}
		newStatus := git.InspectRepo(ctx, repo.Name, targetDir)
		return RepoClonedMsg{RepoName: repo.Name, TargetDir: targetDir, Status: newStatus, Err: nil, PostAction: postAction}
	}
}

func (m Model) renderReposView() string {
	items := m.getRepoTableItems()
	repos := m.getDisplayRepos()

	if m.Loading && len(m.Repos) == 0 {
		return fmt.Sprintf("\n  %s Fetching repositories from GitHub...\n", m.Spinner.View())
	}
	if len(items) == 0 {
		if m.RepoFilter != "" || m.RepoGroupFilter != "" {
			return "\n  No repositories match the current filter(s).\n  Press [Esc] to clear filters.\n"
		}
		return "\n  No repositories found for " + m.CurrentOrg + ".\n"
	}

	if m.RepoCursor >= len(items) {
		m.RepoCursor = len(items) - 1
	}
	if m.RepoCursor < 0 {
		m.RepoCursor = 0
	}

	cols := computeRepoColumns(m.Width)
	tmuxSessions := m.TmuxSessions
	if tmuxSessions == nil {
		tmuxSessions = GetTmuxSessions()
	}

	headerRow := lipgloss.JoinHorizontal(lipgloss.Top,
		cols.Cursor.Render(""),
		cols.Name.Render("REPOSITORY & LABELS"),
		cols.Summary.Render("(Vuln - PR - Issues)"),
		cols.Git.Render("GIT STATUS"),
		cols.Session.Render("TMUX"),
		cols.Last.Render("LAST PUSHED"),
		cols.Path.Render("LOCAL PATH"),
	)
	renderedHeader := TableHeaderStyle.Render(headerRow)

	var rows []string

	// Active filter indicator if filtering or sorting is on
	var filterBadges []string
	if m.RepoGroupFilter != "" {
		filterBadges = append(filterBadges, lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true).Render("Label: #"+strings.TrimPrefix(m.RepoGroupFilter, "#")))
	}
	if m.RepoFilter != "" {
		filterBadges = append(filterBadges, lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(fmt.Sprintf("Name: %q", m.RepoFilter)))
	}
	if m.RepoSortVulnerabilities {
		filterBadges = append(filterBadges, lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render("Sort: 🛡️ Most Vulns ↓"))
	}

	hotCount, warmCount, coldCount, deadCount := m.getRepoTierCounts()
	tierSummary := fmt.Sprintf("🔥 %d Hot  🌞 %d Warm  🧊 %d Cold  💀 %d Dead", hotCount, warmCount, coldCount, deadCount)

	spinnerSuffix := ""
	if m.Loading {
		spinnerSuffix = fmt.Sprintf("  •  %s Syncing...", m.Spinner.View())
	}

	if len(filterBadges) > 0 {
		filterSummary := MetaBarStyle.Render("  🔍 ") + strings.Join(filterBadges, "  ") +
			MetaBarStyle.Render(fmt.Sprintf("  (%d of %d repos) • %s%s • [Esc] Clear", len(repos), len(m.Repos), tierSummary, spinnerSuffix))
		rows = append(rows, filterSummary)
	} else if len(m.Repos) > 0 {
		tierBar := MetaBarStyle.Render(fmt.Sprintf("  %s%s  (periodic polling focuses on Hot & Warm repos)", tierSummary, spinnerSuffix))
		rows = append(rows, tierBar)
	}

	rows = append(rows, renderedHeader)

	visibleCount := m.Height - 9
	if visibleCount < 5 {
		visibleCount = 10
	}

	start := 0
	if m.RepoCursor >= visibleCount {
		start = m.RepoCursor - visibleCount + 1
	}
	end := start + visibleCount
	if end > len(items) {
		end = len(items)
	}

	totalW := m.Width - 4
	if totalW < 80 {
		totalW = 80
	}

	for i := start; i < end; i++ {
		item := items[i]

		if item.Kind == ItemTierHeader {
			arrow := "▼"
			if !item.Expanded {
				arrow = "▶"
			}
			tierIcon := storage.TierIcon(item.Tier)
			tierName := storage.TierName(item.Tier)

			unit := "repos"
			if item.ChildCount == 1 {
				unit = "repo"
			}
			headerText := fmt.Sprintf("%s %s %s (%d %s)", arrow, tierIcon, tierName, item.ChildCount, unit)
			if !item.Expanded {
				headerText += "  ── [Space to expand]"
			}

			// Add trailing rule line
			if len(headerText) < totalW {
				headerText += " " + strings.Repeat("─", totalW-len(headerText)-1)
			}

			var tierColor lipgloss.Color
			switch item.Tier {
			case storage.TierHot:
				tierColor = lipgloss.Color("#FF7B72")
			case storage.TierWarm:
				tierColor = lipgloss.Color("#E5C07B")
			case storage.TierCold:
				tierColor = lipgloss.Color("#79C0FF")
			case storage.TierDead:
				tierColor = lipgloss.Color("#6E7681")
			default:
				tierColor = lipgloss.Color("#E5C07B")
			}

			styledHeader := lipgloss.NewStyle().Foreground(tierColor).Bold(true).Render(headerText)

			cursorPrefix := "  "
			if i == m.RepoCursor {
				cursorPrefix = "▶ "
			}

			rowContent := cols.Cursor.Render(cursorPrefix) + styledHeader
			if i == m.RepoCursor {
				rows = append(rows, SelectedRowStyle.MaxHeight(1).Render(rowContent))
			} else {
				rows = append(rows, NormalRowStyle.MaxHeight(1).Render(rowContent))
			}
			continue
		}

		// Repo row
		r := *item.Repo
		st := m.GitStatuses[r.Name]
		isDead := item.Tier == storage.TierDead

		// 1. Calculate open PR count for this repository
		prCount := 0
		for _, pr := range m.PRs {
			if strings.EqualFold(pr.Repo, r.Name) {
				prCount++
			}
		}

		// 2. Calculate task/issue count for this repository
		taskCount := 0
		for _, t := range m.PersonalTasks {
			if strings.EqualFold(t.RepoName, r.Name) {
				taskCount++
			}
		}
		for _, t := range m.SquadTasks {
			if strings.EqualFold(t.RepoName, r.Name) {
				taskCount++
			}
		}

		// Name & Labels formatting
		repoDisplayName := r.Name
		labels := m.getRepoLabels(&r)

		var styledLabels []string
		for _, lbl := range labels {
			colorHex := m.getLabelColor(lbl)
			if isDead {
				styledLabels = append(styledLabels, lipgloss.NewStyle().Foreground(lipgloss.Color("#6E7681")).Render("#"+lbl))
			} else {
				styledLabels = append(styledLabels, lipgloss.NewStyle().Foreground(lipgloss.Color("#"+strings.TrimPrefix(colorHex, "#"))).Bold(true).Render("#"+lbl))
			}
		}

		var styledName string
		if isDead {
			styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("#6E7681")).Render(repoDisplayName)
		} else {
			styledName = lipgloss.NewStyle().Bold(true).Render(repoDisplayName)
		}

		nameColText := "  " + styledName
		rawNameLen := 2 + len(repoDisplayName)
		if len(styledLabels) > 0 {
			nameColText += " " + strings.Join(styledLabels, " ")
			for _, lbl := range labels {
				rawNameLen += 2 + len(lbl) // " #" + lbl
			}
		}

		// Display repository "About" text in italics with a slightly less light font
		about := strings.TrimSpace(r.Description)
		if about != "" && cols.NameWidth > rawNameLen+12 {
			avail := cols.NameWidth - rawNameLen - 4
			aboutSnippet := about
			if len(aboutSnippet) > avail {
				if avail > 4 {
					aboutSnippet = aboutSnippet[:avail-1] + "…"
				} else {
					aboutSnippet = ""
				}
			}
			if aboutSnippet != "" {
				var styledAbout string
				if isDead {
					styledAbout = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#555566")).Render(aboutSnippet)
				} else {
					styledAbout = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#7D8590")).Render(aboutSnippet)
				}
				nameColText += "  " + styledAbout
			}
		}

		// 3. (Vuln - PR - Issues) column: ( 0, 0, 0 ) / ( 1, 2, 3 ) - ( prCount ) - ( taskCount )
		var vulnText string
		sum := m.SecuritySummaries[r.Name]
		if isDead {
			deadStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6E7681"))
			if sum.Total == 0 {
				vulnText = deadStyle.Render("( 0, 0, 0 )")
			} else {
				medRest := sum.Medium + sum.Low
				vulnText = deadStyle.Render(fmt.Sprintf("( %d, %d, %d )", sum.Critical, sum.High, medRest))
			}
		} else {
			if sum.Total == 0 {
				vulnText = lipgloss.NewStyle().Foreground(SuccessColor).Render("( 0, 0, 0 )")
			} else {
				medRest := sum.Medium + sum.Low
				var cStr, hStr, mStr string
				if sum.Critical > 0 {
					cStr = lipgloss.NewStyle().Foreground(DangerColor).Bold(true).Render(fmt.Sprintf("%d", sum.Critical))
				} else {
					cStr = lipgloss.NewStyle().Foreground(MutedColor).Render("0")
				}
				if sum.High > 0 {
					hStr = lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render(fmt.Sprintf("%d", sum.High))
				} else {
					hStr = lipgloss.NewStyle().Foreground(MutedColor).Render("0")
				}
				if medRest > 0 {
					mStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B")).Bold(true).Render(fmt.Sprintf("%d", medRest))
				} else {
					mStr = lipgloss.NewStyle().Foreground(MutedColor).Render("0")
				}
				vulnText = fmt.Sprintf("( %s, %s, %s )", cStr, hStr, mStr)
			}
		}

		var prBadge string
		if isDead {
			prBadge = lipgloss.NewStyle().Foreground(lipgloss.Color("#6E7681")).Render(fmt.Sprintf("( %d )", prCount))
		} else if prCount > 0 {
			prBadge = "( " + lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(fmt.Sprintf("%d", prCount)) + " )"
		} else {
			prBadge = lipgloss.NewStyle().Foreground(MutedColor).Render("( 0 )")
		}

		var taskBadge string
		if isDead {
			taskBadge = lipgloss.NewStyle().Foreground(lipgloss.Color("#6E7681")).Render(fmt.Sprintf("( %d )", taskCount))
		} else if taskCount > 0 {
			taskBadge = "( " + lipgloss.NewStyle().Foreground(lipgloss.Color("#98C379")).Bold(true).Render(fmt.Sprintf("%d", taskCount)) + " )"
		} else {
			taskBadge = lipgloss.NewStyle().Foreground(MutedColor).Render("( 0 )")
		}

		summaryColText := fmt.Sprintf("%s - %s - %s", vulnText, prBadge, taskBadge)

		// 4. Git badge
		gitBadge := st.Badge()
		var styledBadge string
		if isDead {
			styledBadge = lipgloss.NewStyle().Foreground(lipgloss.Color("#6E7681")).Render(gitBadge)
		} else {
			switch st.State {
			case git.StateSynced:
				styledBadge = SyncedBadge.Render(gitBadge)
			case git.StateBehind:
				styledBadge = BehindBadge.Render(gitBadge)
			case git.StateDirty:
				styledBadge = DirtyBadge.Render(gitBadge)
			case git.StateNotCloned:
				styledBadge = NotClonedBadge.Render(gitBadge)
			default:
				styledBadge = gitBadge
			}
		}

		// 5. TMUX session
		var sessionColText string
		if sess, ok := FindTmuxSession(r.Name, tmuxSessions); ok && sess.Windows > 0 {
			plural := "win"
			if sess.Windows > 1 {
				plural = "wins"
			}
			if isDead {
				sessionColText = lipgloss.NewStyle().Foreground(lipgloss.Color("#6E7681")).Render(fmt.Sprintf("● %d %s", sess.Windows, plural))
			} else {
				sessionColText = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(fmt.Sprintf("● %d %s", sess.Windows, plural))
			}
		} else {
			sessionColText = lipgloss.NewStyle().Foreground(lipgloss.Color("#505565")).Render("○ none")
		}

		// 6. Last pushed
		lastPushed := formatTimeAgo(r.LastUpdated())
		if isDead {
			lastPushed = lipgloss.NewStyle().Foreground(lipgloss.Color("#6E7681")).Render(lastPushed)
		}

		// 7. Local path
		localPath := "-"
		if st.IsCloned {
			localPath = config.CollapseHome(st.LocalPath)
			if len(localPath) > cols.PathWidth-2 && cols.PathWidth > 3 {
				localPath = "…" + localPath[len(localPath)-(cols.PathWidth-3):]
			}
		}
		if isDead {
			localPath = lipgloss.NewStyle().Foreground(lipgloss.Color("#6E7681")).Render(localPath)
		}

		prefix := "  "
		if i == m.RepoCursor {
			prefix = "▶ "
		}

		rowCells := lipgloss.JoinHorizontal(lipgloss.Top,
			cols.Cursor.Render(prefix),
			cols.Name.Render(nameColText),
			cols.Summary.Render(summaryColText),
			cols.Git.Render(styledBadge),
			cols.Session.Render(sessionColText),
			cols.Last.Render(lastPushed),
			cols.Path.Render(localPath),
		)

		if i == m.RepoCursor {
			rows = append(rows, SelectedRowStyle.MaxHeight(1).Render(rowCells))
		} else {
			rows = append(rows, NormalRowStyle.MaxHeight(1).Render(rowCells))
		}
	}

	return strings.Join(rows, "\n")
}

func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	if d.Hours() < 1 {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d.Hours() < 24 {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "yesterday"
	}
	if days < 30 {
		return fmt.Sprintf("%dd ago", days)
	}
	return t.Format("02 Jan 2006")
}
