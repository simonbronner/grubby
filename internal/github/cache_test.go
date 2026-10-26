package github

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

type mockRoundTripper func(req *http.Request) (*http.Response, error)

func (m mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m(req)
}

func TestETagCache_InMemory(t *testing.T) {
	cache := NewETagCache("")
	testURL := "https://api.github.com/user"

	if _, ok := cache.Get(testURL); ok {
		t.Fatal("expected empty cache to return false")
	}

	header := http.Header{"Content-Type": []string{"application/json"}}
	cache.Set(testURL, `W/"12345"`, []byte(`{"login":"test"}`), header, http.StatusOK)

	entry, ok := cache.Get(testURL)
	if !ok {
		t.Fatal("expected entry to exist in cache")
	}
	if entry.ETag != `W/"12345"` {
		t.Errorf("expected ETag W/\"12345\", got %s", entry.ETag)
	}
	if string(entry.Body) != `{"login":"test"}` {
		t.Errorf("unexpected body: %s", string(entry.Body))
	}
	if entry.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, entry.StatusCode)
	}
}

func TestETagCache_Persistence(t *testing.T) {
	tempDir := t.TempDir()
	cachePath := filepath.Join(tempDir, "cache.json")

	cache1 := NewETagCache(cachePath)
	testURL := "https://api.github.com/orgs/my-org/repos"
	header := http.Header{"X-Custom": []string{"val1"}}
	cache1.Set(testURL, `"abc"`, []byte(`[{"id":1}]`), header, http.StatusOK)
	cache1.Save()

	cache2 := NewETagCache(cachePath)
	entry, ok := cache2.Get(testURL)
	if !ok {
		t.Fatal("expected persisted entry to be loaded in new cache instance")
	}
	if entry.ETag != `"abc"` {
		t.Errorf("expected ETag \"abc\", got %s", entry.ETag)
	}
	if string(entry.Body) != `[{"id":1}]` {
		t.Errorf("unexpected body: %s", string(entry.Body))
	}
	if len(entry.Header["X-Custom"]) == 0 || entry.Header["X-Custom"][0] != "val1" {
		t.Errorf("persisted headers mismatch: %v", entry.Header)
	}
}

func TestETagCache_Eviction(t *testing.T) {
	cache := NewETagCache("")

	// Insert 205 items
	for i := 0; i < 205; i++ {
		url := fmt.Sprintf("https://api.github.com/item/%d", i)
		cache.Set(url, fmt.Sprintf("etag-%d", i), []byte("body"), nil, 200)
		time.Sleep(1 * time.Millisecond) // Ensure distinct timestamps
	}

	cache.mu.RLock()
	count := len(cache.entries)
	cache.mu.RUnlock()

	if count > 200 {
		t.Errorf("expected cache size <= 200, got %d", count)
	}

	// The very first item (item 0) should have been evicted
	if _, ok := cache.Get("https://api.github.com/item/0"); ok {
		t.Errorf("expected oldest item 0 to be evicted")
	}
}

func TestETagCache_Invalidate(t *testing.T) {
	cache := NewETagCache("")
	cache.Set("https://api.github.com/repos/org/repo/issues", "e1", []byte("issues"), nil, 200)
	cache.Set("https://api.github.com/repos/org/repo/pulls", "e2", []byte("pulls"), nil, 200)
	cache.Set("https://api.github.com/notifications", "e3", []byte("notifs"), nil, 200)

	cache.Invalidate("/issues")

	if _, ok := cache.Get("https://api.github.com/repos/org/repo/issues"); ok {
		t.Errorf("expected /issues to be invalidated")
	}
	if _, ok := cache.Get("https://api.github.com/repos/org/repo/pulls"); !ok {
		t.Errorf("expected /pulls to still exist")
	}
	if _, ok := cache.Get("https://api.github.com/notifications"); !ok {
		t.Errorf("expected /notifications to still exist")
	}
}

func TestETagTransport_Get200AndCache(t *testing.T) {
	cache := NewETagCache("")

	mock := mockRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("If-None-Match") != "" {
			t.Errorf("did not expect If-None-Match on first request")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"ETag":                 []string{`"v1"`},
				"X-Ratelimit-Remaining": []string{"4999"},
			},
			Body:    io.NopCloser(bytes.NewReader([]byte(`{"hello":"world"}`))),
			Request: req,
		}, nil
	})

	client := &http.Client{
		Transport: &ETagTransport{
			Base:  mock,
			Cache: cache,
		},
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"hello":"world"}` {
		t.Errorf("unexpected response body: %s", string(body))
	}

	entry, ok := cache.Get("https://api.github.com/user")
	if !ok {
		t.Fatal("expected entry to be cached")
	}
	if entry.ETag != `"v1"` {
		t.Errorf("expected ETag \"v1\", got %s", entry.ETag)
	}
}

func TestETagTransport_304NotModifiedReturnsCachedBody(t *testing.T) {
	cache := NewETagCache("")
	targetURL := "https://api.github.com/repos/owner/repo/pulls"

	// Pre-populate cache
	cache.Set(targetURL, `"etag-abc"`, []byte(`[{"id":100}]`), http.Header{"Content-Type": []string{"application/json"}}, 200)

	callCount := 0
	mock := mockRoundTripper(func(req *http.Request) (*http.Response, error) {
		callCount++
		if ifNoneMatch := req.Header.Get("If-None-Match"); ifNoneMatch != `"etag-abc"` {
			t.Errorf("expected If-None-Match \"etag-abc\", got %q", ifNoneMatch)
		}
		return &http.Response{
			StatusCode: http.StatusNotModified,
			Header: http.Header{
				"X-Ratelimit-Remaining": []string{"4998"},
				"X-Ratelimit-Used":      []string{"2"},
			},
			Body:    io.NopCloser(bytes.NewReader(nil)),
			Request: req,
		}, nil
	})

	client := &http.Client{
		Transport: &ETagTransport{
			Base:  mock,
			Cache: cache,
		},
	}

	req, _ := http.NewRequest(http.MethodGet, targetURL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected synthesized 200 OK, got %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Ratelimit-Remaining") != "4998" {
		t.Errorf("expected rate limit header 4998, got %s", resp.Header.Get("X-Ratelimit-Remaining"))
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `[{"id":100}]` {
		t.Errorf("expected cached body [{\"id\":100}], got %s", string(body))
	}
	if callCount != 1 {
		t.Errorf("expected 1 roundtrip call, got %d", callCount)
	}
}

func TestETagTransport_MutationInvalidates(t *testing.T) {
	cache := NewETagCache("")
	issuesURL := "https://api.github.com/repos/owner/repo/issues"
	cache.Set(issuesURL, `"etag-issues"`, []byte(`[{"number":1}]`), nil, 200)

	mock := mockRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"number":2}`))),
			Request:    req,
		}, nil
	})

	client := &http.Client{
		Transport: &ETagTransport{
			Base:  mock,
			Cache: cache,
		},
	}

	req, _ := http.NewRequest(http.MethodPost, issuesURL, bytes.NewReader([]byte(`{"title":"New Task"}`)))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do failed: %v", err)
	}
	_ = resp.Body.Close()

	// Issues cache should be invalidated
	if _, ok := cache.Get(issuesURL); ok {
		t.Errorf("expected issues cache entry to be invalidated on POST")
	}
}
