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
