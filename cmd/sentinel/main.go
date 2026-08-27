package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gerardrecinto/rollout-sentinel/pkg/analyzer"
	"github.com/gerardrecinto/rollout-sentinel/pkg/client"
	"github.com/gerardrecinto/rollout-sentinel/pkg/config"
	"github.com/gerardrecinto/rollout-sentinel/pkg/triage"
)

const Version = "v1.4.2"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "version":
		fmt.Printf("rollout-sentinel %s (darwin/arm64, Go 1.23+)\n", Version)

	case "check":
		runCheck(os.Args[2:])

	case "monitor":
		runMonitor(os.Args[2:])

	case "triage":
		runTriage(os.Args[2:])

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("rollout-sentinel - Cloud-Native Progressive Delivery & Canary Health Sentinel")
	fmt.Println("\nUsage:")
	fmt.Println("  rollout-sentinel <command> [flags]")
	fmt.Println("\nAvailable Commands:")
	fmt.Println("  check     Evaluate current canary deployment SLO gates (one-shot)")
	fmt.Println("  monitor   Continuous progressive canary weight progression & gate verification")
	fmt.Println("  triage    Run rule-based and AI-assisted root-cause diagnosis on unhealthy pods")
	fmt.Println("  version   Print rollout-sentinel version")
}

func runCheck(args []string) {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	service := fs.String("service", "retail-checkout-api", "Target Kubernetes service name")
	namespace := fs.String("namespace", "production", "Target Kubernetes namespace")
	weight := fs.Int("weight", 10, "Current canary traffic weight percent")
	jsonOutput := fs.Bool("json", false, "Output results in JSON format")
	_ = fs.Parse(args)

	cfg := config.DefaultConfig()
	cfg.ServiceName = *service
	cfg.Namespace = *namespace

	k8sMock := &client.MockK8sClient{}
	metricsMock := &client.MockMetricsProvider{}
	llmMock := &triage.MockLLMClient{}

	ctrl := analyzer.NewController(cfg, k8sMock, metricsMock, llmMock)
	res, err := ctrl.RunStepAnalysis(context.Background(), *weight)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error during evaluation: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(data))
		return
	}

	printReport(res)
}

func runMonitor(args []string) {
	fs := flag.NewFlagSet("monitor", flag.ExitOnError)
	service := fs.String("service", "retail-checkout-api", "Target Kubernetes service name")
	namespace := fs.String("namespace", "production", "Target Kubernetes namespace")
	maxSteps := fs.Int("steps", 3, "Maximum canary progression steps to simulate")
	_ = fs.Parse(args)

	cfg := config.DefaultConfig()
	cfg.ServiceName = *service
	cfg.Namespace = *namespace

	k8sMock := &client.MockK8sClient{}
	metricsMock := &client.MockMetricsProvider{}
	llmMock := &triage.MockLLMClient{}

	ctrl := analyzer.NewController(cfg, k8sMock, metricsMock, llmMock)

	currentWeight := 10
	for step := 1; step <= *maxSteps; step++ {
		fmt.Printf("\n--- [STEP %d/%d] Evaluating canary at %d%% traffic weight ---\n", step, *maxSteps, currentWeight)
		res, err := ctrl.RunStepAnalysis(context.Background(), currentWeight)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Step failure: %v\n", err)
			break
		}
		printReport(res)

		if res.RollbackTriggered {
			fmt.Println("\n[ABORT] Automated rollback was triggered. Halting progressive deployment loop.")
			break
		}

		currentWeight += cfg.StepWeightPercent
		if currentWeight >= 100 {
			fmt.Println("\n[SUCCESS] Canary achieved 100% traffic weight with all SLO quality gates satisfied.")
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func runTriage(args []string) {
	fs := flag.NewFlagSet("triage", flag.ExitOnError)
	podName := fs.String("pod", "retail-checkout-api-canary-7f8d-x9", "Pod name to diagnose")
	reason := fs.String("reason", "OOMKilled", "Reported pod failure reason")
	exitCode := fs.Int("exit-code", 137, "Pod exit code")
	_ = fs.Parse(args)

	pod := client.PodInfo{
		Name:         *podName,
		Namespace:    "production",
		Phase:        "Failed",
		ExitCode:     *exitCode,
		Reason:       *reason,
		RecentLogs:   []string{"2026-08-27T07:22:10Z [ERROR] Heap allocation failed (512MB limit exceeded)"},
		Events:       []string{"Killing container with id containerd://retail-api: cgroup memory limit exceeded"},
		RestartCount: 3,
	}

	fmt.Println("================================================================================")
	fmt.Println("                  ROLLOUT-SENTINEL: POD FAILURE TRIAGE ENGINE                   ")
	fmt.Println("================================================================================")
	fmt.Printf("Target Pod:      %s\n", pod.Name)
	fmt.Printf("Failure Reason:  %s (Exit Code: %d)\n\n", pod.Reason, pod.ExitCode)

	res := triage.ClassifyPodFailure(pod)
	fmt.Printf("[1] DETERMINISTIC RULE CLASSIFIER:\n")
	fmt.Printf("    Category:       %s\n", res.Category)
	fmt.Printf("    Confidence:     %.1f%%\n", res.ConfidenceScore*100)
	fmt.Printf("    Root Cause:     %s\n", res.RootCauseSummary)
	fmt.Printf("    Remediation:    %s\n\n", res.SuggestedRemediation)

	llmMock := &triage.MockLLMClient{}
	llmResp, _ := llmMock.AnalyzeIncident(context.Background(), triage.LLMTriageRequest{
		ServiceName:     "retail-checkout-api",
		Namespace:       "production",
		CanaryVersion:   "v2.14.0",
		BaselineVersion: "v2.13.9",
		UnhealthyPods:   []client.PodInfo{pod},
	})

	if llmResp != nil {
		fmt.Printf("[2] AI / LLM ROOT-CAUSE SYNTHESIS:\n")
		fmt.Printf("    Title:          %s\n", llmResp.IncidentTitle)
		fmt.Printf("    Summary:        %s\n", llmResp.ExecutiveSummary)
		fmt.Printf("    Hypothesis:     %s\n", llmResp.RootCauseHypothesis)
		fmt.Printf("    Rollback Recom: %v\n", llmResp.RollbackRecommended)
		fmt.Println("    Immediate Actions:")
		for _, a := range llmResp.ImmediateActions {
			fmt.Printf("      - %s\n", a)
		}
	}
	fmt.Println("================================================================================")
}

func printReport(res *analyzer.AnalysisResult) {
	fmt.Printf("Status Verdict:  [%s]\n", res.Evaluation.Verdict)
	fmt.Printf("Action:          %s\n", res.ActionTaken)
	fmt.Printf("Summary:         %s\n", res.Evaluation.Summary)
	fmt.Printf("Quality Gates (%d evaluated):\n", len(res.Evaluation.GateResults))
	for _, g := range res.Evaluation.GateResults {
		status := "PASS"
		if !g.Passed {
			status = "FAIL"
		}
		fmt.Printf("  [%s] %-28s -> %s\n", status, g.GateName, g.Details)
	}
}
