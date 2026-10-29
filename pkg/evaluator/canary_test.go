package evaluator

import (
	"math"
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

func TestEvaluateCanaryRejectsMalformedMetricValues(t *testing.T) {
	mutations := []struct {
		name   string
		change func(*client.MetricSnapshot)
	}{
		{"negative requests", func(s *client.MetricSnapshot) { s.TotalRequests = -1 }},
		{"negative errors", func(s *client.MetricSnapshot) { s.ErrorCount5xx = -1 }},
		{"negative error rate", func(s *client.MetricSnapshot) { s.ErrorRatePercent = -1 }},
		{"negative p99", func(s *client.MetricSnapshot) { s.P99LatencyMs = -1 }},
		{"negative CPU", func(s *client.MetricSnapshot) { s.AvgCpuPercent = -1 }},
		{"negative memory", func(s *client.MetricSnapshot) { s.AvgMemoryPercent = -1 }},
		{"non-finite latency", func(s *client.MetricSnapshot) { s.P99LatencyMs = math.Inf(1) }},
		{"not-a-number CPU", func(s *client.MetricSnapshot) { s.AvgCpuPercent = math.NaN() }},
	}
	for _, tt := range mutations {
		for _, target := range []string{"canary", "baseline"} {
			t.Run(tt.name+"/"+target, func(t *testing.T) {
				canary := &client.MetricSnapshot{TotalRequests: 1000, P99LatencyMs: 100}
				baseline := &client.MetricSnapshot{TotalRequests: 1000, P99LatencyMs: 100}
				if target == "canary" {
					tt.change(canary)
				} else {
					tt.change(baseline)
				}
				res := EvaluateCanary(config.DefaultConfig(), canary, baseline, 10)
				if res.Verdict != VerdictAbort {
					t.Fatalf("invalid %s metrics must abort, got %s", target, res.Verdict)
				}
			})
		}
	}
}
