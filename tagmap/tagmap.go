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

type Weights map[string]float64

func DiffRelease[T interface{ Get(string) string }](weights Weights, release *musicbrainz.Release, tracks []musicbrainz.Track, tagFiles []T) (float64, []Diff) {
	if len(tracks) == 0 {
		return 0, nil
	}

	labelInfo := musicbrainz.AnyLabelInfo(release)

	var score float64
	diff := Differ(&score)

	weight := func(t string) float64 {
		if w, ok := weights[t]; ok {
			return w
		}
		return 1
	}

	var diffs []Diff
	{
		tf := tagFiles[0]
		diffs = append(diffs,
			diff(weight("release"), "release", tf.Get(tags.Album), release.Title),
			diff(weight("artist"), "artist", tf.Get(tags.AlbumArtist), musicbrainz.ArtistsString(release.Artists)),
			diff(weight("label"), "label", tf.Get(tags.Label), labelInfo.Label.Name),
			diff(weight("catalogue num"), "catalogue num", tf.Get(tags.CatalogueNum), labelInfo.CatalogNumber),
			diff(weight("upc"), "upc", tf.Get(tags.UPC), release.Barcode),
			diff(weight("media format"), "media format", tf.Get(tags.MediaFormat), release.Media[0].Format),
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
		diffs = append(diffs, diff(weight("track"), fmt.Sprintf("track %d", i+1), a, b))
	}

	return score, diffs
}

func WriteRelease(
	t tags.Tags,
	release *musicbrainz.Release, labelInfo musicbrainz.LabelInfo, genres []musicbrainz.Genre,
	i int, trk *musicbrainz.Track,
) {
	var genreNames []string
	for _, g := range genres[:min(6, len(genres))] { // top 6 genre strings
		genreNames = append(genreNames, g.Name)
	}

	disambiguationParts := trim(release.ReleaseGroup.Disambiguation, release.Disambiguation)
	disambiguation := strings.Join(disambiguationParts, ", ")

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
	t.Set(tags.ReleaseType, trim(strings.ToLower(string(release.ReleaseGroup.PrimaryType)))...)

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
	t.Set(tags.MBTrackID, trim(trk.ID)...)
	t.Set(tags.MBArtistID, trim(mapFunc(trk.Artists, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.ID })...)...)
}

var (
	dm = dmp.New()
)

func Differ(score *float64) func(weight float64, field string, a, b string) Diff {
	var total float64
	var totalDist float64

	return func(w float64, field, a, b string) Diff {
		diffs := dm.DiffMain(a, b, false)

		var d Diff
		d.Field = field
		d.Before = filterFunc(diffs, func(d dmp.Diff) bool { return d.Type <= dmp.DiffEqual })
		d.After = filterFunc(diffs, func(d dmp.Diff) bool { return d.Type >= dmp.DiffEqual })
		d.Equal = a == b

		if a == "" || b == "" {
			return d
		}

		// separate, norm diff for score calculation. only if we have both fields
		aNorm, bNorm := norm(a), norm(b)

		diffs = dm.DiffMain(aNorm, bNorm, false)
		totalDist += float64(dm.DiffLevenshtein(diffs)) * w
		total += float64(max(len([]rune(aNorm)), len([]rune(bNorm))))

		*score = 100 - (totalDist * 100 / total)

		return d
	}
}

type Config struct {
	Keep []string
	Drop []string
}

func ApplyConfig(
	dest, source tags.Tags,
	conf Config,
) {
	for _, k := range defaultKeepConfig {
		dest.Set(k, source.Values(k)...)
	}
	for _, k := range conf.Keep {
		dest.Set(k, source.Values(k)...)
	}
	for _, k := range conf.Drop {
		dest.Set(k)
	}
}

// defaultKeepConfig is set of tags which are kept as-is when replacing tags.
var defaultKeepConfig = []string{
	tags.ReplayGainTrackGain,
	tags.ReplayGainTrackPeak,
	tags.ReplayGainAlbumGain,
	tags.ReplayGainAlbumPeak,
	tags.BPM,
	tags.Lyrics,
	tags.AcoustIDFingerprint,
	tags.AcoustIDID,
	tags.Encoder,
	tags.EncodedBy,
	tags.Comment,
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
	var res = make([]T, 0, len(elms))
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
