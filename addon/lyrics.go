package addon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.senan.xyz/wrtag/addon/lyrics"
	"go.senan.xyz/wrtag/tags"
)

type LyricsAddon struct {
	source lyrics.Source
}

func NewLyricsAddon(conf string) (LyricsAddon, error) {
	var sources lyrics.MultiSource
	for _, arg := range strings.Fields(conf) {
		switch arg {
		case "genius":
			sources = append(sources, &lyrics.Genius{RateLimit: 500 * time.Millisecond})
		case "musixmatch":
			sources = append(sources, &lyrics.Musixmatch{RateLimit: 500 * time.Millisecond})
		default:
			return LyricsAddon{}, fmt.Errorf("unknown lyrics source %q", arg)
		}
	}
	if len(sources) == 0 {
		return LyricsAddon{}, fmt.Errorf("no lyrics sources provided")
	}
	var a LyricsAddon
	a.source = sources
	return a, nil
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

				lyricData, err := l.source.Search(ctx, f.Read(tags.ArtistCredit), f.Read(tags.Title))
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

func (l LyricsAddon) String() string {
	return fmt.Sprintf("lyrics (%s)", l.source)
}
