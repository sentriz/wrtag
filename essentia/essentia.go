package essentia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
)

var ErrNoStreamingExtractorMusic = errors.New("streaming_extractor_music not found in PATH")

const StreamingExtractorMusicCommand = "streaming_extractor_music"

func Read(ctx context.Context, path string) (info *Info, err error) {
	if _, err := exec.LookPath(StreamingExtractorMusicCommand); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoStreamingExtractorMusic, err)
	}

	cmd := exec.CommandContext(ctx, StreamingExtractorMusicCommand, path, "-")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	defer func() {
		if err != nil && stderr.Len() > 0 {
			err = fmt.Errorf("%w: stderr: %q", err, stderr.String())
		}
	}()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("get stdout pipe: %w", err)
	}
	defer stdout.Close()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start cmd: %w", err)
	}

	if err := json.NewDecoder(stdout).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}

	return info, nil
}

type Info struct {
	Rhythm struct {
		BPM float64 `json:"bpm"`
	} `json:"rhythm"`
	Tonal struct {
		KeyKey   string `json:"key_key"`
		KeyScale string `json:"key_scale"`
	} `json:"tonal"`
}
