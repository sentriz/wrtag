package pathformat_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/pathformat"
)

func TestValidation(t *testing.T) {
	t.Parallel()

	var pf pathformat.Format
	_, err := pf.Execute(nil, 0, "")
	require.Error(t, err) // we didn't initialise with Parse() yet

	// bad/ambiguous format
	require.ErrorIs(t, pf.Parse(""), pathformat.ErrInvalidFormat)
	require.ErrorIs(t, pf.Parse(" "), pathformat.ErrInvalidFormat)
	require.ErrorIs(t, pf.Parse("🤤"), pathformat.ErrInvalidFormat)

	require.ErrorIs(t, pf.Parse(`/albums/test/{{ artists .Release.Artists | join " " }}/{{ .Release.Title }}`), pathformat.ErrAmbiguousFormat)
	require.ErrorIs(t, pf.Parse(`/albums/test/{{ .Track.Title }}`), pathformat.ErrAmbiguousFormat)
	require.ErrorIs(t, pf.Parse(`/albums/test/{{ .TrackNum }}`), pathformat.ErrAmbiguousFormat)

	// bad data
	require.ErrorIs(t, pf.Parse(`/albums/test/{{ artists .Release.Artists | join " " }}/{{ .Release.ID }}/`), pathformat.ErrBadData)                   // test case is missing ID
	require.ErrorIs(t, pf.Parse(`/albums/test/{{ artists .Release.Artists | join " " }}//`), pathformat.ErrBadData)                                    // double slash anyway
	require.ErrorIs(t, pf.Parse(`/albums/test/{{ artists .Release.Artists | join " " }}/{{ .Release.Title }}/{{ .Track.ID }}`), pathformat.ErrBadData) // implicit trailing slash from missing ID
	require.ErrorIs(t, pf.Parse(`/albums/test/{{ .Track.ID }}/`), pathformat.ErrBadData)                                                               //

	// good
	require.NoError(t, pf.Parse(`/albums/test/{{ artists .Release.Artists | join " " }}/{{ .Release.Title }}/{{ .TrackNum }}`))
	assert.Equal(t, "/albums/test", pf.Root())
}

func TestPathFormat(t *testing.T) {
	t.Parallel()

	track := musicbrainz.Track{
		Title:    "Sharon's Tone",
		Position: 1,
	}
	release := &musicbrainz.Release{
		Title: "Valvable",
		ReleaseGroup: musicbrainz.ReleaseGroup{
			FirstReleaseDate: musicbrainz.AnyTime{Time: time.Date(2019, time.January, 0, 0, 0, 0, 0, time.UTC)},
		},
		Artists: []musicbrainz.ArtistCredit{
			{
				Name: "credit name",
				Artist: musicbrainz.Artist{
					Name: "Luke Vibert",
				},
			},
		},
		Media: []musicbrainz.Media{{
			Tracks: []musicbrainz.Track{
				track,
			},
		}},
	}

	var pf pathformat.Format
	require.NoError(t, pf.Parse(`/music/albums/{{ artists .Release.Artists | sort | join "; " | safepath }}/({{ .Release.ReleaseGroup.FirstReleaseDate.Year }}) {{ .Release.Title | safepath }}{{ if not (eq .ReleaseDisambiguation "") }} ({{ .ReleaseDisambiguation | safepath }}){{ end }}/{{ pad0 2 .TrackNum }}.{{ len .Tracks | pad0 2 }} {{ .Track.Title | safepath }}{{ .Ext }}`))

	path, err := pf.Execute(release, 0, ".flac")
	require.NoError(t, err)
	assert.Equal(t, `/music/albums/Luke Vibert/(2018) Valvable/01.01 Sharon's Tone.flac`, path)

	release.ReleaseGroup.Disambiguation = "Deluxe Edition"

	path, err = pf.Execute(release, 0, ".flac")
	require.NoError(t, err)
	assert.Equal(t, `/music/albums/Luke Vibert/(2018) Valvable (Deluxe Edition)/01.01 Sharon's Tone.flac`, path)

	require.NoError(t, pf.Parse(`/music/albums/{{ artists .Release.Artists | the | sort | join "; " | safepath }}/{{ .Release.Title }}/{{ .TrackNum }}{{ .Ext }}`))

	release.Artists[0].Artist.Name = "A House"

	path, err = pf.Execute(release, 0, ".flac")
	require.NoError(t, err)
	assert.Equal(t, `/music/albums/House, A/Valvable/1.flac`, path)

	release.Artists[0].Artist.Name = "The House"

	path, err = pf.Execute(release, 0, ".flac")
	require.NoError(t, err)
	assert.Equal(t, `/music/albums/House, The/Valvable/1.flac`, path)
}

func TestPathFormatMultiDisc(t *testing.T) {
	t.Parallel()

	// Create a 2-disc release with disc titles
	release := &musicbrainz.Release{
		Title: "Reise, Reise",
		ReleaseGroup: musicbrainz.ReleaseGroup{
			FirstReleaseDate: musicbrainz.AnyTime{Time: time.Date(2004, time.November, 27, 0, 0, 0, 0, time.UTC)},
		},
		Artists: []musicbrainz.ArtistCredit{
			{
				Name: "Rammstein",
				Artist: musicbrainz.Artist{
					Name: "Rammstein",
				},
			},
		},
		Media: []musicbrainz.Media{
			{
				Position: 1,
				Title:    "Live Recordings",
				Format:   "CD",
				Tracks: []musicbrainz.Track{
					{
						Title:      "Reise, Reise",
						Position:   1,
						DiscNumber: 1,
						DiscTitle:  "Live Recordings",
						DiscFormat: "CD",
					},
					{
						Title:      "Mein Teil",
						Position:   2,
						DiscNumber: 1,
						DiscTitle:  "Live Recordings",
						DiscFormat: "CD",
					},
				},
			},
			{
				Position: 2,
				Title:    "Bonus Material",
				Format:   "CD",
				Tracks: []musicbrainz.Track{
					{
						Title:      "Dalai Lama",
						Position:   1,
						DiscNumber: 2,
						DiscTitle:  "Bonus Material",
						DiscFormat: "CD",
					},
					{
						Title:      "Keine Lust",
						Position:   2,
						DiscNumber: 2,
						DiscTitle:  "Bonus Material",
						DiscFormat: "CD",
					},
				},
			},
		},
	}

	t.Run("disc number in path", func(t *testing.T) {
		t.Parallel()
		var pf pathformat.Format
		require.NoError(t, pf.Parse(`/music/{{ .Release.Title }}/Disc {{ .DiscNum }}/{{ pad0 2 .Track.Position }} {{ .Track.Title }}{{ .Ext }}`))

		// Disc 1, Track 1
		path, err := pf.Execute(release, 0, ".flac")
		require.NoError(t, err)
		assert.Equal(t, `/music/Reise, Reise/Disc 1/01 Reise, Reise.flac`, path)

		// Disc 1, Track 2
		path, err = pf.Execute(release, 1, ".flac")
		require.NoError(t, err)
		assert.Equal(t, `/music/Reise, Reise/Disc 1/02 Mein Teil.flac`, path)

		// Disc 2, Track 1
		path, err = pf.Execute(release, 2, ".flac")
		require.NoError(t, err)
		assert.Equal(t, `/music/Reise, Reise/Disc 2/01 Dalai Lama.flac`, path)

		// Disc 2, Track 2
		path, err = pf.Execute(release, 3, ".flac")
		require.NoError(t, err)
		assert.Equal(t, `/music/Reise, Reise/Disc 2/02 Keine Lust.flac`, path)
	})

	t.Run("disc title in path", func(t *testing.T) {
		t.Parallel()
		var pf pathformat.Format
		// Use safepath to handle empty disc titles gracefully
		require.NoError(t, pf.Parse(`/music/{{ .Release.Title }}/{{ if .DiscTitle }}{{ .DiscTitle | safepath }}{{ else }}Disc {{ .DiscNum }}{{ end }}/{{ .Track.Title }}{{ .Ext }}`))

		path, err := pf.Execute(release, 0, ".flac")
		require.NoError(t, err)
		assert.Equal(t, `/music/Reise, Reise/Live Recordings/Reise, Reise.flac`, path)

		path, err = pf.Execute(release, 2, ".flac")
		require.NoError(t, err)
		assert.Equal(t, `/music/Reise, Reise/Bonus Material/Dalai Lama.flac`, path)
	})

	t.Run("total discs in path", func(t *testing.T) {
		t.Parallel()
		var pf pathformat.Format
		require.NoError(t, pf.Parse(`/music/{{ .Release.Title }}/{{ .DiscNum }} of {{ .TotalDiscs }}/{{ .Track.Title }}{{ .Ext }}`))

		path, err := pf.Execute(release, 0, ".flac")
		require.NoError(t, err)
		assert.Equal(t, `/music/Reise, Reise/1 of 2/Reise, Reise.flac`, path)

		path, err = pf.Execute(release, 2, ".flac")
		require.NoError(t, err)
		assert.Equal(t, `/music/Reise, Reise/2 of 2/Dalai Lama.flac`, path)
	})

	t.Run("conditional disc folder", func(t *testing.T) {
		t.Parallel()
		var pf pathformat.Format
		require.NoError(t, pf.Parse(`/music/{{ .Release.Title }}{{if gt .TotalDiscs 1}}/Disc {{ .DiscNum }}{{end}}/{{ .Track.Title }}{{ .Ext }}`))

		// Multi-disc: should include disc folder
		path, err := pf.Execute(release, 0, ".flac")
		require.NoError(t, err)
		assert.Equal(t, `/music/Reise, Reise/Disc 1/Reise, Reise.flac`, path)

		// Single-disc release: should NOT include disc folder
		singleDiscRelease := &musicbrainz.Release{
			Title: "Single Album",
			ReleaseGroup: musicbrainz.ReleaseGroup{
				FirstReleaseDate: musicbrainz.AnyTime{Time: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)},
			},
			Artists: []musicbrainz.ArtistCredit{
				{Name: "Artist", Artist: musicbrainz.Artist{Name: "Artist"}},
			},
			Media: []musicbrainz.Media{
				{
					Position: 1,
					Format:   "CD",
					Tracks: []musicbrainz.Track{
						{Title: "Track One", Position: 1, DiscNumber: 1, DiscFormat: "CD"},
					},
				},
			},
		}

		path, err = pf.Execute(singleDiscRelease, 0, ".flac")
		require.NoError(t, err)
		assert.Equal(t, `/music/Single Album/Track One.flac`, path)
	})

	t.Run("disc renumbering after filtering", func(t *testing.T) {
		t.Parallel()

		// Release with CD, DVD (filtered), CD pattern
		releaseWithVideo := &musicbrainz.Release{
			Title: "Deluxe Edition",
			ReleaseGroup: musicbrainz.ReleaseGroup{
				FirstReleaseDate: musicbrainz.AnyTime{Time: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)},
			},
			Artists: []musicbrainz.ArtistCredit{
				{Name: "Artist", Artist: musicbrainz.Artist{Name: "Artist"}},
			},
			Media: []musicbrainz.Media{
				{
					Position: 1,
					Format:   "CD",
					Tracks: []musicbrainz.Track{
						{Title: "Audio Track 1", Position: 1, DiscNumber: 1, DiscFormat: "CD"},
					},
				},
				// DVD will be filtered by FlatTracks
				{
					Position: 2,
					Format:   "DVD",
					Tracks: []musicbrainz.Track{
						{Title: "Video Track", Position: 1, DiscNumber: 2, DiscFormat: "DVD"},
					},
				},
				{
					Position: 3,
					Format:   "CD",
					Tracks: []musicbrainz.Track{
						{Title: "Audio Track 2", Position: 1, DiscNumber: 2, DiscFormat: "CD"}, // Should be disc 2, not 3
					},
				},
			},
		}

		var pf pathformat.Format
		require.NoError(t, pf.Parse(`/music/{{ .Release.Title }}/Disc {{ .DiscNum }}/{{ .Track.Title }}{{ .Ext }}`))

		// First CD track should be disc 1
		path, err := pf.Execute(releaseWithVideo, 0, ".flac")
		require.NoError(t, err)
		assert.Equal(t, `/music/Deluxe Edition/Disc 1/Audio Track 1.flac`, path)

		// Second CD track should be disc 2 (not disc 3, because DVD was filtered)
		path, err = pf.Execute(releaseWithVideo, 1, ".flac")
		require.NoError(t, err)
		assert.Equal(t, `/music/Deluxe Edition/Disc 2/Audio Track 2.flac`, path)
	})
}
