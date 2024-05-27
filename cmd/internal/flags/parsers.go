package flags

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"text/template"
	"time"

	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/addon/lyrics"
	"go.senan.xyz/wrtag/addon/replaygain"
	"go.senan.xyz/wrtag/notifications"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/researchlink"
	"go.senan.xyz/wrtag/tagmap"
)

var _ flag.Value = (*pathFormatParser)(nil)
var _ flag.Value = (*querierParser)(nil)
var _ flag.Value = (*notificationsParser)(nil)
var _ flag.Value = (*tagWeightsParser)(nil)
var _ flag.Value = (*keepFileParser)(nil)

type pathFormatParser struct{ *pathformat.Format }

func (pf *pathFormatParser) Set(value string) error {
	return pf.Parse(value)
}
func (pf pathFormatParser) String() string {
	if pf.Format == nil || pf.Root() == "" {
		return ""
	}
	return fmt.Sprintf("%s/...", pf.Root())
}

type querierParser struct{ *researchlink.Querier }

func (q *querierParser) Set(value string) error {
	name, value, _ := strings.Cut(value, " ")
	name, value = strings.TrimSpace(name), strings.TrimSpace(value)
	err := q.AddSource(name, value)
	return err
}
func (q querierParser) String() string {
	if q.Querier == nil {
		return ""
	}
	var names []string
	q.Querier.IterSources(func(s string, _ *template.Template) {
		names = append(names, s)
	})
	return strings.Join(names, ", ")
}

type notificationsParser struct{ *notifications.Notifications }

func (n *notificationsParser) Set(value string) error {
	eventsRaw, uri, ok := strings.Cut(value, " ")
	if !ok {
		return fmt.Errorf("invalid notification uri format. expected eg \"ev1,ev2 uri\"")
	}
	var lineErrs []error
	for _, ev := range strings.Split(eventsRaw, ",") {
		ev, uri = strings.TrimSpace(ev), strings.TrimSpace(uri)
		err := n.AddURI(notifications.Event(ev), uri)
		lineErrs = append(lineErrs, err)
	}
	return errors.Join(lineErrs...)
}
func (n notificationsParser) String() string {
	if n.Notifications == nil {
		return ""
	}
	var parts []string
	n.Notifications.IterMappings(func(e notifications.Event, uri string) {
		url, _ := url.Parse(uri)
		parts = append(parts, fmt.Sprintf("%s: %s://%s/...", e, url.Scheme, url.Host))
	})
	return strings.Join(parts, ", ")
}

type tagWeightsParser struct{ tagmap.TagWeights }

func (tw tagWeightsParser) Set(value string) error {
	const sep = " "
	i := strings.LastIndex(value, sep)
	if i < 0 {
		return fmt.Errorf("invalid tag weight format. expected eg \"tag name 0.5\"")
	}
	tag := strings.TrimSpace(value[:i])
	weightStr := strings.TrimSpace(value[i+len(sep):])
	weight, err := strconv.ParseFloat(weightStr, 64)
	if err != nil {
		return fmt.Errorf("parse weight: %w", err)
	}
	tw.TagWeights[tag] = weight
	return nil
}
func (tw tagWeightsParser) String() string {
	var parts []string
	for a, b := range tw.TagWeights {
		parts = append(parts, fmt.Sprintf("%s: %.2f", a, b))
	}
	return strings.Join(parts, ", ")
}

type keepFileParser struct{ m map[string]struct{} }

func (kf keepFileParser) Set(value string) error {
	kf.m[value] = struct{}{}
	return nil
}
func (kf *keepFileParser) String() string {
	var parts []string
	for k := range kf.m {
		parts = append(parts, k)
	}
	return strings.Join(parts, ", ")
}

type addonsParser struct {
	addons *[]wrtag.Addon
}

func (a *addonsParser) Set(value string) error {
	switch value {
	case "lyrics":
		*a.addons = append(*a.addons, lyrics.Addon{
			Source: lyrics.MultiSource{
				&lyrics.Genius{RateLimit: 500 * time.Millisecond},
				&lyrics.Musixmatch{RateLimit: 500 * time.Millisecond},
			},
		})
	case "replaygain":
		*a.addons = append(*a.addons, replaygain.Addon{})
	}
	return nil
}
func (a addonsParser) String() string {
	if a.addons == nil {
		return ""
	}
	var parts []string
	for _, k := range *a.addons {
		parts = append(parts, k.Name())
	}
	return strings.Join(parts, ", ")
}
