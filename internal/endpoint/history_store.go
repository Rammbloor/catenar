package endpoint

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"catenar/internal/contracts"
)

type HistoryStore interface {
	SaveCallSummary(ctx context.Context, summary contracts.HistoryCallSummary) error
	ListCalls(ctx context.Context, input contracts.HistoryListInput) ([]contracts.HistoryCallSummary, error)
	GetCallSummary(ctx context.Context, callID string) (contracts.HistoryCallSummary, error)
}

type sqliteHistoryStore struct {
	db *sql.DB
}

func newSQLiteHistoryStore(baseDir string) (HistoryStore, error) {
	historyDir := filepath.Join(baseDir, "history")
	if err := os.MkdirAll(historyDir, 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", filepath.Join(historyDir, "history-v1.sqlite"))
	if err != nil {
		return nil, err
	}

	store := &sqliteHistoryStore{db: db}
	if err := store.ensureSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *sqliteHistoryStore) ensureSchema() error {
	schema := `
CREATE TABLE IF NOT EXISTS calls (
	call_id TEXT PRIMARY KEY,
	session_id TEXT NULL,
	workspace_id TEXT NULL,
	environment_name TEXT NULL,
	method TEXT NOT NULL,
	rpc_type TEXT NOT NULL,
	endpoint_id TEXT NOT NULL,
	state TEXT NOT NULL,
	grpc_status_code TEXT NULL,
	started_at TEXT NOT NULL,
	finished_at TEXT NULL,
	duration_ms INTEGER NULL,
	request_count INTEGER NOT NULL DEFAULT 0,
	response_count INTEGER NOT NULL DEFAULT 0,
	truncated INTEGER NOT NULL DEFAULT 0,
	error_category TEXT NULL,
	error_code TEXT NULL,
	summary_path TEXT NULL,
	session_log_path TEXT NULL
);
CREATE INDEX IF NOT EXISTS idx_calls_started_at_desc ON calls(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_calls_method_started_at_desc ON calls(method, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_calls_endpoint_started_at_desc ON calls(endpoint_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_calls_grpc_status_started_at_desc ON calls(grpc_status_code, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_calls_error_category_started_at_desc ON calls(error_category, started_at DESC);
`

	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	return s.ensureColumn("calls", "environment_name", "TEXT NULL")
}

func (s *sqliteHistoryStore) ensureColumn(table, column, definition string) error {
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal any
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &primaryKey); err != nil {
			return err
		}
		if name == column {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	return err
}

func (s *sqliteHistoryStore) SaveCallSummary(ctx context.Context, summary contracts.HistoryCallSummary) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO calls (
	call_id,
	session_id,
	workspace_id,
	environment_name,
	method,
	rpc_type,
	endpoint_id,
	state,
	grpc_status_code,
	started_at,
	finished_at,
	duration_ms,
	request_count,
	response_count,
	truncated,
	error_category,
	error_code,
	summary_path,
	session_log_path
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(call_id) DO UPDATE SET
	session_id = excluded.session_id,
	workspace_id = excluded.workspace_id,
	environment_name = excluded.environment_name,
	method = excluded.method,
	rpc_type = excluded.rpc_type,
	endpoint_id = excluded.endpoint_id,
	state = excluded.state,
	grpc_status_code = excluded.grpc_status_code,
	started_at = excluded.started_at,
	finished_at = excluded.finished_at,
	duration_ms = excluded.duration_ms,
	request_count = excluded.request_count,
	response_count = excluded.response_count,
	truncated = excluded.truncated,
	error_category = excluded.error_category,
	error_code = excluded.error_code,
	summary_path = excluded.summary_path,
	session_log_path = excluded.session_log_path
`,
		summary.CallID,
		nullString(summary.SessionID),
		nullString(summary.WorkspaceID),
		nullString(summary.EnvironmentRef),
		summary.Method,
		string(summary.RPCType),
		summary.EndpointID,
		string(summary.State),
		nullString(summary.GRPCStatusCode),
		summary.StartedAt,
		nullString(summary.FinishedAt),
		summary.DurationMs,
		summary.RequestCount,
		summary.ResponseCount,
		boolToInt(summary.Truncated),
		nullString(string(summary.ErrorCategory)),
		nullString(summary.ErrorCode),
		nullString(summary.SummaryPath),
		nullString(summary.SessionLogPath),
	)
	return err
}

func (s *sqliteHistoryStore) ListCalls(ctx context.Context, input contracts.HistoryListInput) ([]contracts.HistoryCallSummary, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}

	endpointID := strings.TrimSpace(input.EndpointID)
	workspaceID := strings.TrimSpace(input.WorkspaceID)
	environmentRef := strings.TrimSpace(input.EnvironmentRef)
	query := `
SELECT
	call_id,
	session_id,
	workspace_id,
	environment_name,
	method,
	rpc_type,
	endpoint_id,
	state,
	grpc_status_code,
	started_at,
	finished_at,
	duration_ms,
	request_count,
	response_count,
	truncated,
	error_category,
	error_code,
	summary_path,
	session_log_path
FROM calls
`
	args := []any{}
	var filters []string
	if endpointID != "" {
		filters = append(filters, "endpoint_id = ?")
		args = append(args, endpointID)
	}
	if workspaceID != "" {
		filters = append(filters, "workspace_id = ?")
		args = append(args, workspaceID)
	}
	if environmentRef != "" {
		filters = append(filters, "environment_name = ?")
		args = append(args, environmentRef)
	}
	if len(filters) > 0 {
		query += "WHERE " + strings.Join(filters, " AND ") + "\n"
	}
	query += "ORDER BY started_at DESC\nLIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	calls := make([]contracts.HistoryCallSummary, 0, limit)
	for rows.Next() {
		summary, scanErr := scanHistoryCallSummary(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		calls = append(calls, summary)
	}

	return calls, rows.Err()
}

func (s *sqliteHistoryStore) GetCallSummary(ctx context.Context, callID string) (contracts.HistoryCallSummary, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT
	call_id,
	session_id,
	workspace_id,
	environment_name,
	method,
	rpc_type,
	endpoint_id,
	state,
	grpc_status_code,
	started_at,
	finished_at,
	duration_ms,
	request_count,
	response_count,
	truncated,
	error_category,
	error_code,
	summary_path,
	session_log_path
FROM calls
WHERE call_id = ?`, callID)

	summary, err := scanHistoryCallSummary(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return contracts.HistoryCallSummary{}, err
		}
		return contracts.HistoryCallSummary{}, err
	}

	return summary, nil
}

type historySummaryScanner interface {
	Scan(dest ...any) error
}

func scanHistoryCallSummary(scanner historySummaryScanner) (contracts.HistoryCallSummary, error) {
	var (
		callID         string
		sessionID      sql.NullString
		workspaceID    sql.NullString
		environmentRef sql.NullString
		method         string
		rpcType        string
		endpointID     string
		state          string
		grpcStatusCode sql.NullString
		startedAt      string
		finishedAt     sql.NullString
		durationMs     sql.NullInt64
		requestCount   int
		responseCount  int
		truncated      int
		errorCategory  sql.NullString
		errorCode      sql.NullString
		summaryPath    sql.NullString
		sessionLogPath sql.NullString
	)

	if err := scanner.Scan(
		&callID,
		&sessionID,
		&workspaceID,
		&environmentRef,
		&method,
		&rpcType,
		&endpointID,
		&state,
		&grpcStatusCode,
		&startedAt,
		&finishedAt,
		&durationMs,
		&requestCount,
		&responseCount,
		&truncated,
		&errorCategory,
		&errorCode,
		&summaryPath,
		&sessionLogPath,
	); err != nil {
		return contracts.HistoryCallSummary{}, err
	}

	return contracts.HistoryCallSummary{
		CallID:         callID,
		SessionID:      sessionID.String,
		WorkspaceID:    workspaceID.String,
		EnvironmentRef: environmentRef.String,
		Method:         method,
		RPCType:        contracts.RPCType(rpcType),
		EndpointID:     endpointID,
		State:          contracts.StreamState(state),
		GRPCStatusCode: grpcStatusCode.String,
		StartedAt:      startedAt,
		FinishedAt:     finishedAt.String,
		DurationMs:     durationMs.Int64,
		RequestCount:   requestCount,
		ResponseCount:  responseCount,
		Truncated:      truncated == 1,
		ErrorCategory:  contracts.ErrorCategory(errorCategory.String),
		ErrorCode:      errorCode.String,
		SummaryPath:    summaryPath.String,
		SessionLogPath: sessionLogPath.String,
	}, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func wrapHistoryStoreError(action string, err error) *contracts.ErrorEnvelope {
	cause := "unknown history store error"
	if err != nil {
		cause = err.Error()
	}

	return &contracts.ErrorEnvelope{
		Code:     fmt.Sprintf("application.history_%s_failed", action),
		Category: contracts.ErrorCategoryApplication,
		Message:  fmt.Sprintf("The runtime could not %s the local history store.", action),
		Details: map[string]string{
			"cause": cause,
		},
	}
}
