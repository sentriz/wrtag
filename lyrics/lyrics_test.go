package lyrics_test

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.senan.xyz/wrtag/lyrics"
	"golang.org/x/time/rate"
)

//go:embed testdata
var responses embed.FS

type fakeSource struct {
	lyrics string
	err    error
	called bool
}

func (f *fakeSource) Search(ctx context.Context, artist, song string, duration time.Duration) (string, error) {
	f.called = true
	return f.lyrics, f.err
}

func TestMusixmatch(t *testing.T) {
	t.Parallel()

	var src lyrics.Musixmatch
	src.HTTPClient = fsClient(responses, "testdata/musixmatch")
	src.Limiter = rate.NewLimiter(rate.Inf, 0)

	resp, err := src.Search(t.Context(), "The Fall", "Wings", 0)
	require.NoError(t, err)
	assert.Contains(t, resp, "\nI paid them off with stuffing from my wings.\n")
	assert.Contains(t, resp, "\nThey had some fun with those cheapo airline snobs.\n")
	assert.Contains(t, resp, "\nThe stuffing loss made me hit a timelock.\n")

	resp, err = src.Search(t.Context(), "The Fall", "Uhh yeah - uh greath", 0)
	require.ErrorIs(t, err, lyrics.ErrTrackNotFound)
	assert.Empty(t, resp)

	// instrumental
	resp, err = src.Search(t.Context(), "Miles Davis", "Blue In Green", 0)
	require.NoError(t, err)
	assert.Empty(t, resp)
}

func TestGenius(t *testing.T) {
	t.Parallel()

	var src lyrics.Genius
	src.HTTPClient = fsClient(responses, "testdata/genius")
	src.Limiter = rate.NewLimiter(rate.Inf, 0)

	resp, err := src.Search(t.Context(), "the fall", "totally wired", 0)
	require.NoError(t, err)

	assert.Contains(t, resp, "\nI'm totally wired (can't you see?)\n")
	assert.Contains(t, resp, "\nI drank a jar of coffee\n")
	assert.Contains(t, resp, "\nAnd then I took some of these\n")

	resp, err = src.Search(t.Context(), "the fall", "uhh yeah - uh greath", 0)
	require.ErrorIs(t, err, lyrics.ErrTrackNotFound)
	assert.Empty(t, resp)

	// instrumental
	resp, err = src.Search(t.Context(), "miles davis", "blue in green", 0)
	require.NoError(t, err)
	assert.Empty(t, resp)
}

func TestGeniusLineBreak(t *testing.T) {
	t.Parallel()

	var src lyrics.Genius
	src.HTTPClient = fsClient(responses, "testdata/genius")
	src.Limiter = rate.NewLimiter(rate.Inf, 0)

	resp, err := src.Search(t.Context(), "pink floyd", "breathe in the air", 0)
	require.NoError(t, err)

	// assert it's one line, even though there's a link
	assert.Contains(t, resp, `[Segue from "Speak to Me": Clare Torry]`)
}

func TestMultiSource(t *testing.T) {
	t.Parallel()

	t.Run("continues past hard error and finds lyrics", func(t *testing.T) {
		src1 := &fakeSource{err: lyrics.ErrTrackNotFound}
		src2 := &fakeSource{err: errors.New("genius returned non 2xx: 403")}
		src3 := &fakeSource{lyrics: "some lyrics"}

		resp, err := (lyrics.MultiSource{src1, src2, src3}).Search(t.Context(), "", "", 0)
		require.NoError(t, err)
		assert.Equal(t, "some lyrics", resp)
		assert.True(t, src3.called)
	})

	t.Run("returns joined hard errors", func(t *testing.T) {
		err1 := errors.New("source 1 failed")
		err2 := errors.New("source 2 failed")
		src1 := &fakeSource{err: err1}
		src2 := &fakeSource{err: err2}

		resp, err := (lyrics.MultiSource{src1, src2}).Search(t.Context(), "", "", 0)
		require.Error(t, err)
		assert.Empty(t, resp)
		require.False(t, errors.Is(err, lyrics.ErrTrackNotFound))
		assert.True(t, errors.Is(err, err1))
		assert.True(t, errors.Is(err, err2))
	})

	t.Run("stops when track has no lyrics", func(t *testing.T) {
		src1 := &fakeSource{}
		src2 := &fakeSource{lyrics: "later lyrics"}

		resp, err := (lyrics.MultiSource{src1, src2}).Search(t.Context(), "", "", 0)
		require.NoError(t, err)
		assert.Empty(t, resp)
		assert.False(t, src2.called)
	})

	t.Run("returns not found when all sources miss", func(t *testing.T) {
		src1 := &fakeSource{err: lyrics.ErrTrackNotFound}
		src2 := &fakeSource{err: lyrics.ErrTrackNotFound}

		resp, err := (lyrics.MultiSource{src1, src2}).Search(t.Context(), "", "", 0)
		require.ErrorIs(t, err, lyrics.ErrTrackNotFound)
		assert.Empty(t, resp)
	})

	t.Run("stops at first source with lyrics", func(t *testing.T) {
		src1 := &fakeSource{lyrics: "first"}
		src2 := &fakeSource{lyrics: "later"}

		resp, err := (lyrics.MultiSource{src1, src2}).Search(t.Context(), "", "", 0)
		require.NoError(t, err)
		assert.Equal(t, "first", resp)
		assert.False(t, src2.called)
	})
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
