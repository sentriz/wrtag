package wrtaglog

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"sync/atomic"
)

func Setup() (exit func()) {
	var logLevel slog.LevelVar
	flag.TextVar(&logLevel, "log-level", &logLevel, "Set the logging level")

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
