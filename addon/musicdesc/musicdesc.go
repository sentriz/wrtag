package musicdesc

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"go.senan.xyz/wrtag/addon"
	"go.senan.xyz/wrtag/essentia"
	"go.senan.xyz/wrtag/tags"
)

func init() {
	addon.Register("musicdesc", NewMusicDescAddon)
}

type MusicDescAddon struct {
	force bool
}

func NewMusicDescAddon(conf string) (MusicDescAddon, error) {
	var a MusicDescAddon
	for arg := range strings.FieldsSeq(conf) {
		switch arg {
		case "force":
			a.force = true
		default:
			return MusicDescAddon{}, fmt.Errorf("unknown option %q", arg)
		}
	}
	return a, nil
}

func (a MusicDescAddon) ProcessRelease(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	if !a.force {
		first, err := tags.ReadTags(paths[0])
		if err != nil {
			return fmt.Errorf("read first file: %w", err)
		}
		if first.Get(tags.BPM) != "" && first.Get(tags.Key) != "" {
			return nil
		}
	}

	var wg sync.WaitGroup
	var sem = make(chan struct{}, max(runtime.NumCPU()/2, 1))

	var pathErrs = make([]error, len(paths))
	for i, path := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			pathErrs[i] = func() error {
				info, err := essentia.Read(ctx, path)
				if err != nil {
					return fmt.Errorf("read essentia: %w", err)
				}

				t := tags.NewTags(
					tags.BPM, fmtBPM(info.Rhythm.BPM),
					tags.Key, fmtKey(info.Tonal.KeyKey, info.Tonal.KeyScale),
				)
				if err := tags.WriteTags(path, t, 0); err != nil {
					return fmt.Errorf("write new tags: %w", err)
				}
				return nil
			}()
		}()
	}

	wg.Wait()

	return errors.Join(pathErrs...)
}

func (a MusicDescAddon) String() string {
	return fmt.Sprintf("musicdesc (force: %t)", a.force)
}

func fmtBPM(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
func fmtKey(k string, kscale string) string {
	switch kscale {
	case "minor":
		return k + "m"
	case "major":
		return k
	default:
		return k + kscale
	}
}
