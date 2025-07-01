package wrtagflag

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
	"time"

	"go.senan.xyz/flagconf"
	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/addon"
	"go.senan.xyz/wrtag/clientutil"
	"go.senan.xyz/wrtag/notifications"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/researchlink"

	_ "go.senan.xyz/wrtag/addon/lyrics"
	_ "go.senan.xyz/wrtag/addon/musicdesc"
	_ "go.senan.xyz/wrtag/addon/replaygain"
	_ "go.senan.xyz/wrtag/addon/subproc"
)

func DefaultClient() {
	chain := clientutil.Chain(
		clientutil.WithLogging(slog.Default()),
		clientutil.WithUserAgent(fmt.Sprintf(`%s/%s`, wrtag.Name, wrtag.Version)),
	)

	http.DefaultTransport = chain(http.DefaultTransport)
}

func Parse() {
	userConfig, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}

	defaultConfigPath := filepath.Join(userConfig, wrtag.Name, "config")
	configPath := flag.String("config-path", defaultConfigPath, "Path to config file")

	printVersion := flag.Bool("version", false, "Print the version and exit")
	printConfig := flag.Bool("config", false, "Print the parsed config and exit")

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

func Config() *wrtag.Config {
	var cfg wrtag.Config

	flag.Var(&pathFormatParser{&cfg.PathFormat}, "path-format", "Path to root music directory including path format rules (see [Path format](#path-format))")
	flag.Var(&addonsParser{&cfg.Addons}, "addon", "Define an addon for extra metadata writing (see [Addons](#addons)) (stackable)")

	cfg.KeepFiles = map[string]struct{}{}
	flag.Var(&keepFileParser{cfg.KeepFiles}, "keep-file", "Define an extra file path to keep when moving/copying to root dir (stackable)")

	cfg.DiffWeights = wrtag.DiffWeights{}
	flag.Var(&diffWeightsParser{cfg.DiffWeights}, "diff-weight", "Adjust distance weighting for a tag (0 to ignore) (stackable)")

	cfg.TagConfig = wrtag.TagConfig{}
	flag.Var(&tagConfigParser{&cfg.TagConfig}, "tag-config", "Specify tag keep and drop rules when writing new tag revisions (see [Tagging](#tagging)) (stackable)")

	flag.StringVar(&cfg.MusicBrainzClient.BaseURL, "mb-base-url", `https://musicbrainz.org/ws/2/`, "MusicBrainz base URL")
	flag.DurationVar(&cfg.MusicBrainzClient.RateLimit, "mb-rate-limit", 1*time.Second, "MusicBrainz rate limit duration")

	flag.StringVar(&cfg.CoverArtArchiveClient.BaseURL, "caa-base-url", `https://coverartarchive.org/`, "CoverArtArchive base URL")
	flag.DurationVar(&cfg.CoverArtArchiveClient.RateLimit, "caa-rate-limit", 0, "CoverArtArchive rate limit duration")

	flag.BoolVar(&cfg.UpgradeCover, "cover-upgrade", false, "Fetch new cover art even if it exists locally")

	return &cfg
}

func Notifications() *notifications.Notifications {
	var n notifications.Notifications
	flag.Var(&notificationsParser{&n}, "notification-uri", "Add a shoutrrr notification URI for an event (see [Notifications](#notifications)) (stackable)")
	return &n
}

func ResearchLinks() *researchlink.Builder {
	var r researchlink.Builder
	flag.Var(&researchLinkParser{&r}, "research-link", "Define a helper URL to help find information about an unmatched release (stackable)")
	return &r
}

func OperationByName(name string, dryRun bool) (wrtag.FileSystemOperation, error) {
	switch name {
	case "copy":
		return wrtag.NewCopy(dryRun), nil
	case "move":
		return wrtag.NewMove(dryRun), nil
	case "reflink":
		return wrtag.NewReflink(dryRun), nil
	default:
		return nil, errors.New("unknown operation")
	}
}

var _ flag.Value = (*pathFormatParser)(nil)
var _ flag.Value = (*researchLinkParser)(nil)
var _ flag.Value = (*notificationsParser)(nil)
var _ flag.Value = (*diffWeightsParser)(nil)
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
	return pf.Root() + "/..."
}

type researchLinkParser struct{ *researchlink.Builder }

func (r *researchLinkParser) Set(value string) error {
	name, value, _ := strings.Cut(value, " ")
	name, value = strings.TrimSpace(name), strings.TrimSpace(value)
	err := r.AddSource(name, value)
	return err
}
func (r researchLinkParser) String() string {
	if r.Builder == nil {
		return ""
	}
	var names []string
	for s := range r.Builder.IterSources() {
		names = append(names, s)
	}
	return strings.Join(names, ", ")
}

type notificationsParser struct{ *notifications.Notifications }

func (n *notificationsParser) Set(value string) error {
	eventsRaw, uri, ok := strings.Cut(value, " ")
	if !ok {
		return errors.New("invalid notification uri format. expected eg \"ev1,ev2 uri\"")
	}
	var lineErrs []error
	for ev := range strings.SplitSeq(eventsRaw, ",") {
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

type diffWeightsParser struct{ wrtag.DiffWeights }

func (tw diffWeightsParser) Set(value string) error {
	const sep = " "
	i := strings.LastIndex(value, sep)
	if i < 0 {
		return errors.New("invalid diff weight format. expected eg \"tag name 0.5\"")
	}
	k := strings.TrimSpace(value[:i])
	weightStr := strings.TrimSpace(value[i+len(sep):])
	weight, err := strconv.ParseFloat(weightStr, 64)
	if err != nil {
		return fmt.Errorf("parse weight: %w", err)
	}
	tw.DiffWeights[k] = weight
	return nil
}
func (tw diffWeightsParser) String() string {
	var parts []string
	for a, b := range tw.DiffWeights {
		parts = append(parts, fmt.Sprintf("%s: %.2f", a, b))
	}
	return strings.Join(parts, ", ")
}

type tagConfigParser struct{ *wrtag.TagConfig }

func (tw tagConfigParser) Set(value string) error {
	op, tag, ok := strings.Cut(value, " ")
	if !ok {
		return errors.New("invalid tag config format. expected eg \"<op> <tag>\"")
	}
	switch op {
	case "keep":
		tw.Keep = append(tw.Keep, tag)
	case "drop":
		tw.Drop = append(tw.Drop, tag)
	default:
		return fmt.Errorf("invalid tag config op %q", op)
	}
	return nil
}

func (tw tagConfigParser) String() string {
	if tw.TagConfig == nil {
		return ""
	}
	var parts []string
	for _, k := range tw.Keep {
		parts = append(parts, fmt.Sprintf("keep %q", k))
	}
	for _, k := range tw.Drop {
		parts = append(parts, fmt.Sprintf("drop %q", k))
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
	addons *[]addon.Addon
}

func (a *addonsParser) Set(value string) error {
	name, rest, _ := strings.Cut(strings.TrimLeft(value, " "), " ")
	addn, err := addon.New(name, rest)
	if err != nil {
		return fmt.Errorf("addon %q: %w", name, err)
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
