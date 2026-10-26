package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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

// SquadRepoLabelsFile represents the JSON structure of .grubby/repo-labels.json
type SquadRepoLabelsFile struct {
	Version   int                 `json:"version"`
	UpdatedAt time.Time           `json:"updated_at"`
	Mappings  map[string][]string `json:"mappings"` // repoID -> []label
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

func ioReadAll(r ioReader) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 512))
	_, err := buf.ReadFrom(r)
	return buf.Bytes(), err
}

type ioReader interface {
	Read(p []byte) (n int, err error)
}
