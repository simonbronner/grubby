package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"grubby/internal/config"
	"grubby/internal/github"
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

	// 1. Default subtab is My Tasks
	if m.ActiveTaskSubtab != TaskSubtabMine {
		t.Errorf("expected default subtab to be TaskSubtabMine, got %v", m.ActiveTaskSubtab)
	}

	rendered := m.renderTasksView()
	// Must contain subtab headers with indicators
	if !strings.Contains(rendered, "My Tasks (4)") {
		t.Errorf("expected rendered view to contain 'My Tasks (4)', got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Squad Tasks (2)") {
		t.Errorf("expected rendered view to contain 'Squad Tasks (2)', got:\n%s", rendered)
	}
	// Must contain personal tasks
	if !strings.Contains(rendered, "High priority personal task (newer)") {
		t.Errorf("expected rendered view to contain personal task")
	}

	// 2. Switch subtab to Squad Tasks via 'l'
	updated, _ := m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = updated.(Model)
	if m.ActiveTaskSubtab != TaskSubtabSquad {
		t.Errorf("expected ActiveTaskSubtab == TaskSubtabSquad after 'l', got %v", m.ActiveTaskSubtab)
	}

	squadRendered := m.renderTasksView()
	if !strings.Contains(squadRendered, "Squad task assigned to alice") {
		t.Errorf("expected squad view to contain alice squad task")
	}
	if !strings.Contains(squadRendered, "Squad task unassigned") {
		t.Errorf("expected squad view to contain unassigned squad task")
	}

	// 3. Switch back to My Tasks via 'h'
	updated, _ = m.handleTasksKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = updated.(Model)
	if m.ActiveTaskSubtab != TaskSubtabMine {
		t.Errorf("expected ActiveTaskSubtab == TaskSubtabMine after 'h', got %v", m.ActiveTaskSubtab)
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
