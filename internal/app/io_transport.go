package app

import (
	"fmt"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const sshConnectTimeout = 10 * time.Second
const scpFallbackMessage = "SFTP unavailable, using SCP fallback."

type tboxFileClient interface {
	Close() error
	Exists(filePath string) bool
	ExistsAndNotEmpty(filePath string) bool
	Download(remotePath, localPath string) error
	Upload(localPath, remotePath string) error
	Remove(filePath string) error
}

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

func newTBoxFileClient(sshClient *ssh.Client) (tboxFileClient, error) {
	sftpClient, err := sftp.NewClient(sshClient)
	if err == nil {
		return &sftpFileClient{client: sftpClient}, nil
	}

	scpClient := &scpFileClient{sshClient: sshClient}
	if pingErr := scpClient.ping(); pingErr != nil {
		return nil, fmt.Errorf("sftp init failed (%w); scp fallback failed: %w", err, pingErr)
	}

	fmt.Println(scpFallbackMessage)

	return scpClient, nil
}
