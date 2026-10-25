package search

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// RenderPreview reads a file and formats a syntax-highlighted code snippet around targetLine
func RenderPreview(absPath string, targetLine int, viewportWidth int) (string, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err), err
	}

	allLines := strings.Split(string(data), "\n")
	totalLines := len(allLines)
	if totalLines == 0 {
		return "(Empty file)", nil
	}

	start := targetLine - 15
	if start < 1 {
		start = 1
	}
	end := targetLine + 25
	if end > totalLines {
		end = totalLines
	}

	windowText := strings.Join(allLines[start-1:end], "\n")

	// Determine lexer
	lexer := lexers.Match(filepath.Base(absPath))
	if lexer == nil {
		lexer = lexers.Analyse(windowText)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("dracula")
	if style == nil {
		style = styles.Fallback
	}

	iterator, err := lexer.Tokenise(nil, windowText)
	if err != nil {
		return windowText, nil
	}

	var buf bytes.Buffer
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	if err := formatter.Format(&buf, style, iterator); err != nil {
		return windowText, nil
	}

	highlightedLines := strings.Split(buf.String(), "\n")

	var result strings.Builder
	for i, hLine := range highlightedLines {
		currentLineNum := start + i
		if currentLineNum > end {
			break
		}

		if currentLineNum == targetLine {
			// Distinct indicator and background highlight
			result.WriteString(fmt.Sprintf("\x1b[1;33m▶ %4d \x1b[0m│ \x1b[48;5;236m%s\x1b[0m\n", currentLineNum, hLine))
		} else {
			result.WriteString(fmt.Sprintf("\x1b[90m  %4d │ \x1b[0m%s\n", currentLineNum, hLine))
		}
	}

	return result.String(), nil
}
