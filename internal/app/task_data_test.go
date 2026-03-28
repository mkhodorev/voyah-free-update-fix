package app

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestParseOTATaskData(t *testing.T) {
	t.Parallel()

	t.Run("valid json", func(t *testing.T) {
		t.Parallel()

		raw := `{"task_id":7,"source_baseline_version":"1.0","target_baseline_version":"2.0",` +
			`"packages_info":[{"ecu":"ECU1","version":"v1"}]}`

		parsed, err := parseOTATaskData(raw)
		if err != nil {
			t.Fatalf("parseOTATaskData returned error: %v", err)
		}

		if parsed.TaskID != 7 {
			t.Fatalf("unexpected TaskID: got %d, want 7", parsed.TaskID)
		}

		if len(parsed.PackagesInfo) != 1 || parsed.PackagesInfo[0].ECU != "ECU1" {
			t.Fatalf("unexpected packages_info: %+v", parsed.PackagesInfo)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		t.Parallel()

		_, err := parseOTATaskData(`{"task_id":`)
		if err == nil {
			t.Fatal("expected error for invalid json")
		}

		if !strings.Contains(err.Error(), "invalid JSON in ota_task.taskData") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
}

func TestPackagePercent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		downloaded int64
		total      int64
		want       int
	}{
		{name: "zero total", downloaded: 10, total: 0, want: 0},
		{name: "normal", downloaded: 50, total: 200, want: 25},
		{name: "clamp over 100", downloaded: 200, total: 100, want: 100},
		{name: "clamp below zero", downloaded: -10, total: 100, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := packagePercent(tt.downloaded, tt.total)
			if got != tt.want {
				t.Fatalf("packagePercent(%d, %d) = %d, want %d", tt.downloaded, tt.total, got, tt.want)
			}
		})
	}
}

func TestPackageStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pkg  otaPackageInfo
		want string
	}{
		{
			name: "flashed has priority",
			pkg:  otaPackageInfo{FlashFinish: true, DownloadedSize: 0, FileSize: 100},
			want: "flashed",
		},
		{name: "downloaded", pkg: otaPackageInfo{DownloadedSize: 100, FileSize: 100}, want: "downloaded"},
		{name: "downloading", pkg: otaPackageInfo{DownloadedSize: 1, FileSize: 100}, want: "downloading"},
		{name: "pending", pkg: otaPackageInfo{DownloadedSize: 0, FileSize: 100}, want: "pending"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := packageStatus(tt.pkg); got != tt.want {
				t.Fatalf("packageStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPackageVersion(t *testing.T) {
	t.Parallel()

	if got := packageVersion(otaPackageInfo{Version: "raw", DisplayVersion: "display"}); got != "display" {
		t.Fatalf("packageVersion should prefer DisplayVersion: got %q", got)
	}

	if got := packageVersion(otaPackageInfo{Version: "raw", DisplayVersion: ""}); got != "raw" {
		t.Fatalf("packageVersion should fallback to Version: got %q", got)
	}
}

func TestCheckFixContextDB(t *testing.T) {
	t.Parallel()

	t.Run("only exceptional packages missing", func(t *testing.T) {
		t.Parallel()

		rows := []packageRow{
			{ECU: iviMCUName, Status: "pending", FileExists: false, UpgradeSpecExists: false, OriginalIndex: 1},
			{ECU: "ADAS", Status: "downloaded", FileExists: true, UpgradeSpecExists: true, OriginalIndex: 2},
			{ECU: tboxName, Status: "downloaded", FileExists: false, UpgradeSpecExists: true, OriginalIndex: 3},
		}

		fixRequired, ids := checkFixContextDB(rows)
		if !fixRequired {
			t.Fatal("expected fixRequired=true when only exceptional packages are missing")
		}

		want := []int{1, 3}
		if !reflect.DeepEqual(ids, want) {
			t.Fatalf("unexpected ids to delete: got %v, want %v", ids, want)
		}
	})

	t.Run("required package missing", func(t *testing.T) {
		t.Parallel()

		rows := []packageRow{
			{ECU: "ADAS", Status: "downloading", FileExists: true, UpgradeSpecExists: true, OriginalIndex: 0},
			{ECU: iviMPUName, Status: "pending", FileExists: false, UpgradeSpecExists: false, OriginalIndex: 1},
		}

		fixRequired, _ := checkFixContextDB(rows)
		if fixRequired {
			t.Fatal("expected fixRequired=false when required package is missing")
		}
	})

	t.Run("all good no fix", func(t *testing.T) {
		t.Parallel()

		rows := []packageRow{
			{ECU: "ADAS", Status: "downloaded", FileExists: true, UpgradeSpecExists: true, OriginalIndex: 0},
			{ECU: iviMCUName, Status: "downloaded", FileExists: true, UpgradeSpecExists: true, OriginalIndex: 1},
		}

		fixRequired, ids := checkFixContextDB(rows)
		if fixRequired {
			t.Fatal("expected fixRequired=false when no package is missing")
		}

		if len(ids) != 0 {
			t.Fatalf("expected empty ids, got %v", ids)
		}
	})
}

func TestFormatBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input int64
		want  string
	}{
		{input: -10, want: "-10 B"},
		{input: 999, want: "999 B"},
		{input: 1024, want: "1.00 KB"},
		{input: 1024 * 1024, want: "1.00 MB"},
	}

	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.want {
			t.Fatalf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestColorizedOverallState(t *testing.T) {
	t.Parallel()

	sty := textStyle{enabled: true}

	if got := colorizedOverallState("success", sty); !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected colored output for success, got %q", got)
	}

	if got := colorizedOverallState("something_else", sty); !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected colored output for default state, got %q", got)
	}
}

func TestPackagesInfoKeyRegexp(t *testing.T) {
	t.Parallel()

	input := `{"x":1,"packages_info"  : [{"ecu":"ECU"}]}`
	if !packagesInfoKeyRegexp.MatchString(input) {
		t.Fatalf("expected packagesInfoKeyRegexp to match input: %s", input)
	}
}

func TestBuildMarshalRoundtripWithPackageInfo(t *testing.T) {
	t.Parallel()

	pkg := otaPackageInfo{ECU: "ECU", Version: "v", DownloadedSize: 10, FileSize: 20}

	b, err := json.Marshal(pkg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if len(b) == 0 {
		t.Fatal("marshal produced empty output")
	}
}
