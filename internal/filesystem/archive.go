package filesystem

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// CreateArchive creates a tar.gz stream for selected paths relative to root.
func CreateArchive(root string, paths []string) (io.Reader, error) {
	reader, writer := io.Pipe()
	go func() {
		gz := gzip.NewWriter(writer)
		tw := tar.NewWriter(gz)
		writeErr := func(err error) {
			_ = tw.Close()
			_ = gz.Close()
			_ = writer.CloseWithError(err)
		}
		for _, rel := range paths {
			resolved := filepath.Join(root, filepath.Clean(rel))
			if err := filepath.Walk(resolved, func(path string, info os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				hdr, err := tar.FileInfoHeader(info, "")
				if err != nil {
					return err
				}
				archivePath := strings.TrimPrefix(path, root)
				archivePath = strings.TrimPrefix(archivePath, "/")
				hdr.Name = archivePath
				if err := tw.WriteHeader(hdr); err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				_, err = io.Copy(tw, file)
				closeErr := file.Close()
				if err != nil {
					return err
				}
				if closeErr != nil {
					return closeErr
				}
				return nil
			}); err != nil {
				writeErr(fmt.Errorf("walking archive paths: %w", err))
				return
			}
		}
		if err := tw.Close(); err != nil {
			_ = writer.CloseWithError(fmt.Errorf("closing tar writer: %w", err))
			return
		}
		if err := gz.Close(); err != nil {
			_ = writer.CloseWithError(fmt.Errorf("closing gzip writer: %w", err))
			return
		}
		_ = writer.Close()
	}()
	return reader, nil
}

// ExtractArchive extracts tar.gz content into dest.
func ExtractArchive(reader io.Reader, dest string) error {
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}
		cleanDest := filepath.Clean(dest)
		cleanName, err := sanitizeArchiveEntryName(hdr.Name)
		if err != nil {
			return fmt.Errorf("archive path escapes destination: %s", hdr.Name)
		}
		if cleanName == "" {
			continue
		}
		target := filepath.Join(cleanDest, cleanName)
		cleanTarget := filepath.Clean(target)
		destPrefix := cleanDest + string(filepath.Separator)
		if cleanTarget != cleanDest && !strings.HasPrefix(cleanTarget, destPrefix) {
			return fmt.Errorf("archive path escapes destination: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(cleanTarget, os.FileMode(hdr.Mode)); err != nil {
				return fmt.Errorf("creating extracted directory: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(cleanTarget), 0o755); err != nil {
				return fmt.Errorf("creating extracted parent: %w", err)
			}
			file, err := os.OpenFile(cleanTarget, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return fmt.Errorf("opening extracted file: %w", err)
			}
			if _, err := io.Copy(file, tr); err != nil {
				_ = file.Close()
				return fmt.Errorf("writing extracted file: %w", err)
			}
			if err := file.Close(); err != nil {
				return fmt.Errorf("closing extracted file: %w", err)
			}
		}
	}
}

func sanitizeArchiveEntryName(name string) (string, error) {
	clean := path.Clean("/" + name)
	rel := strings.TrimPrefix(clean, "/")
	if rel == "." || rel == "" {
		return "", nil
	}
	if strings.Contains(rel, "../") || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("invalid archive path")
	}
	return filepath.Clean(filepath.FromSlash(rel)), nil
}
