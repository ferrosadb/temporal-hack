package store

import (
	"context"
	"time"
)

// Schema lives in deploy/postgres/init-ota.sql.

func (t *Timescale) RecordRolloutStarted(ctx context.Context, rolloutID, specJSON string) error {
	_, err := t.pool.Exec(ctx,
		`INSERT INTO ota_rollouts (rollout_id, spec, status, started_at)
		 VALUES ($1, $2::jsonb, 'pending', now())
		 ON CONFLICT (rollout_id) DO NOTHING`,
		rolloutID, specJSON,
	)
	return err
}

func (t *Timescale) RecordRolloutEnded(ctx context.Context, rolloutID, status, detail string) error {
	_, err := t.pool.Exec(ctx,
		`UPDATE ota_rollouts SET status = $2, detail = $3, ended_at = now()
		 WHERE rollout_id = $1`,
		rolloutID, status, detail,
	)
	return err
}

func (t *Timescale) RecordRobotResult(ctx context.Context, robotID, phase, detail string, startedAt, endedAt time.Time) error {
	_, err := t.pool.Exec(ctx,
		`INSERT INTO ota_robot_results (robot_id, phase, detail, started_at, ended_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		robotID, phase, detail, startedAt, endedAt,
	)
	return err
}

func (t *Timescale) RecordRollbackOutcome(ctx context.Context, rolloutID, robotID, status, detail string) error {
	_, err := t.pool.Exec(ctx,
		`INSERT INTO ota_rollbacks (rollout_id, robot_id, status, detail, at)
		 VALUES ($1, $2, $3, $4, now())`,
		rolloutID, robotID, status, detail,
	)
	return err
}

type RolloutSummary struct {
	RolloutID string     `json:"rollout_id"`
	Status    string     `json:"status"`
	Detail    string     `json:"detail,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

func (t *Timescale) GetRollout(ctx context.Context, rolloutID string) (*RolloutSummary, error) {
	var s RolloutSummary
	var endedAt *time.Time
	err := t.pool.QueryRow(ctx,
		`SELECT rollout_id, status, COALESCE(detail,''), started_at, ended_at
		 FROM ota_rollouts WHERE rollout_id = $1`, rolloutID,
	).Scan(&s.RolloutID, &s.Status, &s.Detail, &s.StartedAt, &endedAt)
	if err != nil {
		return nil, err
	}
	s.EndedAt = endedAt
	return &s, nil
}

func (t *Timescale) ListRollouts(ctx context.Context, limit int) ([]RolloutSummary, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := t.pool.Query(ctx,
		`SELECT rollout_id, status, COALESCE(detail,''), started_at, ended_at
		 FROM ota_rollouts ORDER BY started_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RolloutSummary
	for rows.Next() {
		var s RolloutSummary
		var endedAt *time.Time
		if err := rows.Scan(&s.RolloutID, &s.Status, &s.Detail, &s.StartedAt, &endedAt); err != nil {
			return nil, err
		}
		s.EndedAt = endedAt
		out = append(out, s)
	}
	return out, rows.Err()
}
