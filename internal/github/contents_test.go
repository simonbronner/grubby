package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestGenerateLabelColor_DistinctColors(t *testing.T) {
	colors := make(map[string]bool)
	for i := 0; i < 10; i++ {
		c := GenerateLabelColor(i)
		if len(c) != 6 {
			t.Errorf("expected 6-char hex color, got %q", c)
		}
		if colors[c] {
			t.Errorf("expected unique color at index %d, duplicate: %s", i, c)
		}
		colors[c] = true
	}
}

func TestSaveSquadRepoLabels_ColorGenerationAndGarbageCollection(t *testing.T) {
	var savedFile SquadRepoLabelsFile

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Initially repo 101 has "backend" and "legacy"
			initial := SquadRepoLabelsFile{
				Version: 1,
				Labels: map[string]LabelInfo{
					"backend": {Color: "0075ca"},
					"legacy":  {Color: "b60205"},
				},
				Mappings: map[string][]string{
					"101": {"backend", "legacy"},
				},
			}
			b, _ := json.Marshal(initial)
			encoded := base64.StdEncoding.EncodeToString(b)
			resp := GitHubFileContentResponse{
				SHA:     "sha-init",
				Content: encoded,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		if r.Method == http.MethodPut {
			var payload GitHubFileCommitPayload
			_ = json.NewDecoder(r.Body).Decode(&payload)
			decoded, _ := base64.StdEncoding.DecodeString(payload.Content)
			_ = json.Unmarshal(decoded, &savedFile)

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"content": {"sha": "sha-new"}}`))
			return
		}
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		apiBaseURL: server.URL,
	}

	// Update mappings: repo 101 removes "legacy" and adds "frontend"
	err := client.SaveSquadRepoLabels(context.Background(), "org", "tasks", func(mappings map[string][]string) error {
		mappings["101"] = []string{"backend", "frontend"}
		return nil
	})
	if err != nil {
		t.Fatalf("SaveSquadRepoLabels failed: %v", err)
	}

	// 1. Verify "legacy" was garbage collected because 0 repos use it
	if _, exists := savedFile.Labels["legacy"]; exists {
		t.Errorf("expected 'legacy' label to be garbage collected, but it still exists: %+v", savedFile.Labels)
	}

	// 2. Verify "backend" preserved its existing color ("0075ca")
	if info, exists := savedFile.Labels["backend"]; !exists || info.Color != "0075ca" {
		t.Errorf("expected 'backend' to preserve color '0075ca', got: %+v", info)
	}

	// 3. Verify "frontend" was assigned a newly generated non-empty color
	if info, exists := savedFile.Labels["frontend"]; !exists || info.Color == "" || len(info.Color) != 6 {
		t.Errorf("expected 'frontend' to have a generated 6-digit color, got: %+v", info)
	}
}

func TestFetchSquadsFile(t *testing.T) {
	initialData := SquadsFile{
		Version: 1,
		Squads: []SquadDefinition{
			{Name: "core", DisplayName: "Core Team", Members: []string{"simonb", "alice"}},
			{Name: "frontend", DisplayName: "UI Team", Members: []string{"bob"}},
		},
	}
	bytesData, _ := json.Marshal(initialData)
	encoded := base64.StdEncoding.EncodeToString(bytesData)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/my-org/grubby-tasks/contents/squads.json" {
			resp := GitHubFileContentResponse{
				Name:    "squads.json",
				SHA:     "sha-squads-1",
				Content: encoded,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		apiBaseURL: server.URL,
	}

	file, sha, err := client.FetchSquadsFile(context.Background(), "my-org", "grubby-tasks")
	if err != nil {
		t.Fatalf("expected FetchSquadsFile to succeed, got: %v", err)
	}
	if sha != "sha-squads-1" {
		t.Errorf("expected sha-squads-1, got %s", sha)
	}
	if len(file.Squads) != 2 || file.Squads[0].Name != "core" {
		t.Errorf("unexpected squads: %+v", file.Squads)
	}

	// Test Not Found
	_, _, err = client.FetchSquadsFile(context.Background(), "my-org", "non-existent")
	if !errors.Is(err, ErrFileNotFound) {
		t.Errorf("expected ErrFileNotFound, got: %v", err)
	}
}

func TestSaveSquadsFile_CASAndNormalization(t *testing.T) {
	var savedFile SquadsFile
	attempt := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			initial := SquadsFile{
				Version: 1,
				Squads: []SquadDefinition{
					{Name: "core", DisplayName: "Core Team", Members: []string{"simonb"}},
				},
			}
			b, _ := json.Marshal(initial)
			encoded := base64.StdEncoding.EncodeToString(b)
			resp := GitHubFileContentResponse{
				SHA:     "sha-v1",
				Content: encoded,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		if r.Method == http.MethodPut {
			attempt++
			if attempt == 1 {
				// Simulate HTTP 409 Conflict on first attempt
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"message": "Conflict"}`))
				return
			}
			// Second attempt succeeds
			var payload GitHubFileCommitPayload
			_ = json.NewDecoder(r.Body).Decode(&payload)
			decoded, _ := base64.StdEncoding.DecodeString(payload.Content)
			_ = json.Unmarshal(decoded, &savedFile)

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

	err := client.SaveSquadsFile(context.Background(), "my-org", "grubby-tasks", "simonb", func(squads *[]SquadDefinition) error {
		*squads = append(*squads, SquadDefinition{
			Name:    "infra",
			Members: []string{"@Alice", "alice", " bob "},
		})
		return nil
	})
	if err != nil {
		t.Fatalf("expected SaveSquadsFile to succeed after retry, got: %v", err)
	}

	if len(savedFile.Squads) != 2 {
		t.Fatalf("expected 2 squads, got %d", len(savedFile.Squads))
	}
	// Check member normalization on "infra" squad (alice deduplicated, @ stripped, bob trimmed)
	infra := savedFile.Squads[1]
	if len(infra.Members) != 2 || infra.Members[0] != "alice" || infra.Members[1] != "bob" {
		t.Errorf("expected normalized members [alice, bob], got: %+v", infra.Members)
	}
	if savedFile.UpdatedBy != "simonb" {
		t.Errorf("expected UpdatedBy simonb, got: %s", savedFile.UpdatedBy)
	}
}

func TestFetchSquadsFile_FallbackToTasksRepo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If requesting grubby-tasks, return 404
		if strings.Contains(r.URL.Path, "grubby-tasks") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "Not Found"}`))
			return
		}

		// If requesting tasks/squads.json, return squads file
		if strings.Contains(r.URL.Path, "tasks") && strings.Contains(r.URL.Path, "squads.json") {
			fileData := SquadsFile{
				Version: 1,
				Squads: []SquadDefinition{
					{Name: "platform", DisplayName: "Platform Squad", Members: []string{"dev1", "dev2"}},
				},
			}
			b, _ := json.Marshal(fileData)
			encoded := base64.StdEncoding.EncodeToString(b)
			resp := GitHubFileContentResponse{
				SHA:     "tasks-sha",
				Content: encoded,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		apiBaseURL: server.URL,
	}

	// Should fallback from grubby-tasks to tasks repo and find platform squad
	file, sha, err := client.FetchSquadsFile(context.Background(), "my-corp", "grubby-tasks")
	if err != nil {
		t.Fatalf("expected FetchSquadsFile to fallback and succeed, got: %v", err)
	}
	if sha != "tasks-sha" {
		t.Errorf("expected sha tasks-sha, got %s", sha)
	}
	if len(file.Squads) != 1 || file.Squads[0].Name != "platform" {
		t.Errorf("unexpected squads: %+v", file.Squads)
	}
}

func TestFetchOrgTeams(t *testing.T) {
	respJSON := `{
		"data": {
			"organization": {
				"teams": {
					"nodes": [
						{
							"slug": "backend-eng",
							"name": "Backend Engineering",
							"description": "Backend services team",
							"members": {
								"nodes": [
									{"login": "alice"},
									{"login": "bob"}
								]
							}
						},
						{
							"slug": "frontend-eng",
							"name": "Frontend Engineering",
							"description": "Frontend UI team",
							"members": {
								"nodes": [
									{"login": "charlie"}
								]
							}
						}
					]
				}
			}
		}
	}`

	mock := mockRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(respJSON))),
			Request:    req,
		}, nil
	})

	client := &Client{
		token: "mock-token",
		httpClient: &http.Client{
			Transport: mock,
		},
	}

	teams, err := client.FetchOrgTeams(context.Background(), "my-corp")
	if err != nil {
		t.Fatalf("expected FetchOrgTeams to succeed, got: %v", err)
	}
	if len(teams) != 2 {
		t.Fatalf("expected 2 teams, got: %d", len(teams))
	}
	if teams[0].Name != "backend-eng" || teams[0].DisplayName != "Backend Engineering" || len(teams[0].Members) != 2 {
		t.Errorf("unexpected team 0: %+v", teams[0])
	}
	if teams[1].Name != "frontend-eng" || teams[1].DisplayName != "Frontend Engineering" || len(teams[1].Members) != 1 {
		t.Errorf("unexpected team 1: %+v", teams[1])
	}
}



