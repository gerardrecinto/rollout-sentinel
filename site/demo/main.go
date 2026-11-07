//go:build js && wasm

package main

import (
	"context"
	"encoding/json"
	"syscall/js"
	"time"

	"github.com/gerardrecinto/rollout-sentinel/pkg/analyzer"
	"github.com/gerardrecinto/rollout-sentinel/pkg/client"
	"github.com/gerardrecinto/rollout-sentinel/pkg/config"
	"github.com/gerardrecinto/rollout-sentinel/pkg/triage"
)

type stepIn struct {
	Weight    int                    `json:"weight"`
	Cfg       config.CanaryConfig    `json:"cfg"`
	Canary    *client.MetricSnapshot `json:"canary"`
	Baseline  *client.MetricSnapshot `json:"baseline"`
	Unhealthy []client.PodInfo       `json:"unhealthy"`
}

func fail(err error) any {
	b, _ := json.Marshal(map[string]string{"error": err.Error()})
	return string(b)
}

func step(this js.Value, args []js.Value) any {
	var in stepIn
	if err := json.Unmarshal([]byte(args[0].String()), &in); err != nil {
		return fail(err)
	}
	cfg := in.Cfg
	cfg.EnableLLMTriage = false
	if in.Canary != nil {
		in.Canary.Timestamp = time.Now()
	}
	if in.Baseline != nil {
		in.Baseline.Timestamp = time.Now()
	}
	k8s := &client.MockK8sClient{}
	if len(in.Unhealthy) > 0 {
		k8s.Status = &client.RolloutStatus{DeploymentName: cfg.ServiceName, UnhealthyPods: in.Unhealthy}
	}
	m := &client.MockMetricsProvider{CanarySnapshot: in.Canary, BaselineSnapshot: in.Baseline}
	ctrl := analyzer.NewController(cfg, k8s, m, nil)
	res, err := ctrl.RunStepAnalysis(context.Background(), in.Weight)
	if err != nil {
		return fail(err)
	}
	b, _ := json.Marshal(res)
	return string(b)
}

func classify(this js.Value, args []js.Value) any {
	var pod client.PodInfo
	if err := json.Unmarshal([]byte(args[0].String()), &pod); err != nil {
		return fail(err)
	}
	b, _ := json.Marshal(triage.ClassifyPodFailure(pod))
	return string(b)
}

func main() {
	js.Global().Set("sentinelStep", js.FuncOf(step))
	js.Global().Set("sentinelClassify", js.FuncOf(classify))
	js.Global().Set("__SENT_READY__", true)
	select {}
}
