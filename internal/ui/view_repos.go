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

func (m Model) firstSelectableRepoIndex() int {
	items := m.getRepoTableItems()
	for idx, it := range items {
		if it.Kind == ItemRepo && it.Repo != nil {
			return idx
		}
	}
	return 0
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
	return nil
}

func (m Model) isTierCollapsed(tier storage.RepoTier) bool {
	// Auto-expand tiers when an active text search query ('/') or label filter ('#') is active
	// so that matching repositories across all tiers are visible to the user.
	// (Note: Squad ownership filter 'Shift-S' does NOT auto-expand; Dead and Hidden remain collapsed by default)
	if strings.TrimSpace(m.RepoFilter) != "" || strings.TrimSpace(m.RepoGroupFilter) != "" {
		return false
	}
	if m.CollapsedTiers == nil {
		return tier == storage.TierDead || tier == storage.TierHidden
	}
	collapsed, ok := m.CollapsedTiers[tier]
	if !ok {
		return tier == storage.TierDead || tier == storage.TierHidden
	}
	return collapsed
}

func (m Model) getRepoTableItems(reposOpt ...[]github.Repository) []RepoTableItem {
	var repos []github.Repository
	if len(reposOpt) > 0 && reposOpt[0] != nil {
		repos = reposOpt[0]
	} else {
		repos = m.getDisplayRepos()
	}
	if len(repos) == 0 {
		return nil
	}

	tierOrder := []storage.RepoTier{
		storage.TierHot,
		storage.TierWarm,
		storage.TierCold,
		storage.TierDead,
		storage.TierHidden,
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

		// 3. Squad ownership filter (from Shift-S hotkey)
		if m.SquadFilterActive && m.Config != nil {
			activeSquad := m.Config.GetActiveSquad()
			if activeSquad != nil && !m.Config.IsRepoOwnedBySquad(activeSquad.Name, r.Name) {
				continue
			}
		}

		result = append(result, r)
	}

	if m.RepoSortVulnerabilities {
		type vulnSortKey struct {
			total     int
			critical  int
			high      int
			medium    int
			lowerName string
		}
		keys := make(map[string]vulnSortKey, len(result))
		for _, r := range result {
			sec := m.SecuritySummaries[r.Name]
			keys[r.Name] = vulnSortKey{
				total:     sec.Total,
				critical:  sec.Critical,
				high:      sec.High,
				medium:    sec.Medium,
				lowerName: strings.ToLower(r.Name),
			}
		}

		sort.SliceStable(result, func(i, j int) bool {
			kA := keys[result[i].Name]
			kB := keys[result[j].Name]

			if kA.total != kB.total {
				return kA.total > kB.total
			}
			if kA.critical != kB.critical {
				return kA.critical > kB.critical
			}
			if kA.high != kB.high {
				return kA.high > kB.high
			}
			if kA.medium != kB.medium {
				return kA.medium > kB.medium
			}
			return kA.lowerName < kB.lowerName
		})
	} else {
		tmuxSessions := m.TmuxSessions
		if tmuxSessions == nil {
			tmuxSessions = GetTmuxSessions()
		}

		type repoSortKey struct {
			tierPriority int
			lastAct      time.Time
			lastUpdated  time.Time
			lowerName    string
		}
		keys := make(map[string]repoSortKey, len(result))
		for _, r := range result {
			keys[r.Name] = repoSortKey{
				tierPriority: storage.TierPriority(m.getRepoTier(r.Name)),
				lastAct:      m.getRepoLastInteractedTime(r.Name, tmuxSessions),
				lastUpdated:  r.LastUpdated(),
				lowerName:    strings.ToLower(r.Name),
			}
		}

		sort.SliceStable(result, func(i, j int) bool {
			kA := keys[result[i].Name]
			kB := keys[result[j].Name]

			// 0. Group by Tier priority (Hot=0, Warm=1, Cold=2, Dead=3)
			if kA.tierPriority != kB.tierPriority {
				return kA.tierPriority < kB.tierPriority
			}

			hasActA := !kA.lastAct.IsZero()
			hasActB := !kB.lastAct.IsZero()

			// 1. Repositories opened / interacted with come first, sorted by last interaction descending
			if hasActA && hasActB {
				if !kA.lastAct.Equal(kB.lastAct) {
					return kA.lastAct.After(kB.lastAct)
				}
			} else if hasActA != hasActB {
				return hasActA
			}

			// 2. Then by last updated / pushed descending
			if !kA.lastUpdated.Equal(kB.lastUpdated) {
				return kA.lastUpdated.After(kB.lastUpdated)
			}

			// 3. Alphabetical tiebreaker
			return kA.lowerName < kB.lowerName
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
		m.RepoCursor = m.firstSelectableRepoIndex()
		if m.RepoSortVulnerabilities {
			return m, func() tea.Msg { return StatusToastMsg("Sorted repositories by vulnerabilities (most to least)") }
		}
		return m, func() tea.Msg { return StatusToastMsg("Reset repository sort order to default") }
	case "S":
		// 'S' (Shift-S): filter out all repositories not owned by the active squad
		if m.Config == nil || m.Config.ActiveSquad == "" {
			return m, func() tea.Msg { return StatusToastMsg("No active squad selected") }
		}
		m.SquadFilterActive = !m.SquadFilterActive
		m.RepoCursor = m.firstSelectableRepoIndex()
		if m.SquadFilterActive {
			squadName := m.Config.ActiveSquad
			if s := m.Config.GetActiveSquad(); s != nil && s.DisplayName != "" {
				squadName = s.DisplayName
			}
			return m, func() tea.Msg { return StatusToastMsg(fmt.Sprintf("Filtered repos owned by squad '%s'", squadName)) }
		}
		return m, func() tea.Msg { return StatusToastMsg("Cleared squad repo filter") }
	case "esc":
		// 'esc' clears active filters and sorting
		if m.RepoFilter != "" || m.RepoGroupFilter != "" || m.RepoSortVulnerabilities || m.SquadFilterActive {
			m.RepoFilter = ""
			m.RepoGroupFilter = ""
			m.RepoSortVulnerabilities = false
			m.SquadFilterActive = false
			m.RepoCursor = m.firstSelectableRepoIndex()
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
		case "enter", " ", "c":
			if m.CollapsedTiers == nil {
				m.CollapsedTiers = make(map[storage.RepoTier]bool)
			}
			isCollapsed := m.isTierCollapsed(currentItem.Tier)
			m.CollapsedTiers[currentItem.Tier] = !isCollapsed
			newItems := m.getRepoTableItems()
			if m.RepoCursor >= len(newItems) {
				m.RepoCursor = len(newItems) - 1
			}
			action := "Expanded"
			if !isCollapsed {
				action = "Collapsed"
			}
			return m, func() tea.Msg {
				return StatusToastMsg(fmt.Sprintf("%s %s tier", action, storage.TierName(currentItem.Tier)))
			}
		case "h", "left", "[":
			if m.CollapsedTiers == nil {
				m.CollapsedTiers = make(map[storage.RepoTier]bool)
			}
			m.CollapsedTiers[currentItem.Tier] = true
			newItems := m.getRepoTableItems()
			if m.RepoCursor >= len(newItems) {
				m.RepoCursor = len(newItems) - 1
			}
			return m, func() tea.Msg {
				return StatusToastMsg(fmt.Sprintf("Collapsed %s tier", storage.TierName(currentItem.Tier)))
			}
		case "l", "right", "]":
			if m.CollapsedTiers == nil {
				m.CollapsedTiers = make(map[storage.RepoTier]bool)
			}
			m.CollapsedTiers[currentItem.Tier] = false
			return m, func() tea.Msg {
				return StatusToastMsg(fmt.Sprintf("Expanded %s tier", storage.TierName(currentItem.Tier)))
			}
		case "C":
			// Collapse all tiers
			if m.CollapsedTiers == nil {
				m.CollapsedTiers = make(map[storage.RepoTier]bool)
			}
			for _, t := range []storage.RepoTier{storage.TierHot, storage.TierWarm, storage.TierCold, storage.TierDead, storage.TierHidden} {
				m.CollapsedTiers[t] = true
			}
			newItems := m.getRepoTableItems()
			if m.RepoCursor >= len(newItems) {
				m.RepoCursor = len(newItems) - 1
			}
			return m, func() tea.Msg { return StatusToastMsg("Collapsed all repository tiers") }
		case "E":
			// Expand all tiers
			if m.CollapsedTiers == nil {
				m.CollapsedTiers = make(map[storage.RepoTier]bool)
			}
			for _, t := range []storage.RepoTier{storage.TierHot, storage.TierWarm, storage.TierCold, storage.TierDead, storage.TierHidden} {
				m.CollapsedTiers[t] = false
			}
			return m, func() tea.Msg { return StatusToastMsg("Expanded all repository tiers") }
		default:
			return m, nil
		}
	}

	// 2. Key handling when cursor is on a Repository row
	repo := *currentItem.Repo

	switch msg.String() {
	case "left":
		// Jump cursor to parent tier header
		items := m.getRepoTableItems()
		for idx, it := range items {
			if it.Kind == ItemTierHeader && it.Tier == currentItem.Tier {
				m.RepoCursor = idx
				break
			}
		}
		return m, nil
	case "C":
		// Collapse all tiers
		if m.CollapsedTiers == nil {
			m.CollapsedTiers = make(map[storage.RepoTier]bool)
		}
		for _, t := range []storage.RepoTier{storage.TierHot, storage.TierWarm, storage.TierCold, storage.TierDead, storage.TierHidden} {
			m.CollapsedTiers[t] = true
		}
		newItems := m.getRepoTableItems()
		if m.RepoCursor >= len(newItems) {
			m.RepoCursor = len(newItems) - 1
		}
		return m, func() tea.Msg { return StatusToastMsg("Collapsed all repository tiers") }
	case "E":
		// Expand all tiers
		if m.CollapsedTiers == nil {
			m.CollapsedTiers = make(map[storage.RepoTier]bool)
		}
		for _, t := range []storage.RepoTier{storage.TierHot, storage.TierWarm, storage.TierCold, storage.TierDead, storage.TierHidden} {
			m.CollapsedTiers[t] = false
		}
		return m, func() tea.Msg { return StatusToastMsg("Expanded all repository tiers") }
	case "h":
		// Hide or unhide repository
		isCurrentlyHidden := m.isRepoHidden(repo.Name)
		newHidden := !isCurrentlyHidden
		if m.HiddenRepos == nil {
			m.HiddenRepos = make(map[string]bool)
		}
		if newHidden {
			m.HiddenRepos[strings.ToLower(repo.Name)] = true
		} else {
			delete(m.HiddenRepos, strings.ToLower(repo.Name))
		}
		if m.DB != nil {
			_ = m.DB.SetRepoHidden(repo.Name, newHidden)
		}
		if m.Config != nil {
			_ = m.Config.SetRepoHidden(repo.Name, newHidden)
		}
		m = m.focusRepoCursor(repo.Name)
		actionDesc := "Hidden"
		if !newHidden {
			actionDesc = "Unhid"
		}
		return m, func() tea.Msg {
			return StatusToastMsg(fmt.Sprintf("%s repository '%s'", actionDesc, repo.Name))
		}
	case "[":
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
	case "e":
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
		m.RepoTabsYankLeader = false
		return m, m.fetchSingleRepoCmd(m.CurrentOrg, repo.Name)
	case "O":
		// 'O' (Shift-O) assigns or unassigns ownership of the selected repo to the current active squad
		if m.Config == nil || m.Config.ActiveSquad == "" {
			return m, func() tea.Msg { return StatusToastMsg("No active squad configured") }
		}
		activeSquad := m.Config.ActiveSquad
		nowOwned, err := m.Config.ToggleRepoOwnership(activeSquad, repo.Name)
		if err != nil {
			return m, func() tea.Msg { return StatusToastMsg("Error: " + err.Error()) }
		}
		if m.DB != nil {
			var squadDefs []github.SquadDefinition
			for _, s := range m.Config.Squads {
				squadDefs = append(squadDefs, github.SquadDefinition{
					Name:        s.Name,
					DisplayName: s.DisplayName,
					Members:     s.Members,
					Repos:       s.Repos,
				})
			}
			_ = m.DB.SaveSquads(squadDefs)
		}
		dispName := activeSquad
		if s := m.Config.GetActiveSquad(); s != nil && s.DisplayName != "" {
			dispName = s.DisplayName
		}
		toastText := fmt.Sprintf("✓ Assigned '%s' to squad '%s'", repo.Name, dispName)
		if !nowOwned {
			toastText = fmt.Sprintf("Removed '%s' from squad '%s'", repo.Name, dispName)
		}
		return m, tea.Batch(
			func() tea.Msg { return StatusToastMsg(toastText) },
			m.syncSquadsFileCmd(),
		)
	case "l", "g":
		// Open Labels modal for current repo
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
	repos := m.getDisplayRepos()
	items := m.getRepoTableItems(repos)

	if m.Loading && len(m.Repos) == 0 {
		return fmt.Sprintf("\n  %s Fetching repositories from GitHub...\n", m.Spinner.View())
	}
	if len(items) == 0 {
		if m.RepoFilter != "" || m.RepoGroupFilter != "" || m.SquadFilterActive {
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
		cols.Summary.Render("(Vulns - PRs - Issues)"),
		cols.Git.Render("GIT STATUS"),
		cols.Session.Render("TMUX"),
		cols.Last.Render("LAST PUSHED"),
		cols.Path.Render("LOCAL PATH"),
	)
	renderedHeader := TableHeaderStyle.Render(headerRow)

	var rows []string

	// Active filter indicator if filtering or sorting is on
	var filterBadges []string
	if m.SquadFilterActive && m.Config != nil {
		squadName := m.Config.ActiveSquad
		if s := m.Config.GetActiveSquad(); s != nil && s.DisplayName != "" {
			squadName = s.DisplayName
		}
		filterBadges = append(filterBadges, lipgloss.NewStyle().Foreground(HighlightColor).Bold(true).Render("Squad: "+squadName))
	}
	if m.RepoGroupFilter != "" {
		filterBadges = append(filterBadges, lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true).Render("Label: #"+strings.TrimPrefix(m.RepoGroupFilter, "#")))
	}
	if m.RepoFilter != "" {
		filterBadges = append(filterBadges, lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(fmt.Sprintf("Name: %q", m.RepoFilter)))
	}
	if m.RepoSortVulnerabilities {
		filterBadges = append(filterBadges, lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render("Sort: 🛡️ Most Vulns ↓"))
	}

	hotCount, warmCount, coldCount, deadCount, hiddenCount := m.getRepoTierCounts()
	tierSummary := fmt.Sprintf("🔥 %d Hot  🌞 %d Warm  🧊 %d Cold  💀 %d Dead", hotCount, warmCount, coldCount, deadCount)
	if hiddenCount > 0 {
		tierSummary += fmt.Sprintf("  🙈 %d Hidden", hiddenCount)
	}

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

	prCountByRepo := make(map[string]int, len(m.PRs))
	for _, pr := range m.PRs {
		prCountByRepo[strings.ToLower(github.NormalizeRepoName(pr.Repo))]++
	}

	taskCountByRepo := make(map[string]int, len(m.PersonalTasks)+len(m.SquadTasks))
	for _, t := range m.PersonalTasks {
		taskCountByRepo[strings.ToLower(github.NormalizeRepoName(t.RepoName))]++
	}
	for _, t := range m.SquadTasks {
		taskCountByRepo[strings.ToLower(github.NormalizeRepoName(t.RepoName))]++
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
			} else {
				headerText += "  ── [Space to collapse]"
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
			case storage.TierHidden:
				tierColor = lipgloss.Color("#555566")
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
		isDead := item.Tier == storage.TierDead || item.Tier == storage.TierHidden

		rNorm := strings.ToLower(github.NormalizeRepoName(r.Name))
		prCount := prCountByRepo[rNorm]
		taskCount := taskCountByRepo[rNorm]

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

		// Squad ownership indicator
		isOwned := false
		if m.Config != nil && m.Config.ActiveSquad != "" {
			isOwned = m.Config.IsRepoOwnedBySquad(m.Config.ActiveSquad, r.Name)
		}

		var styledName string
		squadPrefix := ""
		if isOwned {
			squadPrefix = "👥 "
		}
		if isDead {
			styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("#6E7681")).Render(squadPrefix + repoDisplayName)
		} else if isOwned {
			styledName = lipgloss.NewStyle().Foreground(HighlightColor).Bold(true).Render(squadPrefix + repoDisplayName)
		} else {
			styledName = lipgloss.NewStyle().Bold(true).Render(repoDisplayName)
		}

		nameColText := "  " + styledName
		rawNameLen := 2 + len(squadPrefix) + len(repoDisplayName)
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

		// 3. (Vulns - PRs - Issues) column: ( 0 0 0 ) / ( 1 2 3 ) - ( prCount ) - ( taskCount )
		var vulnText string
		sum := m.SecuritySummaries[r.Name]
		if isDead {
			deadStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6E7681"))
			if sum.Total == 0 {
				vulnText = deadStyle.Render("( 0 0 0 )")
			} else {
				medRest := sum.Medium + sum.Low
				vulnText = deadStyle.Render(fmt.Sprintf("( %d %d %d )", sum.Critical, sum.High, medRest))
			}
		} else {
			if sum.Total == 0 {
				vulnText = lipgloss.NewStyle().Foreground(SuccessColor).Render("( 0 0 0 )")
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
				vulnText = fmt.Sprintf("( %s %s %s )", cStr, hStr, mStr)
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
