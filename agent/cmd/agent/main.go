package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/temporal-hack/agent/internal/buffer"
	"github.com/example/temporal-hack/agent/internal/mqttx"
	"github.com/example/temporal-hack/agent/internal/telemetry"
)

type config struct {
	robotID     string
	brokerURL   string
	bufferPath  string
	bridgeAddr  string
	heartbeatMs int
}

func loadConfig() config {
	c := config{
		robotID:     os.Getenv("ROBOT_ID"),
		brokerURL:   os.Getenv("BROKER_URL"),
		bufferPath:  os.Getenv("BUFFER_PATH"),
		bridgeAddr:  os.Getenv("BRIDGE_ADDR"),
		heartbeatMs: 10000,
	}
	flag.StringVar(&c.robotID, "robot-id", c.robotID, "stable robot identifier")
	flag.StringVar(&c.brokerURL, "broker", firstNonEmpty(c.brokerURL, "tcp://localhost:1883"), "MQTT broker URL")
	flag.StringVar(&c.bufferPath, "buffer", firstNonEmpty(c.bufferPath, "/var/lib/temporal-hack-agent/buffer.db"), "SQLite buffer path")
	flag.StringVar(&c.bridgeAddr, "bridge", firstNonEmpty(c.bridgeAddr, "unix:///run/temporal-hack-bridge.sock"), "ROS bridge gRPC address")
	flag.IntVar(&c.heartbeatMs, "heartbeat-ms", c.heartbeatMs, "heartbeat interval in ms")
	flag.Parse()
	return c
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func main() {
	cfg := loadConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if cfg.robotID == "" {
		logger.Error("ROBOT_ID is required")
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	buf, err := buffer.Open(cfg.bufferPath)
	if err != nil {
		logger.Error("buffer open", "err", err)
		os.Exit(1)
	}
	defer buf.Close()

	pub, err := mqttx.NewPublisher(mqttx.PublisherConfig{
		BrokerURL: cfg.brokerURL,
		ClientID:  "agent-" + cfg.robotID,
		Logger:    logger,
	})
	if err != nil {
		logger.Error("mqtt publisher", "err", err)
		os.Exit(1)
	}
	defer pub.Close()

	pump := telemetry.NewPump(telemetry.PumpConfig{
		RobotID:    cfg.robotID,
		Buffer:     buf,
		Publisher:  pub,
		BridgeAddr: cfg.bridgeAddr,
		Streams:    []string{"battery", "pose", "diag"},
		Logger:     logger,
	})

	logger.Info("agent starting", "robot_id", cfg.robotID, "broker", cfg.brokerURL)
	if err := pump.Run(ctx); err != nil && ctx.Err() == nil {
		logger.Error("pump exited", "err", err)
		os.Exit(1)
	}
	logger.Info("agent stopped")
}
