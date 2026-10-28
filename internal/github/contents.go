package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

var (
	ErrFileNotFound = errors.New("file not found on GitHub")
	ErrConflict     = errors.New("conflict writing file to GitHub (HTTP 409)")
)

// GitHubFileContentResponse represents the GitHub Contents API response
type GitHubFileContentResponse struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	SHA      string `json:"sha"`
	Size     int    `json:"size"`
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

// GitHubFileCommitPayload is sent to PUT /repos/{owner}/{repo}/contents/{path}
type GitHubFileCommitPayload struct {
	Message string `json:"message"`
	Content string `json:"content"` // base64
	SHA     string `json:"sha,omitempty"`
	Branch  string `json:"branch,omitempty"`
}

// SquadDefinition defines a single squad, its roster, and its owned repositories in squads.json.
type SquadDefinition struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description,omitempty"`
	Members     []string `json:"members"`
	Repos       []string `json:"repos,omitempty"`
}

// SquadsFile represents the schema of squads.json in the central repository.
type SquadsFile struct {
	Version   int               `json:"version"`
	UpdatedAt time.Time         `json:"updated_at"`
	UpdatedBy string            `json:"updated_by,omitempty"`
	Squads    []SquadDefinition `json:"squads"`
}

// LabelInfo represents the metadata of a label, including its color.
type LabelInfo struct {
	Color       string `json:"color"`                 // 6-digit hex color without '#' (e.g. "1d76db")
	Description string `json:"description,omitempty"` // Optional description
}

// SquadRepoLabelsFile represents the JSON structure of .grubby/repo-labels.json
type SquadRepoLabelsFile struct {
	Version   int                  `json:"version"`
	UpdatedAt time.Time            `json:"updated_at"`
	Labels    map[string]LabelInfo `json:"labels"`   // labelName -> LabelInfo
	Mappings  map[string][]string  `json:"mappings"` // repoID -> []label
}

// GenerateLabelColor generates a distinct, aesthetically pleasing 6-character hex color
// using golden-angle (137.508°) hue distribution in HSL color space.
func GenerateLabelColor(index int) string {
	goldenAngle := 137.50776405003785
	hue := math.Mod(float64(index)*goldenAngle, 360.0)
	saturation := 0.70
	lightness := 0.60
	return hslToHex(hue, saturation, lightness)
}

func hslToHex(h, s, l float64) string {
	c := (1.0 - math.Abs(2.0*l-1.0)) * s
	x := c * (1.0 - math.Abs(math.Mod(h/60.0, 2.0)-1.0))
	m := l - c/2.0

	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	ri := int(math.Round((r + m) * 255))
	gi := int(math.Round((g + m) * 255))
	bi := int(math.Round((b + m) * 255))
	return fmt.Sprintf("%02x%02x%02x", ri, gi, bi)
}

// GetFileContent fetches a file from a repository using GitHub's Contents API
func (c *Client) GetFileContent(ctx context.Context, owner, repo, path string) ([]byte, string, error) {
	if c == nil || c.httpClient == nil {
		return nil, "", fmt.Errorf("GitHub client not initialized")
	}

	cleanPath := strings.TrimPrefix(path, "/")
	apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.apiBaseURLOrDefault(), owner, repo, cleanPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, "", err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", ErrFileNotFound
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := ioReadAll(resp.Body)
		return nil, "", fmt.Errorf("GitHub Contents API error (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var fileResp GitHubFileContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&fileResp); err != nil {
		return nil, "", fmt.Errorf("failed to decode GitHub file response: %w", err)
	}

	// Remove newlines and decode base64
	cleanContent := strings.ReplaceAll(fileResp.Content, "\n", "")
	cleanContent = strings.ReplaceAll(cleanContent, "\r", "")
	decodedBytes, err := base64.StdEncoding.DecodeString(cleanContent)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode base64 content: %w", err)
	}

	return decodedBytes, fileResp.SHA, nil
}

// PutFileContent writes or updates a file in a repository using GitHub's Contents API
func (c *Client) PutFileContent(ctx context.Context, owner, repo, path string, content []byte, sha, message string) (string, error) {
	if c == nil || c.httpClient == nil {
		return "", fmt.Errorf("GitHub client not initialized")
	}

	cleanPath := strings.TrimPrefix(path, "/")
	apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.apiBaseURLOrDefault(), owner, repo, cleanPath)

	encodedContent := base64.StdEncoding.EncodeToString(content)
	payload := GitHubFileCommitPayload{
		Message: message,
		Content: encodedContent,
		SHA:     sha,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return "", ErrConflict
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := ioReadAll(resp.Body)
		return "", fmt.Errorf("GitHub Contents API error (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Content struct {
			SHA string `json:"sha"`
		} `json:"content"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	return result.Content.SHA, nil
}

// SaveSquadRepoLabels persists repo-to-label bindings in .grubby/repo-labels.json
// with Compare-And-Swap (CAS) conflict resolution on HTTP 409.
func (c *Client) SaveSquadRepoLabels(ctx context.Context, owner, squadTasksRepo string, updateFn func(mappings map[string][]string) error) error {
	if owner == "" || squadTasksRepo == "" {
		return nil
	}

	const path = ".grubby/repo-labels.json"
	const maxRetries = 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		contentBytes, sha, err := c.GetFileContent(ctx, owner, squadTasksRepo, path)
		if err != nil && !errors.Is(err, ErrFileNotFound) {
			return err
		}

		var fileData SquadRepoLabelsFile
		if len(contentBytes) > 0 {
			_ = json.Unmarshal(contentBytes, &fileData)
		}
		if fileData.Mappings == nil {
			fileData.Mappings = make(map[string][]string)
		}

		if err := updateFn(fileData.Mappings); err != nil {
			return err
		}

		if fileData.Labels == nil {
			fileData.Labels = make(map[string]LabelInfo)
		}

		// 1. Gather all active labels across mappings and clean empty mappings
		usedLabels := make(map[string]bool)
		for repoKey, labels := range fileData.Mappings {
			var cleanList []string
			seenForRepo := make(map[string]bool)
			for _, lbl := range labels {
				clean := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(lbl), "#"))
				clean = strings.ReplaceAll(clean, " ", "-")
				if clean != "" && !seenForRepo[clean] {
					seenForRepo[clean] = true
					cleanList = append(cleanList, clean)
					usedLabels[clean] = true
				}
			}
			if len(cleanList) > 0 {
				fileData.Mappings[repoKey] = cleanList
			} else {
				delete(fileData.Mappings, repoKey)
			}
		}

		// 2. Garbage collect: remove any label that is no longer used by any repository
		for lbl := range fileData.Labels {
			if !usedLabels[lbl] {
				delete(fileData.Labels, lbl)
			}
		}

		// 3. Assign distinct colors to any new labels without colors
		usedColors := make(map[string]bool)
		for _, info := range fileData.Labels {
			if info.Color != "" {
				usedColors[strings.ToLower(strings.TrimPrefix(info.Color, "#"))] = true
			}
		}

		var sortedLabels []string
		for lbl := range usedLabels {
			sortedLabels = append(sortedLabels, lbl)
		}
		sort.Strings(sortedLabels)

		colorIdx := 0
		for _, lbl := range sortedLabels {
			info, exists := fileData.Labels[lbl]
			if !exists || info.Color == "" {
				for {
					candidate := GenerateLabelColor(colorIdx)
					colorIdx++
					if !usedColors[candidate] {
						info.Color = candidate
						usedColors[candidate] = true
						break
					}
				}
				fileData.Labels[lbl] = info
			}
		}

		fileData.Version = 1
		fileData.UpdatedAt = time.Now()

		newContentBytes, err := json.MarshalIndent(fileData, "", "  ")
		if err != nil {
			return err
		}

		msg := "chore(grubby): update repository labels [skip ci]"
		_, err = c.PutFileContent(ctx, owner, squadTasksRepo, path, newContentBytes, sha, msg)
		if err == nil {
			return nil
		}

		if !errors.Is(err, ErrConflict) {
			return err
		}

		// Conflict (HTTP 409): exponential backoff with retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(100*(1<<attempt)) * time.Millisecond):
		}
	}

	return fmt.Errorf("failed to save repository labels after %d retries due to concurrent updates", maxRetries)
}

// FetchSquadRepoLabels loads .grubby/repo-labels.json from the squad tasks repo.
func (c *Client) FetchSquadRepoLabels(ctx context.Context, owner, squadTasksRepo string) (*SquadRepoLabelsFile, error) {
	if owner == "" || squadTasksRepo == "" {
		return nil, nil
	}

	reposToTry := []string{squadTasksRepo}
	if squadTasksRepo == "grubby-tasks" {
		reposToTry = append(reposToTry, "tasks", ".github")
	} else if squadTasksRepo == "tasks" {
		reposToTry = append(reposToTry, "grubby-tasks", ".github")
	}

	for _, r := range reposToTry {
		for _, p := range []string{".grubby/repo-labels.json", "repo-labels.json"} {
			contentBytes, _, err := c.GetFileContent(ctx, owner, r, p)
			if err == nil {
				var fileData SquadRepoLabelsFile
				if err := json.Unmarshal(contentBytes, &fileData); err == nil {
					if fileData.Labels == nil {
						fileData.Labels = make(map[string]LabelInfo)
					}
					if fileData.Mappings == nil {
						fileData.Mappings = make(map[string][]string)
					}
					return &fileData, nil
				}
			}
		}
	}

	return &SquadRepoLabelsFile{
		Version:  1,
		Labels:   make(map[string]LabelInfo),
		Mappings: make(map[string][]string),
	}, nil
}


// FetchSquadsFile loads squads.json (or .grubby/squads.json) from the central tasks repo.
func (c *Client) FetchSquadsFile(ctx context.Context, owner, repo string) (*SquadsFile, string, error) {
	if owner == "" || repo == "" {
		return nil, "", nil
	}

	reposToTry := []string{repo}
	if repo == "grubby-tasks" {
		reposToTry = append(reposToTry, "tasks", ".github")
	} else if repo == "tasks" {
		reposToTry = append(reposToTry, "grubby-tasks", ".github")
	}

	pathsToTry := []string{"squads.json", ".grubby/squads.json"}

	var lastErr error
	for _, r := range reposToTry {
		for _, p := range pathsToTry {
			contentBytes, sha, err := c.GetFileContent(ctx, owner, r, p)
			if err == nil {
				var fileData SquadsFile
				if err := json.Unmarshal(contentBytes, &fileData); err != nil {
					return nil, "", fmt.Errorf("failed to parse squads.json: %w", err)
				}
				return &fileData, sha, nil
			}
			lastErr = err
		}
	}

	if lastErr != nil && errors.Is(lastErr, ErrFileNotFound) {
		return &SquadsFile{
			Version: 1,
			Squads:  nil,
		}, "", ErrFileNotFound
	}

	return nil, "", lastErr
}

// SaveSquadsFile persists squad structures in squads.json with Compare-And-Swap (CAS) conflict resolution.
func (c *Client) SaveSquadsFile(ctx context.Context, owner, repo, updatedBy string, updateFn func(squads *[]SquadDefinition) error) error {
	if owner == "" || repo == "" {
		return nil
	}

	const maxRetries = 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		targetRepo := repo
		targetPath := "squads.json"
		contentBytes, sha, err := c.GetFileContent(ctx, owner, targetRepo, targetPath)
		if err != nil && errors.Is(err, ErrFileNotFound) {
			// Check if .grubby/squads.json already exists
			gbBytes, gbSha, gbErr := c.GetFileContent(ctx, owner, targetRepo, ".grubby/squads.json")
			if gbErr == nil {
				targetPath = ".grubby/squads.json"
				contentBytes = gbBytes
				sha = gbSha
				err = nil
			} else if repo == "grubby-tasks" {
				// Fallback to tasks repo if grubby-tasks is not found
				if tBytes, tSha, tErr := c.GetFileContent(ctx, owner, "tasks", "squads.json"); tErr == nil {
					targetRepo = "tasks"
					targetPath = "squads.json"
					contentBytes = tBytes
					sha = tSha
					err = nil
				} else if gtBytes, gtSha, gtErr := c.GetFileContent(ctx, owner, "tasks", ".grubby/squads.json"); gtErr == nil {
					targetRepo = "tasks"
					targetPath = ".grubby/squads.json"
					contentBytes = gtBytes
					sha = gtSha
					err = nil
				}
			}
		}

		if err != nil && !errors.Is(err, ErrFileNotFound) {
			return err
		}

		var fileData SquadsFile
		if len(contentBytes) > 0 {
			_ = json.Unmarshal(contentBytes, &fileData)
		}

		if err := updateFn(&fileData.Squads); err != nil {
			return err
		}

		// Normalize members and repos: clean handles/names, deduplicate
		for i := range fileData.Squads {
			var cleanMembers []string
			seen := make(map[string]bool)
			for _, m := range fileData.Squads[i].Members {
				clean := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(m), "@"))
				if clean != "" && !seen[clean] {
					seen[clean] = true
					cleanMembers = append(cleanMembers, clean)
				}
			}
			fileData.Squads[i].Members = cleanMembers

			var cleanRepos []string
			seenRepo := make(map[string]bool)
			for _, r := range fileData.Squads[i].Repos {
				cleanR := strings.ToLower(strings.TrimSpace(r))
				if cleanR != "" && !seenRepo[cleanR] {
					seenRepo[cleanR] = true
					cleanRepos = append(cleanRepos, cleanR)
				}
			}
			fileData.Squads[i].Repos = cleanRepos

			if fileData.Squads[i].DisplayName == "" {
				fileData.Squads[i].DisplayName = fileData.Squads[i].Name
			}
		}

		fileData.Version = 1
		fileData.UpdatedAt = time.Now()
		if updatedBy != "" {
			fileData.UpdatedBy = updatedBy
		}

		newBytes, err := json.MarshalIndent(fileData, "", "  ")
		if err != nil {
			return err
		}

		msg := "chore(grubby): update squad definitions [skip ci]"
		_, err = c.PutFileContent(ctx, owner, targetRepo, targetPath, newBytes, sha, msg)
		if err == nil {
			return nil
		}

		if !errors.Is(err, ErrConflict) {
			return err
		}

		// Conflict (HTTP 409): exponential backoff and retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(100*(1<<attempt)) * time.Millisecond):
		}
	}

	return fmt.Errorf("failed to save squads.json after %d retries due to concurrent updates", maxRetries)
}

func ioReadAll(r ioReader) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 512))
	_, err := buf.ReadFrom(r)
	return buf.Bytes(), err
}

type ioReader interface {
	Read(p []byte) (n int, err error)
}
