package ui

import (
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AvatarColors provides a curated, vibrant, high-contrast palette for deterministic user avatars.
var AvatarColors = []string{
	"#61AFEF", // Vibrant Blue
	"#98C379", // Sage Green
	"#E5C07B", // Warm Amber
	"#E06C75", // Coral Red
	"#C678DD", // Orchid Purple
	"#56B6C2", // Cyan
	"#D19A66", // Orange Ochre
	"#4FACFE", // Sky Blue
	"#00F2FE", // Mint
	"#FA709A", // Rose Pink
}

// GetUserAvatarColor deterministically hashes a GitHub username or login to a vibrant Lip Gloss color.
func GetUserAvatarColor(login string) lipgloss.Color {
	clean := strings.ToLower(strings.TrimSpace(login))
	if clean == "" {
		return lipgloss.Color("#61AFEF")
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(clean))
	idx := int(hash.Sum32()) % len(AvatarColors)
	return lipgloss.Color(AvatarColors[idx])
}

// ExtractInitials generates a 2-character monogram from a user's name or login handle.
func ExtractInitials(name, login string) string {
	cleanName := strings.TrimSpace(name)
	fields := strings.FieldsFunc(cleanName, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '-' || r == '_' || r == '.'
	})

	if len(fields) >= 2 {
		first := []rune(fields[0])
		last := []rune(fields[len(fields)-1])
		if len(first) > 0 && len(last) > 0 {
			return strings.ToUpper(string([]rune{first[0], last[0]}))
		}
	}

	target := cleanName
	if target == "" {
		target = strings.TrimSpace(login)
	}
	target = strings.TrimPrefix(target, "@")
	runes := []rune(target)
	if len(runes) == 0 {
		return "??"
	}
	if len(runes) == 1 {
		return strings.ToUpper(string(runes[0])) + " "
	}
	return strings.ToUpper(string(runes[:2]))
}

// RenderASCIIInitialsAvatar generates a clean, deterministic, multi-color ASCII/Unicode
// avatar badge with rounded borders using Lip Gloss. Zero network calls, zero corporate proxy issues.
func RenderASCIIInitialsAvatar(name, login string, targetWidth, targetHeight int) string {
	initials := ExtractInitials(name, login)
	color := GetUserAvatarColor(login)

	innerWidth := targetWidth - 2
	if innerWidth < 2 {
		innerWidth = 2
	}
	innerHeight := targetHeight - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	// For standard 8x4 contributor card avatars, render a 6x1 inner box (8x3 total) + 1 blank line padding
	if targetWidth == 8 && targetHeight == 4 {
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(color).
			Foreground(color).
			Bold(true).
			Align(lipgloss.Center, lipgloss.Center).
			Width(6).
			Height(1).
			Render(initials)
		return box + "\n" + strings.Repeat(" ", 8)
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Foreground(color).
		Bold(true).
		Align(lipgloss.Center, lipgloss.Center).
		Width(innerWidth).
		Height(innerHeight).
		Render(initials)
}

// BuildITerm2ImageSequence constructs the iTerm2 native inline graphics escape sequence.
func BuildITerm2ImageSequence(b64Data string, width, height int) string {
	seq := fmt.Sprintf("\x1b]1337;File=inline=1;width=%d;height=%d;preserveAspectRatio=1:%s\x07", width, height, b64Data)
	if os.Getenv("TMUX") != "" {
		escaped := strings.ReplaceAll(seq, "\x1b", "\x1b\x1b")
		seq = fmt.Sprintf("\x1bPtmux;%s\x1b\\", escaped)
	}
	return seq
}

// AvatarPlacement represents a positioned avatar graphic.
type AvatarPlacement struct {
	Login    string
	Row      int
	Col      int
	Width    int
	Height   int
	RawBytes []byte
}

// EmitITerm2ImagesCmd outputs multiple native iTerm2 inline image sequences.
func EmitITerm2ImagesCmd(placements []AvatarPlacement) tea.Cmd {
	if len(placements) == 0 {
		return nil
	}
	return func() tea.Msg {
		var buf strings.Builder
		for _, p := range placements {
			if len(p.RawBytes) == 0 {
				continue
			}
			b64 := base64.StdEncoding.EncodeToString(p.RawBytes)
			seq := BuildITerm2ImageSequence(b64, p.Width, p.Height)
			if p.Row > 0 && p.Col > 0 {
				buf.WriteString(fmt.Sprintf("\x1b7\x1b[%d;%dH%s\x1b8", p.Row, p.Col, seq))
			} else {
				buf.WriteString(seq)
			}
		}
		if buf.Len() > 0 {
			_, _ = os.Stdout.WriteString(buf.String())
		}
		return nil
	}
}

// EmitITerm2ImageCmd outputs a single native iTerm2 inline image sequence.
func EmitITerm2ImageCmd(rawBytes []byte, row, col, width, height int) tea.Cmd {
	if len(rawBytes) == 0 {
		return nil
	}
	return EmitITerm2ImagesCmd([]AvatarPlacement{
		{
			Row:      row,
			Col:      col,
			Width:    width,
			Height:   height,
			RawBytes: rawBytes,
		},
	})
}

// RenderAvatarBoundingBox reserves a blank rectangular bounding box using Lip Gloss
// matching the exact character cell dimensions (used by splash screen for Grubby logo).
func RenderAvatarBoundingBox(targetWidth, targetHeight int) string {
	var lines []string
	for i := 0; i < targetHeight; i++ {
		lines = append(lines, strings.Repeat(" ", targetWidth))
	}
	return strings.Join(lines, "\n")
}

// RenderFallbackAvatar returns an ASCII monogram avatar box.
func RenderFallbackAvatar(targetWidth, targetHeight int) string {
	return RenderASCIIInitialsAvatar("?", "grubby", targetWidth, targetHeight)
}
