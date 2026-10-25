package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Base Colors
	PrimaryColor   = lipgloss.Color("#7D56F4") // Purple
	SecondaryColor = lipgloss.Color("#00D7D7") // Cyan
	SuccessColor   = lipgloss.Color("#04B575") // Green
	WarningColor   = lipgloss.Color("#FFAF00") // Amber
	DangerColor    = lipgloss.Color("#FF5F87") // Coral Red
	MutedColor     = lipgloss.Color("#626262") // Dim Gray
	DarkBgColor    = lipgloss.Color("#14141E") // Shade darker than tabs
	HighlightColor = lipgloss.Color("#F1F1F1")

	// Header & Tabs
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Background(PrimaryColor).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1)

	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Background(PrimaryColor).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			MarginRight(1)

	InactiveTabStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#222234")).
			Foreground(lipgloss.Color("#8E8EA8")).
			Padding(0, 1).
			MarginRight(1)

	TopBarStyle = lipgloss.NewStyle().
			Background(DarkBgColor)

	TabBarBorder = lipgloss.NewStyle().
			Background(DarkBgColor).
			Foreground(lipgloss.Color("#3E3E50"))

	// Status & Meta Bar
	MetaBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888899")).
			Padding(0, 1)

	// Tables & Lists
	SelectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Background(lipgloss.Color("#2A2640")).
				Foreground(lipgloss.Color("#FFFFFF"))

	NormalRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E0E0E0"))

	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#8A8A9E")).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("#3E3E50"))

	// Badges
	SyncedBadge = lipgloss.NewStyle().
			Foreground(SuccessColor).
			Bold(true)

	BehindBadge = lipgloss.NewStyle().
			Foreground(WarningColor).
			Bold(true)

	DirtyBadge = lipgloss.NewStyle().
			Foreground(DangerColor).
			Bold(true)

	NotClonedBadge = lipgloss.NewStyle().
			Foreground(MutedColor)

	StaleBadge = lipgloss.NewStyle().
			Foreground(DangerColor).
			Bold(true)

	ApprovedBadge = lipgloss.NewStyle().
			Foreground(SuccessColor).
			Bold(true)

	ChangesReqBadge = lipgloss.NewStyle().
			Foreground(DangerColor).
			Bold(true)

	ReviewReqBadge = lipgloss.NewStyle().
			Foreground(WarningColor).
			Bold(true)

	DangerBadge = lipgloss.NewStyle().
			Foreground(DangerColor).
			Bold(true)

	// Status Dots for Tabs
	StatusHealthyDot = lipgloss.NewStyle().
				Foreground(SuccessColor).
				Bold(true)

	StatusDegradedDot = lipgloss.NewStyle().
				Foreground(WarningColor).
				Bold(true)

	StatusOutageDot = lipgloss.NewStyle().
			Foreground(DangerColor).
			Bold(true)

	StatusUnknownDot = lipgloss.NewStyle().
				Foreground(MutedColor)

	GroupTagBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#302B4B")).
			Foreground(SecondaryColor).
			Padding(0, 1)

	// Modal / Dialog Box
	ModalBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Background(DarkBgColor).
			Padding(1, 2)

	ModalTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(SecondaryColor).
			MarginBottom(1)

	// Footer / Keybindings
	FooterStyle = lipgloss.NewStyle().
			Background(DarkBgColor).
			Foreground(lipgloss.Color("#777788")).
			Padding(0, 1)

	KeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(SecondaryColor)
)
