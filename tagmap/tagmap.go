package tagmap

import (
	"cmp"
	"fmt"
	"slices"
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

type MatchedTrack struct {
	*musicbrainz.Track
	*tags.File
}

func DiffRelease(weights TagWeights, release *musicbrainz.Release, tracks []MatchedTrack) (float64, []Diff) {
	if len(tracks) == 0 {
		return 0, nil
	}

	labelInfo := musicbrainz.AnyLabelInfo(release)

	var score float64
	diff := Differ(weights, &score)

	var diffs []Diff
	{
		firstTr := tracks[0]
		diffs = append(diffs,
			diff("release", firstTr.Read(tags.Album), release.Title),
			diff("artist", firstTr.Read(tags.AlbumArtist), musicbrainz.ArtistsString(release.Artists)),
			diff("label", firstTr.Read(tags.Label), labelInfo.Label.Name),
			diff("catalogue num", firstTr.Read(tags.CatalogueNum), labelInfo.CatalogNumber),
			diff("media format", firstTr.Read(tags.MediaFormat), release.Media[0].Format),
		)
	}

	for i, track := range tracks {
		diffs = append(diffs, diff(
			fmt.Sprintf("track %d", i+1),
			strings.Join(deleteZero(track.Read(tags.Artist), track.Read(tags.Title)), " – "),
			strings.Join(deleteZero(musicbrainz.ArtistsString(track.Artists), track.Title), " – "),
		))
	}

	return score, diffs
}

func WriteTo(
	release *musicbrainz.Release, labelInfo musicbrainz.LabelInfo, genres []musicbrainz.Genre,
	i int, trk *musicbrainz.Track, f *tags.File,
) {
	f.ClearUnknown()

	var genreNames []string
	for _, g := range genres[:min(6, len(genres))] { // top 6 genre strings
		genreNames = append(genreNames, g.Name)
	}

	disambiguationParts := deleteZero(release.ReleaseGroup.Disambiguation, release.Disambiguation)
	disambiguation := strings.Join(disambiguationParts, ", ")

	f.Write(tags.Album, release.Title)
	f.Write(tags.AlbumArtist, musicbrainz.ArtistsString(release.Artists))
	f.Write(tags.AlbumArtists, musicbrainz.ArtistsNames(release.Artists)...)
	f.Write(tags.AlbumArtistCredit, musicbrainz.ArtistsCreditString(release.Artists))
	f.Write(tags.AlbumArtistsCredit, musicbrainz.ArtistsCreditNames(release.Artists)...)
	f.Write(tags.Date, formatDate(release.Date.Time))
	f.Write(tags.OriginalDate, formatDate(release.ReleaseGroup.FirstReleaseDate.Time))
	f.Write(tags.MediaFormat, release.Media[0].Format)
	f.Write(tags.Label, labelInfo.Label.Name)
	f.Write(tags.CatalogueNum, labelInfo.CatalogNumber)
	f.Write(tags.UPC, release.Barcode)

	f.Write(tags.MBReleaseID, release.ID)
	f.Write(tags.MBReleaseGroupID, release.ReleaseGroup.ID)
	f.Write(tags.MBAlbumArtistID, mapFunc(release.Artists, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.ID })...)
	f.Write(tags.MBAlbumComment, disambiguation)

	f.Write(tags.Title, trk.Title)
	f.Write(tags.Artist, musicbrainz.ArtistsString(trk.Artists))
	f.Write(tags.Artists, musicbrainz.ArtistsNames(trk.Artists)...)
	f.Write(tags.ArtistCredit, musicbrainz.ArtistsCreditString(trk.Artists))
	f.Write(tags.ArtistsCredit, musicbrainz.ArtistsCreditNames(trk.Artists)...)
	f.Write(tags.Genre, cmp.Or(genreNames...))
	f.Write(tags.Genres, genreNames...)
	f.WriteNum(tags.TrackNumber, i+1)
	f.WriteNum(tags.DiscNumber, 1)

	f.Write(tags.MBRecordingID, trk.Recording.ID)
	f.Write(tags.MBArtistID, mapFunc(trk.Artists, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.ID })...)
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

func deleteZero[T comparable](elms ...T) []T {
	var zero T
	return slices.DeleteFunc(elms, func(t T) bool { return t == zero })
}
