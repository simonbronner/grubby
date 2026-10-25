package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"grubby/internal/github"
)

type RepoActivitySummary struct {
	RepoName     string
	Commits      int
	Pushes       int
	Reviews      int
	Merges       int
	TotalActions int
	Contributors []string
}

type MemberContribution struct {
	ID           string
	Login        string
	Name         string
	AvatarArt    string
	Commits      int
	Pushes       int
	Reviews      int
	Merges       int
	TotalActions int
	TopRepos     []string
	RecentAction string
	LastActive   time.Time
}

type SquadDashboardData struct {
	TimeWindow          string
	ActiveRepos         []RepoActivitySummary
	MemberContributions []MemberContribution
}

func (m Model) computeSquadDashboard() SquadDashboardData {
	cutoff := time.Now().Add(-14 * 24 * time.Hour) // Last 2 weeks
	activeSquad := m.Config.GetActiveSquad()

	var squadMembers []string
	if activeSquad != nil {
		squadMembers = activeSquad.Members
	}

	memberMap := make(map[string]bool)
	for _, mem := range squadMembers {
		memberMap[strings.ToLower(strings.TrimSpace(mem))] = true
	}

	orgMemberDetails := make(map[string]github.OrgMember)
	for _, om := range m.OrgMembers {
		orgMemberDetails[strings.ToLower(om.Login)] = om
	}

	// Filter squad events within the last 14 days
	repoMap := make(map[string]*RepoActivitySummary)
	repoContribMap := make(map[string]map[string]bool)

	memberContribMap := make(map[string]*MemberContribution)
	memberRepoCounts := make(map[string]map[string]int)

	// Initialize squad members even if they had 0 activity in 2 weeks
	for _, mem := range squadMembers {
		memLower := strings.ToLower(strings.TrimSpace(mem))
		name := mem
		var id string
		if om, ok := orgMemberDetails[memLower]; ok {
			if om.Name != "" {
				name = om.Name
			}
			id = om.FormattedID()
		} else if prof, ok := m.ProfileCache[memLower]; ok {
			if prof.Name != "" {
				name = prof.Name
			}
			id = prof.FormattedID()
		}

		var avatarArt string
		if m.AvatarCache != nil {
			avatarArt = m.AvatarCache[memLower]
		}

		memberContribMap[memLower] = &MemberContribution{
			ID:        id,
			Login:     mem,
			Name:      name,
			AvatarArt: avatarArt,
		}
		memberRepoCounts[memLower] = make(map[string]int)
	}

	for _, ev := range m.SquadEvents {
		if ev.Timestamp.Before(cutoff) {
			continue
		}
		actorLower := strings.ToLower(strings.TrimSpace(ev.Actor))
		if !memberMap[actorLower] {
			continue
		}

		repoName := ev.Repo
		if repoName == "" {
			repoName = "unknown"
		}

		if repoMap[repoName] == nil {
			repoMap[repoName] = &RepoActivitySummary{
				RepoName: repoName,
			}
			repoContribMap[repoName] = make(map[string]bool)
		}
		repoSummary := repoMap[repoName]
		repoContribMap[repoName][ev.Actor] = true

		if memberContribMap[actorLower] == nil {
			name := ev.Actor
			var id string
			if om, ok := orgMemberDetails[actorLower]; ok {
				if om.Name != "" {
					name = om.Name
				}
				id = om.FormattedID()
			} else if prof, ok := m.ProfileCache[actorLower]; ok {
				if prof.Name != "" {
					name = prof.Name
				}
				id = prof.FormattedID()
			}
			var avatarArt string
			if m.AvatarCache != nil {
				avatarArt = m.AvatarCache[actorLower]
			}
			memberContribMap[actorLower] = &MemberContribution{
				ID:        id,
				Login:     ev.Actor,
				Name:      name,
				AvatarArt: avatarArt,
			}
			memberRepoCounts[actorLower] = make(map[string]int)
		}
		memContrib := memberContribMap[actorLower]
		memberRepoCounts[actorLower][repoName]++

		actionLower := strings.ToLower(ev.Action)
		detailsLower := strings.ToLower(ev.Details)

		isPush := strings.HasPrefix(actionLower, "push")
		isCommit := strings.Contains(actionLower, "commit") && !isPush
		isReview := strings.Contains(actionLower, "review") || actionLower == "approved"
		isMerge := strings.Contains(actionLower, "merge") || strings.Contains(detailsLower, "merged")

		if isCommit {
			repoSummary.Commits++
			memContrib.Commits++
		}
		if isPush {
			repoSummary.Pushes++
			memContrib.Pushes++
		}
		if isReview {
			repoSummary.Reviews++
			memContrib.Reviews++
		}
		if isMerge {
			repoSummary.Merges++
			memContrib.Merges++
		}

		repoSummary.TotalActions++
		memContrib.TotalActions++

		if ev.Timestamp.After(memContrib.LastActive) {
			memContrib.LastActive = ev.Timestamp
			memContrib.RecentAction = fmt.Sprintf("%s on %s (%s)", ev.Action, ev.Target, formatTimeAgo(ev.Timestamp))
		}
	}

	// Finalize Active Repos
	var activeRepos []RepoActivitySummary
	for repoName, summary := range repoMap {
		for contrib := range repoContribMap[repoName] {
			summary.Contributors = append(summary.Contributors, contrib)
		}
		sort.Strings(summary.Contributors)
		activeRepos = append(activeRepos, *summary)
	}
	// Multi-tiered deterministic tiebreakers to eliminate visual jitter
	sort.Slice(activeRepos, func(i, j int) bool {
		if activeRepos[i].TotalActions != activeRepos[j].TotalActions {
			return activeRepos[i].TotalActions > activeRepos[j].TotalActions
		}
		if activeRepos[i].Commits != activeRepos[j].Commits {
			return activeRepos[i].Commits > activeRepos[j].Commits
		}
		if activeRepos[i].Reviews != activeRepos[j].Reviews {
			return activeRepos[i].Reviews > activeRepos[j].Reviews
		}
		if activeRepos[i].Pushes != activeRepos[j].Pushes {
			return activeRepos[i].Pushes > activeRepos[j].Pushes
		}
		if activeRepos[i].Merges != activeRepos[j].Merges {
			return activeRepos[i].Merges > activeRepos[j].Merges
		}
		return strings.ToLower(activeRepos[i].RepoName) < strings.ToLower(activeRepos[j].RepoName)
	})

	// Finalize Member Contributions
	var memberContributions []MemberContribution
	for _, mem := range squadMembers {
		memLower := strings.ToLower(strings.TrimSpace(mem))
		c := memberContribMap[memLower]
		if c == nil {
			continue
		}

		// Calculate top repos for member
		type repoPair struct {
			repo  string
			count int
		}
		var pairs []repoPair
		for r, count := range memberRepoCounts[memLower] {
			pairs = append(pairs, repoPair{repo: r, count: count})
		}
		sort.Slice(pairs, func(i, j int) bool {
			if pairs[i].count != pairs[j].count {
				return pairs[i].count > pairs[j].count
			}
			return strings.ToLower(pairs[i].repo) < strings.ToLower(pairs[j].repo)
		})

		for i, p := range pairs {
			if i >= 3 {
				break
			}
			c.TopRepos = append(c.TopRepos, fmt.Sprintf("%s (%d)", p.repo, p.count))
		}

		memberContributions = append(memberContributions, *c)
	}

	// Sort member contributions by total actions descending with deterministic tiebreakers
	sort.Slice(memberContributions, func(i, j int) bool {
		if memberContributions[i].TotalActions != memberContributions[j].TotalActions {
			return memberContributions[i].TotalActions > memberContributions[j].TotalActions
		}
		if memberContributions[i].Commits != memberContributions[j].Commits {
			return memberContributions[i].Commits > memberContributions[j].Commits
		}
		if memberContributions[i].Reviews != memberContributions[j].Reviews {
			return memberContributions[i].Reviews > memberContributions[j].Reviews
		}
		if memberContributions[i].Pushes != memberContributions[j].Pushes {
			return memberContributions[i].Pushes > memberContributions[j].Pushes
		}
		if memberContributions[i].Merges != memberContributions[j].Merges {
			return memberContributions[i].Merges > memberContributions[j].Merges
		}
		return strings.ToLower(memberContributions[i].Login) < strings.ToLower(memberContributions[j].Login)
	})

	return SquadDashboardData{
		TimeWindow:          "Last 14 Days (2 Weeks)",
		ActiveRepos:         activeRepos,
		MemberContributions: memberContributions,
	}
}

func (m Model) handleSquadKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	dashboard := m.computeSquadDashboard()
	totalItems := len(dashboard.MemberContributions)
	if totalItems == 0 {
		totalItems = 1
	}

	switch msg.String() {
	case "up", "k":
		if m.SquadCursor > 0 {
			m.SquadCursor--
		}
	case "down", "j":
		if m.SquadCursor < totalItems-1 {
			m.SquadCursor++
		}
	case "home", "g":
		m.SquadCursor = 0
	case "end", "G":
		m.SquadCursor = totalItems - 1
	case "m":
		// Open Manage Squad modal for active squad
		activeSquad := m.Config.GetActiveSquad()
		if activeSquad != nil {
			m.ManagedSquadName = activeSquad.Name
		}
		m.SquadManageMode = SquadModeOverview
		m.ActiveModal = ModalManageSquad
		return m, nil
	case "s":
		// Open Squad switch modal
		m.ActiveModal = ModalSelectSquad
		m.ModalCursor = 0
	case "n", "+":
		// Create new squad directly
		m.ActiveModal = ModalCreateSquad
		m.ModalInput.Reset()
		m.ModalInput.Placeholder = "New squad name (e.g. backend-team)"
		m.ModalInput.Focus()
		return m, textinput.Blink
	case "a":
		// Open Add member mode in Manage Squad modal
		activeSquad := m.Config.GetActiveSquad()
		if activeSquad != nil {
			m.ManagedSquadName = activeSquad.Name
		}
		m.SquadManageMode = SquadModeAdd
		m.ModalCursor = 0
		m.ModalInput.Reset()
		m.ModalInput.Placeholder = "Search members or enter handle..."
		m.ModalInput.Focus()
		m.SelectedSquadMembers = make(map[string]bool)
		m.ActiveModal = ModalManageSquad
		return m, textinput.Blink
	case "d":
		// Open Remove member mode in Manage Squad modal
		activeSquad := m.Config.GetActiveSquad()
		if activeSquad != nil {
			m.ManagedSquadName = activeSquad.Name
		}
		m.SquadManageMode = SquadModeRemove
		m.ModalCursor = 0
		m.SelectedSquadMembers = make(map[string]bool)
		m.ActiveModal = ModalManageSquad
		return m, nil
	case "o", "enter":
		// Open member GitHub profile in browser
		if len(dashboard.MemberContributions) > 0 && m.SquadCursor < len(dashboard.MemberContributions) {
			mem := dashboard.MemberContributions[m.SquadCursor]
			url := fmt.Sprintf("https://github.com/%s", mem.Login)
			_ = OpenBrowser(url)
			return m, func() tea.Msg { return StatusToastMsg("Opened @" + mem.Login + " on GitHub") }
		}
	}
	return m, nil
}

func (m Model) renderSquadView() string {
	activeSquad := m.Config.GetActiveSquad()
	squadName := "None Configured"
	if activeSquad != nil {
		squadName = activeSquad.DisplayName
	}

	dashboard := m.computeSquadDashboard()

	// 1. Dashboard Title & Time Range Banner
	titleBadge := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		Render(fmt.Sprintf("👥 Squad Dashboard: %s", squadName))

	metaRange := MetaBarStyle.Render("  •  Time Range: Last 2 Weeks (14 Days)  •  " + fmt.Sprintf("%d Squad Members", len(dashboard.MemberContributions)))
	banner := "  " + titleBadge + metaRange + "\n"

	if len(dashboard.MemberContributions) == 0 {
		return banner + "\n  No members configured in this squad.\n  Press 'm' to manage squad, 'a' to add members, or 's' to switch squads.\n"
	}

	var sections []string
	sections = append(sections, banner)

	// 2. Most Active Repositories (Squad-Wide, Last 14 Days)
	repoHeader := lipgloss.NewStyle().Bold(true).Foreground(SecondaryColor).Render("  🔥 MOST ACTIVE REPOSITORIES (LAST 14 DAYS)")
	sections = append(sections, repoHeader)

	if len(dashboard.ActiveRepos) == 0 {
		sections = append(sections, lipgloss.NewStyle().Foreground(MutedColor).Render("     No repository activity recorded for squad members in the last 14 days."))
	} else {
		// Render top 5 active repos table
		displayLimit := 5
		if len(dashboard.ActiveRepos) < displayLimit {
			displayLimit = len(dashboard.ActiveRepos)
		}

		repoColW := 24
		statColW := 10
		contribColW := m.Width - repoColW - (statColW * 4) - 22
		if contribColW < 16 {
			contribColW = 16
		}

		repoStyle := lipgloss.NewStyle().Width(repoColW).MaxWidth(repoColW).MaxHeight(1).Inline(true)
		statStyle := lipgloss.NewStyle().Width(statColW).MaxWidth(statColW).MaxHeight(1).Inline(true)
		contribStyle := lipgloss.NewStyle().Width(contribColW).MaxWidth(contribColW).MaxHeight(1).Inline(true)

		tableHead := lipgloss.JoinHorizontal(
			lipgloss.Top,
			"     ",
			TableHeaderStyle.Render(repoStyle.Render("REPOSITORY")),
			TableHeaderStyle.Render(statStyle.Render("COMMITS")),
			TableHeaderStyle.Render(statStyle.Render("PUSHES")),
			TableHeaderStyle.Render(statStyle.Render("REVIEWS")),
			TableHeaderStyle.Render(statStyle.Render("MERGES")),
			TableHeaderStyle.Render(contribStyle.Render("CONTRIBUTORS")),
		)
		var tableRows []string
		tableRows = append(tableRows, tableHead)

		for i := 0; i < displayLimit; i++ {
			r := dashboard.ActiveRepos[i]
			rank := fmt.Sprintf("#%d  ", i+1)

			repoText := r.RepoName
			if len(repoText) > repoColW-2 {
				repoText = repoText[:repoColW-3] + "…"
			}

			contribText := "-"
			if len(r.Contributors) > 0 {
				var handles []string
				for _, c := range r.Contributors {
					handles = append(handles, "@"+c)
				}
				contribText = strings.Join(handles, ", ")
				if len(contribText) > contribColW-2 {
					contribText = contribText[:contribColW-3] + "…"
				}
			}

			row := lipgloss.JoinHorizontal(
				lipgloss.Top,
				lipgloss.NewStyle().Foreground(MutedColor).Render(rank),
				repoStyle.Render(lipgloss.NewStyle().Bold(true).Foreground(HighlightColor).Render(repoText)),
				statStyle.Render(fmt.Sprintf("%d", r.Commits)),
				statStyle.Render(fmt.Sprintf("%d", r.Pushes)),
				statStyle.Render(fmt.Sprintf("%d", r.Reviews)),
				statStyle.Render(fmt.Sprintf("%d", r.Merges)),
				contribStyle.Render(lipgloss.NewStyle().Foreground(SecondaryColor).Render(contribText)),
			)
			tableRows = append(tableRows, "  "+row)
		}
		sections = append(sections, strings.Join(tableRows, "\n"))
	}

	// 3. Squad Member Contribution Panes
	memberHeader := lipgloss.NewStyle().Bold(true).Foreground(SecondaryColor).Render("  👥 SQUAD MEMBER CONTRIBUTIONS (LAST 14 DAYS)")
	sections = append(sections, memberHeader)

	visiblePanes := 4
	if m.Height > 34 {
		visiblePanes = 6
	}

	start := 0
	if m.SquadCursor >= visiblePanes {
		start = m.SquadCursor - visiblePanes + 1
	}
	end := start + visiblePanes
	if end > len(dashboard.MemberContributions) {
		end = len(dashboard.MemberContributions)
	}

	cardWidth := m.Width - 6
	if cardWidth < 40 {
		cardWidth = 40
	}

	var memberPanes []string
	for i := start; i < end; i++ {
		mem := dashboard.MemberContributions[i]

		isSelected := (i == m.SquadCursor)
		cardBorderColor := lipgloss.Color("#303045")
		cursorIndicator := "  "
		nameColor := lipgloss.Color("#FFFFFF")
		if isSelected {
			cardBorderColor = PrimaryColor
			cursorIndicator = "▶ "
			nameColor = SecondaryColor
		}

		nameDisplay := "@" + mem.Login
		if mem.Name != "" && mem.Name != mem.Login {
			nameDisplay = fmt.Sprintf("@%s (%s)", mem.Login, mem.Name)
		}

		cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(SecondaryColor)
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(nameColor)
		idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7F8497"))

		line1 := cursorStyle.Render(cursorIndicator) + titleStyle.Render(nameDisplay)
		if mem.ID != "" {
			line1 += idStyle.Render(fmt.Sprintf("  •  ID: %s", mem.ID))
		}

		// Line 2: Contribution stats
		statCommits := fmt.Sprintf("📦 %d Commits", mem.Commits)
		statPushes := fmt.Sprintf("🔀 %d Pushes", mem.Pushes)
		statReviews := fmt.Sprintf("👁️ %d Reviews", mem.Reviews)
		statMerges := fmt.Sprintf("🚀 %d Merges", mem.Merges)
		statTotal := lipgloss.NewStyle().Bold(true).Foreground(HighlightColor).Render(fmt.Sprintf("(%d Actions)", mem.TotalActions))

		line2 := fmt.Sprintf("  %s   %s   %s   %s   %s", statCommits, statPushes, statReviews, statMerges, statTotal)

		// Line 3: Top repos
		topReposStr := "None"
		if len(mem.TopRepos) > 0 {
			topReposStr = strings.Join(mem.TopRepos, ", ")
		}
		line3 := lipgloss.NewStyle().Foreground(lipgloss.Color("#7F8497")).Render("  Top Repos: ") +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#B3B9D1")).Render(topReposStr)

		// Line 4: Recent highlight
		recentStr := "No activity in last 14 days"
		if mem.RecentAction != "" {
			recentStr = mem.RecentAction
		}
		line4 := lipgloss.NewStyle().Foreground(lipgloss.Color("#7F8497")).Render("  Latest: ") +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#B3B9D1")).Render(recentStr)

		textBlock := lipgloss.JoinVertical(lipgloss.Left, line1, line2, line3, line4)

		avatar := mem.AvatarArt
		if avatar == "" {
			avatar = RenderFallbackAvatar(8, 4)
		}

		cardContent := lipgloss.JoinHorizontal(lipgloss.Top, avatar, "  ", textBlock)
		cardStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cardBorderColor).
			Padding(0, 1).
			MarginLeft(2).
			Width(cardWidth)

		memberPanes = append(memberPanes, cardStyle.Render(cardContent))
	}

	sections = append(sections, strings.Join(memberPanes, "\n"))

	return strings.Join(sections, "\n\n")
}
