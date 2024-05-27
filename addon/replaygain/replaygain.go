package replaygain

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os/exec"
	"strconv"
)

var ErrNoRsgain = fmt.Errorf("rsgain not found in PATH")

const RsgainCommand = "rsgain"

type Loudness struct {
	GaindB, Peak float64
}

func Calculate(ctx context.Context, truePeak bool, trackPaths []string) (Loudness, []Loudness, error) {
	if _, err := exec.LookPath(RsgainCommand); err != nil {
		return Loudness{}, nil, fmt.Errorf("%w: %w", ErrNoRsgain, err)
	}
	if len(trackPaths) == 0 {
		return Loudness{}, nil, nil
	}

	cmd := exec.CommandContext(ctx,
		RsgainCommand,
		append([]string{"custom", "--output", "--tagmode", "s", "--album"}, trackPaths...)...,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Loudness{}, nil, fmt.Errorf("get stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return Loudness{}, nil, fmt.Errorf("start cmd: %w", err)
	}

	reader := csv.NewReader(stdout)
	reader.Comma = '\t'
	reader.ReuseRecord = true

	if _, err := reader.Read(); err != nil {
		return Loudness{}, nil, fmt.Errorf("read header: %w", err)
	}

	album := Loudness{}
	tracks := make([]Loudness, 0, len(trackPaths))
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Loudness{}, nil, fmt.Errorf("read line: %w", err)
		}
		if len(record) != NumColumns {
			return Loudness{}, nil, fmt.Errorf("num columns mismatch %d / %d", len(record), NumColumns)
		}

		var m Loudness
		if m.GaindB, err = strconv.ParseFloat(record[GaindB], 64); err != nil {
			return Loudness{}, nil, fmt.Errorf("read gain dB: %w", err)
		}
		if m.Peak, err = strconv.ParseFloat(record[Peak], 64); err != nil {
			return Loudness{}, nil, fmt.Errorf("read peak: %w", err)
		}

		switch record[Filename] {
		case "Album":
			album = m
		default:
			tracks = append(tracks, m)
		}
	}
	if err := cmd.Wait(); err != nil {
		return Loudness{}, nil, fmt.Errorf("wait cmd: %w", err)
	}

	if len(tracks) != len(trackPaths) {
		return Loudness{}, nil, fmt.Errorf("num tracks mismatch %d / %d", len(tracks), len(trackPaths))
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
	NumColumns
)
