package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/example/temporal-hack/cloud/internal/collision"
)

// collision-worker hosts:
//   - Temporal worker for the CollisionResponse workflow + SendTwist
//     activity
//   - MQTT bridge that subscribes to events/+/collision and starts a
//     workflow per event
//
// Both share the same MQTT client so the workflow's outgoing twist
// publishes and the bridge's inbound collision subscriptions sit on
// one connection.
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	temporalAddr := envOr("TEMPORAL_ADDR", "localhost:7233")
	brokerURL := envOr("BROKER_URL", "tcp://localhost:1883")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	mqttCli, err := connectMQTT(brokerURL)
	if err != nil {
		logger.Error("mqtt", "err", err)
		os.Exit(1)
	}
	defer mqttCli.Disconnect(500)

	tcli, err := client.Dial(client.Options{HostPort: temporalAddr})
	if err != nil {
		logger.Error("temporal", "err", err)
		os.Exit(1)
	}
	defer tcli.Close()

	acts := &collision.Activities{MQTT: mqttCli}
	w := worker.New(tcli, collision.TaskQueue, worker.Options{})
	w.RegisterWorkflow(collision.CollisionResponse)
	w.RegisterActivity(acts.SendTwist)

	bridge := &collision.MQTTBridge{MQTT: mqttCli, Temporal: tcli, Logger: logger}
	go func() {
		if err := bridge.Start(ctx); err != nil && ctx.Err() == nil {
			logger.Error("mqtt bridge stopped", "err", err)
		}
	}()

	logger.Info("collision-worker starting",
		"task_queue", collision.TaskQueue, "broker", brokerURL, "temporal", temporalAddr)
	if err := w.Run(worker.InterruptCh()); err != nil {
		logger.Error("worker exit", "err", err)
		os.Exit(1)
	}
}

func connectMQTT(url string) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(url).
		SetClientID("collision-worker").
		// CleanSession=true on purpose: a missed collision event is
		// fine (the next contact will fire another). With clean=false
		// EMQX queues every event the bridge missed; after a storm or
		// restart the worker reconnects to a flood of replays that
		// blocks the publisher with backpressure.
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetMaxReconnectInterval(60 * time.Second).
		SetOrderMatters(false)
	cli := mqtt.NewClient(opts)
	tok := cli.Connect()
	tok.WaitTimeout(10 * time.Second)
	return cli, tok.Error()
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
