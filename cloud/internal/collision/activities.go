package collision

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Activities owns the MQTT client used to publish twist commands.
type Activities struct {
	MQTT mqtt.Client
}

// SendTwist republishes the configured twist at 10 Hz for the
// requested duration so the gz DiffDrive command timeout (~0.5 s)
// doesn't stop the rover mid-phase. Sends one final 0,0 stop frame
// after the duration before returning, so the rover is at rest if
// the workflow doesn't immediately follow up.
func (a *Activities) SendTwist(ctx context.Context, args SendTwistArgs) error {
	topic := fmt.Sprintf("cmd/%s/twist", args.RobotID)
	body, err := json.Marshal(map[string]float64{
		"linear_x":  args.LinearX,
		"angular_z": args.AngularZ,
	})
	if err != nil {
		return err
	}
	stop, _ := json.Marshal(map[string]float64{"linear_x": 0, "angular_z": 0})

	end := time.Now().Add(args.Duration)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		// QoS 0: twist messages are republished at 10 Hz so a dropped
		// frame doesn't matter, and the QoS-1 PUBACK round-trip
		// causes inflight-queue saturation against EMQX. Fire-and-
		// forget is the right choice for high-rate control commands.
		tok := a.MQTT.Publish(topic, 0, false, body)
		if !tok.WaitTimeout(2 * time.Second) {
			return fmt.Errorf("mqtt publish timeout for %s", topic)
		}
		if err := tok.Error(); err != nil {
			return err
		}
		if time.Now().After(end) {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}

	// Final explicit stop frame (QoS 1 here so the rover always sees the
	// stop even if the last QoS 0 packet got dropped).
	tok := a.MQTT.Publish(topic, 1, false, stop)
	tok.WaitTimeout(2 * time.Second)
	return tok.Error()
}
