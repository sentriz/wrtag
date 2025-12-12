package main

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"log/slog"
	"time"

	"github.com/rogpeppe/go-internal/txtar"
	"go.senan.xyz/sqlb"
	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/researchlink"
)

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

//go:generate go tool sqlbgen -to schema.gen.go -generated ID Job
type Job struct {
	ID            uint64
	Status        JobStatus
	Error         string
	Operation     string
	Time          time.Time
	UpdatedTime   sql.NullTime
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
