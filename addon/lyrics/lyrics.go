package lyrics

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.senan.xyz/wrtag/addon"
	"go.senan.xyz/wrtag/lyrics"
	"go.senan.xyz/wrtag/tags"
)

func init() {
	addon.Register("lyrics", NewLyricsAddon)
}

type LyricsAddon struct {
	source lyrics.Source
}

func NewLyricsAddon(conf string) (LyricsAddon, error) {
	var sources lyrics.MultiSource
	for _, arg := range strings.Fields(conf) {
		source, err := lyrics.NewSource(arg, 500*time.Millisecond)
		if err != nil {
			return LyricsAddon{}, fmt.Errorf("source %q: %w", arg, err)
		}
		sources = append(sources, source)
	}
	if len(sources) == 0 {
		return LyricsAddon{}, errors.New("no lyrics sources provided")
	}

	return LyricsAddon{sources}, nil
}

func (l LyricsAddon) ProcessRelease(ctx context.Context, paths []string) error {
	var wg sync.WaitGroup

	var pathErrs = make([]error, len(paths))
	for i, path := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pathErrs[i] = func() error {
				t, err := tags.ReadTags(path)
				if err != nil {
					return fmt.Errorf("read first: %w", err)
				}
				if t.Get(tags.Lyrics) != "" {
					return nil
				}
				lyricData, err := l.source.Search(ctx, t.Get(tags.ArtistCredit), t.Get(tags.Title))
				if err != nil && !errors.Is(err, lyrics.ErrLyricsNotFound) {
					return err
				}
				if err := tags.WriteTags(path, tags.NewTags(tags.Lyrics, lyricData)); err != nil {
					return fmt.Errorf("write new lyrics: %w", err)
				}
				return nil
			}()
		}()
	}

	wg.Wait()

	return errors.Join(pathErrs...)
}

func (l LyricsAddon) String() string {
	return fmt.Sprintf("lyrics (%s)", l.source)
}
