package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/pronzzz/zenmonitor/internal/monitor"
	_ "modernc.org/sqlite" // Import generic driver
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	// Open database (creates file if not exists)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Enable WAL mode for concurrency
	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.initSchema(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *SQLiteStore) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS checks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		monitor_name TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		status INTEGER NOT NULL, -- 1=UP, 0=DOWN
		latency_ms INTEGER NOT NULL,
		error_msg TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_monitor_time ON checks(monitor_name, timestamp);
	`
	_, err := s.db.Exec(query)
	return err
}

func (s *SQLiteStore) LogCheck(result monitor.CheckResult) error {
	query := `
	INSERT INTO checks (monitor_name, timestamp, status, latency_ms, error_msg)
	VALUES (?, ?, ?, ?, ?)
	`
	statusInt := 0
	if result.Status {
		statusInt = 1
	}
	
	_, err := s.db.Exec(query, 
		result.MonitorName, 
		result.Timestamp, 
		statusInt, 
		result.Latency.Milliseconds(), 
		result.Error,
	)
	return err
}

func (s *SQLiteStore) GetHistory(monitorName string, limit int) ([]monitor.CheckResult, error) {
	query := `
	SELECT timestamp, status, latency_ms, error_msg 
	FROM checks 
	WHERE monitor_name = ? 
	ORDER BY timestamp DESC 
	LIMIT ?
	`
	
	rows, err := s.db.Query(query, monitorName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []monitor.CheckResult
	for rows.Next() {
		var r monitor.CheckResult
		var statusInt int
		var latMs int64
		var ts time.Time
		r.MonitorName = monitorName

		if err := rows.Scan(&ts, &statusInt, &latMs, &r.Error); err != nil {
			return nil, err
		}
		r.Status = (statusInt == 1)
		r.Latency = time.Duration(latMs) * time.Millisecond
		r.Timestamp = ts
		results = append(results, r)
	}
	
	// Since we order by DESC (newest first), we might want to reverse if the UI expects time order, 
	// but UI usually handles that or we can order ASC in a subquery. 
	// The PRD says "grid of green/red dots representing the last 90 days". 
	// Typical dot matrix is left-to-right (oldest to newest).
	// So we should reverse this list or query ASC with offset.
	// But getting last N usually implies DESC limit.
	// Let's reverse them in code for convenience.
	// Or just ORDER BY timestamp DESC LIMIT ? -> then reverse.
	
	// Reversing in place
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}

	return results, nil
}

func (s *SQLiteStore) PruneOldData(days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	query := `DELETE FROM checks WHERE timestamp < ?`
	_, err := s.db.Exec(query, cutoff)
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
