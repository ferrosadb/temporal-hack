package ota

import "time"

// RolloutSpec is the operator-supplied input for a rollout.
type RolloutSpec struct {
	ImageRef       string        `json:"image_ref"`
	ImageDigest    string        `json:"image_digest"`
	CohortSelector CohortFilter  `json:"cohort_selector"`
	CanarySize     int           `json:"canary_size"`     // default 1
	BatchPercent   int           `json:"batch_percent"`   // default 25
	FailureBudget  int           `json:"failure_budget"`  // per-batch tolerated failures (count)
	SmokeTimeout   time.Duration `json:"smoke_timeout"`   // default 5m
	SmokeCommand   string        `json:"smoke_command"`   // executed in new container
	PullTimeout    time.Duration `json:"pull_timeout"`    // default 15m
	SwapTimeout    time.Duration `json:"swap_timeout"`    // default 2m
	Force          bool          `json:"force"`
}

// CohortFilter selects which robots a rollout targets. v1 supports a
// pinned list and a label selector. Other selector kinds land in v1.5.
type CohortFilter struct {
	RobotIDs []string          `json:"robot_ids,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

// SinglePhase is the phase result returned by an OTASingleRobot child.
type SinglePhase string

const (
	SingleOK         SinglePhase = "ok"
	SingleFailed     SinglePhase = "failed"
	SingleRolledBack SinglePhase = "rolled_back"
	SingleTimedOut   SinglePhase = "timed_out"
)

// SingleResult is the structured outcome of a per-robot child workflow.
type SingleResult struct {
	RobotID   string      `json:"robot_id"`
	Phase     SinglePhase `json:"phase"`
	Detail    string      `json:"detail,omitempty"`
	StartedAt time.Time   `json:"started_at"`
	EndedAt   time.Time   `json:"ended_at"`
}

// RolloutStatus is the operator-visible state of a rollout.
type RolloutStatus string

const (
	StatusPending      RolloutStatus = "pending"
	StatusCanary       RolloutStatus = "canary"
	StatusCanaryFailed RolloutStatus = "canary_failed"
	StatusBatching     RolloutStatus = "batching"
	StatusAborted      RolloutStatus = "aborted"
	StatusCompleted    RolloutStatus = "completed"
	StatusEmptyCohort  RolloutStatus = "empty_cohort"
)

func defaultsApplied(s RolloutSpec) RolloutSpec {
	if s.CanarySize == 0 {
		s.CanarySize = 1
	}
	if s.BatchPercent == 0 {
		s.BatchPercent = 25
	}
	if s.SmokeTimeout == 0 {
		s.SmokeTimeout = 5 * time.Minute
	}
	if s.PullTimeout == 0 {
		s.PullTimeout = 15 * time.Minute
	}
	if s.SwapTimeout == 0 {
		s.SwapTimeout = 2 * time.Minute
	}
	return s
}
