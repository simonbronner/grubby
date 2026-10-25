package ui

import "github.com/charmbracelet/lipgloss"

type RepoColumns struct {
	Cursor lipgloss.Style
	Name   lipgloss.Style
	Sec    lipgloss.Style
	Group  lipgloss.Style
	Git    lipgloss.Style
	Branch lipgloss.Style
	Last   lipgloss.Style
	Path   lipgloss.Style

	NameWidth   int
	GroupWidth  int
	BranchWidth int
	PathWidth   int
}

func computeRepoColumns(termWidth int) RepoColumns {
	w := termWidth - 2
	// Minimum width where each column meets its minimum:
	// Cursor (3) + Sec (11) + Git (16) + Branch (12) + Last (13) + MinName (22) + MinGroup (12) + MinPath (18) = 107
	if w < 107 {
		w = 107
	}

	fixedWidth := 3 + 11 + 16 + 12 + 13 // 55
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

	return RepoColumns{
		Cursor:      lipgloss.NewStyle().Width(3),
		Name:        lipgloss.NewStyle().Width(nameW),
		Sec:         lipgloss.NewStyle().Width(11),
		Group:       lipgloss.NewStyle().Width(groupW),
		Git:         lipgloss.NewStyle().Width(16),
		Branch:      lipgloss.NewStyle().Width(12),
		Last:        lipgloss.NewStyle().Width(13),
		Path:        lipgloss.NewStyle().Width(pathW),
		NameWidth:   nameW,
		GroupWidth:  groupW,
		BranchWidth: 12,
		PathWidth:   pathW,
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
	LabelsWidth int
}

func computePRColumns(termWidth int) PRColumns {
	w := termWidth - 2
	// Minimum width where each column meets its minimum:
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

	return PRColumns{
		Cursor:      lipgloss.NewStyle().Width(3),
		Num:         lipgloss.NewStyle().Width(7),
		Repo:        lipgloss.NewStyle().Width(repoW),
		Title:       lipgloss.NewStyle().Width(titleW),
		Author:      lipgloss.NewStyle().Width(12),
		Age:         lipgloss.NewStyle().Width(10),
		Review:      lipgloss.NewStyle().Width(15),
		Comments:    lipgloss.NewStyle().Width(10),
		Labels:      lipgloss.NewStyle().Width(labelsW),
		RepoWidth:   repoW,
		TitleWidth:  titleW,
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

	return SquadColumns{
		Cursor:       lipgloss.NewStyle().Width(3),
		Time:         lipgloss.NewStyle().Width(11),
		Actor:        lipgloss.NewStyle().Width(15),
		Action:       lipgloss.NewStyle().Width(16),
		Target:       lipgloss.NewStyle().Width(targetW),
		Details:      lipgloss.NewStyle().Width(detailsW),
		TargetWidth:  targetW,
		DetailsWidth: detailsW,
	}
}

type SecurityColumns struct {
	Cursor lipgloss.Style
	Repo   lipgloss.Style
	Crit   lipgloss.Style
	High   lipgloss.Style
	Med    lipgloss.Style
	Low    lipgloss.Style
	Total  lipgloss.Style
	Status lipgloss.Style

	RepoWidth   int
	StatusWidth int
}

func computeSecurityColumns(termWidth int) SecurityColumns {
	w := termWidth - 2
	// Minimum width:
	// Cursor (3) + Crit (10) + High (10) + Med (8) + Low (8) + Total (10) + MinRepo (20) + MinStatus (20) = 89
	if w < 89 {
		w = 89
	}

	fixedWidth := 3 + 10 + 10 + 8 + 8 + 10 // 49
	elastic := w - fixedWidth

	repoW := elastic * 35 / 100
	if repoW < 20 {
		repoW = 20
	}
	statusW := elastic - repoW

	return SecurityColumns{
		Cursor:      lipgloss.NewStyle().Width(3),
		Repo:        lipgloss.NewStyle().Width(repoW),
		Crit:        lipgloss.NewStyle().Width(10),
		High:        lipgloss.NewStyle().Width(10),
		Med:         lipgloss.NewStyle().Width(8),
		Low:         lipgloss.NewStyle().Width(8),
		Total:       lipgloss.NewStyle().Width(10),
		Status:      lipgloss.NewStyle().Width(statusW),
		RepoWidth:   repoW,
		StatusWidth: statusW,
	}
}
