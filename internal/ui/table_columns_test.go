package ui

import (
	"testing"
)

func TestComputeRepoColumns(t *testing.T) {
	widths := []int{70, 90, 100, 112, 120, 150, 180, 220, 300}

	for _, w := range widths {
		cols := computeRepoColumns(w)

		if cols.NameWidth <= 0 || cols.GroupWidth <= 0 || cols.PathWidth <= 0 || cols.BranchWidth <= 0 {
			t.Fatalf("Width %d produced invalid column width: %+v", w, cols)
		}

		target := w - 2
		if target < 112 {
			target = 112
		}

		fixedWidth := 3 + 16 + 16 + 12 + 13 // 60
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

func TestComputeNotificationColumns(t *testing.T) {
	widths := []int{75, 94, 100, 120, 150, 180, 220, 300}

	for _, w := range widths {
		cols := computeNotificationColumns(w)

		if cols.RepoWidth <= 0 || cols.TitleWidth <= 0 {
			t.Fatalf("Width %d produced invalid column width: %+v", w, cols)
		}

		target := w - 2
		if target < 94 {
			target = 94
		}

		fixedWidth := 3 + 7 + 13 + 14 + 11 // 48
		total := fixedWidth + cols.RepoWidth + cols.TitleWidth

		if total != target {
			t.Errorf("Width %d: total columns %d != target %d", w, total, target)
		}
	}
}

func TestDistributeEvenly(t *testing.T) {
	items := []string{"[Mascot]", "🏢 Org", "👥 Squad", "⑂ 2 PRs", "🛡️ Clean", "👤 @user"}

	// Test wide width
	renderedWide := distributeEvenly(items, 140)
	if len(renderedWide) < 100 {
		t.Errorf("Expected wide rendering, got %q", renderedWide)
	}

	// Test narrow width
	renderedNarrow := distributeEvenly(items, 60)
	if renderedNarrow == "" {
		t.Errorf("Expected fallback string, got empty")
	}

	// Test edge cases
	if distributeEvenly(nil, 100) != "" {
		t.Errorf("Expected empty string for nil items")
	}
	if distributeEvenly([]string{"Solo"}, 100) != "Solo" {
		t.Errorf("Expected single item unchanged")
	}
}
