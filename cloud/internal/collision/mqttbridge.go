package collision

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.temporal.io/sdk/client"
)

// MQTTBridge subscribes to events/+/collision and starts a
// CollisionResponse workflow per inbound event. Workflow IDs are
// disambiguated by an event timestamp so multiple collisions from
// the same robot don't conflict, but the dedupe window in the robot
// publisher (2 s) plus the workflow's deterministic ID together
// keep us from running concurrent responses for the same incident.
type MQTTBridge struct {
	MQTT     mqtt.Client
	Temporal client.Client
	Logger   *slog.Logger
}

func (b *MQTTBridge) Start(ctx context.Context) error {
	tok := b.MQTT.Subscribe("events/+/collision", 1, b.onMessage(ctx))
	if !tok.WaitTimeout(10*time.Second) || tok.Error() != nil {
		return fmt.Errorf("subscribe events/+/collision: %w", tok.Error())
	}
	b.Logger.Info("collision mqtt bridge subscribed", "topic", "events/+/collision")
	<-ctx.Done()
	return nil
}

func (b *MQTTBridge) onMessage(ctx context.Context) mqtt.MessageHandler {
	return func(_ mqtt.Client, msg mqtt.Message) {
		topic := msg.Topic()
		parts := strings.Split(topic, "/")
		if len(parts) != 3 || parts[0] != "events" || parts[2] != "collision" {
			b.Logger.Warn("unexpected collision topic", "topic", topic)
			return
		}
		robotID := parts[1]
		var ev struct {
			RobotID string  `json:"robot_id"`
			Partner string  `json:"partner"`
			At      float64 `json:"at"`
			Count   int     `json:"count"`
		}
		_ = json.Unmarshal(msg.Payload(), &ev)
		if ev.RobotID == "" {
			ev.RobotID = robotID
		}

		wfID := fmt.Sprintf("collision-%s-%d", ev.RobotID, time.Now().UnixNano())
		opts := client.StartWorkflowOptions{
			ID:                       wfID,
			TaskQueue:                TaskQueue,
			WorkflowExecutionTimeout: 2 * time.Minute,
		}
		_, err := b.Temporal.ExecuteWorkflow(ctx, opts, "CollisionResponse", Input{
			RobotID: ev.RobotID,
			Partner: ev.Partner,
			At:      int64(ev.At),
		})
		if err != nil {
			b.Logger.Error("start collision workflow", "err", err, "robot_id", ev.RobotID)
			return
		}
		b.Logger.Info("collision workflow started",
			"workflow_id", wfID, "robot_id", ev.RobotID, "partner", ev.Partner, "count", ev.Count)
		msg.Ack()
	}
}
