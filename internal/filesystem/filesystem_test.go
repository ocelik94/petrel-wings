package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fs := New(root)

	if err := fs.WriteFile("logs/latest.log", []byte("hello")); err != nil {
		t.Fatalf("write file: %v", err)
	}
	data, err := fs.ReadFile("logs/latest.log")
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected file content: %q", data)
	}
}

func TestPreventsPathTraversal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fs := New(root)

	outside := filepath.Join(root, "..", "outside.txt")
	_ = os.WriteFile(outside, []byte("x"), 0o644)

	if _, err := fs.ReadFile("../outside.txt"); err == nil {
		t.Fatal("expected path traversal error")
	}
}
