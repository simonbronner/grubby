package search

import (
	"bufio"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
)

type RipgrepEngine struct {
	rgPath string
}

func NewRipgrepEngine() *RipgrepEngine {
	path, _ := exec.LookPath("rg")
	return &RipgrepEngine{rgPath: path}
}

func (r *RipgrepEngine) Available() bool {
	return r.rgPath != ""
}

type rgMessage struct {
	Type string `json:"type"`
	Data struct {
		Path struct {
			Text string `json:"text"`
		} `json:"path"`
		Lines struct {
			Text string `json:"text"`
		} `json:"lines"`
		LineNumber int `json:"line_number"`
		Submatches []struct {
			Start int `json:"start"`
			End   int `json:"end"`
		} `json:"submatches"`
	} `json:"data"`
}

func (r *RipgrepEngine) Search(ctx context.Context, query string, rootPaths []string) ([]RepoMatches, error) {
	if len(rootPaths) == 0 || query == "" {
		return nil, nil
	}

	args := []string{"--json", "-i", "--max-count=15", "--max-columns=200", "-e", query}
	args = append(args, rootPaths...)

	cmd := exec.CommandContext(ctx, r.rgPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	repoMap := make(map[string]*RepoMatches)
	fileMap := make(map[string]*FileMatches) // absPath -> FileMatches

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg rgMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		if msg.Type == "match" {
			filePath := msg.Data.Path.Text
			absPath, err := filepath.Abs(filePath)
			if err != nil {
				absPath = filePath
			}

			// Identify which repo this file belongs to
			repoName := ""
			repoRoot := ""
			for _, root := range rootPaths {
				rel, err := filepath.Rel(root, absPath)
				if err == nil && !strings.HasPrefix(rel, "..") {
					repoName = filepath.Base(root)
					repoRoot = root
					break
				}
			}
			if repoName == "" {
				repoName = filepath.Base(filepath.Dir(absPath))
				repoRoot = filepath.Dir(absPath)
			}

			colStart := 0
			colEnd := 0
			if len(msg.Data.Submatches) > 0 {
				colStart = msg.Data.Submatches[0].Start
				colEnd = msg.Data.Submatches[0].End
			}

			lineText := strings.TrimRight(msg.Data.Lines.Text, "\r\n")

			lMatch := LineMatch{
				LineNumber: msg.Data.LineNumber,
				LineText:   lineText,
				ColStart:   colStart,
				ColEnd:     colEnd,
			}

			relPath, _ := filepath.Rel(repoRoot, absPath)
			if relPath == "" {
				relPath = filepath.Base(absPath)
			}

			fm, ok := fileMap[absPath]
			if !ok {
				fm = &FileMatches{
					RelPath: relPath,
					AbsPath: absPath,
				}
				fileMap[absPath] = fm

				rm, rok := repoMap[repoName]
				if !rok {
					rm = &RepoMatches{
						RepoName: repoName,
						RepoPath: repoRoot,
					}
					repoMap[repoName] = rm
				}
				rm.FileMatches = append(rm.FileMatches, *fm)
			}

			// Add match to repo
			rm := repoMap[repoName]
			rm.TotalHits++
			for i := range rm.FileMatches {
				if rm.FileMatches[i].AbsPath == absPath {
					rm.FileMatches[i].Matches = append(rm.FileMatches[i].Matches, lMatch)
					break
				}
			}
		}
	}

	_ = cmd.Wait()

	var result []RepoMatches
	for _, rm := range repoMap {
		result = append(result, *rm)
	}
	return result, nil
}
