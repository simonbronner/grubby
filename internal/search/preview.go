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

// RenderPreview reads a file and formats syntax-highlighted code with line numbers and active line indicator
func RenderPreview(absPath string, targetLine int) (string, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err), err
	}

	allLines := strings.Split(string(data), "\n")
	totalLines := len(allLines)
	if totalLines == 0 {
		return "(Empty file)", nil
	}

	// Cap at 1500 lines for snappy preview performance
	maxLines := totalLines
	if maxLines > 1500 {
		maxLines = 1500
	}

	fullText := strings.Join(allLines[:maxLines], "\n")

	// Determine lexer
	lexer := lexers.Match(filepath.Base(absPath))
	if lexer == nil {
		lexer = lexers.Analyse(fullText)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("dracula")
	if style == nil {
		style = styles.Fallback
	}

	iterator, err := lexer.Tokenise(nil, fullText)
	if err != nil {
		return fullText, nil
	}

	var buf bytes.Buffer
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	if err := formatter.Format(&buf, style, iterator); err != nil {
		return fullText, nil
	}

	highlightedLines := strings.Split(buf.String(), "\n")

	var result strings.Builder
	for i, hLine := range highlightedLines {
		currentLineNum := i + 1
		if currentLineNum > maxLines {
			break
		}

		if currentLineNum == targetLine {
			// Distinct pointer and background highlight
			result.WriteString(fmt.Sprintf("\x1b[1;33m▶ %4d \x1b[0m│ \x1b[48;5;236m%s\x1b[0m\n", currentLineNum, hLine))
		} else {
			result.WriteString(fmt.Sprintf("\x1b[90m  %4d │ \x1b[0m%s\n", currentLineNum, hLine))
		}
	}

	return result.String(), nil
}
