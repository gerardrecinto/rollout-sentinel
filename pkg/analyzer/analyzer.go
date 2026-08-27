package analyzer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gerardrecinto/rollout-sentinel/pkg/client"
	"github.com/gerardrecinto/rollout-sentinel/pkg/config"
	"github.com/gerardrecinto/rollout-sentinel/pkg/evaluator"
	"github.com/gerardrecinto/rollout-sentinel/pkg/triage"
)

// AnalysisResult holds the consolidated outcome of a rollout health inspection cycle.
type AnalysisResult struct {
	Timestamp          time.Time                    `json:"timestamp"`
	ServiceName        string                       `json:"service_name"`
	Namespace          string                       `json:"namespace"`
	Evaluation         *evaluator.CanaryEvaluation  `json:"evaluation"`
	UnhealthyPodTriages []*triage.TriageResult      `json:"unhealthy_pod_triages"`
	LLMAnalysis        *triage.LLMTriageResponse    `json:"llm_analysis,omitempty"`
	ActionTaken        string                       `json:"action_taken"`
	RollbackTriggered  bool                         `json:"rollback_triggered"`
}

// Controller orchestrates the canary health lifecycle.
type Controller struct {
	cfg        config.CanaryConfig
	k8sClient  client.K8sClient
	metrics    client.MetricsProvider
	llmClient  triage.LLMClient
}

// NewController creates a new Controller instance.
func NewController(cfg config.CanaryConfig, k8s client.K8sClient, metrics client.MetricsProvider, llm triage.LLMClient) *Controller {
	return &Controller{
		cfg:       cfg,
		k8sClient: k8s,
		metrics:   metrics,
		llmClient: llm,
	}
}

// RunStepAnalysis executes one complete cycle of progressive rollout evaluation.
func (c *Controller) RunStepAnalysis(ctx context.Context, currentWeight int) (*AnalysisResult, error) {
	result := &AnalysisResult{
		Timestamp:          time.Now(),
		ServiceName:        c.cfg.ServiceName,
		Namespace:          c.cfg.Namespace,
		UnhealthyPodTriages: make([]*triage.TriageResult, 0),
	}

	// 1. Fetch metrics concurrently using goroutines
	var canarySnapshot, baselineSnapshot *client.MetricSnapshot
	var metricsErr error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		s, err := c.metrics.GetCanaryMetrics(ctx, c.cfg.ServiceName, c.cfg.Namespace, 5*time.Minute)
		if err != nil {
			metricsErr = fmt.Errorf("failed fetching canary metrics: %w", err)
			return
		}
		canarySnapshot = s
	}()

	go func() {
		defer wg.Done()
		s, err := c.metrics.GetBaselineMetrics(ctx, c.cfg.ServiceName, c.cfg.Namespace, 5*time.Minute)
		if err != nil {
			// Baseline failure is non-fatal; we proceed with absolute thresholds
			return
		}
		baselineSnapshot = s
	}()

	wg.Wait()

	if metricsErr != nil {
		return nil, metricsErr
	}

	// 2. Fetch Kubernetes pod health and events
	rolloutStatus, err := c.k8sClient.GetRolloutStatus(ctx, c.cfg.Namespace, c.cfg.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed fetching k8s rollout status: %w", err)
	}

	// 3. Evaluate SLO quality gates
	eval := evaluator.EvaluateCanary(c.cfg, canarySnapshot, baselineSnapshot, currentWeight)
	result.Evaluation = eval

	// 4. Inspect unhealthy pods if any
	if rolloutStatus != nil && len(rolloutStatus.UnhealthyPods) > 0 {
		eval.Verdict = evaluator.VerdictBreach
		for _, pod := range rolloutStatus.UnhealthyPods {
			t := triage.ClassifyPodFailure(pod)
			result.UnhealthyPodTriages = append(result.UnhealthyPodTriages, t)
		}
	}

	// 5. Decision & Remediation logic
	switch eval.Verdict {
	case evaluator.VerdictPass:
		nextWeight := currentWeight + c.cfg.StepWeightPercent
		if nextWeight > 100 {
			nextWeight = 100
		}
		if err := c.k8sClient.ShiftTrafficWeight(ctx, c.cfg.Namespace, c.cfg.ServiceName, nextWeight); err == nil {
			result.ActionTaken = fmt.Sprintf("SLO gates healthy. Advanced canary traffic weight to %d%%.", nextWeight)
		} else {
			result.ActionTaken = fmt.Sprintf("SLO gates healthy, but traffic shift failed: %v", err)
		}

	case evaluator.VerdictWarn:
		result.ActionTaken = fmt.Sprintf("Warning threshold reached. Holding traffic weight at %d%% for stabilization.", currentWeight)

	case evaluator.VerdictBreach, evaluator.VerdictAbort:
		result.ActionTaken = "SLO breach detected. Executing immediate zero-downtime rollback to baseline."
		_ = c.k8sClient.TriggerRollback(ctx, c.cfg.Namespace, c.cfg.ServiceName)
		result.RollbackTriggered = true

		// Run AI / LLM root cause synthesis
		if c.cfg.EnableLLMTriage && c.llmClient != nil {
			events, _ := c.k8sClient.GetRecentEvents(ctx, c.cfg.Namespace, c.cfg.ServiceName)
			llmReq := triage.LLMTriageRequest{
				ServiceName:     c.cfg.ServiceName,
				Namespace:       c.cfg.Namespace,
				CanaryVersion:   c.cfg.CanaryVersion,
				BaselineVersion: c.cfg.BaselineVersion,
				CanaryMetrics:   canarySnapshot,
				BaselineMetrics: baselineSnapshot,
				UnhealthyPods:   rolloutStatus.UnhealthyPods,
				RecentEvents:    events,
			}
			llmResp, llmErr := c.llmClient.AnalyzeIncident(ctx, llmReq)
			if llmErr == nil {
				result.LLMAnalysis = llmResp
			}
		}
	}

	return result, nil
}
