package notify

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
)

// NATSPublisher publishes rollout-sentinel events as JSON messages to a NATS
// server. It satisfies EventPublisher.
type NATSPublisher struct {
	conn *nats.Conn
}

// NewNATSPublisher connects to the NATS server at url and returns a ready to
// use publisher. The connection uses nats.go's default reconnect behavior,
// so a server that's briefly unreachable won't take the CLI down with it.
func NewNATSPublisher(url string) (*NATSPublisher, error) {
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("connecting to nats at %s: %w", url, err)
	}
	return &NATSPublisher{conn: conn}, nil
}

// PublishStep marshals event to JSON and publishes it to StepSubject(event.Service).
func (p *NATSPublisher) PublishStep(ctx context.Context, event StepEvent) error {
	return p.publish(StepSubject(event.Service), event)
}

// PublishRollback marshals event to JSON and publishes it to RollbackSubject(event.Service).
func (p *NATSPublisher) PublishRollback(ctx context.Context, event RollbackEvent) error {
	return p.publish(RollbackSubject(event.Service), event)
}

func (p *NATSPublisher) publish(subject string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling event for subject %s: %w", subject, err)
	}
	if err := p.conn.Publish(subject, data); err != nil {
		return fmt.Errorf("publishing to subject %s: %w", subject, err)
	}
	return nil
}

// Close drains the connection, waiting for any in-flight publishes to reach
// the server before closing.
func (p *NATSPublisher) Close() error {
	if p.conn == nil {
		return nil
	}
	return p.conn.Drain()
}
