package github

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type httpCacheEntry struct {
	ETag       string              `json:"etag"`
	Body       []byte              `json:"body"`
	Header     map[string][]string `json:"header"`
	StatusCode int                 `json:"status_code"`
	SavedAt    time.Time           `json:"saved_at"`
}

type ETagCache struct {
	mu       sync.RWMutex
	filePath string
	entries  map[string]httpCacheEntry
	dirty    bool
}

// NewETagCache initializes an ETag cache with optional disk persistence.
func NewETagCache(filePath string) *ETagCache {
	c := &ETagCache{
		filePath: filePath,
		entries:  make(map[string]httpCacheEntry),
	}
	c.load()
	return c
}

func (c *ETagCache) load() {
	if c.filePath == "" {
		return
	}
	data, err := os.ReadFile(c.filePath)
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = json.Unmarshal(data, &c.entries)
}

func (c *ETagCache) Save() {
	if c.filePath == "" {
		return
	}
	c.mu.RLock()
	if !c.dirty || len(c.entries) == 0 {
		c.mu.RUnlock()
		return
	}
	data, err := json.Marshal(c.entries)
	c.mu.RUnlock()

	if err != nil {
		return
	}

	dir := filepath.Dir(c.filePath)
	_ = os.MkdirAll(dir, 0755)
	_ = os.WriteFile(c.filePath, data, 0644)
	c.mu.Lock()
	c.dirty = false
	c.mu.Unlock()
}

func (c *ETagCache) Get(url string) (httpCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[url]
	return entry, ok
}

func (c *ETagCache) Set(url string, etag string, body []byte, header http.Header, statusCode int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Prune if cache gets too large (>= 200 entries) and adding a new key
	if _, exists := c.entries[url]; !exists {
		for len(c.entries) >= 200 {
			var oldestURL string
			var oldestTime time.Time
			for u, e := range c.entries {
				if oldestTime.IsZero() || e.SavedAt.Before(oldestTime) {
					oldestTime = e.SavedAt
					oldestURL = u
				}
			}
			if oldestURL != "" {
				delete(c.entries, oldestURL)
			} else {
				break
			}
		}
	}

	headerMap := make(map[string][]string)
	for k, v := range header {
		headerMap[k] = v
	}

	c.entries[url] = httpCacheEntry{
		ETag:       etag,
		Body:       body,
		Header:     headerMap,
		StatusCode: statusCode,
		SavedAt:    time.Now(),
	}
	c.dirty = true
}

func (c *ETagCache) Invalidate(urlSubstring string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for u := range c.entries {
		if strings.Contains(u, urlSubstring) {
			delete(c.entries, u)
			c.dirty = true
		}
	}
}

// ETagTransport is an http.RoundTripper that handles ETag / If-None-Match caching.
// When GitHub returns 304 Not Modified, it serves the cached response with zero rate limit cost.
type ETagTransport struct {
	Base       http.RoundTripper
	Cache      *ETagCache
	OnCacheHit func()
}

func (t *ETagTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.Cache == nil {
		return t.Base.RoundTrip(req)
	}

	reqURL := req.URL.String()

	// Mutations invalidate related cache entries
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		if strings.Contains(reqURL, "/notifications") {
			t.Cache.Invalidate("/notifications")
		} else if strings.Contains(reqURL, "/issues") {
			t.Cache.Invalidate("/issues")
		} else if strings.Contains(reqURL, "/pulls") {
			t.Cache.Invalidate("/pulls")
		}
		t.Cache.Save()
		return t.Base.RoundTrip(req)
	}

	// Check if cached entry exists with an ETag
	cached, hasCached := t.Cache.Get(reqURL)
	clonedReq := req.Clone(req.Context())
	if hasCached && cached.ETag != "" {
		clonedReq.Header.Set("If-None-Match", cached.ETag)
	}

	resp, err := t.Base.RoundTrip(clonedReq)
	if err != nil {
		return nil, err
	}

	// 304 Not Modified: Return cached response with updated rate limit headers!
	if resp.StatusCode == http.StatusNotModified && hasCached {
		if t.OnCacheHit != nil {
			t.OnCacheHit()
		}
		resp.Body.Close()

		synthHeader := make(http.Header)
		for k, v := range cached.Header {
			synthHeader[k] = v
		}
		// Overlay rate limit headers from 304 response so rate limits stay current
		for k, v := range resp.Header {
			if strings.HasPrefix(strings.ToLower(k), "x-ratelimit-") || strings.EqualFold(k, "etag") {
				synthHeader[k] = v
			}
		}

		statusCode := cached.StatusCode
		if statusCode == 0 {
			statusCode = http.StatusOK
		}

		return &http.Response{
			StatusCode:    statusCode,
			Status:        "200 OK (from cache)",
			Proto:         resp.Proto,
			ProtoMajor:    resp.ProtoMajor,
			ProtoMinor:    resp.ProtoMinor,
			Header:        synthHeader,
			Body:          io.NopCloser(bytes.NewReader(cached.Body)),
			ContentLength: int64(len(cached.Body)),
			Request:       req,
		}, nil
	}

	// 200 OK: Cache response if ETag header present
	if resp.StatusCode == http.StatusOK {
		etag := resp.Header.Get("ETag")
		if etag == "" {
			for k, v := range resp.Header {
				if strings.EqualFold(k, "etag") && len(v) > 0 {
					etag = v[0]
					break
				}
			}
		}
		if etag != "" {
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr == nil {
				t.Cache.Set(reqURL, etag, bodyBytes, resp.Header, resp.StatusCode)
				t.Cache.Save()
				resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}
	}

	return resp, nil
}
