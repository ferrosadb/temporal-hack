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

	"github.com/example/temporal-hack/cloud/internal/fleet"
	"github.com/example/temporal-hack/cloud/internal/ota"
	"github.com/example/temporal-hack/cloud/internal/store"
)

// ota-worker hosts:
//   - Temporal worker for OTA workflows + activities
//   - MQTT bridge that translates OTAAck messages into workflow signals
//
// Both share the same MQTT client and Temporal client so that
// activities and the bridge see consistent broker state.
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	temporalAddr := envOr("TEMPORAL_ADDR", "localhost:7233")
	brokerURL := envOr("BROKER_URL", "tcp://localhost:1883")
	tsdbDSN := envOr("TSDB_DSN", "postgres://temporal:temporal@localhost:5432/telemetry?sslmode=disable")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	tsdb, err := store.OpenTimescale(ctx, tsdbDSN)
	if err != nil {
		logger.Error("tsdb", "err", err)
		os.Exit(1)
	}
	defer tsdb.Close()

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

	registry := fleet.NewRegistry(tsdb.Pool())
	acts := &ota.Activities{MQTT: mqttCli, Store: tsdb, Registry: registry}

	w := worker.New(tcli, ota.TaskQueue, worker.Options{})
	w.RegisterWorkflow(ota.OTARollout)
	w.RegisterWorkflow(ota.OTASingleRobot)
	w.RegisterWorkflow(ota.OTARollback)
	w.RegisterActivity(acts.SendOTACommand)
	w.RegisterActivity(acts.SendRollbackCommand)
	w.RegisterActivity(acts.ResolveCohort)
	w.RegisterActivity(acts.RecordRolloutStarted)
	w.RegisterActivity(acts.RecordRolloutEnded)
	w.RegisterActivity(acts.RecordRobotResult)
	w.RegisterActivity(acts.RecordRollbackOutcome)

	bridge := &ota.MQTTBridge{MQTT: mqttCli, Temporal: tcli, Logger: logger}
	go func() {
		if err := bridge.Start(ctx); err != nil && ctx.Err() == nil {
			logger.Error("mqtt bridge stopped", "err", err)
		}
	}()

	logger.Info("ota-worker starting", "task_queue", ota.TaskQueue, "broker", brokerURL, "temporal", temporalAddr)
	if err := w.Run(worker.InterruptCh()); err != nil {
		logger.Error("worker exit", "err", err)
		os.Exit(1)
	}
}

func connectMQTT(url string) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(url).
		SetClientID("ota-worker").
		SetCleanSession(false).
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
