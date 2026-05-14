package app

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

type scpFileClient struct {
	sshClient *ssh.Client
}

func (c *scpFileClient) Close() error {
	return nil
}

func (c *scpFileClient) ping() error {
	_, err := c.runCommand("command -v scp >/dev/null 2>&1")
	if err != nil {
		return fmt.Errorf("scp is unavailable on T-Box: %w", err)
	}

	return nil
}

func (c *scpFileClient) Exists(filePath string) bool {
	if filePath == "" {
		return false
	}

	_, err := c.runCommand("test -e " + shellQuote(filePath))

	return err == nil
}

func (c *scpFileClient) ExistsAndNotEmpty(filePath string) bool {
	if filePath == "" {
		return false
	}

	_, err := c.runCommand("test -s " + shellQuote(filePath))

	return err == nil
}

func (c *scpFileClient) Download(remotePath, localPath string) error {
	dst, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("error creating local file %s: %w", localPath, err)
	}
	defer dst.Close()

	if err := c.downloadWithSCP(remotePath, dst); err != nil {
		return err
	}

	if err := dst.Sync(); err != nil {
		return fmt.Errorf("error syncing local file %s: %w", localPath, err)
	}

	return nil
}

func (c *scpFileClient) Upload(localPath, remotePath string) error {
	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("error opening local file %s: %w", localPath, err)
	}
	defer src.Close()

	srcInfo, err := src.Stat()
	if err != nil {
		return fmt.Errorf("error getting stats for local file %s: %w", localPath, err)
	}

	if err := c.uploadWithSCP(remotePath, src, srcInfo.Size()); err != nil {
		return err
	}

	return nil
}

func (c *scpFileClient) Remove(filePath string) error {
	if _, err := c.runCommand("rm -f " + shellQuote(filePath)); err != nil {
		return fmt.Errorf("error deleting file %s on T-Box: %w", filePath, err)
	}

	return nil
}

//nolint:unparam
func (c *scpFileClient) runCommand(cmd string) ([]byte, error) {
	session, err := c.sshClient.NewSession()
	if err != nil {
		return nil, fmt.Errorf("error creating SSH session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		trimmedOutput := strings.TrimSpace(string(output))
		if trimmedOutput == "" {
			return nil, err
		}

		return nil, fmt.Errorf("%w: %s", err, trimmedOutput)
	}

	return output, nil
}

//nolint:cyclop,funlen
func (c *scpFileClient) downloadWithSCP(remotePath string, dst io.Writer) error {
	session, err := c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("error creating SSH session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("error creating stdin pipe for scp: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe for scp: %w", err)
	}

	var stderr bytes.Buffer

	session.Stderr = &stderr

	if err := session.Start("scp -f " + shellQuote(remotePath)); err != nil {
		return fmt.Errorf("error starting scp download command: %w", err)
	}

	reader := bufio.NewReader(stdout)

	if _, err := stdin.Write([]byte{0}); err != nil {
		return fmt.Errorf("error sending initial scp ack: %w", err)
	}

	fileSize, err := readSCPHeader(reader)
	if err != nil {
		return fmt.Errorf("error reading scp header: %w", err)
	}

	if _, err := stdin.Write([]byte{0}); err != nil {
		return fmt.Errorf("error confirming scp header: %w", err)
	}

	if _, err := io.CopyN(dst, reader, fileSize); err != nil {
		return fmt.Errorf("error reading file data via scp: %w", err)
	}

	if err := readSCPAck(reader); err != nil {
		return fmt.Errorf("error waiting final scp ack: %w", err)
	}

	if _, err := stdin.Write([]byte{0}); err != nil {
		return fmt.Errorf("error sending final scp ack: %w", err)
	}

	if err := stdin.Close(); err != nil {
		return fmt.Errorf("error closing scp stdin: %w", err)
	}

	if err := session.Wait(); err != nil {
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText != "" {
			return fmt.Errorf("scp download failed: %w: %s", err, stderrText)
		}

		return fmt.Errorf("scp download failed: %w", err)
	}

	return nil
}

//nolint:cyclop,funlen
func (c *scpFileClient) uploadWithSCP(remotePath string, src io.Reader, size int64) error {
	session, err := c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("error creating SSH session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("error creating stdin pipe for scp: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe for scp: %w", err)
	}

	var stderr bytes.Buffer

	session.Stderr = &stderr

	if err := session.Start("scp -t " + shellQuote(remotePath)); err != nil {
		return fmt.Errorf("error starting scp upload command: %w", err)
	}

	reader := bufio.NewReader(stdout)
	if err := readSCPAck(reader); err != nil {
		return fmt.Errorf("remote scp is not ready to receive file: %w", err)
	}

	if _, err := fmt.Fprintf(stdin, "C0644 %d %s\n", size, path.Base(remotePath)); err != nil {
		return fmt.Errorf("error sending scp file header: %w", err)
	}

	if err := readSCPAck(reader); err != nil {
		return fmt.Errorf("remote scp rejected file header: %w", err)
	}

	if _, err := io.Copy(stdin, src); err != nil {
		return fmt.Errorf("error sending file data via scp: %w", err)
	}

	if _, err := stdin.Write([]byte{0}); err != nil {
		return fmt.Errorf("error sending file end marker via scp: %w", err)
	}

	if err := readSCPAck(reader); err != nil {
		return fmt.Errorf("remote scp rejected uploaded data: %w", err)
	}

	if err := stdin.Close(); err != nil {
		return fmt.Errorf("error closing scp stdin: %w", err)
	}

	if err := session.Wait(); err != nil {
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText != "" {
			return fmt.Errorf("scp upload failed: %w: %s", err, stderrText)
		}

		return fmt.Errorf("scp upload failed: %w", err)
	}

	return nil
}

//nolint:cyclop,mnd
func readSCPHeader(reader *bufio.Reader) (int64, error) {
	firstByte, err := reader.ReadByte()
	if err != nil {
		return 0, err
	}

	switch firstByte {
	case 0x01, 0x02:
		message, readErr := reader.ReadString('\n')
		if readErr != nil {
			return 0, readErr
		}

		return 0, errors.New(strings.TrimSpace(message))
	case 'C':
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			return 0, readErr
		}

		parts := strings.Fields(strings.TrimSpace(line))
		if len(parts) != 3 {
			return 0, fmt.Errorf("unexpected scp header format: %q", line)
		}

		if !isValidSCPFileMode(parts[0]) {
			return 0, fmt.Errorf("invalid scp file mode in header %q", line)
		}

		size, parseErr := strconv.ParseInt(parts[1], 10, 64)
		if parseErr != nil {
			return 0, fmt.Errorf("invalid scp file size in header %q: %w", line, parseErr)
		}

		if size < 0 {
			return 0, fmt.Errorf("invalid negative scp file size in header %q", line)
		}

		if strings.TrimSpace(parts[2]) == "" {
			return 0, fmt.Errorf("empty scp file name in header %q", line)
		}

		return size, nil
	default:
		return 0, fmt.Errorf("unexpected scp header prefix: %q", firstByte)
	}
}

//nolint:mnd
func isValidSCPFileMode(mode string) bool {
	if len(mode) != 4 {
		return false
	}

	for _, r := range mode {
		if r < '0' || r > '7' {
			return false
		}
	}

	return true
}

//nolint:mnd
func readSCPAck(reader *bufio.Reader) error {
	ack, err := reader.ReadByte()
	if err != nil {
		return err
	}

	switch ack {
	case 0:
		return nil
	case 0x01, 0x02:
		message, readErr := reader.ReadString('\n')
		if readErr != nil {
			return readErr
		}

		trimmedMessage := strings.TrimSpace(message)
		if trimmedMessage == "" {
			return errors.New("unknown scp error")
		}

		return errors.New(trimmedMessage)
	default:
		return fmt.Errorf("unexpected scp ack value: %d", ack)
	}
}

func shellQuote(input string) string {
	if input == "" {
		return "''"
	}

	return "'" + strings.ReplaceAll(input, "'", "'\\''") + "'"
}
