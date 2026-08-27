package analyzer

import (
	"context"
	"testing"
	"time"

	"github.com/gerardrecinto/rollout-sentinel/pkg/client"
	"github.com/gerardrecinto/rollout-sentinel/pkg/config"
	"github.com/gerardrecinto/rollout-sentinel/pkg/evaluator"
	"github.com/gerardrecinto/rollout-sentinel/pkg/triage"
)

func TestControllerHealthyAdvance(t *testing.T) {
	cfg := config.DefaultConfig()
	mockMetrics := &client.MockMetricsProvider{}
	mockK8s := &client.MockK8sClient{}
	mockLLM := &triage.MockLLMClient{}

	ctrl := NewController(cfg, mockK8s, mockMetrics, mockLLM)
	res, err := ctrl.RunStepAnalysis(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Evaluation.Verdict != evaluator.VerdictPass {
		t.Fatalf("expected VerdictPass, got %s", res.Evaluation.Verdict)
	}
	if mockK8s.CurrentWeight != 20 {
		t.Fatalf("expected next weight 20, got %d", mockK8s.CurrentWeight)
	}
	if res.RollbackTriggered {
		t.Fatal("expected no rollback")
	}
}

func TestControllerBreachTriggersRollbackAndLLMTriage(t *testing.T) {
	cfg := config.DefaultConfig()
	mockMetrics := &client.MockMetricsProvider{
		CanarySnapshot: &client.MetricSnapshot{
			Timestamp:        time.Now(),
			TotalRequests:    5000,
			ErrorCount5xx:    250,
			ErrorRatePercent: 5.0, // 5.0% > 0.5% max
			P99LatencyMs:     800.0,
			AvgCpuPercent:    95.0,
			AvgMemoryPercent: 92.0,
		},
	}
	mockK8s := &client.MockK8sClient{
		Status: &client.RolloutStatus{
			DeploymentName: "retail-checkout-api",
			UnhealthyPods: []client.PodInfo{
				{
					Name:       "retail-checkout-api-canary-01",
					ExitCode:   137,
					Reason:     "OOMKilled",
					RecentLogs: []string{"Out of memory in worker thread"},
				},
			},
		},
	}
	mockLLM := &triage.MockLLMClient{}

	ctrl := NewController(cfg, mockK8s, mockMetrics, mockLLM)
	res, err := ctrl.RunStepAnalysis(context.Background(), 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.RollbackTriggered {
		t.Fatal("expected rollback to be triggered")
	}
	if !mockK8s.RollbackExecuted {
		t.Fatal("expected k8s rollback action to execute")
	}
	if len(res.UnhealthyPodTriages) != 1 {
		t.Fatalf("expected 1 pod triage result, got %d", len(res.UnhealthyPodTriages))
	}
	if res.UnhealthyPodTriages[0].Category != triage.CategoryOOMKilled {
		t.Fatalf("expected OOMKilled category, got %s", res.UnhealthyPodTriages[0].Category)
	}
	if res.LLMAnalysis == nil {
		t.Fatal("expected LLM incident synthesis to be present")
	}
	if !res.LLMAnalysis.RollbackRecommended {
		t.Fatal("expected LLM analysis to recommend rollback")
	}
}
