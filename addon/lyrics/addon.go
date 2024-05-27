package lyrics

import (
	"context"
	"errors"

	"go.senan.xyz/wrtag/tags"
)

type Addon struct {
	Source
}

func (a Addon) ProcessRelease(ctx context.Context, paths []string) error {
	var trackErrs []error
	for _, path := range paths {
		err := tags.Write(path, func(f *tags.File) error {
			lyricData, err := a.Search(ctx, f.Read(tags.ArtistCredit), f.Read(tags.Title))
			if err != nil && !errors.Is(err, ErrLyricsNotFound) {
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

func (a Addon) Name() string {
	return "lyrics"
}
