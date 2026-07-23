// Package storage provides a SQLite-backed key-value store used by singa
// to persist all runtime configuration (except .srs files and uploaded
// sing-box configs which remain as plain files).
//
// Schema (two tables):
//
//	settings  (key TEXT PRIMARY KEY, value TEXT NOT NULL)
//	  – stores singa_settings, proxy_settings, state, ipfilter
//	  – each value is a JSON blob matching the corresponding Go struct
//
//	entities  (kind TEXT NOT NULL, id TEXT NOT NULL, data TEXT NOT NULL,
//	           updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
//	           PRIMARY KEY (kind, id))
//	  – stores nodes, subscriptions (metadata + wizardConfig),
//	    subscription proxy caches, and profiles
//	  – `kind` is a namespace string, `id` is the record key,
//	    `data` is a JSON blob
package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// DB is the application-wide SQLite database handle.
// It is safe for concurrent use; the underlying driver serialises writes.
type DB struct {
	mu  sync.RWMutex
	sql *sql.DB
}

// Open opens (or creates) the SQLite database at path and initialises the
// schema.  Call Close() when done.
func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	// Single writer at a time is fine; WAL mode allows concurrent readers.
	sqlDB.SetMaxOpenConns(1)
	db := &DB{sql: sqlDB}
	if err := db.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

// Close closes the underlying database connection.
func (db *DB) Close() error {
	return db.sql.Close()
}

const schema = `
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS entities (
    kind       TEXT     NOT NULL,
    id         TEXT     NOT NULL,
    data       TEXT     NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (kind, id)
);
`

func (db *DB) migrate() error {
	_, err := db.sql.Exec(schema)
	return err
}

// ── Settings (key-value for singleton structs) ──────────────────────────────

// LoadSetting reads a JSON-encoded value from the settings table.
// If the key does not exist v is left unchanged and nil is returned.
func (db *DB) LoadSetting(key string, v interface{}) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var raw string
	err := db.sql.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil // not set yet — caller keeps default
	}
	if err != nil {
		return fmt.Errorf("load setting %q: %w", key, err)
	}
	if err := json.Unmarshal([]byte(raw), v); err != nil {
		return fmt.Errorf("unmarshal setting %q: %w", key, err)
	}
	return nil
}

// SaveSetting JSON-encodes v and upserts it into the settings table.
func (db *DB) SaveSetting(key string, v interface{}) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal setting %q: %w", key, err)
	}
	_, err = db.sql.Exec(
		`INSERT INTO settings(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, string(data),
	)
	if err != nil {
		return fmt.Errorf("save setting %q: %w", key, err)
	}
	return nil
}

// ── Entities (typed records with kind + id) ─────────────────────────────────

// Entity is a raw database row from the entities table.
type Entity struct {
	Kind      string
	ID        string
	Data      string    // raw JSON
	UpdatedAt time.Time
}

// GetEntity fetches a single entity by (kind, id).
// Returns (nil, nil) if not found.
func (db *DB) GetEntity(kind, id string) (*Entity, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	e := &Entity{Kind: kind, ID: id}
	err := db.sql.QueryRow(
		`SELECT data, updated_at FROM entities WHERE kind = ? AND id = ?`,
		kind, id,
	).Scan(&e.Data, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get entity %s/%s: %w", kind, id, err)
	}
	return e, nil
}

// ListEntities returns all entities of a given kind, ordered by updated_at ASC.
func (db *DB) ListEntities(kind string) ([]Entity, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	rows, err := db.sql.Query(
		`SELECT id, data, updated_at FROM entities WHERE kind = ? ORDER BY updated_at ASC`,
		kind,
	)
	if err != nil {
		return nil, fmt.Errorf("list entities %s: %w", kind, err)
	}
	defer rows.Close()

	var out []Entity
	for rows.Next() {
		var e Entity
		e.Kind = kind
		if err := rows.Scan(&e.ID, &e.Data, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan entity %s: %w", kind, err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UpsertEntity inserts or replaces an entity row.
// v is JSON-marshalled into the data column.
func (db *DB) UpsertEntity(kind, id string, v interface{}) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal entity %s/%s: %w", kind, id, err)
	}
	_, err = db.sql.Exec(
		`INSERT INTO entities(kind, id, data, updated_at) VALUES(?, ?, ?, ?)
		 ON CONFLICT(kind, id) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`,
		kind, id, string(data), time.Now(),
	)
	if err != nil {
		return fmt.Errorf("upsert entity %s/%s: %w", kind, id, err)
	}
	return nil
}

// DeleteEntity removes a single entity.  Returns nil if not found.
func (db *DB) DeleteEntity(kind, id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.sql.Exec(`DELETE FROM entities WHERE kind = ? AND id = ?`, kind, id)
	if err != nil {
		return fmt.Errorf("delete entity %s/%s: %w", kind, id, err)
	}
	return nil
}

// DeleteEntitiesByKind removes all entities of a given kind.
func (db *DB) DeleteEntitiesByKind(kind string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.sql.Exec(`DELETE FROM entities WHERE kind = ?`, kind)
	if err != nil {
		return fmt.Errorf("delete entities %s: %w", kind, err)
	}
	return nil
}

// ── Store (backward-compat shim for settings) ───────────────────────────────

// Store wraps DB for a single named settings key, providing the original
// Load / Save API so callers need minimal changes.
type Store struct {
	db  *DB
	key string
}

// NewStore returns a Store that persists to the given settings key.
func NewStore(db *DB, key string) *Store {
	return &Store{db: db, key: key}
}

// Load reads the JSON value for this key into v.
func (s *Store) Load(v interface{}) error {
	return s.db.LoadSetting(s.key, v)
}

// Save writes v as the JSON value for this key.
func (s *Store) Save(v interface{}) error {
	return s.db.SaveSetting(s.key, v)
}
