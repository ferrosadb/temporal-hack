package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Timescale is the read/write adapter for the telemetry store.
// We use Postgres + the TimescaleDB extension; the same instance can
// host the telemetry hypertable, the fleet registry, and any other
// cloud-side state. (Temporal uses its own database — keep them
// separate.)
type Timescale struct {
	pool *pgxpool.Pool
}

func OpenTimescale(ctx context.Context, dsn string) (*Timescale, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Timescale{pool: pool}, nil
}

func (t *Timescale) Close() { t.pool.Close() }

// Pool exposes the underlying pgxpool for callers that need direct
// access (e.g., the fleet registry runs its own queries).
func (t *Timescale) Pool() *pgxpool.Pool { return t.pool }

// InsertSample writes one telemetry sample. Called by the
// telemetry-ingest worker that subscribes to MQTT.
func (t *Timescale) InsertSample(ctx context.Context, robotID, stream string, capturedAt time.Time, payload []byte, schema string) error {
	_, err := t.pool.Exec(ctx,
		`INSERT INTO telemetry (robot_id, stream, captured_at, payload, schema) VALUES ($1, $2, $3, $4, $5)`,
		robotID, stream, capturedAt, payload, schema,
	)
	return err
}

type RobotRow struct {
	RobotID    string    `json:"robot_id"`
	LastSeen   time.Time `json:"last_seen"`
	BufferedN  int64     `json:"buffered_samples"`
	StatusText string    `json:"status"`
}

// ListRobots returns one row per robot we have ever heard from. The
// row aggregates the latest heartbeat. Heartbeats are inserted into
// the same telemetry table with stream="heartbeat".
func (t *Timescale) ListRobots(ctx context.Context) ([]RobotRow, error) {
	rows, err := t.pool.Query(ctx, `
		SELECT robot_id, MAX(captured_at) AS last_seen
		FROM telemetry
		WHERE stream = 'heartbeat'
		GROUP BY robot_id
		ORDER BY last_seen DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RobotRow
	for rows.Next() {
		var r RobotRow
		if err := rows.Scan(&r.RobotID, &r.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type SampleRow struct {
	RobotID    string    `json:"robot_id"`
	Stream     string    `json:"stream"`
	CapturedAt time.Time `json:"captured_at"`
	Payload    []byte    `json:"payload"`
	Schema     string    `json:"schema"`
}

func (t *Timescale) RecentTelemetry(ctx context.Context, robotID, stream string, since time.Time, limit int) ([]SampleRow, error) {
	q := `SELECT robot_id, stream, captured_at, payload, schema
	      FROM telemetry
	      WHERE robot_id = $1 AND captured_at >= $2`
	args := []any{robotID, since}
	if stream != "" {
		q += ` AND stream = $3`
		args = append(args, stream)
		q += ` ORDER BY captured_at DESC LIMIT $4`
	} else {
		q += ` ORDER BY captured_at DESC LIMIT $3`
	}
	args = append(args, limit)

	rows, err := t.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SampleRow
	for rows.Next() {
		var s SampleRow
		if err := rows.Scan(&s.RobotID, &s.Stream, &s.CapturedAt, &s.Payload, &s.Schema); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
