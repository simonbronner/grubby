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
	HasAvatar    bool
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

		hasAvatar := false
		if m.RawAvatarCache != nil && len(m.RawAvatarCache[memLower]) > 0 {
			hasAvatar = true
		}

		memberContribMap[memLower] = &MemberContribution{
			ID:        id,
			Login:     mem,
			Name:      name,
			HasAvatar: hasAvatar,
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

		repoName := github.NormalizeRepoName(ev.Repo)
		for _, r := range m.Repos {
			if strings.EqualFold(r.Name, repoName) {
				repoName = r.Name
				break
			}
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
			hasAvatar := false
			if m.RawAvatarCache != nil && len(m.RawAvatarCache[actorLower]) > 0 {
				hasAvatar = true
			}
			memberContribMap[actorLower] = &MemberContribution{
				ID:        id,
				Login:     ev.Actor,
				Name:      name,
				HasAvatar: hasAvatar,
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
	for _, summary := range repoMap {
		if summary.TotalActions == 0 {
			continue
		}
		for contrib := range repoContribMap[summary.RepoName] {
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
		return m, m.emitSquadMemberAvatarsCmd()
	case "down", "j":
		if m.SquadCursor < totalItems-1 {
			m.SquadCursor++
		}
		return m, m.emitSquadMemberAvatarsCmd()
	case "home", "g":
		m.SquadCursor = 0
		return m, m.emitSquadMemberAvatarsCmd()
	case "end", "G":
		m.SquadCursor = totalItems - 1
		return m, m.emitSquadMemberAvatarsCmd()
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
	view, _ := m.renderSquadDashboardView()
	return view
}

func (m Model) emitSquadMemberAvatarsCmd() tea.Cmd {
	return nil
}

func (m Model) renderSquadDashboardView() (string, []AvatarPlacement) {
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
	banner := "  " + titleBadge + metaRange

	if len(dashboard.MemberContributions) == 0 {
		emptyView := banner + "\n\n  No members configured in this squad.\n  Press 'm' to manage squad, 'a' to add members, or 's' to switch squads.\n"
		return emptyView, nil
	}

	var sections []string
	sections = append(sections, banner)

	// 2. Active Repositories & Security Overview Side-by-Side
	displayLimit := 10

	// --- Left Table: Most Active Repositories ---
	var leftLines []string
	repoHeader := lipgloss.NewStyle().Bold(true).Foreground(SecondaryColor).Render("  🔥 MOST ACTIVE REPOSITORIES (LAST 14 DAYS)")
	leftLines = append(leftLines, repoHeader)

	leftWidth := (m.Width - 6) / 2
	if leftWidth < 48 {
		leftWidth = 48
	}
	rightWidth := m.Width - leftWidth - 6
	if rightWidth < 46 {
		rightWidth = 46
	}

	repoColW := 18
	statColW := 6
	contribColW := leftWidth - 4 - repoColW - (statColW * 4) - 4
	if contribColW < 10 {
		contribColW = 10
	}

	repoStyle := lipgloss.NewStyle().Width(repoColW).MaxWidth(repoColW).MaxHeight(1).Inline(true)
	statStyle := lipgloss.NewStyle().Width(statColW).MaxWidth(statColW).MaxHeight(1).Inline(true)
	contribStyle := lipgloss.NewStyle().Width(contribColW).MaxWidth(contribColW).MaxHeight(1).Inline(true)

	tableHead := lipgloss.JoinHorizontal(
		lipgloss.Top,
		"    ",
		TableHeaderStyle.Render(repoStyle.Render("REPOSITORY")),
		TableHeaderStyle.Render(statStyle.Render("CMT")),
		TableHeaderStyle.Render(statStyle.Render("PSH")),
		TableHeaderStyle.Render(statStyle.Render("REV")),
		TableHeaderStyle.Render(statStyle.Render("MRG")),
		TableHeaderStyle.Render(contribStyle.Render("CONTRIB")),
	)
	leftLines = append(leftLines, tableHead)

	if len(dashboard.ActiveRepos) == 0 {
		leftLines = append(leftLines, lipgloss.NewStyle().Foreground(MutedColor).Render("    No repository activity recorded for squad members."))
	} else {
		limit := displayLimit
		if len(dashboard.ActiveRepos) < limit {
			limit = len(dashboard.ActiveRepos)
		}
		for i := 0; i < limit; i++ {
			r := dashboard.ActiveRepos[i]
			rank := fmt.Sprintf("#%-2d", i+1)

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
				contribText = strings.Join(handles, ",")
				if len(contribText) > contribColW-2 {
					contribText = contribText[:contribColW-3] + "…"
				}
			}

			row := lipgloss.JoinHorizontal(
				lipgloss.Top,
				lipgloss.NewStyle().Foreground(MutedColor).Render(rank+" "),
				repoStyle.Render(lipgloss.NewStyle().Bold(true).Foreground(HighlightColor).Render(repoText)),
				statStyle.Render(fmt.Sprintf("%d", r.Commits)),
				statStyle.Render(fmt.Sprintf("%d", r.Pushes)),
				statStyle.Render(fmt.Sprintf("%d", r.Reviews)),
				statStyle.Render(fmt.Sprintf("%d", r.Merges)),
				contribStyle.Render(lipgloss.NewStyle().Foreground(SecondaryColor).Render(contribText)),
			)
			leftLines = append(leftLines, "  "+row)
		}
	}

	// --- Right Table: Security Overview (Squad-Owned Repos) ---
	type SquadSecItem struct {
		RepoName string
		Critical int
		High     int
		Medium   int
		Rest     int
		Total    int
	}

	var squadSecItems []SquadSecItem
	var ownedRepos []string
	if activeSquad != nil && m.Config != nil {
		ownedRepos = m.Config.GetReposForSquad(activeSquad.Name)
	}

	for _, repoName := range ownedRepos {
		cleanName := strings.TrimSpace(repoName)
		if cleanName == "" {
			continue
		}
		sec, ok := m.SecuritySummaries[cleanName]
		if !ok {
			for k, v := range m.SecuritySummaries {
				if strings.EqualFold(k, cleanName) {
					sec = v
					ok = true
					break
				}
			}
		}
		if ok && (sec.Total > 0 || sec.Critical+sec.High+sec.Medium+sec.Low > 0) {
			squadSecItems = append(squadSecItems, SquadSecItem{
				RepoName: cleanName,
				Critical: sec.Critical,
				High:     sec.High,
				Medium:   sec.Medium,
				Rest:     sec.Low,
				Total:    sec.Total,
			})
		}
	}

	sort.Slice(squadSecItems, func(i, j int) bool {
		if squadSecItems[i].Critical != squadSecItems[j].Critical {
			return squadSecItems[i].Critical > squadSecItems[j].Critical
		}
		if squadSecItems[i].High != squadSecItems[j].High {
			return squadSecItems[i].High > squadSecItems[j].High
		}
		if squadSecItems[i].Medium != squadSecItems[j].Medium {
			return squadSecItems[i].Medium > squadSecItems[j].Medium
		}
		if squadSecItems[i].Rest != squadSecItems[j].Rest {
			return squadSecItems[i].Rest > squadSecItems[j].Rest
		}
		return strings.ToLower(squadSecItems[i].RepoName) < strings.ToLower(squadSecItems[j].RepoName)
	})

	var rightLines []string
	secHeader := lipgloss.NewStyle().Bold(true).Foreground(WarningColor).Render("  🛡️ SQUAD SECURITY OVERVIEW")
	rightLines = append(rightLines, secHeader)

	secColW := 18
	secStatW := 6

	secRepoStyle := lipgloss.NewStyle().Width(secColW).MaxWidth(secColW).MaxHeight(1).Inline(true)
	secStatStyle := lipgloss.NewStyle().Width(secStatW).MaxWidth(secStatW).MaxHeight(1).Inline(true)

	secTableHead := lipgloss.JoinHorizontal(
		lipgloss.Top,
		"  ",
		TableHeaderStyle.Render(secRepoStyle.Render("REPOSITORY")),
		TableHeaderStyle.Render(secStatStyle.Render("CRIT")),
		TableHeaderStyle.Render(secStatStyle.Render("HIGH")),
		TableHeaderStyle.Render(secStatStyle.Render("MED")),
		TableHeaderStyle.Render(secStatStyle.Render("REST")),
		TableHeaderStyle.Render(secStatStyle.Render("TOTAL")),
	)
	rightLines = append(rightLines, secTableHead)

	if len(ownedRepos) == 0 {
		rightLines = append(rightLines, lipgloss.NewStyle().Foreground(MutedColor).Render("    No repositories assigned to squad yet.\n    (Press [O] on Repositories tab to assign)"))
	} else if len(squadSecItems) == 0 {
		rightLines = append(rightLines, lipgloss.NewStyle().Foreground(SuccessColor).Bold(true).Render(fmt.Sprintf("    ✓ 0 vulnerabilities across %d squad repos", len(ownedRepos))))
	} else {
		limit := displayLimit
		if len(squadSecItems) < limit {
			limit = len(squadSecItems)
		}
		for i := 0; i < limit; i++ {
			item := squadSecItems[i]
			rName := item.RepoName
			if len(rName) > secColW-2 {
				rName = rName[:secColW-3] + "…"
			}

			var critStr, highStr, medStr, restStr string
			if item.Critical > 0 {
				critStr = lipgloss.NewStyle().Bold(true).Foreground(DangerColor).Render(fmt.Sprintf("%d", item.Critical))
			} else {
				critStr = lipgloss.NewStyle().Foreground(MutedColor).Render("-")
			}
			if item.High > 0 {
				highStr = lipgloss.NewStyle().Bold(true).Foreground(WarningColor).Render(fmt.Sprintf("%d", item.High))
			} else {
				highStr = lipgloss.NewStyle().Foreground(MutedColor).Render("-")
			}
			if item.Medium > 0 {
				medStr = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E5C07B")).Render(fmt.Sprintf("%d", item.Medium))
			} else {
				medStr = lipgloss.NewStyle().Foreground(MutedColor).Render("-")
			}
			if item.Rest > 0 {
				restStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#56B6C2")).Render(fmt.Sprintf("%d", item.Rest))
			} else {
				restStr = lipgloss.NewStyle().Foreground(MutedColor).Render("-")
			}
			totalStr := lipgloss.NewStyle().Bold(true).Foreground(HighlightColor).Render(fmt.Sprintf("%d", item.Total))

			secRow := lipgloss.JoinHorizontal(
				lipgloss.Top,
				secRepoStyle.Render(lipgloss.NewStyle().Bold(true).Foreground(HighlightColor).Render(rName)),
				secStatStyle.Render(critStr),
				secStatStyle.Render(highStr),
				secStatStyle.Render(medStr),
				secStatStyle.Render(restStr),
				secStatStyle.Render(totalStr),
			)
			rightLines = append(rightLines, "  "+secRow)
		}
	}

	leftBlock := strings.Join(leftLines, "\n")
	rightBlock := strings.Join(rightLines, "\n")

	if m.Width >= 100 {
		sections = append(sections, lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, "   ", rightBlock))
	} else {
		sections = append(sections, leftBlock, rightBlock)
	}

	// 3. Squad Member Contribution Panes
	memberHeader := lipgloss.NewStyle().Bold(true).Foreground(SecondaryColor).Render("  👥 SQUAD MEMBER CONTRIBUTIONS (LAST 14 DAYS)")
	sections = append(sections, memberHeader)

	// Calculate starting row for member panes dynamically based on rendered line count
	topBar := m.renderTopBar()
	topBarLines := strings.Count(topBar, "\n") + 1

	preMemberSections := strings.Join(sections, "\n\n") + "\n\n"
	preMemberLines := strings.Count(preMemberSections, "\n")

	currentRow := topBarLines + preMemberLines + 1

	// Dynamically calculate visiblePanes based on remaining terminal height
	// Each member pane is 5 lines (4 lines text + 1 line bottom padding).
	remainingHeight := m.Height - currentRow - 1
	visiblePanes := remainingHeight / 5
	if visiblePanes < 2 {
		visiblePanes = 2
	}
	if visiblePanes > 6 {
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

	var placements []AvatarPlacement
	var memberPanes []string

	for i := start; i < end; i++ {
		mem := dashboard.MemberContributions[i]

		isSelected := (i == m.SquadCursor)
		cursorIndicator := "  "
		nameColor := lipgloss.Color("#FFFFFF")
		if isSelected {
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

		// Render deterministic, multi-color ASCII initials avatar
		avatar := RenderASCIIInitialsAvatar(mem.Name, mem.Login, 8, 4)

		cardContent := lipgloss.JoinHorizontal(lipgloss.Top, avatar, "  ", textBlock)
		contributorStyle := lipgloss.NewStyle().
			MarginLeft(2).
			PaddingBottom(1)

		renderedPane := contributorStyle.Render(cardContent)
		memberPanes = append(memberPanes, renderedPane)

		// Advance currentRow by the exact number of lines in this rendered pane
		linesInPane := strings.Count(renderedPane, "\n") + 1
		currentRow += linesInPane
	}

	sections = append(sections, strings.Join(memberPanes, "\n"))

	return strings.Join(sections, "\n\n"), placements
}
