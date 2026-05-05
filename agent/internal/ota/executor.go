package ota

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Executor subscribes to cmd/{robot_id}/ota and dispatches OTA
// commands to the docker CLI. It tracks the last-known image digest
// so a rollback command can revert without the cloud needing to
// remember it.
type Executor struct {
	RobotID string
	MQTT    mqtt.Client
	Docker  *DockerCLI
	Logger  *slog.Logger

	mu               sync.Mutex
	lastSuccessDigest string // digest before the most recent swap
}

// Start subscribes to the OTA command topic. Subscription is at QoS 1
// against a persistent session so commands queued during a disconnect
// arrive on reconnect.
func (e *Executor) Start(ctx context.Context) error {
	topic := fmt.Sprintf("cmd/%s/ota", e.RobotID)
	tok := e.MQTT.Subscribe(topic, 1, func(_ mqtt.Client, msg mqtt.Message) {
		go e.handle(ctx, msg)
	})
	if !tok.WaitTimeout(10 * time.Second) || tok.Error() != nil {
		return fmt.Errorf("subscribe %s: %w", topic, tok.Error())
	}
	e.Logger.Info("ota executor subscribed", "topic", topic)
	<-ctx.Done()
	return nil
}

func (e *Executor) handle(ctx context.Context, msg mqtt.Message) {
	defer msg.Ack()
	var cmd Command
	if err := json.Unmarshal(msg.Payload(), &cmd); err != nil {
		e.Logger.Warn("ota cmd unmarshal", "err", err)
		return
	}
	e.Logger.Info("ota cmd received", "rollout", cmd.RolloutID, "action", cmd.Action, "image_ref", cmd.ImageRef)
	e.publishAck(cmd, "PHASE_RECEIVED", "")

	if cmd.Action == string(CmdRollback) {
		e.runRollback(ctx, cmd)
		return
	}
	e.runForward(ctx, cmd)
}

func (e *Executor) runForward(ctx context.Context, cmd Command) {
	if cmd.ImageRef == "" {
		e.publishAck(cmd, "PHASE_FAILED", "image_ref empty")
		return
	}

	// Pull
	if err := e.Docker.Pull(ctx, cmd.ImageRef); err != nil {
		e.publishAck(cmd, "PHASE_FAILED", "pull: "+err.Error())
		return
	}
	e.publishAck(cmd, "PHASE_PULLED", "")

	// Swap
	prev, err := e.Docker.Swap(ctx, cmd.ImageRef)
	if err != nil {
		e.publishAck(cmd, "PHASE_FAILED", "swap: "+err.Error())
		return
	}
	e.recordPrev(prev)
	e.publishAck(cmd, "PHASE_SWAPPED", "")

	// Smoke
	smokeCtx := ctx
	if cmd.SmokeTimeoutSec > 0 {
		var cancel context.CancelFunc
		smokeCtx, cancel = context.WithTimeout(ctx, time.Duration(cmd.SmokeTimeoutSec)*time.Second)
		defer cancel()
	}
	if err := e.Docker.Exec(smokeCtx, cmd.SmokeCommand); err != nil {
		// Smoke check failed — initiate self-rollback.
		e.Logger.Warn("smoke check failed; rolling back", "err", err)
		_ = e.Docker.Rollback(ctx, prev)
		e.publishAck(cmd, "PHASE_FAILED", "smoke: "+err.Error())
		e.publishAck(cmd, "PHASE_ROLLED_BACK", "self-rollback after smoke failure")
		return
	}
	e.publishAck(cmd, "PHASE_HEALTHY", "")
}

func (e *Executor) runRollback(ctx context.Context, cmd Command) {
	e.mu.Lock()
	prev := e.lastSuccessDigest
	e.mu.Unlock()
	if err := e.Docker.Rollback(ctx, prev); err != nil {
		e.publishAck(cmd, "PHASE_FAILED", "rollback: "+err.Error())
		return
	}
	e.publishAck(cmd, "PHASE_ROLLED_BACK", "")
}

func (e *Executor) recordPrev(d string) {
	if d == "" {
		return
	}
	e.mu.Lock()
	e.lastSuccessDigest = d
	e.mu.Unlock()
}

func (e *Executor) publishAck(cmd Command, phase, detail string) {
	ack := Ack{
		RolloutID: cmd.RolloutID,
		RobotID:   e.RobotID,
		Phase:     phase,
		Detail:    detail,
		At:        time.Now().UTC(),
	}
	body, _ := json.Marshal(ack)
	topic := fmt.Sprintf("ack/%s/ota", e.RobotID)
	tok := e.MQTT.Publish(topic, 1, false, body)
	if !tok.WaitTimeout(5 * time.Second) {
		e.Logger.Warn("ack publish timeout", "topic", topic, "phase", phase)
		return
	}
	if err := tok.Error(); err != nil {
		e.Logger.Warn("ack publish err", "err", err, "topic", topic, "phase", phase)
	}
}
