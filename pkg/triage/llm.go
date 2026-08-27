package triage

import (
	"context"
	"fmt"
	"strings"

	"github.com/gerardrecinto/rollout-sentinel/pkg/client"
)

// LLMTriageRequest encapsulates full runtime context for automated LLM investigation.
type LLMTriageRequest struct {
	ServiceName     string                 `json:"service_name"`
	Namespace       string                 `json:"namespace"`
	CanaryVersion   string                 `json:"canary_version"`
	BaselineVersion string                 `json:"baseline_version"`
	CanaryMetrics   *client.MetricSnapshot `json:"canary_metrics"`
	BaselineMetrics *client.MetricSnapshot `json:"baseline_metrics"`
	UnhealthyPods   []client.PodInfo       `json:"unhealthy_pods"`
	RecentEvents    []string               `json:"recent_events"`
}

// LLMTriageResponse represents structured root-cause insights and remediation steps.
type LLMTriageResponse struct {
	IncidentTitle       string   `json:"incident_title"`
	ExecutiveSummary    string   `json:"executive_summary"`
	RootCauseHypothesis string   `json:"root_cause_hypothesis"`
	ConfidenceScore     float64  `json:"confidence_score"`
	ImmediateActions    []string `json:"immediate_actions"`
	LongTermFixes       []string `json:"long_term_fixes"`
	RollbackRecommended bool     `json:"rollback_recommended"`
}

// LLMClient defines the interface for calling AI triage models (Claude / OpenAI / local endpoint).
type LLMClient interface {
	AnalyzeIncident(ctx context.Context, req LLMTriageRequest) (*LLMTriageResponse, error)
}

// BuildPrompt constructs a high-density, grounded prompt for the LLM.
func BuildPrompt(req LLMTriageRequest) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("You are a senior site reliability and release engineer triaging a progressive canary deployment anomaly for service '%s' in namespace '%s'.\n\n", req.ServiceName, req.Namespace))
	sb.WriteString(fmt.Sprintf("CANARY VERSION: %s | BASELINE: %s\n\n", req.CanaryVersion, req.BaselineVersion))

	if req.CanaryMetrics != nil {
		sb.WriteString("CANARY METRICS:\n")
		sb.WriteString(fmt.Sprintf("- 5xx Error Rate: %.3f%%\n", req.CanaryMetrics.ErrorRatePercent))
		sb.WriteString(fmt.Sprintf("- P99 Latency: %.1fms\n", req.CanaryMetrics.P99LatencyMs))
		sb.WriteString(fmt.Sprintf("- CPU: %.1f%% | Memory: %.1f%%\n\n", req.CanaryMetrics.AvgCpuPercent, req.CanaryMetrics.AvgMemoryPercent))
	}

	if len(req.UnhealthyPods) > 0 {
		sb.WriteString("UNHEALTHY PODS DIAGNOSTICS:\n")
		for _, pod := range req.UnhealthyPods {
			sb.WriteString(fmt.Sprintf("Pod: %s (Phase: %s, Restarts: %d, ExitCode: %d, Reason: %s)\n", pod.Name, pod.Phase, pod.RestartCount, pod.ExitCode, pod.Reason))
			if len(pod.RecentLogs) > 0 {
				sb.WriteString("Recent Logs:\n")
				for _, l := range pod.RecentLogs {
					sb.WriteString(fmt.Sprintf("  %s\n", l))
				}
			}
		}
		sb.WriteString("\n")
	}

	if len(req.RecentEvents) > 0 {
		sb.WriteString("KUBERNETES EVENTS:\n")
		for _, ev := range req.RecentEvents {
			sb.WriteString(fmt.Sprintf("- %s\n", ev))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Analyze the above telemetry and return a structured assessment with root cause hypothesis, immediate mitigation steps, and whether rollback is recommended.")
	return sb.String()
}

// MockLLMClient provides a deterministic test double for AI triage.
type MockLLMClient struct{}

func (m *MockLLMClient) AnalyzeIncident(ctx context.Context, req LLMTriageRequest) (*LLMTriageResponse, error) {
	if len(req.UnhealthyPods) > 0 && req.UnhealthyPods[0].ExitCode == 137 {
		return &LLMTriageResponse{
			IncidentTitle:       "Canary Pod Memory Exhaustion (OOMKilled)",
			ExecutiveSummary:    "Canary pods failed liveness check due to container cgroup memory limits being exceeded under retail peak load.",
			RootCauseHypothesis: "New canary build v2.14.0 introduced unbuffered in-memory JSON payload caching for catalog queries.",
			ConfidenceScore:     0.96,
			ImmediateActions: []string{
				"Rollback traffic weight from canary to baseline immediately",
				"Increase pod memory limit from 512Mi to 1Gi as temporary mitigation",
			},
			LongTermFixes: []string{
				"Refactor JSON serialization to stream via io.Reader/io.Writer",
				"Add memory profiling benchmark to CI regression pipeline",
			},
			RollbackRecommended: true,
		}, nil
	}

	return &LLMTriageResponse{
		IncidentTitle:       "Canary Performance Anomaly Detected",
		ExecutiveSummary:    "Telemetry shows elevated error rates and latency degradation exceeding service SLOs.",
		RootCauseHypothesis: "Downstream microservice connection timeout or connection pool exhaustion under load.",
		ConfidenceScore:     0.88,
		ImmediateActions: []string{
			"Halt progressive canary traffic step increments",
			"Check downstream database pool metrics and network latency",
		},
		LongTermFixes: []string{
			"Tune HTTP keep-alive and connection pool max-idle connections",
			"Implement circuit breaker pattern for downstream calls",
		},
		RollbackRecommended: true,
	}, nil
}
