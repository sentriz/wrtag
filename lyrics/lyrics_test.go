package lyrics_test

import (
	"embed"
	"io/fs"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.senan.xyz/wrtag/lyrics"
)

//go:embed testdata
var responses embed.FS

func TestMusixmatch(t *testing.T) {
	t.Parallel()

	var src lyrics.Musixmatch
	src.HTTPClient = fsClient(responses, "testdata/musixmatch")

	resp, err := src.Search(t.Context(), "The Fall", "Wings")
	require.NoError(t, err)
	assert.Contains(t, resp, "\nI paid them off with stuffing from my wings.\n")
	assert.Contains(t, resp, "\nThey had some fun with those cheapo airline snobs.\n")
	assert.Contains(t, resp, "\nThe stuffing loss made me hit a timelock.\n")

	resp, err = src.Search(t.Context(), "The Fall", "Uhh yeah - uh greath")
	require.ErrorIs(t, err, lyrics.ErrLyricsNotFound)
	assert.Empty(t, resp)
}

func TestGenius(t *testing.T) {
	t.Parallel()

	var src lyrics.Genius
	src.HTTPClient = fsClient(responses, "testdata/genius")

	resp, err := src.Search(t.Context(), "the fall", "totally wired")
	require.NoError(t, err)

	assert.Contains(t, resp, "\nI'm totally wired (can't you see?)\n")
	assert.Contains(t, resp, "\nI drank a jar of coffee\n")
	assert.Contains(t, resp, "\nAnd then I took some of these\n")

	resp, err = src.Search(t.Context(), "the fall", "uhh yeah - uh greath")
	require.ErrorIs(t, err, lyrics.ErrLyricsNotFound)
	assert.Empty(t, resp)
}

func TestGeniusLineBreak(t *testing.T) {
	t.Parallel()

	var src lyrics.Genius
	src.HTTPClient = fsClient(responses, "testdata/genius")

	resp, err := src.Search(t.Context(), "pink floyd", "breathe in the air")
	require.NoError(t, err)

	// assert it's one line, even though there's a link
	assert.Contains(t, resp, `[Segue from "Speak to Me": Clare Torry]`)
}

func fsClient(fsys fs.FS, sub string) *http.Client {
	fsys, err := fs.Sub(fsys, sub)
	if err != nil {
		panic(err)
	}
	var c http.Client
	c.Transport = http.NewFileTransportFS(fsys)
	return &c
}
