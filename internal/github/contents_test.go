package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetFileContent_Success(t *testing.T) {
	testContent := `{"version": 1, "mappings": {"101": ["backend"]}}`
	encoded := base64.StdEncoding.EncodeToString([]byte(testContent))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/my-org/my-squad-tasks/contents/.grubby/repo-labels.json" {
			t.Fatalf("unexpected URL path: %s", r.URL.Path)
		}
		resp := GitHubFileContentResponse{
			Name:     "repo-labels.json",
			Path:     ".grubby/repo-labels.json",
			SHA:      "abc123sha",
			Size:     len(testContent),
			Encoding: "base64",
			Content:  encoded,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		apiBaseURL: server.URL,
	}

	content, sha, err := client.GetFileContent(context.Background(), "my-org", "my-squad-tasks", ".grubby/repo-labels.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != "abc123sha" {
		t.Errorf("expected sha 'abc123sha', got '%s'", sha)
	}
	if string(content) != testContent {
		t.Errorf("expected content '%s', got '%s'", testContent, string(content))
	}
}

func TestGetFileContent_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		apiBaseURL: server.URL,
	}

	_, _, err := client.GetFileContent(context.Background(), "my-org", "my-squad-tasks", ".grubby/repo-labels.json")
	if !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("expected ErrFileNotFound, got: %v", err)
	}
}

func TestPutFileContent_ConflictAndSuccess(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Return current file
			resp := GitHubFileContentResponse{
				Name:     "repo-labels.json",
				Path:     ".grubby/repo-labels.json",
				SHA:      "sha-v1",
				Encoding: "base64",
				Content:  base64.StdEncoding.EncodeToString([]byte(`{"mappings":{}}`)),
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		if r.Method == http.MethodPut {
			attempts++
			if attempts == 1 {
				// Simulate conflict on first write
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"message": "conflict"}`))
				return
			}

			// Success on second write
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"content": {"sha": "sha-v2"}}`))
			return
		}
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		apiBaseURL: server.URL,
	}

	err := client.SaveSquadRepoLabels(context.Background(), "my-org", "my-squad-tasks", func(mappings map[string][]string) error {
		mappings["12345"] = []string{"backend", "core"}
		return nil
	})
	if err != nil {
		t.Fatalf("expected SaveSquadRepoLabels to succeed after conflict retry, got: %v", err)
	}

	if attempts != 2 {
		t.Errorf("expected 2 attempts (1 conflict + 1 success), got %d", attempts)
	}
}
