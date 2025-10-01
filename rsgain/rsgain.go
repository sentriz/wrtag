// Package rsgain provides a wrapper for the rsgain command-line tool
// to calculate ReplayGain values for audio files.
package rsgain

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strconv"
)

var ErrRsgain = errors.New("rsgain error")
var ErrNoRsgain = errors.New("rsgain not found in PATH")

const RsgainCommand = "rsgain"

type Level struct {
	GaindB, Peak float64
}

func Calculate(ctx context.Context, truePeak bool, trackPaths []string) (album Level, tracks []Level, err error) {
	if _, err := exec.LookPath(RsgainCommand); err != nil {
		return Level{}, nil, fmt.Errorf("%w: %w", ErrNoRsgain, err)
	}
	if len(trackPaths) == 0 {
		return Level{}, nil, nil
	}

	var args []string
	args = append(args, "custom")
	args = append(args, "--output")
	args = append(args, "--tagmode", "s")
	if truePeak {
		args = append(args, "--true-peak")
	}
	args = append(args, "--album")
	args = append(args, trackPaths...)

	cmd := exec.CommandContext(ctx, RsgainCommand, args...) //nolint:gosec // args are only args and paths

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	defer func() {
		if err != nil && stderr.Len() > 0 {
			err = fmt.Errorf("%w: stderr: %q", err, stderr.String())
		}
	}()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Level{}, nil, fmt.Errorf("get stdout pipe: %w", err)
	}

	slog.DebugContext(ctx, "starting subprocess", "command", cmd.Args)

	if err := cmd.Start(); err != nil {
		return Level{}, nil, fmt.Errorf("start cmd: %w", err)
	}

	reader := csv.NewReader(stdout)
	reader.Comma = '\t'
	reader.ReuseRecord = true

	if _, err := reader.Read(); err != nil && !errors.Is(err, io.EOF) {
		return Level{}, nil, fmt.Errorf("read header: %w", err)
	}

	for {
		columns, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Level{}, nil, fmt.Errorf("read line: %w", err)
		}
		if len(columns) != numColumns {
			return Level{}, nil, fmt.Errorf("num columns mismatch %d / %d", len(columns), numColumns)
		}

		var gaindB, peak float64
		if gaindB, err = strconv.ParseFloat(columns[GaindB], 64); err != nil {
			return Level{}, nil, fmt.Errorf("read gain dB: %w", err)
		}
		if peak, err = strconv.ParseFloat(columns[Peak], 64); err != nil {
			return Level{}, nil, fmt.Errorf("read peak: %w", err)
		}

		switch columns[Filename] {
		case "Album":
			album.GaindB = gaindB
			album.Peak = peak
		default:
			tracks = append(tracks, Level{GaindB: gaindB, Peak: peak})
		}
	}
	if err := cmd.Wait(); err != nil {
		return Level{}, nil, fmt.Errorf("wait cmd: %w", err)
	}

	if len(tracks) != len(trackPaths) {
		return Level{}, nil, fmt.Errorf("%w: didn't return a level for all tracks", err)
	}

	return album, tracks, nil
}

const (
	Filename = iota
	LoudnessLUFS
	GaindB
	Peak
	PeakdB
	PeakType
	ClippingAdjustment
	numColumns
)
