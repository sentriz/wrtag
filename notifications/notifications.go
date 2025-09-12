package notifications

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"net/url"
	"time"

	"github.com/containrrr/shoutrrr"
	shoutrrrtypes "github.com/containrrr/shoutrrr/pkg/types"
)

var (
	ErrInvalidURI = errors.New("invalid URI")
)

type Destination struct {
	URI                 string
	SuppressAfterAction time.Duration // suppress this long after manual actions
}

type Notifications struct {
	mappings map[string][]Destination
}

func (n *Notifications) AddDestination(event string, uri string, suppressAfterAction time.Duration) error {
	if n.mappings == nil {
		n.mappings = map[string][]Destination{}
	}
	if _, err := url.Parse(uri); err != nil {
		return fmt.Errorf("parse uri: %w", err)
	}
	n.mappings[event] = append(n.mappings[event], Destination{URI: uri, SuppressAfterAction: suppressAfterAction})
	return nil
}

func (n *Notifications) Destinations() iter.Seq2[string, Destination] {
	mappings := maps.Clone(n.mappings)

	return func(yield func(string, Destination) bool) {
		for event, destinations := range mappings {
			for _, destination := range destinations {
				if !yield(event, destination) {
					return
				}
			}
		}
	}
}

func (n *Notifications) Sendf(ctx context.Context, event string, f string, a ...any) {
	n.Send(ctx, event, fmt.Sprintf(f, a...))
}

// Send a simple string for now, maybe later message could instead be a type which
// implements a notifications.Bodyer or something so that notifiers can send rich notifications.
func (n *Notifications) Send(ctx context.Context, event string, message string) {
	destinations := n.mappings[event]
	if len(destinations) == 0 {
		return
	}

	var timeSinceAction time.Duration
	if actionTime, ok := ctx.Value(actionKey{}).(time.Time); ok {
		timeSinceAction = time.Since(actionTime)
	}

	var uris []string
	for _, dest := range destinations {
		if timeSinceAction == 0 || timeSinceAction >= dest.SuppressAfterAction {
			uris = append(uris, dest.URI)
		} else {
			slog.DebugContext(ctx, "suppressing notification due to recent manual action",
				"event", event, "suppress_after_action", dest.SuppressAfterAction, "since_action", timeSinceAction)
		}
	}
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

type actionKey struct{}

// RecordAction records the current time of a user action and returns a context which may
// be used to suppres notifications later.
func RecordAction(ctx context.Context) context.Context {
	return RecordActionTime(ctx, time.Now())
}

// RecordActionTime records a specific time as a user action and returns a context which may
// be used to suppress notifications later.
func RecordActionTime(ctx context.Context, actionTime time.Time) context.Context {
	return context.WithValue(ctx, actionKey{}, actionTime)
}

func ActionTime(ctx context.Context) time.Time {
	t, _ := ctx.Value(actionKey{}).(time.Time)
	return t
}
