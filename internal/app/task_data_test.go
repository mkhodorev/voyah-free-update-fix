package app

import (
	"encoding/json"
	"io"
	"os"
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

	t.Run("missing otx does not block fix readiness", func(t *testing.T) {
		t.Parallel()

		rows := []packageRow{
			{ECU: "ADAS", Status: "downloaded", FileExists: true, UpgradeSpecExists: false, OriginalIndex: 0},
			{ECU: iviMPUName, Status: "pending", FileExists: false, UpgradeSpecExists: false, OriginalIndex: 1},
		}

		fixRequired, ids := checkFixContextDB(rows)
		if !fixRequired {
			t.Fatal("expected fixRequired=true when only exceptional package is missing and required package has enc file")
		}

		want := []int{1}
		if !reflect.DeepEqual(ids, want) {
			t.Fatalf("unexpected ids to delete: got %v, want %v", ids, want)
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

func TestIsFixStateAllowed(t *testing.T) {
	t.Parallel()

	t.Run("download process state is allowed", func(t *testing.T) {
		t.Parallel()

		taskData := otaTaskData{
			//nolint:misspell
			DownloadState: otaDownloadState{Stage: "Retrive Packages"},
			OverallState:  otaOverallState{Stage: "Download", State: "Process"},
		}

		if !isFixStateAllowed(taskData) {
			t.Fatal("expected state to be allowed")
		}
	})

	t.Run("flash failure state is allowed", func(t *testing.T) {
		t.Parallel()

		taskData := otaTaskData{
			DownloadState: otaDownloadState{Stage: "Complete"},
			OverallState:  otaOverallState{Stage: "Terminate", State: "Failed"},
			FlashState: &otaFlashState{
				FailureReason:          "FLASH_FAIL",
				FailureReasonExtraInfo: "IVI_MPU flash failed",
			},
		}

		if !isFixStateAllowed(taskData) {
			t.Fatal("expected flash-failure state to be allowed")
		}
	})

	t.Run("complete terminate idle state is allowed", func(t *testing.T) {
		t.Parallel()

		taskData := otaTaskData{
			DownloadState: otaDownloadState{Stage: "Complete"},
			OverallState:  otaOverallState{Stage: "Terminate", State: "Idle"},
		}

		if !isFixStateAllowed(taskData) {
			t.Fatal("expected complete/terminate/idle state to be allowed")
		}
	})

	t.Run("terminate failed with unknown state is not allowed", func(t *testing.T) {
		t.Parallel()

		taskData := otaTaskData{
			DownloadState: otaDownloadState{Stage: "Complete"},
			OverallState:  otaOverallState{Stage: "Terminate", State: "XZ"},
		}

		if isFixStateAllowed(taskData) {
			t.Fatal("expected state to be rejected with unknown overall_state.state")
		}
	})
}

func TestPrintTaskOverviewShowsDownloadFailInfo(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	output := captureStdout(t, func() {
		printTaskOverview(otaTaskData{
			//nolint:misspell
			DownloadState: otaDownloadState{
				FailInfo: "network timeout",
				Percents: 42,
				Stage:    "Retrive Packages",
			},
		}, newTextStyle())
	})

	if !strings.Contains(output, "Download state fail info: network timeout") {
		t.Fatalf("expected download fail info in output, got:\n%s", output)
	}
}

func TestPrintTaskOverviewAllowsFlashFailureCaseWithoutFlashState(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	output := captureStdout(t, func() {
		printTaskOverview(otaTaskData{
			DownloadState: otaDownloadState{
				Percents: 100,
				Stage:    downloadStageComplete,
			},
			OverallState: otaOverallState{
				Stage: overallStageTerminate,
				State: overallStateIdle,
			},
		}, newTextStyle())
	})

	if strings.Contains(output, "Flash failure reason") {
		t.Fatalf("expected missing flash state to skip flash failure details, got:\n%s", output)
	}
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

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}

	os.Stdout = writer

	defer func() {
		os.Stdout = originalStdout
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}

	if err := reader.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}

	return string(output)
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

func TestCollectMissingUpgradeSpecs(t *testing.T) {
	t.Parallel()

	rows := []packageRow{
		{ECU: "ADAS", Status: "downloaded", FileExists: true, UpgradeSpecExists: false},
		{ECU: "BMS", Status: "downloaded", FileExists: true, UpgradeSpecExists: true},
		{ECU: "VCU", Status: "downloading", FileExists: true, UpgradeSpecExists: false},
		{ECU: "T-BOX", Status: "flashed", FileExists: true, UpgradeSpecExists: false},
		{ECU: "IVI_MPU", Status: "downloaded", FileExists: false, UpgradeSpecExists: false},
	}

	got := collectMissingUpgradeSpecs(rows)
	want := []string{"ADAS", "T-BOX"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectMissingUpgradeSpecs() = %v, want %v", got, want)
	}
}

func TestExtractTargetBaselineVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "plain OTA version",
			text: "OTA 6.5.4 update",
			want: "6.5.4",
		},
		{
			name: "prefers three component version",
			text: "OTA6.6 update (V6.6.1)",
			want: "6.6.1",
		},
		{name: "not found", text: "OTA update", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := extractTargetBaselineVersion(tt.text); got != tt.want {
				t.Fatalf("extractTargetBaselineVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrintUnsupportedReadinessStatus(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	output := captureStdout(t, func() {
		printUnsupportedReadinessStatus(newTextStyle())
	})

	for _, expected := range []string{
		"The OTA task is in an unsupported state",
		"download_state.stage='Retrive Packages'",
		"download_state.stage='Complete'",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in output, got:\n%s", expected, output)
		}
	}
}

func TestPrintFlashFailureStatusBlocksUnknownTargetVersion(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	rows := []packageRow{{
		ECU:           iviMPUName,
		Status:        packageStatusPending,
		FileExists:    false,
		OriginalIndex: 0,
	}}
	output := captureStdout(t, func() {
		printFlashFailureStatus(otaTaskData{}, rows, newTextStyle())
	})

	if !strings.Contains(output, "target version could not be determined") ||
		strings.Contains(output, "The context.db fix can be started.") {
		t.Fatalf("unexpected FlashFailure readiness output:\n%s", output)
	}
}
