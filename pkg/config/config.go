package config

import (
	"errors"
	"fmt"
	"time"
)

// CanaryConfig defines thresholds and parameters for progressive canary deployment verification.
type CanaryConfig struct {
	ServiceName           string        `json:"service_name"`
	Namespace             string        `json:"namespace"`
	CanaryVersion         string        `json:"canary_version"`
	BaselineVersion       string        `json:"baseline_version"`
	StepWeightPercent     int           `json:"step_weight_percent"`
	EvaluationInterval    time.Duration `json:"evaluation_interval"`
	MaxErrorRatePercent   float64       `json:"max_error_rate_percent"`
	MaxP99LatencyMs       float64       `json:"max_p99_latency_ms"`
	MaxCpuSaturationPct   float64       `json:"max_cpu_saturation_pct"`
	MaxMemorySaturationPct float64      `json:"max_memory_saturation_pct"`
	MaxConsecutiveFailures int          `json:"max_consecutive_failures"`
	PrometheusURL         string        `json:"prometheus_url"`
	EnableLLMTriage       bool          `json:"enable_llm_triage"`
}

// DefaultConfig returns a production-hardened default configuration for high-availability services.
func DefaultConfig() CanaryConfig {
	return CanaryConfig{
		ServiceName:            "retail-checkout-api",
		Namespace:              "production",
		CanaryVersion:          "v2.14.0",
		BaselineVersion:        "v2.13.9",
		StepWeightPercent:      10,
		EvaluationInterval:     15 * time.Second,
		MaxErrorRatePercent:    0.5,   // 0.5% 5xx error budget
		MaxP99LatencyMs:        450.0, // 450ms p99 latency SLA
		MaxCpuSaturationPct:    85.0,  // 85% CPU limit
		MaxMemorySaturationPct: 90.0,  // 90% memory threshold
		MaxConsecutiveFailures: 3,
		PrometheusURL:          "http://prometheus-k8s.monitoring.svc:9090",
		EnableLLMTriage:        true,
	}
}

// Validate checks configuration sanity and returns meaningful errors.
func (c *CanaryConfig) Validate() error {
	if c.ServiceName == "" {
		return errors.New("service_name is required")
	}
	if c.Namespace == "" {
		return errors.New("namespace is required")
	}
	if c.StepWeightPercent <= 0 || c.StepWeightPercent > 100 {
		return fmt.Errorf("step_weight_percent must be between 1 and 100, got %d", c.StepWeightPercent)
	}
	if c.MaxErrorRatePercent < 0 || c.MaxErrorRatePercent > 100 {
		return fmt.Errorf("max_error_rate_percent must be between 0 and 100, got %.2f", c.MaxErrorRatePercent)
	}
	if c.MaxP99LatencyMs <= 0 {
		return fmt.Errorf("max_p99_latency_ms must be positive, got %.2f", c.MaxP99LatencyMs)
	}
	if c.MaxConsecutiveFailures <= 0 {
		return fmt.Errorf("max_consecutive_failures must be >= 1, got %d", c.MaxConsecutiveFailures)
	}
	return nil
}
