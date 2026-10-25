package ui

import (
	"os"
	"path/filepath"
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
