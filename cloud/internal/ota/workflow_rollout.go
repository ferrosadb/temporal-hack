package ota

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// TaskQueue is the Temporal task queue name shared by all OTA
// workflows and activities.
const TaskQueue = "ota"

// OTARollout is the parent workflow. It resolves the cohort, runs a
// canary, then a 25% batch, then the rest. A failure in any phase
// either continues (within the failure budget) or aborts the rollout.
//
// Workflow ID convention:  rollout-{uuid}
// Each child uses:         rollout-{uuid}-robot-{robot_id}
//
// The workflow is deterministic: all wall-clock and IO operations
// happen via activities or workflow.Sleep / workflow.NewTimer.
func OTARollout(ctx workflow.Context, spec RolloutSpec) (string, error) {
	spec = defaultsApplied(spec)

	rolloutID := workflow.GetInfo(ctx).WorkflowExecution.ID
	logger := workflow.GetLogger(ctx)
	logger.Info("rollout starting", "image_ref", spec.ImageRef, "canary_size", spec.CanarySize)

	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumInterval: 30 * time.Second,
			MaximumAttempts: 5,
		},
	})

	if err := workflow.ExecuteActivity(actx, RecordRolloutStarted, rolloutID, spec).Get(actx, nil); err != nil {
		return "", err
	}

	var cohort []string
	if err := workflow.ExecuteActivity(actx, ResolveCohort, spec.CohortSelector).Get(actx, &cohort); err != nil {
		_ = workflow.ExecuteActivity(actx, RecordRolloutEnded, rolloutID, StatusAborted, "resolve cohort failed: "+err.Error()).Get(actx, nil)
		return string(StatusAborted), err
	}

	if len(cohort) == 0 {
		_ = workflow.ExecuteActivity(actx, RecordRolloutEnded, rolloutID, StatusEmptyCohort, "no robots matched selector").Get(actx, nil)
		return string(StatusEmptyCohort), nil
	}

	// Canary phase
	canarySize := minInt(spec.CanarySize, len(cohort))
	canaryRobots := cohort[:canarySize]
	canaryResults, err := runBatch(ctx, rolloutID, spec, canaryRobots, "canary")
	if err != nil {
		return string(StatusAborted), err
	}
	if failureCount(canaryResults) > 0 {
		_ = workflow.ExecuteActivity(actx, RecordRolloutEnded, rolloutID, StatusCanaryFailed,
			fmt.Sprintf("canary failed on %d robot(s)", failureCount(canaryResults))).Get(actx, nil)
		return string(StatusCanaryFailed), nil
	}

	rest := cohort[canarySize:]
	if len(rest) == 0 {
		_ = workflow.ExecuteActivity(actx, RecordRolloutEnded, rolloutID, StatusCompleted, "").Get(actx, nil)
		return string(StatusCompleted), nil
	}

	// 25% batch
	batchSize := maxInt(1, len(rest)*spec.BatchPercent/100)
	batchRobots := rest[:minInt(batchSize, len(rest))]
	batchResults, err := runBatch(ctx, rolloutID, spec, batchRobots, "batch1")
	if err != nil {
		return string(StatusAborted), err
	}
	if failureCount(batchResults) > spec.FailureBudget {
		_ = workflow.ExecuteActivity(actx, RecordRolloutEnded, rolloutID, StatusAborted,
			fmt.Sprintf("batch1 failures %d exceeded budget %d", failureCount(batchResults), spec.FailureBudget)).Get(actx, nil)
		return string(StatusAborted), nil
	}

	// Remainder
	remainder := rest[minInt(batchSize, len(rest)):]
	if len(remainder) > 0 {
		remResults, err := runBatch(ctx, rolloutID, spec, remainder, "batch2")
		if err != nil {
			return string(StatusAborted), err
		}
		if failureCount(remResults) > spec.FailureBudget*4 {
			_ = workflow.ExecuteActivity(actx, RecordRolloutEnded, rolloutID, StatusAborted,
				fmt.Sprintf("batch2 failures %d exceeded scaled budget", failureCount(remResults))).Get(actx, nil)
			return string(StatusAborted), nil
		}
	}

	_ = workflow.ExecuteActivity(actx, RecordRolloutEnded, rolloutID, StatusCompleted, "").Get(actx, nil)
	return string(StatusCompleted), nil
}

// runBatch executes OTASingleRobot children in parallel for the given
// cohort slice and waits for all to complete. Failures do not cancel
// peers in the same batch — the parent decides what to do with the
// per-robot result tally after the batch returns.
func runBatch(ctx workflow.Context, rolloutID string, spec RolloutSpec, robots []string, phaseName string) ([]SingleResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("running batch", "phase", phaseName, "size", len(robots))

	futures := make([]workflow.ChildWorkflowFuture, len(robots))
	for i, r := range robots {
		opts := workflow.ChildWorkflowOptions{
			WorkflowID: childID(rolloutID, r),
			TaskQueue:  TaskQueue,
		}
		cctx := workflow.WithChildOptions(ctx, opts)
		futures[i] = workflow.ExecuteChildWorkflow(cctx, OTASingleRobot, OTASingleRobotInput{
			RolloutID: rolloutID,
			RobotID:   r,
			Spec:      spec,
		})
	}

	results := make([]SingleResult, len(robots))
	for i, f := range futures {
		var res SingleResult
		if err := f.Get(ctx, &res); err != nil {
			res = SingleResult{RobotID: robots[i], Phase: SingleFailed, Detail: err.Error()}
		}
		results[i] = res
	}
	return results, nil
}

func childID(rolloutID, robotID string) string {
	return fmt.Sprintf("%s-robot-%s", rolloutID, robotID)
}

func failureCount(rs []SingleResult) int {
	n := 0
	for _, r := range rs {
		if r.Phase != SingleOK {
			n++
		}
	}
	return n
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
