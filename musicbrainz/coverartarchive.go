package musicbrainz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.senan.xyz/wrtag/clientutil"
)

type CAAClient struct {
	BaseURL   string
	RateLimit time.Duration

	initOnce   sync.Once
	HTTPClient *http.Client
}

func (c *CAAClient) request(ctx context.Context, r *http.Request, dest any) error {
	c.initOnce.Do(func() {
		c.HTTPClient = clientutil.Wrap(c.HTTPClient, clientutil.Chain(
			clientutil.WithCache(),
			clientutil.WithRateLimit(c.RateLimit),
		))
	})

	r = r.WithContext(ctx)
	resp, err := c.HTTPClient.Do(r)
	if err != nil {
		return fmt.Errorf("make caa request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("caa returned non 2xx: %w", StatusError(resp.StatusCode))
	}
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decode caa response: %w", err)
	}
	return nil
}

func (c *CAAClient) GetCoverURL(ctx context.Context, release *Release) (string, error) {
	var candidateURLs []string
	if release.CoverArtArchive.Front {
		candidateURLs = append(candidateURLs, joinPath(c.BaseURL, "release", release.ID))
	}
	candidateURLs = append(candidateURLs, joinPath(c.BaseURL, "release-group", release.ReleaseGroup.ID))

	for _, candidate := range candidateURLs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}

		var caa caaResponse
		err = c.request(ctx, req, &caa)
		if se := StatusError(0); errors.As(err, &se) && se == http.StatusNotFound {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("make caa release request: %w", err)
		}

		for _, img := range caa.Images {
			if img.Front {
				return img.Image, nil
			}
		}
		return "", nil
	}
	return "", nil
}

type caaResponse struct {
	Release string `json:"release"`
	Images  []struct {
		Approved   bool     `json:"approved"`
		Back       bool     `json:"back"`
		Comment    string   `json:"comment"`
		Edit       int      `json:"edit"`
		Front      bool     `json:"front"`
		ID         any      `json:"id"`
		Image      string   `json:"image"`
		Types      []string `json:"types"`
		Thumbnails struct {
			Num250  string `json:"250"`
			Num500  string `json:"500"`
			Num1200 string `json:"1200"`
			Large   string `json:"large"`
			Small   string `json:"small"`
		} `json:"thumbnails"`
	} `json:"images"`
}
