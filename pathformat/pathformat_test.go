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
	_, err := pf.Execute(pathformat.Data{})
	assert.Error(t, err) // we didn't initalise with Parse() yet

	// bad/ambiguous format
	assert.ErrorIs(t, pf.Parse(""), pathformat.ErrInvalidFormat)
	assert.ErrorIs(t, pf.Parse(" "), pathformat.ErrInvalidFormat)
	assert.ErrorIs(t, pf.Parse("🤤"), pathformat.ErrInvalidFormat)

	assert.ErrorIs(t, pf.Parse(`/albums/test/{{ artists .Release.Artists | join " " }}/{{ .Release.Title }}`), pathformat.ErrAmbiguousFormat)
	assert.ErrorIs(t, pf.Parse(`/albums/test/{{ .Track.Title }}`), pathformat.ErrAmbiguousFormat)
	assert.ErrorIs(t, pf.Parse(`/albums/test/{{ .TrackNum }}`), pathformat.ErrAmbiguousFormat)

	// bad data
	assert.ErrorIs(t, pf.Parse(`/albums/test/{{ artists .Release.Artists | join " " }}/{{ .Release.ID }}/`), pathformat.ErrBadData)                   // test case is missing ID
	assert.ErrorIs(t, pf.Parse(`/albums/test/{{ artists .Release.Artists | join " " }}//`), pathformat.ErrBadData)                                    // double slash anyway
	assert.ErrorIs(t, pf.Parse(`/albums/test/{{ artists .Release.Artists | join " " }}/{{ .Release.Title }}/{{ .Track.ID }}`), pathformat.ErrBadData) // implicit trailing slash from missing ID
	assert.ErrorIs(t, pf.Parse(`/albums/test/{{ .Track.ID }}/`), pathformat.ErrBadData)                                                               //

	// good
	assert.NoError(t, pf.Parse(`/albums/test/{{ artists .Release.Artists | join " " }}/{{ .Release.Title }}/{{ .TrackNum }}`))
	assert.Equal(t, "/albums/test", pf.Root())
}

func TestPathFormat(t *testing.T) {
	t.Parallel()

	var pf pathformat.Format
	require.NoError(t, pf.Parse(`/music/albums/{{ artists .Release.Artists | sort | join "; " | safepath }}/({{ .Release.ReleaseGroup.FirstReleaseDate.Year }}) {{ .Release.Title | safepath }}{{ if not (eq (disambig .Release) "") }} ({{ disambig .Release | safepath }}){{ end }}/{{ pad0 2 .TrackNum }}.{{ flatTracks .Release.Media | len | pad0 2 }} {{ .Track.Title | safepath }}{{ .Ext }}`))

	track := musicbrainz.Track{
		Title: "Sharon's Tone",
	}
	release := musicbrainz.Release{
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

	path, err := pf.Execute(pathformat.Data{Release: &release, Track: &track, TrackNum: 1, Ext: ".flac"})
	require.NoError(t, err)
	assert.Equal(t, `/music/albums/Luke Vibert/(2018) Valvable/01.01 Sharon's Tone.flac`, path)

	release.ReleaseGroup.Disambiguation = "Deluxe Edition"

	path, err = pf.Execute(pathformat.Data{Release: &release, Track: &track, TrackNum: 1, Ext: ".flac"})
	require.NoError(t, err)
	assert.Equal(t, `/music/albums/Luke Vibert/(2018) Valvable (Deluxe Edition)/01.01 Sharon's Tone.flac`, path)
}
