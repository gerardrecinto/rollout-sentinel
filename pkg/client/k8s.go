package client

import (
	"context"
	"fmt"
)

// PodInfo holds metadata and runtime status for a Kubernetes pod.
type PodInfo struct {
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	Phase        string   `json:"phase"`
	Ready        bool     `json:"ready"`
	RestartCount int32    `json:"restart_count"`
	RecentLogs   []string `json:"recent_logs"`
	Events       []string `json:"events"`
	ExitCode     int      `json:"exit_code,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	Message      string   `json:"message,omitempty"`
}

// RolloutStatus represents the state of a progressive deployment.
type RolloutStatus struct {
	DeploymentName   string    `json:"deployment_name"`
	TargetReplicas   int32     `json:"target_replicas"`
	UpdatedReplicas  int32     `json:"updated_replicas"`
	ReadyReplicas    int32     `json:"ready_replicas"`
	AvailablePods    []PodInfo `json:"available_pods"`
	UnhealthyPods    []PodInfo `json:"unhealthy_pods"`
}

// K8sClient defines the interface for interacting with the Kubernetes API.
type K8sClient interface {
	GetRolloutStatus(ctx context.Context, namespace, deployment string) (*RolloutStatus, error)
	GetPodLogs(ctx context.Context, namespace, podName string, tailLines int64) ([]string, error)
	GetRecentEvents(ctx context.Context, namespace, deployment string) ([]string, error)
	TriggerRollback(ctx context.Context, namespace, deployment string) error
	ShiftTrafficWeight(ctx context.Context, namespace, service string, weightPercent int) error
}

// MockK8sClient provides a test and demonstration implementation of K8sClient.
type MockK8sClient struct {
	Status           *RolloutStatus
	RollbackExecuted bool
	CurrentWeight    int
	Err              error
}

func (m *MockK8sClient) GetRolloutStatus(ctx context.Context, namespace, deployment string) (*RolloutStatus, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Status != nil {
		return m.Status, nil
	}
	return &RolloutStatus{
		DeploymentName:  deployment,
		TargetReplicas:  10,
		UpdatedReplicas: 2,
		ReadyReplicas:   2,
		AvailablePods: []PodInfo{
			{Name: deployment + "-canary-7f8d9b-a1", Namespace: namespace, Phase: "Running", Ready: true, RestartCount: 0},
			{Name: deployment + "-canary-7f8d9b-a2", Namespace: namespace, Phase: "Running", Ready: true, RestartCount: 0},
		},
		UnhealthyPods: nil,
	}, nil
}

func (m *MockK8sClient) GetPodLogs(ctx context.Context, namespace, podName string, tailLines int64) ([]string, error) {
	return []string{
		`2026-08-27T07:15:01Z [INFO] Server started on port :8080`,
		`2026-08-27T07:15:05Z [INFO] Handled GET /healthz 200 OK (2ms)`,
		`2026-08-27T07:15:10Z [INFO] Processing checkout order_id=ord_984129`,
	}, nil
}

func (m *MockK8sClient) GetRecentEvents(ctx context.Context, namespace, deployment string) ([]string, error) {
	return []string{
		fmt.Sprintf("ScalingReplicaSet: Scaled up replica set %s-canary to 2", deployment),
	}, nil
}

func (m *MockK8sClient) TriggerRollback(ctx context.Context, namespace, deployment string) error {
	m.RollbackExecuted = true
	return nil
}

func (m *MockK8sClient) ShiftTrafficWeight(ctx context.Context, namespace, service string, weightPercent int) error {
	m.CurrentWeight = weightPercent
	return nil
}
