package tagmap

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"
	"unicode"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/sync/errgroup"

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

func DiffRelease(weights TagWeights, release *musicbrainz.Release, files []*tags.File) (float64, []Diff) {
	if len(files) == 0 {
		return 0, nil
	}
	f := files[0]

	labelInfo := musicbrainz.AnyLabelInfo(release)

	var score float64
	diff := Differ(weights, &score)

	var diffs []Diff
	diffs = append(diffs,
		diff("release", f.Read(tags.Album), release.Title),
		diff("artist", f.Read(tags.AlbumArtist), musicbrainz.ArtistsString(release.Artists)),
		diff("label", f.Read(tags.Label), labelInfo.Label.Name),
		diff("catalogue num", f.Read(tags.CatalogueNum), labelInfo.CatalogNumber),
		diff("media format", f.Read(tags.MediaFormat), release.Media[0].Format),
	)

	rtracks := musicbrainz.FlatTracks(release.Media)
	if len(rtracks) != len(files) {
		panic(fmt.Errorf("len(rtracks) != len(files)"))
	}

	for i, f := range files {
		diffs = append(diffs, diff(
			fmt.Sprintf("track %d", i+1),
			strings.Join(deleteZero(f.Read(tags.Artist), f.Read(tags.Title)), " – "),
			strings.Join(deleteZero(musicbrainz.ArtistsString(rtracks[i].Artists), rtracks[i].Title), " – "),
		))
	}

	return score, diffs
}

type Addon interface {
	ProcessTrack(context.Context, *musicbrainz.Release, *musicbrainz.Track, *tags.File) error
}

func WriteFile(
	ctx context.Context,
	addons []Addon,
	release *musicbrainz.Release, labelInfo musicbrainz.LabelInfo, genres []musicbrainz.Genre,
	releaseTrack *musicbrainz.Track, i int, f *tags.File,
) error {
	before := f.CloneRaw()
	f.ClearAll()

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
	f.Write(tags.Date, release.Date.Format(time.DateOnly))
	f.Write(tags.OriginalDate, release.ReleaseGroup.FirstReleaseDate.Format(time.DateOnly))
	f.Write(tags.MediaFormat, release.Media[0].Format)
	f.Write(tags.Label, labelInfo.Label.Name)
	f.Write(tags.CatalogueNum, labelInfo.CatalogNumber)

	f.Write(tags.MBReleaseID, release.ID)
	f.Write(tags.MBReleaseGroupID, release.ReleaseGroup.ID)
	f.Write(tags.MBAlbumArtistID, mapFunc(release.Artists, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.ID })...)
	f.Write(tags.MBAlbumComment, disambiguation)

	f.Write(tags.Title, releaseTrack.Title)
	f.Write(tags.Artist, musicbrainz.ArtistsString(releaseTrack.Artists))
	f.Write(tags.Artists, musicbrainz.ArtistsNames(releaseTrack.Artists)...)
	f.Write(tags.ArtistCredit, musicbrainz.ArtistsCreditString(releaseTrack.Artists))
	f.Write(tags.ArtistsCredit, musicbrainz.ArtistsCreditNames(releaseTrack.Artists)...)
	f.Write(tags.Genre, cmp.Or(genreNames...))
	f.Write(tags.Genres, genreNames...)
	f.WriteNum(tags.TrackNumber, i+1)
	f.WriteNum(tags.DiscNumber, 1)

	f.Write(tags.MBRecordingID, releaseTrack.Recording.ID)
	f.Write(tags.MBArtistID, mapFunc(releaseTrack.Artists, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.ID })...)

	var wg errgroup.Group
	for _, addon := range addons {
		wg.Go(func() error {
			return addon.ProcessTrack(ctx, release, releaseTrack, f)
		})
	}
	if err := wg.Wait(); err != nil {
		return fmt.Errorf("run addons: %w", err)
	}

	// try to avoid extra filesystem writes if we can
	after := f.CloneRaw()
	if maps.EqualFunc(before, after, slices.Equal) {
		return nil
	}

	if l := slog.Default(); l.Enabled(context.Background(), slog.LevelDebug) {
		for k := range after {
			if b, a := before[k], after[k]; !slices.Equal(b, a) {
				l.Debug("tag change", "key", k, "from", b, "to", a)
			}
		}
	}

	if err := f.Save(); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	return nil
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
