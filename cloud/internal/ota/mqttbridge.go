package ota

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.temporal.io/sdk/client"
)

// MQTTBridge subscribes to ack/+/ota and translates each ack message
// into a Temporal signal targeting the workflow execution that owns
// the (rollout_id, robot_id) pair. Workflow IDs are deterministic
// (rollout-{rolloutID}-robot-{robotID}) so the bridge does not need
// to track open executions — it just signals by ID.
//
// SignalWorkflow on a closed/non-existent execution returns
// "workflow execution not found" which we log and drop.
type MQTTBridge struct {
	MQTT     mqtt.Client
	Temporal client.Client
	Logger   *slog.Logger
}

// Start subscribes and runs until the context is canceled.
func (b *MQTTBridge) Start(ctx context.Context) error {
	if tok := b.MQTT.Subscribe("ack/+/ota", 1, b.onMessage(ctx)); tok.Wait() && tok.Error() != nil {
		return tok.Error()
	}
	b.Logger.Info("ota mqtt bridge subscribed", "topic", "ack/+/ota")
	<-ctx.Done()
	return nil
}

func (b *MQTTBridge) onMessage(ctx context.Context) mqtt.MessageHandler {
	return func(_ mqtt.Client, msg mqtt.Message) {
		topic := msg.Topic()
		parts := strings.Split(topic, "/")
		if len(parts) != 3 || parts[0] != "ack" || parts[2] != "ota" {
			b.Logger.Warn("unexpected topic", "topic", topic)
			return
		}
		robotID := parts[1]

		var ack struct {
			RolloutID           string `json:"rollout_id"`
			Phase               string `json:"phase"`
			Detail              string `json:"detail"`
			ImageDigest         string `json:"image_digest"`
			PreviousImageDigest string `json:"previous_image_digest"`
		}
		if err := json.Unmarshal(msg.Payload(), &ack); err != nil {
			b.Logger.Warn("ack unmarshal", "err", err, "topic", topic)
			return
		}

		// Determine target workflow: rollback workflow has -rollback suffix,
		// signaled only when the ack phase is PHASE_ROLLED_BACK.
		var targetWorkflowID string
		if ack.Phase == "PHASE_ROLLED_BACK" {
			targetWorkflowID = fmt.Sprintf("%s-robot-%s-rollback", ack.RolloutID, robotID)
		} else {
			targetWorkflowID = fmt.Sprintf("%s-robot-%s", ack.RolloutID, robotID)
		}

		sig := AckSignal{
			Phase:               ack.Phase,
			Detail:              ack.Detail,
			ImageDigest:         ack.ImageDigest,
			PreviousImageDigest: ack.PreviousImageDigest,
		}
		if err := b.Temporal.SignalWorkflow(ctx, targetWorkflowID, "", SignalAck, sig); err != nil {
			b.Logger.Warn("signal workflow", "err", err, "workflow_id", targetWorkflowID, "phase", ack.Phase)
			return
		}
		b.Logger.Debug("signaled workflow", "workflow_id", targetWorkflowID, "phase", ack.Phase)
		msg.Ack()
	}
}
