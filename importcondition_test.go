package wrtag

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/tags"
	"go.senan.xyz/wrtag/tags/normtag"
)

func TestNeverImportsHighScore(t *testing.T) {
	t.Parallel()

	const katModa = "e47d04a4-7460-427d-a731-cc82386d85f1"

	// serve the recorded MusicBrainz responses used by the cmd/wrtag tests
	var tr http.Transport
	tr.RegisterProtocol("file", http.NewFileTransportFS(os.DirFS(filepath.Join("cmd", "wrtag", "testdata", "responses"))))

	emptyFLAC, err := os.ReadFile(filepath.Join("cmd", "wrtag", "testdata", "empty.flac"))
	require.NoError(t, err)

	srcDir := t.TempDir()
	for i := range 3 {
		p := filepath.Join(srcDir, strconv.Itoa(i+1)+".flac")
		require.NoError(t, os.WriteFile(p, emptyFLAC, 0o644))

		tg := map[string][]string{}
		normtag.Set(tg, normtag.MusicBrainzReleaseID, katModa)
		normtag.Set(tg, normtag.Album, "Kat Moda")
		normtag.Set(tg, normtag.TrackNumber, strconv.Itoa(i+1))
		require.NoError(t, tags.WriteTags(p, tg, 0))
	}

	var cfg Config
	cfg.MusicBrainzClient = musicbrainz.MBClient{BaseURL: "file:///musicbrainz/ws/2/", HTTPClient: &http.Client{Transport: &tr}, Limiter: rate.NewLimiter(rate.Inf, 1)}
	require.NoError(t, cfg.PathFormat.Parse(filepath.Join(t.TempDir(), "{{ .Release.Title | safepath }}/{{ .Track.Position }}{{ .Ext }}")))

	// the match is good enough to import on score alone
	res, err := ProcessDir(t.Context(), &cfg, NewCopy(true), srcDir, HighScore, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, res.Score, float64(MinScore))

	// but Never still waits for confirmation
	res, err = ProcessDir(t.Context(), &cfg, NewCopy(true), srcDir, Never, "")
	require.ErrorIs(t, err, ErrNeedsConfirm)
	assert.Equal(t, katModa, res.Release.ID)
	assert.GreaterOrEqual(t, res.Score, float64(MinScore))
}
