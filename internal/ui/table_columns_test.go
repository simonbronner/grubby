package ui

import (
	"testing"
)

func TestComputeRepoColumns(t *testing.T) {
	widths := []int{70, 90, 100, 110, 120, 150, 180, 220, 300}

	for _, w := range widths {
		cols := computeRepoColumns(w)

		if cols.NameWidth <= 0 || cols.GroupWidth <= 0 || cols.PathWidth <= 0 || cols.BranchWidth <= 0 {
			t.Fatalf("Width %d produced invalid column width: %+v", w, cols)
		}

		target := w - 2
		if target < 107 {
			target = 107
		}

		fixedWidth := 3 + 11 + 16 + 12 + 13 // 55
		total := fixedWidth + cols.NameWidth + cols.GroupWidth + cols.PathWidth

		if total != target {
			t.Errorf("Width %d: total columns %d != target %d", w, total, target)
		}
	}
}

func TestComputePRColumns(t *testing.T) {
	widths := []int{80, 100, 114, 120, 150, 180, 220, 300}

	for _, w := range widths {
		cols := computePRColumns(w)

		if cols.RepoWidth <= 0 || cols.TitleWidth <= 0 || cols.LabelsWidth <= 0 {
			t.Fatalf("Width %d produced invalid column width: %+v", w, cols)
		}

		target := w - 2
		if target < 114 {
			target = 114
		}

		fixedWidth := 3 + 7 + 12 + 10 + 15 + 10 // 57
		total := fixedWidth + cols.RepoWidth + cols.TitleWidth + cols.LabelsWidth

		if total != target {
			t.Errorf("Width %d: total columns %d != target %d", w, total, target)
		}
	}
}

func TestComputeSquadColumns(t *testing.T) {
	widths := []int{75, 90, 100, 120, 150, 180, 220, 300}

	for _, w := range widths {
		cols := computeSquadColumns(w)

		if cols.TargetWidth <= 0 || cols.DetailsWidth <= 0 {
			t.Fatalf("Width %d produced invalid column width: %+v", w, cols)
		}

		target := w - 2
		if target < 90 {
			target = 90
		}

		fixedWidth := 3 + 11 + 15 + 16 // 45
		total := fixedWidth + cols.TargetWidth + cols.DetailsWidth

		if total != target {
			t.Errorf("Width %d: total columns %d != target %d", w, total, target)
		}
	}
}

func TestComputeSecurityColumns(t *testing.T) {
	widths := []int{75, 89, 100, 120, 150, 180, 220, 300}

	for _, w := range widths {
		cols := computeSecurityColumns(w)

		if cols.RepoWidth <= 0 || cols.StatusWidth <= 0 {
			t.Fatalf("Width %d produced invalid column width: %+v", w, cols)
		}

		target := w - 2
		if target < 89 {
			target = 89
		}

		fixedWidth := 3 + 10 + 10 + 8 + 8 + 10 // 49
		total := fixedWidth + cols.RepoWidth + cols.StatusWidth

		if total != target {
			t.Errorf("Width %d: total columns %d != target %d", w, total, target)
		}
	}
}
