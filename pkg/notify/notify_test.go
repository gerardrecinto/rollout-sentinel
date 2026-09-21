package notify

import (
	"context"
	"testing"
	"time"
)

// fakePublisher is a test double implementing EventPublisher. It records
// every call so tests can assert on both the number of calls and the
// payload that was passed in.
type fakePublisher struct {
	steps     []StepEvent
	rollbacks []RollbackEvent
	closed    bool
	stepErr   error
	rollErr   error
}

func (f *fakePublisher) PublishStep(ctx context.Context, event StepEvent) error {
	f.steps = append(f.steps, event)
	return f.stepErr
}

func (f *fakePublisher) PublishRollback(ctx context.Context, event RollbackEvent) error {
	f.rollbacks = append(f.rollbacks, event)
	return f.rollErr
}

func (f *fakePublisher) Close() error {
	f.closed = true
	return nil
}

func TestStepSubject(t *testing.T) {
	got := StepSubject("retail-checkout-api")
	want := "rollout-sentinel.retail-checkout-api.step"
	if got != want {
		t.Fatalf("StepSubject: got %q, want %q", got, want)
	}
}

func TestRollbackSubject(t *testing.T) {
	got := RollbackSubject("retail-checkout-api")
	want := "rollout-sentinel.retail-checkout-api.rollback"
	if got != want {
		t.Fatalf("RollbackSubject: got %q, want %q", got, want)
	}
}

func TestFakePublisherRecordsStepEvent(t *testing.T) {
	f := &fakePublisher{}
	event := StepEvent{
		Timestamp: time.Now(),
		Service:   "retail-checkout-api",
		Namespace: "production",
		Weight:    20,
		Verdict:   "PASS",
		Summary:   "All 5 SLO gates passed at 20% traffic weight.",
		GateResults: []GateResult{
			{GateName: "ErrorRateGate", Passed: true, ActualValue: 0.09, Threshold: 0.5, Details: "ok"},
		},
	}

	if err := f.PublishStep(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(f.steps) != 1 {
		t.Fatalf("expected 1 recorded step event, got %d", len(f.steps))
	}
	got := f.steps[0]
	if got.Service != "retail-checkout-api" || got.Namespace != "production" {
		t.Fatalf("unexpected service/namespace on recorded event: %+v", got)
	}
	if got.Weight != 20 {
		t.Fatalf("expected weight 20, got %d", got.Weight)
	}
	if got.Verdict != "PASS" {
		t.Fatalf("expected verdict PASS, got %s", got.Verdict)
	}
	if len(got.GateResults) != 1 || got.GateResults[0].GateName != "ErrorRateGate" {
		t.Fatalf("gate results not carried through: %+v", got.GateResults)
	}
}

func TestFakePublisherRecordsRollbackEvent(t *testing.T) {
	f := &fakePublisher{}
	event := RollbackEvent{
		Timestamp: time.Now(),
		Service:   "retail-checkout-api",
		Namespace: "production",
		Weight:    20,
		Verdict:   "BREACH",
		Reason:    "SLO Breach: One or more critical quality gates failed at 20% traffic weight.",
	}

	if err := f.PublishRollback(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(f.rollbacks) != 1 {
		t.Fatalf("expected 1 recorded rollback event, got %d", len(f.rollbacks))
	}
	got := f.rollbacks[0]
	if got.Verdict != "BREACH" {
		t.Fatalf("expected verdict BREACH, got %s", got.Verdict)
	}
	if got.Reason == "" {
		t.Fatal("expected rollback reason to be carried through")
	}
}

func TestNoopPublisherDoesNothing(t *testing.T) {
	var pub EventPublisher = NoopPublisher{}

	if err := pub.PublishStep(context.Background(), StepEvent{Service: "svc"}); err != nil {
		t.Fatalf("unexpected error from NoopPublisher.PublishStep: %v", err)
	}
	if err := pub.PublishRollback(context.Background(), RollbackEvent{Service: "svc"}); err != nil {
		t.Fatalf("unexpected error from NoopPublisher.PublishRollback: %v", err)
	}
	if err := pub.Close(); err != nil {
		t.Fatalf("unexpected error from NoopPublisher.Close: %v", err)
	}
}

func TestFakePublisherPropagatesErrors(t *testing.T) {
	wantErr := context.DeadlineExceeded
	f := &fakePublisher{stepErr: wantErr, rollErr: wantErr}

	if err := f.PublishStep(context.Background(), StepEvent{}); err != wantErr {
		t.Fatalf("expected error to propagate, got %v", err)
	}
	if err := f.PublishRollback(context.Background(), RollbackEvent{}); err != wantErr {
		t.Fatalf("expected error to propagate, got %v", err)
	}
}
