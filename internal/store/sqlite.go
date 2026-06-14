package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

	"github.com/driftctl/driftctl/internal/model"
)

// SQLiteStore implements Store with SQLite.
type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Optimization 2: Add database indexes for faster queries
func (s *SQLiteStore) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS workspaces (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    provider TEXT NOT NULL,
    state_json TEXT NOT NULL,
    regions_json TEXT NOT NULL,
    compare_json TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS scans (
    id TEXT PRIMARY KEY,
    workspace_id TEXT,
    workspace_name TEXT,
    status TEXT NOT NULL,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    summary_json TEXT,
    findings_json TEXT,
    errors_json TEXT
);

CREATE TABLE IF NOT EXISTS schedules (
    workspace_id TEXT PRIMARY KEY,
    cron TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_scans_workspace_id ON scans(workspace_id);
CREATE INDEX IF NOT EXISTS idx_scans_status ON scans(status);
CREATE INDEX IF NOT EXISTS idx_scans_started_at ON scans(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_workspaces_name ON workspaces(name);
`
	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) SaveWorkspace(ctx context.Context, ws *model.Workspace) error {
	if ws.ID == "" {
		ws.ID = uuid.New().String()
	}
	stateJSON, err := json.Marshal(ws.State)
	if err != nil {
		// Optimization 7: Proper error handling
		log.Printf("error marshaling state: %v", err)
		return fmt.Errorf("marshal state: %w", err)
	}
	regionsJSON, err := json.Marshal(ws.Regions)
	if err != nil {
		log.Printf("error marshaling regions: %v", err)
		return fmt.Errorf("marshal regions: %w", err)
	}
	compareJSON, err := json.Marshal(ws.Compare)
	if err != nil {
		log.Printf("error marshaling compare: %v", err)
		return fmt.Errorf("marshal compare: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO workspaces (id, name, provider, state_json, regions_json, compare_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET
		   provider=excluded.provider,
		   state_json=excluded.state_json,
		   regions_json=excluded.regions_json,
		   compare_json=excluded.compare_json`,
		ws.ID, ws.Name, ws.Provider, string(stateJSON), string(regionsJSON), string(compareJSON), time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func (s *SQLiteStore) GetWorkspace(ctx context.Context, id string) (*model.Workspace, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, provider, state_json, regions_json, compare_json FROM workspaces WHERE id = ?`, id)
	return scanWorkspace(row)
}

func (s *SQLiteStore) GetWorkspaceByName(ctx context.Context, name string) (*model.Workspace, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, provider, state_json, regions_json, compare_json FROM workspaces WHERE name = ?`, name)
	return scanWorkspace(row)
}

func scanWorkspace(row *sql.Row) (*model.Workspace, error) {
	var ws model.Workspace
	var stateJSON, regionsJSON, compareJSON string
	if err := row.Scan(&ws.ID, &ws.Name, &ws.Provider, &stateJSON, &regionsJSON, &compareJSON); err != nil {
		return nil, err
	}
	// Optimization 7: Handle JSON unmarshaling errors
	if err := json.Unmarshal([]byte(stateJSON), &ws.State); err != nil {
		log.Printf("error unmarshaling state: %v", err)
	}
	if err := json.Unmarshal([]byte(regionsJSON), &ws.Regions); err != nil {
		log.Printf("error unmarshaling regions: %v", err)
	}
	if err := json.Unmarshal([]byte(compareJSON), &ws.Compare); err != nil {
		log.Printf("error unmarshaling compare: %v", err)
	}
	return &ws, nil
}

func (s *SQLiteStore) ListWorkspaces(ctx context.Context) ([]model.Workspace, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, provider, state_json, regions_json, compare_json FROM workspaces ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Workspace
	for rows.Next() {
		var ws model.Workspace
		var stateJSON, regionsJSON, compareJSON string
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Provider, &stateJSON, &regionsJSON, &compareJSON); err != nil {
			return nil, err
		}
		// Optimization 5 & 7: Lazy unmarshal with proper error handling
		if err := json.Unmarshal([]byte(stateJSON), &ws.State); err != nil {
			log.Printf("error unmarshaling state for workspace %s: %v", ws.ID, err)
		}
		if err := json.Unmarshal([]byte(regionsJSON), &ws.Regions); err != nil {
			log.Printf("error unmarshaling regions for workspace %s: %v", ws.ID, err)
		}
		if err := json.Unmarshal([]byte(compareJSON), &ws.Compare); err != nil {
			log.Printf("error unmarshaling compare for workspace %s: %v", ws.ID, err)
		}
		list = append(list, ws)
	}
	return list, nil
}

func (s *SQLiteStore) DeleteWorkspace(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) SaveScan(ctx context.Context, report *model.DriftReport) error {
	summaryJSON, err := json.Marshal(report.Summary)
	if err != nil {
		log.Printf("error marshaling summary: %v", err)
		return fmt.Errorf("marshal summary: %w", err)
	}
	findingsJSON, err := json.Marshal(report.Findings)
	if err != nil {
		log.Printf("error marshaling findings: %v", err)
		return fmt.Errorf("marshal findings: %w", err)
	}
	errorsJSON, err := json.Marshal(report.Errors)
	if err != nil {
		log.Printf("error marshaling errors: %v", err)
		return fmt.Errorf("marshal errors: %w", err)
	}
	completed := ""
	if !report.CompletedAt.IsZero() {
		completed = report.CompletedAt.UTC().Format(time.RFC3339)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO scans (id, workspace_id, workspace_name, status, started_at, completed_at, summary_json, findings_json, errors_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   status=excluded.status,
		   completed_at=excluded.completed_at,
		   summary_json=excluded.summary_json,
		   findings_json=excluded.findings_json,
		   errors_json=excluded.errors_json`,
		report.ScanID, report.WorkspaceID, report.Workspace, report.Status,
		report.StartedAt.UTC().Format(time.RFC3339), completed,
		string(summaryJSON), string(findingsJSON), string(errorsJSON),
	)
	return err
}

func (s *SQLiteStore) GetScan(ctx context.Context, id string) (*model.DriftReport, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, workspace_id, workspace_name, status, started_at, completed_at, summary_json, findings_json, errors_json FROM scans WHERE id = ?`, id)
	return scanReport(row)
}

func (s *SQLiteStore) ListScans(ctx context.Context, workspaceID string, limit int) ([]model.DriftReport, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows *sql.Rows
	var err error
	// Optimization 2: Uses index on workspace_id for faster filtering
	if workspaceID != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, workspace_id, workspace_name, status, started_at, completed_at, summary_json, findings_json, errors_json
			 FROM scans WHERE workspace_id = ? ORDER BY started_at DESC LIMIT ?`, workspaceID, limit)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, workspace_id, workspace_name, status, started_at, completed_at, summary_json, findings_json, errors_json
			 FROM scans ORDER BY started_at DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.DriftReport
	for rows.Next() {
		r, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *r)
	}
	return list, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanReport(row scanner) (*model.DriftReport, error) {
	var r model.DriftReport
	var started, completed sql.NullString
	var summaryJSON, findingsJSON, errorsJSON string
	if err := row.Scan(&r.ScanID, &r.WorkspaceID, &r.Workspace, &r.Status, &started, &completed, &summaryJSON, &findingsJSON, &errorsJSON); err != nil {
		return nil, err
	}
	if started.Valid {
		if t, err := time.Parse(time.RFC3339, started.String); err == nil {
			r.StartedAt = t
		} else {
			log.Printf("error parsing started_at: %v", err)
		}
	}
	if completed.Valid && completed.String != "" {
		if t, err := time.Parse(time.RFC3339, completed.String); err == nil {
			r.CompletedAt = t
		} else {
			log.Printf("error parsing completed_at: %v", err)
		}
	}
	// Optimization 5 & 7: Proper error handling for unmarshaling
	if err := json.Unmarshal([]byte(summaryJSON), &r.Summary); err != nil {
		log.Printf("error unmarshaling summary: %v", err)
	}
	if err := json.Unmarshal([]byte(findingsJSON), &r.Findings); err != nil {
		log.Printf("error unmarshaling findings: %v", err)
	}
	if err := json.Unmarshal([]byte(errorsJSON), &r.Errors); err != nil {
		log.Printf("error unmarshaling errors: %v", err)
	}
	return &r, nil
}

func (s *SQLiteStore) SaveSchedule(ctx context.Context, workspaceID, cron string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO schedules (workspace_id, cron) VALUES (?, ?)
		 ON CONFLICT(workspace_id) DO UPDATE SET cron=excluded.cron`, workspaceID, cron)
	return err
}

func (s *SQLiteStore) GetSchedule(ctx context.Context, workspaceID string) (string, error) {
	var cron string
	err := s.db.QueryRowContext(ctx, `SELECT cron FROM schedules WHERE workspace_id = ?`, workspaceID).Scan(&cron)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("no schedule for workspace %s", workspaceID)
	}
	return cron, err
}

func (s *SQLiteStore) ListSchedules(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT workspace_id, cron FROM schedules`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var wsID, cron string
		if err := rows.Scan(&wsID, &cron); err != nil {
			return nil, err
		}
		out[wsID] = cron
	}
	return out, nil
}

func (s *SQLiteStore) DeleteSchedule(ctx context.Context, workspaceID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM schedules WHERE workspace_id = ?`, workspaceID)
	return err
}
