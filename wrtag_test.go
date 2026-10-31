package wrtag

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/tags"
	"go.senan.xyz/wrtag/tags/normtag"
)

// ⚠️ Note, core wrtag functionality is tested from ./cmd/wrtag/

func TestDiffer(t *testing.T) {
	t.Parallel()

	var score float64
	diff := Differ(&score)

	diff(1, "x", "aaaaa", "aaaaa")
	diff(1, "x", "aaaaa", "aaaaX")
	assert.InEpsilon(t, 90.0, score, 0) // 9 of 10 chars the same
}

func TestDiffWeightsLowerBound(t *testing.T) {
	t.Parallel()

	var score float64
	diff := Differ(&score)

	// all the same, but label/catalogue num mismatch
	diff(0, "label", "Columbia", "uh some other label")
	diff(0, "catalogue num", "Columbia", "not the same catalogue num")

	diff(1, "track 1", "The Day I Met God", "The Day I Met God")
	diff(1, "track 2", "Catholic Day", "Catholic Day")
	diff(1, "track 3", "Nine Plan Failed", "Nine Plan Failed")
	diff(1, "track 4", "Family of Noise", "Family of Noise")
	diff(1, "track 5", "Digital Tenderness", "Digital Tenderness")

	// but that's fine since we gave those 0 weight
	assert.InEpsilon(t, 100.0, score, 0)
}

func TestDiffWeightsUpperBound(t *testing.T) {
	t.Parallel()

	var score float64
	diff := Differ(&score)

	// all the same, but label/catalogue num mismatch
	diff(2, "label", "Columbia", "uh some other label")
	diff(2, "catalogue num", "Columbia", "not the same catalogue num")

	diff(1, "track 1", "The Day I Met God", "The Day I Met God")
	diff(1, "track 2", "Catholic Day", "Catholic Day")
	diff(1, "track 3", "Nine Plan Failed", "Nine Plan Failed")
	diff(1, "track 4", "Family of Noise", "Family of Noise")
	diff(1, "track 5", "Digital Tenderness", "Digital Tenderness")

	// bad score since we really care about label / catalogue num
	assert.InDelta(t, 32.0, score, 1)
}

func TestDiffNorm(t *testing.T) {
	t.Parallel()

	var score float64
	diff := Differ(&score)

	diff(1, "label", "Columbia", "COLUMBIA")
	diff(1, "catalogue num", "CLO LP 3", "CLOLP3")

	require.InEpsilon(t, 100.0, score, 0) // we don't care about case or spaces
}

func TestDiffIgnoreMissing(t *testing.T) {
	t.Parallel()

	var score float64
	diff := Differ(&score)

	diff(1, "label", "", "COLUMBIA")
	diff(1, "catalogue num", "CLO LP 3", "CLOLP3")

	assert.InEpsilon(t, 100.0, score, 0)
}

// https://github.com/sentriz/wrtag/issues/99
func TestNegativeScore(t *testing.T) {
	t.Parallel()

	var score float64
	diff := Differ(&score)

	diff(1, "release", "Moon Boots", "Moon Boots")
	diff(1, "artist", "Bird Bear Hare and Fish", "BBHF")
	diff(1, "label", "", "SME Records")
	diff(1, "catalogue num", "", "SECL-2324")
	diff(1, "upc", "", "4547366368383")
	diff(1, "media format", "", "CD")

	diff(1, "track 1", "Bird Bear Hare and Fish – ウクライナ", "BBHF – ウクライナ")
	diff(1, "track 2", "Bird Bear Hare and Fish – ライカ", "BBHF – ライカ")
	diff(1, "track 3", "Bird Bear Hare and Fish – ダッシュボード", "BBHF – ダッシュボード")
	diff(1, "track 4", "Bird Bear Hare and Fish – レプリカント", "BBHF – レプリカント")
	diff(1, "track 5", "Bird Bear Hare and Fish – Hearts", "BBHF – Hearts")
	diff(1, "track 6", "Bird Bear Hare and Fish – 夏の光", "BBHF – 夏の光")
	diff(1, "track 7", "Bird Bear Hare and Fish – ページ", "BBHF – ページ")
	diff(1, "track 8", "Bird Bear Hare and Fish – Wake Up", "BBHF – Wake Up")
	diff(1, "track 9", "Bird Bear Hare and Fish – Different", "BBHF – Different")
	diff(1, "track 10", "Bird Bear Hare and Fish – 骨の音", "BBHF – 骨の音")
	diff(1, "track 11", "Bird Bear Hare and Fish – 次の火", "BBHF – 次の火")
	diff(1, "track 12", "Bird Bear Hare and Fish – Work", "BBHF – Work")

	assert.InEpsilon(t, 37, score, 1)
}

func TestDiffNormText(t *testing.T) {
	t.Parallel()

	assert.Empty(t, diffNormText(""))
	assert.Empty(t, diffNormText(" "))
	assert.Equal(t, "123", diffNormText(" 1!2!3 "))
	assert.Equal(t, "séan", diffNormText("SÉan"))
	assert.Equal(t, "hello世界", diffNormText("~~ 【 Hello, 世界。 】~~ 😉"))
}

func TestIsNonFatalError(t *testing.T) {
	t.Parallel()

	assert.True(t, IsNonFatalError(ErrScoreTooLow))
	assert.True(t, IsNonFatalError(ErrTrackCountMismatch))
	assert.False(t, IsNonFatalError(ErrNoTracks))
	assert.False(t, IsNonFatalError(ErrNotSortable))
	assert.False(t, IsNonFatalError(ErrSelfCopy))
}

func TestNewDirContext(t *testing.T) {
	t.Parallel()

	dc := NewDirContext()
	assert.NotNil(t, dc.knownDestPaths)
	assert.Empty(t, dc.knownDestPaths)
}

func TestMoveCanModifyDest(t *testing.T) {
	t.Parallel()

	move := NewMove(false)
	assert.True(t, move.CanModifyDest())

	dryRunMove := NewMove(true)
	assert.False(t, dryRunMove.CanModifyDest())
}

func TestCopyCanModifyDest(t *testing.T) {
	t.Parallel()

	cpy := NewCopy(false)
	assert.True(t, cpy.CanModifyDest())

	dryRunCopy := NewCopy(true)
	assert.False(t, dryRunCopy.CanModifyDest())
}

func TestReflinkCanModifyDest(t *testing.T) {
	t.Parallel()

	reflink := NewReflink(false)
	assert.True(t, reflink.CanModifyDest())

	dryRunReflink := NewReflink(true)
	assert.False(t, dryRunReflink.CanModifyDest())
}

func TestForceSearchIgnoresTaggedRelease(t *testing.T) {
	t.Parallel()

	const (
		taggedMBID   = "21a03203-91a4-4948-ae1e-2d0977f1bdbc" // Ship-Scope
		searchedMBID = "e47d04a4-7460-427d-a731-cc82386d85f1" // Kat Moda
	)

	var searches atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /release", func(w http.ResponseWriter, r *http.Request) {
		searches.Add(1)
		_, _ = fmt.Fprintf(w, `{"releases":[{"id":%q,"score":100}]}`, searchedMBID)
	})
	mux.HandleFunc("GET /release/{mbid}", func(w http.ResponseWriter, r *http.Request) {
		if mbid := r.PathValue("mbid"); mbid != searchedMBID {
			t.Errorf("unexpected release fetch %q", mbid)
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join("cmd", "wrtag", "testdata", "responses", "musicbrainz", "ws", "2", "release", searchedMBID))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	emptyFLAC, err := os.ReadFile(filepath.Join("cmd", "wrtag", "testdata", "empty.flac"))
	require.NoError(t, err)

	srcDir := t.TempDir()
	for i := range 3 {
		p := filepath.Join(srcDir, fmt.Sprintf("%02d.flac", i+1))
		require.NoError(t, os.WriteFile(p, emptyFLAC, 0o644))

		tg := map[string][]string{}
		normtag.Set(tg, normtag.MusicBrainzReleaseID, taggedMBID)
		normtag.Set(tg, normtag.Album, "Kat Moda")
		normtag.Set(tg, normtag.TrackNumber, strconv.Itoa(i+1))
		require.NoError(t, tags.WriteTags(p, tg, 0))
	}

	var cfg Config
	cfg.MusicBrainzClient = musicbrainz.MBClient{BaseURL: srv.URL + "/", HTTPClient: srv.Client(), Limiter: rate.NewLimiter(rate.Inf, 1)}
	require.NoError(t, cfg.PathFormat.Parse(filepath.Join(t.TempDir(), "{{ .Release.Title | safepath }}/{{ .Track.Position }}{{ .Ext }}")))

	res, err := ProcessDir(t.Context(), &cfg, NewCopy(true), srcDir, HighScore, ForceSearch)
	require.NoError(t, err)
	assert.Equal(t, int32(1), searches.Load())
	assert.Equal(t, searchedMBID, res.Release.ID)
}
