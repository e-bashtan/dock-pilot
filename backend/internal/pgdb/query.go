package pgdb

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ebash/dock-pilot/backend/internal/db"
	"github.com/ebash/dock-pilot/backend/internal/docker"
)

const (
	defaultSelectLimit = 100
	maxSelectLimit     = 500
)

type TableInfo struct {
	Name       string `json:"name"`
	ApproxRows int64  `json:"approx_rows"`
}

type QueryResult struct {
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
	SQL     string     `json:"sql"`
	Limit   int        `json:"limit"`
}

type SelectTableRequest struct {
	Table string `json:"table"`
	Limit int    `json:"limit"`
}

func (s *Service) ListPublicTables(ctx context.Context, instanceID, databaseID uuid.UUID) ([]TableInfo, error) {
	inst, dbRow, err := s.requireDatabase(ctx, instanceID, databaseID)
	if err != nil {
		return nil, err
	}

	result, err := s.querySQL(ctx, inst, dbRow.Name, `
SELECT c.relname AS name,
       COALESCE(s.n_live_tup, 0)::bigint AS approx_rows
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_catalog.pg_stat_user_tables s ON s.relid = c.oid
WHERE n.nspname = 'public'
  AND c.relkind = 'r'
ORDER BY c.relname`)
	if err != nil {
		return nil, err
	}

	out := make([]TableInfo, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) < 1 {
			continue
		}
		info := TableInfo{Name: row[0]}
		if len(row) > 1 {
			var n int64
			_, _ = fmt.Sscanf(row[1], "%d", &n)
			info.ApproxRows = n
		}
		out = append(out, info)
	}
	return out, nil
}

func (s *Service) SelectFromTable(ctx context.Context, instanceID, databaseID uuid.UUID, req SelectTableRequest) (QueryResult, error) {
	inst, dbRow, err := s.requireDatabase(ctx, instanceID, databaseID)
	if err != nil {
		return QueryResult{}, err
	}

	tableIdent, err := quoteIdent(req.Table)
	if err != nil {
		return QueryResult{}, err
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultSelectLimit
	}
	if limit > maxSelectLimit {
		limit = maxSelectLimit
	}

	sql := fmt.Sprintf("SELECT * FROM public.%s LIMIT %d", tableIdent, limit)
	result, err := s.querySQL(ctx, inst, dbRow.Name, sql)
	if err != nil {
		return QueryResult{}, err
	}
	result.SQL = sql
	result.Limit = limit
	return result, nil
}

func (s *Service) requireDatabase(ctx context.Context, instanceID, databaseID uuid.UUID) (db.PdbInstance, db.PdbDatabase, error) {
	inst, err := s.requireInstance(ctx, instanceID)
	if err != nil {
		return db.PdbInstance{}, db.PdbDatabase{}, err
	}
	row, err := s.queries.GetPgDatabase(ctx, databaseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.PdbInstance{}, db.PdbDatabase{}, ErrNotFound
		}
		return db.PdbInstance{}, db.PdbDatabase{}, err
	}
	if row.InstanceID != instanceID {
		return db.PdbInstance{}, db.PdbDatabase{}, ErrNotFound
	}
	return inst, row, nil
}

func (s *Service) querySQL(ctx context.Context, inst db.PdbInstance, database, sql string) (QueryResult, error) {
	password, err := s.adminPassword(inst)
	if err != nil {
		return QueryResult{}, fmt.Errorf("decrypt admin password: %w", err)
	}

	var stdout, stderr bytes.Buffer
	code, err := s.docker.Exec(ctx, docker.ExecOptions{
		ContainerName: s.containerName(inst),
		Cmd: []string{
			"psql",
			"-v", "ON_ERROR_STOP=1",
			"-U", inst.AdminUser,
			"-d", database,
			"--csv",
			"-c", sql,
		},
		Env: []string{"PGPASSWORD=" + password},
	}, nil, &stdout, &stderr)
	if err != nil {
		return QueryResult{}, err
	}
	if code != 0 {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = fmt.Sprintf("psql exit %d", code)
		}
		return QueryResult{}, fmt.Errorf("%s", msg)
	}

	return parseCSVResult(stdout.Bytes())
}

func parseCSVResult(raw []byte) (QueryResult, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return QueryResult{Columns: []string{}, Rows: [][]string{}}, nil
	}

	reader := csv.NewReader(bytes.NewReader(raw))
	reader.ReuseRecord = false
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return QueryResult{}, fmt.Errorf("parse query result: %w", err)
	}
	if len(records) == 0 {
		return QueryResult{Columns: []string{}, Rows: [][]string{}}, nil
	}

	columns := records[0]
	rows := make([][]string, 0, len(records)-1)
	for _, rec := range records[1:] {
		// Normalize short rows (CSV can omit trailing empties).
		row := make([]string, len(columns))
		copy(row, rec)
		for i := len(rec); i < len(columns); i++ {
			row[i] = ""
		}
		rows = append(rows, row)
	}
	return QueryResult{Columns: columns, Rows: rows}, nil
}
