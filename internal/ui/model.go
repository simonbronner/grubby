package ui

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"grubby/internal/config"
	"grubby/internal/git"
	"grubby/internal/github"
	"grubby/internal/search"
)

type TabType int

const (
	TabRepos TabType = iota
	TabPRs
	TabSquad
	TabSearch
	TabNotifications
	TabStatus
)

type ModalType int

const (
	ModalNone ModalType = iota
	ModalManageGroups
	ModalSelectSquad
	ModalAddMember
	ModalRepoTabs
	ModalHelp
	ModalFilterGroup
	ModalFilterRepoName
	ModalFilterPR
	ModalCreateSquad
	ModalManageSquad
)

// Async messages
type RepoClonedMsg struct {
	RepoName  string
	TargetDir string
	Status    git.RepoGitStatus
	Err       error
}

type DataLoadedMsg struct {
	AuthUser      string
	Repos         []github.Repository
	PRs           []github.PullRequest
	SquadEvents   []github.SquadEvent
	Security      map[string]github.SecurityAlertSummary
	Notifications []github.Notification
	OrgMembers    []github.OrgMember
	RateLimit     github.RateLimitInfo
	Err           error
}

type GitHubStatusLoadedMsg struct {
	Summary *github.StatusSummary
	Err     error
}

type GitStatusUpdatedMsg struct {
	Statuses map[string]git.RepoGitStatus
}

type SearchResultsMsg struct {
	Results []search.RepoMatches
	Query   string
	Err     error
}

type StatusToastMsg string

type AvatarLoadedMsg struct {
	Login   string
	RawData []byte
}

type ProfilesLoadedMsg struct {
	Profiles map[string]github.OrgMember
}

type AutoRefreshTickMsg time.Time

func autoRefreshTickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return AutoRefreshTickMsg(t)
	})
}

type Model struct {
	ActiveTab TabType
	Config    *config.Config
	GHClient  *github.Client
	GitMgr    *git.WorkspaceManager
	SearchEng search.Engine

	// Data
	CurrentOrg          string
	AuthUser            string
	Repos               []github.Repository
	PRs                 []github.PullRequest
	SquadEvents         []github.SquadEvent
	SecuritySummaries   map[string]github.SecurityAlertSummary
	Notifications       []github.Notification
	OrgMembers          []github.OrgMember
	RawAvatarCache      map[string][]byte            // login -> raw image bytes for iTerm2 native graphics
	ProfileCache        map[string]github.OrgMember  // login -> enriched profile (id, name, avatarUrl)
	SeenNotificationIDs map[string]bool
	GitStatuses         map[string]git.RepoGitStatus
	RateLimit           github.RateLimitInfo
	Loading             bool
	InitialDataLoaded   bool
	LastRefreshTime     time.Time

	// GitHub Status
	GitHubStatus          *github.StatusSummary
	LastStatusCheck       time.Time
	PrevComponentStatuses map[string]string
	StatusCursor          int

	// Squad Management
	ManagedSquadName     string
	SquadManageMode      int
	SelectedSquadMembers map[string]bool

	// Navigation & Cursors
	RepoCursor              int
	RepoFilter              string
	RepoGroupFilter         string
	RepoSortVulnerabilities bool
	RepoFilterInput         textinput.Model

	PRCursor      int
	PRFilter      string
	PRFilterInput textinput.Model

	SquadCursor int

	NotifCursor int

	// Search View
	SearchInput       textinput.Model
	SearchResults     []search.RepoMatches
	SearchTree        []*search.SearchNode
	SearchFlatNodes   []*search.SearchNode
	SearchCursor      int
	SearchPreview     viewport.Model
	SearchFocusedPane int // 0: Tree, 1: Preview
	Searching         bool

	// Modals
	ActiveModal        ModalType
	ModalCursor        int
	ModalInput         textinput.Model
	SelectedRepoGroups map[string]bool // groupName -> inGroup

	// Layout
	Width  int
	Height int

	// Status / Toast
	StatusMsg      string
	StatusMsgTimer time.Time
	Spinner        spinner.Model
}

func InitialModel(cfg *config.Config, gh *github.Client, gitMgr *git.WorkspaceManager, org string) Model {
	si := textinput.New()
	si.Placeholder = "Type search query (e.g. func ValidateToken) and press Enter..."
	si.Prompt = "🔍 "
	si.CharLimit = 100
	si.Width = 60

	mi := textinput.New()
	mi.Placeholder = "Type here..."
	mi.CharLimit = 50

	rfi := textinput.New()
	rfi.Placeholder = "Type repository name substring..."
	rfi.Prompt = "🔍 "
	rfi.CharLimit = 50
	rfi.Width = 40

	prfi := textinput.New()
	prfi.Placeholder = "Type PR title, repo, or author..."
	prfi.Prompt = "🔍 "
	prfi.CharLimit = 50
	prfi.Width = 40

	vp := viewport.New(60, 20)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(SecondaryColor)

	m := Model{
		ActiveTab:             TabRepos,
		Config:                cfg,
		GHClient:              gh,
		GitMgr:                gitMgr,
		SearchEng:             search.NewRipgrepEngine(),
		CurrentOrg:            org,
		AuthUser:              org,
		GitStatuses:           make(map[string]git.RepoGitStatus),
		SecuritySummaries:     make(map[string]github.SecurityAlertSummary),
		SeenNotificationIDs:   make(map[string]bool),
		RawAvatarCache:        make(map[string][]byte),
		ProfileCache:          make(map[string]github.OrgMember),
		Loading:               true,
		InitialDataLoaded:     false,
		LastRefreshTime:       time.Now(),
		LastStatusCheck:       time.Now().Add(-2 * time.Minute), // trigger status fetch right away
		PrevComponentStatuses: make(map[string]string),
		SelectedSquadMembers:  make(map[string]bool),
		SearchInput:           si,
		ModalInput:            mi,
		RepoFilterInput:       rfi,
		PRFilterInput:         prfi,
		SearchPreview:         vp,
		SelectedRepoGroups:    make(map[string]bool),
		Spinner:               sp,
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadDataCmd(),
		m.fetchGitHubStatusCmd(),
		textinput.Blink,
		m.Spinner.Tick,
		autoRefreshTickCmd(),
	)
}

func (m Model) fetchGitHubStatusCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		summary, err := m.GHClient.FetchStatus(ctx)
		return GitHubStatusLoadedMsg{
			Summary: summary,
			Err:     err,
		}
	}
}

func (m Model) loadDataCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		authUser, _ := m.GHClient.GetAuthenticatedUser(ctx)
		if authUser == "" {
			authUser = m.CurrentOrg
		}

		repos, prs, commitEvents, secAlerts, err := m.GHClient.FetchRepositoriesAndPRs(ctx, m.CurrentOrg)
		if err != nil {
			return DataLoadedMsg{Err: err}
		}

		// Fetch squad activity
		var members []string
		if activeSquad := m.Config.GetActiveSquad(); activeSquad != nil && len(activeSquad.Members) > 0 {
			members = activeSquad.Members
		} else {
			// Auto-seed squad with current user/org
			_ = m.Config.AddMemberToSquad("my-squad", m.CurrentOrg)
			_ = m.Config.SetActiveSquad("my-squad")
			members = []string{m.CurrentOrg}
		}
		events, _ := m.GHClient.FetchSquadActivity(ctx, members, commitEvents)

		// Fetch notifications
		notifs, _ := m.GHClient.FetchNotifications(ctx)

		// Fetch organization members and merge with unique actors
		orgMembers, _ := m.GHClient.FetchOrgMembers(ctx, m.CurrentOrg)
		seenMembers := make(map[string]bool)
		for _, om := range orgMembers {
			seenMembers[strings.ToLower(om.Login)] = true
		}
		for _, ce := range commitEvents {
			if ce.Actor != "" && !seenMembers[strings.ToLower(ce.Actor)] {
				seenMembers[strings.ToLower(ce.Actor)] = true
				orgMembers = append(orgMembers, github.OrgMember{Login: ce.Actor, Name: ce.Actor})
			}
		}
		for _, pr := range prs {
			if pr.Author != "" && !seenMembers[strings.ToLower(pr.Author)] {
				seenMembers[strings.ToLower(pr.Author)] = true
				orgMembers = append(orgMembers, github.OrgMember{Login: pr.Author, Name: pr.Author})
			}
		}

		// Enrich squad member profiles with ID, full name, and avatar URL
		if len(members) > 0 {
			if squadProfiles, err := m.GHClient.FetchUserProfiles(ctx, members); err == nil {
				for _, sp := range squadProfiles {
					spLower := strings.ToLower(sp.Login)
					if !seenMembers[spLower] {
						seenMembers[spLower] = true
						orgMembers = append(orgMembers, sp)
					} else {
						for i, om := range orgMembers {
							if strings.EqualFold(om.Login, sp.Login) {
								if sp.Name != "" {
									orgMembers[i].Name = sp.Name
								}
								if sp.FormattedID() != "" {
									orgMembers[i].ID = sp.ID
									orgMembers[i].DatabaseID = sp.DatabaseID
								}
								if sp.AvatarURL != "" {
									orgMembers[i].AvatarURL = sp.AvatarURL
								}
							}
						}
					}
				}
			}
		}

		return DataLoadedMsg{
			AuthUser:      authUser,
			Repos:         repos,
			PRs:           prs,
			SquadEvents:   events,
			Security:      secAlerts,
			Notifications: notifs,
			OrgMembers:    orgMembers,
			RateLimit:     m.GHClient.RateLimit(),
		}
	}
}

func (m Model) scanWorkspacesCmd(repoNames []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		statuses := m.GitMgr.ScanWorkspaces(ctx, repoNames)
		return GitStatusUpdatedMsg{Statuses: statuses}
	}
}

func (m Model) runSearchCmd(query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		paths := m.GitMgr.ListClonedPaths()
		results, err := m.SearchEng.Search(ctx, query, paths)
		return SearchResultsMsg{
			Results: results,
			Query:   query,
			Err:     err,
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.SearchPreview.Width = (m.Width / 2) - 4
		m.SearchPreview.Height = m.Height - 12
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd

	case AutoRefreshTickMsg:
		var cmds []tea.Cmd
		cmds = append(cmds, autoRefreshTickCmd())

		// Poll GitHub Status every 60 seconds
		if time.Since(m.LastStatusCheck) >= 60*time.Second {
			m.LastStatusCheck = time.Now()
			cmds = append(cmds, m.fetchGitHubStatusCmd())
		}

		// Auto-refresh main GitHub data every 5 minutes
		if time.Since(m.LastRefreshTime) >= 5*time.Minute && !m.Loading {
			m.Loading = true
			m.LastRefreshTime = time.Now()
			m.StatusMsg = "Auto-refreshing data from GitHub..."
			m.StatusMsgTimer = time.Now().Add(4 * time.Second)
			cmds = append(cmds, m.loadDataCmd(), m.Spinner.Tick)
		}
		return m, tea.Batch(cmds...)

	case GitHubStatusLoadedMsg:
		if msg.Err == nil && msg.Summary != nil {
			m.GitHubStatus = msg.Summary

			// Detect service status changes
			if len(m.PrevComponentStatuses) > 0 {
				for _, comp := range msg.Summary.Components {
					if comp.Group || comp.Name == "Visit www.githubstatus.com for more information" {
						continue
					}
					prev, known := m.PrevComponentStatuses[comp.Name]
					if known && prev != comp.Status {
						if comp.Status != "operational" {
							title := fmt.Sprintf("GitHub Alert: %s Degraded", comp.Name)
							desc := fmt.Sprintf("Service is now %s (%s)", comp.Status, comp.Description)
							SendNativeNotification("Grubby • GitHub Status Alert", title, desc)
							m.StatusMsg = "⚠️ " + title + " (" + comp.Status + ")"
							m.StatusMsgTimer = time.Now().Add(8 * time.Second)
							m.Notifications = append([]github.Notification{{
								ID:         fmt.Sprintf("gh-status-%s-%d", comp.ID, time.Now().Unix()),
								Repository: "githubstatus.com",
								Title:      title + ": " + comp.Status,
								Type:       "ServiceAlert",
								Unread:     true,
								UpdatedAt:  time.Now(),
								URL:        "https://www.githubstatus.com",
							}}, m.Notifications...)
						} else if prev != "operational" {
							title := fmt.Sprintf("GitHub Resolved: %s Operational", comp.Name)
							SendNativeNotification("Grubby • GitHub Status Restored", title, "Service restored to operational")
							m.StatusMsg = "✅ " + title
							m.StatusMsgTimer = time.Now().Add(8 * time.Second)
							m.Notifications = append([]github.Notification{{
								ID:         fmt.Sprintf("gh-status-res-%s-%d", comp.ID, time.Now().Unix()),
								Repository: "githubstatus.com",
								Title:      title,
								Type:       "ServiceRestored",
								Unread:     true,
								UpdatedAt:  time.Now(),
								URL:        "https://www.githubstatus.com",
							}}, m.Notifications...)
						}
					}
				}
			}

			// Store latest statuses
			if m.PrevComponentStatuses == nil {
				m.PrevComponentStatuses = make(map[string]string)
			}
			for _, comp := range msg.Summary.Components {
				m.PrevComponentStatuses[comp.Name] = comp.Status
			}
		}
		return m, nil

	case DataLoadedMsg:
		m.Loading = false
		m.InitialDataLoaded = true
		m.LastRefreshTime = time.Now()
		if msg.Err != nil {
			m.StatusMsg = fmt.Sprintf("Error loading data: %v", msg.Err)
			m.StatusMsgTimer = time.Now().Add(5 * time.Second)
			return m, nil
		}
		m.Repos = msg.Repos
		m.PRs = msg.PRs
		m.SquadEvents = msg.SquadEvents
		m.SecuritySummaries = msg.Security
		m.OrgMembers = msg.OrgMembers
		m.RateLimit = msg.RateLimit
		if msg.AuthUser != "" {
			m.AuthUser = msg.AuthUser
		}

		// Process notifications and alert for new unread items
		var newNotifs []github.Notification
		for _, n := range msg.Notifications {
			if n.Unread && !m.SeenNotificationIDs[n.ID] {
				newNotifs = append(newNotifs, n)
				m.SeenNotificationIDs[n.ID] = true
			}
		}
		m.Notifications = msg.Notifications

		if len(newNotifs) > 0 {
			if len(newNotifs) == 1 {
				n := newNotifs[0]
				SendNativeNotification("Grubby • GitHub Notification", n.Repository, fmt.Sprintf("[%s] %s", n.Type, n.Title))
			} else {
				first := newNotifs[0]
				SendNativeNotification("Grubby • GitHub Notifications", fmt.Sprintf("%d New Notifications", len(newNotifs)), fmt.Sprintf("%s: %s", first.Repository, first.Title))
			}

			// Automatically mark popped-up notifications as read
			for _, n := range newNotifs {
				for i := range m.Notifications {
					if m.Notifications[i].ID == n.ID {
						m.Notifications[i].Unread = false
					}
				}
				threadID := n.ID
				cmds = append(cmds, func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					_ = m.GHClient.MarkNotificationRead(ctx, threadID)
					return nil
				})
			}
		}

		m.StatusMsg = "Refreshed data from GitHub"
		m.StatusMsgTimer = time.Now().Add(4 * time.Second)

		// Trigger workspace git scan once repos are known
		var names []string
		for _, r := range m.Repos {
			names = append(names, r.Name)
		}
		cmds = append(cmds, m.scanWorkspacesCmd(names))

		// Seed ProfileCache
		for _, om := range m.OrgMembers {
			m.ProfileCache[strings.ToLower(om.Login)] = om
		}

		// Trigger avatar downloads for active squad members
		var squadLogins []string
		if activeSquad := m.Config.GetActiveSquad(); activeSquad != nil {
			squadLogins = activeSquad.Members
		}
		if len(squadLogins) > 0 && m.GHClient != nil {
			cmds = append(cmds, m.fetchSquadAvatarsCmd(squadLogins))
		}

	case AvatarLoadedMsg:
		if len(msg.RawData) > 0 {
			if m.RawAvatarCache == nil {
				m.RawAvatarCache = make(map[string][]byte)
			}
			m.RawAvatarCache[strings.ToLower(msg.Login)] = msg.RawData
			return m, EmitITerm2ImageCmd(msg.RawData, 8, 4)
		}
		return m, nil

	case ProfilesLoadedMsg:
		if m.ProfileCache == nil {
			m.ProfileCache = make(map[string]github.OrgMember)
		}
		for _, p := range msg.Profiles {
			m.ProfileCache[strings.ToLower(p.Login)] = p
		}
		return m, nil

	case GitStatusUpdatedMsg:
		m.GitStatuses = msg.Statuses

	case RepoClonedMsg:
		if msg.Err != nil {
			m.StatusMsg = "Clone failed: " + msg.Err.Error()
			m.StatusMsgTimer = time.Now().Add(5 * time.Second)
			return m, nil
		}
		m.GitStatuses[msg.RepoName] = msg.Status
		collapsed := config.CollapseHome(msg.TargetDir)
		m.StatusMsg = "Successfully cloned " + msg.RepoName + " to " + collapsed
		m.StatusMsgTimer = time.Now().Add(4 * time.Second)
		return m, nil

	case SearchResultsMsg:
		m.Searching = false
		if msg.Err != nil {
			m.StatusMsg = fmt.Sprintf("Search error: %v", msg.Err)
			return m, nil
		}
		m.SearchResults = msg.Results
		m.SearchTree = search.BuildTree(m.SearchResults)
		m.SearchFlatNodes = search.FlattenVisibleNodes(m.SearchTree)
		m.SearchCursor = 0
		m.updateSearchPreview()

	case StatusToastMsg:
		m.StatusMsg = string(msg)
		m.StatusMsgTimer = time.Now().Add(4 * time.Second)

	case tea.KeyMsg:
		// Modal Key Handling
		if m.ActiveModal != ModalNone {
			return m.handleModalKeys(msg)
		}

		// When search input is focused, route keystrokes directly to it
		if m.ActiveTab == TabSearch && m.SearchInput.Focused() {
			return m.handleSearchKeys(msg)
		}

		// Global Key Handling (only active when not typing in an input field)
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			return m, tea.Quit
		case "1":
			m.ActiveTab = TabRepos
			return m, nil
		case "2":
			m.ActiveTab = TabPRs
			return m, nil
		case "3":
			m.ActiveTab = TabSquad
			return m, nil
		case "4":
			m.ActiveTab = TabSearch
			return m, nil
		case "5":
			m.ActiveTab = TabNotifications
			return m, nil
		case "6":
			m.ActiveTab = TabStatus
			return m, nil
		case "tab":
			m.ActiveTab = (m.ActiveTab + 1) % 6
			return m, nil
		case "shift+tab":
			m.ActiveTab = (m.ActiveTab + 5) % 6
			return m, nil
		case "?":
			m.ActiveModal = ModalHelp
			return m, nil
		case "r":
			m.Loading = true
			m.LastRefreshTime = time.Now()
			m.LastStatusCheck = time.Now()
			m.StatusMsg = "Refreshing data from GitHub..."
			m.StatusMsgTimer = time.Now().Add(5 * time.Second)
			return m, tea.Batch(m.loadDataCmd(), m.fetchGitHubStatusCmd(), m.Spinner.Tick)
		}

		// Tab-Specific Key Handling
		switch m.ActiveTab {
		case TabRepos:
			return m.handleReposKeys(msg)
		case TabPRs:
			return m.handlePRsKeys(msg)
		case TabSquad:
			return m.handleSquadKeys(msg)
		case TabSearch:
			return m.handleSearchKeys(msg)
		case TabNotifications:
			return m.handleNotificationsKeys(msg)
		case TabStatus:
			return m.handleStatusKeys(msg)
		}
	}

	if m.ActiveTab == TabSearch && m.SearchInput.Focused() {
		var cmd tea.Cmd
		m.SearchInput, cmd = m.SearchInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.Width == 0 || !m.InitialDataLoaded {
		return m.renderSplashScreen()
	}

	topBar := m.renderTopBar()

	var body string
	switch m.ActiveTab {
	case TabRepos:
		body = m.renderReposView()
	case TabPRs:
		body = m.renderPRsView()
	case TabSquad:
		body = m.renderSquadView()
	case TabSearch:
		body = m.renderSearchView()
	case TabNotifications:
		body = m.renderNotificationsView()
	case TabStatus:
		body = m.renderStatusView()
	}

	footer := m.renderFooter()

	baseView := lipgloss.JoinVertical(lipgloss.Left, topBar, body, footer)

	if m.ActiveModal != ModalNone {
		return m.renderModalOverlay(baseView)
	}

	return baseView
}

func (m Model) renderTopBar() string {
	// 1. Hobbit avatar mascot on the left
	hobbitBadge := HeaderStyle.Render(" ᘛ( ᵔ ᴗ ᵔ*)ᘚ ")

	// 2. Tabs immediately to the right of the mascot
	unreadNotifs := 0
	for _, n := range m.Notifications {
		if n.Unread {
			unreadNotifs++
		}
	}

	// Status dot conveying health: Healthy (green ●) vs Degraded (amber ▲) vs Unavailable (red ✖)
	var statusDot string
	if m.GitHubStatus == nil {
		statusDot = StatusUnknownDot.Render("○")
	} else {
		switch m.GitHubStatus.Status.Indicator {
		case "none", "":
			statusDot = StatusHealthyDot.Render("●")
		case "minor":
			statusDot = StatusDegradedDot.Render("▲")
		case "major", "critical":
			statusDot = StatusOutageDot.Render("✖")
		default:
			statusDot = StatusDegradedDot.Render("▲")
		}
	}

	var tabs []string
	if m.Width >= 140 {
		tabs = []string{
			"1: Repositories",
			fmt.Sprintf("2: Pull Requests (%d)", len(m.PRs)),
			"3: Squad Activity",
			"4: Code Search",
			fmt.Sprintf("5: Notifications (%d)", unreadNotifs),
			fmt.Sprintf("6: Status %s", statusDot),
		}
	} else if m.Width >= 115 {
		prCountStr := ""
		if len(m.PRs) > 0 {
			prCountStr = fmt.Sprintf(" (%d)", len(m.PRs))
		}
		notifCountStr := ""
		if unreadNotifs > 0 {
			notifCountStr = fmt.Sprintf(" (%d)", unreadNotifs)
		}
		tabs = []string{
			"1: Repos",
			fmt.Sprintf("2: PRs%s", prCountStr),
			"3: Squad",
			"4: Search",
			fmt.Sprintf("5: Notifs%s", notifCountStr),
			fmt.Sprintf("6: Status %s", statusDot),
		}
	} else {
		tabs = []string{
			"1: Repos", "2: PRs", "3: Squad", "4: Search", "5: Notifs", fmt.Sprintf("6: %s", statusDot),
		}
	}

	var renderedTabs []string
	for i, t := range tabs {
		if TabType(i) == m.ActiveTab {
			renderedTabs = append(renderedTabs, ActiveTabStyle.Render(t))
		} else {
			renderedTabs = append(renderedTabs, InactiveTabStyle.Render(t))
		}
	}

	tabsRow := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	leftPart := lipgloss.JoinHorizontal(lipgloss.Top, hobbitBadge, "  ", tabsRow)

	// 3. Security vulnerabilities indicator placed between tabs and refresh
	totalCrit := 0
	totalHigh := 0
	totalMedRest := 0
	totalAlerts := 0
	for _, sum := range m.SecuritySummaries {
		totalCrit += sum.Critical
		totalHigh += sum.High
		totalMedRest += (sum.Medium + sum.Low)
		totalAlerts += sum.Total
	}

	var secItem string
	if totalAlerts == 0 {
		secItem = lipgloss.NewStyle().Foreground(SuccessColor).Render("🛡️ Clean")
	} else {
		var parts []string
		if totalCrit > 0 {
			parts = append(parts, lipgloss.NewStyle().Foreground(DangerColor).Bold(true).Render(fmt.Sprintf("%dc", totalCrit)))
		}
		if totalHigh > 0 {
			parts = append(parts, lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render(fmt.Sprintf("%dh", totalHigh)))
		}
		if totalMedRest > 0 {
			parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B")).Render(fmt.Sprintf("%dm+", totalMedRest)))
		}
		secItem = "🛡️ " + strings.Join(parts, " ")
	}

	// 4. Consolidated Refresh + Requests Per Hour on the far right
	reqsLastHour := 0
	if m.RateLimit.Limit > 0 && m.RateLimit.Limit >= m.RateLimit.Remaining {
		reqsLastHour = m.RateLimit.Limit - m.RateLimit.Remaining
	}

	var reqsStr string
	if m.Width >= 130 {
		reqsStr = fmt.Sprintf("%d reqs/h", reqsLastHour)
	} else {
		reqsStr = fmt.Sprintf("%d/h", reqsLastHour)
	}

	var refreshStr string
	if m.Loading {
		refreshStr = "⟳ Refreshing..."
	} else if !m.LastRefreshTime.IsZero() {
		sinceLast := time.Since(m.LastRefreshTime)
		const autoInterval = 5 * time.Minute
		untilNext := autoInterval - sinceLast
		if untilNext < 0 {
			untilNext = 0
		}

		var sinceStr string
		if sinceLast < time.Minute {
			sinceStr = fmt.Sprintf("%ds", int(sinceLast.Seconds()))
		} else {
			sinceStr = fmt.Sprintf("%dm", int(sinceLast.Minutes()))
		}

		mins := int(untilNext.Minutes())
		secs := int(untilNext.Seconds()) % 60
		refreshStr = fmt.Sprintf("⟳ %s (%d:%02d)", sinceStr, mins, secs)
	} else {
		refreshStr = "⟳ Ready"
	}

	rightText := lipgloss.NewStyle().Foreground(MutedColor).Render(fmt.Sprintf("%s • %s", refreshStr, reqsStr))

	leftWidth := lipgloss.Width(leftPart)
	secWidth := lipgloss.Width(secItem)
	rightWidth := lipgloss.Width(rightText)

	var fullLine string
	if m.Width >= leftWidth+secWidth+rightWidth+2 {
		// Three-way layout: LeftPart ... SecItem ... RightText
		avail := m.Width - leftWidth - secWidth - rightWidth
		gap1 := avail / 2
		if gap1 < 1 {
			gap1 = 1
		}
		gap2 := avail - gap1
		if gap2 < 1 {
			gap2 = 1
		}
		fullLine = leftPart + strings.Repeat(" ", gap1) + secItem + strings.Repeat(" ", gap2) + rightText
	} else if m.Width >= leftWidth+secWidth+2 {
		padding := m.Width - leftWidth - secWidth
		fullLine = leftPart + strings.Repeat(" ", padding) + secItem
	} else if m.Width >= leftWidth+rightWidth+2 {
		padding := m.Width - leftWidth - rightWidth
		fullLine = leftPart + strings.Repeat(" ", padding) + rightText
	} else if rightWidth > 0 && m.Width >= leftWidth+rightWidth {
		fullLine = leftPart + rightText
	} else {
		fullLine = leftPart
	}

	return TabBarBorder.Render(fullLine)
}

func (m Model) renderFooter() string {
	// Single-line efficient footer
	// Left: Contextual hotkeys & toast message
	var help string
	switch m.ActiveTab {
	case TabRepos:
		if m.RepoFilter != "" || m.RepoGroupFilter != "" || m.RepoSortVulnerabilities {
			help = "[v] View Group  [f] Filter  [s] Sort Vulns  [Esc] Clear  [Enter] Web  [o] Submenu  [g] Groups  [c] Clone  [e] Session  [t] Shell  [E] Editor  [?] Help"
		} else {
			help = "[v] View Group  [f] Filter  [s] Sort Vulns  [Enter] Web  [o] Submenu  [g] Groups  [c] Clone  [e] Session  [t] Shell  [E] Editor  [u] Sync  [?] Help"
		}
	case TabPRs:
		if m.PRFilter != "" {
			help = "[f] Filter  [Esc] Clear Filter  [o] Browser  [p] Copilot Review  [c] Checkout Branch  [?] Help"
		} else {
			help = "[f] Filter  [o] Browser  [p] Copilot Review  [c] Checkout Branch  [r] Refresh  [?] Help"
		}
	case TabSquad:
		help = "[m] Manage Squad  [s] Switch Squad  [n] New Squad  [r] Refresh  [?] Help"
	case TabSearch:
		help = "[/] Query  [j/k] Nodes  [h/l/Space] Toggle  [Tab] Switch Viewport  [Enter] Open Editor  [?] Help"
	case TabNotifications:
		help = "[Enter / o] Open in Browser  [m] Mark Read  [M] Mark All Read  [r] Refresh  [?] Help  [q] Quit"
	case TabStatus:
		help = "[r] Refresh Status  [o / Enter] Statuspage  [j/k] Navigate  [?] Help  [q] Quit"
	}

	toast := ""
	if m.StatusMsg != "" && time.Now().Before(m.StatusMsgTimer) {
		toast = "  |  " + lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render(m.StatusMsg)
	}

	leftFooter := FooterStyle.Render(help + toast)

	// Right: Consolidated user identity: '{username} @ {org} in {squad}'
	user := m.AuthUser
	if user == "" {
		user = m.CurrentOrg
	}
	watchedSquad := "None"
	if s := m.Config.GetActiveSquad(); s != nil {
		watchedSquad = s.DisplayName
	}
	identityText := fmt.Sprintf("%s @ %s in %s", user, m.CurrentOrg, watchedSquad)
	rightFooter := lipgloss.NewStyle().Foreground(lipgloss.Color("#7A7A8C")).Render(identityText)

	leftWidth := lipgloss.Width(leftFooter)
	rightWidth := lipgloss.Width(rightFooter)

	var footerLine string
	if m.Width >= leftWidth+rightWidth+2 {
		padding := m.Width - leftWidth - rightWidth
		footerLine = leftFooter + strings.Repeat(" ", padding) + rightFooter
	} else {
		footerLine = leftFooter
	}

	return footerLine
}

func distributeEvenly(items []string, totalWidth int) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0]
	}

	totalItemWidth := 0
	for _, it := range items {
		totalItemWidth += lipgloss.Width(it)
	}

	availSpace := totalWidth - totalItemWidth
	numGaps := len(items) - 1

	if availSpace < numGaps {
		return strings.Join(items, " ")
	}

	baseGap := availSpace / numGaps
	extra := availSpace % numGaps

	var b strings.Builder
	for i, it := range items {
		b.WriteString(it)
		if i < numGaps {
			gap := baseGap
			if i < extra {
				gap++
			}
			b.WriteString(strings.Repeat(" ", gap))
		}
	}
	return b.String()
}

// Helpers for opening in browser or editor

func OpenBrowser(url string) error {
	return OpenInDedicatedBrowser(url)
}

func OpenInITermNvim(dir string, filePath string, lineNum int) error {
	if _, err := os.Stat("/Applications/iTerm.app"); err == nil {
		var nvimCmd string
		if filePath != "" && lineNum > 0 {
			nvimCmd = fmt.Sprintf("cd %q && nvim +%d %q", dir, lineNum, filePath)
		} else if filePath != "" {
			nvimCmd = fmt.Sprintf("cd %q && nvim %q", dir, filePath)
		} else {
			nvimCmd = fmt.Sprintf("cd %q && nvim .", dir)
		}

		script := fmt.Sprintf(`
tell application "iTerm"
	activate
	try
		tell current window
			create tab with default profile
			tell current session
				write text %q
			end tell
		end tell
	on error
		create window with default profile
		tell current session of current window
			write text %q
		end tell
	end try
end tell`, nvimCmd, nvimCmd)

		cmd := exec.Command("osascript", "-e", script)
		return cmd.Run()
	}

	// Fallback if iTerm is not installed
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nvim"
	}
	if filePath != "" && lineNum > 0 {
		return exec.Command(editor, fmt.Sprintf("+%d", lineNum), filePath).Start()
	}
	return exec.Command(editor, dir).Start()
}

func OpenInEditor(editor, path string, lineNum int) error {
	dir := path
	file := ""
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		dir = filepath.Dir(path)
		file = path
	}
	return OpenInITermNvim(dir, file, lineNum)
}

func OpenInShell(dir string) error {
	if _, err := os.Stat("/Applications/iTerm.app"); err == nil {
		script := fmt.Sprintf(`
tell application "iTerm"
	activate
	try
		tell current window
			create tab with default profile
			tell current session
				write text "cd %q && clear"
			end tell
		end tell
	on error
		create window with default profile
		tell current session of current window
			write text "cd %q && clear"
		end tell
	end try
end tell`, dir, dir)
		cmd := exec.Command("osascript", "-e", script)
		if err := cmd.Run(); err == nil {
			return nil
		}
		return exec.Command("open", "-a", "iTerm", dir).Start()
	}
	return exec.Command("open", "-a", "Terminal", dir).Start()
}

func (m Model) fetchSquadAvatarsCmd(logins []string) tea.Cmd {
	if m.GHClient == nil {
		return nil
	}
	var cmds []tea.Cmd
	for _, login := range logins {
		lClean := strings.TrimSpace(login)
		if lClean == "" {
			continue
		}
		lLower := strings.ToLower(lClean)
		if m.RawAvatarCache != nil {
			if _, exists := m.RawAvatarCache[lLower]; exists {
				continue
			}
		}

		ghClient := m.GHClient
		profile := m.ProfileCache[lLower]

		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()

			avatarURL := profile.AvatarURL
			if avatarURL == "" {
				avatarURL = fmt.Sprintf("https://github.com/%s.png?size=64", url.PathEscape(lClean))
			}

			data, err := ghClient.DownloadAvatar(ctx, avatarURL)
			if err != nil || len(data) == 0 {
				return nil
			}

			return AvatarLoadedMsg{
				Login:   lLower,
				RawData: data,
			}
		})
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}
