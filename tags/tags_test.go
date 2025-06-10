package tags

import (
	"bytes"
	_ "embed"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrackNum(t *testing.T) {
	t.Parallel()

	path := newFile(t, emptyFLAC, ".flac")
	withf(t, path, func(f Tags) {
		f.Set(TrackNumber, strconv.Itoa(69))
	})
	withf(t, path, func(f Tags) {
		f.Set(TrackNumber, strconv.Itoa(69))
	})
	withf(t, path, func(f Tags) {
		assert.Equal(t, "69", f.Get(TrackNumber))
	})
}

func TestDoubleSave(t *testing.T) {
	t.Parallel()

	path := newFile(t, emptyFLAC, ".flac")
	f, err := ReadTags(path)
	require.NoError(t, err)

	f.Set(Album, "a")
	require.NoError(t, WriteTags(path, f, Clear))
	f.Set(Album, "b")
	require.NoError(t, WriteTags(path, f, Clear))
	f.Set(Album, "c")
	require.NoError(t, WriteTags(path, f, Clear))
}

func TestExtendedTags(t *testing.T) {
	t.Parallel()

	for _, tf := range testFiles {
		t.Run(tf.name, func(t *testing.T) {
			t.Parallel()

			p := newFile(t, tf.data, tf.ext)
			withf(t, p, func(f Tags) {
				f.Set(Artist, "1. steely dan")            // standard
				f.Set(AlbumArtist, "2. steely dan")       // extended
				f.Set(AlbumArtistCredit, "3. steely dan") // non standard
			})
			withf(t, p, func(f Tags) {
				assert.Equal(t, "1. steely dan", f.Get(Artist))
				assert.Equal(t, "2. steely dan", f.Get(AlbumArtist))
				assert.Equal(t, "3. steely dan", f.Get(AlbumArtistCredit))
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

var (
	//go:embed testdata/empty.flac
	emptyFLAC []byte
	//go:embed testdata/empty.mp3
	emptyMP3 []byte
	//go:embed testdata/empty.m4a
	emptyM4A []byte
)

func newFile(t *testing.T, data []byte, ext string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "f"+ext)
	f, err := os.Create(path)
	require.NoError(t, err)

	_, err = io.Copy(f, bytes.NewReader(data))
	require.NoError(t, err)

	return f.Name()
}

func withf(t *testing.T, path string, fn func(Tags)) {
	t.Helper()

	tags, err := ReadTags(path)
	require.NoError(t, err)

	fn(tags)

	require.NoError(t, WriteTags(path, tags, Clear))
}
