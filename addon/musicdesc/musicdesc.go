package musicdesc

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

	var trackErrs []error
	for _, path := range paths {
		info, err := essentia.Read(ctx, path)
		if err != nil {
			trackErrs = append(trackErrs, fmt.Errorf("read essentia: %w", err))
			continue
		}

		t := tags.NewTags(
			tags.BPM, fmt.Sprintf("%.2f", info.Rhythm.BPM),
			tags.Key, info.Tonal.KeyKey,
		)
		if err := tags.WriteTags(path, t, 0); err != nil {
			trackErrs = append(trackErrs, err)
			continue
		}
	}

	return errors.Join(trackErrs...)
}

func (a MusicDescAddon) String() string {
	return fmt.Sprintf("musicdesc (force: %t)", a.force)
}
