package evaluator

import (
	"fmt"

	"github.com/gerardrecinto/rollout-sentinel/pkg/client"
	"github.com/gerardrecinto/rollout-sentinel/pkg/config"
)

type Verdict string

const (
	VerdictPass   Verdict = "PASS"
	VerdictWarn   Verdict = "WARN"
	VerdictBreach Verdict = "BREACH"
	VerdictAbort  Verdict = "ABORT"
)

type GateResult struct {
	GateName    string  `json:"gate_name"`
	Passed      bool    `json:"passed"`
	ActualValue float64 `json:"actual_value"`
	Threshold   float64 `json:"threshold"`
	Details     string  `json:"details"`
}

type CanaryEvaluation struct {
	Verdict        Verdict               `json:"verdict"`
	CanaryWeight   int                   `json:"canary_weight_percent"`
	Summary        string                `json:"summary"`
	GateResults    []GateResult          `json:"gate_results"`
	CanarySnapshot *client.MetricSnapshot `json:"canary_snapshot"`
	BaselineSnapshot *client.MetricSnapshot `json:"baseline_snapshot"`
}

// EvaluateCanary performs multi-dimensional SLO gate checks comparing canary to baseline.
func EvaluateCanary(cfg config.CanaryConfig, canary, baseline *client.MetricSnapshot, weight int) *CanaryEvaluation {
	eval := &CanaryEvaluation{
		Verdict:          VerdictPass,
		CanaryWeight:     weight,
		CanarySnapshot:   canary,
		BaselineSnapshot: baseline,
		GateResults:      make([]GateResult, 0),
	}

	if canary == nil {
		eval.Verdict = VerdictAbort
		eval.Summary = "Critical: Canary metric snapshot is missing"
		return eval
	}

	// 1. Error Rate Gate
	errGatePassed := canary.ErrorRatePercent <= cfg.MaxErrorRatePercent
	errDetails := fmt.Sprintf("Canary 5xx rate: %.3f%% (Max allowed: %.3f%%)", canary.ErrorRatePercent, cfg.MaxErrorRatePercent)
	if !errGatePassed {
		eval.Verdict = VerdictBreach
	}
	eval.GateResults = append(eval.GateResults, GateResult{
		GateName:    "ErrorRateGate",
		Passed:      errGatePassed,
		ActualValue: canary.ErrorRatePercent,
		Threshold:   cfg.MaxErrorRatePercent,
		Details:     errDetails,
	})

	// 2. Relative Error Rate Degradation (vs Baseline)
	if baseline != nil && baseline.TotalRequests > 100 && canary.TotalRequests > 100 {
		relErrorDiff := canary.ErrorRatePercent - baseline.ErrorRatePercent
		if relErrorDiff > (cfg.MaxErrorRatePercent * 0.75) {
			eval.Verdict = VerdictBreach
			eval.GateResults = append(eval.GateResults, GateResult{
				GateName:    "RelativeErrorDegradationGate",
				Passed:      false,
				ActualValue: relErrorDiff,
				Threshold:   cfg.MaxErrorRatePercent * 0.75,
				Details:     fmt.Sprintf("Canary error rate elevated by +%.3f%% over baseline (%.3f%% vs %.3f%%)", relErrorDiff, canary.ErrorRatePercent, baseline.ErrorRatePercent),
			})
		}
	}

	// 3. P99 Latency SLA Gate
	latGatePassed := canary.P99LatencyMs <= cfg.MaxP99LatencyMs
	latDetails := fmt.Sprintf("Canary p99 latency: %.1fms (Max SLA: %.1fms)", canary.P99LatencyMs, cfg.MaxP99LatencyMs)
	if !latGatePassed {
		eval.Verdict = VerdictBreach
	}
	eval.GateResults = append(eval.GateResults, GateResult{
		GateName:    "P99LatencyGate",
		Passed:      latGatePassed,
		ActualValue: canary.P99LatencyMs,
		Threshold:   cfg.MaxP99LatencyMs,
		Details:     latDetails,
	})

	// 4. Relative P99 Latency Drift (> 30% slower than baseline is a warning/breach)
	if baseline != nil && baseline.P99LatencyMs > 0 {
		latencyDriftPct := ((canary.P99LatencyMs - baseline.P99LatencyMs) / baseline.P99LatencyMs) * 100.0
		driftPassed := latencyDriftPct <= 30.0
		if !driftPassed && eval.Verdict == VerdictPass {
			eval.Verdict = VerdictWarn
		}
		eval.GateResults = append(eval.GateResults, GateResult{
			GateName:    "LatencyDriftGate",
			Passed:      driftPassed,
			ActualValue: latencyDriftPct,
			Threshold:   30.0,
			Details:     fmt.Sprintf("Canary latency drift: +%.1f%% compared to baseline (%.1fms vs %.1fms)", latencyDriftPct, canary.P99LatencyMs, baseline.P99LatencyMs),
		})
	}

	// 5. Resource Saturation Gates
	cpuPassed := canary.AvgCpuPercent <= cfg.MaxCpuSaturationPct
	if !cpuPassed && eval.Verdict == VerdictPass {
		eval.Verdict = VerdictWarn
	}
	eval.GateResults = append(eval.GateResults, GateResult{
		GateName:    "CpuSaturationGate",
		Passed:      cpuPassed,
		ActualValue: canary.AvgCpuPercent,
		Threshold:   cfg.MaxCpuSaturationPct,
		Details:     fmt.Sprintf("Canary CPU saturation: %.1f%% (Threshold: %.1f%%)", canary.AvgCpuPercent, cfg.MaxCpuSaturationPct),
	})

	memPassed := canary.AvgMemoryPercent <= cfg.MaxMemorySaturationPct
	if !memPassed && eval.Verdict != VerdictAbort {
		eval.Verdict = VerdictBreach
	}
	eval.GateResults = append(eval.GateResults, GateResult{
		GateName:    "MemorySaturationGate",
		Passed:      memPassed,
		ActualValue: canary.AvgMemoryPercent,
		Threshold:   cfg.MaxMemorySaturationPct,
		Details:     fmt.Sprintf("Canary memory saturation: %.1f%% (Threshold: %.1f%%)", canary.AvgMemoryPercent, cfg.MaxMemorySaturationPct),
	})

	// Summary creation
	if eval.Verdict == VerdictPass {
		eval.Summary = fmt.Sprintf("All %d SLO gates passed at %d%% traffic weight.", len(eval.GateResults), weight)
	} else if eval.Verdict == VerdictWarn {
		eval.Summary = fmt.Sprintf("Warning: Minor performance drift detected at %d%% traffic weight.", weight)
	} else {
		eval.Summary = fmt.Sprintf("SLO Breach: One or more critical quality gates failed at %d%% traffic weight.", weight)
	}

	return eval
}
