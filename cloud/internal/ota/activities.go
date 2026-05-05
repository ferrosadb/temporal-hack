package ota

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/example/temporal-hack/cloud/internal/fleet"
	"github.com/example/temporal-hack/cloud/internal/store"
)

// Activities is the registered Temporal activity surface. The struct
// owns dependencies (mqtt client, db) so workflows can be deterministic
// and tests can substitute fakes.
type Activities struct {
	MQTT     mqtt.Client
	Store    *store.Timescale
	Registry *fleet.Registry
}

// SendOTACommand publishes an OTACommand to cmd/{robot_id}/ota.
// The wire format is a JSON serialization of the OTACommand fields
// (we'll switch to protobuf wire when bridge gen is wired up; using
// JSON here keeps the activity readable and tractable).
func (a *Activities) SendOTACommand(ctx context.Context, rolloutID, robotID string, spec RolloutSpec) error {
	payload := map[string]any{
		"rollout_id":        rolloutID,
		"robot_id":          robotID,
		"image_ref":         spec.ImageRef,
		"image_digest":      spec.ImageDigest,
		"smoke_timeout_sec": int(spec.SmokeTimeout / time.Second),
		"smoke_command":     spec.SmokeCommand,
		"force":             spec.Force,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	topic := fmt.Sprintf("cmd/%s/ota", robotID)
	tok := a.MQTT.Publish(topic, 1, false, body)
	if !tok.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("mqtt publish timeout to %s", topic)
	}
	return tok.Error()
}

// SendRollbackCommand publishes a rollback command to cmd/{robot_id}/ota.
// The robot agent treats action="rollback" as a separate flow: revert
// to the previous container without pulling a new image.
func (a *Activities) SendRollbackCommand(ctx context.Context, rolloutID, robotID, reason string) error {
	payload := map[string]any{
		"rollout_id": rolloutID,
		"robot_id":   robotID,
		"action":     "rollback",
		"reason":     reason,
	}
	body, _ := json.Marshal(payload)
	topic := fmt.Sprintf("cmd/%s/ota", robotID)
	tok := a.MQTT.Publish(topic, 1, false, body)
	if !tok.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("mqtt publish timeout to %s", topic)
	}
	return tok.Error()
}

// ResolveCohort returns the list of robot IDs matching the selector.
// v1 supports an explicit ID list and a label selector.
func (a *Activities) ResolveCohort(ctx context.Context, sel CohortFilter) ([]string, error) {
	if len(sel.RobotIDs) > 0 {
		return sel.RobotIDs, nil
	}
	return a.Registry.SelectByLabels(ctx, sel.Labels)
}

// RecordRolloutStarted persists the initial rollout row.
func (a *Activities) RecordRolloutStarted(ctx context.Context, rolloutID string, spec RolloutSpec) error {
	specJSON, _ := json.Marshal(spec)
	return a.Store.RecordRolloutStarted(ctx, rolloutID, string(specJSON))
}

// RecordRolloutEnded marks the rollout terminal.
func (a *Activities) RecordRolloutEnded(ctx context.Context, rolloutID string, status RolloutStatus, detail string) error {
	return a.Store.RecordRolloutEnded(ctx, rolloutID, string(status), detail)
}

// RecordRobotResult persists per-robot OTA outcome.
func (a *Activities) RecordRobotResult(ctx context.Context, _ string, res SingleResult) error {
	return a.Store.RecordRobotResult(ctx, res.RobotID, string(res.Phase), res.Detail, res.StartedAt, res.EndedAt)
}

// RecordRollbackOutcome persists rollback outcome.
func (a *Activities) RecordRollbackOutcome(ctx context.Context, rolloutID, robotID, status, detail string) error {
	return a.Store.RecordRollbackOutcome(ctx, rolloutID, robotID, status, detail)
}

// Activity-name aliases, for registration. Workflows reference these
// constants, not method values, to avoid coupling the workflow code
// to the receiver pointer.
var (
	SendOTACommand        = "SendOTACommand"
	SendRollbackCommand   = "SendRollbackCommand"
	ResolveCohort         = "ResolveCohort"
	RecordRolloutStarted  = "RecordRolloutStarted"
	RecordRolloutEnded    = "RecordRolloutEnded"
	RecordRobotResult     = "RecordRobotResult"
	RecordRollbackOutcome = "RecordRollbackOutcome"
)
