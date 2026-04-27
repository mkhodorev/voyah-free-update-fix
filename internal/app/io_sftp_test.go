package app

import "testing"

func TestSFTPFileClientEmptyPathShortCircuit(t *testing.T) {
	t.Parallel()

	client := &sftpFileClient{}

	if client.Exists("") {
		t.Fatal("Exists should return false for empty path")
	}

	if client.ExistsAndNotEmpty("") {
		t.Fatal("ExistsAndNotEmpty should return false for empty path")
	}
}
