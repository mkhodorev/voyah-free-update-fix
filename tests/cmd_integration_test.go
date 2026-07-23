//go:build integration

package integration

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

const (
	containerName         = "voyah-integration-test-target"
	remoteDBPath          = "/mnt/ota/data/fota/context.db"
	remoteContextJSONPath = "/mnt/ota/data/fota/context.json"

	bmsEnc    = "/mnt/ota/data/fota/download/BMS/1c300c23da1394b143c0ad1e85c2df1cd8b815bdcc6f7fbca07ab452ccf3e6c7.enc.full"
	bmsOtx    = "/mnt/ota/data/fota/download/BMS/a0bd5ee5ccc5bf80cc904eaa0fd96c9ff93b07d064b29d7632db8b07d6354ffa.otx"
	bleEnc    = "/mnt/ota/data/fota/download/BLE/1e66522c39e86a0bc040224b681671f3bb1c09ec82ae5dd1ff169208a3ef13fa.enc.full"
	bleOtx    = "/mnt/ota/data/fota/download/BLE/6fcaa328487f0e0ef05f92ba98d4007809863b7507aadf7feb984b3e36e29008.otx"
	mcuf0Enc  = "/mnt/ota/data/fota/download/MCUF0/21d6be0a84c3692bdf4e7ddc5d6ae67616b6d96446bcd13607c658521d0fdac1.enc.full"
	mcuf0Otx  = "/mnt/ota/data/fota/download/MCUF0/7d7d288b0d452f41a065125e2e17ea6f4b9e818f4e0db4fe3a44c1ea16ad62dc.otx"
	bcmEnc    = "/mnt/ota/data/fota/download/BCM/35a515039e101195dd20d4234b51a464b31ac3401e841219ff61ac8a3599cb15.enc.full"
	bcmOtx    = "/mnt/ota/data/fota/download/BCM/a8474f110280f7930b39fe22401c0421cc386b948fb2ace3fb614dc601bd04f1.otx"
	gtwEnc    = "/mnt/ota/data/fota/download/GTW/370752f474f766268fb379cbc21c5bc34e46a01b9bae2ba171117663a81f9cd7.enc.full"
	gtwOtx    = "/mnt/ota/data/fota/download/GTW/06b4309fd54740d1487a250bd34789b693e9baef0e1d0ecd1ff9e056f8d16918.otx"
	dscuEnc   = "/mnt/ota/data/fota/download/DSCU/44883e96c33863e544afb6910605a603d3bb542db29c0f7dda3abd1abedbe36e.enc.full"
	dscuOtx   = "/mnt/ota/data/fota/download/DSCU/fe026de01dcd95952e65fa08bc54364f22647eef974d50ffc1a83cce4666e450.otx"
	iviMcuEnc = "/mnt/ota/data/fota/download/IVI_MCU/52be2acd4dadd7efa3276c694294f87f7820f41c06a0599876e01206e2474110.enc.full"
	iviMpuEnc = "/mnt/ota/data/fota/download/IVI_MPU/d5414999a98e6109c4d5b8951fa577caa9b53e3f01a171dac1202c83b49187d5.enc.full"
	acEnc     = "/mnt/ota/data/fota/download/AC/6a75074f6d79f4db5d81be2c8e504209ceef5180be5017373c8780b0584df8e6.enc.full"
	acOtx     = "/mnt/ota/data/fota/download/AC/5ed57e69074d4865b20db966b32355c17eca82bdfea1a8a385a71f2a3d41ea0a.otx"
	tBoxEnc   = "/mnt/ota/data/fota/download/T-BOX/8824cf636494f3a86ff10c406065a9c956f7bbcc2960177e17a1c6a2d8a5481f.enc.full"
	tBoxEnc2  = "/mnt/ota/data/fota/download/T-BOX/390ae279d8d8765ec1b170ed8dd2d6472cb43cd7edd5984579f0af5230c21ed1.enc.full"
	tBoxOtx   = "/mnt/ota/data/fota/download/T-BOX/49b4536d036f54f8fa065bc592836bffb967163fbab4a759c554b356b7d2ff5e.otx"
	vcuEnc    = "/mnt/ota/data/fota/download/VCU/cfc0168a15c3d9e33a706c1bea6c2e108d0bd4a07274bfee54a841a6007fcc3b.enc.full"
	vcuOtx    = "/mnt/ota/data/fota/download/VCU/c099a0788b66e8d18f6f94f03b9e6f8eda7e6b6f835021f65c58ecbaf2df2466.otx"
	iviMcuOtx = "/mnt/ota/data/fota/download/IVI_MCU/59023bed66f21fd4a7ba00f9f95ea012189660cf6ff5d6e69402da081be0d4ac.otx"
	iviMpuOtx = "/mnt/ota/data/fota/download/IVI_MPU/a770e0597ef6fa8288e12fc16d1f9f10695d5124e6d1ffc48a411c07cc068161.otx"
)

type testDirs struct {
	repoRoot    string
	testsDir    string
	dockerDir   string
	fixturesDir string
	composeFile string
	tmpDir      string
}

type fixContextDBCmdRunConfig struct {
	originTaskFileName   string
	modifiedTaskFileName string
	expectedTaskFileName string
	filesToCreate        []string
	expectBackup         bool
	disableSFTP          bool
	expectFlashFailure   bool
	expectedRemovedECUs  []string
	expectedTarget       string
}

func TestFixContextDB_FlashFailureWithMissingIVIPackages(t *testing.T) {
	runFixContextDBViaCmd(t, flashFailureWithMissingIVIPackagesConfig(false))
}

func TestFixContextDBViaSCP_FlashFailureWithMissingIVIPackages(t *testing.T) {
	runFixContextDBViaCmd(t, flashFailureWithMissingIVIPackagesConfig(true))
}

func flashFailureWithMissingIVIPackagesConfig(disableSFTP bool) fixContextDBCmdRunConfig {
	return fixContextDBCmdRunConfig{
		originTaskFileName:   "flash_failure_ivi_missing_before.json",
		expectedTaskFileName: "flash_failure_ivi_missing_after.json",
		filesToCreate: []string{
			remoteContextJSONPath,
			"/mnt/ota/data/fota/download/BMS/0d7147dc742eab662637e44d5c39bf8d8bb1647d2c9de77826a658558352cd83.enc.full",
			bmsOtx,
			"/mnt/ota/data/fota/download/ADCU/16252442ea7f0edc7d7ea7f202996ff78cec5764cf9ff6b74bf236cc249fe2cc.enc.full",
			"/mnt/ota/data/fota/download/ADCU/74d40c30770ea9abecbfa07003b72028f5dc6a1dc6daa01e57a6609735d3e029.otx",
			"/mnt/ota/data/fota/download/BCM/2e2a5f015533a14a651944b39f5d0e70d828485f20dbdc3eb608821260badbd5.enc.full",
			bcmOtx,
			"/mnt/ota/data/fota/download/T-BOX/9a305a8e1988fa10fe6e4a843288e77e24958b7d89f50f249256940775f93fc8.enc.full",
			tBoxOtx,
			"/mnt/ota/data/fota/download/MCUF0/a5b7d3abdd6ca92cfd2872df9f9ba2f2d4ba368b569d50dd4492b2be6f33747b.enc.full",
			mcuf0Otx,
			"/mnt/ota/data/fota/download/POT/b578aed902c5a160d9ba10ba899da223b19313c52fc1ba8007e3e4362ab4e2e4.enc.full",
			"/mnt/ota/data/fota/download/POT/a819ed5204044974f1227b75abccb5cad8e3382a7139f6687f04133af7889e88.otx",
			"/mnt/ota/data/fota/download/VCU/bd7bfb90b3a14d634eecce46ced49ddf01924c3735186e923db1f16871935051.enc.full",
			vcuOtx,
			"/mnt/ota/data/fota/download/MCUR0/d50ea4d31107e0ca9c897497478b9f27253ea8363fcb8873cdbee9326245c284.enc.full",
			"/mnt/ota/data/fota/download/MCUR0/26ab12b6562566db22a812d3997f26b532dc193616752e1a74234e345035b822.otx",
			"/mnt/ota/data/fota/download/GTW/e80d1ca02c951c16d4e8653a5708f89fd12697dd9e079de351fe17c8908a327a.enc.full",
			gtwOtx,
			"/mnt/ota/data/fota/download/OBC/fe6db2991c8146c6caf59c98d9acabec62c4f5e70dfe19a1422ab7a6322e251b.enc.full",
			"/mnt/ota/data/fota/download/OBC/a0c2276c62adefe6d252ce228ef4cceedf784734cb9cebca7179343055e6a66f.otx",
		},
		expectBackup:        true,
		disableSFTP:         disableSFTP,
		expectFlashFailure:  true,
		expectedRemovedECUs: []string{"IVI_MCU", "IVI_MPU"},
		expectedTarget:      "6.5.4",
	}
}

func TestFixContextDB_WithoutIVI_MPU_IVI_MCU(t *testing.T) {
	cfg := fixContextDBCmdRunConfig{
		originTaskFileName:   "origin.txt",
		modifiedTaskFileName: "modified_without_ivi_mpu_ivi_mcu.txt",
		filesToCreate: []string{
			remoteContextJSONPath,
			bmsEnc,
			bmsOtx,
			bleEnc,
			bleOtx,
			mcuf0Enc,
			mcuf0Otx,
			bcmEnc,
			bcmOtx,
			gtwEnc,
			gtwOtx,
			dscuEnc,
			dscuOtx,
			acEnc,
			acOtx,
			tBoxEnc,
			tBoxOtx,
			vcuEnc,
			vcuOtx,
		},
		expectBackup: true,
	}

	runFixContextDBViaCmd(t, cfg)
}

func TestFixContextDB_WithoutIVI_MPU_IVI_MCU_and_context_json_exists(t *testing.T) {
	cfg := fixContextDBCmdRunConfig{
		originTaskFileName:   "origin.txt",
		modifiedTaskFileName: "modified_without_ivi_mpu_ivi_mcu.txt",
		filesToCreate: []string{
			remoteContextJSONPath,
			bmsEnc,
			bmsOtx,
			bleEnc,
			bleOtx,
			mcuf0Enc,
			mcuf0Otx,
			bcmEnc,
			bcmOtx,
			gtwEnc,
			gtwOtx,
			dscuEnc,
			dscuOtx,
			acEnc,
			acOtx,
			tBoxEnc,
			tBoxOtx,
			vcuEnc,
			vcuOtx,
		},
		expectBackup: true,
	}

	runFixContextDBViaCmd(t, cfg)
}

func TestFixContextDB_WithoutIVI_MPU_IVI_MCU_and_context_json_exists_and_without_BCM_OTX(t *testing.T) {
	cfg := fixContextDBCmdRunConfig{
		originTaskFileName:   "origin.txt",
		modifiedTaskFileName: "modified_without_ivi_mpu_ivi_mcu.txt",
		filesToCreate: []string{
			remoteContextJSONPath,
			bmsEnc,
			bmsOtx,
			bleEnc,
			bleOtx,
			mcuf0Enc,
			mcuf0Otx,
			bcmEnc,
			gtwEnc,
			gtwOtx,
			dscuEnc,
			dscuOtx,
			acEnc,
			acOtx,
			tBoxEnc,
			tBoxOtx,
			vcuEnc,
			vcuOtx,
		},
		expectBackup: true,
	}

	runFixContextDBViaCmd(t, cfg)
}

func TestFixContextDB_WithoutIVI_MPU_IVI_MCU_and_when_tbox_files_not_exist(t *testing.T) {
	cfg := fixContextDBCmdRunConfig{
		originTaskFileName:   "origin.txt",
		modifiedTaskFileName: "modified_without_ivi_mpu_ivi_mcu_tbox.txt",
		filesToCreate: []string{
			bmsEnc,
			bmsOtx,
			bleEnc,
			bleOtx,
			mcuf0Enc,
			mcuf0Otx,
			bcmEnc,
			bcmOtx,
			gtwEnc,
			gtwOtx,
			dscuEnc,
			dscuOtx,
			acEnc,
			acOtx,
			tBoxOtx,
			vcuEnc,
			vcuOtx,
		},
		expectBackup: true,
	}

	runFixContextDBViaCmd(t, cfg)
}

func TestFixContextDB_when_not_required_download_files_exist(t *testing.T) {
	cfg := fixContextDBCmdRunConfig{
		originTaskFileName:   "origin.txt",
		modifiedTaskFileName: "origin.txt",
		filesToCreate: []string{
			bmsOtx,
			bleEnc,
			bleOtx,
			mcuf0Enc,
			mcuf0Otx,
			bcmEnc,
			bcmOtx,
			gtwEnc,
			gtwOtx,
			dscuEnc,
			dscuOtx,
			acEnc,
			acOtx,
			tBoxOtx,
			vcuEnc,
			vcuOtx,
		},
		expectBackup: false,
	}

	runFixContextDBViaCmd(t, cfg)
}

func TestFixContextDB_when_already_fixed(t *testing.T) {
	cfg := fixContextDBCmdRunConfig{
		originTaskFileName:   "modified_without_ivi_mpu_ivi_mcu.txt",
		modifiedTaskFileName: "modified_without_ivi_mpu_ivi_mcu.txt",
		filesToCreate: []string{
			bmsEnc,
			bmsOtx,
			bleEnc,
			bleOtx,
			mcuf0Enc,
			mcuf0Otx,
			bcmEnc,
			bcmOtx,
			gtwEnc,
			gtwOtx,
			dscuEnc,
			dscuOtx,
			acEnc,
			acOtx,
			tBoxEnc,
			tBoxOtx,
			vcuEnc,
			vcuOtx,
		},
		expectBackup: false,
	}

	runFixContextDBViaCmd(t, cfg)
}

func TestFixContextDB_WhenFlashFailOnlyIVIOrTBoxMissing(t *testing.T) {
	cfg := fixContextDBCmdRunConfig{
		originTaskFileName:   "origin_flash_fail.txt",
		modifiedTaskFileName: "modified_when_ivi_flash_fail.txt",
		filesToCreate: []string{
			remoteContextJSONPath,
			tBoxEnc2,
			tBoxOtx,
		},
		expectBackup:        true,
		expectFlashFailure:  true,
		expectedRemovedECUs: []string{"IVI_MPU"},
		expectedTarget:      "6.6.1",
	}

	runFixContextDBViaCmd(t, cfg)
}

func TestFixContextDB_WhenIdleAndIVIMissing(t *testing.T) {
	cfg := fixContextDBCmdRunConfig{
		originTaskFileName:   "origin_idle.txt",
		modifiedTaskFileName: "modified_when_idle.txt",
		filesToCreate: []string{
			remoteContextJSONPath,
			bmsEnc,
			bmsOtx,
			bleEnc,
			bleOtx,
			mcuf0Enc,
			mcuf0Otx,
			bcmEnc,
			bcmOtx,
			gtwEnc,
			gtwOtx,
			dscuEnc,
			dscuOtx,
			acEnc,
			acOtx,
			tBoxEnc,
			tBoxOtx,
			vcuEnc,
			vcuOtx,
		},
		expectBackup:        true,
		expectFlashFailure:  true,
		expectedRemovedECUs: []string{"IVI_MCU", "IVI_MPU"},
		expectedTarget:      "6.5.4",
	}

	runFixContextDBViaCmd(t, cfg)
}

func TestFixContextDBViaSCP_WithoutIVI_MPU_IVI_MCU(t *testing.T) {
	cfg := fixContextDBCmdRunConfig{
		originTaskFileName:   "origin.txt",
		modifiedTaskFileName: "modified_without_ivi_mpu_ivi_mcu.txt",
		filesToCreate: []string{
			remoteContextJSONPath,
			bmsEnc,
			bmsOtx,
			bleEnc,
			bleOtx,
			mcuf0Enc,
			mcuf0Otx,
			bcmEnc,
			bcmOtx,
			gtwEnc,
			gtwOtx,
			dscuEnc,
			dscuOtx,
			acEnc,
			acOtx,
			tBoxEnc,
			tBoxOtx,
			vcuEnc,
			vcuOtx,
		},
		expectBackup: true,
		disableSFTP:  true,
	}

	runFixContextDBViaCmd(t, cfg)
}

func TestFixContextDBViaSCP_WithoutIVI_MPU_IVI_MCU_and_when_tbox_files_not_exist(t *testing.T) {
	cfg := fixContextDBCmdRunConfig{
		originTaskFileName:   "origin.txt",
		modifiedTaskFileName: "modified_without_ivi_mpu_ivi_mcu_tbox.txt",
		filesToCreate: []string{
			bmsEnc,
			bmsOtx,
			bleEnc,
			bleOtx,
			mcuf0Enc,
			mcuf0Otx,
			bcmEnc,
			bcmOtx,
			gtwEnc,
			gtwOtx,
			dscuEnc,
			dscuOtx,
			acEnc,
			acOtx,
			tBoxOtx,
			vcuEnc,
			vcuOtx,
		},
		expectBackup: true,
		disableSFTP:  true,
	}

	runFixContextDBViaCmd(t, cfg)
}

func TestFixContextDBViaSCP_WhenFlashFailOnlyIVIOrTBoxMissing(t *testing.T) {
	cfg := fixContextDBCmdRunConfig{
		originTaskFileName:   "origin_flash_fail.txt",
		modifiedTaskFileName: "modified_when_ivi_flash_fail.txt",
		filesToCreate: []string{
			remoteContextJSONPath,
			tBoxEnc2,
			tBoxOtx,
		},
		expectBackup:        true,
		disableSFTP:         true,
		expectFlashFailure:  true,
		expectedRemovedECUs: []string{"IVI_MPU"},
		expectedTarget:      "6.6.1",
	}

	runFixContextDBViaCmd(t, cfg)
}

func setupTestDirs(t *testing.T) testDirs {
	t.Helper()

	if !dockerAvailable() {
		t.Fatal("docker is not available")
	}

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to detect current file path")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), ".."))
	testsDir := filepath.Join(repoRoot, "tests")
	dockerDir := filepath.Join(testsDir, "docker")
	fixturesDir := filepath.Join(testsDir, "fixtures")
	composeFile := filepath.Join(dockerDir, "docker-compose.yml")
	tmpDir := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to switch working dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWD) })

	return testDirs{
		repoRoot:    repoRoot,
		testsDir:    testsDir,
		dockerDir:   dockerDir,
		fixturesDir: fixturesDir,
		composeFile: composeFile,
		tmpDir:      tmpDir,
	}
}

type packageInfo struct {
	File            string `json:"file"`
	UpgradeSpecFile string `json:"upgrade_spec_file"`
}

type taskDataPayload struct {
	PackagesInfo []packageInfo `json:"packages_info"`
}

func runFixContextDBViaCmd(
	t *testing.T,
	cfg fixContextDBCmdRunConfig,
) {
	t.Helper()

	dirs := setupTestDirs(t)

	originTaskData := readTextFile(t, filepath.Join(dirs.fixturesDir, cfg.originTaskFileName))
	modifiedTaskData := ""
	if cfg.modifiedTaskFileName != "" {
		modifiedTaskData = readTextFile(t, filepath.Join(dirs.fixturesDir, cfg.modifiedTaskFileName))
	}
	expectedTaskData := ""
	if cfg.expectedTaskFileName != "" {
		expectedTaskData = readTextFile(t, filepath.Join(dirs.fixturesDir, cfg.expectedTaskFileName))
	}

	if cfg.disableSFTP {
		t.Setenv("DISABLE_SFTP", "1")
	}

	runCmd(t, dirs.repoRoot, "docker", "compose", "-f", dirs.composeFile, "up", "-d", "--build")
	t.Cleanup(func() {
		runCmdNoFail(dirs.repoRoot, "docker", "compose", "-f", dirs.composeFile, "down", "-v", "--remove-orphans")
	})
	waitPort(t, "127.0.0.1:2222", 60*time.Second)

	binaryPath := filepath.Join(dirs.tmpDir, "voyah-free-update-fix")
	runCmd(t, dirs.repoRoot, "go", "build", "-o", binaryPath, "./cmd/voyah-free-update-fix")

	preparedDB := filepath.Join(dirs.tmpDir, "prepared_context.db")
	copyFile(t, filepath.Join(dirs.fixturesDir, "context.db"), preparedDB)
	writeTaskDataToDB(t, preparedDB, originTaskData)
	dockerCopyToContainer(t, preparedDB, containerName+":"+remoteDBPath)
	createFilesInContainer(t, cfg.filesToCreate)

	t.Setenv("REBOOT_TBOX_COMMAND", "true")
	backupDir := filepath.Join(dirs.tmpDir, "backup")

	cmd := exec.Command(
		binaryPath,
		"--ip", "127.0.0.1",
		"--port", "2222",
		"--username", "root",
		"--password", "12345",
		"--backup-dir", backupDir,
	)
	cmd.Dir = dirs.tmpDir
	cmd.Stdin = strings.NewReader("start\n")

	fixStartedAt := time.Now()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cmd failed:\n%s\nerror: %v", string(out), err)
	}
	if cfg.disableSFTP && !strings.Contains(string(out), "SFTP unavailable, using SCP fallback.") {
		t.Fatalf("expected SCP fallback message in output, got:\n%s", string(out))
	}

	resultDB := filepath.Join(dirs.tmpDir, "context_after_fix.db")
	dockerCopyFromContainer(t, containerName+":"+remoteDBPath, resultDB)
	gotTaskData := readTaskDataFromDB(t, resultDB)
	if cfg.expectFlashFailure {
		assertFlashFailureTaskData(t, originTaskData, expectedTaskData, gotTaskData, cfg, fixStartedAt)
	} else if gotTaskData != strings.TrimSpace(modifiedTaskData) {
		t.Fatalf("unexpected taskData after cmd run: %s", rawDiffSummary(strings.TrimSpace(modifiedTaskData), gotTaskData))
	}

	if cfg.expectBackup {
		assertRemoteFileNotExists(t, remoteContextJSONPath)
		verifyBackupArchiveAndCleanup(t, backupDir, originTaskData)

		return
	}

	if err := os.RemoveAll(backupDir); err != nil {
		t.Fatalf("failed to cleanup backup dir %s: %v", backupDir, err)
	}
}

//nolint:funlen,gocognit
func assertFlashFailureTaskData(
	t *testing.T,
	originTaskData string,
	expectedTaskData string,
	gotTaskData string,
	cfg fixContextDBCmdRunConfig,
	fixStartedAt time.Time,
) {
	t.Helper()

	var expected map[string]any
	if expectedTaskData == "" {
		expectedTaskData = originTaskData
	}
	if err := json.Unmarshal([]byte(expectedTaskData), &expected); err != nil {
		t.Fatalf("failed to parse expected taskData: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(gotTaskData), &got); err != nil {
		t.Fatalf("failed to parse fixed taskData: %v", err)
	}

	if cfg.expectedTaskFileName == "" {
		removedECUs := make(map[string]struct{}, len(cfg.expectedRemovedECUs))
		for _, ecu := range cfg.expectedRemovedECUs {
			removedECUs[ecu] = struct{}{}
		}

		expected["packages_info"] = filterECUObjects(t, expected["packages_info"], removedECUs)
		expected["ecu_rollback_versions"] = filterECUObjects(t, expected["ecu_rollback_versions"], removedECUs)
		delete(expected, "flash_state")
		expected["overall_state"] = map[string]any{"stage": "Schedule", "state": "Process"}
		expected["schedule_state"] = map[string]any{"set_time": float64(0), "stage": "Wait Set Time"}
		expected["target_baseline_version"] = cfg.expectedTarget
	}

	gotExpireTime, ok := got["expire_time"].(float64)
	if !ok {
		t.Fatalf("expire_time has unexpected type: %T", got["expire_time"])
	}

	wantExpireTime := fixStartedAt.AddDate(0, 0, 60).Unix()
	if delta := int64(gotExpireTime) - wantExpireTime; delta < 0 || delta > 30 {
		t.Fatalf("unexpected expire_time: got %d, want approximately %d", int64(gotExpireTime), wantExpireTime)
	}

	expected["expire_time"] = gotExpireTime
	if !reflect.DeepEqual(got, expected) {
		expectedJSON, _ := json.Marshal(expected)
		gotJSON, _ := json.Marshal(got)
		t.Fatalf("unexpected taskData after FlashFailure fix: %s", rawDiffSummary(string(expectedJSON), string(gotJSON)))
	}
}

func filterECUObjects(t *testing.T, value any, removedECUs map[string]struct{}) []any {
	t.Helper()

	items, ok := value.([]any)
	if !ok {
		t.Fatalf("ECU collection has unexpected type: %T", value)
	}

	filtered := make([]any, 0, len(items))
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("ECU collection item has unexpected type: %T", item)
		}

		ecu, ok := object["ecu"].(string)
		if !ok {
			t.Fatalf("ECU collection item has invalid ecu: %T", object["ecu"])
		}

		if _, remove := removedECUs[ecu]; !remove {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

func verifyBackupArchiveAndCleanup(t *testing.T, backupDir, originTaskData string) {
	t.Helper()

	archives, err := filepath.Glob(filepath.Join(backupDir, "context.backup.*.zip"))
	if err != nil {
		t.Fatalf("failed to list backup archives: %v", err)
	}
	if len(archives) != 1 {
		t.Fatalf("expected exactly one backup archive, got %d", len(archives))
	}

	archivePath := archives[0]
	extractDir := filepath.Join(backupDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatalf("failed to create extract dir: %v", err)
	}

	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("failed to open backup archive %s: %v", archivePath, err)
	}
	defer zr.Close()

	extractedDBPath := filepath.Join(extractDir, "context.db")
	foundDB := false

	for _, f := range zr.File {
		if f.Name != "context.db" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to open zip entry %s: %v", f.Name, err)
		}

		out, err := os.Create(extractedDBPath)
		if err != nil {
			_ = rc.Close()
			t.Fatalf("failed to create extracted db: %v", err)
		}

		if _, err := io.Copy(out, rc); err != nil {
			_ = out.Close()
			_ = rc.Close()
			t.Fatalf("failed to extract context.db: %v", err)
		}
		if err := out.Close(); err != nil {
			_ = rc.Close()
			t.Fatalf("failed to close extracted db: %v", err)
		}
		if err := rc.Close(); err != nil {
			t.Fatalf("failed to close zip entry context.db: %v", err)
		}

		foundDB = true
		break
	}

	if !foundDB {
		t.Fatalf("context.db not found in backup archive")
	}

	gotOriginTaskData := readTaskDataFromDB(t, extractedDBPath)
	if gotOriginTaskData != strings.TrimSpace(originTaskData) {
		t.Fatalf("unexpected taskData in backup archive: %s", rawDiffSummary(strings.TrimSpace(originTaskData), gotOriginTaskData))
	}

	if err := os.RemoveAll(backupDir); err != nil {
		t.Fatalf("failed to cleanup backup dir %s: %v", backupDir, err)
	}
}

func createFilesInContainer(t *testing.T, filesToCreate []string) {
	t.Helper()

	for _, file := range filesToCreate {
		if strings.TrimSpace(file) == "" {
			continue
		}
		dockerExec(t, "mkdir -p "+shellQuote(filepath.Dir(file))+" && printf x > "+shellQuote(file))
	}
}

func assertRemoteFileNotExists(t *testing.T, remotePath string) {
	t.Helper()
	dockerExec(t, "test ! -f "+shellQuote(remotePath))
}

func writeTaskDataToDB(t *testing.T, dbPath, taskData string) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db %s: %v", dbPath, err)
	}
	defer db.Close()

	res, err := db.Exec(`UPDATE ota_task SET taskData = ? WHERE type = 'task_info'`, taskData)
	if err != nil {
		t.Fatalf("failed to update taskData: %v", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		t.Fatalf("failed to read rows affected: %v", err)
	}
	if rows == 0 {
		t.Fatalf("no rows updated in ota_task")
	}
}

func readTaskDataFromDB(t *testing.T, dbPath string) string {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db %s: %v", dbPath, err)
	}
	defer db.Close()

	var taskData string
	if err := db.QueryRow(`SELECT taskData FROM ota_task WHERE type = 'task_info'`).Scan(&taskData); err != nil {
		t.Fatalf("failed to query taskData: %v", err)
	}
	return strings.TrimSpace(taskData)
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("failed to read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", dst, err)
	}
}

func readTextFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return strings.TrimSpace(string(data))
}

func rawDiffSummary(want, got string) string {
	const contextBytes = 80

	maxLen := len(want)
	if len(got) < maxLen {
		maxLen = len(got)
	}

	diffAt := maxLen
	for i := range maxLen {
		if want[i] != got[i] {
			diffAt = i
			break
		}
	}

	start := diffAt - contextBytes
	if start < 0 {
		start = 0
	}

	wantEnd := diffAt + contextBytes
	if wantEnd > len(want) {
		wantEnd = len(want)
	}

	gotEnd := diffAt + contextBytes
	if gotEnd > len(got) {
		gotEnd = len(got)
	}

	return "len(want)=" + strconv.Itoa(len(want)) +
		" len(got)=" + strconv.Itoa(len(got)) +
		" diff_at=" + strconv.Itoa(diffAt) +
		" want_near=" + strconv.Quote(want[start:wantEnd]) +
		" got_near=" + strconv.Quote(got[start:gotEnd])
}

func dockerAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "version")
	return cmd.Run() == nil
}

func waitPort(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			_ = conn.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("port %s is not available after %s", addr, timeout)
		}
		time.Sleep(time.Second)
	}
}

func dockerExec(t *testing.T, command string) {
	t.Helper()
	runCmd(t, "", "docker", "exec", containerName, "sh", "-lc", command)
}

func dockerCopyToContainer(t *testing.T, src, dst string) {
	t.Helper()
	runCmd(t, "", "docker", "cp", src, dst)
}

func dockerCopyFromContainer(t *testing.T, src, dst string) {
	t.Helper()
	runCmd(t, "", "docker", "cp", src, dst)
}

func runCmd(t *testing.T, workdir string, name string, args ...string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	if workdir != "" {
		cmd.Dir = workdir
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %s %s\n%s\nerror: %v", name, strings.Join(args, " "), string(out), err)
	}
}

func runCmdNoFail(workdir string, name string, args ...string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	if workdir != "" {
		cmd.Dir = workdir
	}

	_, _ = cmd.CombinedOutput()
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
