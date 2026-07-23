package app

import (
	"reflect"
	"strings"
	"testing"
)

//nolint:cyclop
func TestBuildContextDBFix(t *testing.T) {
	t.Parallel()

	t.Run("infers missing flash failure target version", func(t *testing.T) {
		t.Parallel()

		taskData := otaTaskData{
			ReleaseNoteBrief: "OTA6.6 update (V6.6.1)",
			PackagesInfo: []otaPackageInfo{
				{ECU: "BMS"},
				{ECU: iviMPUName},
			},
		}

		fix, err := buildContextDBFix(taskData, []int{1}, contextDBFixModeFlashFailure)
		if err != nil {
			t.Fatalf("buildContextDBFix returned error: %v", err)
		}

		if fix.TargetBaselineVersion != "6.6.1" || !fix.TargetVersionInferred {
			t.Fatalf("unexpected inferred target: %+v", fix)
		}

		if !reflect.DeepEqual(fix.PackageECUs, []string{iviMPUName}) {
			t.Fatalf("unexpected package ECUs: %v", fix.PackageECUs)
		}
	})

	t.Run("preserves existing target version", func(t *testing.T) {
		t.Parallel()

		taskData := otaTaskData{
			TargetBaselineVersion: "6.16.1",
			ReleaseNoteBrief:      "contains 1.2.3",
			PackagesInfo:          []otaPackageInfo{{ECU: tboxName}},
		}

		fix, err := buildContextDBFix(taskData, []int{0}, contextDBFixModeFlashFailure)
		if err != nil {
			t.Fatalf("buildContextDBFix returned error: %v", err)
		}

		if fix.TargetBaselineVersion != "6.16.1" || fix.TargetVersionInferred {
			t.Fatalf("unexpected target handling: %+v", fix)
		}
	})

	t.Run("rejects flash failure without target version", func(t *testing.T) {
		t.Parallel()

		_, err := buildContextDBFix(
			otaTaskData{PackagesInfo: []otaPackageInfo{{ECU: iviMCUName}}},
			[]int{0},
			contextDBFixModeFlashFailure,
		)
		if err == nil || !strings.Contains(err.Error(), "no X.Y.Z version") {
			t.Fatalf("expected missing target version error, got %v", err)
		}
	})

	t.Run("undownloaded fix does not require target version", func(t *testing.T) {
		t.Parallel()

		fix, err := buildContextDBFix(
			otaTaskData{PackagesInfo: []otaPackageInfo{{ECU: iviMCUName}}},
			[]int{0},
			contextDBFixModeUndownloaded,
		)
		if err != nil {
			t.Fatalf("buildContextDBFix returned error: %v", err)
		}

		if fix.TargetBaselineVersion != "" || fix.TargetVersionInferred {
			t.Fatalf("unexpected target version for undownloaded fix: %+v", fix)
		}
	})
}

//nolint:paralleltest // captureStdout temporarily replaces process-wide stdout.
func TestPrintInferredTargetWarning(t *testing.T) {
	output := captureStdout(t, func() {
		printInferredTargetWarning("6.6.1")
	})

	if !strings.Contains(output, "Version 6.6.1 was extracted") ||
		!strings.Contains(output, "Please verify") {
		t.Fatalf("unexpected warning output:\n%s", output)
	}
}
