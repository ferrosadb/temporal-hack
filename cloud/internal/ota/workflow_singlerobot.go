package ota

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// OTASingleRobotInput is the per-robot child workflow input.
type OTASingleRobotInput struct {
	RolloutID string
	RobotID   string
	Spec      RolloutSpec
}

// AckSignal is the structured signal payload posted by mqttbridge
// when an OTAAck arrives for a workflow's robot.
type AckSignal struct {
	Phase                string `json:"phase"` // values map to ota.proto Phase enum names
	Detail               string `json:"detail"`
	ImageDigest          string `json:"image_digest"`
	PreviousImageDigest  string `json:"previous_image_digest"`
}

// SignalAck is the signal channel name used for ack delivery.
const SignalAck = "ota-ack"

// OTASingleRobot orchestrates a single robot's OTA: send command,
// wait for pull → swap → healthy acks (each with its own timeout),
// or roll back via OTARollback child on any failure.
func OTASingleRobot(ctx workflow.Context, in OTASingleRobotInput) (SingleResult, error) {
	res := SingleResult{
		RobotID:   in.RobotID,
		StartedAt: workflow.Now(ctx),
	}
	logger := workflow.GetLogger(ctx)

	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 5,
		},
	})

	if err := workflow.ExecuteActivity(actx, SendOTACommand, in.RolloutID, in.RobotID, in.Spec).Get(actx, nil); err != nil {
		return finishFailed(ctx, res, "send command failed: "+err.Error())
	}

	ackChan := workflow.GetSignalChannel(ctx, SignalAck)

	// Phase: PULLED
	if err := waitForPhase(ctx, ackChan, "PHASE_PULLED", in.Spec.PullTimeout); err != nil {
		_ = executeRollback(ctx, in, "pull timeout/error: "+err.Error())
		return finishRolledBack(ctx, res, err.Error())
	}

	// Phase: SWAPPED
	if err := waitForPhase(ctx, ackChan, "PHASE_SWAPPED", in.Spec.SwapTimeout); err != nil {
		_ = executeRollback(ctx, in, "swap timeout/error: "+err.Error())
		return finishRolledBack(ctx, res, err.Error())
	}

	// Phase: HEALTHY
	if err := waitForPhase(ctx, ackChan, "PHASE_HEALTHY", in.Spec.SmokeTimeout); err != nil {
		_ = executeRollback(ctx, in, "smoke check failed: "+err.Error())
		return finishRolledBack(ctx, res, err.Error())
	}

	logger.Info("single-robot OTA complete", "robot_id", in.RobotID)
	res.Phase = SingleOK
	res.EndedAt = workflow.Now(ctx)
	_ = workflow.ExecuteActivity(actx, RecordRobotResult, in.RolloutID, res).Get(actx, nil)
	return res, nil
}

// waitForPhase blocks the workflow until either the desired phase
// signal arrives or timeout elapses. Any PHASE_FAILED signal short-
// circuits with an error.
func waitForPhase(ctx workflow.Context, ackChan workflow.ReceiveChannel, want string, timeout time.Duration) error {
	timer := workflow.NewTimer(ctx, timeout)
	for {
		sel := workflow.NewSelector(ctx)
		var sig AckSignal
		sel.AddReceive(ackChan, func(c workflow.ReceiveChannel, _ bool) {
			c.Receive(ctx, &sig)
		})
		var timedOut bool
		sel.AddFuture(timer, func(workflow.Future) {
			timedOut = true
		})
		sel.Select(ctx)

		if timedOut {
			return fmt.Errorf("timeout waiting for %s after %s", want, timeout)
		}
		switch sig.Phase {
		case want:
			return nil
		case "PHASE_FAILED":
			return fmt.Errorf("robot reported failed: %s", sig.Detail)
		case "PHASE_ROLLED_BACK":
			return fmt.Errorf("robot rolled back independently: %s", sig.Detail)
		default:
			// Earlier-phase signal or duplicate; ignore and keep waiting.
		}
	}
}

func executeRollback(ctx workflow.Context, in OTASingleRobotInput, reason string) error {
	logger := workflow.GetLogger(ctx)
	logger.Warn("executing rollback", "robot_id", in.RobotID, "reason", reason)
	opts := workflow.ChildWorkflowOptions{
		WorkflowID: childID(in.RolloutID, in.RobotID) + "-rollback",
		TaskQueue:  TaskQueue,
	}
	cctx := workflow.WithChildOptions(ctx, opts)
	return workflow.ExecuteChildWorkflow(cctx, OTARollback, OTARollbackInput{
		RolloutID: in.RolloutID,
		RobotID:   in.RobotID,
		Reason:    reason,
	}).Get(cctx, nil)
}

func finishFailed(ctx workflow.Context, res SingleResult, detail string) (SingleResult, error) {
	res.Phase = SingleFailed
	res.Detail = detail
	res.EndedAt = workflow.Now(ctx)
	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	})
	_ = workflow.ExecuteActivity(actx, RecordRobotResult, "", res).Get(actx, nil)
	return res, nil
}

func finishRolledBack(ctx workflow.Context, res SingleResult, detail string) (SingleResult, error) {
	res.Phase = SingleRolledBack
	res.Detail = detail
	res.EndedAt = workflow.Now(ctx)
	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	})
	_ = workflow.ExecuteActivity(actx, RecordRobotResult, "", res).Get(actx, nil)
	return res, nil
}
