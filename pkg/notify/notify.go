// Package notify publishes rollout-sentinel step and rollback events to an
// external system (a dashboard, an alerting bot, anything subscribing) so
// those systems can observe canary progress in real time instead of only
// reading stdout.
package notify

import (
	"context"
	"fmt"
	"time"
)

// GateResult mirrors the fields of evaluator.GateResult that are useful to
// an external subscriber. It's a separate type on purpose: notify is meant
// to be a standalone wire schema, not a re-export of internal evaluator types.
type GateResult struct {
	GateName    string  `json:"gate_name"`
	Passed      bool    `json:"passed"`
	ActualValue float64 `json:"actual_value"`
	Threshold   float64 `json:"threshold"`
	Details     string  `json:"details"`
}

// StepEvent is published once per canary step evaluation, pass or fail.
type StepEvent struct {
	Timestamp   time.Time    `json:"timestamp"`
	Service     string       `json:"service"`
	Namespace   string       `json:"namespace"`
	Weight      int          `json:"weight_percent"`
	Verdict     string       `json:"verdict"`
	Summary     string       `json:"summary"`
	GateResults []GateResult `json:"gate_results"`
}

// RollbackEvent is published the moment a rollback is triggered, in addition
// to the StepEvent for that same step, so a subscriber can alert on rollback
// alone without parsing every step event's verdict.
type RollbackEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Service   string    `json:"service"`
	Namespace string    `json:"namespace"`
	Weight    int       `json:"weight_percent"`
	Verdict   string    `json:"verdict"`
	Reason    string    `json:"reason"`
}

// EventPublisher publishes rollout-sentinel events to an external sink.
// The zero-value behavior of the CLI is NoopPublisher: nothing is published
// unless a sink (currently NATS) is explicitly configured with -nats-url.
type EventPublisher interface {
	PublishStep(ctx context.Context, event StepEvent) error
	PublishRollback(ctx context.Context, event RollbackEvent) error
	Close() error
}

// NoopPublisher discards every event. It's the default EventPublisher so the
// CLI behaves exactly as it did before this package existed.
type NoopPublisher struct{}

func (NoopPublisher) PublishStep(ctx context.Context, event StepEvent) error { return nil }

func (NoopPublisher) PublishRollback(ctx context.Context, event RollbackEvent) error { return nil }

func (NoopPublisher) Close() error { return nil }

// StepSubject is the NATS subject a StepEvent for the given service is
// published on: rollout-sentinel.<service>.step
func StepSubject(service string) string {
	return fmt.Sprintf("rollout-sentinel.%s.step", service)
}

// RollbackSubject is the NATS subject a RollbackEvent for the given service
// is published on: rollout-sentinel.<service>.rollback
func RollbackSubject(service string) string {
	return fmt.Sprintf("rollout-sentinel.%s.rollback", service)
}
