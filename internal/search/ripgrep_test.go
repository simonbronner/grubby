package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRipgrepSearchAndTree(t *testing.T) {
	engine := NewRipgrepEngine()
	if !engine.Available() {
		t.Skip("ripgrep not available on PATH")
	}

	tmpRepo := t.TempDir()
	testFile := filepath.Join(tmpRepo, "sample.go")
	content := `package main

func ValidateToken() bool {
	return true
}
`
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx := context.Background()
	matches, err := engine.Search(ctx, "ValidateToken", []string{tmpRepo})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(matches) == 0 {
		t.Fatalf("expected matches, got none")
	}

	if matches[0].TotalHits != 1 {
		t.Errorf("expected 1 hit, got %d", matches[0].TotalHits)
	}

	// Test Tree building
	tree := BuildTree(matches)
	if len(tree) != 1 {
		t.Fatalf("expected 1 root repo node, got %d", len(tree))
	}
	repoNode := tree[0]
	if repoNode.Kind != NodeRepo || len(repoNode.Children) != 1 {
		t.Fatalf("expected 1 file child under repo")
	}

	fileNode := repoNode.Children[0]
	if fileNode.Kind != NodeFile || len(fileNode.Children) != 1 {
		t.Fatalf("expected 1 match child under file")
	}

	matchNode := fileNode.Children[0]
	if matchNode.Kind != NodeMatch || matchNode.Match == nil {
		t.Fatalf("expected valid match node")
	}
	if matchNode.Match.LineNumber != 3 {
		t.Errorf("expected line number 3, got %d", matchNode.Match.LineNumber)
	}

	// Test flattening
	flat := FlattenVisibleNodes(tree)
	if len(flat) != 3 {
		t.Errorf("expected 3 flat nodes (repo, file, match), got %d", len(flat))
	}

	// Test collapsing file
	fileNode.Expanded = false
	flatCollapsed := FlattenVisibleNodes(tree)
	if len(flatCollapsed) != 2 {
		t.Errorf("expected 2 flat nodes when file collapsed, got %d", len(flatCollapsed))
	}
}
