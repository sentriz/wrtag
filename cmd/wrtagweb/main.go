// Command wrtagweb provides a web interface for wrtag,
// allowing users to queue and manage music tagging operations through a browser.
package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql"
	"embed"
	"errors"
	"flag"
	"fmt"
	htmltemplate "html/template"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.senan.xyz/wrtag"
	wrtagflag "go.senan.xyz/wrtag/cmd/internal/wrtagflag"
	"go.senan.xyz/wrtag/cmd/internal/wrtaglog"
	"go.senan.xyz/wrtag/notifications"
	"go.senan.xyz/wrtag/researchlink"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"go.senan.xyz/sqlb"
	"golang.org/x/sync/errgroup"
)

func init() {
	flag := flag.CommandLine
	flag.Usage = func() {
		fmt.Fprintf(flag.Output(), "Usage:\n")
		fmt.Fprintf(flag.Output(), "  $ %s [<options>]\n", flag.Name())
		fmt.Fprintf(flag.Output(), "\n")
		fmt.Fprintf(flag.Output(), "Options:\n")
		flag.PrintDefaults()
	}
}

const (
	notifComplete   = "complete"
	notifNeedsInput = "needs-input"
)

func main() {
	defer wrtaglog.Setup()()
	wrtagflag.DefaultClient()
	var (
		cfg                 = wrtagflag.Config()
		notifs              = wrtagflag.Notifications()
		researchLinkQuerier = wrtagflag.ResearchLinks()
		apiKey              = flag.String("web-api-key", "", "Key for external API endpoints")
		auth                = flag.String("web-auth", string(authBasicFromAPIKey), "Authentication mode, one of \"disabled\", \"basic-auth-from-api-key\" (optional)")
		listenAddr          = flag.String("web-listen-addr", ":7373", "Listen address for web interface (optional)")
		dbPath              = flag.String("web-db-path", "", "Path to persistent database path for web interface (optional)")
		publicURL           = flag.String("web-public-url", "", "Public URL for web interface (optional)")
		numWorkers          = flag.Int("web-num-workers", max(runtime.NumCPU()/4, 1), "Number of directories to process concurrently")
	)
	wrtagflag.Parse()

	if cfg.PathFormat.Root() == "" {
		slog.Error("no path-format configured")
		return
	}

	if *apiKey == "" {
		slog.Error("need an api key")
		return
	}
	if *listenAddr == "" {
		slog.Error("need a listen addr")
		return
	}

	switch ath := authMode(*auth); ath {
	case authDisabled, authBasicFromAPIKey:
	default:
		slog.Error("unknown auth mode", "value", ath)
		return
	}

	if *dbPath == "" {
		tmpf, err := os.CreateTemp("", "wrtagweb*.db")
		if err != nil {
			slog.Error("error creating tmp file", "error", err)
			return
		}

		*dbPath = tmpf.Name()

		defer func() {
			_ = tmpf.Close()
			_ = os.Remove(tmpf.Name())
		}()
	}

	dbURI, _ := url.Parse("file://?cache=shared&_fk=1")
	dbURI.Path = *dbPath
	db, err := sql.Open("sqlite3", dbURI.String())
	if err != nil {
		slog.Error("open db template", "err", err)
		return
	}
	defer db.Close()

	ctx := context.Background()
	if lev := slog.LevelDebug; slog.Default().Enabled(context.Background(), lev) {
		ctx = sqlb.WithLogFunc(ctx, func(ctx context.Context, typ string, query string, duration time.Duration) {
			slog.Log(ctx, lev, typ, "took", duration, "query", query)
		})
	}

	if err := dbMigrate(ctx, db); err != nil {
		slog.Error("migrate db", "err", err)
		return
	}

	var sse broadcast[uint64]
	jobSSENew := func() { sse.send(0) }
	jobSSEUpdate := func(id uint64) { sse.send(id) }

	var jobQueue = make(chan uint64, 32_768)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /sse", func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		rc.Flush()

		ctx := r.Context()

		for id := range sse.receive(ctx, 128) {
			fmt.Fprintf(w, "data: %d\n\n", id)
			rc.Flush()
		}
	})

	mux.HandleFunc("GET /jobs", func(w http.ResponseWriter, r *http.Request) {
		search := r.URL.Query().Get("search")
		filter := JobStatus(r.URL.Query().Get("filter"))
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))

		ctx := r.Context()

		jl, err := listJobs(ctx, db, filter, search, page, listJobsPageSize)
		if err != nil {
			respErrf(w, http.StatusInternalServerError, "error listing jobs: %v", err)
			return
		}
		respTmpl(w, "jobs", jl)
	})

	mux.HandleFunc("POST /jobs", func(w http.ResponseWriter, r *http.Request) {
		operationStr := r.FormValue("operation")
		if _, err := wrtagflag.OperationByName(operationStr, false); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		path := r.FormValue("path")
		if path == "" {
			respErrf(w, http.StatusBadRequest, "no path provided")
			return
		}
		if !filepath.IsAbs(path) {
			respErrf(w, http.StatusInternalServerError, "filepath not abs")
			return
		}
		path = filepath.Clean(path)

		ctx := r.Context()

		var job Job
		if err := sqlb.ScanRow(ctx, db, &job, "insert into jobs (source_path, operation, time) values (?, ?, ?) returning *", path, operationStr, time.Now()); err != nil {
			http.Error(w, fmt.Sprintf("error saving job: %v", err), http.StatusInternalServerError)
			return
		}

		respTmpl(w, "job-import", struct{ Operation string }{Operation: operationStr})

		jobSSENew()
		jobQueue <- job.ID
	})

	mux.HandleFunc("GET /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))

		ctx := r.Context()

		var job Job
		if err := sqlb.ScanRow(ctx, db, &job, "select * from jobs where id=?", id); err != nil {
			respErrf(w, http.StatusInternalServerError, "error getting job")
			return
		}
		respTmpl(w, "job", job)
	})

	mux.HandleFunc("PUT /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))

		confirm, _ := strconv.ParseBool(r.FormValue("confirm"))

		useMBID := r.FormValue("mbid")
		if strings.Contains(useMBID, "/") {
			useMBID = path.Base(useMBID) // accept release URL
		}

		ctx := r.Context()

		var job Job
		if err := sqlb.ScanRow(ctx, db, &job, "update jobs set confirm=?, use_mbid=?, status=?, updated_time=? where id=? and status<>? returning *", confirm, useMBID, StatusEnqueued, time.Now(), id, StatusInProgress); err != nil {
			respErrf(w, http.StatusInternalServerError, "error getting job")
			return
		}

		respTmpl(w, "job", job)

		jobSSENew()
		jobQueue <- job.ID
	})

	mux.HandleFunc("DELETE /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))

		ctx := r.Context()

		if err := sqlb.Exec(ctx, db, "delete from jobs where id=? and status<>?", id, StatusInProgress); err != nil {
			respErrf(w, http.StatusInternalServerError, "error getting job")
			return
		}
		jobSSENew()
	})

	mux.HandleFunc("GET /dirs", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		if path == "" || !filepath.IsAbs(path) {
			return
		}
		path = filepath.Clean(path)
		path = os.ExpandEnv(path)

		if entries, err := os.ReadDir(path); err == nil {
			var dirs []string
			for _, entry := range entries {
				if entry.IsDir() {
					dirs = append(dirs, filepath.Join(path, entry.Name()))
				}
			}
			respTmpl(w, "dropdown", dirs)
			return
		}

		if matches, err := filepath.Glob(path + "*"); err == nil {
			var dirs []string
			for _, match := range matches {
				if stat, err := os.Stat(match); err == nil && stat.IsDir() {
					dirs = append(dirs, match)
				}
			}
			respTmpl(w, "dropdown", dirs)
			return
		}
	})

	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		jl, err := listJobs(ctx, db, "", "", 0, listJobsPageSize)
		if err != nil {
			respErrf(w, http.StatusInternalServerError, "error listing jobs: %v", err)
			return
		}
		respTmpl(w, "index", struct {
			jobsListing
			Operation string
		}{
			jl, OperationCopy,
		})
	})

	mux.Handle("/", http.FileServer(http.FS(ui)))

	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/*", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)

	// external API
	muxExternal := http.NewServeMux()

	muxExternal.HandleFunc("POST /op/{operation}", func(w http.ResponseWriter, r *http.Request) {
		operationStr := r.PathValue("operation")
		if _, err := wrtagflag.OperationByName(operationStr, false); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		pth := r.FormValue("path")
		if pth == "" {
			http.Error(w, "no path provided", http.StatusBadRequest)
			return
		}
		if !filepath.IsAbs(pth) {
			http.Error(w, "filepath not abs", http.StatusBadRequest)
			return
		}
		pth = filepath.Clean(pth)
		useMBID := r.FormValue("mbid")
		if strings.Contains(useMBID, "/") {
			useMBID = path.Base(useMBID) // accept release URL
		}

		ctx := r.Context()

		var job Job
		if err := sqlb.ScanRow(ctx, db, &job, "insert into jobs (source_path, operation, use_mbid, time) values (?, ?, ?, ?) returning *", pth, operationStr, useMBID, time.Now()); err != nil {
			http.Error(w, fmt.Sprintf("error saving job: %v", err), http.StatusInternalServerError)
			return
		}

		jobSSENew()
		jobQueue <- job.ID
	})

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	errgrp, ctx := errgroup.WithContext(ctx)

	errgrp.Go(func() error {
		defer logJob("http", "addr", *listenAddr)()

		m := http.NewServeMux()
		m.Handle("/", authMiddleware(mux, authMode(*auth), *apiKey))
		m.Handle("/op/", apiKeyMiddleware(muxExternal, *apiKey))

		var h http.Handler
		h = m
		h = logMiddleware(h)

		server := &http.Server{
			Addr:              *listenAddr,
			Handler:           h,
			ReadHeaderTimeout: 2 * time.Second,
			BaseContext:       func(l net.Listener) context.Context { return ctx },
		}

		errgrp.Go(func() error {
			<-ctx.Done()
			_ = server.Shutdown(context.Background()) //nolint:contextcheck
			return nil
		})
		errgrp.Go(func() error {
			<-ctx.Done()
			sse.close()
			return nil
		})

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	for w := range *numWorkers {
		errgrp.Go(func() error {
			defer logJob("process jobs", "worker", w)()

			for {
				select {
				case <-ctx.Done():
					return nil
				case jobID := <-jobQueue:
					if err := processJob(ctx, cfg, notifs, researchLinkQuerier, *publicURL, db, jobSSEUpdate, jobID); err != nil {
						return fmt.Errorf("next job: %w", err)
					}
				}
			}
		})
	}

	// restart old jobs just in case the process was killed abruptly last time
	errgrp.Go(func() error {
		for job, err := range sqlb.IterRows[Job](ctx, db, "update jobs set status=? where status=? returning *", StatusEnqueued, StatusInProgress) {
			if err != nil {
				return fmt.Errorf("iter old jobs: %w", err)
			}
			jobQueue <- job.ID
		}
		return nil
	})

	if err := errgrp.Wait(); err != nil {
		slog.Error("wait for jobs", "err", err)
		return
	}
}

func processJob(ctx context.Context, cfg *wrtag.Config, notifs *notifications.Notifications, researchLinkQuerier *researchlink.Builder, publicURL string, db *sql.DB, jobSSEUpdate func(uint64), jobID uint64) error {
	var job Job
	err := sqlb.ScanRow(ctx, db, &job, "update jobs set status=? where id=? and status=? returning *", StatusInProgress, jobID, StatusEnqueued)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	jobSSEUpdate(job.ID)
	defer jobSSEUpdate(job.ID)

	op, err := wrtagflag.OperationByName(job.Operation, false)
	if err != nil {
		return fmt.Errorf("find operation: %w", err)
	}

	var ic wrtag.ImportCondition
	if job.Confirm {
		ic = wrtag.Always
	}

	searchResult, processErr := wrtag.ProcessDir(ctx, cfg, op, job.SourcePath, ic, job.UseMBID)

	if searchResult != nil && searchResult.Query.Artist != "" {
		researchLinks, err := researchLinkQuerier.Build(researchlink.Query{
			Artist:  searchResult.Query.Artist,
			Album:   searchResult.Query.Release,
			Barcode: searchResult.Query.Barcode,
			Date:    searchResult.Query.Date,
		})
		if err != nil {
			return fmt.Errorf("build links: %w", err)
		}

		job.ResearchLinks = sqlb.NewJSON(researchLinks)
	}

	if searchResult != nil && searchResult.Release != nil && (processErr == nil || wrtag.IsNonFatalError(processErr)) {
		job.DestPath, err = wrtag.DestDir(&cfg.PathFormat, searchResult.Release)
		if err != nil {
			return fmt.Errorf("gen dest dir: %w", err)
		}
	}

	job.SearchResult = sqlb.NewJSON(searchResult)
	job.Confirm = false

	if processErr != nil {
		job.Status = StatusError
		job.Error = processErr.Error()
		if errors.Is(processErr, wrtag.ErrScoreTooLow) {
			job.Status = StatusNeedsInput
		}
	} else {
		job.Status = StatusComplete
		job.Error = ""
		job.UseMBID = ""
		job.Operation = OperationMove // allow re-tag from dest
		job.SourcePath = job.DestPath
	}

	if err := sqlb.ScanRow(ctx, db, &job, "update jobs set ? where id=? returning *", sqlb.UpdateSQL(job), job.ID); err != nil {
		return err
	}

	{
		ctx := context.WithoutCancel(ctx)

		// If we have no action time in the ctx, use the job's updated at
		if job.UpdatedTime.Valid && notifications.ActionTime(ctx).IsZero() {
			ctx = notifications.RecordActionTime(ctx, job.UpdatedTime.Time)
		}

		switch job.Status {
		case StatusComplete:
			go notifs.Send(ctx, notifComplete, jobNotificationMessage(publicURL, job))
		case StatusNeedsInput:
			go notifs.Send(ctx, notifNeedsInput, jobNotificationMessage(publicURL, job))
		}
	}

	return nil
}

var tmplBuffPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

func respTmpl(w http.ResponseWriter, name string, data any) {
	buff, _ := tmplBuffPool.Get().(*bytes.Buffer)
	defer tmplBuffPool.Put(buff)
	buff.Reset()

	if err := uiTmpl.ExecuteTemplate(buff, name, data); err != nil {
		http.Error(w, "error executing template", http.StatusInternalServerError)
		slog.Error("error executing template", "err", err)
		return
	}
	if _, err := io.Copy(w, buff); err != nil {
		slog.Error("copy template data", "err", err)
		return
	}
}

func respErrf(w http.ResponseWriter, code int, f string, a ...any) {
	w.WriteHeader(code)
	respTmpl(w, "error", fmt.Sprintf(f, a...))
}

const listJobsPageSize = 20

type jobsListing struct {
	Filter    JobStatus
	Search    string
	Page      int
	PageCount int
	Total     int
	Jobs      []*Job
}

func listJobs(ctx context.Context, db *sql.DB, status JobStatus, search string, page, pageSize int) (jobsListing, error) {
	cond := sqlb.NewQuery("1")
	if search != "" {
		cond.Append("and source_path like ?", "%"+search+"%")
	}
	if status != "" {
		cond.Append("and status=?", status)
	}

	var total int
	if err := sqlb.ScanRow(ctx, db, sqlb.Values(&total), "select count(1) from jobs where ?", cond); err != nil {
		return jobsListing{}, fmt.Errorf("count total: %w", err)
	}

	pageCount := max(1, int(math.Ceil(float64(total)/float64(pageSize))))
	if page > pageCount-1 {
		page = 0 // reset if gone too far
	}

	var jobs []*Job
	if err := sqlb.ScanRows(ctx, db, sqlb.AppendPtr(&jobs), "select * from jobs where ? order by time desc limit ? offset ?", cond, pageSize, pageSize*page); err != nil {
		return jobsListing{}, fmt.Errorf("list jobs: %w", err)
	}

	return jobsListing{status, search, page, pageCount, total, jobs}, nil
}

func jobNotificationMessage(publicURL string, job Job) string {
	var status string
	if job.Error != "" {
		status = job.Error
	} else if job.Status != "" {
		status = string(job.Status)
	}

	url, _ := url.Parse(publicURL)
	url.Fragment = strconv.FormatUint(job.ID, 10)

	return fmt.Sprintf(`%s %s (%s) %s`,
		job.Operation, status, job.SourcePath, url)
}

//go:embed *.gohtml dist/*
var ui embed.FS
var uiTmpl = htmltemplate.Must(
	htmltemplate.
		New("template").
		Funcs(funcMap).
		ParseFS(ui, "*.gohtml"),
)

var funcMap = htmltemplate.FuncMap{
	"now":  func() int64 { return time.Now().UnixMilli() },
	"file": func(p string) string { ur, _ := url.Parse("file://"); ur.Path = p; return ur.String() },
	"url":  func(u string) htmltemplate.URL { return htmltemplate.URL(u) }, //nolint:gosec
	"join": func(delim string, items []string) string { return strings.Join(items, delim) },
	"pad0": func(amount, n int) string { return fmt.Sprintf("%0*d", amount, n) },
	"divc": func(a, b int) int { return int(math.Ceil(float64(a) / float64(b))) },
	"add":  func(a, b int) int { return a + b },
	"rangeN": func(n int) []int {
		r := make([]int, 0, n)
		for i := range n {
			r = append(r, i)
		}
		return r
	},
	"panic": func(msg string) string { panic(msg) },
}

func logJob(jobName string, args ...any) func() {
	slog.Info("starting job", append([]any{"job", jobName}, args...)...)
	return func() { slog.Info("stopping job", "job", jobName) }
}

type authMode string

const (
	authDisabled        authMode = "disabled"
	authBasicFromAPIKey authMode = "basic-auth-from-api-key" //nolint:gosec
)

const cookieKey = "api-key"

func authMiddleware(next http.Handler, mode authMode, apiKey string) http.Handler {
	switch mode {
	case authDisabled:
		return next
	case authBasicFromAPIKey:
	default:
		panic("invalid mode")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// exchange a valid basic auth request for a cookie that lasts 30 days
		if cookie, _ := r.Cookie(cookieKey); cookie != nil && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(apiKey)) == 1 {
			next.ServeHTTP(w, r)
			return
		}
		if _, key, _ := r.BasicAuth(); subtle.ConstantTimeCompare([]byte(key), []byte(apiKey)) == 1 {
			http.SetCookie(w, &http.Cookie{Name: cookieKey, Value: apiKey, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(30 * 24 * time.Hour)})
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "unauthorised", http.StatusUnauthorized)
	})
}

func apiKeyMiddleware(next http.Handler, apiKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, key, _ := r.BasicAuth(); subtle.ConstantTimeCompare([]byte(key), []byte(apiKey)) == 1 {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "unauthorised", http.StatusUnauthorized)
	})
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.InfoContext(r.Context(), "request", "url", r.URL)
		next.ServeHTTP(w, r)
	})
}

type broadcast[T any] struct {
	mu       sync.Mutex
	closed   bool
	channels map[chan T]struct{}
}

func (b *broadcast[T]) send(t T) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for c := range b.channels {
		c <- t
	}
}

func (b *broadcast[T]) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for c := range b.channels {
		close(c)
	}
	clear(b.channels)
	b.closed = true
}

func (b *broadcast[T]) receive(ctx context.Context, buff int) chan T {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.channels == nil {
		b.channels = map[chan T]struct{}{}
	}
	ch := make(chan T, buff)
	b.channels[ch] = struct{}{}
	context.AfterFunc(ctx, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if b.closed {
			return
		}
		delete(b.channels, ch)
		close(ch)
	})
	return ch
}
