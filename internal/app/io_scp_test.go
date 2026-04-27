package app

import (
	"bufio"
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: "''"},
		{name: "simple", input: "/tmp/file", want: "'/tmp/file'"},
		{name: "with quote", input: "a'b", want: "'a'\\''b'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := shellQuote(tt.input)
			if got != tt.want {
				t.Fatalf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestReadSCPHeader(t *testing.T) {
	t.Parallel()

	t.Run("valid C header", func(t *testing.T) {
		t.Parallel()

		reader := bufio.NewReader(strings.NewReader("C0644 42 context.db\n"))
		size, err := readSCPHeader(reader)
		if err != nil {
			t.Fatalf("readSCPHeader returned error: %v", err)
		}

		if size != 42 {
			t.Fatalf("unexpected size: got %d, want 42", size)
		}
	})

	t.Run("scp error prefix", func(t *testing.T) {
		t.Parallel()

		reader := bufio.NewReader(strings.NewReader(string([]byte{0x01}) + "permission denied\n"))
		_, err := readSCPHeader(reader)
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid size", func(t *testing.T) {
		t.Parallel()

		reader := bufio.NewReader(strings.NewReader("C0644 xyz context.db\n"))
		_, err := readSCPHeader(reader)
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "invalid scp file size") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("negative size", func(t *testing.T) {
		t.Parallel()

		reader := bufio.NewReader(strings.NewReader("C0644 -1 context.db\n"))
		_, err := readSCPHeader(reader)
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "invalid negative scp file size") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid mode", func(t *testing.T) {
		t.Parallel()

		reader := bufio.NewReader(strings.NewReader("C0999 42 context.db\n"))
		_, err := readSCPHeader(reader)
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "invalid scp file mode") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("extra fields", func(t *testing.T) {
		t.Parallel()

		reader := bufio.NewReader(strings.NewReader("C0644 42 context.db extra\n"))
		_, err := readSCPHeader(reader)
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "unexpected scp header format") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("unexpected prefix", func(t *testing.T) {
		t.Parallel()

		reader := bufio.NewReader(strings.NewReader("X0644 42 context.db\n"))
		_, err := readSCPHeader(reader)
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "unexpected scp header prefix") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestReadSCPAck(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		reader := bufio.NewReader(strings.NewReader(string([]byte{0x00})))
		if err := readSCPAck(reader); err != nil {
			t.Fatalf("readSCPAck returned error: %v", err)
		}
	})

	t.Run("error with message", func(t *testing.T) {
		t.Parallel()

		reader := bufio.NewReader(strings.NewReader(string([]byte{0x02}) + "fatal\n"))
		err := readSCPAck(reader)
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "fatal") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("error without message", func(t *testing.T) {
		t.Parallel()

		reader := bufio.NewReader(strings.NewReader(string([]byte{0x01}) + "\n"))
		err := readSCPAck(reader)
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "unknown scp error") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("unexpected ack", func(t *testing.T) {
		t.Parallel()

		reader := bufio.NewReader(strings.NewReader(string([]byte{0x7F})))
		err := readSCPAck(reader)
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "unexpected scp ack value") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestSCPFileClientEmptyPathShortCircuit(t *testing.T) {
	t.Parallel()

	client := &scpFileClient{}

	if client.Exists("") {
		t.Fatal("Exists should return false for empty path")
	}

	if client.ExistsAndNotEmpty("") {
		t.Fatal("ExistsAndNotEmpty should return false for empty path")
	}
}

func TestIsValidSCPFileMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode string
		want bool
	}{
		{mode: "0644", want: true},
		{mode: "0755", want: true},
		{mode: "644", want: false},
		{mode: "0999", want: false},
		{mode: "abcd", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			t.Parallel()

			got := isValidSCPFileMode(tt.mode)
			if got != tt.want {
				t.Fatalf("isValidSCPFileMode(%q) = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}
