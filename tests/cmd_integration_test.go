//go:build integration

package integration

import (
	"archive/zip"
	"context"
	"database/sql"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	filesToCreate        []string
	expectBackup         bool
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
	modifiedTaskData := readTextFile(t, filepath.Join(dirs.fixturesDir, cfg.modifiedTaskFileName))

	runCmd(t, dirs.repoRoot, "docker", "compose", "-f", dirs.composeFile, "up", "-d", "--build")
	t.Cleanup(func() {
		runCmdNoFail(dirs.repoRoot, "docker", "compose", "-f", dirs.composeFile, "down", "-v", "--remove-orphans")
	})
	waitPort(t, "127.0.0.1:2222", 60*time.Second)

	binaryPath := filepath.Join(dirs.tmpDir, "voyah-update-fix")
	runCmd(t, dirs.repoRoot, "go", "build", "-o", binaryPath, "./cmd/voyah-update-fix")

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

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cmd failed:\n%s\nerror: %v", string(out), err)
	}

	resultDB := filepath.Join(dirs.tmpDir, "context_after_fix.db")
	dockerCopyFromContainer(t, containerName+":"+remoteDBPath, resultDB)
	gotTaskData := readTaskDataFromDB(t, resultDB)
	if gotTaskData != strings.TrimSpace(modifiedTaskData) {
		t.Fatalf("unexpected taskData after cmd run")
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
		t.Fatalf("unexpected taskData in backup archive")
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
		dockerExec(t, "mkdir -p "+shellQuote(filepath.Dir(file))+" && touch "+shellQuote(file))
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
