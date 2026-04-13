package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxReadSize = 4 << 20 // 4 MiB

// Entry describes a filesystem entry.
type Entry struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modified_at"`
	IsDir   bool      `json:"is_dir"`
}

// Filesystem performs sandboxed operations in a root directory.
type Filesystem struct {
	root string
}

// New creates a new sandboxed filesystem instance.
func New(root string) *Filesystem {
	return &Filesystem{root: root}
}

// List lists directory entries relative to sandbox root.
func (f *Filesystem) List(dir string) ([]Entry, error) {
	path, err := f.resolve(dir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}
	out := make([]Entry, 0, len(entries))
	for _, item := range entries {
		info, err := item.Info()
		if err != nil {
			return nil, fmt.Errorf("reading file info: %w", err)
		}
		out = append(out, Entry{
			Name:    item.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   item.IsDir(),
		})
	}
	return out, nil
}

// ReadFile reads a file in sandbox with max size protection.
func (f *Filesystem) ReadFile(path string) ([]byte, error) {
	resolved, err := f.resolve(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	if info.Size() > maxReadSize {
		return nil, fmt.Errorf("file too large: %d > %d", info.Size(), maxReadSize)
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	return data, nil
}

// WriteFile writes file content in sandbox.
func (f *Filesystem) WriteFile(path string, content []byte) error {
	resolved, err := f.resolve(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return fmt.Errorf("creating parent directories: %w", err)
	}
	if err := os.WriteFile(resolved, content, 0o644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	return nil
}

// DeleteFiles deletes files or directories in sandbox.
func (f *Filesystem) DeleteFiles(paths []string) error {
	for _, path := range paths {
		resolved, err := f.resolve(path)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(resolved); err != nil {
			return fmt.Errorf("deleting %q: %w", path, err)
		}
	}
	return nil
}

// CreateDirectory creates a directory recursively in sandbox.
func (f *Filesystem) CreateDirectory(path string) error {
	resolved, err := f.resolve(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(resolved, 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	return nil
}

func (f *Filesystem) resolve(rel string) (string, error) {
	cleanRel := filepath.Clean("/" + rel)
	joined := filepath.Join(f.root, strings.TrimPrefix(cleanRel, "/"))
	rootResolved, err := filepath.EvalSymlinks(f.root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			rootResolved = f.root
		} else {
			return "", fmt.Errorf("resolving root symlink: %w", err)
		}
	}

	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			resolved = joined
		} else {
			return "", fmt.Errorf("resolving path symlink: %w", err)
		}
	}

	relPath, err := filepath.Rel(rootResolved, resolved)
	if err != nil {
		return "", fmt.Errorf("checking sandbox path: %w", err)
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes sandbox")
	}
	return resolved, nil
}
