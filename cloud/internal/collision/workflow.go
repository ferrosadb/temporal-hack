package collision

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// CollisionResponse is the demo workflow: when the robot reports a
// collision via MQTT, this runs in the cloud and drives the robot
// out of the obstacle:
//
//  1. back up at 0.3 m/s for 3 s
//  2. stop briefly (settle)
//  3. turn right (clockwise) at 0.5 rad/s for ~3.14 s   (=> 90°)
//  4. stop briefly
//  5. drive forward at 0.4 m/s for 5 s
//  6. stop
//
// Each step is one SendTwist activity. The activity republishes the
// twist at 10 Hz for the duration so the gz diff-drive command
// timeout (~0.5 s) doesn't stall the rover, then emits a stop frame.
//
// Workflow ID convention: `collision-{robotID}-{ts}` so multiple
// collision events from the same robot don't collide.
func CollisionResponse(ctx workflow.Context, in Input) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("collision response starting", "robot_id", in.RobotID, "partner", in.Partner)

	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    500 * time.Millisecond,
			BackoffCoefficient: 2,
			MaximumAttempts:    3,
		},
	})

	steps := []SendTwistArgs{
		{RobotID: in.RobotID, LinearX: -0.30, AngularZ: 0.0, Duration: 3 * time.Second},         // back up
		{RobotID: in.RobotID, LinearX: 0.0, AngularZ: 0.0, Duration: 500 * time.Millisecond},    // settle
		{RobotID: in.RobotID, LinearX: 0.0, AngularZ: -0.50, Duration: 3140 * time.Millisecond}, // turn right ~90°
		{RobotID: in.RobotID, LinearX: 0.0, AngularZ: 0.0, Duration: 500 * time.Millisecond},    // settle
		{RobotID: in.RobotID, LinearX: 0.40, AngularZ: 0.0, Duration: 5 * time.Second},          // forward
		{RobotID: in.RobotID, LinearX: 0.0, AngularZ: 0.0, Duration: 200 * time.Millisecond},    // final stop
	}

	for i, s := range steps {
		logger.Info("collision phase", "i", i, "linear_x", s.LinearX, "angular_z", s.AngularZ, "duration", s.Duration)
		if err := workflow.ExecuteActivity(actx, ActSendTwist, s).Get(actx, nil); err != nil {
			return err
		}
	}

	logger.Info("collision response complete", "robot_id", in.RobotID)
	return nil
}
