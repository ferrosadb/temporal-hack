package ota

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type OTARollbackInput struct {
	RolloutID string
	RobotID   string
	Reason    string
}

// OTARollback instructs a robot to revert to its previous container.
// The robot performs the actual revert; this workflow is responsible
// for issuing the rollback command, waiting for the corresponding
// ack, and recording the outcome. If the robot does not ack within
// the timeout, the rollback is reported as STUCK and surfaced for
// operator attention — we do not try harder than once.
func OTARollback(ctx workflow.Context, in OTARollbackInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("rollback requested", "robot_id", in.RobotID, "reason", in.Reason)

	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 5,
		},
	})

	if err := workflow.ExecuteActivity(actx, SendRollbackCommand, in.RolloutID, in.RobotID, in.Reason).Get(actx, nil); err != nil {
		return fmt.Errorf("send rollback failed: %w", err)
	}

	ack := workflow.GetSignalChannel(ctx, SignalAck)
	timer := workflow.NewTimer(ctx, 5*time.Minute)

	for {
		sel := workflow.NewSelector(ctx)
		var sig AckSignal
		sel.AddReceive(ack, func(c workflow.ReceiveChannel, _ bool) {
			c.Receive(ctx, &sig)
		})
		var timedOut bool
		sel.AddFuture(timer, func(workflow.Future) {
			timedOut = true
		})
		sel.Select(ctx)

		if timedOut {
			_ = workflow.ExecuteActivity(actx, RecordRollbackOutcome, in.RolloutID, in.RobotID, "stuck", "no rollback ack within 5m").Get(actx, nil)
			return fmt.Errorf("rollback stuck: no ack within 5m")
		}
		if sig.Phase == "PHASE_ROLLED_BACK" {
			_ = workflow.ExecuteActivity(actx, RecordRollbackOutcome, in.RolloutID, in.RobotID, "ok", sig.Detail).Get(actx, nil)
			return nil
		}
		if sig.Phase == "PHASE_FAILED" {
			_ = workflow.ExecuteActivity(actx, RecordRollbackOutcome, in.RolloutID, in.RobotID, "failed", sig.Detail).Get(actx, nil)
			return fmt.Errorf("rollback failed: %s", sig.Detail)
		}
	}
}
