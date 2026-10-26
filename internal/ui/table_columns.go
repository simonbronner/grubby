package ui

import "github.com/charmbracelet/lipgloss"

type RepoColumns struct {
	Cursor       lipgloss.Style
	Name         lipgloss.Style
	Sec          lipgloss.Style
	Group        lipgloss.Style
	Git          lipgloss.Style
	Session      lipgloss.Style
	Last         lipgloss.Style
	Path         lipgloss.Style

	NameWidth    int
	GroupWidth   int
	SessionWidth int
	PathWidth    int
}

func computeRepoColumns(termWidth int) RepoColumns {
	w := termWidth - 2
	// Minimum width:
	// Cursor (3) + Sec (16) + Git (16) + Session (12) + Last (13) + MinName (22) + MinGroup (12) + MinPath (18) = 112
	if w < 112 {
		w = 112
	}

	fixedWidth := 3 + 16 + 16 + 12 + 13 // 60
	elastic := w - fixedWidth

	nameW := elastic * 35 / 100
	if nameW < 22 {
		nameW = 22
	}
	groupW := elastic * 20 / 100
	if groupW < 12 {
		groupW = 12
	}
	pathW := elastic - nameW - groupW

	colStyle := func(width int) lipgloss.Style {
		return lipgloss.NewStyle().Width(width).MaxHeight(1).Inline(true)
	}

	return RepoColumns{
		Cursor:       colStyle(3),
		Name:         colStyle(nameW),
		Sec:          colStyle(16),
		Group:        colStyle(groupW),
		Git:          colStyle(16),
		Session:      colStyle(12),
		Last:         colStyle(13),
		Path:         colStyle(pathW),
		NameWidth:    nameW,
		GroupWidth:   groupW,
		SessionWidth: 12,
		PathWidth:    pathW,
	}
}

type PRColumns struct {
	Cursor   lipgloss.Style
	Num      lipgloss.Style
	Repo     lipgloss.Style
	Title    lipgloss.Style
	Author   lipgloss.Style
	Age      lipgloss.Style
	Review   lipgloss.Style
	Comments lipgloss.Style
	Labels   lipgloss.Style

	RepoWidth   int
	TitleWidth  int
	AuthorWidth int
	AgeWidth    int
	ReviewWidth int
	LabelsWidth int
}

func computePRColumns(termWidth int) PRColumns {
	w := termWidth - 2
	// Minimum width:
	// Cursor (3) + Num (7) + Author (12) + Age (10) + Review (15) + Comments (10) + MinRepo (16) + MinLabels (15) + MinTitle (26) = 114
	if w < 114 {
		w = 114
	}

	fixedWidth := 3 + 7 + 12 + 10 + 15 + 10 // 57
	elastic := w - fixedWidth

	repoW := elastic * 20 / 100
	if repoW < 16 {
		repoW = 16
	}
	labelsW := elastic * 20 / 100
	if labelsW < 15 {
		labelsW = 15
	}
	titleW := elastic - repoW - labelsW

	colStyle := func(width int) lipgloss.Style {
		return lipgloss.NewStyle().Width(width).MaxHeight(1).Inline(true)
	}

	return PRColumns{
		Cursor:      colStyle(3),
		Num:         colStyle(7),
		Repo:        colStyle(repoW),
		Title:       colStyle(titleW),
		Author:      colStyle(12),
		Age:         colStyle(10),
		Review:      colStyle(15),
		Comments:    colStyle(10),
		Labels:      colStyle(labelsW),
		RepoWidth:   repoW,
		TitleWidth:  titleW,
		AuthorWidth: 12,
		AgeWidth:    10,
		ReviewWidth: 15,
		LabelsWidth: labelsW,
	}
}

type SquadColumns struct {
	Cursor  lipgloss.Style
	Time    lipgloss.Style
	Actor   lipgloss.Style
	Action  lipgloss.Style
	Target  lipgloss.Style
	Details lipgloss.Style

	TargetWidth  int
	DetailsWidth int
}

func computeSquadColumns(termWidth int) SquadColumns {
	w := termWidth - 2
	// Minimum width:
	// Cursor (3) + Time (11) + Actor (15) + Action (16) + MinTarget (20) + MinDetails (25) = 90
	if w < 90 {
		w = 90
	}

	fixedWidth := 3 + 11 + 15 + 16 // 45
	elastic := w - fixedWidth

	targetW := elastic * 30 / 100
	if targetW < 20 {
		targetW = 20
	}
	detailsW := elastic - targetW

	colStyle := func(width int) lipgloss.Style {
		return lipgloss.NewStyle().Width(width).MaxHeight(1).Inline(true)
	}

	return SquadColumns{
		Cursor:       colStyle(3),
		Time:         colStyle(11),
		Actor:        colStyle(15),
		Action:       colStyle(16),
		Target:       colStyle(targetW),
		Details:      colStyle(detailsW),
		TargetWidth:  targetW,
		DetailsWidth: detailsW,
	}
}

type NotificationColumns struct {
	Cursor lipgloss.Style
	Status lipgloss.Style
	Type   lipgloss.Style
	Reason lipgloss.Style
	Repo   lipgloss.Style
	Title  lipgloss.Style
	Time   lipgloss.Style

	RepoWidth  int
	TitleWidth int
}

func computeNotificationColumns(termWidth int) NotificationColumns {
	w := termWidth - 2
	// Minimum width:
	// Cursor (3) + Status (7) + Type (13) + Reason (14) + Time (11) + MinRepo (18) + MinTitle (28) = 94
	if w < 94 {
		w = 94
	}

	fixedWidth := 3 + 7 + 13 + 14 + 11 // 48
	elastic := w - fixedWidth

	repoW := elastic * 25 / 100
	if repoW < 18 {
		repoW = 18
	}
	titleW := elastic - repoW

	colStyle := func(width int) lipgloss.Style {
		return lipgloss.NewStyle().Width(width).MaxHeight(1).Inline(true)
	}

	return NotificationColumns{
		Cursor:     colStyle(3),
		Status:     colStyle(7),
		Type:       colStyle(13),
		Reason:     colStyle(14),
		Repo:       colStyle(repoW),
		Title:      colStyle(titleW),
		Time:       colStyle(11),
		RepoWidth:  repoW,
		TitleWidth: titleW,
	}
}
