package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		t.Errorf("expected non-nil splash image cmd when InitialDataLoaded is false")
	}

	m.InitialDataLoaded = true
	cmdLoaded := m.emitSplashImageCmd()
	if cmdLoaded != nil {
		t.Errorf("expected nil splash image cmd when InitialDataLoaded is true")
	}
}
