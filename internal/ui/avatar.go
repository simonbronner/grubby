package ui

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// BuildITerm2ImageSequence constructs the iTerm2 native inline graphics escape sequence.
// If running inside tmux, it wraps the sequence in tmux DCS passthrough escape codes
// so iTerm2 receives the inline image directly.
func BuildITerm2ImageSequence(b64Data string, width, height int) string {
	seq := fmt.Sprintf("\x1b]1337;File=inline=1;width=%d;height=%d;preserveAspectRatio=1:%s\x07", width, height, b64Data)
	if os.Getenv("TMUX") != "" {
		// In tmux, double every ESC character and wrap with DCS passthrough
		escaped := strings.ReplaceAll(seq, "\x1b", "\x1b\x1b")
		seq = fmt.Sprintf("\x1bPtmux;%s\x1b\\", escaped)
	}
	return seq
}

// AvatarPlacement represents a positioned avatar graphic to render via iTerm2 inline escape sequences.
type AvatarPlacement struct {
	Login    string
	Row      int
	Col      int
	Width    int
	Height   int
	RawBytes []byte
}

// EmitITerm2ImagesCmd outputs multiple native iTerm2 inline image sequences directly to the terminal
// sequentially in a single batch command, preventing race conditions or interleaving on stdout.
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
				// Save cursor (DEC \x1b7), jump to (row, col), write image, restore cursor (DEC \x1b8)
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

// EmitITerm2ImageCmd outputs a single native iTerm2 inline image sequence directly to the terminal at (row, col).
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
// matching the exact character cell dimensions (width and height) of the image.
// This ensures Lip Gloss and Bubble Tea diff layout boundaries correctly without
// overwriting the graphic cells on subsequent redraws.
func RenderAvatarBoundingBox(targetWidth, targetHeight int) string {
	var lines []string
	for i := 0; i < targetHeight; i++ {
		lines = append(lines, strings.Repeat(" ", targetWidth))
	}
	return strings.Join(lines, "\n")
}

// RenderFallbackAvatar returns a monogram placeholder avatar box of dimensions matching targetWidth x targetHeight.
func RenderFallbackAvatar(targetWidth, targetHeight int) string {
	if targetWidth == 8 && targetHeight == 4 {
		return "  ╭────╮  \n  │ 👤 │  \n  ╰────╯  \n          "
	}
	if targetWidth == 6 && targetHeight == 3 {
		return " ╭──╮ \n │👤│ \n ╰──╯ "
	}
	return RenderAvatarBoundingBox(targetWidth, targetHeight)
}
