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

	"golang.org/x/crypto/ssh"
)

const backupDirMode = 0o755

var errContextJSONNotFound = errors.New("context.json not found on tbox")

func contextDBJournalExists(client tboxFileClient, contextDBPath string) bool {
	// SQLite может использовать:
	// - rollback journal: context.db-journal
	// - WAL: context.db-wal + context.db-shm
	journalFiles := []string{
		contextDBPath + "-journal",
		contextDBPath + "-wal",
		contextDBPath + "-shm",
	}

	for _, p := range journalFiles {
		if ok := fileExistsOnTBox(client, p); ok {
			return true
		}
	}

	return false
}

func downloadContextDB(client tboxFileClient) error {
	contextDBPath := path.Join(tboxDir, contextDBName)

	if ok := fileExistsOnTBox(client, contextDBPath); !ok {
		return fmt.Errorf("file %s not found", contextDBPath)
	}

	var journalExists bool

	for range 60 {
		journalExists = contextDBJournalExists(client, contextDBPath)
		if !journalExists {
			break
		}

		time.Sleep(1 * time.Second)
	}

	if journalExists {
		return errors.New("db journal exists, file is busy! Try again later")
	}

	if err := client.Download(contextDBPath, localContextDBTmpFile); err != nil {
		return err
	}

	return nil
}

func downloadContextJSON(client tboxFileClient) error {
	contextJSONPath := path.Join(tboxDir, contextJSONName)

	if ok := fileExistsOnTBox(client, contextJSONPath); !ok {
		return errContextJSONNotFound
	}

	if err := client.Download(contextJSONPath, localContextJSONTmpFile); err != nil {
		return err
	}

	return nil
}

func fileExistsOnTBox(client tboxFileClient, filePath string) bool {
	return client.Exists(filePath)
}

func fileExistsAndNotEmptyOnTBox(client tboxFileClient, filePath string) bool {
	return client.ExistsAndNotEmpty(filePath)
}

func fileExistsAndNotEmpty(statFn func(string) (os.FileInfo, error), filePath string) bool {
	if filePath == "" {
		return false
	}

	info, err := statFn(filePath)
	if err != nil || info == nil {
		return false
	}

	return info.Size() > 0
}

func deleteContextJSONOnTBox(client tboxFileClient) error {
	contextJSON := path.Join(tboxDir, contextJSONName)

	if err := client.Remove(contextJSON); err != nil {
		return err
	}

	return nil
}

func uploadContextDB(client tboxFileClient) error {
	contextDBPath := path.Join(tboxDir, contextDBName)

	var journalExists bool
	for range 60 {
		journalExists = contextDBJournalExists(client, contextDBPath)
		if !journalExists {
			break
		}

		time.Sleep(1 * time.Second)
	}

	if journalExists {
		return errors.New("db journal exists, file is busy! Try again later")
	}

	if err := client.Upload(localContextDBTmpFile, contextDBPath); err != nil {
		return err
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
