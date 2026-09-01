package app

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

type fixtureFileClient struct {
	t             *testing.T
	taskData      string
	missingECUs   map[string]struct{}
	packageByPath map[string]string
}

func (c fixtureFileClient) Close() error { return nil }

func (c fixtureFileClient) Exists(filePath string) bool {
	return filePath == filepath.ToSlash(filepath.Join(tboxDir, contextDBName))
}

func (c fixtureFileClient) ExistsAndNotEmpty(filePath string) bool {
	if ecu, ok := c.packageByPath[filePath]; ok {
		_, missing := c.missingECUs[ecu]

		return !missing
	}

	return false
}

func (c fixtureFileClient) Download(remotePath, localPath string) error {
	if remotePath != filepath.ToSlash(filepath.Join(tboxDir, contextDBName)) {
		return os.ErrNotExist
	}

	copyFixtureFile(c.t, "context.db", localPath)

	db, err := sql.Open("sqlite", localPath)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`UPDATE ota_task SET taskData = ? WHERE type = 'task_info'`, c.taskData)

	return err
}

func (c fixtureFileClient) Upload(_, _ string) error { return nil }
func (c fixtureFileClient) Remove(string) error      { return nil }

func fixturePath(t *testing.T, name string) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to determine fixture path")
	}

	return filepath.Join(filepath.Dir(currentFile), "..", "..", "tests", "fixtures", name)
}

func readFixture(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(fixturePath(t, name))
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}

	return string(data)
}

func copyFixtureFile(t *testing.T, name, dst string) {
	t.Helper()

	data, err := os.ReadFile(fixturePath(t, name))
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("failed to write fixture copy %s: %v", dst, err)
	}
}

func fixtureFileClientForTask(t *testing.T, taskData string, missingECUs ...string) fixtureFileClient {
	t.Helper()

	parsed, err := parseOTATaskData(taskData)
	if err != nil {
		t.Fatalf("failed to parse fixture taskData: %v", err)
	}

	paths := make(map[string]string, len(parsed.PackagesInfo)*2)
	for _, pkg := range parsed.PackagesInfo {
		paths[pkg.File] = pkg.ECU
		paths[pkg.UpgradeSpecFile] = pkg.ECU
	}

	missing := make(map[string]struct{}, len(missingECUs))
	for _, ecu := range missingECUs {
		missing[ecu] = struct{}{}
	}

	return fixtureFileClient{t: t, taskData: taskData, missingECUs: missing, packageByPath: paths}
}

func TestAnalyzeContextDBWithExistingFixtures(t *testing.T) {
	tests := []struct {
		name        string
		fixtureName string
		missingECUs []string
		wantNil     bool
		wantMode    contextDBFixMode
		wantECUs    []string
		wantTarget  string
		inferred    bool
	}{
		{
			name:        "idle task infers target version and removes both IVI packages",
			fixtureName: "origin_idle.txt",
			missingECUs: []string{iviMCUName, iviMPUName},
			wantMode:    contextDBFixModeFlashFailure,
			wantECUs:    []string{iviMPUName, iviMCUName},
			wantTarget:  "6.5.4",
			inferred:    true,
		},
		{
			name:        "failed task preserves target version",
			fixtureName: "origin_flash_fail.txt",
			missingECUs: []string{iviMPUName},
			wantMode:    contextDBFixModeFlashFailure,
			wantECUs:    []string{iviMPUName},
			wantTarget:  "6.6.1",
		},
		{
			name:        "before_1 removes both undownloaded IVI packages",
			fixtureName: "../../before_1.json",
			missingECUs: []string{iviMCUName, iviMPUName},
			wantMode:    contextDBFixModeUndownloaded,
			wantECUs:    []string{iviMPUName, iviMCUName},
		},
		{
			name:        "missing required package does not offer a fix",
			fixtureName: "origin.txt",
			missingECUs: []string{"BMS"},
			wantNil:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workdir := t.TempDir()
			previousWorkdir, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}
			if err := os.Chdir(workdir); err != nil {
				t.Fatalf("failed to switch working directory: %v", err)
			}
			t.Cleanup(func() { _ = os.Chdir(previousWorkdir) })

			fix, err := analyzeContextDB(fixtureFileClientForTask(t, readFixture(t, tt.fixtureName), tt.missingECUs...))
			if err != nil {
				t.Fatalf("analyzeContextDB returned error: %v", err)
			}
			if tt.wantNil {
				if fix != nil {
					t.Fatalf("expected no fix, got %+v", *fix)
				}

				return
			}
			if fix == nil {
				t.Fatal("expected a fix")
			}
			if fix.Mode != tt.wantMode || !reflect.DeepEqual(fix.PackageECUs, tt.wantECUs) ||
				fix.TargetBaselineVersion != tt.wantTarget || fix.TargetVersionInferred != tt.inferred {
				t.Fatalf("unexpected fix: %+v", *fix)
			}
		})
	}
}

func TestApplyContextDBFixWithFlashFailureFixtures(t *testing.T) {
	workdir := t.TempDir()
	dbPath := filepath.Join(workdir, "context.db")
	copyFixtureFile(t, "context.db", dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open fixture DB: %v", err)
	}
	defer db.Close()

	before := readFixture(t, "flash_failure_ivi_missing_before.json")
	if _, err := db.Exec(`UPDATE ota_task SET taskData = ? WHERE type = 'task_info'`, before); err != nil {
		t.Fatalf("failed to write before fixture: %v", err)
	}

	taskData, err := parseOTATaskData(before)
	if err != nil {
		t.Fatalf("failed to parse before fixture: %v", err)
	}
	packageIDs := make([]int, 0, 2)
	for index, pkg := range taskData.PackagesInfo {
		if pkg.ECU == iviMCUName || pkg.ECU == iviMPUName {
			packageIDs = append(packageIDs, index)
		}
	}

	fix, err := buildContextDBFix(taskData, packageIDs, contextDBFixModeFlashFailure)
	if err != nil {
		t.Fatalf("failed to build fix: %v", err)
	}

	startedAt := time.Now()
	if err := applyContextDBFix(db, fix); err != nil {
		t.Fatalf("applyContextDBFix returned error: %v", err)
	}

	var got, want map[string]any
	var fixedTaskData string
	if err := db.QueryRow(`SELECT taskData FROM ota_task WHERE type = 'task_info'`).Scan(&fixedTaskData); err != nil {
		t.Fatalf("failed to read fixed taskData: %v", err)
	}
	if err := json.Unmarshal([]byte(fixedTaskData), &got); err != nil {
		t.Fatalf("failed to parse fixed taskData: %v", err)
	}
	if err := json.Unmarshal([]byte(readFixture(t, "flash_failure_ivi_missing_after.json")), &want); err != nil {
		t.Fatalf("failed to parse after fixture: %v", err)
	}

	expireTime, ok := got["expire_time"].(float64)
	if !ok {
		t.Fatalf("expire_time has unexpected type: %T", got["expire_time"])
	}
	wantExpiry := startedAt.AddDate(0, 0, flashFailureExpiryDays).Unix()
	if delta := int64(expireTime) - wantExpiry; delta < 0 || delta > 2 {
		t.Fatalf("unexpected expire_time: got %d, want approximately %d", int64(expireTime), wantExpiry)
	}

	want["expire_time"] = expireTime
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fixed taskData does not match after fixture")
	}
}

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
