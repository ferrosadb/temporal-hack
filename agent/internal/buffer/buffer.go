package buffer

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// Buffer is a bounded, durable, FIFO local store for telemetry samples.
// Backed by SQLite for zero ops. Bounded by sample count with a
// drop-oldest eviction policy when capacity is exceeded.
//
// Acceptance criterion (S2): survive 30-min disconnect with 100% delivery
// of buffered samples on reconnect, up to MaxSamples capacity.
type Buffer struct {
	db         *sql.DB
	maxSamples int
}

// Sample is the persisted form of a telemetry sample awaiting publish.
// Wire fields stay opaque; the buffer does not interpret payload.
type Sample struct {
	ID         int64
	RobotID    string
	Stream     string
	CapturedAt int64 // unix nanos
	Payload    []byte
	Schema     string
}

// Open creates the buffer file (and parent dir) if needed and returns
// a ready-to-use Buffer. Default capacity is 100k samples.
func Open(path string) (*Buffer, error) {
	if err := ensureDir(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return &Buffer{db: db, maxSamples: 100_000}, nil
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS samples (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    robot_id    TEXT    NOT NULL,
    stream      TEXT    NOT NULL,
    captured_at INTEGER NOT NULL,
    payload     BLOB    NOT NULL,
    schema      TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_samples_id ON samples(id);
`

func (b *Buffer) Close() error { return b.db.Close() }

// Append enqueues a sample. If the buffer is over capacity after
// insert, the oldest samples are dropped until back at capacity.
func (b *Buffer) Append(s Sample) error {
	tx, err := b.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO samples (robot_id, stream, captured_at, payload, schema) VALUES (?, ?, ?, ?, ?)`,
		s.RobotID, s.Stream, s.CapturedAt, s.Payload, s.Schema,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`DELETE FROM samples WHERE id IN (
			SELECT id FROM samples ORDER BY id ASC
			LIMIT MAX(0, (SELECT COUNT(*) FROM samples) - ?)
		)`,
		b.maxSamples,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// Peek returns up to n oldest samples without removing them.
func (b *Buffer) Peek(n int) ([]Sample, error) {
	rows, err := b.db.Query(
		`SELECT id, robot_id, stream, captured_at, payload, schema FROM samples ORDER BY id ASC LIMIT ?`,
		n,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Sample, 0, n)
	for rows.Next() {
		var s Sample
		if err := rows.Scan(&s.ID, &s.RobotID, &s.Stream, &s.CapturedAt, &s.Payload, &s.Schema); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Ack removes samples whose IDs are in the given slice. Used after
// successful broker publish.
func (b *Buffer) Ack(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := b.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`DELETE FROM samples WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.Exec(id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Count returns the number of buffered samples.
func (b *Buffer) Count() (int, error) {
	var n int
	err := b.db.QueryRow(`SELECT COUNT(*) FROM samples`).Scan(&n)
	return n, err
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	if err := mkdirAll(dir); err != nil && !errors.Is(err, errExists) {
		return err
	}
	return nil
}
