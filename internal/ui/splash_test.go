package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRenderSplashArt(t *testing.T) {
	// Find icon path relative to repo root
	paths := []string{
		"../../internal/assets/grubby-icon.png",
		"internal/assets/grubby-icon.png",
	}

	var iconPath string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			iconPath = p
			break
		}
	}

	if iconPath == "" {
		t.Skip("Icon not found for test")
	}

	art, w, h := RenderSplashArt(iconPath, 30)
	if art == "" || w != 30 || h <= 0 {
		t.Fatalf("RenderSplashArt failed: w=%d, h=%d", w, h)
	}

	// Verify file path resolution
	abs, err := filepath.Abs(iconPath)
	if err != nil || abs == "" {
		t.Fatalf("Failed to resolve absolute path: %v", err)
	}
}

func TestRenderSplashScreen_AvatarBoundingBox(t *testing.T) {
	m := Model{
		Width:             100,
		Height:            30,
		InitialDataLoaded: false,
		SplashDismissed:   false,
		CurrentOrg:        "acme",
	}

	splash := m.renderSplashScreen()
	if !strings.Contains(splash, "G R U B B Y") {
		t.Errorf("expected splash screen to contain 'G R U B B Y'")
	}
	if !strings.Contains(splash, "@acme") {
		t.Errorf("expected splash screen to contain '@acme'")
	}

	cmd := m.emitSplashImageCmd()
	if cmd == nil {
		t.Errorf("expected non-nil splash image cmd when SplashDismissed is false")
	}

	m.SplashDismissed = true
	cmdLoaded := m.emitSplashImageCmd()
	if cmdLoaded != nil {
		t.Errorf("expected nil splash image cmd when SplashDismissed is true")
	}
}

func TestSplashScreen_DismissTimerAndInstantKeyBypass(t *testing.T) {
	m := Model{
		Width:           100,
		Height:          30,
		SplashDismissed: false,
		CurrentOrg:      "acme",
	}

	// 1. Initially view renders splash screen
	view := m.View()
	if !strings.Contains(view, "G R U B B Y") {
		t.Errorf("expected splash screen to be displayed initially")
	}

	// 2. Pressing a key immediately bypasses splash screen
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	upModel := updated.(Model)
	if !upModel.SplashDismissed {
		t.Errorf("expected splash screen dismissed on keypress")
	}

	// 3. SplashDoneMsg also dismisses splash screen
	m2 := Model{
		Width:           100,
		Height:          30,
		SplashDismissed: false,
		CurrentOrg:      "acme",
	}
	updated2, _ := m2.Update(SplashDoneMsg{})
	upModel2 := updated2.(Model)
	if !upModel2.SplashDismissed {
		t.Errorf("expected splash screen dismissed on SplashDoneMsg")
	}
}
