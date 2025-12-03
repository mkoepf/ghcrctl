package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// logEntry represents a single API call log entry
type logEntry struct {
	Timestamp     time.Time `json:"timestamp"`
	Category      string    `json:"category"`
	Method        string    `json:"method"`
	URL           string    `json:"url"`
	Path          string    `json:"path"`
	Status        int       `json:"status"`
	DurationMs    int64     `json:"duration_ms"`
	RequestBytes  int64     `json:"request_bytes"`
	ResponseBytes int64     `json:"response_bytes"`
	Caller        string    `json:"caller"`
	Error         string    `json:"error,omitempty"`
}

// loggingRoundTripper is an http.RoundTripper that logs API calls
type loggingRoundTripper struct {
	transport http.RoundTripper
	output    io.Writer
}

// NewLoggingRoundTripper creates a new logging transport
func NewLoggingRoundTripper(transport http.RoundTripper, output io.Writer) *loggingRoundTripper {
	return &loggingRoundTripper{
		transport: transport,
		output:    output,
	}
}

// RoundTrip implements http.RoundTripper
func (t *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	// Capture request size
	reqSize := int64(0)
	if req.Body != nil && req.ContentLength > 0 {
		reqSize = req.ContentLength
	}

	// Get caller information
	caller := getCallerInfo()

	// Perform the actual request
	resp, err := t.transport.RoundTrip(req)

	duration := time.Since(start)

	// Categorize the API call
	category, path := categorizeAPICall(req.URL.String())

	// Build log entry
	entry := logEntry{
		Timestamp:    start,
		Category:     category,
		Method:       req.Method,
		URL:          req.URL.String(),
		Path:         path,
		DurationMs:   duration.Microseconds() / 1000, // Convert to ms but preserve fractional part
		RequestBytes: reqSize,
		Caller:       caller,
	}

	// Ensure we always have at least 1ms if request completed
	if entry.DurationMs == 0 && err == nil {
		entry.DurationMs = 1
	}

	if err != nil {
		entry.Error = err.Error()
	} else {
		entry.Status = resp.StatusCode
		if resp.ContentLength > 0 {
			entry.ResponseBytes = resp.ContentLength
		}
	}

	// Write log entry as JSON
	data, _ := json.Marshal(entry)
	fmt.Fprintln(t.output, string(data))

	return resp, err
}

// categorizeAPICall determines the API category and extracts the path
func categorizeAPICall(url string) (category, path string) {
	// Extract path from URL
	if idx := strings.Index(url, "://"); idx >= 0 {
		url = url[idx+3:] // Skip protocol
	}
	if idx := strings.Index(url, "/"); idx >= 0 {
		path = url[idx:]
	} else {
		path = "/"
	}

	// Categorize based on host/path patterns
	if strings.Contains(url, "api.github.com") {
		return "github", path
	}
	if strings.Contains(url, "ghcr.io") || strings.Contains(path, "/v2/") {
		return "oci", path
	}
	return "other", path
}

// getCallerInfo returns the caller information (file:line:function)
func getCallerInfo() string {
	// Skip frames: getCallerInfo, RoundTrip, and http internals
	// Look for first frame outside of http package and logging package
	for i := 2; i < 15; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		// Get function name
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}

		funcName := fn.Name()

		// Skip internal http and logging packages
		if strings.Contains(funcName, "net/http") ||
			strings.Contains(funcName, "internal/logging") ||
			strings.Contains(file, "net/http") {
			continue
		}

		// Extract just the filename from the full path
		if idx := strings.LastIndex(file, "/"); idx >= 0 {
			file = file[idx+1:]
		}

		// Extract just the function name without package path
		if idx := strings.LastIndex(funcName, "/"); idx >= 0 {
			funcName = funcName[idx+1:]
		}
		if idx := strings.LastIndex(funcName, "."); idx >= 0 {
			funcName = funcName[idx+1:]
		}

		return fmt.Sprintf("%s:%d:%s", file, line, funcName)
	}

	return "unknown"
}
