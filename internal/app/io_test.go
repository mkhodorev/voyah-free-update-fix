package app

import (
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"
)

func TestFileExistsAndNotEmpty(t *testing.T) {
	t.Parallel()

	//nolint:nilnil
	t.Run("empty path", func(t *testing.T) {
		t.Parallel()

		called := false

		got := fileExistsAndNotEmpty(func(string) (os.FileInfo, error) {
			called = true

			return nil, nil
		}, "")
		if got {
			t.Fatal("expected false for empty path")
		}

		if called {
			t.Fatal("stat function must not be called for empty path")
		}
	})

	t.Run("stat error", func(t *testing.T) {
		t.Parallel()

		got := fileExistsAndNotEmpty(func(string) (os.FileInfo, error) {
			return nil, errors.New("boom")
		}, "/tmp/file")
		if got {
			t.Fatal("expected false when stat returns error")
		}
	})

	//nolint:nilnil
	t.Run("nil file info", func(t *testing.T) {
		t.Parallel()

		got := fileExistsAndNotEmpty(func(string) (os.FileInfo, error) {
			return nil, nil
		}, "/tmp/file")
		if got {
			t.Fatal("expected false when stat returns nil file info")
		}
	})

	t.Run("zero size file", func(t *testing.T) {
		t.Parallel()

		got := fileExistsAndNotEmpty(func(string) (os.FileInfo, error) {
			return fakeFileInfo{size: 0}, nil
		}, "/tmp/file")
		if got {
			t.Fatal("expected false for zero-size file")
		}
	})

	t.Run("non-empty file", func(t *testing.T) {
		t.Parallel()

		got := fileExistsAndNotEmpty(func(string) (os.FileInfo, error) {
			return fakeFileInfo{size: 1}, nil
		}, "/tmp/file")
		if !got {
			t.Fatal("expected true for non-empty file")
		}
	})
}

type fakeFileInfo struct {
	size int64
}

func (f fakeFileInfo) Name() string       { return "fake" }
func (f fakeFileInfo) Size() int64        { return f.size }
func (f fakeFileInfo) Mode() fs.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }
