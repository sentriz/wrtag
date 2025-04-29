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

	var pf pathformat.Format
	require.NoError(t, pf.Parse(`/music/albums/{{ artists .Release.Artists | sort | join "; " | safepath }}/({{ .Release.ReleaseGroup.FirstReleaseDate.Year }}) {{ .Release.Title | safepath }}{{ if not (eq .ReleaseDisambiguation "") }} ({{ .ReleaseDisambiguation | safepath }}){{ end }}/{{ pad0 2 .TrackNum }}.{{ len .Tracks | pad0 2 }} {{ .Track.Title | safepath }}{{ .Ext }}`))

	track := musicbrainz.Track{
		Title: "Sharon's Tone",
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

	path, err := pf.Execute(release, 0, ".flac")
	require.NoError(t, err)
	assert.Equal(t, `/music/albums/Luke Vibert/(2018) Valvable/01.01 Sharon's Tone.flac`, path)

	release.ReleaseGroup.Disambiguation = "Deluxe Edition"

	path, err = pf.Execute(release, 0, ".flac")
	require.NoError(t, err)
	assert.Equal(t, `/music/albums/Luke Vibert/(2018) Valvable (Deluxe Edition)/01.01 Sharon's Tone.flac`, path)
}
