package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/containrrr/shoutrrr"
	shoutrrrtypes "github.com/containrrr/shoutrrr/pkg/types"
)

var (
	ErrInvalidURI = errors.New("invalid URI")
)

type Notifications struct {
	mappings map[string][]string
}

func (n *Notifications) AddURI(event string, uri string) error {
	if n.mappings == nil {
		n.mappings = map[string][]string{}
	}
	if _, err := url.Parse(uri); err != nil {
		return fmt.Errorf("parse uri: %w", err)
	}
	n.mappings[event] = append(n.mappings[event], uri)
	return nil
}

func (n *Notifications) IterMappings(f func(string, string)) {
	for event, uris := range n.mappings {
		for _, uri := range uris {
			f(event, uri)
		}
	}
}
func (n *Notifications) Sendf(ctx context.Context, event string, f string, a ...any) {
	n.Send(ctx, event, fmt.Sprintf(f, a...))
}

// Send a simple string for now, maybe later message could instead be be a type which
// implements a notifications.Bodyer or something so that notifiers can send rich notifications.
func (n *Notifications) Send(ctx context.Context, event string, message string) {
	uris := n.mappings[event]
	if len(uris) == 0 {
		return
	}

	sender, err := shoutrrr.CreateSender(uris...)
	if err != nil {
		slog.ErrorContext(ctx, "create sender", "err", err)
		return
	}

	params := &shoutrrrtypes.Params{}
	params.SetTitle("wrtag")

	if err := errors.Join(sender.Send(message, params)...); err != nil {
		slog.ErrorContext(ctx, "sending notifications", "err", err)
		return
	}
}
