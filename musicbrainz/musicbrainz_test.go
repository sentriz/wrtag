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

	// Multi-disc tests
	t.Run("preserves disc information", func(t *testing.T) {
		t.Parallel()
		media := []Media{
			{Position: 1, Title: "Disc One", Format: "CD", Tracks: []Track{
				{Position: 1, Title: "Track 1"},
				{Position: 2, Title: "Track 2"},
			}},
			{Position: 2, Title: "Disc Two", Format: "CD", Tracks: []Track{
				{Position: 1, Title: "Track 3"},
				{Position: 2, Title: "Track 4"},
			}},
		}
		tracks := FlatTracks(media)

		assert.Len(t, tracks, 4)
		// Disc 1 tracks
		assert.Equal(t, 1, tracks[0].DiscNumber)
		assert.Equal(t, "Disc One", tracks[0].DiscTitle)
		assert.Equal(t, "CD", tracks[0].DiscFormat)
		assert.Equal(t, 1, tracks[1].DiscNumber)
		// Disc 2 tracks
		assert.Equal(t, 2, tracks[2].DiscNumber)
		assert.Equal(t, "Disc Two", tracks[2].DiscTitle)
		assert.Equal(t, 2, tracks[3].DiscNumber)
	})

	t.Run("renumbers discs after filtering", func(t *testing.T) {
		t.Parallel()
		media := []Media{
			{Position: 1, Format: "CD", Tracks: []Track{{Position: 1, Title: "Track 1"}}},
			{Position: 2, Format: "DVD", Tracks: []Track{{Position: 1, Title: "Video 1"}}},
			{Position: 3, Format: "CD", Tracks: []Track{{Position: 1, Title: "Track 2"}}},
			{Position: 4, Format: "Blu-ray", Tracks: []Track{{Position: 1, Title: "Concert"}}},
			{Position: 5, Format: "CD", Tracks: []Track{{Position: 1, Title: "Track 3"}}},
		}
		tracks := FlatTracks(media)

		assert.Len(t, tracks, 3)
		// Verify sequential disc numbering (1, 2, 3) not (1, 3, 5)
		assert.Equal(t, 1, tracks[0].DiscNumber)
		assert.Equal(t, 2, tracks[1].DiscNumber)
		assert.Equal(t, 3, tracks[2].DiscNumber)
	})

	t.Run("range variable fix", func(t *testing.T) {
		t.Parallel()
		media := []Media{
			{Position: 1, Title: "Disc 1", Format: "CD", Tracks: []Track{
				{Position: 1, Title: "A"},
				{Position: 2, Title: "B"},
			}},
			{Position: 2, Title: "Disc 2", Format: "CD", Tracks: []Track{
				{Position: 1, Title: "C"},
				{Position: 2, Title: "D"},
			}},
		}
		tracks := FlatTracks(media)

		// Verify each track has correct disc info (not all pointing to last disc)
		assert.Equal(t, "Disc 1", tracks[0].DiscTitle)
		assert.Equal(t, "Disc 1", tracks[1].DiscTitle)
		assert.Equal(t, "Disc 2", tracks[2].DiscTitle)
		assert.Equal(t, "Disc 2", tracks[3].DiscTitle)
	})

	t.Run("pregap tracks on multi-disc", func(t *testing.T) {
		t.Parallel()
		pregap := Track{Position: 0, Title: "[hidden]"}
		media := []Media{
			{Position: 1, Title: "Disc 1", Format: "CD", Pregap: &pregap, Tracks: []Track{
				{Position: 1, Title: "Track 1"},
			}},
			{Position: 2, Title: "Disc 2", Format: "CD", Tracks: []Track{
				{Position: 1, Title: "Track 2"},
			}},
		}
		tracks := FlatTracks(media)

		assert.Len(t, tracks, 3)
		// Pregap should be on disc 1
		assert.Equal(t, 0, tracks[0].Position)
		assert.Equal(t, 1, tracks[0].DiscNumber)
		assert.Equal(t, "Disc 1", tracks[0].DiscTitle)
	})
}

func TestCountNonFilteredDiscs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		media []Media
		want  int
	}{
		{
			name: "all CDs",
			media: []Media{
				{Format: "CD"},
				{Format: "CD"},
				{Format: "CD"},
			},
			want: 3,
		},
		{
			name: "with DVD",
			media: []Media{
				{Format: "CD"},
				{Format: "DVD"},
				{Format: "CD"},
			},
			want: 2,
		},
		{
			name: "with Blu-ray",
			media: []Media{
				{Format: "CD"},
				{Format: "Blu-ray"},
				{Format: "CD"},
			},
			want: 2,
		},
		{
			name: "mixed formats",
			media: []Media{
				{Format: "CD"},
				{Format: "DVD"},
				{Format: "CD"},
				{Format: "Blu-ray"},
				{Format: "CD"},
			},
			want: 3,
		},
		{
			name: "only video",
			media: []Media{
				{Format: "DVD"},
				{Format: "Blu-ray"},
			},
			want: 0,
		},
		{
			name:  "empty",
			media: []Media{},
			want:  0,
		},
		{
			name: "single CD",
			media: []Media{
				{Format: "CD"},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := CountNonFilteredDiscs(tt.media)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestArtistEnName(t *testing.T) {
	t.Parallel()

	t.Run("returns primary non-ended English alias", func(t *testing.T) {
		t.Parallel()
		artist := Artist{
			Name: "跡部進一",
			Aliases: []Alias{
				{Name: "Shinichi Atobe", Locale: "en", Primary: true, Ended: false},
				{Name: "Other English Name", Locale: "en", Primary: false, Ended: false},
			},
		}
		assert.Equal(t, "Shinichi Atobe", artistEnName(artist))
	})

	t.Run("returns non-primary non-ended English alias when no primary exists", func(t *testing.T) {
		t.Parallel()
		artist := Artist{
			Name: "ネイティブ名",
			Aliases: []Alias{
				{Name: "English Name", Locale: "en", Primary: false, Ended: false},
			},
		}
		assert.Equal(t, "English Name", artistEnName(artist))
	})

	t.Run("skips ended English aliases", func(t *testing.T) {
		t.Parallel()
		artist := Artist{
			Name: "Taylor Swift",
			Aliases: []Alias{
				{Name: "Dr. Taylor Alison Swift", Locale: "en", Primary: false, Ended: true},
				{Name: "Taylor Swift", Locale: "en", Primary: true, Ended: false},
			},
		}
		assert.Equal(t, "Taylor Swift", artistEnName(artist))
	})

	t.Run("returns artist name when only ended English aliases exist", func(t *testing.T) {
		t.Parallel()
		artist := Artist{
			Name: "Artist Name",
			Aliases: []Alias{
				{Name: "Old English Name", Locale: "en", Primary: true, Ended: true},
			},
		}
		assert.Equal(t, "Artist Name", artistEnName(artist))
	})

	t.Run("returns artist name when no English aliases exist", func(t *testing.T) {
		t.Parallel()
		artist := Artist{
			Name: "Artist Name",
			Aliases: []Alias{
				{Name: "日本語名", Locale: "ja", Primary: true, Ended: false},
			},
		}
		assert.Equal(t, "Artist Name", artistEnName(artist))
	})

	t.Run("prioritizes primary over non-primary even if non-primary appears first", func(t *testing.T) {
		t.Parallel()
		artist := Artist{
			Name: "ネイティブ名",
			Aliases: []Alias{
				{Name: "Non-Primary English", Locale: "en", Primary: false, Ended: false},
				{Name: "Primary English", Locale: "en", Primary: true, Ended: false},
			},
		}
		assert.Equal(t, "Primary English", artistEnName(artist))
	})

	t.Run("returns Latin name directly without checking aliases", func(t *testing.T) {
		t.Parallel()
		artist := Artist{
			Name: "Chris Brown",
			Aliases: []Alias{
				{Name: "Christopher Maurice Brown", Locale: "en", Primary: true, Ended: false},
			},
		}
		assert.Equal(t, "Chris Brown", artistEnName(artist))
	})
}
