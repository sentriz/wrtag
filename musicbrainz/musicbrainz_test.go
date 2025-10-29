package musicbrainz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUUID(t *testing.T) {
	t.Parallel()

	assert.False(t, uuidExpr.MatchString(""))
	assert.False(t, uuidExpr.MatchString("123"))
	assert.False(t, uuidExpr.MatchString("uhh dd720ac8-1c68-4484-abb7-0546413a55e3"))
	assert.True(t, uuidExpr.MatchString("dd720ac8-1c68-4484-abb7-0546413a55e3"))
	assert.True(t, uuidExpr.MatchString("DD720AC8-1C68-4484-ABB7-0546413A55E3"))
}

func TestMergeAndSortGenres(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		[]Genre{
			{ID: "a psychedelic", Name: "a psychedelic", Count: 3},
			{ID: "psy trance", Name: "psy trance", Count: 3},
			{ID: "techno", Name: "techno", Count: 2},
			{ID: "electronic a", Name: "electronic a", Count: 1},
			{ID: "electronic b", Name: "electronic b", Count: 1},
		},
		mergeAndSortGenres([]Genre{
			{ID: "electronic b", Name: "electronic b", Count: 1},
			{ID: "electronic a", Name: "electronic a", Count: 1},
			{ID: "psy trance", Name: "psy trance", Count: 3},
			{ID: "a psychedelic", Name: "a psychedelic", Count: 2},
			{ID: "a psychedelic", Name: "a psychedelic", Count: 1},
			{ID: "techno", Name: "techno", Count: 2},
		}),
	)
}

func TestFlatTracks(t *testing.T) {
	t.Parallel()

	t.Run("filters DVD media", func(t *testing.T) {
		t.Parallel()
		media := []Media{
			{Format: "DVD", Tracks: []Track{{ID: "dvd-track"}}},
			{Format: "CD", Tracks: []Track{{ID: "cd-track"}}},
		}
		tracks := FlatTracks(media)
		assert.Len(t, tracks, 1)
		assert.Equal(t, "cd-track", tracks[0].ID)
	})

	t.Run("filters Blu-ray media", func(t *testing.T) {
		t.Parallel()
		media := []Media{
			{Format: "Blu-ray", Tracks: []Track{{ID: "bluray-track"}}},
			{Format: "CD", Tracks: []Track{{ID: "cd-track"}}},
		}
		tracks := FlatTracks(media)
		assert.Len(t, tracks, 1)
		assert.Equal(t, "cd-track", tracks[0].ID)
	})

	t.Run("includes CD media", func(t *testing.T) {
		t.Parallel()
		media := []Media{
			{Format: "CD", Tracks: []Track{{ID: "cd-track-1"}}},
			{Format: "CD", Tracks: []Track{{ID: "cd-track-2"}}},
		}
		tracks := FlatTracks(media)
		assert.Len(t, tracks, 2)
		assert.Equal(t, "cd-track-1", tracks[0].ID)
		assert.Equal(t, "cd-track-2", tracks[1].ID)
	})

	t.Run("filters video tracks", func(t *testing.T) {
		t.Parallel()
		media := []Media{
			{Format: "CD", Tracks: []Track{
				{ID: "audio-track", Recording: struct {
					FirstReleaseDate string         `json:"first-release-date"`
					Genres           []Genre        `json:"genres"`
					Video            bool           `json:"video"`
					Disambiguation   string         `json:"disambiguation"`
					ID               string         `json:"id"`
					Length           int            `json:"length"`
					Title            string         `json:"title"`
					Artists          []ArtistCredit `json:"artist-credit"`
					Relations        []Relation     `json:"relations"`
					ISRCs            []string       `json:"isrcs"`
				}{Video: false}},
				{ID: "video-track", Recording: struct {
					FirstReleaseDate string         `json:"first-release-date"`
					Genres           []Genre        `json:"genres"`
					Video            bool           `json:"video"`
					Disambiguation   string         `json:"disambiguation"`
					ID               string         `json:"id"`
					Length           int            `json:"length"`
					Title            string         `json:"title"`
					Artists          []ArtistCredit `json:"artist-credit"`
					Relations        []Relation     `json:"relations"`
					ISRCs            []string       `json:"isrcs"`
				}{Video: true}},
			}},
		}
		tracks := FlatTracks(media)
		assert.Len(t, tracks, 1)
		assert.Equal(t, "audio-track", tracks[0].ID)
	})

	t.Run("includes pregap track", func(t *testing.T) {
		t.Parallel()
		pregapTrack := Track{ID: "pregap-track"}
		media := []Media{
			{Format: "CD", Pregap: &pregapTrack, Tracks: []Track{{ID: "regular-track"}}},
		}
		tracks := FlatTracks(media)
		assert.Len(t, tracks, 2)
		assert.Equal(t, "pregap-track", tracks[0].ID)
		assert.Equal(t, "regular-track", tracks[1].ID)
	})
}
