package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchStatus_Success(t *testing.T) {
	mockJSON := `{
		"page": {
			"id": "kctbh9vrtdwd",
			"name": "GitHub",
			"url": "https://www.githubstatus.com",
			"updated_at": "2026-10-25T10:00:00Z"
		},
		"status": {
			"indicator": "minor",
			"description": "Minor Service Outage"
		},
		"components": [
			{
				"id": "8l4ygp009s5s",
				"name": "Git Operations",
				"status": "operational",
				"description": "Performance of git clones, pulls, pushes"
			},
			{
				"id": "br0l2tvcx85d",
				"name": "Actions",
				"status": "degraded_performance",
				"description": "Workflows and Compute for GitHub Actions"
			}
		],
		"incidents": [
			{
				"id": "inc123",
				"name": "Actions Run Delays",
				"status": "investigating",
				"impact": "minor",
				"incident_updates": [
					{
						"id": "upd1",
						"status": "investigating",
						"body": "We are investigating delayed workflow runs."
					}
				]
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockJSON))
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
	}

	oldURL := GitHubStatusURL
	GitHubStatusURL = server.URL
	defer func() { GitHubStatusURL = oldURL }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	summary, err := client.FetchStatus(ctx)
	if err != nil {
		t.Fatalf("FetchStatus failed: %v", err)
	}
	if summary.Status.Description != "Minor Service Outage" {
		t.Errorf("expected 'Minor Service Outage', got %q", summary.Status.Description)
	}
	if len(summary.Components) != 2 {
		t.Errorf("expected 2 components, got %d", len(summary.Components))
	}
	if summary.Components[1].Status != "degraded_performance" {
		t.Errorf("expected Actions to be degraded_performance, got %s", summary.Components[1].Status)
	}
	if len(summary.Incidents) != 1 || summary.Incidents[0].Name != "Actions Run Delays" {
		t.Errorf("expected incident 'Actions Run Delays', got %+v", summary.Incidents)
	}
}
