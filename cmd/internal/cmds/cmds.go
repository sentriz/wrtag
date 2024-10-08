package cmds

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"go.senan.xyz/flagconf"
	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/addon"
	"go.senan.xyz/wrtag/clientutil"
	"go.senan.xyz/wrtag/notifications"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/researchlink"
	"go.senan.xyz/wrtag/tagmap"
)

func WrapClient() {
	chain := clientutil.Chain(
		clientutil.WithLogging(slog.Default()),
		clientutil.WithUserAgent(fmt.Sprintf(`%s/%s`, wrtag.Name, wrtag.Version)),
	)

	http.DefaultTransport = chain(http.DefaultTransport)
}

func Parse() {
	userConfig, _ := os.UserConfigDir()
	defaultConfigPath := filepath.Join(userConfig, wrtag.Name, "config")
	configPath := flag.String("config-path", defaultConfigPath, "path config file")

	printVersion := flag.Bool("version", false, "print the version")
	printConfig := flag.Bool("config", false, "print the parsed config")

	flag.Parse()
	flagconf.ReadEnvPrefix = func(_ *flag.FlagSet) string { return wrtag.Name }
	flagconf.ParseEnv()
	flagconf.ParseConfig(*configPath)

	if *printVersion {
		fmt.Printf("%s %s\n", flag.CommandLine.Name(), wrtag.Version)
		os.Exit(0)
	}
	if *printConfig {
		flag.VisitAll(func(f *flag.Flag) {
			fmt.Printf("%-16s %s\n", f.Name, f.Value)
		})
		os.Exit(0)
	}
}

func WrtagConfig() *wrtag.Config {
	var cfg wrtag.Config

	flag.Var(&pathFormatParser{&cfg.PathFormat}, "path-format", "music directory and go templated path format to define music library layout")
	flag.Var(&querierParser{&cfg.ResearchLinkQuerier}, "research-link", "define a helper url to help find information about an unmatched release")
	flag.Var(&notificationsParser{&cfg.Notifications}, "notification-uri", "add a shoutrrr notification uri for an event")
	flag.Var(&addonsParser{&cfg.Addons}, "addon", "add some extra metadata when importing tracks")

	cfg.KeepFiles = map[string]struct{}{}
	flag.Var(&keepFileParser{cfg.KeepFiles}, "keep-file", "files to keep from source directories")

	cfg.TagWeights = tagmap.TagWeights{}
	flag.Var(&tagWeightsParser{cfg.TagWeights}, "tag-weight", "adjust distance weighting for a tag between. 0 to ignore")

	flag.StringVar(&cfg.MusicBrainzClient.BaseURL, "mb-base-url", `https://musicbrainz.org/ws/2/`, "musicbrainz base url")
	flag.DurationVar(&cfg.MusicBrainzClient.RateLimit, "mb-rate-limit", 1*time.Second, "musicbrainz rate limit duration")

	flag.StringVar(&cfg.CoverArtArchiveClient.BaseURL, "caa-base-url", `https://coverartarchive.org/`, "coverartarchive base url")
	flag.DurationVar(&cfg.CoverArtArchiveClient.RateLimit, "caa-rate-limit", 0, "coverartarchive rate limit duration")

	flag.BoolVar(&cfg.UpgradeCover, "cover-upgrade", false, "fetch new cover art even if it exists locally")

	return &cfg
}

var _ flag.Value = (*pathFormatParser)(nil)
var _ flag.Value = (*querierParser)(nil)
var _ flag.Value = (*notificationsParser)(nil)
var _ flag.Value = (*tagWeightsParser)(nil)
var _ flag.Value = (*keepFileParser)(nil)
var _ flag.Value = (*addonsParser)(nil)

type pathFormatParser struct{ *pathformat.Format }

func (pf *pathFormatParser) Set(value string) error {
	value, err := filepath.Abs(value)
	if err != nil {
		return fmt.Errorf("make abs: %w", err)
	}
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
		err := n.AddURI(ev, uri)
		lineErrs = append(lineErrs, err)
	}
	return errors.Join(lineErrs...)
}
func (n notificationsParser) String() string {
	if n.Notifications == nil {
		return ""
	}
	var parts []string
	n.Notifications.IterMappings(func(e string, uri string) {
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
	var addn wrtag.Addon
	var err error
	switch name, rest, _ := strings.Cut(strings.TrimLeft(value, " "), " "); name {
	case "lyrics":
		addn, err = addon.NewLyricsAddon(rest)
	case "replaygain":
		addn, err = addon.NewReplayGainAddon(rest)
	case "subproc":
		addn, err = addon.NewSubprocAddon(rest)
	default:
		err = fmt.Errorf("unknown addon %q", name)
	}
	if err != nil {
		return err
	}
	*a.addons = append(*a.addons, addn)
	return nil
}
func (a addonsParser) String() string {
	if a.addons == nil {
		return ""
	}
	var parts []string
	for _, a := range *a.addons {
		parts = append(parts, fmt.Sprint(a))
	}
	return strings.Join(parts, ", ")
}
