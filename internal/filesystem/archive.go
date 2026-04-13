package filesystem

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
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
				defer file.Close()
				_, err = io.Copy(tw, file)
				return err
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
		target := filepath.Join(dest, hdr.Name)
		cleanDest := filepath.Clean(dest)
		cleanTarget := filepath.Clean(target)
		if !strings.HasPrefix(cleanTarget, cleanDest) {
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
