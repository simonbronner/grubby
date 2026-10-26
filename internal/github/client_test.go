package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestProxyResolution(t *testing.T) {
	// Test normalizeProxyURL
	u, err := normalizeProxyURL("http://proxy.corp.com:8080")
	if err != nil || u.Host != "proxy.corp.com:8080" || u.Scheme != "http" {
		t.Errorf("normalizeProxyURL failed for http: %v, %v", u, err)
	}

	u, err = normalizeProxyURL("socks5://127.0.0.1:1080")
	if err != nil || u.Host != "127.0.0.1:1080" || u.Scheme != "socks5" {
		t.Errorf("normalizeProxyURL failed for socks5: %v, %v", u, err)
	}

	u, err = normalizeProxyURL("proxy.internal:3128")
	if err != nil || u.Host != "proxy.internal:3128" || u.Scheme != "http" {
		t.Errorf("normalizeProxyURL failed for missing scheme: %v, %v", u, err)
	}

	u, err = normalizeProxyURL("")
	if err != nil || u != nil {
		t.Errorf("normalizeProxyURL failed for empty string: %v, %v", u, err)
	}

	// Test ResolveProxy priority
	explicit := "http://explicit-proxy:8080"
	if p := ResolveProxy(explicit); p != explicit {
		t.Errorf("expected explicit proxy %q, got %q", explicit, p)
	}

	// Test env var fallback
	t.Setenv("HTTPS_PROXY", "http://env-proxy:8443")
	if p := ResolveProxy(""); p != "http://env-proxy:8443" {
		t.Errorf("expected env proxy %q, got %q", "http://env-proxy:8443", p)
	}
}

func TestLiveFetchRepositoriesAndPRs(t *testing.T) {
	token, err := GetGHToken()
	if err != nil {
		t.Skip("gh auth not available")
	}

	client, err := NewClient(token)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	repos, prs, commitEvents, secMap, err := client.FetchRepositoriesAndPRs(ctx, "simonbronner")
	if err != nil {
		t.Fatalf("FetchRepositoriesAndPRs failed: %v", err)
	}

	if len(repos) == 0 {
		t.Fatalf("expected repos, got 0")
	}

	// Verify open PRs are fetched properly
	if len(prs) == 0 {
		t.Logf("no open PRs returned, verifying PR structure")
	} else {
		for _, pr := range prs {
			if pr.Repo == "" || pr.Number <= 0 || pr.Title == "" {
				t.Errorf("invalid PR struct: %+v", pr)
			}
		}
	}

	// Verify commit events include grubby
	foundGrubbyCommit := false
	for _, ce := range commitEvents {
		if ce.Repo == "grubby" {
			foundGrubbyCommit = true
			break
		}
	}
	if !foundGrubbyCommit {
		t.Errorf("expected commit events for grubby")
	}

	// Verify squad activity includes recent activity
	squadEvents, err := client.FetchSquadActivity(ctx, []string{"simonbronner"}, commitEvents)
	if err != nil {
		t.Fatalf("FetchSquadActivity failed: %v", err)
	}
	if len(squadEvents) == 0 {
		t.Fatalf("expected squad events, got 0")
	}

	// Verify the most recent squad event is from today (within last 24h)
	mostRecent := squadEvents[0]
	if time.Since(mostRecent.Timestamp) > 24*time.Hour {
		t.Errorf("expected most recent event from today, got timestamp %v (%s)", mostRecent.Timestamp, mostRecent.Details)
	}

	// Verify security map includes best-resume-ever
	sec, ok := secMap["best-resume-ever"]
	if !ok || sec.Total == 0 {
		t.Errorf("expected security alerts for best-resume-ever, got: %+v", sec)
	}
}

func TestLiveFetchOrgMembers(t *testing.T) {
	token, err := GetGHToken()
	if err != nil {
		t.Skip("gh auth not available")
	}

	client, err := NewClient(token)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	members, err := client.FetchOrgMembers(ctx, "github")
	if err != nil {
		t.Fatalf("FetchOrgMembers failed: %v", err)
	}
	if len(members) == 0 {
		t.Fatalf("expected members for github org, got 0")
	}
	for _, m := range members {
		if m.Login == "" {
			t.Errorf("member has empty login: %+v", m)
		}
	}
}

func TestLiveFetchUserProfile(t *testing.T) {
	token, err := GetGHToken()
	if err != nil {
		t.Skip("gh auth not available")
	}

	client, err := NewClient(token)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	prof, err := client.FetchUserProfile(ctx, "simonbronner")
	if err != nil {
		t.Fatalf("FetchUserProfile failed: %v", err)
	}
	if prof.Login != "simonbronner" {
		t.Errorf("expected login simonbronner, got %s", prof.Login)
	}
	if prof.FormattedID() == "" {
		t.Errorf("expected non-empty user ID")
	}
	if prof.AvatarURL == "" {
		t.Errorf("expected non-empty avatar URL")
	}
}

func TestLiveFetchUserProfiles(t *testing.T) {
	token, err := GetGHToken()
	if err != nil {
		t.Skip("gh auth not available")
	}

	client, err := NewClient(token)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	profs, err := client.FetchUserProfiles(ctx, []string{"simonbronner", "octocat"})
	if err != nil {
		t.Fatalf("FetchUserProfiles failed: %v", err)
	}
	if len(profs) < 2 {
		t.Fatalf("expected at least 2 profiles, got %d", len(profs))
	}
	if sb, ok := profs["simonbronner"]; !ok || sb.Login != "simonbronner" {
		t.Errorf("expected simonbronner in profiles, got %+v", profs)
	}
}

func TestNormalizeRepoName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"grubby", "grubby"},
		{"simonbronner/grubby", "grubby"},
		{"org/repo.git", "repo"},
		{"https://github.com/org/repo", "repo"},
		{"https://github.com/org/repo.git", "repo"},
		{"git@github.com:org/repo.git", "repo"},
		{"org/nested/repo", "repo"},
		{"", "unknown"},
		{"  org/repo  ", "repo"},
	}

	for _, tc := range tests {
		actual := NormalizeRepoName(tc.input)
		if actual != tc.expected {
			t.Errorf("NormalizeRepoName(%q) = %q; expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestRequestTracking(t *testing.T) {
	client, err := NewClient("dummy-token")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	client.SetInitialRequestStats(10, 25)
	if client.RequestsMade() != 10 {
		t.Errorf("expected 10 requests made, got %d", client.RequestsMade())
	}
	if client.RequestsSaved() != 25 {
		t.Errorf("expected 25 requests saved, got %d", client.RequestsSaved())
	}

	client.RecordRequestMade()
	if client.RequestsMade() != 11 {
		t.Errorf("expected 11 requests made, got %d", client.RequestsMade())
	}

	client.RecordRequestSaved(5)
	if client.RequestsSaved() != 30 {
		t.Errorf("expected 30 requests saved, got %d", client.RequestsSaved())
	}
}

func TestMergePullRequest(t *testing.T) {
	var requestedPath string
	var requestedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedMethod = r.Method

		if strings.Contains(r.URL.Path, "fail") {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte(`{"message": "Pull Request is not mergeable"}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"sha": "abcdef123456", "merged": true, "message": "Pull Request successfully merged"}`))
	}))
	defer server.Close()

	client := &Client{
		token:      "dummy-token",
		httpClient: server.Client(),
		apiBaseURL: server.URL,
	}

	ctx := context.Background()

	// 1. Successful merge
	err := client.MergePullRequest(ctx, "my-org", "my-repo", 123)
	if err != nil {
		t.Fatalf("expected merge to succeed, got: %v", err)
	}
	if requestedMethod != "PUT" {
		t.Errorf("expected PUT method, got %s", requestedMethod)
	}
	if requestedPath != "/repos/my-org/my-repo/pulls/123/merge" {
		t.Errorf("expected path /repos/my-org/my-repo/pulls/123/merge, got %s", requestedPath)
	}

	// 2. Failed merge
	err = client.MergePullRequest(ctx, "my-org", "fail-repo", 456)
	if err == nil {
		t.Fatalf("expected merge error, got nil")
	}
	if !strings.Contains(err.Error(), "Pull Request is not mergeable") {
		t.Errorf("expected error message to contain 'Pull Request is not mergeable', got %v", err)
	}
}
