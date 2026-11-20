// Package ziputil extracts a release from a zip archive, so it can be imported like a directory.
package ziputil

import (
	"archive/zip"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// IsArchive reports whether path is a zip archive.
func IsArchive(path string) bool {
	if !strings.EqualFold(filepath.Ext(path), ".zip") {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// Extract extracts the zip archive at path to a temporary directory. It returns the directory of the release inside,
// and a cleanup func which removes the temporary directory.
func Extract(path string) (dir string, cleanup func(), err error) {
	tmp, err := os.MkdirTemp("", "wrtag-zip-*")
	if err != nil {
		return "", nil, fmt.Errorf("make temp dir: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(tmp) }

	slog.Info("extracting archive", "path", path, "to", tmp)
	if err := extract(path, tmp); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("extract: %w", err)
	}
	dir, err = releaseRoot(tmp)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("find release: %w", err)
	}
	return dir, cleanup, nil
}

// extract extracts the zip archive at path into dir. Entry names can't escape dir, since [zip.Reader] cleans them of
// leading "/" and "../" when used as an [fs.FS], and it can't read symlinks so [os.CopyFS] rejects them.
func extract(path, dir string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer zr.Close() //nolint:errcheck

	return os.CopyFS(dir, zr)
}

// releaseRoot descends into dir while it holds only a single directory, since archives often wrap the release in a
// folder of its own.
func releaseRoot(dir string) (string, error) {
	for {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return "", err
		}
		if len(entries) != 1 || !entries[0].IsDir() {
			return dir, nil
		}
		dir = filepath.Join(dir, entries[0].Name())
	}
}
