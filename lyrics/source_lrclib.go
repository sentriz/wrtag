package lyrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"go.senan.xyz/wrtag/clientutil"
)

var lrclibBaseURL = `https://lrclib.net/api/get`

type LRCLib struct {
	RateLimit time.Duration

	initOnce   sync.Once
	HTTPClient *http.Client
}

func (l *LRCLib) Search(ctx context.Context, artist, song string, duration time.Duration) (string, error) {
	l.initOnce.Do(func() {
		l.HTTPClient = clientutil.Wrap(l.HTTPClient, clientutil.Chain(
			clientutil.WithRateLimit(l.RateLimit),
		))
	})

	u, _ := url.Parse(lrclibBaseURL)
	q := u.Query()
	q.Set("artist_name", artist)
	q.Set("track_name", song)
	if duration > 0 {
		q.Set("duration", fmt.Sprintf("%.0f", duration.Seconds()))
	}
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	resp, err := l.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("req page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", ErrTrackNotFound
	}
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result lrclibResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	// prefer synced lyrics if available, otherwise fall back to plain lyrics
	if result.SyncedLyrics != "" {
		return result.SyncedLyrics, nil
	}
	if result.PlainLyrics != "" {
		return result.PlainLyrics, nil
	}

	return "", nil
}

func (l *LRCLib) String() string {
	return "lrclib"
}

type lrclibResponse struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}
