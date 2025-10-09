package lyrics

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/cascadia"
	"go.senan.xyz/wrtag/clientutil"
	"golang.org/x/net/html"
)

var musixmatchBaseURL = `https://www.musixmatch.com/lyrics`
var musixmatchSelectContent = cascadia.MustCompile(`div.r-1v1z2uz:nth-child(1)`)
var musixmatchNotFound = []string{"Still no lyrics here"}
var musixmatchInstrumental = []string{"This music is instrumental"}
var musixmatchEsc = strings.NewReplacer(
	" ", "-",
	"'", "-",
	"(", "",
	")", "",
	"[", "",
	"]", "",
	".", "",
	"/", "",
	"?", "",
	"!", "",
	",", "",
)

type Musixmatch struct {
	RateLimit time.Duration

	initOnce   sync.Once
	HTTPClient *http.Client
}

func (mm *Musixmatch) Search(ctx context.Context, artist, song string, duration time.Duration) (string, error) {
	mm.initOnce.Do(func() {
		mm.HTTPClient = clientutil.Wrap(mm.HTTPClient, clientutil.Chain(
			clientutil.WithRateLimit(mm.RateLimit),
		))
	})

	url, _ := url.Parse(musixmatchBaseURL)
	url = url.JoinPath(musixmatchEsc.Replace(artist))
	url = url.JoinPath(musixmatchEsc.Replace(song))

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	resp, err := mm.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("req page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", ErrTrackNotFound
	}
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("musixmatch returned non 2xx: %d", resp.StatusCode)
	}

	node, err := html.Parse(resp.Body)
	if err != nil {
		return "", fmt.Errorf("parse page: %w", err)
	}

	var out strings.Builder
	findDocumentText(cascadia.Query(node, musixmatchSelectContent), &out)

	outStr := out.String()
	outStr = strings.TrimSpace(outStr)

	for _, notFound := range musixmatchNotFound {
		if strings.Contains(outStr, notFound) {
			return "", ErrTrackNotFound
		}
	}

	for _, instrumental := range musixmatchInstrumental {
		if strings.Contains(outStr, instrumental) {
			return "", nil
		}
	}

	return outStr, nil
}

func (mm *Musixmatch) String() string {
	return "musixmatch"
}
