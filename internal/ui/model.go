package ui

import (
	"context"
	"fmt"
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
)

type ModalType int

const (
	ModalNone ModalType = iota
	ModalManageGroups
	ModalSelectSquad
	ModalAddMember
	ModalRepoTabs
	ModalHelp
)

// Async messages
type DataLoadedMsg struct {
	AuthUser      string
	Repos         []github.Repository
	PRs           []github.PullRequest
	SquadEvents   []github.SquadEvent
	Security      map[string]github.SecurityAlertSummary
	Notifications []github.Notification
	RateLimit     github.RateLimitInfo
	Err           error
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
	SeenNotificationIDs map[string]bool
	GitStatuses         map[string]git.RepoGitStatus
	RateLimit           github.RateLimitInfo
	Loading             bool

	// Navigation & Cursors
	RepoCursor int
	RepoFilter string

	PRCursor int
	PRFilter string

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

	vp := viewport.New(60, 20)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(SecondaryColor)

	m := Model{
		ActiveTab:           TabRepos,
		Config:              cfg,
		GHClient:            gh,
		GitMgr:              gitMgr,
		SearchEng:           search.NewRipgrepEngine(),
		CurrentOrg:          org,
		AuthUser:            org,
		GitStatuses:         make(map[string]git.RepoGitStatus),
		SecuritySummaries:   make(map[string]github.SecurityAlertSummary),
		SeenNotificationIDs: make(map[string]bool),
		Loading:             true,
		SearchInput:         si,
		ModalInput:          mi,
		SearchPreview:       vp,
		SelectedRepoGroups:  make(map[string]bool),
		Spinner:             sp,
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadDataCmd(),
		textinput.Blink,
		m.Spinner.Tick,
	)
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

		return DataLoadedMsg{
			AuthUser:      authUser,
			Repos:         repos,
			PRs:           prs,
			SquadEvents:   events,
			Security:      secAlerts,
			Notifications: notifs,
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

	case DataLoadedMsg:
		m.Loading = false
		if msg.Err != nil {
			m.StatusMsg = fmt.Sprintf("Error loading data: %v", msg.Err)
			m.StatusMsgTimer = time.Now().Add(5 * time.Second)
			return m, nil
		}
		m.Repos = msg.Repos
		m.PRs = msg.PRs
		m.SquadEvents = msg.SquadEvents
		m.SecuritySummaries = msg.Security
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

	case GitStatusUpdatedMsg:
		m.GitStatuses = msg.Statuses

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
		case "tab":
			m.ActiveTab = (m.ActiveTab + 1) % 5
			return m, nil
		case "shift+tab":
			m.ActiveTab = (m.ActiveTab + 4) % 5
			return m, nil
		case "?":
			m.ActiveModal = ModalHelp
			return m, nil
		case "r":
			m.Loading = true
			m.StatusMsg = "Refreshing data from GitHub..."
			m.StatusMsgTimer = time.Now().Add(5 * time.Second)
			return m, tea.Batch(m.loadDataCmd(), m.Spinner.Tick)
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
	if m.Width == 0 {
		return "Initializing Grubby..."
	}

	header := m.renderHeader()
	tabBar := m.renderTabBar()

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
	}

	footer := m.renderFooter()

	baseView := lipgloss.JoinVertical(lipgloss.Left, header, tabBar, body, footer)

	if m.ActiveModal != ModalNone {
		return m.renderModalOverlay(baseView)
	}

	return baseView
}

func (m Model) renderHeader() string {
	// 1. Hobbit avatar mascot replacing "GRUBBY"
	hobbitBadge := HeaderStyle.Render(" ᘛ( ᵔ ᴗ ᵔ*)ᘚ ")

	// 2. Organization with icon
	orgItem := MetaBarStyle.Render("🏢 " + m.CurrentOrg)

	// 3. Squad with icon
	watchedSquad := "None"
	if s := m.Config.GetActiveSquad(); s != nil {
		watchedSquad = s.DisplayName
	}
	squadItem := MetaBarStyle.Render("👥 " + watchedSquad)

	// 4. Summary of total open pull requests
	prCount := len(m.PRs)
	var prItem string
	if prCount > 0 {
		prItem = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(fmt.Sprintf("⑂ %d Open PRs", prCount))
	} else {
		prItem = lipgloss.NewStyle().Foreground(MutedColor).Render("⑂ 0 PRs")
	}

	// 5. Summary of total outstanding security issues across all repos
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
		secItem = lipgloss.NewStyle().Foreground(SuccessColor).Render("🛡️ 0 Alerts [✓ clean]")
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

	// 6. Logged-in GitHub user and rate quota
	user := m.AuthUser
	if user == "" {
		user = m.CurrentOrg
	}

	var userItem string
	if m.Width >= 110 {
		userItem = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render("👤 @"+user) +
			" " + MetaBarStyle.Render(fmt.Sprintf("(%d/%d API)", m.RateLimit.Remaining, m.RateLimit.Limit))
	} else {
		userItem = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render("👤 @" + user)
	}

	// Assemble header items evenly distributed across line 1
	var items []string
	if m.Width >= 100 {
		items = []string{hobbitBadge, orgItem, squadItem, prItem, secItem, userItem}
	} else {
		items = []string{hobbitBadge, orgItem, prItem, secItem, userItem}
	}

	return distributeEvenly(items, m.Width)
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

func (m Model) renderTabBar() string {
	unreadNotifs := 0
	for _, n := range m.Notifications {
		if n.Unread {
			unreadNotifs++
		}
	}

	tabs := []string{
		"1: Repositories",
		fmt.Sprintf("2: Pull Requests (%d)", len(m.PRs)),
		"3: Squad Activity",
		"4: Code Search",
		fmt.Sprintf("5: Notifications (%d)", unreadNotifs),
	}

	var renderedTabs []string
	for i, t := range tabs {
		if TabType(i) == m.ActiveTab {
			renderedTabs = append(renderedTabs, ActiveTabStyle.Render(t))
		} else {
			renderedTabs = append(renderedTabs, InactiveTabStyle.Render(t))
		}
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	return TabBarBorder.Render(bar)
}

func (m Model) renderFooter() string {
	var help string
	switch m.ActiveTab {
	case TabRepos:
		help = "[Enter] Open Web  [o] Submenu  [S] Security Tab  [g] Groups  [c] Clone  [e] Editor (nvim)  [t] Shell  [u] Fast-forward  [?] Help  [q] Quit"
	case TabPRs:
		help = "[o] Browser  [p] Trigger Copilot Review  [c] Checkout Branch  [r] Refresh  [?] Help  [q] Quit"
	case TabSquad:
		help = "[s] Switch Squad  [a] Add Member  [d] Remove Member  [r] Refresh  [?] Help  [q] Quit"
	case TabSearch:
		help = "[/] Query  [j/k] Nodes  [h/l/Space] Toggle  [Tab] Switch Viewport  [Enter] Open Editor  [?] Help"
	case TabNotifications:
		help = "[Enter / o] Open in Browser  [m] Mark Read  [M] Mark All Read  [r] Refresh  [?] Help  [q] Quit"
	}

	toast := ""
	if m.StatusMsg != "" && time.Now().Before(m.StatusMsgTimer) {
		toast = "  |  " + lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render(m.StatusMsg)
	}

	content := FooterStyle.Render(help + toast)
	return content
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
