package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Open 打开（必要时创建）SQLite 数据库并完成建表迁移。
// 使用现代纯 Go 驱动，CGO 无关，离线可构建。
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)",
		path,
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite 单写连接最稳，避免并发写锁。
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := Migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

// Migrate 执行建表（幂等）。
func Migrate(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS entropy_sources (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	device TEXT NOT NULL,
	state TEXT NOT NULL,
	created_at TEXT NOT NULL,
	sealed_at TEXT,
	last_seq INTEGER NOT NULL DEFAULT 0,
	recovery_baseline_seq INTEGER
);
CREATE INDEX IF NOT EXISTS idx_sources_device ON entropy_sources(device);
CREATE UNIQUE INDEX IF NOT EXISTS uq_sources_name_device ON entropy_sources(name, device);

CREATE TABLE IF NOT EXISTS sample_windows (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	source_id INTEGER NOT NULL,
	seq INTEGER NOT NULL,
	received_at TEXT NOT NULL,
	byte_len INTEGER NOT NULL,
	sample_hash TEXT NOT NULL,
	est_entropy REAL NOT NULL,
	repeat_score REAL NOT NULL,
	status TEXT NOT NULL,
	anomaly_type TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_windows_source_seq ON sample_windows(source_id, seq);
CREATE INDEX IF NOT EXISTS idx_windows_source_status ON sample_windows(source_id, status);

CREATE TABLE IF NOT EXISTS health_tests (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	window_id INTEGER NOT NULL,
	source_id INTEGER NOT NULL,
	category TEXT NOT NULL,
	passed INTEGER NOT NULL,
	statistic REAL NOT NULL,
	threshold REAL NOT NULL,
	detail TEXT,
	evaluated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_tests_window ON health_tests(window_id);
CREATE INDEX IF NOT EXISTS idx_tests_source ON health_tests(source_id);

CREATE TABLE IF NOT EXISTS health_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	source_id INTEGER NOT NULL,
	state TEXT NOT NULL,
	anomaly_type TEXT NOT NULL,
	first_seq INTEGER NOT NULL,
	last_seq INTEGER NOT NULL,
	window_count INTEGER NOT NULL,
	after_restart INTEGER NOT NULL,
	created_at TEXT NOT NULL,
	confirmed_at TEXT,
	closed_at TEXT,
	note TEXT
);
CREATE INDEX IF NOT EXISTS idx_events_source ON health_events(source_id);
CREATE INDEX IF NOT EXISTS idx_events_state ON health_events(state);

CREATE TABLE IF NOT EXISTS restart_boundaries (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	source_id INTEGER NOT NULL,
	at_seq INTEGER NOT NULL,
	occurred_at TEXT NOT NULL,
	baseline_seq INTEGER NOT NULL,
	baseline_missing INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_restarts_source ON restart_boundaries(source_id);

CREATE TABLE IF NOT EXISTS diagnostic_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	source_id INTEGER NOT NULL,
	version INTEGER NOT NULL,
	state TEXT NOT NULL,
	payload TEXT NOT NULL,
	created_at TEXT NOT NULL,
	published_at TEXT,
	superseded_by INTEGER
);
CREATE INDEX IF NOT EXISTS idx_snapshots_source ON diagnostic_snapshots(source_id);
`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}
	return nil
}

// nullTimeVal 将可空时间转为可写入参数。
func nullTimeVal(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}

// scanNullTime 从 sql.NullTime 安全读取可空时间。
func scanNullTime(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	t := nt.Time
	return &t
}
