package http2

import (
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"time"
)

const (
	maxVisitedCacheEntries = 100
)

type cacheEntry struct {
	start, end time.Time
	code       int
	etag       string
	headers    http.Header
	body       []byte
}

func (e *cacheEntry) cache(f func(http.ResponseWriter)) {
	recorder := httptest.NewRecorder()
	f(recorder)

	e.code = recorder.Code
	e.headers = recorder.HeaderMap
	e.body = recorder.Body.Bytes()
	hash := sha1.New()
	hash.Write(e.body)
	e.etag = fmt.Sprintf("%x", hash.Sum(nil))

	e.headers.Set("Expires", e.end.Format(http.TimeFormat))
	e.headers.Set("Content-Length", strconv.FormatInt(int64(len(e.body)), 10))
}

func (e *cacheEntry) serve(w http.ResponseWriter, r *http.Request) {
	for k, v := range e.headers {
		w.Header()[k] = v
	}

	if CheckETag(e.etag, false, w, r) || CheckLastModified(e.start, w, r) {
		return
	}
	w.WriteHeader(e.code)

	w.Write(e.body)
}

func (e *cacheEntry) valid() bool {
	return time.Since(e.end) < 0
}

type ResponseCache struct {
	cached map[string]*cacheEntry
	lock   sync.RWMutex
}

// Cache a repsonse based on its request path. An ETag will be computed based
// on the sha1 hashsum of the response body and a Last-Modified and Expires
// header will be set as the time of generation and the time of expiration.
//
// Usage:
//        cache.Response(r.URL.String(), 2*time.Hour, w, r, func(w http.ResponseWriter) {
//            // Write response unconditionally (no Check* functions should be
//            // called - they will be handled automatically by the cache).
//        })
func (c *ResponseCache) Response(path string, d time.Duration, w http.ResponseWriter, r *http.Request, f func(http.ResponseWriter)) {
	c.lock.RLock()
	cached := c.cached[path]
	c.lock.RUnlock()

	if cached != nil && cached.valid() {
		cached.serve(w, r)
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	cached = c.cached[path]

	if cached != nil && cached.valid() {
		cached.serve(w, r)
		return
	}

	visited := 0
	for p, e := range c.cached {
		if visited > maxVisitedCacheEntries {
			break
		}
		if !e.valid() {
			delete(c.cached, p)
		}
		visited++
	}

	if c.cached == nil {
		c.cached = make(map[string]*cacheEntry)
	}

	c.cached[path] = &cacheEntry{
		start: time.Now(),
		end:   time.Now().Add(d),
	}
	c.cached[path].cache(f)
	c.cached[path].serve(w, r)
}
