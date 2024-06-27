package addon

import (
	"context"
	"errors"
	"sync"

	"go.senan.xyz/wrtag/addon/lyrics"
	"go.senan.xyz/wrtag/tags"
)

type LyricsAddon struct {
	lyrics.Source
}

func (l LyricsAddon) ProcessRelease(ctx context.Context, paths []string) error {
	var pathErrs = make([]error, len(paths))
	var wg sync.WaitGroup
	for i, path := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pathErrs[i] = tags.Write(path, func(f *tags.File) error {
				if f.Read(tags.Lyrics) != "" {
					return nil
				}

				lyricData, err := l.Search(ctx, f.Read(tags.ArtistCredit), f.Read(tags.Title))
				if err != nil && !errors.Is(err, lyrics.ErrLyricsNotFound) {
					return err
				}
				f.Write(tags.Lyrics, lyricData)
				return nil
			})
		}()
	}
	wg.Wait()
	return errors.Join(pathErrs...)
}

func (l LyricsAddon) Name() string {
	return "lyrics"
}
