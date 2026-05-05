package mqttx

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Publisher wraps a paho MQTT client with the v1 connection policy:
// persistent session (CleanStart=false), QoS 1 publishes, automatic
// reconnect with exponential backoff, large in-flight queue. Identity
// is PSK-based per D-11; this package will be replaced with an mTLS
// variant when the identity work item closes.
type Publisher struct {
	cli    mqtt.Client
	logger *slog.Logger
	mu     sync.RWMutex
}

type PublisherConfig struct {
	BrokerURL string
	ClientID  string
	Username  string
	Password  string
	Logger    *slog.Logger
}

func NewPublisher(cfg PublisherConfig) (*Publisher, error) {
	if cfg.ClientID == "" {
		return nil, errors.New("ClientID required")
	}
	opts := mqtt.NewClientOptions().
		AddBroker(cfg.BrokerURL).
		SetClientID(cfg.ClientID).
		SetCleanSession(false).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetMaxReconnectInterval(60 * time.Second).
		SetOrderMatters(false).
		SetMessageChannelDepth(4096)

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username).SetPassword(cfg.Password)
	}

	logger := cfg.Logger
	opts.OnConnectionLost = func(_ mqtt.Client, err error) {
		logger.Warn("mqtt connection lost", "err", err)
	}
	opts.OnReconnecting = func(_ mqtt.Client, _ *mqtt.ClientOptions) {
		logger.Info("mqtt reconnecting")
	}
	opts.OnConnect = func(_ mqtt.Client) {
		logger.Info("mqtt connected", "broker", cfg.BrokerURL)
	}

	cli := mqtt.NewClient(opts)
	tok := cli.Connect()
	// Don't block on initial connect — paho will retry. We only error
	// if the option setup itself failed.
	tok.WaitTimeout(2 * time.Second)
	return &Publisher{cli: cli, logger: logger}, nil
}

// Publish sends a message at QoS 1 and blocks until the broker
// acknowledges or the timeout elapses. The caller is responsible for
// only ack'ing the buffered sample on success.
func (p *Publisher) Publish(topic string, payload []byte, timeout time.Duration) error {
	p.mu.RLock()
	cli := p.cli
	p.mu.RUnlock()
	if !cli.IsConnectionOpen() {
		return fmt.Errorf("mqtt: not connected")
	}
	tok := cli.Publish(topic, 1, false, payload)
	if !tok.WaitTimeout(timeout) {
		return fmt.Errorf("mqtt: publish timed out after %s", timeout)
	}
	return tok.Error()
}

// IsConnected reports whether the underlying client has an active session.
func (p *Publisher) IsConnected() bool {
	return p.cli.IsConnectionOpen()
}

func (p *Publisher) Close() {
	p.cli.Disconnect(250)
}
