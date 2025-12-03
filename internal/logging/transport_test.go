package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggingRoundTripper(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		responseStatus int
		responseBody   string
		wantCategory   string
	}{
		{
			name:           "GitHub API call",
			method:         "GET",
			path:           "/user/packages",
			responseStatus: 200,
			responseBody:   `{"packages":[]}`,
			wantCategory:   "github",
		},
		{
			name:           "OCI Registry manifest",
			method:         "GET",
			path:           "/v2/owner/image/manifests/latest",
			responseStatus: 200,
			responseBody:   `{"schemaVersion":2}`,
			wantCategory:   "oci",
		},
		{
			name:           "OCI Registry blob",
			method:         "GET",
			path:           "/v2/owner/image/blobs/sha256:abc123",
			responseStatus: 200,
			responseBody:   "binary data",
			wantCategory:   "oci",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			// Capture log output
			var logBuf bytes.Buffer

			// Create logging transport
			transport := NewLoggingRoundTripper(http.DefaultTransport, &logBuf)

			// Create client with logging transport
			client := &http.Client{Transport: transport}

			// Make request
			req, err := http.NewRequest(tt.method, server.URL+tt.path, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			// Read response body
			io.Copy(io.Discard, resp.Body)

			// Parse log output
			var logEntry logEntry
			if err := json.Unmarshal(logBuf.Bytes(), &logEntry); err != nil {
				t.Fatalf("Failed to parse log output: %v\nOutput: %s", err, logBuf.String())
			}

			// Verify log entry (category will be "other" for test server)
			if logEntry.Method != tt.method {
				t.Errorf("Expected method %q, got %q", tt.method, logEntry.Method)
			}

			if logEntry.Status != tt.responseStatus {
				t.Errorf("Expected status %d, got %d", tt.responseStatus, logEntry.Status)
			}

			if logEntry.DurationMs <= 0 {
				t.Error("Expected positive duration")
			}

			if logEntry.Timestamp.IsZero() {
				t.Error("Expected non-zero timestamp")
			}

			if !strings.Contains(logEntry.URL, tt.path) {
				t.Errorf("Expected URL to contain %q, got %q", tt.path, logEntry.URL)
			}
		})
	}
}

func TestLoggingRoundTripperError(t *testing.T) {
	// Create a transport that always fails
	failingTransport := &errorTransport{err: io.EOF}

	var logBuf bytes.Buffer
	transport := NewLoggingRoundTripper(failingTransport, &logBuf)

	client := &http.Client{Transport: transport}
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)

	_, err := client.Do(req)
	if err == nil {
		t.Fatal("Expected error but got none")
	}

	// Parse log output
	var logEntry logEntry
	if err := json.Unmarshal(logBuf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Verify error is logged
	if logEntry.Error == "" {
		t.Error("Expected error to be logged")
	}

	if !strings.Contains(logEntry.Error, "EOF") {
		t.Errorf("Expected error to contain 'EOF', got %q", logEntry.Error)
	}
}

func TestCategorizeAPICall(t *testing.T) {
	tests := []struct {
		url      string
		want     string
		wantPath string
	}{
		{
			url:      "https://api.github.com/user/packages",
			want:     "github",
			wantPath: "/user/packages",
		},
		{
			url:      "https://api.github.com/orgs/myorg/packages",
			want:     "github",
			wantPath: "/orgs/myorg/packages",
		},
		{
			url:      "https://ghcr.io/v2/owner/image/manifests/latest",
			want:     "oci",
			wantPath: "/v2/owner/image/manifests/latest",
		},
		{
			url:      "https://ghcr.io/v2/owner/image/blobs/sha256:abc123",
			want:     "oci",
			wantPath: "/v2/owner/image/blobs/sha256:abc123",
		},
		{
			url:      "https://other.example.com/api",
			want:     "other",
			wantPath: "/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			category, path := categorizeAPICall(tt.url)
			if category != tt.want {
				t.Errorf("Expected category %q, got %q", tt.want, category)
			}
			if path != tt.wantPath {
				t.Errorf("Expected path %q, got %q", tt.wantPath, path)
			}
		})
	}
}

func TestCallerInfo(t *testing.T) {
	caller := getCallerInfo()

	// Should contain file:line:function format (may be testing.go in test context)
	// Just verify it's not "unknown" and has the right format
	if caller == "unknown" {
		t.Error("Expected caller info but got 'unknown'")
	}

	// Should have format: file:line:function
	parts := strings.Split(caller, ":")
	if len(parts) < 3 {
		t.Errorf("Expected caller format 'file:line:function', got %q", caller)
	}
}

// errorTransport is a test helper that always returns an error
type errorTransport struct {
	err error
}

func (t *errorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, t.err
}

func TestLoggingDisabledByDefault(t *testing.T) {
	ctx := context.Background()

	if IsLoggingEnabled(ctx) {
		t.Error("Expected logging to be disabled by default")
	}
}

func TestEnableLogging(t *testing.T) {
	ctx := context.Background()
	ctx = EnableLogging(ctx)

	if !IsLoggingEnabled(ctx) {
		t.Error("Expected logging to be enabled")
	}
}
