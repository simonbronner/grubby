package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"grubby/internal/github"
)

func TestRepoTabsModal_MnemonicLetterHotkeys(t *testing.T) {
	repos := []github.Repository{
		{Name: "my-service", FullName: "my-org/my-service"},
	}

	m := Model{
		ActiveTab:   TabRepos,
		Repos:       repos,
		RepoCursor:  0,
		ActiveModal: ModalRepoTabs,
		Width:       120,
		Height:      30,
	}

	rendered := m.renderRepoTabsModal()
	// Must contain mnemonic keys in brackets
	for _, k := range []string{"[c]", "[p]", "[i]", "[a]", "[d]", "[r]", "[n]", "[s]"} {
		if !strings.Contains(rendered, k) {
			t.Errorf("expected repo tabs modal to contain mnemonic key %s, got:\n%s", k, rendered)
		}
	}

	// Test pressing 'p' for Pull Requests
	updated, cmd := m.handleRepoTabsModalKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	upModel := updated.(Model)
	if upModel.ActiveModal != ModalNone {
		t.Errorf("expected modal to close after pressing 'p'")
	}
	if cmd == nil {
		t.Fatalf("expected toast message cmd after pressing 'p'")
	}
	msg := string(cmd().(StatusToastMsg))
	if !strings.Contains(msg, "Pull Requests") {
		t.Errorf("expected toast to mention Pull Requests, got: %s", msg)
	}

	// Test pressing 'd' for Dependabot
	m.ActiveModal = ModalRepoTabs
	updatedD, cmdD := m.handleRepoTabsModalKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	upModelD := updatedD.(Model)
	if upModelD.ActiveModal != ModalNone {
		t.Errorf("expected modal to close after pressing 'd'")
	}
	msgD := string(cmdD().(StatusToastMsg))
	if !strings.Contains(msgD, "Dependabot") {
		t.Errorf("expected toast to mention Dependabot, got: %s", msgD)
	}

	// Test fallback number key '1' for Code
	m.ActiveModal = ModalRepoTabs
	updated1, cmd1 := m.handleRepoTabsModalKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	upModel1 := updated1.(Model)
	if upModel1.ActiveModal != ModalNone {
		t.Errorf("expected modal to close after pressing '1'")
	}
	msg1 := string(cmd1().(StatusToastMsg))
	if !strings.Contains(msg1, "Code (Overview)") {
		t.Errorf("expected toast to mention Code (Overview), got: %s", msg1)
	}
}
