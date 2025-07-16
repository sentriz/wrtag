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
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.senan.xyz/wrtag"
	wrtagflag "go.senan.xyz/wrtag/cmd/internal/wrtagflag"
	"go.senan.xyz/wrtag/cmd/internal/wrtaglog"
	"go.senan.xyz/wrtag/researchlink"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/rogpeppe/go-internal/txtar"
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
		notifications       = wrtagflag.Notifications()
		researchLinkQuerier = wrtagflag.ResearchLinks()
		apiKey              = flag.String("web-api-key", "", "API key for web interface")
		listenAddr          = flag.String("web-listen-addr", ":7373", "Listen address for web interface (optional)")
		dbPath              = flag.String("web-db-path", "", "Path to persistent database path for web interface (optional)")
		publicURL           = flag.String("web-public-url", "", "Public URL for web interface (optional)")
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

	if lev := slog.LevelDebug; slog.Default().Enabled(context.Background(), lev) {
		sqlb.SetLog(func(ctx context.Context, typ string, duration time.Duration, query string) {
			slog.Log(ctx, lev, typ, "took", duration, "query", query)
		})
	}

	if err := dbMigrate(context.Background(), db); err != nil {
		slog.Error("migrate db", "err", err)
		return
	}

	var sse broadcast[uint64]

	processNextJob := func(ctx context.Context) error {
		var job Job
		err := sqlb.ScanRow(ctx, db, &job, "update jobs set status=? where id=(select id from jobs where status=? limit 1) returning *", StatusInProgress, StatusEnqueued)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}

		sse.send(job.ID)
		defer sse.send(job.ID)

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

		if searchResult != nil && searchResult.Release != nil {
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

		switch job.Status {
		case StatusComplete:
			go notifications.Send(context.WithoutCancel(ctx), notifComplete, jobNotificationMessage(*publicURL, job))
		case StatusNeedsInput:
			go notifications.Send(context.WithoutCancel(ctx), notifNeedsInput, jobNotificationMessage(*publicURL, job))
		}

		return nil
	}

	var buffPool = sync.Pool{
		New: func() any { return new(bytes.Buffer) },
	}
	respTmpl := func(w http.ResponseWriter, name string, data any) {
		buff, _ := buffPool.Get().(*bytes.Buffer)
		defer buffPool.Put(buff)
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
	respErrf := func(w http.ResponseWriter, code int, f string, a ...any) {
		w.WriteHeader(code)
		respTmpl(w, "error", fmt.Sprintf(f, a...))
	}

	type jobsListing struct {
		Filter    JobStatus
		Search    string
		Page      int
		PageCount int
		Total     int
		Jobs      []*Job
	}

	const pageSize = 20
	listJobs := func(ctx context.Context, status JobStatus, search string, page int) (jobsListing, error) {
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
		if err := sqlb.ScanPtr(ctx, db, &jobs, "select * from jobs where ? order by time desc limit ? offset ?", cond, pageSize, pageSize*page); err != nil {
			return jobsListing{}, fmt.Errorf("list jobs: %w", err)
		}

		return jobsListing{status, search, page, pageCount, total, jobs}, nil
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /sse", func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		rc.Flush()

		for id := range sse.receive(r.Context(), 0) {
			fmt.Fprintf(w, "data: %d\n\n", id)
			rc.Flush()
		}
	})

	mux.HandleFunc("GET /jobs", func(w http.ResponseWriter, r *http.Request) {
		search := r.URL.Query().Get("search")
		filter := JobStatus(r.URL.Query().Get("filter"))
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		jl, err := listJobs(r.Context(), filter, search, page)
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

		var job Job
		if err := sqlb.ScanRow(r.Context(), db, &job, "insert into jobs (source_path, operation, time) values (?, ?, ?) returning *", path, operationStr, time.Now()); err != nil {
			http.Error(w, fmt.Sprintf("error saving job: %v", err), http.StatusInternalServerError)
			return
		}

		respTmpl(w, "job-import", struct{ Operation string }{Operation: operationStr})

		sse.send(0)
	})

	mux.HandleFunc("GET /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		var job Job
		if err := sqlb.ScanRow(r.Context(), db, &job, "select * from jobs where id=?", id); err != nil {
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
			useMBID = filepath.Base(useMBID) // accept release URL
		}

		var job Job
		if err := sqlb.ScanRow(r.Context(), db, &job, "update jobs set confirm=?, use_mbid=?, status=? where id=? and status<>? returning *", confirm, useMBID, StatusEnqueued, id, StatusInProgress); err != nil {
			respErrf(w, http.StatusInternalServerError, "error getting job")
			return
		}

		respTmpl(w, "job", job)

		sse.send(0)
	})

	mux.HandleFunc("DELETE /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		if err := sqlb.Exec(r.Context(), db, "delete from jobs where id=? and status<>?", id, StatusInProgress); err != nil {
			respErrf(w, http.StatusInternalServerError, "error getting job")
			return
		}
		sse.send(0)
	})

	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		jl, err := listJobs(r.Context(), "", "", 0)
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

	// external API
	mux.HandleFunc("POST /op/{operation}", func(w http.ResponseWriter, r *http.Request) {
		operationStr := r.PathValue("operation")
		if _, err := wrtagflag.OperationByName(operationStr, false); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		path := r.FormValue("path")
		if path == "" {
			http.Error(w, "no path provided", http.StatusBadRequest)
			return
		}
		if !filepath.IsAbs(path) {
			http.Error(w, "filepath not abs", http.StatusBadRequest)
			return
		}
		path = filepath.Clean(path)

		if err := sqlb.Exec(r.Context(), db, "insert into jobs (source_path, operation, time) values (?, ?, ?)", path, operationStr, time.Now()); err != nil {
			http.Error(w, fmt.Sprintf("error saving job: %v", err), http.StatusInternalServerError)
			return
		}

		sse.send(0)
	})

	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/*", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	errgrp, ctx := errgroup.WithContext(ctx)

	errgrp.Go(func() error {
		defer logJob("http", "addr", *listenAddr)()

		var h http.Handler
		h = mux
		h = authMiddleware(h, *apiKey)
		h = logMiddleware(h)

		server := &http.Server{Addr: *listenAddr, Handler: h, ReadHeaderTimeout: 2 * time.Second}

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

	errgrp.Go(func() error {
		defer logJob("process jobs")()

		// restart old jobs just in case the process was killed abruptly last time
		if err := sqlb.Exec(ctx, db, "update jobs set status=? where status=?", StatusEnqueued, StatusInProgress); err != nil {
			return err
		}

		t := time.NewTicker(1 * time.Second)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil
			case <-t.C:
				if err := processNextJob(ctx); err != nil {
					return fmt.Errorf("next job: %w", err)
				}
			}
		}
	})

	if err := errgrp.Wait(); err != nil {
		slog.Error("wait for jobs", "err", err)
		return
	}
}

type JobStatus string

const (
	StatusEnqueued   JobStatus = ""
	StatusInProgress JobStatus = "in-progress"
	StatusNeedsInput JobStatus = "needs-input"
	StatusError      JobStatus = "error"
	StatusComplete   JobStatus = "complete"
)

const (
	OperationCopy = "copy"
	OperationMove = "move"
)

//go:generate go tool sqlbgen Job
type Job struct {
	ID            uint64
	Status        JobStatus
	Error         string
	Operation     string
	Time          time.Time
	UseMBID       string
	SourcePath    string
	DestPath      string
	SearchResult  sqlb.JSON[*wrtag.SearchResult]
	ResearchLinks sqlb.JSON[[]researchlink.SearchResult]
	Confirm       bool
}

//go:embed schema.sql
var schema []byte

func dbMigrate(ctx context.Context, db *sql.DB) error {
	var nextVer int
	if err := sqlb.ScanRow(ctx, db, sqlb.Values(&nextVer), "pragma user_version"); err != nil {
		return fmt.Errorf("get schema version: %w", err)
	}

	migrations := txtar.Parse(schema)
	for i := nextVer; i < len(migrations.Files); i++ {
		migration := migrations.Files[i]
		slog.InfoContext(ctx, "running migration", "name", migration.Name, "query", string(migration.Data))

		if err := sqlb.Exec(ctx, db, string(migration.Data)); err != nil {
			return fmt.Errorf("run migration %d: %w", i, err)
		}
		if err := sqlb.Exec(ctx, db, fmt.Sprintf("pragma user_version = %d", i+1)); err != nil {
			return fmt.Errorf("run migration %d: %w", i, err)
		}
	}
	return nil
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

const cookieKey = "api-key"

func authMiddleware(next http.Handler, apiKey string) http.Handler {
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
