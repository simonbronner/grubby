package ui

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"grubby/internal/config"
	"grubby/internal/git"
	"grubby/internal/github"
	"grubby/internal/search"
	"grubby/internal/storage"
)

type TabType int

const (
	TabRepos TabType = iota
	TabPRs
	TabMyTasks
	TabSquadTasks
	TabSquad
	TabSearch
	TabStatus

	// Backward compatibility alias
	TabTasks = TabMyTasks
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
	ModalCreateTask
	ModalEditTask
	ModalTaskNotes
	ModalCompletedTasks
	ModalCreatePRTask
	ModalAssignSquadTask

	// Aliases for Labels
	ModalFilterLabel  = ModalFilterGroup
	ModalManageLabels = ModalManageGroups
)

type SplashDoneMsg struct{}

type RepoActionType int

const (
	RepoActionNone RepoActionType = iota
	RepoActionSession
	RepoActionEditor
	RepoActionShell
)

// Async messages
type RepoClonedMsg struct {
	RepoName   string
	TargetDir  string
	Status     git.RepoGitStatus
	Err        error
	PostAction RepoActionType
}

type SingleRepoLoadedMsg struct {
	Repo     *github.Repository
	Events   []github.SquadEvent
	Security *github.SecurityAlertSummary
	Err      error
}

type DataLoadedMsg struct {
	AuthUser          string
	Repos             []github.Repository
	PRs               []github.PullRequest
	SquadEvents       []github.SquadEvent
	PersonalTasks     []github.Task
	PersonalDoneTasks []github.Task
	SquadTasks        []github.Task
	SquadDoneTasks    []github.Task
	Security          map[string]github.SecurityAlertSummary
	Notifications     []github.Notification
	OrgMembers        []github.OrgMember
	RateLimit         github.RateLimitInfo
	AutoClosedCount   int
	Err               error
}

type TasksLoadedMsg struct {
	PersonalTasks     []github.Task
	PersonalDoneTasks []github.Task
	SquadTasks        []github.Task
	SquadDoneTasks    []github.Task
	AutoClosedCount   int
	Err               error
}

type TaskCreatedMsg struct {
	Task *github.Task
	Err  error
}

type TaskUpdatedMsg struct {
	Task *github.Task
	Err  error
}

type TaskClosedMsg struct {
	TaskID   int64
	Number   int
	RepoName string
	IsSquad  bool
	Err      error
}

type TaskReopenedMsg struct {
	TaskID   int64
	Number   int
	RepoName string
	IsSquad  bool
	Err      error
}

type TaskPriorityUpdatedMsg struct {
	TaskID   int64
	Priority github.TaskPriority
	Err      error
}

type TaskAssigneesUpdatedMsg struct {
	TaskID    int64
	Assignees []string
	Err       error
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

	// SQLite & Performance Caching
	DB               *storage.DB
	Debouncer        *storage.TaskDebouncer
	RepoTiers        map[string]storage.RepoTier
	RepoInteractions map[string]time.Time
	RequestsMade     int64
	RequestsSaved    int64

	// GitHub Status
	GitHubStatus          *github.StatusSummary
	LastStatusCheck       time.Time
	PrevComponentStatuses map[string]string
	StatusCursor          int

	// Splash Screen & Tier Tree
	SplashDismissed  bool
	CollapsedTiers   map[storage.RepoTier]bool
	RepoLabels       map[int64][]string
	RepoLabelsByName map[string][]string
	RepoLabelColors  map[string]string
	HiddenRepos      map[string]bool

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
	TmuxSessions            map[string]TmuxSessionInfo

	PRCursor      int
	PRFilter      string
	PRFilterInput textinput.Model

	SquadCursor int

	// Tasks
	PersonalTasks          []github.Task
	PersonalDoneTasks      []github.Task
	SquadTasks             []github.Task
	SquadDoneTasks         []github.Task
	ActiveTaskSubtab       TaskSubtab
	TaskShowDone           bool
	TaskCursor             int
	TaskCreateTarget       TaskTarget
	TaskCreateTitleInput   textinput.Model
	TaskCreateBodyInput    textarea.Model
	TaskCreatePriority     github.TaskPriority
	TaskCreateAssignee     string
	TaskCreateFocusedField int

	// Inline Task Management
	InlineAdding           bool
	InlineNewTitleInput    textinput.Model
	InlineNewNotes         string
	InlineNewPriority      github.TaskPriority
	InlineNewAssignee      string

	InlineEditing          bool
	InlineEditTaskID       int64
	InlineEditTitleInput   textinput.Model

	// Notes Modal
	TaskNotesTarget        *github.Task
	TaskNotesIsNewTask     bool
	TaskNotesInput         textarea.Model

	// Completed Tasks Modal
	CompletedTasksCursor   int

	// PR Review Task Modal
	PRTaskTarget           github.PullRequest
	PRTaskAssignMode       int // 0: Squad (all except me), 1: Individual, 2: Myself
	PRTaskAssignee         string
	PRTaskAssigneeIndex    int
	PRTaskPriority         github.TaskPriority
	PRTaskFocusedField     int // 0: Assignee list, 1: Priority, 2: Buttons
	PRTaskActiveButton     int // 0: Create, 1: Cancel
	PRTaskCandidates       []string
	PRTaskMemberCursor     int
	PRTaskSelectedMembers  map[string]bool

	// Assign Squad Task Modal ([A] in squad mode)
	AssignTaskTarget          *github.Task
	AssignTaskCandidates      []string
	AssignTaskMemberCursor    int
	AssignTaskSelectedMembers map[string]bool
	AssignTaskFocusedField    int // 0: Member list, 1: Buttons
	AssignTaskActiveButton    int // 0: Save, 1: Cancel

	// Task Details & Editing
	TaskEditTarget         github.Task
	TaskEditTitleInput     textinput.Model
	TaskEditBodyInput      textarea.Model
	TaskEditPriority       github.TaskPriority
	TaskEditAssignee       string
	TaskEditFocusedField   int

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

func InitialModel(cfg *config.Config, gh *github.Client, gitMgr *git.WorkspaceManager, org string, dbOpt ...*storage.DB) Model {
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

	tcbi := textarea.New()
	tcbi.Placeholder = "Optional task notes/context"
	tcbi.ShowLineNumbers = false
	tcbi.SetHeight(5)
	tcbi.SetWidth(60)

	tebi := textarea.New()
	tebi.Placeholder = "Task notes/details..."
	tebi.ShowLineNumbers = false
	tebi.SetHeight(5)
	tebi.SetWidth(60)

	inti := textinput.New()
	inti.Placeholder = "What needs to be done? (Enter to create, Esc to cancel)"
	inti.Prompt = ""
	inti.CharLimit = 120

	ieti := textinput.New()
	ieti.Placeholder = "Task title... (Enter to save, Esc to cancel)"
	ieti.Prompt = ""
	ieti.CharLimit = 120

	tni := textarea.New()
	tni.Placeholder = "Task notes/details (Enter for newline, Esc to save)..."
	tni.ShowLineNumbers = false
	tni.SetHeight(8)
	tni.SetWidth(65)

	vp := viewport.New(60, 20)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(SecondaryColor)

	var db *storage.DB
	if len(dbOpt) > 0 && dbOpt[0] != nil {
		db = dbOpt[0]
	}

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
		RepoTiers:             make(map[string]storage.RepoTier),
		RepoInteractions:      make(map[string]time.Time),
		DB:                    db,
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
		TmuxSessions:          GetTmuxSessions(),
		TaskCreateBodyInput:   tcbi,
		TaskEditBodyInput:     tebi,
		InlineNewTitleInput:   inti,
		InlineEditTitleInput:  ieti,
		TaskNotesInput:        tni,
		SplashDismissed:       false,
		CollapsedTiers:        map[storage.RepoTier]bool{storage.TierDead: true, storage.TierHidden: true},
		RepoLabels:            make(map[int64][]string),
		RepoLabelsByName:      make(map[string][]string),
		RepoLabelColors:       make(map[string]string),
		HiddenRepos:           make(map[string]bool),
	}

	if cfg != nil {
		for _, r := range cfg.HiddenRepos {
			m.HiddenRepos[strings.ToLower(strings.TrimSpace(r))] = true
		}
	}

	if db != nil {
		for k, v := range db.GetAllHiddenRepos() {
			if v {
				m.HiddenRepos[k] = true
			}
		}
		if labels, err := db.GetRepoLabelsForSquad(cfg.ActiveSquad); err == nil && len(labels) > 0 {
			m.RepoLabels = labels
		}
		if nameLabels, err := db.GetRepoLabelsByNameForSquad(cfg.ActiveSquad); err == nil && len(nameLabels) > 0 {
			m.RepoLabelsByName = nameLabels
		}
		if colors, err := db.GetLabelColors(cfg.ActiveSquad); err == nil && len(colors) > 0 {
			m.RepoLabelColors = colors
		}
		made, saved, _ := db.GetRequestStats()
		if gh != nil {
			gh.SetInitialRequestStats(made, saved)
		}
		m.RequestsMade = made
		m.RequestsSaved = saved

		// Eagerly read SQLite data for instantaneous UI render
		if cachedRepos, err := db.GetRepositories(); err == nil && len(cachedRepos) > 0 {
			m.Repos = cachedRepos
			m.RepoTiers = db.GetAllRepoTiers()
			m.RepoInteractions = db.GetAllRepoInteractions()
			if cachedPRs, err := db.GetPullRequests(); err == nil {
				m.PRs = cachedPRs
			}
			if pTasks, err := db.GetTasks(false, "open"); err == nil {
				m.PersonalTasks = pTasks
			}
			if pDone, err := db.GetTasks(false, "closed"); err == nil {
				m.PersonalDoneTasks = pDone
			}
			if sTasks, err := db.GetTasks(true, "open"); err == nil {
				m.SquadTasks = sTasks
			}
			if sDone, err := db.GetTasks(true, "closed"); err == nil {
				m.SquadDoneTasks = sDone
			}
			if events, err := db.GetSquadEvents(100); err == nil {
				m.SquadEvents = events
			}
			if members, err := db.GetOrgMembers(); err == nil {
				m.OrgMembers = members
				for _, mem := range members {
					m.ProfileCache[strings.ToLower(mem.Login)] = mem
				}
			}
			if sec, err := db.GetSecuritySummaries(); err == nil {
				m.SecuritySummaries = sec
			}

			m.InitialDataLoaded = true
			m.Loading = false
		}

		m.Debouncer = storage.NewTaskDebouncer(
			1500*time.Millisecond,
			func() {
				if gh != nil {
					gh.RecordRequestSaved(1)
				}
				_ = db.IncrementRequestsSaved(1)
			},
			func(job *storage.TaskDebounceJob, err error) {
				if err == nil {
					_ = db.UpdateTaskSyncStatus(job.TaskID, "synced")
				}
			},
		)
	}

	if m.RepoCursor == 0 && len(m.Repos) > 0 {
		m.RepoCursor = m.firstSelectableRepoIndex()
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
		m.emitSplashImageCmd(),
		tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return SplashDoneMsg{}
		}),
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

		// Fetch Tasks for Personal and Squad
		allPersonal, _ := m.GHClient.FetchTasks(ctx, authUser, "tasks", false)
		personalTasks, personalDoneTasks, autoClosedPersonal := m.GHClient.CheckAndAutoClosePRTasks(ctx, allPersonal)

		squadName := "default-squad"
		if m.Config != nil && m.Config.ActiveSquad != "" {
			squadName = m.Config.ActiveSquad
		}
		squadRepo := fmt.Sprintf("%s-tasks", squadName)
		orgTarget := m.CurrentOrg
		if orgTarget == "" {
			orgTarget = authUser
		}
		allSquad, _ := m.GHClient.FetchTasks(ctx, orgTarget, squadRepo, true)
		squadTasks, squadDoneTasks, autoClosedSquad := m.GHClient.CheckAndAutoClosePRTasks(ctx, allSquad)

		if m.DB != nil {
			_ = m.DB.SaveRepositories(repos, m.CurrentOrg)
			_ = m.DB.SavePullRequests(prs)
			_ = m.DB.UpdateRepoTiers(prs, events)
			_ = m.DB.SaveTasks(append(append([]github.Task{}, personalTasks...), personalDoneTasks...), false, "synced")
			_ = m.DB.SaveTasks(append(append([]github.Task{}, squadTasks...), squadDoneTasks...), true, "synced")
			_ = m.DB.SaveSquadEvents(events)
			_ = m.DB.SaveOrgMembers(orgMembers)
			_ = m.DB.SaveSecuritySummaries(secAlerts)
		}

		return DataLoadedMsg{
			AuthUser:          authUser,
			Repos:             repos,
			PRs:               prs,
			SquadEvents:       events,
			PersonalTasks:     personalTasks,
			PersonalDoneTasks: personalDoneTasks,
			SquadTasks:        squadTasks,
			SquadDoneTasks:    squadDoneTasks,
			AutoClosedCount:   autoClosedPersonal + autoClosedSquad,
			Security:          secAlerts,
			Notifications:     notifs,
			OrgMembers:        orgMembers,
			RateLimit:         m.GHClient.RateLimit(),
		}
	}
}

func (m Model) fetchSingleRepoCmd(owner, repoName string) tea.Cmd {
	return func() tea.Msg {
		if m.GHClient == nil || owner == "" || repoName == "" {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		repo, events, secAlert, err := m.GHClient.FetchSingleRepo(ctx, owner, repoName)
		return SingleRepoLoadedMsg{
			Repo:     repo,
			Events:   events,
			Security: secAlert,
			Err:      err,
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

func (m Model) loadTasksCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		user := m.AuthUser
		if user == "" {
			user = m.CurrentOrg
		}
		allPersonal, _ := m.GHClient.FetchTasks(ctx, user, "tasks", false)
		personalTasks, personalDoneTasks, autoClosedPersonal := m.GHClient.CheckAndAutoClosePRTasks(ctx, allPersonal)

		squadName := "default-squad"
		if m.Config != nil && m.Config.ActiveSquad != "" {
			squadName = m.Config.ActiveSquad
		}
		squadRepo := fmt.Sprintf("%s-tasks", squadName)
		org := m.CurrentOrg
		if org == "" {
			org = user
		}
		allSquad, _ := m.GHClient.FetchTasks(ctx, org, squadRepo, true)
		squadTasks, squadDoneTasks, autoClosedSquad := m.GHClient.CheckAndAutoClosePRTasks(ctx, allSquad)

		return TasksLoadedMsg{
			PersonalTasks:     personalTasks,
			PersonalDoneTasks: personalDoneTasks,
			SquadTasks:        squadTasks,
			SquadDoneTasks:    squadDoneTasks,
			AutoClosedCount:   autoClosedPersonal + autoClosedSquad,
		}
	}
}

func (m Model) createTaskCmd(owner, repo, title, body string, priority github.TaskPriority, assignees []string, isSquad bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		task, err := m.GHClient.CreateTask(ctx, owner, repo, title, body, priority, assignees, isSquad)
		return TaskCreatedMsg{Task: task, Err: err}
	}
}

func (m Model) createPRTaskCmd(owner, repo, title, body string, priority github.TaskPriority, labels []string, assignees []string, isSquad bool, pr github.PullRequest) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		task, err := m.GHClient.CreateTaskWithLabels(ctx, owner, repo, title, body, priority, labels, assignees, isSquad)
		if err == nil && task != nil && pr.URL != "" {
			if prOwner, prRepo, prNum, ok := github.ParsePRURL(pr.URL); ok {
				_ = m.GHClient.LinkPRToIssue(ctx, prOwner, prRepo, prNum, task.URL)
			}
		}
		return TaskCreatedMsg{Task: task, Err: err}
	}
}

func (m Model) getSquadAssigneeOptions() []string {
	var list []string
	seen := make(map[string]bool)

	addCandidate := func(candidate string) {
		clean := strings.TrimSpace(candidate)
		lower := strings.ToLower(clean)
		if lower == "" || seen[lower] {
			return
		}
		seen[lower] = true
		list = append(list, clean)
	}

	// 1. Active Squad Members
	if m.Config != nil {
		if active := m.Config.GetActiveSquad(); active != nil {
			for _, mem := range active.Members {
				addCandidate(mem)
			}
		}
		// 2. Members from all configured squads
		for _, s := range m.Config.Squads {
			for _, mem := range s.Members {
				addCandidate(mem)
			}
		}
	}

	// 3. Organization Members
	for _, om := range m.OrgMembers {
		addCandidate(om.Login)
	}

	// 4. Contributors from PRs & Squad Events
	for _, pr := range m.PRs {
		addCandidate(pr.Author)
	}
	for _, ev := range m.SquadEvents {
		addCandidate(ev.Actor)
	}

	// 5. Current Authenticated User
	if m.AuthUser != "" {
		addCandidate(m.AuthUser)
	}

	return list
}

func (m Model) getSquadReviewCandidates(prAuthor string) []string {
	var list []string
	me := strings.ToLower(strings.TrimSpace(m.AuthUser))
	author := strings.ToLower(strings.TrimSpace(prAuthor))
	seen := make(map[string]bool)

	for _, cand := range m.getSquadAssigneeOptions() {
		clean := strings.TrimSpace(cand)
		lower := strings.ToLower(clean)
		if lower == "" || lower == me || lower == author || seen[lower] {
			continue
		}
		seen[lower] = true
		list = append(list, clean)
	}
	return list
}

func (m Model) getSquadIndividualCandidates() []string {
	return m.getSquadAssigneeOptions()
}

func (m Model) updateTaskInSlices(updated github.Task) Model {
	for i := range m.PersonalTasks {
		if m.PersonalTasks[i].ID == updated.ID {
			m.PersonalTasks[i] = updated
		}
	}
	for i := range m.PersonalDoneTasks {
		if m.PersonalDoneTasks[i].ID == updated.ID {
			m.PersonalDoneTasks[i] = updated
		}
	}
	for i := range m.SquadTasks {
		if m.SquadTasks[i].ID == updated.ID {
			m.SquadTasks[i] = updated
		}
	}
	for i := range m.SquadDoneTasks {
		if m.SquadDoneTasks[i].ID == updated.ID {
			m.SquadDoneTasks[i] = updated
		}
	}
	return m
}

func (m Model) updateTaskCmd(target github.Task, title, body string, priority github.TaskPriority, assignees []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		task, err := m.GHClient.UpdateTask(ctx, target.RepoOwner, target.RepoName, target.Number, title, body, priority, assignees, target.Labels, target.IsSquadTask)
		return TaskUpdatedMsg{Task: task, Err: err}
	}
}

func (m Model) closeTaskCmd(target github.Task) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := m.GHClient.CloseTask(ctx, target.RepoOwner, target.RepoName, target.Number)
		return TaskClosedMsg{
			TaskID:   target.ID,
			Number:   target.Number,
			RepoName: target.RepoName,
			IsSquad:  target.IsSquadTask,
			Err:      err,
		}
	}
}

func (m Model) reopenTaskCmd(target github.Task) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := m.GHClient.ReopenTask(ctx, target.RepoOwner, target.RepoName, target.Number)
		return TaskReopenedMsg{
			TaskID:   target.ID,
			Number:   target.Number,
			RepoName: target.RepoName,
			IsSquad:  target.IsSquadTask,
			Err:      err,
		}
	}
}

func (m Model) updateTaskPriorityCmd(target github.Task, newPriority github.TaskPriority) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := m.GHClient.UpdateTaskPriority(ctx, target.RepoOwner, target.RepoName, target.Number, newPriority, target.Labels)
		return TaskPriorityUpdatedMsg{
			TaskID:   target.ID,
			Priority: newPriority,
			Err:      err,
		}
	}
}

func (m Model) updateTaskAssigneesCmd(target github.Task, assignees []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := m.GHClient.UpdateTaskAssignees(ctx, target.RepoOwner, target.RepoName, target.Number, assignees)
		return TaskAssigneesUpdatedMsg{
			TaskID:    target.ID,
			Assignees: assignees,
			Err:       err,
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
		if !m.SplashDismissed {
			return m, m.emitSplashImageCmd()
		}
		if m.ActiveTab == TabSquad {
			return m, m.emitSquadMemberAvatarsCmd()
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		if !m.SplashDismissed {
			return m, tea.Batch(cmd, m.emitSplashImageCmd())
		}
		return m, cmd

	case SplashDoneMsg:
		m.SplashDismissed = true
		return m, nil

	case AutoRefreshTickMsg:
		m.TmuxSessions = GetTmuxSessions()
		var cmds []tea.Cmd
		cmds = append(cmds, autoRefreshTickCmd())

		if m.ActiveTab == TabSquad {
			cmds = append(cmds, m.emitSquadMemberAvatarsCmd())
		}

		// Poll GitHub Status every 60 seconds
		if time.Since(m.LastStatusCheck) >= 60*time.Second {
			m.LastStatusCheck = time.Now()
			cmds = append(cmds, m.fetchGitHubStatusCmd())
		}

		// Auto-refresh main GitHub data every 5 minutes
		if time.Since(m.LastRefreshTime) >= 5*time.Minute && !m.Loading {
			m.Loading = true
			m.LastRefreshTime = time.Now()
			inactiveSkipped := 0
			if m.DB != nil {
				_, _, cold, dead, err := m.DB.GetTierCounts()
				if err == nil && (cold+dead) > 0 {
					inactiveSkipped = cold + dead
					if m.GHClient != nil {
						m.GHClient.RecordRequestSaved(int64(inactiveSkipped))
					}
					_ = m.DB.IncrementRequestsSaved(int64(inactiveSkipped))
				}
			}
			if inactiveSkipped > 0 {
				m.StatusMsg = fmt.Sprintf("Auto-refreshing Hot & Warm repos (%d cold/dead skipped)...", inactiveSkipped)
			} else {
				m.StatusMsg = "Auto-refreshing data from GitHub..."
			}
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

	case SingleRepoLoadedMsg:
		if msg.Err == nil && msg.Repo != nil {
			found := false
			for i := range m.Repos {
				if strings.EqualFold(m.Repos[i].Name, msg.Repo.Name) {
					m.Repos[i] = *msg.Repo
					found = true
					break
				}
			}
			if !found {
				m.Repos = append(m.Repos, *msg.Repo)
			}

			tier := storage.CalculateRepoTier(*msg.Repo, nil, msg.Events)
			if m.RepoTiers == nil {
				m.RepoTiers = make(map[string]storage.RepoTier)
			}
			m.RepoTiers[strings.ToLower(msg.Repo.Name)] = tier

			if m.SecuritySummaries != nil && msg.Security != nil {
				m.SecuritySummaries[msg.Repo.Name] = *msg.Security
			}
			if len(msg.Events) > 0 {
				m.SquadEvents = append(msg.Events, m.SquadEvents...)
			}
			if m.DB != nil {
				_ = m.DB.SaveRepositories([]github.Repository{*msg.Repo}, m.CurrentOrg)
				_ = m.DB.SetRepoTier(msg.Repo.Name, tier)
				_ = m.DB.TouchRepo(msg.Repo.Name)
				if msg.Security != nil {
					_ = m.DB.SaveSecuritySummaries(map[string]github.SecurityAlertSummary{msg.Repo.Name: *msg.Security})
				}
			}
			m.StatusMsg = fmt.Sprintf("✓ Refreshed %s (%s)", msg.Repo.Name, storage.TierBadgeText(tier))
			m.StatusMsgTimer = time.Now().Add(3 * time.Second)
		}
		return m, nil

	case PRMergedMsg:
		if msg.Err != nil {
			m.StatusMsg = fmt.Sprintf("Failed to merge PR #%d: %v", msg.Number, msg.Err)
			m.StatusMsgTimer = time.Now().Add(5 * time.Second)
			return m, nil
		}
		m.StatusMsg = fmt.Sprintf("✓ Successfully merged PR #%d in %s", msg.Number, msg.Repo)
		m.StatusMsgTimer = time.Now().Add(5 * time.Second)
		if m.DB != nil {
			_ = m.DB.DeletePullRequest(msg.Repo, msg.Number)
			_ = m.DB.TouchRepo(msg.Repo)
		}
		// Remove merged PR from in-memory PRs
		var remainingPRs []github.PullRequest
		for _, p := range m.PRs {
			if strings.EqualFold(p.Repo, msg.Repo) && p.Number == msg.Number {
				continue
			}
			remainingPRs = append(remainingPRs, p)
		}
		m.PRs = remainingPRs
		displayPRs := m.getDisplayPRs()
		if m.PRCursor >= len(displayPRs) && m.PRCursor > 0 {
			m.PRCursor = len(displayPRs) - 1
		}

		// Auto-close any matching PR review tasks in personal or squad tasks
		for i := len(m.PersonalTasks) - 1; i >= 0; i-- {
			task := m.PersonalTasks[i]
			if task.IsPRReview() {
				_, pRepo, pNum, ok := github.ParsePRURL(task.PRURL())
				if ok && strings.EqualFold(pRepo, msg.Repo) && pNum == msg.Number {
					task.State = "closed"
					now := time.Now()
					task.ClosedAt = &now
					m.PersonalDoneTasks = append([]github.Task{task}, m.PersonalDoneTasks...)
					m.PersonalTasks = append(m.PersonalTasks[:i], m.PersonalTasks[i+1:]...)
					if m.DB != nil {
						_ = m.DB.SaveTask(&task, "synced")
					}
					if m.GHClient != nil {
						_ = m.GHClient.CloseTask(context.Background(), task.RepoOwner, task.RepoName, task.Number)
					}
				}
			}
		}
		for i := len(m.SquadTasks) - 1; i >= 0; i-- {
			task := m.SquadTasks[i]
			if task.IsPRReview() {
				_, pRepo, pNum, ok := github.ParsePRURL(task.PRURL())
				if ok && strings.EqualFold(pRepo, msg.Repo) && pNum == msg.Number {
					task.State = "closed"
					now := time.Now()
					task.ClosedAt = &now
					m.SquadDoneTasks = append([]github.Task{task}, m.SquadDoneTasks...)
					m.SquadTasks = append(m.SquadTasks[:i], m.SquadTasks[i+1:]...)
					if m.DB != nil {
						_ = m.DB.SaveTask(&task, "synced")
					}
					if m.GHClient != nil {
						_ = m.GHClient.CloseTask(context.Background(), task.RepoOwner, task.RepoName, task.Number)
					}
				}
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
		m.PersonalTasks = msg.PersonalTasks
		m.PersonalDoneTasks = msg.PersonalDoneTasks
		m.SquadTasks = msg.SquadTasks
		m.SquadDoneTasks = msg.SquadDoneTasks
		if msg.AutoClosedCount > 0 {
			m.StatusMsg = fmt.Sprintf("✓ Auto-closed %d review task(s) for merged/closed PR(s)", msg.AutoClosedCount)
			m.StatusMsgTimer = time.Now().Add(5 * time.Second)
		}
		m.SecuritySummaries = msg.Security
		m.OrgMembers = msg.OrgMembers
		m.RateLimit = msg.RateLimit
		if msg.AuthUser != "" {
			m.AuthUser = msg.AuthUser
		}
		if m.DB != nil {
			m.RepoTiers = m.DB.GetAllRepoTiers()
			m.RepoInteractions = m.DB.GetAllRepoInteractions()
		}
		if m.GHClient != nil {
			m.RequestsMade = m.GHClient.RequestsMade()
			m.RequestsSaved = m.GHClient.RequestsSaved()
		}
		if m.RepoCursor == 0 && len(m.Repos) > 0 {
			m.RepoCursor = m.firstSelectableRepoIndex()
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
		if m.ActiveTab == TabSquad {
			cmds = append(cmds, m.emitSquadMemberAvatarsCmd())
		}

	case AvatarLoadedMsg:
		if len(msg.RawData) > 0 {
			if m.RawAvatarCache == nil {
				m.RawAvatarCache = make(map[string][]byte)
			}
			m.RawAvatarCache[strings.ToLower(msg.Login)] = msg.RawData
			if m.ActiveTab == TabSquad {
				return m, m.emitSquadMemberAvatarsCmd()
			}
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

		switch msg.PostAction {
		case RepoActionSession:
			outMsg, err := GoToRepoSession(msg.RepoName, msg.TargetDir)
			if err != nil {
				m.StatusMsg = "Cloned, but session failed: " + err.Error()
			} else {
				m.StatusMsg = fmt.Sprintf("Cloned %s & %s", msg.RepoName, outMsg)
			}
		case RepoActionEditor:
			outMsg, err := OpenRepoEditor(msg.RepoName, msg.TargetDir)
			if err != nil {
				m.StatusMsg = "Cloned, but editor failed: " + err.Error()
			} else {
				m.StatusMsg = fmt.Sprintf("Cloned %s & %s", msg.RepoName, outMsg)
			}
		case RepoActionShell:
			outMsg, err := OpenRepoShell(msg.RepoName, msg.TargetDir)
			if err != nil {
				m.StatusMsg = "Cloned, but shell failed: " + err.Error()
			} else {
				m.StatusMsg = fmt.Sprintf("Cloned %s & %s", msg.RepoName, outMsg)
			}
		default:
			m.StatusMsg = "Successfully cloned " + msg.RepoName + " to " + collapsed
		}
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

	case TasksLoadedMsg:
		if msg.Err == nil {
			m.PersonalTasks = msg.PersonalTasks
			m.PersonalDoneTasks = msg.PersonalDoneTasks
			m.SquadTasks = msg.SquadTasks
			m.SquadDoneTasks = msg.SquadDoneTasks
			if msg.AutoClosedCount > 0 {
				m.StatusMsg = fmt.Sprintf("✓ Auto-closed %d review task(s) for merged/closed PR(s)", msg.AutoClosedCount)
				m.StatusMsgTimer = time.Now().Add(5 * time.Second)
			}
		}
		return m, nil

	case TaskCreatedMsg:
		if msg.Err != nil {
			m.StatusMsg = fmt.Sprintf("Failed to create task: %v", msg.Err)
			m.StatusMsgTimer = time.Now().Add(5 * time.Second)
			return m, nil
		}
		if msg.Task != nil {
			if m.DB != nil {
				_ = m.DB.SaveTask(msg.Task, "synced")
			}
			replaceOrPrepend := func(slice []github.Task) []github.Task {
				replaced := false
				for i := range slice {
					if slice[i].ID < 0 && slice[i].Title == msg.Task.Title && slice[i].RepoName == msg.Task.RepoName {
						slice[i] = *msg.Task
						replaced = true
						break
					}
				}
				if !replaced {
					slice = append([]github.Task{*msg.Task}, slice...)
				}
				return slice
			}
			if msg.Task.IsSquadTask {
				m.SquadTasks = replaceOrPrepend(m.SquadTasks)
			} else {
				m.PersonalTasks = replaceOrPrepend(m.PersonalTasks)
			}
			m.TaskCursor = 0
			m.StatusMsg = fmt.Sprintf("✓ Created task #%d in %s", msg.Task.Number, msg.Task.RepoName)
			m.StatusMsgTimer = time.Now().Add(4 * time.Second)
		}
		return m, nil

	case TaskUpdatedMsg:
		if msg.Err != nil {
			m.StatusMsg = fmt.Sprintf("Failed to update task: %v", msg.Err)
			m.StatusMsgTimer = time.Now().Add(5 * time.Second)
			return m, nil
		}
		if msg.Task != nil {
			if m.DB != nil {
				_ = m.DB.SaveTask(msg.Task, "synced")
			}
			updateInSlice := func(slice []github.Task) []github.Task {
				for i := range slice {
					if slice[i].ID == msg.Task.ID || (slice[i].Number == msg.Task.Number && slice[i].RepoName == msg.Task.RepoName) {
						slice[i].Title = msg.Task.Title
						slice[i].Body = msg.Task.Body
						slice[i].Priority = msg.Task.Priority
						slice[i].Assignees = msg.Task.Assignees
						slice[i].Labels = msg.Task.Labels
						slice[i].UpdatedAt = msg.Task.UpdatedAt
						break
					}
				}
				return slice
			}
			m.PersonalTasks = updateInSlice(m.PersonalTasks)
			m.PersonalDoneTasks = updateInSlice(m.PersonalDoneTasks)
			m.SquadTasks = updateInSlice(m.SquadTasks)
			m.SquadDoneTasks = updateInSlice(m.SquadDoneTasks)
			m.StatusMsg = fmt.Sprintf("✓ Saved changes to task #%d", msg.Task.Number)
			m.StatusMsgTimer = time.Now().Add(4 * time.Second)
		}
		return m, nil

	case TaskClosedMsg:
		if msg.Err != nil {
			m.StatusMsg = fmt.Sprintf("Failed to complete task: %v", msg.Err)
			m.StatusMsgTimer = time.Now().Add(5 * time.Second)
			return m, nil
		}
		now := time.Now()
		if msg.IsSquad {
			var updated []github.Task
			for _, t := range m.SquadTasks {
				if (msg.TaskID > 0 && t.ID == msg.TaskID) || (t.Number == msg.Number && t.RepoName == msg.RepoName) {
					t.State = "closed"
					t.ClosedAt = &now
					t.UpdatedAt = now
					m.SquadDoneTasks = append([]github.Task{t}, m.SquadDoneTasks...)
				} else {
					updated = append(updated, t)
				}
			}
			m.SquadTasks = updated
		} else {
			var updated []github.Task
			for _, t := range m.PersonalTasks {
				if (msg.TaskID > 0 && t.ID == msg.TaskID) || (t.Number == msg.Number && t.RepoName == msg.RepoName) {
					t.State = "closed"
					t.ClosedAt = &now
					t.UpdatedAt = now
					m.PersonalDoneTasks = append([]github.Task{t}, m.PersonalDoneTasks...)
				} else {
					updated = append(updated, t)
				}
			}
			m.PersonalTasks = updated
		}
		if m.TaskCursor >= len(m.getDisplayTasks()) && m.TaskCursor > 0 {
			m.TaskCursor = len(m.getDisplayTasks()) - 1
		}
		m.StatusMsg = fmt.Sprintf("✓ Completed task #%d (press 'd' to view done tasks)", msg.Number)
		m.StatusMsgTimer = time.Now().Add(4 * time.Second)
		return m, nil

	case TaskReopenedMsg:
		if msg.Err != nil {
			m.StatusMsg = fmt.Sprintf("Failed to reopen task: %v", msg.Err)
			m.StatusMsgTimer = time.Now().Add(5 * time.Second)
			return m, nil
		}
		now := time.Now()
		if msg.IsSquad {
			var updatedDone []github.Task
			for _, t := range m.SquadDoneTasks {
				if (msg.TaskID > 0 && t.ID == msg.TaskID) || (t.Number == msg.Number && t.RepoName == msg.RepoName) {
					t.State = "open"
					t.ClosedAt = nil
					t.UpdatedAt = now
					m.SquadTasks = append([]github.Task{t}, m.SquadTasks...)
				} else {
					updatedDone = append(updatedDone, t)
				}
			}
			m.SquadDoneTasks = updatedDone
		} else {
			var updatedDone []github.Task
			for _, t := range m.PersonalDoneTasks {
				if (msg.TaskID > 0 && t.ID == msg.TaskID) || (t.Number == msg.Number && t.RepoName == msg.RepoName) {
					t.State = "open"
					t.ClosedAt = nil
					t.UpdatedAt = now
					m.PersonalTasks = append([]github.Task{t}, m.PersonalTasks...)
				} else {
					updatedDone = append(updatedDone, t)
				}
			}
			m.PersonalDoneTasks = updatedDone
		}
		if m.TaskCursor >= len(m.getDisplayTasks()) && m.TaskCursor > 0 {
			m.TaskCursor = len(m.getDisplayTasks()) - 1
		}
		m.StatusMsg = fmt.Sprintf("✓ Reopened task #%d (now active)", msg.Number)
		m.StatusMsgTimer = time.Now().Add(4 * time.Second)
		return m, nil

	case TaskPriorityUpdatedMsg:
		if msg.Err != nil {
			m.StatusMsg = fmt.Sprintf("Failed to update priority: %v", msg.Err)
			m.StatusMsgTimer = time.Now().Add(5 * time.Second)
			return m, nil
		}
		updatePrio := func(slice []github.Task) {
			for i := range slice {
				if slice[i].ID == msg.TaskID {
					slice[i].Priority = msg.Priority
					break
				}
			}
		}
		updatePrio(m.PersonalTasks)
		updatePrio(m.PersonalDoneTasks)
		updatePrio(m.SquadTasks)
		updatePrio(m.SquadDoneTasks)
		m.StatusMsg = fmt.Sprintf("✓ Set priority to %s", msg.Priority)
		m.StatusMsgTimer = time.Now().Add(4 * time.Second)
		return m, nil

	case TaskAssigneesUpdatedMsg:
		if msg.Err != nil {
			m.StatusMsg = fmt.Sprintf("Failed to update assignees: %v", msg.Err)
			m.StatusMsgTimer = time.Now().Add(5 * time.Second)
			return m, nil
		}
		updateAssignees := func(slice []github.Task) {
			for i := range slice {
				if slice[i].ID == msg.TaskID {
					slice[i].Assignees = msg.Assignees
					break
				}
			}
		}
		updateAssignees(m.PersonalTasks)
		updateAssignees(m.PersonalDoneTasks)
		updateAssignees(m.SquadTasks)
		updateAssignees(m.SquadDoneTasks)
		m.StatusMsg = "✓ Updated task assignee"
		m.StatusMsgTimer = time.Now().Add(4 * time.Second)
		return m, nil

	case tea.KeyMsg:
		if !m.SplashDismissed {
			m.SplashDismissed = true
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}

		// Modal Key Handling
		if m.ActiveModal != ModalNone {
			prevModal := m.ActiveModal
			newM, cmd := m.handleModalKeys(msg)
			if updatedModel, ok := newM.(Model); ok {
				if prevModal != ModalNone && updatedModel.ActiveModal == ModalNone && updatedModel.ActiveTab == TabSquad {
					return updatedModel, tea.Batch(cmd, updatedModel.emitSquadMemberAvatarsCmd())
				}
			}
			return newM, cmd
		}

		// When search input is focused, route keystrokes directly to it
		if m.ActiveTab == TabSearch && m.SearchInput.Focused() {
			return m.handleSearchKeys(msg)
		}

		// When editing or adding a task inline, route keystrokes directly to task keys (disabling global hotkeys)
		if m.ActiveTab == TabTasks && (m.InlineEditing || m.InlineAdding) {
			return m.handleTasksKeys(msg)
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
			m.ActiveTab = TabMyTasks
			m.ActiveTaskSubtab = TaskSubtabMine
			return m, nil
		case "4":
			m.ActiveTab = TabSquadTasks
			m.ActiveTaskSubtab = TaskSubtabSquad
			return m, nil
		case "5":
			m.ActiveTab = TabSquad
			return m, m.emitSquadMemberAvatarsCmd()
		case "6":
			m.ActiveTab = TabSearch
			return m, nil
		case "7":
			m.ActiveTab = TabStatus
			return m, nil
		case "tab":
			m.ActiveTab = (m.ActiveTab + 1) % 7
			if m.ActiveTab == TabMyTasks {
				m.ActiveTaskSubtab = TaskSubtabMine
			} else if m.ActiveTab == TabSquadTasks {
				m.ActiveTaskSubtab = TaskSubtabSquad
			} else if m.ActiveTab == TabSquad {
				return m, m.emitSquadMemberAvatarsCmd()
			}
			return m, nil
		case "shift+tab":
			m.ActiveTab = (m.ActiveTab + 6) % 7
			if m.ActiveTab == TabMyTasks {
				m.ActiveTaskSubtab = TaskSubtabMine
			} else if m.ActiveTab == TabSquadTasks {
				m.ActiveTaskSubtab = TaskSubtabSquad
			} else if m.ActiveTab == TabSquad {
				return m, m.emitSquadMemberAvatarsCmd()
			}
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
		case TabMyTasks, TabSquadTasks:
			return m.handleTasksKeys(msg)
		case TabSquad:
			return m.handleSquadKeys(msg)
		case TabSearch:
			return m.handleSearchKeys(msg)
		case TabStatus:
			return m.handleStatusKeys(msg)
		}
	}

	if m.ActiveTab == TabSearch && m.SearchInput.Focused() {
		var cmd tea.Cmd
		m.SearchInput, cmd = m.SearchInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.ActiveTab == TabTasks && m.InlineEditing {
		var cmd tea.Cmd
		m.InlineEditTitleInput, cmd = m.InlineEditTitleInput.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.ActiveTab == TabTasks && m.InlineAdding {
		var cmd tea.Cmd
		m.InlineNewTitleInput, cmd = m.InlineNewTitleInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.Width == 0 || !m.SplashDismissed {
		return m.renderSplashScreen()
	}

	topBar := m.renderTopBar()

	var body string
	switch m.ActiveTab {
	case TabRepos:
		body = m.renderReposView()
	case TabPRs:
		body = m.renderPRsView()
	case TabMyTasks, TabSquadTasks:
		body = m.renderTasksView()
	case TabSquad:
		body = m.renderSquadView()
	case TabSearch:
		body = m.renderSearchView()
	case TabStatus:
		body = m.renderStatusView()
	default:
		body = m.renderReposView()
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
	hobbitBadge := HeaderStyle.Render("ᘛ( ᵔ ᴗ ᵔ*)ᘚ")

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

	mineOpen := len(m.PersonalTasks)
	squadOpen := len(m.SquadTasks)

	var tabs []string
	if m.Width >= 145 {
		tabs = []string{
			"1: Repositories",
			fmt.Sprintf("2: Pull Requests (%d)", len(m.PRs)),
			fmt.Sprintf("3: My Tasks (%d)", mineOpen),
			fmt.Sprintf("4: Squad Tasks (%d)", squadOpen),
			"5: Squad Activity",
			"6: Code Search",
			fmt.Sprintf("7: Status %s", statusDot),
		}
	} else if m.Width >= 115 {
		prCountStr := ""
		if len(m.PRs) > 0 {
			prCountStr = fmt.Sprintf(" (%d)", len(m.PRs))
		}
		mineCountStr := ""
		if mineOpen > 0 {
			mineCountStr = fmt.Sprintf(" (%d)", mineOpen)
		}
		squadCountStr := ""
		if squadOpen > 0 {
			squadCountStr = fmt.Sprintf(" (%d)", squadOpen)
		}
		tabs = []string{
			"1: Repos",
			fmt.Sprintf("2: PRs%s", prCountStr),
			fmt.Sprintf("3: My Tasks%s", mineCountStr),
			fmt.Sprintf("4: Squad Tasks%s", squadCountStr),
			"5: Squad",
			"6: Search",
			fmt.Sprintf("7: Status %s", statusDot),
		}
	} else {
		tabs = []string{
			"1: Repos", "2: PRs", "3: My Tasks", "4: Squad Tasks", "5: Squad", "6: Search", fmt.Sprintf("7: %s", statusDot),
		}
	}

	activeStyle := ActiveTabStyle.MarginRight(0)
	inactiveStyle := InactiveTabStyle.MarginRight(0)
	var renderedTabs []string
	for i, t := range tabs {
		if TabType(i) == m.ActiveTab {
			renderedTabs = append(renderedTabs, activeStyle.Render(t))
		} else {
			renderedTabs = append(renderedTabs, inactiveStyle.Render(t))
		}
	}

	tabsRow := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	leftPart := lipgloss.JoinHorizontal(lipgloss.Top, hobbitBadge, " ", tabsRow)

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

	// 4. Consolidated Refresh + Requests Per Hour + Saved on the far right
	reqsLastHour := 0
	if m.RateLimit.Limit > 0 && m.RateLimit.Limit >= m.RateLimit.Remaining {
		reqsLastHour = m.RateLimit.Limit - m.RateLimit.Remaining
	}

	reqsSaved := m.RequestsSaved
	if m.GHClient != nil {
		reqsSaved = m.GHClient.RequestsSaved()
	}

	var reqsStr string
	if m.Width >= 140 {
		reqsStr = fmt.Sprintf("%d/h • %d saved", reqsLastHour, reqsSaved)
	} else if m.Width >= 130 {
		reqsStr = fmt.Sprintf("%d/h • %dsav", reqsLastHour, reqsSaved)
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
		if m.Width >= 125 {
			refreshStr = fmt.Sprintf("⟳ %s (%d:%02d)", sinceStr, mins, secs)
		} else {
			refreshStr = fmt.Sprintf("⟳ %s", sinceStr)
		}
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
	} else if m.Width >= leftWidth+secWidth+rightWidth+1 {
		fullLine = leftPart + " " + secItem + " " + rightText
	} else if m.Width >= leftWidth+rightWidth+2 {
		padding := m.Width - leftWidth - rightWidth
		fullLine = leftPart + strings.Repeat(" ", padding) + rightText
	} else if m.Width >= leftWidth+secWidth+2 {
		padding := m.Width - leftWidth - secWidth
		fullLine = leftPart + strings.Repeat(" ", padding) + secItem
	} else if rightWidth > 0 && m.Width >= leftWidth+rightWidth {
		fullLine = leftPart + rightText
	} else {
		fullLine = leftPart
	}

	return TopBarStyle.Render(fullLine)
}

func (m Model) renderFooter() string {
	// Single-line efficient footer
	// Left: Contextual hotkeys & toast message
	var help string
	switch m.ActiveTab {
	case TabRepos:
		if m.RepoFilter != "" || m.RepoGroupFilter != "" || m.RepoSortVulnerabilities {
			help = "[p] PRs  [/] Filter  [#] Labels  [v] Sort Vulns  [s] Session  [e] Editor  [t] Shell  [Esc] Clear  [Enter] Web  [o] Submenu  [l] Labels  [c] Clone  [?] Help"
		} else {
			help = "[p] PRs  [/] Filter  [#] Labels  [v] Sort Vulns  [s] Session  [e] Editor  [t] Shell  [Enter] Web  [o] Submenu  [l] Labels  [c] Clone  [u] Sync  [?] Help"
		}
	case TabPRs:
		if m.PRFilter != "" {
			help = "[m] Merge  [f] Filter  [Esc] Clear Filter  [t] Create Task  [o] Browser  [p] Copilot Review  [c] Checkout Branch  [?] Help"
		} else {
			help = "[m] Merge  [f] Filter  [t] Create Task  [o] Browser  [p] Copilot Review  [c] Checkout Branch  [r] Refresh  [?] Help"
		}
	case TabMyTasks:
		if m.InlineAdding {
			help = "[Enter] Create Task  [Esc] Cancel  [ctrl+n] Notes  [ctrl+p] Priority"
		} else if m.InlineEditing {
			help = "[Enter] Save Title  [Esc] Cancel"
		} else {
			_, cursorTask := m.getCursorTarget()
			if cursorTask != nil && cursorTask.IsPRReview() {
				help = "[Enter/o] Open PR  [i] Issue  [Space] Toggle Done  [n] Notes  [p] Priority  [c] Assign  [d] All Done  [a] Add  [?] Help"
			} else {
				help = "[a] Add  [Enter] Edit Title  [Space] Toggle Done  [n] Notes  [p] Priority  [c] Assign  [d] All Done  [i/o] Issue  [?] Help"
			}
		}
	case TabSquadTasks:
		if m.InlineAdding {
			help = "[Enter] Create Task  [Esc] Cancel  [ctrl+n] Notes  [ctrl+p] Priority  [Tab] Assignee"
		} else if m.InlineEditing {
			help = "[Enter] Save Title  [Esc] Cancel"
		} else {
			_, cursorTask := m.getCursorTarget()
			if cursorTask != nil && cursorTask.IsPRReview() {
				help = "[Enter/o] Open PR  [i] Issue  [Space] Toggle Done  [A] Assign  [n] Notes  [p] Priority  [c] Cycle  [d] All Done  [a] Add  [?] Help"
			} else {
				help = "[a] Add  [Enter] Edit Title  [Space] Toggle Done  [A] Assign  [n] Notes  [p] Priority  [c] Cycle  [d] All Done  [i/o] Issue  [?] Help"
			}
		}
	case TabSquad:
		help = "[m] Manage Squad  [s] Switch Squad  [n] New Squad  [r] Refresh  [?] Help"
	case TabSearch:
		help = "[/] Query  [j/k] Nodes  [h/l/Space] Toggle  [Tab] Switch Viewport  [Enter] Open Editor  [?] Help"
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

func (m Model) getOpenTasksCount(subtab TaskSubtab) int {
	if subtab == TaskSubtabMine {
		return len(m.PersonalTasks)
	}
	return len(m.SquadTasks)
}

func (m Model) getDoneTasksCount(subtab TaskSubtab) int {
	if subtab == TaskSubtabMine {
		return len(m.PersonalDoneTasks)
	}
	return len(m.SquadDoneTasks)
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

func (m Model) recordRepoInteraction(repoName string) Model {
	if repoName == "" {
		return m
	}
	now := time.Now()
	if m.RepoInteractions == nil {
		m.RepoInteractions = make(map[string]time.Time)
	}
	m.RepoInteractions[strings.ToLower(repoName)] = now
	if m.Config != nil {
		_ = m.Config.RecordRepoInteraction(repoName)
	}
	if m.DB != nil {
		_ = m.DB.TouchRepo(repoName)
	}
	return m
}

func (m Model) isRepoHidden(repoName string) bool {
	clean := strings.ToLower(strings.TrimSpace(repoName))
	if m.HiddenRepos != nil && m.HiddenRepos[clean] {
		return true
	}
	if m.DB != nil && m.DB.IsRepoHidden(repoName) {
		return true
	}
	if m.Config != nil && m.Config.IsRepoHidden(repoName) {
		return true
	}
	return false
}

func (m Model) getRepoTier(repoName string) storage.RepoTier {
	if m.isRepoHidden(repoName) {
		return storage.TierHidden
	}
	if m.RepoTiers != nil {
		if t, ok := m.RepoTiers[strings.ToLower(repoName)]; ok && t != "" {
			return t
		}
	}
	if m.DB != nil {
		return m.DB.GetRepoTier(repoName)
	}
	return storage.TierWarm
}

func (m Model) getRepoTierCounts() (hot, warm, cold, dead, hidden int) {
	for _, r := range m.Repos {
		switch m.getRepoTier(r.Name) {
		case storage.TierHot:
			hot++
		case storage.TierWarm:
			warm++
		case storage.TierCold:
			cold++
		case storage.TierDead:
			dead++
		case storage.TierHidden:
			hidden++
		default:
			warm++
		}
	}
	return hot, warm, cold, dead, hidden
}

func (m Model) closeTaskLocally(target github.Task) Model {
	now := time.Now()
	target.State = "closed"
	target.ClosedAt = &now
	target.UpdatedAt = now
	if target.IsSquadTask {
		var updated []github.Task
		for _, t := range m.SquadTasks {
			if (target.ID > 0 && t.ID == target.ID) || (t.Number == target.Number && t.RepoName == target.RepoName) {
				m.SquadDoneTasks = append([]github.Task{target}, m.SquadDoneTasks...)
			} else {
				updated = append(updated, t)
			}
		}
		m.SquadTasks = updated
	} else {
		var updated []github.Task
		for _, t := range m.PersonalTasks {
			if (target.ID > 0 && t.ID == target.ID) || (t.Number == target.Number && t.RepoName == target.RepoName) {
				m.PersonalDoneTasks = append([]github.Task{target}, m.PersonalDoneTasks...)
			} else {
				updated = append(updated, t)
			}
		}
		m.PersonalTasks = updated
	}
	return m
}

func (m Model) reopenTaskLocally(target github.Task) Model {
	now := time.Now()
	target.State = "open"
	target.ClosedAt = nil
	target.UpdatedAt = now
	if target.IsSquadTask {
		var updatedDone []github.Task
		for _, t := range m.SquadDoneTasks {
			if (target.ID > 0 && t.ID == target.ID) || (t.Number == target.Number && t.RepoName == target.RepoName) {
				m.SquadTasks = append([]github.Task{target}, m.SquadTasks...)
			} else {
				updatedDone = append(updatedDone, t)
			}
		}
		m.SquadDoneTasks = updatedDone
	} else {
		var updatedDone []github.Task
		for _, t := range m.PersonalDoneTasks {
			if (target.ID > 0 && t.ID == target.ID) || (t.Number == target.Number && t.RepoName == target.RepoName) {
				m.PersonalTasks = append([]github.Task{target}, m.PersonalTasks...)
			} else {
				updatedDone = append(updatedDone, t)
			}
		}
		m.PersonalDoneTasks = updatedDone
	}
	return m
}

func (m Model) getRepoLastInteractedTime(repoName string, tmuxSessions map[string]TmuxSessionInfo) time.Time {
	var t time.Time

	// 1. Check in-memory interaction cache
	if m.RepoInteractions != nil {
		if it, ok := m.RepoInteractions[strings.ToLower(repoName)]; ok && !it.IsZero() {
			t = it
		}
	}

	// 2. Check config
	if m.Config != nil {
		ct := m.Config.GetRepoLastInteracted(repoName)
		if ct.After(t) {
			t = ct
		}
	}

	// 3. Tmux session
	if sess, ok := FindTmuxSession(repoName, tmuxSessions); ok && !sess.LastActivity.IsZero() {
		if sess.LastActivity.After(t) {
			t = sess.LastActivity
		}
	}

	// 4. Local git index modified time
	if st, ok := m.GitStatuses[repoName]; ok && st.IsCloned && st.IsDirty {
		indexPath := filepath.Join(st.LocalPath, ".git", "index")
		if info, err := os.Stat(indexPath); err == nil {
			if info.ModTime().After(t) {
				t = info.ModTime()
			}
		}
	}

	return t
}

func (m Model) getLabelColor(label string) string {
	clean := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(label), "#"))
	if m.RepoLabelColors != nil {
		if c, ok := m.RepoLabelColors[clean]; ok && c != "" {
			return c
		}
	}
	hash := 0
	for _, r := range clean {
		hash = hash*31 + int(r)
	}
	if hash < 0 {
		hash = -hash
	}
	return github.GenerateLabelColor(hash)
}

func (m Model) syncSquadRepoLabelsCmd(repoID int64, repoName string, labels []string) tea.Cmd {
	if m.GHClient == nil || m.Config == nil || m.Config.ActiveSquad == "" || m.CurrentOrg == "" {
		return nil
	}
	squadRepo := fmt.Sprintf("%s-tasks", m.Config.ActiveSquad)
	repoIDStr := strconv.FormatInt(repoID, 10)
	if repoID <= 0 {
		repoIDStr = repoName
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = m.GHClient.SaveSquadRepoLabels(ctx, m.CurrentOrg, squadRepo, func(mappings map[string][]string) error {
			if len(labels) > 0 {
				mappings[repoIDStr] = labels
			} else {
				delete(mappings, repoIDStr)
			}
			return nil
		})
		return nil
	}
}

