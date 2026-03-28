package app

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	sshConnectTimeout = 10 * time.Second
	backupDirMode     = 0o755
)

var errContextJSONNotFound = errors.New("context.json not found on tbox")

//nolint:gosec
func connectToTBox(addr, user, password string) (*ssh.Client, error) {
	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         sshConnectTimeout,
	}

	return ssh.Dial("tcp", addr, sshConfig)
}

func contextDBJournalExists(sftpClient *sftp.Client, contextDBPath string) bool {
	// SQLite может использовать:
	// - rollback journal: context.db-journal
	// - WAL: context.db-wal + context.db-shm
	journalFiles := []string{
		contextDBPath + "-journal",
		contextDBPath + "-wal",
		contextDBPath + "-shm",
	}

	for _, p := range journalFiles {
		if ok := fileExistsOnTBox(sftpClient, p); ok {
			return true
		}
	}

	return false
}

func downloadContextDB(sftpClient *sftp.Client) error {
	contextDBPath := path.Join(tboxDir, contextDBName)

	if ok := fileExistsOnTBox(sftpClient, contextDBPath); !ok {
		return fmt.Errorf("file %s not found", contextDBPath)
	}

	var journalExists bool

	for range 60 {
		journalExists = contextDBJournalExists(sftpClient, contextDBPath)
		if !journalExists {
			break
		}

		time.Sleep(1 * time.Second)
	}

	if journalExists {
		return errors.New("db journal exists, file is busy! Try again later")
	}

	src, err := sftpClient.Open(contextDBPath)
	if err != nil {
		return fmt.Errorf("error opening file %s on T-Box: %w", contextDBPath, err)
	}
	defer src.Close()

	dst, err := os.Create(localContextDBTmpFile)
	if err != nil {
		return fmt.Errorf("error creating local file %s: %w", localContextDBTmpFile, err)
	}

	defer func() {
		_ = dst.Close()
	}()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("error copying T-Box->local %s: %w", localContextDBTmpFile, err)
	}

	if err := dst.Sync(); err != nil {
		return fmt.Errorf("error syncing local file %s: %w", localContextDBTmpFile, err)
	}

	return nil
}

func downloadContextJSON(sftpClient *sftp.Client) error {
	contextJSONPath := path.Join(tboxDir, contextJSONName)

	if ok := fileExistsOnTBox(sftpClient, contextJSONPath); !ok {
		return errContextJSONNotFound
	}

	src, err := sftpClient.Open(contextJSONPath)
	if err != nil {
		return fmt.Errorf("error opening remote file %s on T-Box: %w", contextJSONPath, err)
	}
	defer src.Close()

	dst, err := os.Create(localContextJSONTmpFile)
	if err != nil {
		return fmt.Errorf("error creating local file %s: %w", localContextJSONTmpFile, err)
	}

	defer func() {
		_ = dst.Close()
	}()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("error copying T-Box->local %s: %w", localContextJSONTmpFile, err)
	}

	if err := dst.Sync(); err != nil {
		return fmt.Errorf("error syncing local file %s: %w", localContextJSONTmpFile, err)
	}

	return nil
}

//nolint:staticcheck
func fileExistsOnTBox(client *sftp.Client, filePath string) bool {
	if filePath == "" {
		return false
	}

	_, err := client.Stat(filePath)
	if err == nil {
		return true
	}

	return false
}

func deleteContextJSONOnTBox(client *sftp.Client) error {
	contextJSON := path.Join(tboxDir, contextJSONName)

	if err := client.Remove(contextJSON); err != nil {
		return fmt.Errorf("error deleting file %s on T-Box: %w", contextJSON, err)
	}

	return nil
}

func uploadContextDB(sftpClient *sftp.Client) error {
	contextDBPath := path.Join(tboxDir, contextDBName)

	var journalExists bool
	for range 60 {
		journalExists = contextDBJournalExists(sftpClient, contextDBPath)
		if !journalExists {
			break
		}

		time.Sleep(1 * time.Second)
	}

	if journalExists {
		return errors.New("db journal exists, file is busy! Try again later")
	}

	src, err := os.Open(localContextDBTmpFile)
	if err != nil {
		return fmt.Errorf("error opening local file %s: %w", localContextDBTmpFile, err)
	}
	defer src.Close()

	dst, err := sftpClient.OpenFile(contextDBPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("error opening file %s on T-Box for overwrite: %w", contextDBPath, err)
	}

	defer func() {
		_ = dst.Close()
	}()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("error copying local->T-Box %s: %w", contextDBPath, err)
	}

	if err := dst.Sync(); err != nil {
		return fmt.Errorf("error syncing file %s on T-Box: %w", contextDBPath, err)
	}

	if err := dst.Close(); err != nil {
		return fmt.Errorf("error closing file %s on T-Box: %w", contextDBPath, err)
	}

	return nil
}

func backup(withContextJSON bool) error {
	if err := os.MkdirAll(backupDir, backupDirMode); err != nil {
		return fmt.Errorf("error creating backup directory %s: %w", backupDir, err)
	}

	timestamp := time.Now().Format("20060102_150405")
	archivePath := filepath.Join(backupDir, fmt.Sprintf("context.backup.%s.zip", timestamp))

	zipFile, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("error creating backup archive %s: %w", archivePath, err)
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)

	if err := addFileToZip(zw, localContextDBTmpFile, contextDBName, archivePath); err != nil {
		_ = zw.Close()

		return err
	}

	if withContextJSON {
		if err := addFileToZip(zw, localContextJSONTmpFile, contextJSONName, archivePath); err != nil {
			_ = zw.Close()

			return err
		}
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("error closing backup archive %s: %w", archivePath, err)
	}

	return nil
}

func addFileToZip(zw *zip.Writer, localPath, zipName, archivePath string) error {
	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("error opening local file %s: %w", localPath, err)
	}

	srcInfo, err := src.Stat()
	if err != nil {
		_ = src.Close()

		return fmt.Errorf("error getting stats for local file %s: %w", localPath, err)
	}

	header := &zip.FileHeader{
		Name:     zipName,
		Method:   zip.Deflate,
		Modified: srcInfo.ModTime().UTC(),
	}

	entry, err := zw.CreateHeader(header)
	if err != nil {
		_ = src.Close()

		return fmt.Errorf("error creating zip entry %s: %w", zipName, err)
	}

	if _, err := io.Copy(entry, src); err != nil {
		_ = src.Close()

		return fmt.Errorf("error writing backup archive %s: %w", archivePath, err)
	}

	if err := src.Close(); err != nil {
		return fmt.Errorf("error closing local file %s: %w", localPath, err)
	}

	return nil
}

func rebootTBox(sshClient *ssh.Client) error {
	session, err := sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("error creating SSH session: %w", err)
	}
	defer session.Close()

	cmd := rebootCmd
	if envCmd := strings.TrimSpace(os.Getenv(rebootCmdEnvVar)); envCmd != "" {
		cmd = envCmd
	}

	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("error executing reboot command: %w", err)
	}

	return nil
}
