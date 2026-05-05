package fleet

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Registry is the read-side view of the fleet for cohort selection.
// v1 derives membership from heartbeats; an explicit registry table
// arrives in v1.5 alongside enrollment.
type Registry struct {
	pool *pgxpool.Pool
}

func NewRegistry(pool *pgxpool.Pool) *Registry { return &Registry{pool: pool} }

// SelectByLabels returns robot IDs matching all label key/value pairs.
//
// v1 implementation: heartbeats include a `labels` JSON map; we do
// the match in SQL. If labels is empty the result is every robot
// that has reported a heartbeat in the last 24h.
func (r *Registry) SelectByLabels(ctx context.Context, labels map[string]string) ([]string, error) {
	if len(labels) == 0 {
		return r.recentRobots(ctx)
	}
	// JSON containment query against the latest heartbeat per robot.
	conds := make([]string, 0, len(labels))
	args := []any{}
	idx := 1
	for k, v := range labels {
		conds = append(conds, "(labels::jsonb @> jsonb_build_object($"+itoa(idx)+", $"+itoa(idx+1)+"))")
		args = append(args, k, v)
		idx += 2
	}
	q := `
		WITH latest AS (
			SELECT DISTINCT ON (robot_id) robot_id, payload, captured_at
			FROM telemetry
			WHERE stream = 'heartbeat' AND captured_at > now() - interval '24 hours'
			ORDER BY robot_id, captured_at DESC
		), parsed AS (
			SELECT robot_id, payload::jsonb -> 'labels' AS labels
			FROM latest
		)
		SELECT robot_id FROM parsed WHERE ` + strings.Join(conds, " AND ") + ` ORDER BY robot_id`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *Registry) recentRobots(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT robot_id FROM telemetry
		WHERE stream = 'heartbeat' AND captured_at > now() - interval '24 hours'
		ORDER BY robot_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// itoa avoids fmt.Sprintf in a tight loop and is internal to this file.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	const digits = "0123456789"
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = digits[n%10]
		n /= 10
	}
	return string(b[i:])
}
