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

var geniusBaseURL = `https://genius.com`
var geniusSelectContent = cascadia.MustCompile(`div[class^="Lyrics__Container-"]`)
var geniusEsc = strings.NewReplacer(
	" ", "-",
	"&", "and",
	"'", "",
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

type Genius struct {
	RateLimit time.Duration

	initOnce   sync.Once
	HTTPClient *http.Client
}

func (g *Genius) Search(ctx context.Context, artist, song string, duration time.Duration) (string, error) {
	g.initOnce.Do(func() {
		g.HTTPClient = clientutil.Wrap(g.HTTPClient, clientutil.Chain(
			clientutil.WithRateLimit(g.RateLimit),
		))
	})

	// use genius case rules to miminise redirects
	page := fmt.Sprintf("%s-%s-lyrics", artist, song)
	page = strings.ToUpper(string(page[0])) + strings.ToLower(page[1:])

	url, _ := url.Parse(geniusBaseURL)
	url = url.JoinPath(geniusEsc.Replace(page))

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("req page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", ErrTrackNotFound
	}
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("genius returned non 2xx: %d", resp.StatusCode)
	}

	node, err := html.Parse(resp.Body)
	if err != nil {
		return "", fmt.Errorf("parse page: %w", err)
	}

	var out strings.Builder
	findDocumentText(cascadia.Query(node, geniusSelectContent), &out)

	outStr := out.String()
	outStr = strings.TrimSpace(outStr)

	if strings.Contains(outStr, "This song is an instrumental") {
		return "", nil
	}

	return outStr, nil
}

func (g *Genius) String() string {
	return "genius"
}
