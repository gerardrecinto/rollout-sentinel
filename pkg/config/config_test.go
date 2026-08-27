package config

import (
	"testing"
)

func TestDefaultConfigValid(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected default config to be valid, got: %v", err)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*CanaryConfig)
		wantErr bool
	}{
		{
			name: "missing service name",
			mutate: func(c *CanaryConfig) {
				c.ServiceName = ""
			},
			wantErr: true,
		},
		{
			name: "missing namespace",
			mutate: func(c *CanaryConfig) {
				c.Namespace = ""
			},
			wantErr: true,
		},
		{
			name: "invalid step weight",
			mutate: func(c *CanaryConfig) {
				c.StepWeightPercent = 150
			},
			wantErr: true,
		},
		{
			name: "negative error rate",
			mutate: func(c *CanaryConfig) {
				c.MaxErrorRatePercent = -1.0
			},
			wantErr: true,
		},
		{
			name: "zero p99 latency threshold",
			mutate: func(c *CanaryConfig) {
				c.MaxP99LatencyMs = 0
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
