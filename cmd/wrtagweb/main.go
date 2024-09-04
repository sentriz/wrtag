package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql"
	"database/sql/driver"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	htmltemplate "html/template"
	"io"
	"log/slog"
	"math"
	"net/http"
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
	"go.senan.xyz/wrtag/cmd/internal/cmds"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/r3labs/sse/v2"
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
	defer cmds.Logging()()
	cmds.WrapClient()
	var (
		cfg        = cmds.WrtagConfig()
		listenAddr = flag.String("web-listen-addr", "", "listen addr for web interface")
		publicURL  = flag.String("web-public-url", "", "public url for web interface")
		apiKey     = flag.String("web-api-key", "", "api key for web interface")
		dbPath     = flag.String("web-db-path", "wrtag.db", "db path for web interface")
	)
	cmds.Parse()

	if *listenAddr == "" {
		slog.Error("need a listen addr")
		return
	}
	if *apiKey == "" {
		slog.Error("need an api key")
		return
	}

	dbURI, _ := url.Parse("file://?cache=shared&_fk=1")
	dbURI.Opaque = *dbPath
	db, err := sql.Open("sqlite3", dbURI.String())
	if err != nil {
		slog.Error("open db template", "err", err)
		return
	}
	defer db.Close()

	if err := jobsMigrate(context.Background(), db); err != nil {
		slog.Error("migrate db", "err", err)
		return
	}

	sseServ := sse.New()
	sseServ.AutoReplay = false
	defer sseServ.Close()

	jobStream := sseServ.CreateStream("jobs")

	var (
		eventAllJobs = func() string { return "jobs" }
		eventJob     = func(id uint64) string { return fmt.Sprintf("job-%d", id) }
	)

	emit := func(events ...string) {
		for _, eventName := range events {
			sseServ.Publish(jobStream.ID, &sse.Event{Event: []byte(eventName), Data: []byte{0}})
		}
		// remove when hx-trigger="sse:jobs queue:all" works again
		time.Sleep(100 * time.Millisecond)
	}

	processJob := func(ctx context.Context, job *Job, ic wrtag.ImportCondition) error {
		job.Status = StatusInProgress
		if err := sqlb.ScanRow(ctx, db, job, "update jobs set ? where id=? returning *", sqlb.UpdateSQL(job), job.ID); err != nil {
			return fmt.Errorf("update job: %w", err)
		}
		emit(eventJob(job.ID), eventAllJobs())

		defer func() {
			if err := sqlb.ScanRow(ctx, db, job, "update jobs set ? where id=? returning *", sqlb.UpdateSQL(job), job.ID); err != nil {
				slog.Error("update job", "job_id", job.ID, "err", err)
				return
			}
			emit(eventJob(job.ID), eventAllJobs())
		}()

		job.Error = ""
		job.Status = StatusComplete

		searchResult, err := wrtag.ProcessDir(ctx, cfg, wrtagOperation(job.Operation), job.SourcePath, ic, job.UseMBID)
		if err != nil {
			job.Status = StatusError
			job.Error = err.Error()
			if errors.Is(err, wrtag.ErrScoreTooLow) {
				job.Status = StatusNeedsInput
				go cfg.Notifications.Send(ctx, notifNeedsInput, jobNotificationMessage(*publicURL, *job))
			}
			return nil
		}
		job.SearchResult = &SearchResult{SearchResult: *searchResult}

		job.DestPath, err = wrtag.DestDir(&cfg.PathFormat, job.SearchResult.Release)
		if err != nil {
			return fmt.Errorf("gen dest dir: %w", err)
		}

		go cfg.Notifications.Send(ctx, notifComplete, jobNotificationMessage(*publicURL, *job))

		// either if this was a copy or move job, subsequent re-imports should just be a move so we can retag
		job.Operation = OperationMove
		job.SourcePath = job.DestPath

		return nil
	}

	var buffPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}
	respTmpl := func(w http.ResponseWriter, name string, data any) {
		buff := buffPool.Get().(*bytes.Buffer)
		defer buffPool.Put(buff)
		buff.Reset()

		if err := uiTmpl.ExecuteTemplate(w, name, data); err != nil {
			slog.Error("in template", "err", err)
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
		var whereb sqlb.Query
		whereb.Append("1")
		if search != "" {
			whereb.Append("and source_path like ?", search)
		}
		if status != "" {
			whereb.Append("and status=?", status)
		}

		var total int
		if err := sqlb.ScanRow(ctx, db, sqlb.Primative(&total), "select count(1) from jobs where ?", whereb); err != nil {
			return jobsListing{}, fmt.Errorf("count total: %w", err)
		}

		pageCount := max(1, int(math.Ceil(float64(total)/float64(pageSize))))
		if page > pageCount-1 {
			page = 0 // reset if gone too far
		}

		var q sqlb.Query
		q.Append("select * from jobs where ?", whereb)
		q.Append("order by time desc")
		q.Append("limit ? offset ?", pageSize, pageSize*page)

		query, values := q.SQL()

		var jobs []*Job
		if err := sqlb.Scan(ctx, db, &jobs, query, values...); err != nil {
			return jobsListing{}, fmt.Errorf("list jobs: %w", err)
		}

		return jobsListing{status, search, page, pageCount, total, jobs}, nil
	}

	mux := http.NewServeMux()
	mux.Handle("GET /events", sseServ)

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
		operation, err := parseOperation(operationStr)
		if err != nil {
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
		if err := sqlb.ScanRow(r.Context(), db, &job, "insert into jobs (source_path, operation, time) values (?, ?, ?) returning *", path, operation, time.Now()); err != nil {
			http.Error(w, fmt.Sprintf("error saving job: %v", err), http.StatusInternalServerError)
			return
		}
		respTmpl(w, "job-import", struct{ Operation string }{Operation: operationStr})
		emit(eventAllJobs())
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
		var ic wrtag.ImportCondition
		if confirm, _ := strconv.ParseBool(r.FormValue("confirm")); confirm {
			ic = wrtag.Confirm
		}
		useMBID := r.FormValue("mbid")
		if strings.Contains(useMBID, "/") {
			useMBID = filepath.Base(useMBID) // accept release URL
		}

		var job Job
		if err := sqlb.ScanRow(r.Context(), db, &job, "select * from jobs where id=?", id); err != nil {
			respErrf(w, http.StatusInternalServerError, "error getting job")
			return
		}
		job.UseMBID = useMBID
		if err := processJob(r.Context(), &job, ic); err != nil {
			respErrf(w, http.StatusInternalServerError, "error in job")
			return
		}
		respTmpl(w, "job", job)
		emit(eventAllJobs())
	})

	mux.HandleFunc("DELETE /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		if _, err := db.ExecContext(r.Context(), "delete from jobs where id=?", id); err != nil {
			respErrf(w, http.StatusInternalServerError, "error getting job")
			return
		}
		emit(eventAllJobs())
	})

	mux.HandleFunc("GET /dump", func(w http.ResponseWriter, r *http.Request) {
		var jobs []*Job
		if err := sqlb.Scan(r.Context(), db, &jobs, "select * from jobs"); err != nil {
			respErrf(w, http.StatusInternalServerError, "error listing jobs: %v", err)
			return
		}
		if err := json.NewEncoder(w).Encode(jobs); err != nil {
			respErrf(w, http.StatusInternalServerError, "error encoding jobs")
			return
		}
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
			jl, "copy",
		})
	})

	mux.Handle("/", http.FileServer(http.FS(ui)))

	// external API
	mux.HandleFunc("POST /op/{operation}", func(w http.ResponseWriter, r *http.Request) {
		operation, err := parseOperation(r.PathValue("operation"))
		if err != nil {
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
		var job Job
		if err := sqlb.ScanRow(r.Context(), db, &job, "insert into jobs (source_path, operation, time) values (?, ?, ?) returning *", path, operation, time.Now()); err != nil {
			http.Error(w, fmt.Sprintf("error saving job: %v", err), http.StatusInternalServerError)
			return
		}
		emit(eventAllJobs())
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	errgrp, ctx := errgroup.WithContext(ctx)

	errgrp.Go(func() error {
		defer logJob("http")()

		mw := authMiddleware(*apiKey)
		server := &http.Server{Addr: *listenAddr, Handler: mw(mux)}
		errgrp.Go(func() error { <-ctx.Done(); return server.Shutdown(context.Background()) })
		errgrp.Go(func() error { <-ctx.Done(); sseServ.Close(); return nil })

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	errgrp.Go(func() error {
		defer logJob("process jobs")()

		tick := func(ctx context.Context) error {
			var job Job
			switch err := sqlb.ScanRow(ctx, db, &job, "select * from jobs where status=? limit 1", StatusEnqueued); {
			case errors.Is(err, sql.ErrNoRows):
				return nil
			case err != nil:
				return fmt.Errorf("find next job: %w", err)
			}
			return processJob(ctx, &job, wrtag.HighScore)
		}

		ctxTick(ctx, 2*time.Second, func() {
			if err := tick(ctx); err != nil {
				slog.ErrorContext(ctx, "in job", "err", err)
				return
			}
		})
		return nil
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

type Operation string

const (
	OperationCopy Operation = "copy"
	OperationMove Operation = "move"
)

func parseOperation(str string) (Operation, error) {
	switch str {
	case "copy":
		return OperationCopy, nil
	case "move":
		return OperationMove, nil
	default:
		return "", fmt.Errorf("invalid operation %q", str)
	}
}

func wrtagOperation(op Operation) wrtag.FileSystemOperation {
	switch op {
	case OperationCopy:
		return wrtag.Copy{}
	case OperationMove:
		return wrtag.Move{}
	default:
		panic(fmt.Errorf("unknown operation: %q", op))
	}
}

type Job struct {
	ID                   uint64
	Status               JobStatus
	Error                string
	Operation            Operation
	Time                 time.Time
	UseMBID              string
	SourcePath, DestPath string
	SearchResult         *SearchResult
}

func (Job) PrimaryKey() string {
	return "id"
}
func (j Job) Values() []sql.NamedArg {
	return []sql.NamedArg{
		sql.Named("id", j.ID),
		sql.Named("status", j.Status),
		sql.Named("error", j.Error),
		sql.Named("operation", j.Operation),
		sql.Named("time", j.Time),
		sql.Named("use_mbid", j.UseMBID),
		sql.Named("source_path", j.SourcePath),
		sql.Named("dest_path", j.DestPath),
		sql.Named("search_result", j.SearchResult),
	}
}
func (j *Job) ScanFrom(rows *sql.Rows) error {
	return rows.Scan(&j.ID, &j.Status, &j.Error, &j.Operation, &j.Time, &j.UseMBID, &j.SourcePath, &j.DestPath, &j.SearchResult)
}

func jobsMigrate(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		create table if not exists jobs (
			id            integer primary key autoincrement,
			status        text not null default "",
			error         text not null default "",
			operation     text not null,
			time          timestamp not null,
			use_mbid      text not null default "",
			source_path   text not null,
			dest_path     text not null default "",
			search_result jsonb
		);

		create index if not exists idx_jobs_status on jobs (status);
		create index if not exists idx_jobs_source_path on jobs (source_path);
	`)
	return err
}

func jobNotificationMessage(publicURL string, job Job) string {
	var status string
	if job.Error != "" {
		status = job.Error
	} else if job.Status != "" {
		status = string(job.Status)
	}

	url, _ := url.Parse(publicURL)
	url.Fragment = fmt.Sprint(job.ID)

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
	"url":  func(u string) htmltemplate.URL { return htmltemplate.URL(u) },
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

func logJob(jobName string) func() {
	slog.Info("starting job", "job", jobName)
	return func() { slog.Info("stopping job", "job", jobName) }
}

func authMiddleware(apiKey string) func(next http.Handler) http.Handler {
	const cookieKey = "api-key"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slog.Info("request", "url", r.URL)

			// exchange a valid basic basic auth request for a cookie that lasts 30 days
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
}

func ctxTick(ctx context.Context, interval time.Duration, f func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f()
		}
	}
}

type SearchResult struct {
	wrtag.SearchResult
}

func (sr *SearchResult) Scan(v any) error {
	if v == nil {
		return nil
	}
	b, ok := v.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, &sr.SearchResult)
}

func (sr SearchResult) Value() (driver.Value, error) {
	return json.Marshal(sr.SearchResult)
}
