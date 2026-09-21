package analyzer

import (
	"context"
	"testing"

	"github.com/gerardrecinto/rollout-sentinel/pkg/client"
	"github.com/gerardrecinto/rollout-sentinel/pkg/config"
	"github.com/gerardrecinto/rollout-sentinel/pkg/notify"
	"github.com/gerardrecinto/rollout-sentinel/pkg/triage"
)

// recordingPublisher is a test double for notify.EventPublisher used to
// verify the Controller calls the publisher with the right events at the
// right time, without needing a real NATS server.
type recordingPublisher struct {
	steps     []notify.StepEvent
	rollbacks []notify.RollbackEvent
}

func (r *recordingPublisher) PublishStep(ctx context.Context, event notify.StepEvent) error {
	r.steps = append(r.steps, event)
	return nil
}

func (r *recordingPublisher) PublishRollback(ctx context.Context, event notify.RollbackEvent) error {
	r.rollbacks = append(r.rollbacks, event)
	return nil
}

func (r *recordingPublisher) Close() error { return nil }

func TestControllerDefaultsToNoopPublisher(t *testing.T) {
	cfg := config.DefaultConfig()
	ctrl := NewController(cfg, &client.MockK8sClient{}, &client.MockMetricsProvider{}, &triage.MockLLMClient{})

	if ctrl.publisher == nil {
		t.Fatal("expected NewController to default publisher to a non-nil NoopPublisher")
	}
	if _, ok := ctrl.publisher.(notify.NoopPublisher); !ok {
		t.Fatalf("expected default publisher to be notify.NoopPublisher, got %T", ctrl.publisher)
	}
}

func TestControllerPublishesStepEventOnHealthyPass(t *testing.T) {
	cfg := config.DefaultConfig()
	ctrl := NewController(cfg, &client.MockK8sClient{}, &client.MockMetricsProvider{}, &triage.MockLLMClient{})

	rec := &recordingPublisher{}
	ctrl.SetPublisher(rec)

	res, err := ctrl.RunStepAnalysis(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rec.steps) != 1 {
		t.Fatalf("expected 1 step event published, got %d", len(rec.steps))
	}
	got := rec.steps[0]
	if got.Service != cfg.ServiceName || got.Namespace != cfg.Namespace {
		t.Fatalf("unexpected service/namespace on published step event: %+v", got)
	}
	if got.Weight != 10 {
		t.Fatalf("expected weight 10, got %d", got.Weight)
	}
	if got.Verdict != string(res.Evaluation.Verdict) {
		t.Fatalf("expected verdict %s, got %s", res.Evaluation.Verdict, got.Verdict)
	}
	if len(got.GateResults) != len(res.Evaluation.GateResults) {
		t.Fatalf("expected %d gate results, got %d", len(res.Evaluation.GateResults), len(got.GateResults))
	}
	if len(rec.rollbacks) != 0 {
		t.Fatalf("expected no rollback event on a healthy pass, got %d", len(rec.rollbacks))
	}
}

func TestControllerPublishesRollbackEventOnBreach(t *testing.T) {
	cfg := config.DefaultConfig()
	mockMetrics := &client.MockMetricsProvider{
		CanarySnapshot: &client.MetricSnapshot{
			TotalRequests:    5000,
			ErrorCount5xx:    250,
			ErrorRatePercent: 5.0, // 5.0% > 0.5% max, forces a breach
			P99LatencyMs:     800.0,
			AvgCpuPercent:    95.0,
			AvgMemoryPercent: 92.0,
		},
	}
	ctrl := NewController(cfg, &client.MockK8sClient{}, mockMetrics, &triage.MockLLMClient{})

	rec := &recordingPublisher{}
	ctrl.SetPublisher(rec)

	res, err := ctrl.RunStepAnalysis(context.Background(), 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.RollbackTriggered {
		t.Fatal("expected rollback to be triggered by this test's fixture")
	}

	if len(rec.steps) != 1 {
		t.Fatalf("expected 1 step event published, got %d", len(rec.steps))
	}
	if rec.steps[0].Verdict != string(res.Evaluation.Verdict) {
		t.Fatalf("step event verdict mismatch: got %s, want %s", rec.steps[0].Verdict, res.Evaluation.Verdict)
	}

	if len(rec.rollbacks) != 1 {
		t.Fatalf("expected 1 rollback event published, got %d", len(rec.rollbacks))
	}
	rb := rec.rollbacks[0]
	if rb.Service != cfg.ServiceName || rb.Namespace != cfg.Namespace {
		t.Fatalf("unexpected service/namespace on rollback event: %+v", rb)
	}
	if rb.Weight != 20 {
		t.Fatalf("expected rollback weight 20, got %d", rb.Weight)
	}
	if rb.Reason == "" {
		t.Fatal("expected rollback reason to be populated from the evaluation summary")
	}
}

func TestSetPublisherNilFallsBackToNoop(t *testing.T) {
	cfg := config.DefaultConfig()
	ctrl := NewController(cfg, &client.MockK8sClient{}, &client.MockMetricsProvider{}, &triage.MockLLMClient{})

	ctrl.SetPublisher(nil)

	if _, ok := ctrl.publisher.(notify.NoopPublisher); !ok {
		t.Fatalf("expected SetPublisher(nil) to fall back to notify.NoopPublisher, got %T", ctrl.publisher)
	}
}
