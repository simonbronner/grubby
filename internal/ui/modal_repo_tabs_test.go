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
		RepoCursor:  1, // 0 is TierHeader, 1 is my-service
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

func TestRepoTabsModal_ActsOnSelectedRepoNotFirst(t *testing.T) {
	repos := []github.Repository{
		{Name: "first-repo", FullName: "my-org/first-repo"},
		{Name: "second-repo", FullName: "my-org/second-repo"},
		{Name: "third-repo", FullName: "my-org/third-repo"},
	}

	m := Model{
		ActiveTab: TabRepos,
		Repos:     repos,
		Width:     120,
		Height:    30,
	}

	// In table items:
	// Item 0: Tier Header (Warm)
	// Item 1: first-repo (or sorted order)
	// Item 2: second-repo
	// Item 3: third-repo
	items := m.getRepoTableItems()
	var thirdIdx int
	for idx, it := range items {
		if it.Kind == ItemRepo && it.Repo != nil && it.Repo.Name == "third-repo" {
			thirdIdx = idx
			break
		}
	}
	if thirdIdx == 0 {
		t.Fatalf("could not find third-repo in table items")
	}

	// Select third-repo
	m.RepoCursor = thirdIdx

	// Press 'o' -> opens ModalRepoTabs
	updated, _ := m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m = updated.(Model)
	if m.ActiveModal != ModalRepoTabs {
		t.Fatalf("expected ActiveModal to be ModalRepoTabs")
	}

	// Render modal -> must mention third-repo, NOT first-repo!
	view := m.renderRepoTabsModal()
	if !strings.Contains(view, "third-repo") {
		t.Errorf("expected modal title to mention third-repo, got:\n%s", view)
	}
	if strings.Contains(view, "first-repo") {
		t.Errorf("modal should NOT mention first-repo, got:\n%s", view)
	}

	// Press 'p' for Pull Requests -> must act on third-repo
	updatedP, cmdP := m.handleRepoTabsModalKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = updatedP.(Model)
	if m.ActiveModal != ModalNone {
		t.Errorf("expected modal to close")
	}
	if cmdP == nil {
		t.Fatalf("expected toast cmd")
	}
	toast := string(cmdP().(StatusToastMsg))
	if !strings.Contains(toast, "third-repo") {
		t.Errorf("expected toast to mention third-repo, got: %s", toast)
	}
	if strings.Contains(toast, "first-repo") {
		t.Errorf("toast should NOT mention first-repo, got: %s", toast)
	}

	// Cursor must stay focused on third-repo, NOT reset to 0!
	selectedAfter := m.getSelectedRepo()
	if selectedAfter == nil || selectedAfter.Name != "third-repo" {
		t.Errorf("expected cursor to remain focused on third-repo, got %+v (cursor %d)", selectedAfter, m.RepoCursor)
	}
}
