package ui

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"strings"
)

// RenderAvatarArt decodes an image and renders it as ANSI 24-bit TrueColor half-blocks (▀ and ▄)
// with an anti-aliased circular mask.
func RenderAvatarArt(imgData []byte, targetWidth, targetHeight int, circular bool) (string, error) {
	if len(imgData) == 0 {
		return "", fmt.Errorf("empty image data")
	}

	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return "", err
	}

	bounds := img.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()
	if origWidth == 0 || origHeight == 0 {
		return "", fmt.Errorf("invalid image bounds")
	}

	if targetWidth <= 0 {
		targetWidth = 8
	}
	if targetHeight <= 0 {
		targetHeight = 4
	}

	totalPixelHeight := targetHeight * 2
	var lines []string

	cx := float64(targetWidth-1) / 2.0
	cy := float64(totalPixelHeight-1) / 2.0

	for row := 0; row < targetHeight; row++ {
		var line strings.Builder
		y1 := row * 2
		y2 := row*2 + 1

		for x := 0; x < targetWidth; x++ {
			inCircle1 := true
			inCircle2 := true
			if circular {
				dx := (float64(x) - cx) / cx
				dy1 := (float64(y1) - cy) / cy
				dy2 := (float64(y2) - cy) / cy
				if math.Sqrt(dx*dx+dy1*dy1) > 1.05 {
					inCircle1 = false
				}
				if math.Sqrt(dx*dx+dy2*dy2) > 1.05 {
					inCircle2 = false
				}
			}

			srcX := bounds.Min.X + (x * origWidth / targetWidth)
			srcY1 := bounds.Min.Y + (y1 * origHeight / totalPixelHeight)
			r1, g1, b1, a1 := img.At(srcX, srcY1).RGBA()

			srcY2 := bounds.Min.Y + (y2 * origHeight / totalPixelHeight)
			r2, g2, b2, a2 := img.At(srcX, srcY2).RGBA()

			R1, G1, B1, A1 := uint8(r1>>8), uint8(g1>>8), uint8(b1>>8), uint8(a1>>8)
			R2, G2, B2, A2 := uint8(r2>>8), uint8(g2>>8), uint8(b2>>8), uint8(a2>>8)

			if !inCircle1 {
				A1 = 0
			}
			if !inCircle2 {
				A2 = 0
			}

			if A1 < 30 && A2 < 30 {
				line.WriteString(" ")
			} else if A1 < 30 {
				line.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm▄\x1b[0m", R2, G2, B2))
			} else if A2 < 30 {
				line.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm▀\x1b[0m", R1, G1, B1))
			} else {
				line.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀\x1b[0m", R1, G1, B1, R2, G2, B2))
			}
		}
		lines = append(lines, line.String())
	}

	return strings.Join(lines, "\n"), nil
}

// RenderFallbackAvatar returns a neat placeholder avatar box of dimensions matching targetWidth x targetHeight.
func RenderFallbackAvatar(targetWidth, targetHeight int) string {
	if targetWidth == 8 && targetHeight == 4 {
		return "  ╭────╮  \n  │ 👤 │  \n  ╰────╯  \n          "
	}
	if targetWidth == 6 && targetHeight == 3 {
		return " ╭──╮ \n │👤│ \n ╰──╯ "
	}
	var lines []string
	for i := 0; i < targetHeight; i++ {
		lines = append(lines, strings.Repeat(" ", targetWidth))
	}
	return strings.Join(lines, "\n")
}
