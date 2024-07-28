package tags

import (
	"bytes"
	_ "embed"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrackNum(t *testing.T) {
	t.Parallel()

	path := newFile(t, emptyFLAC, ".flac")
	withf(t, path, func(f *File) {
		f.WriteNum(TrackNumber, 69)
	})
	withf(t, path, func(f *File) {
		f.WriteNum(TrackNumber, 69)
	})
	withf(t, path, func(f *File) {
		assert.Equal(t, 69, f.ReadNum(TrackNumber))
	})
}

func TestZero(t *testing.T) {
	t.Parallel()

	check := func(f *File) {
		var n int
		f.ReadAll(func(k string, vs []string) bool {
			n++
			return true
		})
		assert.Equal(t, 0, n)
	}

	for _, tf := range testFiles {
		t.Run(tf.name, func(t *testing.T) {
			path := newFile(t, tf.data, tf.ext)
			withf(t, path, func(f *File) {
				f.Write("catalognumber", "")
				check(f)
			})
			withf(t, path, func(f *File) {
				check(f)
			})
		})
	}
}

func TestNormalise(t *testing.T) {
	t.Parallel()

	raw := map[string][]string{
		"media":               {"CD"},
		"trackc":              {"14"},
		"year":                {"1967"},
		"album artist credit": {"Steve"},
	}
	normalise(raw, alternatives)

	exp := map[string][]string{
		"media":              {"CD"},
		"tracknumber":        {"14"},
		"date":               {"1967"},
		"albumartist_credit": {"Steve"},
	}
	require.Equal(t, exp, raw)
}

func TestDoubleSave(t *testing.T) {
	t.Parallel()

	path := newFile(t, emptyFLAC, ".flac")
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

func TestExtract(t *testing.T) {
	path := newFile(t, emptyFLAC, ".flac")
	f, err := Read(path)
	require.NoError(t, err)
	defer f.Close()

	f.Write("v", "it's -0.244!")
	require.Equal(t, -0.244, f.ReadFloat("v"))

	f.Write("v", "2010-09-08")
	require.Equal(t, time.Date(2010, time.September, 8, 0, 0, 0, 0, time.UTC), f.ReadTime("v"))

	f.Write("v", "2/12")
	require.Equal(t, 2, f.ReadNum("v"))
}

func TestExtendedTags(t *testing.T) {
	for _, tf := range testFiles {
		t.Run(tf.name, func(t *testing.T) {
			if tf.name == "m4a" {
				t.Skip("need https://github.com/taglib/taglib/pull/1240")
			}

			p := newFile(t, tf.data, tf.ext)
			withf(t, p, func(f *File) {
				f.Write(Artist, "steely dan")            // standard
				f.Write(AlbumArtist, "steely dan")       // extended
				f.Write(AlbumArtistCredit, "steely dan") // non standard
			})
			withf(t, p, func(f *File) {
				assert.Equal(t, "steely dan", f.Read(Artist))
				assert.Equal(t, "steely dan", f.Read(AlbumArtist))
				assert.Equal(t, "steely dan", f.Read(AlbumArtistCredit))
			})
		})
	}
}

var testFiles = []struct {
	name string
	data []byte
	ext  string
}{
	{"flac", emptyFLAC, ".flac"},
	{"mp3", emptyMP3, ".mp3"},
	{"m4a", emptyM4A, ".m4a"},
}

//go:embed testdata/empty.flac
var emptyFLAC []byte

//go:embed testdata/empty.mp3
var emptyMP3 []byte

//go:embed testdata/empty.m4a
var emptyM4A []byte

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

func withf(t *testing.T, path string, fn func(*File)) {
	t.Helper()

	f, err := Read(path)
	require.NoError(t, err)
	fn(f)
	require.NoError(t, f.Save())
	f.Close()
}
