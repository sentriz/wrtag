package lyrics_test

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.senan.xyz/wrtag/addon/lyrics"
)

//go:embed testdata
var responses embed.FS

func TestMusixmatch(t *testing.T) {
	t.Parallel()

	var src lyrics.Musixmatch
	src.HTTPClient = fsClient(responses, "testdata/musixmatch")

	resp, err := src.Search(context.Background(), "The Fall", "Wings")
	require.NoError(t, err)
	assert.True(t, strings.Contains(resp, `I paid them off with stuffing from my wings.`))
	assert.True(t, strings.Contains(resp, `They had some fun with those cheapo airline snobs.`))
	assert.True(t, strings.Contains(resp, `The stuffing loss made me hit a timelock.`))

	resp, err = src.Search(context.Background(), "The Fall", "Uhh yeah - uh greath")
	require.ErrorIs(t, err, lyrics.ErrLyricsNotFound)
	assert.Empty(t, resp)
}

func TestGenius(t *testing.T) {
	t.Parallel()

	var src lyrics.Genius
	src.HTTPClient = fsClient(responses, "testdata/genius")

	resp, err := src.Search(context.Background(), "the fall", "totally wired")
	require.NoError(t, err)

	assert.True(t, strings.Contains(resp, `I'm totally wired (can't you see?)`))
	assert.True(t, strings.Contains(resp, `I drank a jar of coffee`))
	assert.True(t, strings.Contains(resp, `And then I took some of these`))

	resp, err = src.Search(context.Background(), "the fall", "uhh yeah - uh greath")
	require.ErrorIs(t, err, lyrics.ErrLyricsNotFound)
	assert.Empty(t, resp)
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
