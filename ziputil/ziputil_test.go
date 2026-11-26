package ziputil

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsArchive(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	writeZip(t, filepath.Join(tmp, "Release.ZIP"), "01.flac")
	require.NoError(t, os.Mkdir(filepath.Join(tmp, "dir.zip"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "01.flac"), nil, 0o600))

	assert.True(t, IsArchive(filepath.Join(tmp, "Release.ZIP")))
	assert.False(t, IsArchive(filepath.Join(tmp, "dir.zip")))
	assert.False(t, IsArchive(filepath.Join(tmp, "01.flac")))
	assert.False(t, IsArchive(filepath.Join(tmp, "missing.zip")))
}

func TestExtractStaysInDir(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	archive := filepath.Join(tmp, "release.zip")
	writeZip(t, archive, "../escaped.flac", "/abs.flac", "ok/01.flac")

	dir := filepath.Join(tmp, "out", "dir")
	require.NoError(t, extract(archive, dir))

	assert.FileExists(t, filepath.Join(dir, "escaped.flac"))
	assert.FileExists(t, filepath.Join(dir, "abs.flac"))
	assert.FileExists(t, filepath.Join(dir, "ok", "01.flac"))
	assert.NoFileExists(t, filepath.Join(tmp, "out", "escaped.flac"))
}

func TestExtractFindsReleaseAndCleansUp(t *testing.T) {
	t.Parallel()

	archive := filepath.Join(t.TempDir(), "release.zip")
	writeZip(t, archive, "wrap/Album/CD1/01.flac", "wrap/Album/CD2/01.flac")

	dir, cleanup, err := Extract(archive)
	require.NoError(t, err)
	assert.Equal(t, "Album", filepath.Base(dir))
	assert.FileExists(t, filepath.Join(dir, "CD1", "01.flac"))
	assert.FileExists(t, filepath.Join(dir, "CD2", "01.flac"))

	cleanup()
	assert.NoDirExists(t, dir)
	assert.NoDirExists(t, filepath.Dir(filepath.Dir(dir)))
	assert.FileExists(t, archive)
}

func writeZip(t *testing.T, path string, names ...string) {
	t.Helper()

	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	zw := zip.NewWriter(f)
	for _, name := range names {
		_, err := zw.Create(name)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
}
