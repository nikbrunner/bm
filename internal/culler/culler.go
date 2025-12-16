package culler

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/nikbrunner/bm/internal/model"
)

// Status represents the health status of a URL.
type Status int

const (
	Healthy     Status = iota // 2xx or 3xx response
	Dead                      // 404 or 410 Gone
	Unreachable               // timeout, DNS failure, connection refused, etc.
)

// Result holds the check result for a single bookmark.
type Result struct {
	Bookmark   *model.Bookmark
	Status     Status
	StatusCode int    // HTTP status code (0 if connection failed)
	Error      string // Error message for unreachable URLs
}

// ProgressFunc is called after each URL is checked.
// completed is the number of URLs checked so far, total is the total count.
type ProgressFunc func(completed, total int)

// CheckURLs checks all bookmark URLs concurrently and returns results.
// excludeDomains is a list of domains where 404s should be treated as "possibly private" instead of dead.
func CheckURLs(bookmarks []model.Bookmark, concurrency int, timeout time.Duration, excludeDomains []string, onProgress ProgressFunc) []Result {
	if len(bookmarks) == 0 {
		return nil
	}

	// Suppress noisy HTTP client logging (protocol errors, unsolicited responses, etc.)
	originalOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(originalOutput)

	// Build exclude map for fast lookup
	excludeMap := make(map[string]bool)
	for _, domain := range excludeDomains {
		excludeMap[strings.ToLower(domain)] = true
	}

	results := make([]Result, len(bookmarks))
	jobs := make(chan int, len(bookmarks))
	var wg sync.WaitGroup

	// Progress tracking
	var progressMu sync.Mutex
	completed := 0

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Follow redirects but limit to 10
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	// Start workers
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				results[idx] = checkURL(client, &bookmarks[idx], excludeMap)

				if onProgress != nil {
					progressMu.Lock()
					completed++
					onProgress(completed, len(bookmarks))
					progressMu.Unlock()
				}
			}
		}()
	}

	// Send jobs
	for i := range bookmarks {
		jobs <- i
	}
	close(jobs)

	wg.Wait()
	return results
}

// checkURL checks a single URL and returns the result.
func checkURL(client *http.Client, bookmark *model.Bookmark, excludeMap map[string]bool) Result {
	result := Result{
		Bookmark: bookmark,
	}

	// Try HEAD first (faster, less bandwidth)
	resp, err := client.Head(bookmark.URL)
	if err != nil {
		// HEAD failed, try GET as fallback (some servers don't support HEAD)
		resp, err = client.Get(bookmark.URL)
		if err != nil {
			result.Status = Unreachable
			result.Error = normalizeError(err.Error())
			return result
		}
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 400:
		result.Status = Healthy
	case resp.StatusCode == 404 || resp.StatusCode == 410:
		// Check if this domain is excluded (e.g., private repos)
		if isExcludedDomain(bookmark.URL, excludeMap) {
			result.Status = Unreachable
			result.Error = "Possibly private (auth required)"
		} else {
			result.Status = Dead
		}
	default:
		// Other errors (500, 403, etc.) - treat as unreachable
		// Could be temporary server issues or auth-required pages
		result.Status = Unreachable
		result.Error = http.StatusText(resp.StatusCode)
	}

	return result
}

// isExcludedDomain checks if the URL's domain is in the exclude list.
func isExcludedDomain(rawURL string, excludeMap map[string]bool) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Host)
	// Check exact match and parent domain (e.g., "api.github.com" matches "github.com")
	if excludeMap[host] {
		return true
	}
	// Check if host ends with excluded domain
	for domain := range excludeMap {
		if strings.HasSuffix(host, "."+domain) || host == domain {
			return true
		}
	}
	return false
}

// normalizeError simplifies verbose error messages into readable categories.
func normalizeError(errStr string) string {
	lower := strings.ToLower(errStr)

	switch {
	case strings.Contains(lower, "no such host"):
		return "DNS failure"
	case strings.Contains(lower, "context deadline exceeded"),
		strings.Contains(lower, "timeout"):
		return "Timeout"
	case strings.Contains(lower, "connection refused"):
		return "Connection refused"
	case strings.Contains(lower, "certificate"):
		return "TLS/certificate error"
	case strings.Contains(lower, "network is unreachable"):
		return "Network unreachable"
	case strings.Contains(lower, "tls:"):
		return "TLS error"
	default:
		return errStr
	}
}
