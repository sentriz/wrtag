package mainlib

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"

	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/clientutil"
)

func Logging() (exit func()) {
	var logLevel slog.LevelVar
	flag.TextVar(&logLevel, "log-level", &logLevel, "set the logging level")

	h := &slogErrorHandler{
		Handler: slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: &logLevel}),
	}

	logger := slog.New(h)
	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(slog.LevelError)

	return func() {
		if h.hadSlogError.Load() {
			os.Exit(1)
		}
		os.Exit(0)
	}
}

type slogErrorHandler struct {
	slog.Handler
	hadSlogError atomic.Bool
}

func (n *slogErrorHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level == slog.LevelError {
		n.hadSlogError.Store(true)
	}
	return n.Handler.Handle(ctx, r)
}

func WrapClient() {
	chain := clientutil.Chain(
		clientutil.WithLogging(slog.Default()),
		clientutil.WithUserAgent(fmt.Sprintf(`%s/%s`, wrtag.Name, wrtag.Version)),
	)

	http.DefaultTransport = chain(http.DefaultTransport)
}
