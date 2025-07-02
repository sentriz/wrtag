package replaygain

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.senan.xyz/wrtag/addon"
	"go.senan.xyz/wrtag/rsgain"
	"go.senan.xyz/wrtag/tags"
)

func init() {
	addon.Register("replaygain", NewReplayGainAddon)
}

type ReplayGainAddon struct {
	truePeak bool
	force    bool
}

func NewReplayGainAddon(conf string) (ReplayGainAddon, error) {
	var a ReplayGainAddon
	for arg := range strings.FieldsSeq(conf) {
		switch arg {
		case "true-peak":
			a.truePeak = true
		case "force":
			a.force = true
		default:
			return ReplayGainAddon{}, fmt.Errorf("unknown option %q", arg)
		}
	}
	return a, nil
}

func (a ReplayGainAddon) ProcessRelease(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	if !a.force {
		first, err := tags.ReadTags(paths[0])
		if err != nil {
			return fmt.Errorf("read first file: %w", err)
		}
		if first.Get(tags.ReplayGainTrackGain) != "" {
			return nil
		}
	}

	albumLev, pathLevs, err := rsgain.Calculate(ctx, a.truePeak, paths)
	if err != nil {
		return fmt.Errorf("calculate: %w", err)
	}

	var pathErrs []error
	for i := range paths {
		pathL, path := pathLevs[i], paths[i]

		t := tags.NewTags(
			tags.ReplayGainTrackGain, fmtdB(pathL.GaindB),
			tags.ReplayGainTrackPeak, fmtFloat(pathL.Peak, 6),
			tags.ReplayGainAlbumGain, fmtdB(albumLev.GaindB),
			tags.ReplayGainAlbumPeak, fmtFloat(albumLev.Peak, 6),
		)
		if err := tags.WriteTags(path, t, 0); err != nil {
			pathErrs = append(pathErrs, err)
			continue
		}
	}

	return errors.Join(pathErrs...)
}

func (a ReplayGainAddon) String() string {
	return fmt.Sprintf("replaygain (force: %t, true peak: %t)", a.force, a.truePeak)
}

func fmtFloat(v float64, p int) string {
	return strconv.FormatFloat(v, 'f', p, 64)
}
func fmtdB(v float64) string {
	return fmt.Sprintf("%.2f dB", v)
}
