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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			require.NoError(t, err, "Failed to create request")

			resp, err := client.Do(req)
			require.NoError(t, err, "Request failed")
			defer resp.Body.Close()

			// Read response body
			io.Copy(io.Discard, resp.Body)

			// Parse log output
			var logEntry logEntry
			err = json.Unmarshal(logBuf.Bytes(), &logEntry)
			require.NoError(t, err, "Failed to parse log output: %s", logBuf.String())

			// Verify log entry (category will be "other" for test server)
			assert.Equal(t, tt.method, logEntry.Method)
			assert.Equal(t, tt.responseStatus, logEntry.Status)
			assert.Greater(t, logEntry.DurationMs, int64(0), "Expected positive duration")
			assert.False(t, logEntry.Timestamp.IsZero(), "Expected non-zero timestamp")
			assert.Contains(t, logEntry.URL, tt.path)
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
	require.Error(t, err, "Expected error but got none")

	// Parse log output
	var logEntry logEntry
	err = json.Unmarshal(logBuf.Bytes(), &logEntry)
	require.NoError(t, err, "Failed to parse log output")

	// Verify error is logged
	assert.NotEmpty(t, logEntry.Error, "Expected error to be logged")
	assert.Contains(t, logEntry.Error, "EOF")
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
			assert.Equal(t, tt.want, category)
			assert.Equal(t, tt.wantPath, path)
		})
	}
}

func TestCallerInfo(t *testing.T) {
	caller := getCallerInfo()

	// Should contain file:line:function format (may be testing.go in test context)
	// Just verify it's not "unknown" and has the right format
	assert.NotEqual(t, "unknown", caller, "Expected caller info but got 'unknown'")

	// Should have format: file:line:function
	parts := strings.Split(caller, ":")
	assert.GreaterOrEqual(t, len(parts), 3, "Expected caller format 'file:line:function', got %q", caller)
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
	assert.False(t, IsLoggingEnabled(ctx), "Expected logging to be disabled by default")
}

func TestEnableLogging(t *testing.T) {
	ctx := context.Background()
	ctx = EnableLogging(ctx)
	assert.True(t, IsLoggingEnabled(ctx), "Expected logging to be enabled")
}
