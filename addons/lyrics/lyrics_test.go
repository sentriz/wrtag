package lyrics_test

import (
	"context"
	"embed"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.senan.xyz/wrtag/addons/lyrics"
	"go.senan.xyz/wrtag/clientutil"
)

//go:embed testdata
var responses embed.FS

func TestMusixmatch(t *testing.T) {
	t.Parallel()

	var src lyrics.Musixmatch
	src.HTTPClient = clientutil.FSClient(responses, "testdata/musixmatch")

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
	src.HTTPClient = clientutil.FSClient(responses, "testdata/genius")

	resp, err := src.Search(context.Background(), "the fall", "totally wired")
	require.NoError(t, err)

	assert.True(t, strings.Contains(resp, `I'm totally wired (can't you see?)`))
	assert.True(t, strings.Contains(resp, `I drank a jar of coffee`))
	assert.True(t, strings.Contains(resp, `And then I took some of these`))

	resp, err = src.Search(context.Background(), "the fall", "uhh yeah - uh greath")
	require.ErrorIs(t, err, lyrics.ErrLyricsNotFound)
	assert.Empty(t, resp)
}
