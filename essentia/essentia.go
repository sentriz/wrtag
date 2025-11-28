// Package essentia provides a wrapper for the streaming_extractor_music tool
// from the Essentia audio analysis library, enabling extraction of BPM and key information.
package essentia

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
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

	slog.DebugContext(ctx, "starting subprocess", "command", cmd.Args)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start cmd: %w", err)
	}

	// streaming_extractor_music outputs progress messages and warnings to stdout before the JSON output
	reader, err := skipToJSON(stdout)
	if err != nil {
		return nil, fmt.Errorf("skip to json: %w", err)
	}

	if err := json.NewDecoder(reader).Decode(&info); err != nil {
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

func skipToJSON(r io.Reader) (io.Reader, error) {
	for br := bufio.NewReader(r); ; {
		line, err := br.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return nil, err
		}
		// check for '{'
		if trimmed := bytes.TrimLeft(line, " \t"); len(trimmed) > 0 && trimmed[0] == '{' {
			return io.MultiReader(bytes.NewReader(line), br), nil
		}
		if err == io.EOF {
			return nil, errors.New("no JSON found in output")
		}
	}
}
