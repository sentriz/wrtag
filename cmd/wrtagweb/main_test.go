package main

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/txtar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfirm(t *testing.T) {
	t.Parallel()

	for v, exp := range map[string]ConfirmMode{
		"accept": ConfirmAccept,
		"true":   ConfirmAccept,
		"1":      ConfirmAccept,
		"score":  ConfirmScore,
		"false":  ConfirmScore,
		"0":      ConfirmScore,
		"":       ConfirmScore,
		"nope":   ConfirmScore,
		"manual": ConfirmManual,
	} {
		assert.Equal(t, exp, parseConfirm(v), "confirm=%q", v)
	}
}

func TestMigrateConfirmMode(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	db, err := sql.Open("sqlite3", "file:"+filepath.Join(t.TempDir(), "jobs.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// set up the schema as it was before confirm became a text mode
	migrations := txtar.Parse(schema)
	before := -1
	for i, m := range migrations.Files {
		if m.Name == "2026.10.16 confirm bool to text mode" {
			before = i
		}
	}
	require.NotEqual(t, -1, before)
	for _, m := range migrations.Files[:before] {
		_, err := db.ExecContext(ctx, string(m.Data))
		require.NoError(t, err)
	}
	_, err = db.ExecContext(ctx, fmt.Sprintf("pragma user_version = %d", before))
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `insert into jobs (operation, time, source_path, confirm) values
		('copy', '2026-01-01', '/unconfirmed', false),
		('copy', '2026-01-01', '/confirmed', true)`)
	require.NoError(t, err)

	require.NoError(t, dbMigrate(ctx, db))

	rows, err := db.QueryContext(ctx, "select source_path, confirm from jobs order by id")
	require.NoError(t, err)
	t.Cleanup(func() { _ = rows.Close() })

	got := map[string]ConfirmMode{}
	for rows.Next() {
		var path string
		var confirm ConfirmMode
		require.NoError(t, rows.Scan(&path, &confirm))
		got[path] = confirm
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, map[string]ConfirmMode{"/unconfirmed": ConfirmScore, "/confirmed": ConfirmAccept}, got)
}
