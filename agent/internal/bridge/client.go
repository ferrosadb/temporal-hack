package bridge

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/example/temporal-hack/agent/internal/bridge/pb"
)

// Client is the agent-side gRPC client for the on-robot ROS 2 bridge
// node. The address is typically a Unix domain socket
// (`unix:///run/temporal-hack-bridge.sock`) but a TCP `host:port` is
// also accepted for cross-container sim setups.
type Client struct {
	addr   string
	logger *slog.Logger

	// reconnect backoff caps. Bridge restarts (e.g. during a sim
	// rebuild) should not knock the agent over.
	BackoffInitial time.Duration
	BackoffMax     time.Duration
}

// Event is the agent-facing form of a bridge TopicEvent: a transport-
// agnostic struct that the telemetry pump can hand to the buffer.
type Event struct {
	Stream     string
	CapturedAt time.Time
	Payload    []byte
	Schema     string
}

func New(addr string, logger *slog.Logger) *Client {
	return &Client{
		addr:           addr,
		logger:         logger,
		BackoffInitial: 500 * time.Millisecond,
		BackoffMax:     10 * time.Second,
	}
}

// Subscribe runs a long-lived loop: dial the bridge, request a stream
// for the named streams, push every incoming TopicEvent onto the
// returned channel. On disconnect the loop reconnects with capped
// exponential backoff. The channel is closed when ctx is canceled.
func (c *Client) Subscribe(ctx context.Context, streams []string) <-chan Event {
	out := make(chan Event, 256)
	go c.run(ctx, streams, out)
	return out
}

func (c *Client) run(ctx context.Context, streams []string, out chan<- Event) {
	defer close(out)
	backoff := c.BackoffInitial

	for {
		if err := ctx.Err(); err != nil {
			return
		}
		if err := c.runOnce(ctx, streams, out); err != nil && ctx.Err() == nil {
			c.logger.Warn("bridge subscribe failed; will retry", "err", err, "in", backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = minDuration(backoff*2, c.BackoffMax)
			continue
		}
		backoff = c.BackoffInitial
	}
}

func (c *Client) runOnce(ctx context.Context, streams []string, out chan<- Event) error {
	conn, err := dial(ctx, c.addr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", c.addr, err)
	}
	defer conn.Close()

	cli := pb.NewBridgeClient(conn)
	stream, err := cli.Subscribe(ctx, &pb.SubscribeRequest{Streams: streams})
	if err != nil {
		return fmt.Errorf("Subscribe RPC: %w", err)
	}
	c.logger.Info("bridge connected", "addr", c.addr, "streams", streams)

	for {
		ev, err := stream.Recv()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}
		select {
		case out <- Event{
			Stream:     ev.GetStream(),
			CapturedAt: ev.GetCapturedAt().AsTime(),
			Payload:    ev.GetPayload(),
			Schema:     ev.GetPayloadSchema(),
		}:
		case <-ctx.Done():
			return nil
		default:
			// Buffer full — agent's local buffer is downstream and
			// will absorb. Drop here only if even the channel is
			// saturated (agent backed up). Logged loud, not silent.
			c.logger.Warn("bridge event channel full; dropping", "stream", ev.GetStream())
		}
	}
}

// dial supports both unix:// and tcp host:port forms.
func dial(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if strings.HasPrefix(addr, "unix://") {
		path := strings.TrimPrefix(addr, "unix://")
		return grpc.DialContext(dialCtx, "passthrough:///"+path,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", path)
			}),
		)
	}
	return grpc.DialContext(dialCtx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
