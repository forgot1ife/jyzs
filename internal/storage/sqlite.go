package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

type CaptureEvent struct {
	At           time.Time
	RuleName     string
	RecordType   string
	Method       string
	Host         string
	Path         string
	StatusCode   int
	RequestSize  int
	ResponseSize int
	Fields       map[string]any
}

type PriceSnapshot struct {
	At         time.Time
	ItemKey    string
	ItemName   string
	UnitPrice  float64
	Quantity   int64
	SourceRule string
	RawJSON    string
}

type Recommendation struct {
	At            time.Time `json:"at"`
	ItemKey       string    `json:"item_key"`
	ItemName      string    `json:"item_name"`
	MarketPrice   float64   `json:"market_price"`
	BaselinePrice float64   `json:"baseline_price"`
	DiscountPct   float64   `json:"discount_pct"`
	Reason        string    `json:"reason"`
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := createSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) InsertCaptureEvent(evt CaptureEvent) error {
	if evt.At.IsZero() {
		evt.At = time.Now()
	}
	fieldsJSON, _ := json.Marshal(evt.Fields)
	_, err := s.db.Exec(`
		INSERT INTO capture_events (
			at, rule_name, record_type, method, host, path, status_code, request_size, response_size, fields_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		evt.At.UTC().Format(time.RFC3339Nano),
		evt.RuleName,
		evt.RecordType,
		evt.Method,
		evt.Host,
		evt.Path,
		evt.StatusCode,
		evt.RequestSize,
		evt.ResponseSize,
		string(fieldsJSON),
	)
	return err
}

func (s *SQLiteStore) InsertPriceSnapshot(snapshot PriceSnapshot) error {
	if snapshot.At.IsZero() {
		snapshot.At = time.Now()
	}
	_, err := s.db.Exec(`
		INSERT INTO price_snapshots (
			at, item_key, item_name, unit_price, quantity, source_rule, raw_json
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		snapshot.At.UTC().Format(time.RFC3339Nano),
		snapshot.ItemKey,
		snapshot.ItemName,
		snapshot.UnitPrice,
		snapshot.Quantity,
		snapshot.SourceRule,
		snapshot.RawJSON,
	)
	return err
}

func (s *SQLiteStore) RecentPrices(itemKey string, limit int) ([]float64, error) {
	if limit <= 0 {
		limit = 30
	}
	rows, err := s.db.Query(`
		SELECT unit_price
		FROM price_snapshots
		WHERE item_key = ?
		ORDER BY at DESC
		LIMIT ?`, itemKey, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]float64, 0, limit)
	for rows.Next() {
		var price float64
		if err := rows.Scan(&price); err != nil {
			return nil, err
		}
		out = append(out, price)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) InsertRecommendation(rec Recommendation) error {
	if rec.At.IsZero() {
		rec.At = time.Now()
	}
	_, err := s.db.Exec(`
		INSERT INTO recommendations (
			at, item_key, item_name, market_price, baseline_price, discount_pct, reason
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rec.At.UTC().Format(time.RFC3339Nano),
		rec.ItemKey,
		rec.ItemName,
		rec.MarketPrice,
		rec.BaselinePrice,
		rec.DiscountPct,
		rec.Reason,
	)
	return err
}

func (s *SQLiteStore) ListRecommendations(limit int) ([]Recommendation, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT at, item_key, item_name, market_price, baseline_price, discount_pct, reason
		FROM recommendations
		ORDER BY at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Recommendation, 0, limit)
	for rows.Next() {
		var atRaw string
		var rec Recommendation
		if err := rows.Scan(
			&atRaw,
			&rec.ItemKey,
			&rec.ItemName,
			&rec.MarketPrice,
			&rec.BaselinePrice,
			&rec.DiscountPct,
			&rec.Reason,
		); err != nil {
			return nil, err
		}
		if parsed, err := time.Parse(time.RFC3339Nano, atRaw); err == nil {
			rec.At = parsed
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Counts() (map[string]int64, error) {
	tables := []string{"capture_events", "price_snapshots", "recommendations"}
	out := map[string]int64{}
	for _, table := range tables {
		var c int64
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
		if err := s.db.QueryRow(query).Scan(&c); err != nil {
			return nil, err
		}
		out[table] = c
	}
	return out, nil
}

func createSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS capture_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			at TEXT NOT NULL,
			rule_name TEXT NOT NULL,
			record_type TEXT NOT NULL,
			method TEXT NOT NULL,
			host TEXT NOT NULL,
			path TEXT NOT NULL,
			status_code INTEGER NOT NULL,
			request_size INTEGER NOT NULL,
			response_size INTEGER NOT NULL,
			fields_json TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_capture_events_at ON capture_events(at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_capture_events_rule ON capture_events(rule_name, at DESC);`,
		`CREATE TABLE IF NOT EXISTS price_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			at TEXT NOT NULL,
			item_key TEXT NOT NULL,
			item_name TEXT NOT NULL,
			unit_price REAL NOT NULL,
			quantity INTEGER NOT NULL,
			source_rule TEXT NOT NULL,
			raw_json TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_price_snapshots_key_at ON price_snapshots(item_key, at DESC);`,
		`CREATE TABLE IF NOT EXISTS recommendations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			at TEXT NOT NULL,
			item_key TEXT NOT NULL,
			item_name TEXT NOT NULL,
			market_price REAL NOT NULL,
			baseline_price REAL NOT NULL,
			discount_pct REAL NOT NULL,
			reason TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_recommendations_at ON recommendations(at DESC);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

