package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/gh"
)

// mockVersionLister implements VersionLister for testing
type mockVersionLister struct {
	versions []gh.PackageVersionInfo
	err      error
}

func (m *mockVersionLister) ListPackageVersions(ctx context.Context, owner, ownerType, packageName string) ([]gh.PackageVersionInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.versions, nil
}

func TestStatsCommandExists(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()

	statsCmd, _, err := cmd.Find([]string{"stats"})
	if err != nil {
		t.Fatalf("Failed to find stats command: %v", err)
	}

	if statsCmd.Use != "stats <owner/package>" {
		t.Errorf("Expected Use to be 'stats <owner/package>', got %q", statsCmd.Use)
	}
}

func TestStatsCommandRequiresArg(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"stats"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error when no argument provided")
	}
}

func TestCalculateStats(t *testing.T) {
	t.Parallel()

	versions := []gh.PackageVersionInfo{
		{ID: 1, Name: "sha256:aaa", Tags: []string{"v1.0.0", "latest"}, CreatedAt: "2025-01-15T10:00:00Z"},
		{ID: 2, Name: "sha256:bbb", Tags: []string{"v0.9.0"}, CreatedAt: "2025-01-10T10:00:00Z"},
		{ID: 3, Name: "sha256:ccc", Tags: []string{}, CreatedAt: "2025-01-05T10:00:00Z"},
		{ID: 4, Name: "sha256:ddd", Tags: []string{}, CreatedAt: "2025-01-01T10:00:00Z"},
	}

	stats := CalculateStats(versions)

	if stats.TotalVersions != 4 {
		t.Errorf("Expected TotalVersions=4, got %d", stats.TotalVersions)
	}
	if stats.TaggedVersions != 2 {
		t.Errorf("Expected TaggedVersions=2, got %d", stats.TaggedVersions)
	}
	if stats.UntaggedVersions != 2 {
		t.Errorf("Expected UntaggedVersions=2, got %d", stats.UntaggedVersions)
	}
	if stats.TotalTags != 3 {
		t.Errorf("Expected TotalTags=3, got %d", stats.TotalTags)
	}
	if stats.OldestVersion != "2025-01-01T10:00:00Z" {
		t.Errorf("Expected OldestVersion='2025-01-01T10:00:00Z', got %q", stats.OldestVersion)
	}
	if stats.NewestVersion != "2025-01-15T10:00:00Z" {
		t.Errorf("Expected NewestVersion='2025-01-15T10:00:00Z', got %q", stats.NewestVersion)
	}
}

func TestCalculateStatsEmpty(t *testing.T) {
	t.Parallel()

	stats := CalculateStats([]gh.PackageVersionInfo{})

	if stats.TotalVersions != 0 {
		t.Errorf("Expected TotalVersions=0, got %d", stats.TotalVersions)
	}
}

func TestOutputStatsTable(t *testing.T) {
	t.Parallel()

	stats := PackageStats{
		PackageName:      "myimage",
		TotalVersions:    10,
		TaggedVersions:   3,
		UntaggedVersions: 7,
		TotalTags:        5,
		OldestVersion:    "2024-01-01T00:00:00Z",
		NewestVersion:    "2025-01-15T00:00:00Z",
	}

	var buf bytes.Buffer
	err := OutputStatsTable(&buf, stats, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Check key statistics are present
	if !strings.Contains(output, "myimage") {
		t.Error("output should contain package name")
	}
	if !strings.Contains(output, "10") {
		t.Error("output should contain total versions count")
	}
	if !strings.Contains(output, "3") {
		t.Error("output should contain tagged versions count")
	}
	if !strings.Contains(output, "7") {
		t.Error("output should contain untagged versions count")
	}
}

func TestOutputStatsTableQuiet(t *testing.T) {
	t.Parallel()

	stats := PackageStats{
		PackageName:      "myimage",
		TotalVersions:    10,
		TaggedVersions:   3,
		UntaggedVersions: 7,
	}

	var buf bytes.Buffer
	err := OutputStatsTable(&buf, stats, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Quiet mode should not include decorative headers
	if strings.Contains(output, "Statistics for") {
		t.Error("quiet mode should not include 'Statistics for' header")
	}
}

// =============================================================================
// Tests for ExecuteStats
// =============================================================================

func TestExecuteStats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		versions   []gh.PackageVersionInfo
		listerErr  error
		params     StatsParams
		wantErr    bool
		wantOutput []string
	}{
		{
			name: "successful stats with versions",
			versions: []gh.PackageVersionInfo{
				{ID: 1, Tags: []string{"v1.0", "latest"}, CreatedAt: "2025-01-15T10:00:00Z"},
				{ID: 2, Tags: []string{"v0.9"}, CreatedAt: "2025-01-10T10:00:00Z"},
				{ID: 3, Tags: []string{}, CreatedAt: "2025-01-05T10:00:00Z"},
			},
			params: StatsParams{
				Owner:       "testowner",
				OwnerType:   "user",
				PackageName: "testimage",
				JSONOutput:  false,
				QuietMode:   false,
			},
			wantErr: false,
			wantOutput: []string{
				"Statistics for testimage",
				"Total versions:",
				"3",
				"Tagged versions:",
				"2",
				"Untagged versions:",
				"1",
			},
		},
		{
			name:     "empty package",
			versions: []gh.PackageVersionInfo{},
			params: StatsParams{
				Owner:       "testowner",
				OwnerType:   "user",
				PackageName: "emptyimage",
				JSONOutput:  false,
				QuietMode:   false,
			},
			wantErr: false,
			wantOutput: []string{
				"Total versions:",
				"0",
			},
		},
		{
			name: "json output",
			versions: []gh.PackageVersionInfo{
				{ID: 1, Tags: []string{"v1.0"}, CreatedAt: "2025-01-15T10:00:00Z"},
			},
			params: StatsParams{
				Owner:       "testowner",
				OwnerType:   "user",
				PackageName: "jsonimage",
				JSONOutput:  true,
				QuietMode:   false,
			},
			wantErr: false,
			wantOutput: []string{
				`"package_name": "jsonimage"`,
				`"total_versions": 1`,
				`"tagged_versions": 1`,
			},
		},
		{
			name: "quiet mode",
			versions: []gh.PackageVersionInfo{
				{ID: 1, Tags: []string{"v1.0"}, CreatedAt: "2025-01-15T10:00:00Z"},
			},
			params: StatsParams{
				Owner:       "testowner",
				OwnerType:   "user",
				PackageName: "quietimage",
				JSONOutput:  false,
				QuietMode:   true,
			},
			wantErr:    false,
			wantOutput: []string{"Total versions:"},
		},
		{
			name:      "lister error",
			listerErr: fmt.Errorf("API error"),
			params: StatsParams{
				Owner:       "testowner",
				OwnerType:   "user",
				PackageName: "errorimage",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockVersionLister{
				versions: tt.versions,
				err:      tt.listerErr,
			}

			var buf bytes.Buffer
			ctx := context.Background()

			err := ExecuteStats(ctx, mock, tt.params, &buf)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteStats() error = %v, wantErr = %v", err, tt.wantErr)
				return
			}

			output := buf.String()
			for _, want := range tt.wantOutput {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\nGot:\n%s", want, output)
				}
			}
		})
	}
}

func TestExecuteStats_QuietModeNoHeader(t *testing.T) {
	t.Parallel()

	mock := &mockVersionLister{
		versions: []gh.PackageVersionInfo{
			{ID: 1, Tags: []string{"v1.0"}, CreatedAt: "2025-01-15T10:00:00Z"},
		},
	}

	var buf bytes.Buffer
	params := StatsParams{
		Owner:       "testowner",
		OwnerType:   "user",
		PackageName: "myimage",
		JSONOutput:  false,
		QuietMode:   true,
	}

	err := ExecuteStats(context.Background(), mock, params, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "Statistics for") {
		t.Error("quiet mode should not include 'Statistics for' header")
	}
}
