package analyzer

import (
	"context"
	"fmt"
	"strings"
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

type actionFailureClient struct {
	client.MockK8sClient
	nilStatus bool
	actionErr error
}

func (c *actionFailureClient) GetRolloutStatus(ctx context.Context, namespace, deployment string) (*client.RolloutStatus, error) {
	if c.nilStatus {
		return nil, nil
	}
	return c.MockK8sClient.GetRolloutStatus(ctx, namespace, deployment)
}

func (c *actionFailureClient) TriggerRollback(context.Context, string, string) error {
	return c.actionErr
}
func (c *actionFailureClient) ShiftTrafficWeight(context.Context, string, string, int) error {
	return c.actionErr
}

func TestControllerDoesNotReportFailedRollbackAsSuccessful(t *testing.T) {
	k8s := &actionFailureClient{actionErr: fmt.Errorf("permission denied")}
	metrics := &client.MockMetricsProvider{CanarySnapshot: &client.MetricSnapshot{ErrorRatePercent: 5}}
	res, err := NewController(config.DefaultConfig(), k8s, metrics, nil).RunStepAnalysis(context.Background(), 20)
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected rollback failure, got %v", err)
	}
	if res == nil || res.RollbackTriggered {
		t.Fatalf("failed rollback must not be reported as accepted: %+v", res)
	}
}

func TestControllerReturnsTrafficShiftFailure(t *testing.T) {
	k8s := &actionFailureClient{actionErr: fmt.Errorf("traffic API unavailable")}
	res, err := NewController(config.DefaultConfig(), k8s, &client.MockMetricsProvider{}, nil).RunStepAnalysis(context.Background(), 10)
	if err == nil || !strings.Contains(err.Error(), "traffic API unavailable") {
		t.Fatalf("expected traffic error, got %v", err)
	}
	if res == nil || strings.Contains(res.ActionTaken, "Advanced") {
		t.Fatalf("failed traffic shift reported success: %+v", res)
	}
}

func TestControllerRejectsMissingStatusBeforeAnyAction(t *testing.T) {
	for _, breach := range []bool{false, true} {
		metrics := &client.MockMetricsProvider{}
		if breach {
			metrics.CanarySnapshot = &client.MetricSnapshot{ErrorRatePercent: 5}
		}
		k8s := &actionFailureClient{nilStatus: true}
		res, err := NewController(config.DefaultConfig(), k8s, metrics, &triage.MockLLMClient{}).RunStepAnalysis(context.Background(), 10)
		if err == nil || res != nil || k8s.RollbackExecuted || k8s.CurrentWeight != 0 {
			t.Fatalf("missing status must fail without actions: result=%+v err=%v", res, err)
		}
	}
}

func TestControllerRejectsInvalidWeightWithoutChangingTraffic(t *testing.T) {
	for _, weight := range []int{-1, 101} {
		k8s := &client.MockK8sClient{}
		_, err := NewController(config.DefaultConfig(), k8s, &client.MockMetricsProvider{}, nil).RunStepAnalysis(context.Background(), weight)
		if err == nil || k8s.CurrentWeight != 0 || k8s.RollbackExecuted {
			t.Fatalf("invalid weight %d must not change traffic", weight)
		}
	}
}
