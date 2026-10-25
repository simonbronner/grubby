package github

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseEvents(t *testing.T) {
	samplePushEvent := `{
		"type": "PushEvent",
		"actor": {"login": "simonbronner"},
		"repo": {"name": "simonbronner/seqta-agent-cf"},
		"payload": {
			"ref": "refs/heads/main",
			"commits": [{"message": "feat: improve notification formatting\n\nFull details"}]
		},
		"created_at": "2026-10-17T01:22:21Z"
	}`

	var raw struct {
		Type      string          `json:"type"`
		CreatedAt time.Time       `json:"created_at"`
		Actor     struct {
			Login string `json:"login"`
		} `json:"actor"`
		Repo struct {
			Name string `json:"name"`
		} `json:"repo"`
		Payload json.RawMessage `json:"payload"`
	}

	err := json.Unmarshal([]byte(samplePushEvent), &raw)
	if err != nil {
		t.Fatalf("failed to parse event json: %v", err)
	}

	if raw.Type != "PushEvent" {
		t.Errorf("expected PushEvent, got %s", raw.Type)
	}

	var p struct {
		Ref     string `json:"ref"`
		Commits []struct {
			Message string `json:"message"`
		} `json:"commits"`
	}
	_ = json.Unmarshal(raw.Payload, &p)

	branch := strings.TrimPrefix(p.Ref, "refs/heads/")
	if branch != "main" {
		t.Errorf("expected branch main, got %s", branch)
	}

	if len(p.Commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(p.Commits))
	}

	firstLine := strings.Split(p.Commits[0].Message, "\n")[0]
	if firstLine != "feat: improve notification formatting" {
		t.Errorf("unexpected commit message: %s", firstLine)
	}
}
