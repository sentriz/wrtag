package addons

import (
	"context"
	"errors"

	"go.senan.xyz/wrtag/lyrics"
	"go.senan.xyz/wrtag/tags"
)

type Addon interface {
	ProcessRelease(context.Context, []string) error
	Name() string
}

type Lyrics struct {
	lyrics.Source
}

func (a Lyrics) ProcessRelease(ctx context.Context, paths []string) error {
	var trackErrs []error
	for _, path := range paths {
		err := tags.Write(path, func(f *tags.File) error {
			lyricData, err := a.Search(ctx, f.Read(tags.ArtistCredit), f.Read(tags.Title))
			if err != nil && !errors.Is(err, lyrics.ErrLyricsNotFound) {
				return err
			}

			f.Write(tags.Lyrics, lyricData)
			return nil
		})
		if err != nil {
			trackErrs = append(trackErrs, err)
			continue
		}
	}
	return errors.Join(trackErrs...)
}

func (a Lyrics) Name() string {
	return "lyrics"
}
