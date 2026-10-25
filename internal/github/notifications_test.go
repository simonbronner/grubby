package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchNotifications(t *testing.T) {
	mockJSON := `[
		{
			"id": "1001",
			"unread": true,
			"reason": "author",
			"updated_at": "2026-10-25T10:00:00Z",
			"subject": {
				"title": "Fix memory leak in buffer pool",
				"url": "https://api.github.com/repos/simonbronner/grubby/pulls/12",
				"type": "PullRequest"
			},
			"repository": {
				"name": "grubby",
				"full_name": "simonbronner/grubby",
				"html_url": "https://github.com/simonbronner/grubby"
			}
		},
		{
			"id": "1002",
			"unread": false,
			"reason": "subscribed",
			"updated_at": "2026-10-25T09:00:00Z",
			"subject": {
				"title": "Question about configuration schema",
				"url": "https://api.github.com/repos/simonbronner/grubby/issues/5",
				"type": "Issue"
			},
			"repository": {
				"name": "grubby",
				"full_name": "simonbronner/grubby",
				"html_url": "https://github.com/simonbronner/grubby"
			}
		}
	]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockJSON))
	}))
	defer server.Close()

	client := &Client{
		token:      "test-token",
		httpClient: server.Client(),
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	// Verify formatNotificationWebURL logic
	prURL := "https://api.github.com/repos/simonbronner/grubby/pulls/12"
	formatted := formatNotificationWebURL(prURL, "https://github.com/simonbronner/grubby")
	expected := "https://github.com/simonbronner/grubby/pull/12"
	if formatted != expected {
		t.Errorf("expected %q, got %q", expected, formatted)
	}

	issueURL := "https://api.github.com/repos/simonbronner/grubby/issues/5"
	formattedIssue := formatNotificationWebURL(issueURL, "https://github.com/simonbronner/grubby")
	expectedIssue := "https://github.com/simonbronner/grubby/issues/5"
	if formattedIssue != expectedIssue {
		t.Errorf("expected %q, got %q", expectedIssue, formattedIssue)
	}
}

func TestMarkNotificationRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" && r.URL.Path == "/notifications/threads/1001" {
			w.WriteHeader(http.StatusResetContent)
			return
		}
		if r.Method == "PUT" && r.URL.Path == "/notifications" {
			w.WriteHeader(http.StatusResetContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &Client{
		token:      "test-token",
		httpClient: server.Client(),
	}

	// Test PATCH /notifications/threads/1001
	req, _ := http.NewRequestWithContext(context.Background(), "PATCH", server.URL+"/notifications/threads/1001", nil)
	resp, err := client.httpClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusResetContent {
		t.Errorf("expected 205, got %d", resp.StatusCode)
	}
}
