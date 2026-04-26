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
	At           time.Time      `json:"at"`
	RuleName     string         `json:"rule_name"`
	RecordType   string         `json:"record_type"`
	Method       string         `json:"method"`
	Host         string         `json:"host"`
	Path         string         `json:"path"`
	StatusCode   int            `json:"status_code"`
	RequestSize  int            `json:"request_size"`
	ResponseSize int            `json:"response_size"`
	Fields       map[string]any `json:"fields"`
}

type PriceSnapshot struct {
	At         time.Time `json:"at"`
	ItemKey    string    `json:"item_key"`
	ItemName   string    `json:"item_name"`
	UnitPrice  float64   `json:"unit_price"`
	Quantity   int64     `json:"quantity"`
	SourceRule string    `json:"source_rule"`
	RawJSON    string    `json:"raw_json"`
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

type CharacterStatus struct {
	At             time.Time `json:"at"`
	PlayerName     string    `json:"player_name"`
	LoginSucceeded bool      `json:"login_succeeded"`
	LoginAccount   string    `json:"login_account"`
	AreaName       string    `json:"area_name"`
	ServerName     string    `json:"server_name"`
	Source         string    `json:"source"`
	RawLine        string    `json:"raw_line"`
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

func (s *SQLiteStore) InsertCharacterStatus(status CharacterStatus) error {
	if status.At.IsZero() {
		status.At = time.Now()
	}

	loginSucceeded := 0
	if status.LoginSucceeded {
		loginSucceeded = 1
	}

	_, err := s.db.Exec(`
		INSERT INTO character_status (
			at, player_name, login_succeeded, login_account, area_name, server_name, source, raw_line
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		status.At.UTC().Format(time.RFC3339Nano),
		status.PlayerName,
		loginSucceeded,
		status.LoginAccount,
		status.AreaName,
		status.ServerName,
		status.Source,
		status.RawLine,
	)
	return err
}

func (s *SQLiteStore) LatestCharacterStatus() (*CharacterStatus, error) {
	var (
		atRaw      string
		status     CharacterStatus
		loginAsInt int
	)
	err := s.db.QueryRow(`
		SELECT at, player_name, login_succeeded, login_account, area_name, server_name, source, raw_line
		FROM character_status
		ORDER BY at DESC
		LIMIT 1`,
	).Scan(
		&atRaw,
		&status.PlayerName,
		&loginAsInt,
		&status.LoginAccount,
		&status.AreaName,
		&status.ServerName,
		&status.Source,
		&status.RawLine,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if parsed, err := time.Parse(time.RFC3339Nano, atRaw); err == nil {
		status.At = parsed
	}
	status.LoginSucceeded = loginAsInt == 1
	return &status, nil
}

func (s *SQLiteStore) ListCaptureEvents(limit int, ruleName string) ([]CaptureEvent, error) {
	if limit <= 0 {
		limit = 20
	}

	var (
		rows *sql.Rows
		err  error
	)
	if ruleName == "" {
		rows, err = s.db.Query(`
			SELECT at, rule_name, record_type, method, host, path, status_code, request_size, response_size, fields_json
			FROM capture_events
			ORDER BY at DESC
			LIMIT ?`, limit)
	} else {
		rows, err = s.db.Query(`
			SELECT at, rule_name, record_type, method, host, path, status_code, request_size, response_size, fields_json
			FROM capture_events
			WHERE rule_name = ?
			ORDER BY at DESC
			LIMIT ?`, ruleName, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]CaptureEvent, 0, limit)
	for rows.Next() {
		var atRaw string
		var fieldsRaw string
		var evt CaptureEvent
		if err := rows.Scan(
			&atRaw,
			&evt.RuleName,
			&evt.RecordType,
			&evt.Method,
			&evt.Host,
			&evt.Path,
			&evt.StatusCode,
			&evt.RequestSize,
			&evt.ResponseSize,
			&fieldsRaw,
		); err != nil {
			return nil, err
		}

		if parsed, err := time.Parse(time.RFC3339Nano, atRaw); err == nil {
			evt.At = parsed
		}
		if fieldsRaw != "" {
			_ = json.Unmarshal([]byte(fieldsRaw), &evt.Fields)
		}
		if evt.Fields == nil {
			evt.Fields = map[string]any{}
		}
		out = append(out, evt)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) ListPriceSnapshots(limit int, itemKey string) ([]PriceSnapshot, error) {
	if limit <= 0 {
		limit = 20
	}

	var (
		rows *sql.Rows
		err  error
	)
	if itemKey == "" {
		rows, err = s.db.Query(`
			SELECT at, item_key, item_name, unit_price, quantity, source_rule, raw_json
			FROM price_snapshots
			ORDER BY at DESC
			LIMIT ?`, limit)
	} else {
		rows, err = s.db.Query(`
			SELECT at, item_key, item_name, unit_price, quantity, source_rule, raw_json
			FROM price_snapshots
			WHERE item_key = ?
			ORDER BY at DESC
			LIMIT ?`, itemKey, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]PriceSnapshot, 0, limit)
	for rows.Next() {
		var atRaw string
		var row PriceSnapshot
		if err := rows.Scan(
			&atRaw,
			&row.ItemKey,
			&row.ItemName,
			&row.UnitPrice,
			&row.Quantity,
			&row.SourceRule,
			&row.RawJSON,
		); err != nil {
			return nil, err
		}
		if parsed, err := time.Parse(time.RFC3339Nano, atRaw); err == nil {
			row.At = parsed
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Counts() (map[string]int64, error) {
	tables := []string{"capture_events", "price_snapshots", "recommendations", "character_status"}
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
		`CREATE TABLE IF NOT EXISTS character_status (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			at TEXT NOT NULL,
			player_name TEXT NOT NULL,
			login_succeeded INTEGER NOT NULL,
			login_account TEXT NOT NULL,
			area_name TEXT NOT NULL,
			server_name TEXT NOT NULL,
			source TEXT NOT NULL,
			raw_line TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_character_status_at ON character_status(at DESC);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
