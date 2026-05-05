package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/example/temporal-hack/cloud/internal/store"
)

// telemetry-ingest subscribes to MQTT topics and writes samples into
// TimescaleDB. Topic shape:
//
//	tlm/{robot_id}/{stream}    -> stream="<stream>"
//	hb/{robot_id}              -> stream="heartbeat"
//
// At-least-once: we ack MQTT after the DB write succeeds. Duplicates
// are tolerated (TimescaleDB hypertable lets the operator de-dupe at
// query time; we don't try to be exactly-once on the data plane).
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	brokerURL := envOr("BROKER_URL", "tcp://localhost:1883")
	tsdbDSN := envOr("TSDB_DSN", "postgres://temporal:temporal@localhost:5432/telemetry?sslmode=disable")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	tsdb, err := store.OpenTimescale(ctx, tsdbDSN)
	if err != nil {
		logger.Error("tsdb open", "err", err)
		os.Exit(1)
	}
	defer tsdb.Close()

	var inserted atomic.Uint64

	handler := func(_ mqtt.Client, msg mqtt.Message) {
		topic := msg.Topic()
		robotID, stream, ok := parseTopic(topic)
		if !ok {
			return
		}
		if err := tsdb.InsertSample(ctx, robotID, stream, time.Now(), msg.Payload(), ""); err != nil {
			logger.Error("insert", "err", err, "topic", topic)
			return
		}
		inserted.Add(1)
		msg.Ack()
	}

	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID("telemetry-ingest").
		SetCleanSession(false).
		SetAutoReconnect(true).
		SetOnConnectHandler(func(c mqtt.Client) {
			c.Subscribe("tlm/+/+", 1, handler)
			c.Subscribe("hb/+", 1, handler)
			logger.Info("subscribed", "topics", []string{"tlm/+/+", "hb/+"})
		})

	cli := mqtt.NewClient(opts)
	if tok := cli.Connect(); tok.WaitTimeout(10*time.Second) && tok.Error() != nil {
		logger.Error("mqtt connect", "err", tok.Error())
		os.Exit(1)
	}

	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			cli.Disconnect(500)
			return
		case <-tick.C:
			logger.Info("ingest progress", "inserted_total", inserted.Load())
		}
	}
}

func parseTopic(topic string) (robotID, stream string, ok bool) {
	parts := strings.Split(topic, "/")
	switch parts[0] {
	case "tlm":
		if len(parts) != 3 {
			return "", "", false
		}
		return parts[1], parts[2], true
	case "hb":
		if len(parts) != 2 {
			return "", "", false
		}
		return parts[1], "heartbeat", true
	}
	return "", "", false
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

var _ = errors.New
