package addon

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.senan.xyz/wrtag/addon/lyrics"
	"go.senan.xyz/wrtag/addon/replaygain"
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

type ReplayGainAddon struct {
	TruePeak bool
	Force    bool
}

func (a ReplayGainAddon) ProcessRelease(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	if !a.Force {
		existingTag, err := func() (string, error) {
			f, err := tags.Read(paths[0])
			if err != nil {
				return "", err
			}
			defer f.Close()
			return f.Read(tags.ReplayGainTrackGain), nil
		}()
		if err != nil {
			return fmt.Errorf("read first file: %w", err)
		}
		if existingTag != "" {
			return nil
		}
	}

	albumLev, trackLevs, err := replaygain.Calculate(ctx, a.TruePeak, paths)
	if err != nil {
		return fmt.Errorf("calculate: %w", err)
	}

	var trackErrs []error
	for i := range paths {
		trackL, path := trackLevs[i], paths[i]
		err := tags.Write(path, func(f *tags.File) error {
			f.WritedB(tags.ReplayGainTrackGain, trackL.GaindB)
			f.WriteFloat(tags.ReplayGainTrackPeak, trackL.Peak)
			f.WritedB(tags.ReplayGainAlbumGain, albumLev.GaindB)
			f.WriteFloat(tags.ReplayGainAlbumPeak, albumLev.Peak)
			return nil
		})
		if err != nil {
			trackErrs = append(trackErrs, err)
			continue
		}
	}
	return errors.Join(trackErrs...)
}

func (a ReplayGainAddon) Name() string {
	return "replaygain"
}
