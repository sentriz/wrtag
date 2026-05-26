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

func TestArtists(t *testing.T) {
	t.Parallel()

	// 菊池桃子 has an English alias; Madonna is already Latin so its expansion alias is ignored.
	credits := []ArtistCredit{
		{Name: "桃子", JoinPhrase: " & ", Artist: Artist{
			Name:     "菊池桃子",
			SortName: "Kikuchi, Momoko",
			Aliases:  []Alias{{Name: "Momoko Kikuchi", Locale: "en"}},
		}},
		{Name: "Madonna", Artist: Artist{
			Name:     "Madonna",
			SortName: "Madonna",
			Aliases:  []Alias{{Name: "Madonna Louise Ciccone", Locale: "en"}},
		}},
	}

	assert.Equal(t, []string{"菊池桃子", "Madonna"}, ArtistsNames(credits))
	assert.Equal(t, "菊池桃子 & Madonna", ArtistsString(credits))
	assert.Equal(t, []string{"Momoko Kikuchi", "Madonna"}, ArtistsEnNames(credits))
	assert.Equal(t, "Momoko Kikuchi & Madonna", ArtistsEnString(credits))
	assert.Equal(t, []string{"桃子", "Madonna"}, ArtistsCreditNames(credits))
	assert.Equal(t, "桃子 & Madonna", ArtistsCreditString(credits))
	assert.Equal(t, []string{"Kikuchi, Momoko", "Madonna"}, ArtistsSortNames(credits))
	assert.Equal(t, "Kikuchi, Momoko & Madonna", ArtistsSortString(credits))
}

func TestReleaseEnTitle(t *testing.T) {
	t.Parallel()
	release := Release{Title: "二度寝", Aliases: []Alias{{Name: "Nidone", Locale: "en"}}}
	assert.Equal(t, "Nidone", ReleaseEnTitle(release))
}

func TestReleaseGroupEnTitle(t *testing.T) {
	t.Parallel()
	rg := ReleaseGroup{Title: "二度寝", Aliases: []Alias{{Name: "Nidone", Locale: "en"}}}
	assert.Equal(t, "Nidone", ReleaseGroupEnTitle(rg))
}

func TestReleaseOrGroupEnTitle(t *testing.T) {
	t.Parallel()
	// the release has no alias, so the English title comes from the release group (#137)
	var release Release
	release.Title = "二度寝"
	release.ReleaseGroup.Title = "二度寝"
	release.ReleaseGroup.Aliases = []Alias{{Name: "Nidone", Locale: "en"}}
	assert.Equal(t, "Nidone", ReleaseOrGroupEnTitle(release))
}
