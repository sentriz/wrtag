package replaygain

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
)

var ErrNoRsgain = fmt.Errorf("rsgain not found in PATH")

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

	cmd := exec.CommandContext(ctx,
		RsgainCommand,
		append([]string{"custom", "--output", "--tagmode", "s", "--album"}, trackPaths...)...,
	)

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
	if err := cmd.Start(); err != nil {
		return Level{}, nil, fmt.Errorf("start cmd: %w", err)
	}

	reader := csv.NewReader(stdout)
	reader.Comma = '\t'
	reader.ReuseRecord = true

	if _, err := reader.Read(); err != nil {
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

	return album, tracks, nil
}

type Column uint8

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
