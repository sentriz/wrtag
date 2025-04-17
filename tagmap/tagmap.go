package tagmap

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	dmp "github.com/sergi/go-diff/diffmatchpatch"

	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/tags"
)

type Diff struct {
	Field         string
	Before, After []dmp.Diff
	Equal         bool
}

type TagWeights map[string]float64

func (tw TagWeights) For(field string) float64 {
	if field == "" {
		return 1
	}
	for f, w := range tw {
		if strings.HasPrefix(field, f) {
			return w
		}
	}
	return 1
}

func DiffRelease[T interface{ Get(string) string }](weights TagWeights, release *musicbrainz.Release, tracks []musicbrainz.Track, tagFiles []T) (float64, []Diff) {
	if len(tracks) == 0 {
		return 0, nil
	}

	labelInfo := musicbrainz.AnyLabelInfo(release)

	var score float64
	diff := Differ(weights, &score)

	var diffs []Diff
	{
		tf := tagFiles[0]
		diffs = append(diffs,
			diff("release", tf.Get(tags.Album), release.Title),
			diff("artist", tf.Get(tags.AlbumArtist), musicbrainz.ArtistsString(release.Artists)),
			diff("label", tf.Get(tags.Label), labelInfo.Label.Name),
			diff("catalogue num", tf.Get(tags.CatalogueNum), labelInfo.CatalogNumber),
			diff("upc", tf.Get(tags.UPC), release.Barcode),
			diff("media format", tf.Get(tags.MediaFormat), release.Media[0].Format),
		)
	}

	for i := range max(len(tagFiles), len(tracks)) {
		var a, b string
		if i < len(tagFiles) {
			a = strings.Join(trim(tagFiles[i].Get(tags.Artist), tagFiles[i].Get(tags.Title)), " – ")
		}
		if i < len(tracks) {
			b = strings.Join(trim(musicbrainz.ArtistsString(tracks[i].Artists), tracks[i].Title), " – ")
		}
		diffs = append(diffs, diff(fmt.Sprintf("track %d", i+1), a, b))
	}

	// we can get negative scores sometimes, just clamp to 0 for now
	score = max(0, score)
	score = min(100, score)

	return score, diffs
}

func ReleaseTags(
	release *musicbrainz.Release, labelInfo musicbrainz.LabelInfo, genres []musicbrainz.Genre,
	i int, trk *musicbrainz.Track,
) tags.Tags {
	var genreNames []string
	for _, g := range genres[:min(6, len(genres))] { // top 6 genre strings
		genreNames = append(genreNames, g.Name)
	}

	disambiguationParts := trim(release.ReleaseGroup.Disambiguation, release.Disambiguation)
	disambiguation := strings.Join(disambiguationParts, ", ")

	var t tags.Tags
	t.Set(tags.Album, trim(release.Title)...)
	t.Set(tags.AlbumArtist, trim(musicbrainz.ArtistsString(release.Artists))...)
	t.Set(tags.AlbumArtists, trim(musicbrainz.ArtistsNames(release.Artists)...)...)
	t.Set(tags.AlbumArtistCredit, trim(musicbrainz.ArtistsCreditString(release.Artists))...)
	t.Set(tags.AlbumArtistsCredit, trim(musicbrainz.ArtistsCreditNames(release.Artists)...)...)
	t.Set(tags.Date, trim(formatDate(release.Date.Time))...)
	t.Set(tags.OriginalDate, trim(formatDate(release.ReleaseGroup.FirstReleaseDate.Time))...)
	t.Set(tags.MediaFormat, trim(release.Media[0].Format)...)
	t.Set(tags.Label, trim(labelInfo.Label.Name)...)
	t.Set(tags.CatalogueNum, trim(labelInfo.CatalogNumber)...)
	t.Set(tags.UPC, trim(release.Barcode)...)
	t.Set(tags.Compilation, trim(formatBool(musicbrainz.IsCompilation(release.ReleaseGroup)))...)

	t.Set(tags.MBReleaseID, trim(release.ID)...)
	t.Set(tags.MBReleaseGroupID, trim(release.ReleaseGroup.ID)...)
	t.Set(tags.MBAlbumArtistID, trim(mapFunc(release.Artists, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.ID })...)...)
	t.Set(tags.MBAlbumComment, trim(disambiguation)...)

	t.Set(tags.Title, trim(trk.Title)...)
	t.Set(tags.Artist, trim(musicbrainz.ArtistsString(trk.Artists))...)
	t.Set(tags.Artists, trim(musicbrainz.ArtistsNames(trk.Artists)...)...)
	t.Set(tags.ArtistCredit, trim(musicbrainz.ArtistsCreditString(trk.Artists))...)
	t.Set(tags.ArtistsCredit, trim(musicbrainz.ArtistsCreditNames(trk.Artists)...)...)
	t.Set(tags.Genre, trim(cmp.Or(genreNames...))...)
	t.Set(tags.Genres, trim(genreNames...)...)
	t.Set(tags.TrackNumber, trim(strconv.Itoa(i+1))...)
	t.Set(tags.DiscNumber, trim(strconv.Itoa(1))...)

	t.Set(tags.MBRecordingID, trim(trk.Recording.ID)...)
	t.Set(tags.MBArtistID, trim(mapFunc(trk.Artists, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.ID })...)...)

	return t
}

func Differ(weights TagWeights, score *float64) func(field string, a, b string) Diff {
	dm := dmp.New()

	var total float64
	var dist float64
	return func(field, a, b string) Diff {
		// separate, normalised diff only for score. if we have both fields
		if a != "" && b != "" {
			a, b := norm(a), norm(b)

			diffs := dm.DiffMain(a, b, false)
			dist += float64(dm.DiffLevenshtein(diffs)) * weights.For(field)
			total += float64(len([]rune(b)))

			*score = 100 - (dist * 100 / total)
		}

		diffs := dm.DiffMain(a, b, false)
		dist := float64(dm.DiffLevenshtein(diffs))
		return Diff{
			Field:  field,
			Before: filterFunc(diffs, func(d dmp.Diff) bool { return d.Type <= dmp.DiffEqual }),
			After:  filterFunc(diffs, func(d dmp.Diff) bool { return d.Type >= dmp.DiffEqual }),
			Equal:  dist == 0,
		}
	}
}

func norm(input string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) {
			return unicode.ToLower(r)
		}
		if unicode.IsNumber(r) {
			return r
		}
		return -1
	}, input)
}

func formatDate(d time.Time) string {
	if d.IsZero() {
		return ""
	}
	return d.Format(time.DateOnly)
}

func formatBool(b bool) string {
	if !b {
		return ""
	}
	return "1"
}

func mapFunc[T, To any](elms []T, f func(int, T) To) []To {
	var res = make([]To, 0, len(elms))
	for i, v := range elms {
		res = append(res, f(i, v))
	}
	return res
}

func filterFunc[T any](elms []T, f func(T) bool) []T {
	var res []T
	for _, el := range elms {
		if f(el) {
			res = append(res, el)
		}
	}
	return res
}

func trim[T comparable](elms ...T) []T {
	var zero T
	return slices.DeleteFunc(elms, func(t T) bool { return t == zero })
}
