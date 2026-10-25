package ui

import (
	"bytes"
	"fmt"
	_ "image/jpeg"
	"image/png"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"grubby/internal/assets"
)

var cachedSplashArt string

// RenderSplashArt decodes an image and renders it as ANSI 24-bit half-block pixel art (▀)
func RenderSplashArt(imagePath string, targetWidth int) (string, int, int) {
	if cachedSplashArt != "" && targetWidth == 34 {
		lines := strings.Split(cachedSplashArt, "\n")
		return cachedSplashArt, targetWidth, len(lines)
	}

	var data []byte
	if len(assets.GrubbyIconPNG) > 0 {
		data = assets.GrubbyIconPNG
	} else if imagePath != "" {
		var err error
		data, err = os.ReadFile(imagePath)
		if err != nil {
			return "", 0, 0
		}
	}

	if len(data) == 0 {
		return "", 0, 0
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return "", 0, 0
	}

	bounds := img.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()
	if origWidth == 0 || origHeight == 0 {
		return "", 0, 0
	}

	if targetWidth <= 0 {
		targetWidth = 36
	}

	// Calculate height in characters (each char is 2 vertical pixels)
	// Terminal characters typically have an aspect ratio of ~1:2 (width:height)
	aspect := float64(origHeight) / float64(origWidth)
	targetHeight := int(float64(targetWidth) * aspect)
	if targetHeight%2 != 0 {
		targetHeight++
	}

	var lines []string

	for y := 0; y < targetHeight; y += 2 {
		var line strings.Builder
		for x := 0; x < targetWidth; x++ {
			// Sample top pixel
			srcX := bounds.Min.X + (x * origWidth / targetWidth)
			srcY1 := bounds.Min.Y + (y * origHeight / targetHeight)
			r1, g1, b1, a1 := img.At(srcX, srcY1).RGBA()

			// Sample bottom pixel
			srcY2 := bounds.Min.Y + ((y + 1) * origHeight / targetHeight)
			var r2, g2, b2, a2 uint32
			if y+1 < targetHeight {
				r2, g2, b2, a2 = img.At(srcX, srcY2).RGBA()
			}

			// Convert 16-bit RGBA to 8-bit
			R1, G1, B1, A1 := uint8(r1>>8), uint8(g1>>8), uint8(b1>>8), uint8(a1>>8)
			R2, G2, B2, A2 := uint8(r2>>8), uint8(g2>>8), uint8(b2>>8), uint8(a2>>8)

			if A1 < 30 && A2 < 30 {
				line.WriteString("\x1b[0m ")
			} else if A1 < 30 {
				// Only bottom pixel has content: lower half block ▄
				line.WriteString(fmt.Sprintf("\x1b[0m\x1b[38;2;%d;%d;%dm▄\x1b[0m", R2, G2, B2))
			} else if A2 < 30 {
				// Only top pixel has content: upper half block ▀
				line.WriteString(fmt.Sprintf("\x1b[0m\x1b[38;2;%d;%d;%dm▀\x1b[0m", R1, G1, B1))
			} else {
				// Both pixels: top is foreground, bottom is background
				line.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀\x1b[0m", R1, G1, B1, R2, G2, B2))
			}
		}
		lines = append(lines, line.String())
	}

	art := strings.Join(lines, "\n")
	if targetWidth == 34 {
		cachedSplashArt = art
	}
	return art, targetWidth, len(lines)
}

// RenderSplashScreen renders the full centered startup splash screen
func (m Model) renderSplashScreen() string {
	iconPath := "internal/assets/grubby-icon.png"
	art, _, _ := RenderSplashArt(iconPath, 34)

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		Render("ᘛ( ᵔ ᴗ ᵔ*)ᘚ  G R U B B Y")

	subtitle := lipgloss.NewStyle().
		Foreground(SecondaryColor).
		Bold(true).
		Render("A grubby little UI for GitHub...")

	loadingMsg := fmt.Sprintf("%s Synchronizing workspaces & notifications for %s...",
		m.Spinner.View(),
		lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render("@"+m.CurrentOrg),
	)

	hint := MetaBarStyle.Render("Connecting to GitHub GraphQL API • Scanning ~/workspaces")

	var content string
	if art != "" {
		content = lipgloss.JoinVertical(lipgloss.Center,
			art,
			"",
			title,
			subtitle,
			"",
			loadingMsg,
			hint,
		)
	} else {
		// Fallback ASCII badge if image not found
		mascot := lipgloss.NewStyle().
			Bold(true).
			Background(PrimaryColor).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(1, 4).
			Render("ᘛ( ᵔ ᴗ ᵔ*)ᘚ\n  GRUBBY  ")

		content = lipgloss.JoinVertical(lipgloss.Center,
			mascot,
			"",
			title,
			subtitle,
			"",
			loadingMsg,
			hint,
		)
	}

	w := m.Width
	if w <= 0 {
		w = 80
	}
	h := m.Height
	if h <= 0 {
		h = 24
	}

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)
}
