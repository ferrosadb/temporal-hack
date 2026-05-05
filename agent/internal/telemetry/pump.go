package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/example/temporal-hack/agent/internal/bridge"
	"github.com/example/temporal-hack/agent/internal/buffer"
	"github.com/example/temporal-hack/agent/internal/mqttx"
)

// Pump owns the agent's data plane. It runs three concurrent loops:
//
//  1. ingest: pull TopicEvents from the bridge gRPC stream and append
//     to the local buffer
//  2. publish: drain buffered samples to the MQTT broker
//  3. heartbeat: emit a Heartbeat at a fixed interval regardless of
//     whether telemetry is flowing
//
// During disconnect the publish loop quietly fails and the buffer
// grows. On reconnect the buffer drains in FIFO order. The publish
// loop is the only path that removes samples from the buffer; ingest
// always appends.
type Pump struct {
	cfg PumpConfig
}

type PumpConfig struct {
	RobotID    string
	Buffer     *buffer.Buffer
	Publisher  *mqttx.Publisher
	BridgeAddr string
	Streams    []string
	Logger     *slog.Logger
}

func NewPump(cfg PumpConfig) *Pump { return &Pump{cfg: cfg} }

// Run blocks until ctx is canceled. Returns ctx.Err() on clean shutdown.
func (p *Pump) Run(ctx context.Context) error {
	errc := make(chan error, 3)

	go func() { errc <- p.runIngest(ctx) }()
	go func() { errc <- p.runPublish(ctx) }()
	go func() { errc <- p.runHeartbeat(ctx) }()

	for i := 0; i < 3; i++ {
		if err := <-errc; err != nil && ctx.Err() == nil {
			return err
		}
	}
	return ctx.Err()
}

// runIngest dials the bridge gRPC server (via the agent/internal/bridge
// client, which handles reconnect) and appends each TopicEvent into
// the local buffer as a Sample. The bridge is the only on-robot
// process that touches ROS / DDS — see ADR-004 and D-06.
func (p *Pump) runIngest(ctx context.Context) error {
	cli := bridge.New(p.cfg.BridgeAddr, p.cfg.Logger)
	events := cli.Subscribe(ctx, p.cfg.Streams)
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			capturedNanos := ev.CapturedAt.UnixNano()
			if capturedNanos <= 0 {
				capturedNanos = time.Now().UnixNano()
			}
			s := buffer.Sample{
				RobotID:    p.cfg.RobotID,
				Stream:     ev.Stream,
				CapturedAt: capturedNanos,
				Payload:    ev.Payload,
				Schema:     ev.Schema,
			}
			if err := p.cfg.Buffer.Append(s); err != nil {
				p.cfg.Logger.Error("buffer append", "err", err)
			}
		}
	}
}

var _ = fmt.Sprintf // retained for log strings elsewhere

// runPublish drains the buffer to MQTT in FIFO order. On publish
// failure, the sample stays in the buffer and the loop backs off.
func (p *Pump) runPublish(ctx context.Context) error {
	const batch = 64
	const flushInterval = 500 * time.Millisecond
	const backoffMax = 10 * time.Second
	backoff := 100 * time.Millisecond
	t := time.NewTicker(flushInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}

		if !p.cfg.Publisher.IsConnected() {
			time.Sleep(backoff)
			backoff = min(backoff*2, backoffMax)
			continue
		}

		samples, err := p.cfg.Buffer.Peek(batch)
		if err != nil {
			p.cfg.Logger.Error("buffer peek", "err", err)
			continue
		}
		if len(samples) == 0 {
			backoff = 100 * time.Millisecond
			continue
		}

		acked := make([]int64, 0, len(samples))
		for _, s := range samples {
			topic := fmt.Sprintf("tlm/%s/%s", s.RobotID, s.Stream)
			if err := p.cfg.Publisher.Publish(topic, s.Payload, 5*time.Second); err != nil {
				p.cfg.Logger.Warn("publish failed", "err", err, "stream", s.Stream)
				break
			}
			acked = append(acked, s.ID)
		}
		if len(acked) > 0 {
			if err := p.cfg.Buffer.Ack(acked); err != nil {
				p.cfg.Logger.Error("buffer ack", "err", err, "count", len(acked))
			}
			backoff = 100 * time.Millisecond
		} else {
			time.Sleep(backoff)
			backoff = min(backoff*2, backoffMax)
		}
	}
}

func (p *Pump) runHeartbeat(ctx context.Context) error {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	topic := fmt.Sprintf("hb/%s", p.cfg.RobotID)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			n, _ := p.cfg.Buffer.Count()
			payload := fmt.Sprintf(
				`{"robot_id":%q,"buffered":%d,"emitted_at":%q,"status":"ok"}`,
				p.cfg.RobotID, n, time.Now().UTC().Format(time.RFC3339Nano),
			)
			if err := p.cfg.Publisher.Publish(topic, []byte(payload), 5*time.Second); err != nil {
				p.cfg.Logger.Debug("heartbeat publish failed", "err", err)
			}
		}
	}
}
