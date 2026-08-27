package evaluator

import (
	"testing"
	"time"

	"github.com/gerardrecinto/rollout-sentinel/pkg/client"
	"github.com/gerardrecinto/rollout-sentinel/pkg/config"
)

func TestEvaluateCanaryHealthy(t *testing.T) {
	cfg := config.DefaultConfig()
	canary := &client.MetricSnapshot{
		Timestamp:        time.Now(),
		TotalRequests:    10000,
		ErrorCount5xx:    5,
		ErrorRatePercent: 0.05,
		P99LatencyMs:     210.0,
		AvgCpuPercent:    50.0,
		AvgMemoryPercent: 60.0,
	}
	baseline := &client.MetricSnapshot{
		Timestamp:        time.Now(),
		TotalRequests:    100000,
		ErrorCount5xx:    50,
		ErrorRatePercent: 0.05,
		P99LatencyMs:     200.0,
		AvgCpuPercent:    48.0,
		AvgMemoryPercent: 58.0,
	}

	res := EvaluateCanary(cfg, canary, baseline, 10)
	if res.Verdict != VerdictPass {
		t.Fatalf("expected VerdictPass, got %s", res.Verdict)
	}
}

func TestEvaluateCanaryErrorRateBreach(t *testing.T) {
	cfg := config.DefaultConfig() // Max allowed: 0.5%
	canary := &client.MetricSnapshot{
		Timestamp:        time.Now(),
		TotalRequests:    10000,
		ErrorCount5xx:    150,
		ErrorRatePercent: 1.5, // 1.5% > 0.5%
		P99LatencyMs:     200.0,
		AvgCpuPercent:    50.0,
		AvgMemoryPercent: 60.0,
	}
	baseline := &client.MetricSnapshot{
		Timestamp:        time.Now(),
		TotalRequests:    100000,
		ErrorCount5xx:    50,
		ErrorRatePercent: 0.05,
		P99LatencyMs:     200.0,
		AvgCpuPercent:    48.0,
		AvgMemoryPercent: 58.0,
	}

	res := EvaluateCanary(cfg, canary, baseline, 20)
	if res.Verdict != VerdictBreach {
		t.Fatalf("expected VerdictBreach, got %s", res.Verdict)
	}
}

func TestEvaluateCanaryLatencyDriftWarn(t *testing.T) {
	cfg := config.DefaultConfig()
	canary := &client.MetricSnapshot{
		Timestamp:        time.Now(),
		TotalRequests:    10000,
		ErrorCount5xx:    5,
		ErrorRatePercent: 0.05,
		P99LatencyMs:     300.0, // 50% slower than 200ms baseline, but within 450ms max SLA
		AvgCpuPercent:    50.0,
		AvgMemoryPercent: 60.0,
	}
	baseline := &client.MetricSnapshot{
		Timestamp:        time.Now(),
		TotalRequests:    100000,
		ErrorCount5xx:    50,
		ErrorRatePercent: 0.05,
		P99LatencyMs:     200.0,
		AvgCpuPercent:    48.0,
		AvgMemoryPercent: 58.0,
	}

	res := EvaluateCanary(cfg, canary, baseline, 10)
	if res.Verdict != VerdictWarn {
		t.Fatalf("expected VerdictWarn due to relative drift, got %s", res.Verdict)
	}
}
