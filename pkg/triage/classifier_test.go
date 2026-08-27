package triage

import (
	"context"
	"testing"

	"github.com/gerardrecinto/rollout-sentinel/pkg/client"
)

func TestClassifyOOMKilled(t *testing.T) {
	pod := client.PodInfo{
		Name:      "cart-api-canary-01",
		ExitCode:  137,
		Reason:    "OOMKilled",
		Message:   "Container terminated with error code 137",
		RecentLogs: []string{"fatal: out of memory allocating 512MB buffer"},
	}

	res := ClassifyPodFailure(pod)
	if res.Category != CategoryOOMKilled {
		t.Fatalf("expected CategoryOOMKilled, got %s", res.Category)
	}
	if res.ConfidenceScore < 0.90 {
		t.Fatalf("expected high confidence score, got %.2f", res.ConfidenceScore)
	}
}

func TestClassifyImagePullFailure(t *testing.T) {
	pod := client.PodInfo{
		Name:      "cart-api-canary-02",
		Phase:     "Pending",
		Reason:    "ImagePullBackOff",
		Events:    []string{"Failed to pull image 'docker.apple.com/retail/cart:v2.14.0': unauthorized"},
	}

	res := ClassifyPodFailure(pod)
	if res.Category != CategoryImagePullFailure {
		t.Fatalf("expected CategoryImagePullFailure, got %s", res.Category)
	}
}

func TestClassifyProbeFailure(t *testing.T) {
	pod := client.PodInfo{
		Name:      "cart-api-canary-03",
		Phase:     "Running",
		Events:    []string{"Liveness probe failed: HTTP probe failed with statuscode: 500"},
	}

	res := ClassifyPodFailure(pod)
	if res.Category != CategoryProbeFailure {
		t.Fatalf("expected CategoryProbeFailure, got %s", res.Category)
	}
}

func TestLLMTriage(t *testing.T) {
	clientMock := &MockLLMClient{}
	req := LLMTriageRequest{
		ServiceName:     "checkout-svc",
		Namespace:       "prod",
		CanaryVersion:   "v2.14.0",
		BaselineVersion: "v2.13.9",
		UnhealthyPods: []client.PodInfo{
			{Name: "checkout-canary-1", ExitCode: 137, Reason: "OOMKilled"},
		},
	}

	prompt := BuildPrompt(req)
	if len(prompt) == 0 {
		t.Fatal("expected non-empty prompt")
	}

	resp, err := clientMock.AnalyzeIncident(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.RollbackRecommended {
		t.Fatal("expected rollback to be recommended for OOM failure")
	}
}
