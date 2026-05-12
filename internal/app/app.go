package app

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

var (
	ipAddress = "172.16.104.20"
	sshPort   = "22"
	username  = "root"
	password  = "12345"
	rebootCmd = "reboot"
	backupDir = "backup"
)

type RuntimeConfig struct {
	IPAddress string
	SSHPort   string
	Username  string
	Password  string
	RebootCmd string
	BackupDir string
}

const (
	tboxDir                 = "/mnt/ota/data/fota"
	contextDBName           = "context.db"
	contextJSONName         = "context.json"
	localContextDBTmpFile   = "context.db.tmp"
	localContextJSONTmpFile = "context.json.tmp"
	rebootCmdEnvVar         = "REBOOT_TBOX_COMMAND"

	iviMCUName = "IVI_MCU"
	iviMPUName = "IVI_MPU"
	tboxName   = "T-BOX"

	colorReset = "\033[0m"
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
)

func Run() error {
	return RunWithConfig(DefaultRuntimeConfig())
}

func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		IPAddress: ipAddress,
		SSHPort:   sshPort,
		Username:  username,
		Password:  password,
		RebootCmd: rebootCmd,
		BackupDir: backupDir,
	}
}

func RunWithConfig(cfg RuntimeConfig) error {
	applyRuntimeConfig(cfg)

	sshClient, err := connectToTBox(net.JoinHostPort(ipAddress, sshPort), username, password)
	if err != nil {
		return fmt.Errorf("t-box connection error: %w", err)
	}
	defer sshClient.Close()

	remoteFS, err := newTBoxFileClient(sshClient)
	if err != nil {
		return fmt.Errorf("file transport error: %w", err)
	}
	defer remoteFS.Close()

	acceptToStartFix, missingPackageIDs, err := analyzeContextDB(remoteFS)
	if err != nil {
		return fmt.Errorf("error analyzing context.db: %w", err)
	}

	if !acceptToStartFix {
		return nil
	}

	if !askConfirmation() {
		return nil
	}

	if err := fixContextDB(sshClient, remoteFS, missingPackageIDs); err != nil {
		return fmt.Errorf("error fixing context.db: %w", err)
	}

	fmt.Printf(
		"%sFile context.db has been fixed and uploaded successfully to T-Box!%s\n",
		colorGreen,
		colorReset,
	)

	return nil
}

func applyRuntimeConfig(cfg RuntimeConfig) {
	if cfgIPAddress := strings.TrimSpace(cfg.IPAddress); cfgIPAddress != "" {
		ipAddress = cfg.IPAddress
	}

	if cfgSSHPort := strings.TrimSpace(cfg.SSHPort); cfgSSHPort != "" {
		sshPort = cfg.SSHPort
	}

	if cfgUsername := strings.TrimSpace(cfg.Username); cfgUsername != "" {
		username = cfg.Username
	}

	if cfgPassword := strings.TrimSpace(cfg.Password); cfgPassword != "" {
		password = cfg.Password
	}

	if cfgRebootCmd := strings.TrimSpace(cfg.RebootCmd); cfgRebootCmd != "" {
		rebootCmd = cfg.RebootCmd
	}

	if cfgBackupDir := strings.TrimSpace(cfg.BackupDir); cfgBackupDir != "" {
		backupDir = cfg.BackupDir
	}
}

func analyzeContextDB(client tboxFileClient) (bool, []int, error) {
	err := downloadContextDB(client)
	if err != nil {
		return false, nil, fmt.Errorf("error downloading context.db: %w", err)
	}

	defer os.Remove(localContextDBTmpFile)

	dbConn, err := connectToDB()
	if err != nil {
		return false, nil, err
	}
	defer dbConn.Close()

	taskDataJSON, err := getOTATaskData(dbConn)
	if err != nil {
		return false, nil, err
	}

	taskData, err := parseOTATaskData(taskDataJSON)
	if err != nil {
		return false, nil, err
	}

	rows, stats := buildPackageRowsAndStats(taskData.PackagesInfo, client)

	printTaskDataInfo(taskData, rows, stats)

	fixRequired, missingPackageIDs := checkFixContextDB(rows)
	if !fixRequired {
		return false, nil, nil
	}

	if !isFixStateAllowed(taskData) {
		fmt.Println()
		fmt.Printf(
			"%sThe context.db fix can be started only in one of states:%s\n",
			colorRed,
			colorReset,
		)
		fmt.Println("  - download_state.stage='Retrive Packages' and packages IVI_MCU, IVI_MPU or T-Box are not downloaded")
		fmt.Println("  - download_state.stage='Complete' and flash failure of IVI_MCU, IVI_MPU or T-Box")
		fmt.Println("  - download_state.stage='Complete', overall_state.stage='Terminate', overall_state.state='Idle'")
		fmt.Println("      and packages IVI_MCU, IVI_MPU or T-Box package files are missing")

		return false, nil, nil
	}

	return true, missingPackageIDs, nil
}

//nolint:cyclop
func fixContextDB(sshClient *ssh.Client, client tboxFileClient, missingPackageIDs []int) error {
	fmt.Println()
	fmt.Println("Start fixing context.db...")

	err := downloadContextDB(client)
	if err != nil {
		return fmt.Errorf("error downloading context.db: %w", err)
	}

	defer os.Remove(localContextDBTmpFile)

	existContextJSON := true

	err = downloadContextJSON(client)
	if err != nil {
		if !errors.Is(err, errContextJSONNotFound) {
			return fmt.Errorf("error downloading context.json: %w", err)
		}

		existContextJSON = false
	} else {
		defer os.Remove(localContextJSONTmpFile)
	}

	if err := backup(existContextJSON); err != nil {
		return fmt.Errorf("error backing up context: %w", err)
	}

	dbConn, err := connectToDB()
	if err != nil {
		return err
	}

	if err := removeUndownloadedPackagesInDB(dbConn, missingPackageIDs); err != nil {
		return err
	}

	dbConn.Close()

	if existContextJSON {
		if err := deleteContextJSONOnTBox(client); err != nil {
			return fmt.Errorf("error deleting context.json on T-Box: %w", err)
		}

		fmt.Println("Context.json deleted from T-Box.")
	}

	if err := uploadContextDB(client); err != nil {
		return fmt.Errorf("error uploading fixed context.db: %w", err)
	}

	fmt.Println("Fixed context.db uploaded to T-Box.")

	if err := rebootTBox(sshClient); err != nil {
		return fmt.Errorf("error rebooting T-Box: %w", err)
	}

	fmt.Println("T-Box is rebooting...")

	return nil
}

func askConfirmation() bool {
	fmt.Print("\nAre you sure you want to start fix context.db in T-Box? Type 'start' to confirm: ")

	reader := bufio.NewReader(os.Stdin)

	confirmation, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("failed to read confirmation: %v\n", err)

		return false
	}

	if strings.TrimSpace(confirmation) != "start" {
		fmt.Printf("%sInvalid confirmation. Fix was not started.%s\n", colorRed, colorReset)

		return false
	}

	return true
}
