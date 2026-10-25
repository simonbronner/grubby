package ui

import (
	"testing"

	"grubby/internal/git"
)

func TestSanitizeTmuxSessionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"auth-service", "auth-service"},
		{"api.gateway", "api-gateway"},
		{"core/payments", "core-payments"},
		{"weird@#name!123", "weird-name-123"},
		{"", "repo"},
		{"---", "repo"},
	}

	for _, tt := range tests {
		got := SanitizeTmuxSessionName(tt.input)
		if got != tt.expected {
			t.Errorf("SanitizeTmuxSessionName(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestRepoClonedMsg_PostAction(t *testing.T) {
	m := createTestModel()
	m.GitStatuses = make(map[string]git.RepoGitStatus)

	msg := RepoClonedMsg{
		RepoName:   "docs",
		TargetDir:  "/tmp/docs",
		Status:     git.RepoGitStatus{IsCloned: true},
		Err:        nil,
		PostAction: RepoActionSession,
	}
	updated, _ := m.Update(msg)
	newM := updated.(Model)
	if !newM.GitStatuses["docs"].IsCloned {
		t.Errorf("expected docs to be marked as cloned")
	}
}
