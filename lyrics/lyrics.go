// Package lyrics provides functionality for fetching song lyrics from various sources.
// It supports multiple lyrics providers including LRCLib, Genius, and Musixmatch.
package lyrics

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type Source interface {
	Search(ctx context.Context, artist, song string, duration time.Duration) (string, error)
}

func NewSource(name string) (Source, error) {
	switch name {
	case "genius":
		return &Genius{RateLimit: 500 * time.Millisecond}, nil
	case "musixmatch":
		return &Musixmatch{RateLimit: 500 * time.Millisecond}, nil
	case "lrclib":
		return &LRCLib{RateLimit: 100 * time.Millisecond}, nil
	default:
		return nil, errors.New("unknown source")
	}
}

var ErrTrackNotFound = errors.New("track not found")

type MultiSource []Source

func (ms MultiSource) Search(ctx context.Context, artist, song string, duration time.Duration) (string, error) {
	for _, src := range ms {
		lyricData, err := src.Search(ctx, artist, song, duration)
		if err != nil && !errors.Is(err, ErrTrackNotFound) {
			return "", err
		}
		if lyricData != "" {
			return lyricData, nil
		}
		// if we got empty lyrics without ErrTrackNotFound, the track was found but has no lyrics
		// stop trying other sources
		if err == nil {
			return "", nil
		}
	}
	return "", ErrTrackNotFound
}

func (ms MultiSource) String() string {
	var parts []string
	for _, s := range ms {
		parts = append(parts, fmt.Sprint(s))
	}
	return strings.Join(parts, ", ")
}

func findDocumentText(n *html.Node, buf *strings.Builder) {
	if n == nil {
		return
	}

	for n := range n.Descendants() {
		switch n.Type {
		case html.TextNode:
			buf.WriteString(n.Data)
		case html.ElementNode:
			switch n.Data {
			case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "br":
				buf.WriteString("\n")
			}
		}
	}
}
