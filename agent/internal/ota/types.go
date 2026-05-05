package ota

import "time"

// CommandKind discriminates the message handler. v1 supports two kinds:
// the "" (empty) default, which is a forward OTA, and "rollback", which
// reverts to the previous container without pulling.
type CommandKind string

const (
	CmdForward  CommandKind = ""
	CmdRollback CommandKind = "rollback"
)

// Command is the JSON wire format published on cmd/{robot_id}/ota.
type Command struct {
	RolloutID       string `json:"rollout_id"`
	RobotID         string `json:"robot_id"`
	Action          string `json:"action,omitempty"` // "rollback" or empty (forward)
	Reason          string `json:"reason,omitempty"`
	ImageRef        string `json:"image_ref,omitempty"`
	ImageDigest     string `json:"image_digest,omitempty"`
	SmokeTimeoutSec int    `json:"smoke_timeout_sec,omitempty"`
	SmokeCommand    string `json:"smoke_command,omitempty"`
	Force           bool   `json:"force,omitempty"`
}

// Ack is the JSON wire format published on ack/{robot_id}/ota.
type Ack struct {
	RolloutID            string    `json:"rollout_id"`
	RobotID              string    `json:"robot_id"`
	Phase                string    `json:"phase"`
	Detail               string    `json:"detail,omitempty"`
	At                   time.Time `json:"at"`
	ImageDigest          string    `json:"image_digest,omitempty"`
	PreviousImageDigest  string    `json:"previous_image_digest,omitempty"`
}
