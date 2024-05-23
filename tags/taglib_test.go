package tags

import (
	"bytes"
	_ "embed"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/empty.flac
var emptyFlac []byte

//go:embed testdata/empty.mp3
var emptyMP3 []byte

func newFile(t *testing.T, data []byte, ext string) string {
	t.Helper()

	f, err := os.CreateTemp("", "*"+ext)
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Remove(f.Name())
	})

	_, err = io.Copy(f, bytes.NewReader(data))
	require.NoError(t, err)

	return f.Name()
}

func TestTrackNum(t *testing.T) {
	t.Parallel()

	path := newFile(t, emptyFlac, ".flac")
	withf := func(fn func(*File)) {
		t.Helper()

		f, err := Read(path)
		require.NoError(t, err)
		fn(f)
		require.NoError(t, f.Save())
		f.Close()
	}

	withf(func(f *File) {
		f.WriteNum(TrackNumber, 69)
	})
	withf(func(f *File) {
		f.WriteNum(TrackNumber, 69)
	})
	withf(func(f *File) {
		require.Equal(t, 69, f.ReadNum(TrackNumber))
	})
}

func TestZero(t *testing.T) {
	t.Parallel()

	withf := func(path string, fn func(*File)) {
		t.Helper()

		f, err := Read(path)
		require.NoError(t, err)
		fn(f)
		require.NoError(t, f.Save())
		f.Close()
	}

	check := func(f *File) {
		var n int
		f.ReadAll(func(k string, vs []string) bool {
			n++
			return true
		})
		assert.Equal(t, 0, n)
	}

	run := func(name string, data []byte, ext string) {
		t.Run(name, func(t *testing.T) {
			path := newFile(t, data, ext)
			withf(path, func(f *File) {
				f.Write("catalognumber", "")
				check(f)
			})
			withf(path, func(f *File) {
				check(f)
			})
		})
	}

	run("flac", emptyFlac, ".flac")
	run("mp3", emptyMP3, ".mp3")
}

func TestNormalise(t *testing.T) {
	t.Parallel()

	raw := map[string][]string{
		"Some":                {"a", "b"},
		"other Test":          {"x"},
		"media":               {"CD"},
		"trackc":              {"14"},
		"year":                {"1967"},
		"album artist credit": {"Steve"},
	}
	normalise(raw, replacements)

	exp := map[string][]string{
		"some":       {"a", "b"}, // convert lower
		"other_test": {"x"},      // replace spaces
		"media":      {"CD"},     // leave alone

		// replacements
		"tracknumber":        {"14"},
		"date":               {"1967"},
		"albumartist_credit": {"Steve"},
	}
	require.Equal(t, exp, raw)
}

func TestDoubleSave(t *testing.T) {
	t.Parallel()

	path := newFile(t, emptyFlac, ".flac")
	f, err := Read(path)
	require.NoError(t, err)
	defer f.Close()

	f.Write(Album, "a")
	require.NoError(t, f.Save())
	f.Write(Album, "b")
	require.NoError(t, f.Save())
	f.Write(Album, "c")
	require.NoError(t, f.Save())
}
