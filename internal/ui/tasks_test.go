package ui

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"grubby/internal/config"
	"grubby/internal/github"
	"grubby/internal/storage"
)

func createTestTasksModel() Model {
	now := time.Now()

	cfg := &config.Config{
		Version:     1,
		ActiveSquad: "core-team",
		Squads: []config.SquadConfig{
			{
				Name:        "core-team",
				DisplayName: "Core Team",
				Members:     []string{"alice", "bob"},
			},
		},
	}

	personalTasks := []github.Task{
		{
			ID:          1,
			Number:      101,
			Title:       "Low priority personal task",
			Body:        "Has some body notes here",
			Priority:    github.PriorityLow,
			Assignees:   []string{"tester"},
			RepoName:    "tasks",
			UpdatedAt:   now.Add(-1 * time.Hour),
			IsSquadTask: false,
		},
		{
			ID:          2,
			Number:      102,
			Title:       "High priority personal task (older)",
			Priority:    github.PriorityHigh,
			Assignees:   []string{"tester"},
			RepoName:    "tasks",
			UpdatedAt:   now.Add(-5 * time.Hour),
			IsSquadTask: false,
		},
		{
			ID:          3,
			Number:      103,
			Title:       "High priority personal task (newer)",
			Priority:    github.PriorityHigh,
			Assignees:   []string{"tester"},
			RepoName:    "tasks",
			UpdatedAt:   now.Add(-10 * time.Minute),
			IsSquadTask: false,
		},
		{
			ID:          4,
			Number:      104,
			Title:       "Unlabeled task deemed medium",
			Priority:    github.ParsePriority(nil), // No labels -> medium
			Assignees:   []string{"tester"},
			RepoName:    "tasks",
			UpdatedAt:   now.Add(-30 * time.Minute),
			IsSquadTask: false,
		},
	}

	squadTasks := []github.Task{
		{
			ID:          11,
			Number:      201,
			Title:       "Squad task assigned to alice",
			Priority:    github.PriorityMedium,
			Assignees:   []string{"alice"},
			RepoName:    "core-team-tasks",
			UpdatedAt:   now.Add(-2 * time.Hour),
			IsSquadTask: true,
		},
		{
			ID:          12,
			Number:      202,
			Title:       "Squad task unassigned",
			Priority:    github.PriorityHigh,
			Assignees:   []string{},
			RepoName:    "core-team-tasks",
			UpdatedAt:   now.Add(-1 * time.Hour),
			IsSquadTask: true,
		},
	}

	inti := textinput.New()
	ieti := textinput.New()
	tni := textarea.New()

	m := Model{
		ActiveTab:            TabTasks,
		ActiveTaskSubtab:     TaskSubtabMine,
		Config:               cfg,
		AuthUser:             "tester",
		CurrentOrg:           "my-org",
		PersonalTasks:        personalTasks,
		SquadTasks:           squadTasks,
		InitialDataLoaded:    true,
		SplashDismissed:      true,
		Width:                120,
		Height:               30,
		InlineNewTitleInput:  inti,
		InlineEditTitleInput: ieti,
		TaskNotesInput:       tni,
	}

	return m
}

func TestTaskSorting_PriorityAndUpdated(t *testing.T) {
	m := createTestTasksModel()

	display := m.getDisplayTasks()
	if len(display) != 4 {
		t.Fatalf("expected 4 personal tasks, got %d", len(display))
	}

	// Expected order for Personal Tasks:
	// 1. High (newer: 10m ago) -> #103
	// 2. High (older: 5h ago)  -> #102
	// 3. Medium (unlabeled: 30m ago) -> #104
	// 4. Low (1h ago)          -> #101
	expectedNumbers := []int{103, 102, 104, 101}
	for i, exp := range expectedNumbers {
		if display[i].Number != exp {
			t.Errorf("pos %d: expected task #%d, got #%d (%s, priority=%s)",
				i, exp, display[i].Number, display[i].Title, display[i].Priority)
		}
	}
}

func TestTaskSubtabs_MineAndSquad(t *testing.T) {
	m := createTestTasksModel()

	// 1. Default board is My Tasks
	if m.ActiveTaskSubtab != TaskSubtabMine {
		t.Errorf("expected default subtab to be TaskSubtabMine, got %v", m.ActiveTaskSubtab)
	}

	rendered := m.renderTasksView()
	// Must contain My Tasks board header and personal tasks
	if !strings.Contains(rendered, "My Tasks (4)") {
		t.Errorf("expected rendered view to contain 'My Tasks (4)', got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "High priority personal task (newer)") {
		t.Errorf("expected rendered view to contain personal task")
	}

	// 2. Switch board to Squad Tasks via 'l'
	updated, _ := m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = updated.(Model)
	if m.ActiveTab != TabSquadTasks {
		t.Errorf("expected ActiveTab == TabSquadTasks after 'l', got %v", m.ActiveTab)
	}

	squadRendered := m.renderTasksView()
	if !strings.Contains(squadRendered, "Squad Tasks") || !strings.Contains(squadRendered, "(2)") {
		t.Errorf("expected squad view to contain 'Squad Tasks' and '(2)', got:\n%s", squadRendered)
	}
	if !strings.Contains(squadRendered, "Squad task assigned to alice") {
		t.Errorf("expected squad view to contain alice squad task")
	}
	if !strings.Contains(squadRendered, "Squad task unassigned") {
		t.Errorf("expected squad view to contain unassigned squad task")
	}

	// 3. Switch back to My Tasks via 'h'
	updated, _ = m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = updated.(Model)
	if m.ActiveTab != TabMyTasks {
		t.Errorf("expected ActiveTab == TabMyTasks after 'h', got %v", m.ActiveTab)
	}
}

func TestTaskInlineAddingAndCreation(t *testing.T) {
	m := createTestTasksModel()

	// 1. Press 'a' to add task inline
	res, cmd := m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = res.(Model)
	if cmd == nil {
		t.Errorf("expected textinput blink command on 'a'")
	}
	if !m.InlineAdding {
		t.Fatalf("expected InlineAdding == true after pressing 'a'")
	}
	if m.InlineNewPriority != github.PriorityMedium {
		t.Errorf("expected default priority Medium, got %v", m.InlineNewPriority)
	}

	// Check table view shows inline draft row
	rendered := m.renderTasksView()
	if !strings.Contains(rendered, "(new)") {
		t.Errorf("expected view to render (new) row while inline adding")
	}

	// 2. Type title and press Enter to submit
	m.InlineNewTitleInput.SetValue("Buy new test hardware")
	res, cmd = m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if cmd == nil {
		t.Fatalf("expected createTaskCmd on Enter")
	}
	if m.InlineAdding {
		t.Errorf("expected InlineAdding == false after submitting")
	}

	// 3. Deliver TaskCreatedMsg
	newTask := github.Task{
		ID:          99,
		Number:      105,
		Title:       "Buy new test hardware",
		Priority:    github.PriorityMedium,
		RepoName:    "tasks",
		IsSquadTask: false,
	}
	res, _ = m.Update(TaskCreatedMsg{Task: &newTask})
	m = res.(Model)

	if len(m.PersonalTasks) != 5 {
		t.Errorf("expected 5 personal tasks after creation, got %d", len(m.PersonalTasks))
	}
}

func TestTaskInlineEditing(t *testing.T) {
	m := createTestTasksModel()
	m.TaskCursor = 0 // Top task is #103

	// 1. Press 'Enter' on active task to start editing title inline
	res, cmd := m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if cmd == nil {
		t.Errorf("expected blink command on Enter to edit")
	}
	if !m.InlineEditing {
		t.Fatalf("expected InlineEditing == true")
	}
	if m.InlineEditTitleInput.Value() != "High priority personal task (newer)" {
		t.Errorf("expected edit input pre-populated with title, got %q", m.InlineEditTitleInput.Value())
	}

	// 2. Change title and press Enter
	m.InlineEditTitleInput.SetValue("Updated Title Inline")
	res, cmd = m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if cmd == nil {
		t.Fatalf("expected updateTaskCmd on Enter when editing title")
	}
	if m.InlineEditing {
		t.Errorf("expected InlineEditing == false after submitting title")
	}
}

func TestTaskInlineEditing_HotkeysDisabledUntilFinished(t *testing.T) {
	m := createTestTasksModel()
	m.ActiveTab = TabTasks
	m.TaskCursor = 0 // Top task

	// 1. Enter edit mode via m.Update
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if !m.InlineEditing {
		t.Fatalf("expected InlineEditing == true")
	}

	// 2. Header and Footer should reflect editing mode without regular hotkeys
	footer := m.renderFooter()
	if !strings.Contains(footer, "[Enter] Save Title  [Esc] Cancel") {
		t.Errorf("footer should show save/cancel prompt, got: %s", footer)
	}
	if strings.Contains(footer, "[a] Add") || strings.Contains(footer, "[Space] Toggle Done") || strings.Contains(footer, "[?] Help") {
		t.Errorf("footer should NOT display regular task hotkeys while editing title: %s", footer)
	}

	header := m.renderTasksView()
	if !strings.Contains(header, "[Editing Title: Enter to save, Esc to cancel]") {
		t.Errorf("header should show editing banner, got: %s", header)
	}
	if strings.Contains(header, "[a] Add Task") || strings.Contains(header, "[Space] Done") {
		t.Errorf("header banner should NOT display task hotkeys while editing: %s", header)
	}

	// 3. Global hotkey 'q' must NOT quit - it should type 'q' into input
	res, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = res.(Model)
	if cmd != nil {
		// Verify cmd is not tea.Quit
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatalf("'q' while editing title should NOT quit the application!")
		}
	}
	if !m.InlineEditing {
		t.Fatalf("InlineEditing should remain true while typing")
	}

	// 4. Global hotkey '1' must NOT switch to Repos tab
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = res.(Model)
	if m.ActiveTab != TabTasks {
		t.Fatalf("expected ActiveTab to stay TabTasks, got %d", m.ActiveTab)
	}

	// 5. Global hotkey '?' must NOT open Help modal
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = res.(Model)
	if m.ActiveModal != ModalNone {
		t.Fatalf("expected ActiveModal to remain ModalNone, got %v", m.ActiveModal)
	}

	// 6. Global hotkey 'r' must NOT trigger refresh
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = res.(Model)
	if m.Loading {
		t.Fatalf("'r' should NOT trigger data refresh while editing title")
	}

	// 7. Tasks hotkey 'a' must NOT start InlineAdding
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = res.(Model)
	if m.InlineAdding {
		t.Fatalf("'a' should NOT trigger InlineAdding while editing title")
	}

	// 8. Tasks hotkey 'd' must NOT open Completed Tasks modal
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = res.(Model)
	if m.ActiveModal == ModalCompletedTasks {
		t.Fatalf("'d' should NOT open ModalCompletedTasks while editing title")
	}

	// 9. Tasks hotkey 'Space' must NOT toggle done
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = res.(Model)
	if len(m.PersonalDoneTasks) != 0 { // Started with 0 done tasks, should remain 0
		t.Fatalf("Space should NOT toggle task done while editing title")
	}

	// 10. Pressing 'Esc' finishes editing and restores hotkeys
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(Model)
	if m.InlineEditing {
		t.Fatalf("expected InlineEditing == false after pressing Esc")
	}

	// Normal hotkeys should be restored in footer
	restoredFooter := m.renderFooter()
	if !strings.Contains(restoredFooter, "[Enter] Edit Title") || !strings.Contains(restoredFooter, "[a] Add") {
		t.Errorf("expected regular hotkeys restored after exiting edit mode, got: %s", restoredFooter)
	}

	// 11. Test Ctrl+C also finishes editing without quitting
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if !m.InlineEditing {
		t.Fatalf("expected InlineEditing == true")
	}

	res, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = res.(Model)
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatalf("Ctrl+C while editing title should cancel editing, not quit")
		}
	}
	if m.InlineEditing {
		t.Fatalf("expected InlineEditing == false after Ctrl+C")
	}
}

func TestTaskNotesModal(t *testing.T) {
	m := createTestTasksModel()
	m.TaskCursor = 3 // select #101 which has body notes "Has some body notes here"

	// 1. Check notes indicator 📝 in view
	rendered := m.renderTasksView()
	if !strings.Contains(rendered, "📝") {
		t.Errorf("expected table to show 📝 indicator for task with body")
	}

	// 2. Press 'n' to open notes modal
	res, cmd := m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = res.(Model)
	if cmd == nil {
		t.Errorf("expected command on opening notes modal")
	}
	if m.ActiveModal != ModalTaskNotes {
		t.Fatalf("expected ActiveModal == ModalTaskNotes, got %v", m.ActiveModal)
	}
	if m.TaskNotesInput.Value() != "Has some body notes here" {
		t.Errorf("expected notes textarea pre-populated, got %q", m.TaskNotesInput.Value())
	}

	// 3. Render modal check
	modalStr := m.renderTaskNotesModal()
	if !strings.Contains(modalStr, "Task Notes: #101") {
		t.Errorf("expected modal header with task number, got:\n%s", modalStr)
	}
	if !strings.Contains(modalStr, "Has some body notes here") {
		t.Errorf("expected modal content to contain notes")
	}
	if !strings.Contains(modalStr, "[Esc] Save Notes") {
		t.Errorf("expected modal to show '[Esc] Save Notes'")
	}

	// 4. Edit notes and press 'Esc' to save
	m.TaskNotesInput.SetValue("Line 1 updated notes\nLine 2 updated notes")
	res, cmd = m.handleTaskNotesModalKeys(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(Model)
	if cmd == nil {
		t.Fatalf("expected updateTaskCmd when saving notes with Esc")
	}
	if m.ActiveModal != ModalNone {
		t.Errorf("expected modal to close after saving with Esc")
	}
}

func TestTaskActiveAndDoneOnSamePage(t *testing.T) {
	m := createTestTasksModel()
	now := time.Now()

	time1 := now.Add(-30 * time.Minute)
	time2 := now.Add(-5 * time.Minute) // More recently completed

	m.PersonalDoneTasks = []github.Task{
		{
			ID:          201,
			Number:      301,
			Title:       "Done task 1 (30m ago)",
			State:       "closed",
			Priority:    github.PriorityLow,
			ClosedAt:    &time1,
			UpdatedAt:   time1,
			RepoName:    "tasks",
			IsSquadTask: false,
		},
		{
			ID:          202,
			Number:      302,
			Title:       "Done task 2 (5m ago)",
			State:       "closed",
			Priority:    github.PriorityHigh,
			ClosedAt:    &time2,
			UpdatedAt:   time2,
			RepoName:    "tasks",
			IsSquadTask: false,
		},
	}

	// 1. Rendering on the same page check:
	// Shows Active tasks at the top, and Completed section at the bottom
	rendered := m.renderTasksView()
	if !strings.Contains(rendered, "High priority personal task (newer)") {
		t.Errorf("expected view to contain active tasks")
	}
	if !strings.Contains(rendered, "COMPLETED (Last 2") {
		t.Errorf("expected view to contain 'COMPLETED (Last 2' separator, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "✓ Done task 2 (5m ago)") {
		t.Errorf("expected view to contain done task 2")
	}

	// 2. Move cursor down into completed tasks
	// Active tasks = 4 (indices 0..3). Completed tasks = 2 (indices 4..5).
	m.TaskCursor = 4 // On first completed task (#302, most recently closed)
	loc, task := m.getCursorTarget()
	if loc != locCompletedTask || task == nil || task.Number != 302 {
		t.Fatalf("expected cursor on completed task #302, got loc=%v, task=%v", loc, task)
	}

	// 3. Press Space on completed task -> reopens it!
	res, cmd := m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = res.(Model)
	if cmd == nil {
		t.Fatalf("expected reopenTaskCmd when pressing space on completed task")
	}

	// Deliver TaskReopenedMsg
	reopenedMsg := TaskReopenedMsg{
		TaskID:   202,
		Number:   302,
		RepoName: "tasks",
		IsSquad:  false,
		Err:      nil,
	}
	res, _ = m.Update(reopenedMsg)
	m = res.(Model)

	if len(m.PersonalDoneTasks) != 1 {
		t.Errorf("expected 1 remaining done task, got %d", len(m.PersonalDoneTasks))
	}
	if len(m.PersonalTasks) != 5 {
		t.Errorf("expected 5 active tasks after reopen, got %d", len(m.PersonalTasks))
	}

	// 4. Press Space on active task -> completes it!
	m.TaskCursor = 0
	res, cmd = m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = res.(Model)
	if cmd == nil {
		t.Fatalf("expected closeTaskCmd when pressing space on active task")
	}
}

func TestTaskCompletedPopupModal(t *testing.T) {
	m := createTestTasksModel()
	now := time.Now()
	timeClosed := now.Add(-10 * time.Minute)

	m.PersonalDoneTasks = []github.Task{
		{
			ID:          501,
			Number:      401,
			Title:       "Completed task ready for popup",
			State:       "closed",
			ClosedAt:    &timeClosed,
			RepoOwner:   "tester",
			RepoName:    "tasks",
			IsSquadTask: false,
		},
	}

	// 1. Press 'd' to open Completed Tasks popup modal
	res, _ := m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = res.(Model)

	if m.ActiveModal != ModalCompletedTasks {
		t.Fatalf("expected ActiveModal == ModalCompletedTasks on 'd', got %v", m.ActiveModal)
	}

	// Check modal rendering
	modalStr := m.renderCompletedTasksModal()
	if !strings.Contains(modalStr, "Completed Tasks (1)") {
		t.Errorf("expected modal header with count, got:\n%s", modalStr)
	}
	if !strings.Contains(modalStr, "✓ Completed task ready for popup") {
		t.Errorf("expected modal to list completed task")
	}

	// 2. Press Space in modal to reopen task
	res, cmd := m.handleCompletedTasksModalKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = res.(Model)
	if cmd == nil {
		t.Fatalf("expected reopenTaskCmd from modal")
	}

	// 3. Press 'Esc' to close modal
	res, _ = m.handleCompletedTasksModalKeys(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(Model)
	if m.ActiveModal != ModalNone {
		t.Errorf("expected modal to close on Esc")
	}
}

func TestTaskWebAndRowSpacing(t *testing.T) {
	m := createTestTasksModel()
	m.PersonalTasks[0].URL = "https://github.com/tester/tasks/issues/101"
	m.TaskCursor = 3 // select #101

	// Press 'o' to open in browser
	res, cmd := m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	_ = res.(Model)
	if cmd == nil {
		t.Errorf("expected toast command when pressing 'o'")
	}

	// Test row spacing between table and bottom hotkey row
	view := m.renderTasksView()
	lines := strings.Split(view, "\n")
	// The last line before footer must be an empty line
	if len(lines) < 2 || lines[len(lines)-1] != "" {
		t.Errorf("expected renderTasksView to end with an empty row spacer, got last line %q", lines[len(lines)-1])
	}
}

func TestPRReviewTask_OpenModalFromPRView(t *testing.T) {
	m := createTestTasksModel()
	m.ActiveTab = TabPRs
	m.AuthUser = "simon"
	m.CurrentOrg = "my-org"
	m.PRs = []github.PullRequest{
		{
			Number: 42,
			Title:  "Fix database connection leak",
			Repo:   "backend-service",
			Author: "alice",
			URL:    "https://github.com/my-org/backend-service/pull/42",
		},
	}
	m.PRCursor = 0

	// Press 't' on PR view to open PR review task modal
	res, cmd := m.handlePRsKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = res.(Model)
	if cmd != nil {
		t.Errorf("expected nil cmd when opening modal")
	}
	if m.ActiveModal != ModalCreatePRTask {
		t.Fatalf("expected ActiveModal == ModalCreatePRTask, got %v", m.ActiveModal)
	}
	if m.PRTaskTarget.Number != 42 {
		t.Errorf("expected PRTaskTarget.Number == 42, got %d", m.PRTaskTarget.Number)
	}
	if m.PRTaskPriority != github.PriorityMedium {
		t.Errorf("expected default PRTaskPriority == medium, got %s", m.PRTaskPriority)
	}

	// Verify modal renders PR info and options
	view := m.renderCreatePRTaskModal()
	if !strings.Contains(view, "Fix database connection leak") {
		t.Errorf("expected modal to render PR title")
	}
	if !strings.Contains(view, "Assign to:") {
		t.Errorf("expected modal to render Assign to checklist header")
	}
	if !strings.Contains(view, "pr-review") {
		t.Errorf("expected modal to render pr-review label")
	}

	// Cycle priority with 'p'
	res, _ = m.handleCreatePRTaskModalKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = res.(Model)
	if m.PRTaskPriority != github.PriorityHigh {
		t.Errorf("expected priority High after pressing 'p', got %s", m.PRTaskPriority)
	}

	// Pressing Enter on member list must NOT submit (toggles selection)
	res, noSubmitCmd := m.handleCreatePRTaskModalKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if noSubmitCmd != nil {
		t.Fatalf("pressing Enter on member list should NOT submit task creation!")
	}
	if m.ActiveModal != ModalCreatePRTask {
		t.Fatalf("modal should remain open after Enter on member list")
	}

	// Tab down to buttons field (field 0 -> field 1 -> field 2)
	res, _ = m.handleCreatePRTaskModalKeys(tea.KeyMsg{Type: tea.KeyTab})
	m = res.(Model)
	if m.PRTaskFocusedField != 1 {
		t.Fatalf("expected focused field 1 (Priority) after Tab, got %d", m.PRTaskFocusedField)
	}

	res, _ = m.handleCreatePRTaskModalKeys(tea.KeyMsg{Type: tea.KeyTab})
	m = res.(Model)
	if m.PRTaskFocusedField != 2 {
		t.Fatalf("expected focused field 2 (Buttons) after second Tab, got %d", m.PRTaskFocusedField)
	}
	if m.PRTaskActiveButton != 0 {
		t.Fatalf("expected active button 0 (Create Task), got %d", m.PRTaskActiveButton)
	}

	// Submit modal with 'Enter' on [ Create Task ] button
	res, submitCmd := m.handleCreatePRTaskModalKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if submitCmd == nil {
		t.Fatalf("expected createPRTaskCmd on Enter when on Create Task button")
	}
	if m.ActiveModal != ModalNone {
		t.Errorf("expected modal to close after submitting")
	}
}

func TestPRReviewTask_SpecialRenderingAndContextualEnter(t *testing.T) {
	m := createTestTasksModel()
	m.ActiveTab = TabTasks
	m.ActiveTaskSubtab = TaskSubtabSquad
	m.Width = 100
	m.Height = 30

	now := time.Now()
	// Add a PR review task to SquadTasks
	prTask := github.Task{
		ID:          999,
		Number:      301,
		Title:       "Review PR #42: Fix database connection leak (backend-service)",
		Body:        "Review pull request: https://github.com/my-org/backend-service/pull/42\n\nRepository: backend-service\nAuthor: @alice",
		Priority:    github.PriorityHigh,
		Assignees:   []string{"bob"},
		Labels:      []string{"priority: high", github.LabelPRReview},
		RepoName:    "core-team-tasks",
		UpdatedAt:   now,
		IsSquadTask: true,
	}
	m.SquadTasks = append([]github.Task{prTask}, m.SquadTasks...)

	// Top task is now the PR review task
	m.TaskCursor = 0
	_, cursorTask := m.getCursorTarget()
	if cursorTask == nil || !cursorTask.IsPRReview() {
		t.Fatalf("expected cursor task to be a PR review task")
	}
	if cursorTask.PRURL() != "https://github.com/my-org/backend-service/pull/42" {
		t.Errorf("unexpected PRURL: %s", cursorTask.PRURL())
	}

	// 1. Verify special badge [⑂ HIGH] and ⑂ in tasks view
	view := m.renderTasksView()
	if !strings.Contains(view, "[⑂ HIGH]") {
		t.Errorf("expected view to render [⑂ HIGH] badge for PR review task, got:\n%s", view)
	}

	// 2. Verify footer displays contextual [Enter/o] Open PR
	footer := m.renderFooter()
	if !strings.Contains(footer, "[Enter/o] Open PR") {
		t.Errorf("expected footer to display '[Enter/o] Open PR', got: %s", footer)
	}
	if strings.Contains(footer, "[Enter] Edit Title") {
		t.Errorf("footer should NOT display '[Enter] Edit Title' when on a PR review task")
	}

	// 3. Press 'Enter' on PR review task -> MUST open PR, and MUST NOT enter InlineEditing!
	res, enterCmd := m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if m.InlineEditing {
		t.Errorf("expected InlineEditing == false on PR review task, title must be immutable!")
	}
	if enterCmd == nil {
		t.Fatalf("expected toast command from opening PR in browser")
	}
	toastMsg := enterCmd().(StatusToastMsg)
	if !strings.Contains(string(toastMsg), "Opened PR in browser") {
		t.Errorf("expected toast 'Opened PR in browser', got: %s", toastMsg)
	}
}

func TestPRReviewTask_CompletedRenderingAndEnter(t *testing.T) {
	m := createTestTasksModel()
	m.ActiveTab = TabTasks
	m.ActiveTaskSubtab = TaskSubtabSquad
	m.Width = 100
	m.Height = 30

	now := time.Now()
	// Add completed PR review task
	closedTask := github.Task{
		ID:          998,
		Number:      300,
		Title:       "Review PR #41: Add caching layer (backend-service)",
		Body:        "Review pull request: https://github.com/my-org/backend-service/pull/41",
		Priority:    github.PriorityMedium,
		Assignees:   []string{"alice"},
		Labels:      []string{github.LabelPRReview},
		RepoName:    "core-team-tasks",
		ClosedAt:    &now,
		UpdatedAt:   now,
		IsSquadTask: true,
	}
	m.SquadDoneTasks = []github.Task{closedTask}

	// 1. Verify rendering in tasks view (recent completed section)
	view := m.renderTasksView()
	if !strings.Contains(view, "[✓⑂ MED]") {
		t.Errorf("expected view to render [✓⑂ MED] badge in completed section")
	}

	// 2. Open completed tasks modal
	m.ActiveModal = ModalCompletedTasks
	m.CompletedTasksCursor = 0

	modalView := m.renderCompletedTasksModal()
	if !strings.Contains(modalView, "[✓⑂ MED]") {
		t.Errorf("expected modal to render [✓⑂ MED]")
	}
	if !strings.Contains(modalView, "[Enter/o] Open PR") {
		t.Errorf("expected modal help line to show '[Enter/o] Open PR'")
	}

	// 3. Press Enter on completed PR review task -> opens PR in browser
	res, cmd := m.handleCompletedTasksModalKeys(tea.KeyMsg{Type: tea.KeyEnter})
	_ = res.(Model)
	if cmd == nil {
		t.Fatalf("expected command from Enter in completed tasks modal")
	}
	toastMsg := cmd().(StatusToastMsg)
	if !strings.Contains(string(toastMsg), "Opened PR in browser") {
		t.Errorf("expected toast 'Opened PR in browser', got: %s", toastMsg)
	}
}

func TestPRReviewTask_SoloSquadCreationOnEnter(t *testing.T) {
	m := createTestTasksModel()
	m.ActiveTab = TabPRs
	m.AuthUser = "simonbronner"
	m.CurrentOrg = "simonbronner"
	// Solo squad containing only the user themselves
	m.Config.Squads[0].Members = []string{"simonbronner"}
	m.PRs = []github.PullRequest{
		{
			Number: 42,
			Title:  "Fix something",
			Repo:   "grubby",
			Author: "simonbronner",
			URL:    "https://github.com/simonbronner/grubby/pull/42",
		},
	}
	m.PRCursor = 0

	// 1. Press 't' to open modal
	res, _ := m.handlePRsKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = res.(Model)
	if m.ActiveModal != ModalCreatePRTask {
		t.Fatalf("expected ModalCreatePRTask, got %v", m.ActiveModal)
	}

	// In solo squad, user should be in candidates and checked by default
	if len(m.PRTaskCandidates) == 0 {
		t.Fatalf("expected PRTaskCandidates to include simonbronner")
	}
	if !m.PRTaskSelectedMembers["simonbronner"] {
		t.Errorf("expected simonbronner to be selected by default in solo squad")
	}

	// 2. Press Enter on member list -> toggles selection, does NOT submit
	res, noCmd := m.handleCreatePRTaskModalKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if noCmd != nil {
		t.Fatalf("expected nil cmd on Enter while on member list (stopping quick-save)")
	}

	// 3. Tab down to buttons field (field 0 -> field 1 -> field 2)
	res, _ = m.handleCreatePRTaskModalKeys(tea.KeyMsg{Type: tea.KeyTab})
	m = res.(Model)
	res, _ = m.handleCreatePRTaskModalKeys(tea.KeyMsg{Type: tea.KeyTab})
	m = res.(Model)
	if m.PRTaskFocusedField != 2 {
		t.Fatalf("expected focused field 2 (Buttons), got %d", m.PRTaskFocusedField)
	}

	// 4. Press Enter on [ Create Task ] button -> MUST create task and close modal!
	res, submitCmd := m.handleCreatePRTaskModalKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if submitCmd == nil {
		t.Fatalf("expected submitCmd on Enter on Create Task button, got nil")
	}
	if m.ActiveModal != ModalNone {
		t.Errorf("expected modal to close on Enter, but ActiveModal is %v", m.ActiveModal)
	}

	// 5. Test if user clears all selections (unassigned task) and creates:
	// It must create an unassigned task and close modal, NEVER block!
	m.ActiveModal = ModalCreatePRTask
	m.PRTaskFocusedField = 2
	m.PRTaskActiveButton = 0
	m.PRTaskSelectedMembers = make(map[string]bool)
	res, submitCmdUnassigned := m.handleCreatePRTaskModalKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if submitCmdUnassigned == nil {
		t.Fatalf("expected submitCmdUnassigned on Enter, got nil (must not block!)")
	}
	if m.ActiveModal != ModalNone {
		t.Errorf("expected modal to close for unassigned task, got %v", m.ActiveModal)
	}
}

func TestTask_HotkeyI_OpensIssue(t *testing.T) {
	m := createTestTasksModel()
	m.ActiveTab = TabTasks
	m.ActiveTaskSubtab = TaskSubtabSquad
	m.Width = 100
	m.Height = 30

	now := time.Now()
	prTask := github.Task{
		ID:          999,
		Number:      301,
		Title:       "Review PR #42: Fix database connection leak (backend-service)",
		Body:        "Review pull request: https://github.com/my-org/backend-service/pull/42",
		Priority:    github.PriorityHigh,
		Assignees:   []string{"bob"},
		Labels:      []string{"priority: high", github.LabelPRReview},
		RepoName:    "core-team-tasks",
		URL:         "https://github.com/my-org/core-team-tasks/issues/301",
		UpdatedAt:   now,
		IsSquadTask: true,
	}
	m.SquadTasks = append([]github.Task{prTask}, m.SquadTasks...)
	m.TaskCursor = 0 // On PR task

	// 1. Press 'i' on PR task -> MUST open issue URL (NOT the PR URL!)
	res, cmdI := m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	_ = res.(Model)
	if cmdI == nil {
		t.Fatalf("expected toast command from 'i'")
	}
	toastMsg := string(cmdI().(StatusToastMsg))
	if !strings.Contains(toastMsg, "Opened issue #301 in browser") {
		t.Errorf("expected 'Opened issue #301 in browser', got: %s", toastMsg)
	}

	// 2. Completed modal: Press 'i' on completed task
	m.ActiveModal = ModalCompletedTasks
	m.SquadDoneTasks = []github.Task{prTask}
	m.CompletedTasksCursor = 0
	res, cmdModalI := m.handleCompletedTasksModalKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	_ = res.(Model)
	if cmdModalI == nil {
		t.Fatalf("expected toast command from 'i' in completed tasks modal")
	}
	toastModalMsg := string(cmdModalI().(StatusToastMsg))
	if !strings.Contains(toastModalMsg, "Opened issue #301 in browser") {
		t.Errorf("expected 'Opened issue #301 in browser', got: %s", toastModalMsg)
	}
}

func TestSquadTask_CapitalA_AssignMultiple(t *testing.T) {
	m := createTestTasksModel()
	m.ActiveTab = TabTasks
	m.ActiveTaskSubtab = TaskSubtabSquad
	m.Width = 100
	m.Height = 30
	m.TaskCursor = 0 // On #202 (unassigned)

	// 1. Press capital 'A' on active squad task to open Assign Squad Task modal
	res, _ := m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	m = res.(Model)
	if m.ActiveModal != ModalAssignSquadTask {
		t.Fatalf("expected ActiveModal == ModalAssignSquadTask, got %v", m.ActiveModal)
	}
	if m.AssignTaskTarget == nil || m.AssignTaskTarget.Number != 202 {
		t.Fatalf("expected target task #202, got %+v", m.AssignTaskTarget)
	}

	// Verify modal rendering contains members checklist and buttons
	view := m.renderAssignSquadTaskModal()
	if !strings.Contains(view, "Assign Squad Task #202") {
		t.Errorf("expected modal title in view: %s", view)
	}
	if !strings.Contains(view, "[ Save Assignees ]") {
		t.Errorf("expected Save Assignees button in view: %s", view)
	}

	// 2. Select first member with Space
	res, _ = m.handleAssignSquadTaskModalKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = res.(Model)
	firstMem := m.AssignTaskCandidates[0]
	if !m.AssignTaskSelectedMembers[firstMem] {
		t.Fatalf("expected first member %q to be checked after Space", firstMem)
	}

	// Move cursor down and select second member with Enter (Enter toggles, does NOT save!)
	res, _ = m.handleAssignSquadTaskModalKeys(tea.KeyMsg{Type: tea.KeyDown})
	m = res.(Model)
	if m.AssignTaskMemberCursor != 1 {
		t.Fatalf("expected cursor on index 1, got %d", m.AssignTaskMemberCursor)
	}

	res, noCmd := m.handleAssignSquadTaskModalKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if noCmd != nil {
		t.Fatalf("Enter on member checklist should toggle checkbox, not save")
	}
	secondMem := m.AssignTaskCandidates[1]
	if !m.AssignTaskSelectedMembers[secondMem] {
		t.Fatalf("expected second member %q to be checked after Enter", secondMem)
	}

	// 3. Tab down to buttons (Field 0 -> Field 1)
	res, _ = m.handleAssignSquadTaskModalKeys(tea.KeyMsg{Type: tea.KeyTab})
	m = res.(Model)
	if m.AssignTaskFocusedField != 1 {
		t.Fatalf("expected focused field 1 (Buttons), got %d", m.AssignTaskFocusedField)
	}
	if m.AssignTaskActiveButton != 0 {
		t.Fatalf("expected active button 0 (Save Assignees), got %d", m.AssignTaskActiveButton)
	}

	// 4. Press Enter on [ Save Assignees ] button
	res, saveCmd := m.handleAssignSquadTaskModalKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if saveCmd == nil {
		t.Fatalf("expected saveCmd on Enter on Save Assignees button")
	}
	if m.ActiveModal != ModalNone {
		t.Fatalf("expected modal to close after saving assignees")
	}

	// Verify task in squad tasks was updated with multiple assignees
	foundTask := false
	for _, st := range m.SquadTasks {
		if st.Number == 202 {
			foundTask = true
			if len(st.Assignees) != 2 {
				t.Fatalf("expected 2 assignees on task #202, got %v", st.Assignees)
			}
			if st.Assignees[0] != firstMem || st.Assignees[1] != secondMem {
				t.Errorf("expected assignees [%s, %s], got %v", firstMem, secondMem, st.Assignees)
			}
		}
	}
	if !foundTask {
		t.Fatalf("task #202 not found in SquadTasks")
	}
}

func TestPRReviewTask_LaunchAutoUnassignsMultiples(t *testing.T) {
	m := createTestTasksModel()
	m.ActiveTab = TabTasks
	m.ActiveTaskSubtab = TaskSubtabSquad
	m.AuthUser = "simonbronner"
	m.Width = 100
	m.Height = 30

	now := time.Now()
	multiAssigneePRTask := github.Task{
		ID:          888,
		Number:      302,
		Title:       "Review PR #42: Feature X",
		Body:        "Review pull request: https://github.com/my-org/backend-service/pull/42",
		Priority:    github.PriorityHigh,
		Assignees:   []string{"alice", "bob", "simonbronner"},
		Labels:      []string{github.LabelPRReview},
		RepoName:    "core-team-tasks",
		URL:         "https://github.com/my-org/core-team-tasks/issues/302",
		UpdatedAt:   now,
		IsSquadTask: true,
	}
	m.SquadTasks = append([]github.Task{multiAssigneePRTask}, m.SquadTasks...)
	m.TaskCursor = 0 // On multiAssigneePRTask

	// 1. Press 'Enter' to launch review -> MUST unassign everyone but simonbronner!
	res, enterCmd := m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if enterCmd == nil {
		t.Fatalf("expected cmd batch when launching review with multiple assignees")
	}

	// Verify task locally has only simonbronner assigned
	var updatedTask *github.Task
	for _, t := range m.SquadTasks {
		if t.Number == 302 {
			updatedTask = &t
			break
		}
	}
	if updatedTask == nil {
		t.Fatalf("task #302 not found")
	}
	if len(updatedTask.Assignees) != 1 || updatedTask.Assignees[0] != "simonbronner" {
		t.Fatalf("expected task to be assigned only to simonbronner, got %v", updatedTask.Assignees)
	}

	// 2. Also test with 'o' hotkey on another multi-assigned PR task
	multiAssigneePRTask2 := github.Task{
		ID:          889,
		Number:      303,
		Title:       "Review PR #43: Feature Y",
		Body:        "Review pull request: https://github.com/my-org/backend-service/pull/43",
		Priority:    github.PriorityHigh,
		Assignees:   []string{"alice", "bob"}, // simonbronner was not even in list
		Labels:      []string{github.LabelPRReview},
		RepoName:    "core-team-tasks",
		URL:         "https://github.com/my-org/core-team-tasks/issues/303",
		UpdatedAt:   now,
		IsSquadTask: true,
	}
	m.SquadTasks = append([]github.Task{multiAssigneePRTask2}, m.SquadTasks...)
	m.TaskCursor = 0 // On multiAssigneePRTask2

	res, oCmd := m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m = res.(Model)
	if oCmd == nil {
		t.Fatalf("expected cmd batch when launching review via 'o'")
	}

	for _, item := range m.SquadTasks {
		if item.Number == 303 {
			if len(item.Assignees) != 1 || item.Assignees[0] != "simonbronner" {
				t.Fatalf("expected task #303 claimed by simonbronner on 'o', got %v", item.Assignees)
			}
		}
	}
}

func TestTaskDebouncingAndLocalFirstWrites(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}
	defer db.Close()

	now := time.Now()
	task := github.Task{
		ID:        1001,
		Number:    42,
		Title:     "Original Task",
		Body:      "Body",
		State:     "open",
		Priority:  github.PriorityMedium,
		RepoName:  "tasks",
		RepoOwner: "tester",
		CreatedAt: now,
		UpdatedAt: now,
	}

	_ = db.SaveTask(&task, "synced")

	var coalescedCount int64
	var flushedCount int64
	debouncer := storage.NewTaskDebouncer(
		60*time.Millisecond,
		func() {
			atomic.AddInt64(&coalescedCount, 1)
		},
		func(job *storage.TaskDebounceJob, err error) {
			atomic.AddInt64(&flushedCount, 1)
		},
	)

	ieti := textinput.New()
	m := Model{
		DB:                   db,
		Debouncer:            debouncer,
		PersonalTasks:        []github.Task{task},
		ActiveTaskSubtab:     TaskSubtabMine,
		InlineEditing:        true,
		InlineEditTaskID:     1001,
		InlineEditTitleInput: ieti,
	}

	// 1. Edit 1: change title to "Edit 1"
	m.InlineEditTitleInput.SetValue("Edit 1")
	res, _ := m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)

	// In-memory model must have "Edit 1" immediately (0ms UI latency!)
	if m.PersonalTasks[0].Title != "Edit 1" {
		t.Errorf("expected immediate in-memory update to 'Edit 1', got %q", m.PersonalTasks[0].Title)
	}

	// SQLite database must have "Edit 1" immediately!
	dbTasks, _ := db.GetTasks(false, "open")
	if len(dbTasks) != 1 || dbTasks[0].Title != "Edit 1" {
		t.Fatalf("expected SQLite to be updated immediately to 'Edit 1', got %v", dbTasks)
	}

	// 2. Rapid Edit 2 (within 60ms): change title to "Edit 2"
	m.InlineEditing = true
	m.InlineEditTaskID = 1001
	m.InlineEditTitleInput.SetValue("Edit 2")
	res, _ = m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)

	if m.PersonalTasks[0].Title != "Edit 2" {
		t.Errorf("expected immediate in-memory update to 'Edit 2', got %q", m.PersonalTasks[0].Title)
	}

	// 3. Rapid Edit 3 (within 60ms): change title to "Final Edit 3"
	m.InlineEditing = true
	m.InlineEditTaskID = 1001
	m.InlineEditTitleInput.SetValue("Final Edit 3")
	res, _ = m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)

	if m.PersonalTasks[0].Title != "Final Edit 3" {
		t.Errorf("expected immediate in-memory update to 'Final Edit 3', got %q", m.PersonalTasks[0].Title)
	}

	// Wait for debounce window to fire
	time.Sleep(120 * time.Millisecond)

	// 2 edits were coalesced (2 requests saved!)
	if atomic.LoadInt64(&coalescedCount) != 2 {
		t.Errorf("expected 2 coalesced edits, got %d", coalescedCount)
	}
	// Exactly 1 flush
	if atomic.LoadInt64(&flushedCount) != 1 {
		t.Errorf("expected exactly 1 flush callback, got %d", flushedCount)
	}
}

func TestInitialModel_EagerSQLiteLoading(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}
	defer db.Close()

	now := time.Now()
	repos := []github.Repository{
		{Name: "my-cached-repo", FullName: "org/my-cached-repo", PushedAt: now},
	}
	_ = db.SaveRepositories(repos, "org")
	_ = db.SetRepoTier("my-cached-repo", storage.TierHot)

	tasks := []github.Task{
		{ID: 200, Number: 1, Title: "Cached Task", State: "open", RepoOwner: "org", RepoName: "my-cached-repo"},
	}
	_ = db.SaveTask(&tasks[0], "synced")

	_ = db.IncrementRequestsMade(10)
	_ = db.IncrementRequestsSaved(40)

	cfg := &config.Config{Version: 1}
	m := InitialModel(cfg, nil, nil, "org", db)

	// Instantaneous load from SQLite!
	if !m.InitialDataLoaded {
		t.Errorf("expected InitialDataLoaded to be true immediately from SQLite")
	}
	if m.Loading {
		t.Errorf("expected Loading to be false immediately when data is cached")
	}
	if len(m.Repos) != 1 || m.Repos[0].Name != "my-cached-repo" {
		t.Fatalf("expected 1 cached repo loaded, got %v", m.Repos)
	}
	if tier := m.getRepoTier("my-cached-repo"); tier != storage.TierHot {
		t.Errorf("expected repo tier Hot, got %v", tier)
	}
	if len(m.PersonalTasks) != 1 || m.PersonalTasks[0].Title != "Cached Task" {
		t.Fatalf("expected 1 cached personal task loaded, got %v", m.PersonalTasks)
	}
	if m.RequestsMade != 10 || m.RequestsSaved != 40 {
		t.Errorf("expected 10 made / 40 saved requests loaded, got made=%d saved=%d", m.RequestsMade, m.RequestsSaved)
	}
}
