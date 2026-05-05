package collision

import "time"

// TaskQueue is shared by the CollisionResponse workflow and its
// SendTwist activity.
const TaskQueue = "collision"

// Input is the workflow input — produced by the MQTT bridge from a
// robot's collision event.
type Input struct {
	RobotID string `json:"robot_id"`
	Partner string `json:"partner,omitempty"`
	At      int64  `json:"at,omitempty"`
}

// SendTwistArgs is the activity input. The activity republishes the
// twist on MQTT cmd/{robot_id}/twist at 10 Hz for `Duration`, then
// emits one final 0,0 stop and returns.
type SendTwistArgs struct {
	RobotID  string        `json:"robot_id"`
	LinearX  float64       `json:"linear_x"`
	AngularZ float64       `json:"angular_z"`
	Duration time.Duration `json:"duration"`
}

// Activity name constants used by the workflow (avoids capturing a
// pointer-receiver method value in the workflow definition).
const (
	ActSendTwist = "SendTwist"
)
