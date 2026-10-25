package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInspectRepoNotCloned(t *testing.T) {
	ctx := context.Background()
	st := InspectRepo(ctx, "non-existent", "/path/that/does/not/exist")
	if st.IsCloned {
		t.Errorf("expected IsCloned=false, got true")
	}
	if st.State != StateNotCloned {
		t.Errorf("expected StateNotCloned, got %s", st.State)
	}
	if st.Badge() != "[Not Cloned]" {
		t.Errorf("expected '[Not Cloned]' badge, got %s", st.Badge())
	}
}

func TestWorkspaceManager(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewWorkspaceManager(tmpDir)

	if mgr.WorkspaceDir() != tmpDir {
		t.Errorf("expected %s, got %s", tmpDir, mgr.WorkspaceDir())
	}

	repoPath := filepath.Join(tmpDir, "test-repo")
	_ = os.MkdirAll(repoPath, 0o755)

	statuses := mgr.ScanWorkspaces(context.Background(), []string{"test-repo"})
	st, ok := statuses["test-repo"]
	if !ok {
		t.Fatalf("expected test-repo in status map")
	}
	// No .git directory yet
	if st.IsCloned {
		t.Errorf("expected IsCloned=false for folder without .git")
	}
}
