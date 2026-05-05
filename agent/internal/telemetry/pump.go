package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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

// runIngest will: dial the bridge gRPC server, call Subscribe with the
// configured streams, and append every TopicEvent it receives to the
// buffer as a Sample. Reconnect on disconnect.
//
// TODO(S1): wire up bridge gRPC client. For now the loop is a stub
// that emits a fake "battery" sample every 5s so end-to-end can be
// exercised without the bridge running.
func (p *Pump) runIngest(ctx context.Context) error {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	var seq uint64
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			seq++
			s := buffer.Sample{
				RobotID:    p.cfg.RobotID,
				Stream:     "battery",
				CapturedAt: time.Now().UnixNano(),
				Payload:    []byte(fmt.Sprintf(`{"voltage_v":24.%d,"seq":%d}`, seq%10, seq)),
				Schema:     "stub:battery@v0",
			}
			if err := p.cfg.Buffer.Append(s); err != nil {
				p.cfg.Logger.Error("buffer append", "err", err)
			}
		}
	}
}

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
