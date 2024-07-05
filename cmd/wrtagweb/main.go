package main

import (
	"bytes"
	"context"
	"crypto/subtle"
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

	"github.com/r3labs/sse/v2"
	"github.com/timshannon/bolthold"
	"golang.org/x/sync/errgroup"

	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/cmd/internal/cmds"
	"go.senan.xyz/wrtag/notifications"
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

func main() {
	defer cmds.Logging()()
	cmds.WrapClient()
	var (
		cfg        = cmds.FlagConfig()
		listenAddr = flag.String("web-listen-addr", "", "listen addr for web interface")
		publicURL  = flag.String("web-public-url", "", "public url for web interface")
		apiKey     = flag.String("web-api-key", "", "api key for web interface")
		dbPath     = flag.String("web-db-path", "wrtag.db", "db path for web interface")
	)
	cmds.FlagParse()

	if *listenAddr == "" {
		slog.Error("need a listen addr")
		return
	}
	if *apiKey == "" {
		slog.Error("need an api key")
		return
	}

	db, err := bolthold.Open(*dbPath, 0600, nil)
	if err != nil {
		slog.Error("parsing path format template", "err", err)
		return
	}
	defer db.Close()

	sseServ := sse.New()
	sseServ.AutoReplay = false
	defer sseServ.Close()

	jobStream := sseServ.CreateStream("jobs")

	var (
		eventAllJobs = func() string { return "jobs" }
		eventJob     = func(id uint64) string { return fmt.Sprintf("job-%d", id) }
	)
	emit := func(events ...string) {
		for _, event := range events {
			sseServ.TryPublish(jobStream.ID, &sse.Event{Event: []byte(event), Data: []byte{0}})
		}
	}

	processJob := func(ctx context.Context, job *Job, ic wrtag.ImportCondition) error {
		job.Status = StatusInProgress

		if err := db.Update(job.ID, &job); err != nil {
			return fmt.Errorf("update job: %w", err)
		}
		emit(eventJob(job.ID), eventAllJobs())

		defer func() {
			if err := db.Update(job.ID, &job); err != nil {
				slog.Error("update job", "job_id", job.ID, "err", err)
				return
			}
			emit(eventJob(job.ID), eventAllJobs())
		}()

		job.Error = ""
		job.Status = StatusComplete

		var err error
		job.SearchResult, err = wrtag.ProcessDir(ctx, cfg, wrtagOperation(job.Operation), job.SourcePath, ic, job.UseMBID)
		if err != nil {
			job.Status = StatusError
			job.Error = err.Error()
			if errors.Is(err, wrtag.ErrScoreTooLow) {
				cfg.Notifications.Send(ctx, notifications.NeedsInput, jobNotificationMessage(*publicURL, job))
				job.Status = StatusNeedsInput
			}
			return nil
		}

		job.DestPath, err = wrtag.DestDir(&cfg.PathFormat, job.SearchResult.Release)
		if err != nil {
			return fmt.Errorf("gen dest dir: %w", err)
		}

		cfg.Notifications.Send(ctx, notifications.Complete, jobNotificationMessage(*publicURL, job))

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
	listJobs := func(search string, filter JobStatus, page int) (jobsListing, error) {
		q := &bolthold.Query{}
		if search != "" {
			q = q.And("SourcePath").MatchFunc(func(path string) (bool, error) {
				return strings.Contains(strings.ToLower(path), strings.ToLower(search)), nil
			})
		}
		if filter != "" {
			q = q.And("Status").Eq(filter)
		}

		total, err := db.Count(&Job{}, q)
		if err != nil {
			return jobsListing{}, err
		}

		pageCount := max(1, int(math.Ceil(float64(total)/float64(pageSize))))
		if page > pageCount-1 {
			page = 0 // reset if gone too far
		}

		q = q.
			Skip(pageSize * page).
			Limit(pageSize).
			SortBy("Time").
			Reverse()
		var jobs []*Job
		if err := db.Find(&jobs, q); err != nil {
			return jobsListing{}, err
		}

		return jobsListing{filter, search, page, pageCount, total, jobs}, nil
	}

	mux := http.NewServeMux()
	mux.Handle("GET /events", sseServ)

	mux.HandleFunc("GET /jobs", func(w http.ResponseWriter, r *http.Request) {
		search := r.URL.Query().Get("search")
		filter := JobStatus(r.URL.Query().Get("filter"))
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		jl, err := listJobs(search, filter, page)
		if err != nil {
			respErrf(w, http.StatusInternalServerError, "error listing jobs: %v", err)
			return
		}
		respTmpl(w, "jobs", jl)
	})

	mux.HandleFunc("POST /jobs", func(w http.ResponseWriter, r *http.Request) {
		operation, err := parseOperation(r.FormValue("operation"))
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
		job := Job{SourcePath: path, Operation: operation, Time: time.Now()}
		if err := db.Insert(bolthold.NextSequence(), &job); err != nil {
			http.Error(w, fmt.Sprintf("error saving job: %v", err), http.StatusInternalServerError)
			return
		}
		respTmpl(w, "job-import", nil)
		emit(eventAllJobs())
	})

	mux.HandleFunc("GET /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		var job Job
		if err := db.Get(uint64(id), &job); err != nil {
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
		if err := db.Get(uint64(id), &job); err != nil {
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
		if err := db.Delete(uint64(id), &Job{}); err != nil {
			respErrf(w, http.StatusInternalServerError, "error getting job")
			return
		}
		emit(eventAllJobs())
	})

	mux.HandleFunc("GET /dump", func(w http.ResponseWriter, r *http.Request) {
		var jobs []*Job
		if err := db.Find(&jobs, nil); err != nil {
			respErrf(w, http.StatusInternalServerError, "error listing jobs: %v", err)
			return
		}
		if err := json.NewEncoder(w).Encode(jobs); err != nil {
			respErrf(w, http.StatusInternalServerError, "error encoding jobs")
			return
		}
	})

	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		jl, err := listJobs("", "", 0)
		if err != nil {
			respErrf(w, http.StatusInternalServerError, "error listing jobs: %v", err)
			return
		}
		respTmpl(w, "index", jl)
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
		job := Job{SourcePath: path, Operation: operation, Time: time.Now()}
		if err := db.Insert(bolthold.NextSequence(), &job); err != nil {
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
			switch err := db.FindOne(&job, bolthold.Where("Status").Eq(StatusEnqueued)); {
			case errors.Is(err, bolthold.ErrNotFound):
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
	ID                   uint64    `boltholdKey:"ID"`
	Status               JobStatus `boltholdIndex:"Status"`
	Error                string
	Operation            Operation
	Time                 time.Time `boltholdIndex:"Time"`
	UseMBID              string
	SourcePath, DestPath string
	SearchResult         *wrtag.SearchResult
}

func jobNotificationMessage(publicURL string, job *Job) string {
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
