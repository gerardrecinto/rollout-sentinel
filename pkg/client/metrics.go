package client

import (
	"context"
	"time"
)

// MetricSnapshot captures point-in-time telemetry for comparison.
type MetricSnapshot struct {
	Timestamp          time.Time `json:"timestamp"`
	TotalRequests      int64     `json:"total_requests"`
	ErrorCount5xx      int64     `json:"error_count_5xx"`
	ErrorRatePercent   float64   `json:"error_rate_percent"`
	P50LatencyMs       float64   `json:"p50_latency_ms"`
	P95LatencyMs       float64   `json:"p95_latency_ms"`
	P99LatencyMs       float64   `json:"p99_latency_ms"`
	AvgCpuPercent      float64   `json:"avg_cpu_percent"`
	AvgMemoryPercent   float64   `json:"avg_memory_percent"`
}

// MetricsProvider defines an interface for fetching telemetry from Prometheus or Datadog.
type MetricsProvider interface {
	GetCanaryMetrics(ctx context.Context, service, namespace string, window time.Duration) (*MetricSnapshot, error)
	GetBaselineMetrics(ctx context.Context, service, namespace string, window time.Duration) (*MetricSnapshot, error)
}

// MockMetricsProvider simulates realistic metrics for testing and demonstrations.
type MockMetricsProvider struct {
	CanarySnapshot   *MetricSnapshot
	BaselineSnapshot *MetricSnapshot
	Err              error
}

func (m *MockMetricsProvider) GetCanaryMetrics(ctx context.Context, service, namespace string, window time.Duration) (*MetricSnapshot, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.CanarySnapshot != nil {
		return m.CanarySnapshot, nil
	}
	return &MetricSnapshot{
		Timestamp:        time.Now(),
		TotalRequests:    12500,
		ErrorCount5xx:    12,
		ErrorRatePercent: 0.096,
		P50LatencyMs:     42.0,
		P95LatencyMs:     120.0,
		P99LatencyMs:     245.0,
		AvgCpuPercent:    44.2,
		AvgMemoryPercent: 58.1,
	}, nil
}

func (m *MockMetricsProvider) GetBaselineMetrics(ctx context.Context, service, namespace string, window time.Duration) (*MetricSnapshot, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.BaselineSnapshot != nil {
		return m.BaselineSnapshot, nil
	}
	return &MetricSnapshot{
		Timestamp:        time.Now(),
		TotalRequests:    110000,
		ErrorCount5xx:    88,
		ErrorRatePercent: 0.08,
		P50LatencyMs:     39.5,
		P95LatencyMs:     115.0,
		P99LatencyMs:     230.0,
		AvgCpuPercent:    42.0,
		AvgMemoryPercent: 57.0,
	}, nil
}
