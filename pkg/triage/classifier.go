package triage

import (
	"regexp"
	"strings"

	"github.com/gerardrecinto/rollout-sentinel/pkg/client"
)

type FailureCategory string

const (
	CategoryOOMKilled             FailureCategory = "OOM_KILLED"
	CategoryCrashLoopBackOff      FailureCategory = "CRASH_LOOP_BACKOFF"
	CategoryImagePullFailure      FailureCategory = "IMAGE_PULL_FAILURE"
	CategoryProbeFailure          FailureCategory = "PROBE_FAILURE"
	CategoryNetworkConnectionRef  FailureCategory = "NETWORK_CONNECTION_REFUSED"
	CategoryConfigSecretMissing   FailureCategory = "CONFIG_SECRET_MISSING"
	CategoryDatabaseConnectionErr FailureCategory = "DATABASE_CONNECTION_ERROR"
	CategoryUnknown               FailureCategory = "UNKNOWN_ANOMALY"
)

type TriageResult struct {
	Category            FailureCategory `json:"category"`
	ConfidenceScore     float64         `json:"confidence_score"`
	RootCauseSummary    string          `json:"root_cause_summary"`
	SuggestedRemediation string         `json:"suggested_remediation"`
	TriggeringEvidence  []string        `json:"triggering_evidence"`
	AffectsPod          string          `json:"affects_pod"`
}

var (
	reOOM        = regexp.MustCompile(`(?i)(oomkilled|out of memory|exit code 137|cgroup memory limit)`)
	reCrashLoop  = regexp.MustCompile(`(?i)(crashloopbackoff|panic:|runtime error:|fatal error:)`)
	reImagePull  = regexp.MustCompile(`(?i)(imagepullbackoff|errimagepull|failed to pull image|unauthorized)`)
	reProbe      = regexp.MustCompile(`(?i)(liveness probe failed|readiness probe failed|probe timeout)`)
	reNetwork    = regexp.MustCompile(`(?i)(connection refused|dial tcp|no route to host|i/o timeout|dns resolution)`)
	reSecret     = regexp.MustCompile(`(?i)(secret ".*" not found|configmap ".*" not found|key not found)`)
	reDatabase   = regexp.MustCompile(`(?i)(pq: password authentication failed|database connection closed|connection reset by peer|too many connections)`)
)

// ClassifyPodFailure runs heuristic regex classification across pod logs, status reasons, and events.
func ClassifyPodFailure(pod client.PodInfo) *TriageResult {
	evidence := make([]string, 0)

	// Combine status reason, message, and logs
	haystack := pod.Reason + " " + pod.Message
	for _, l := range pod.RecentLogs {
		haystack += " " + l
	}
	for _, e := range pod.Events {
		haystack += " " + e
	}

	if pod.ExitCode == 137 || reOOM.MatchString(haystack) {
		evidence = append(evidence, "Detected exit code 137 or Out-Of-Memory pattern in container runtime logs.")
		return &TriageResult{
			Category:            CategoryOOMKilled,
			ConfidenceScore:     0.98,
			RootCauseSummary:    "Container exceeded Kubernetes memory limit (cgroup OOMKill).",
			SuggestedRemediation: "Increase pod memory requests/limits in deployment manifest or optimize application heap allocation.",
			TriggeringEvidence:  evidence,
			AffectsPod:          pod.Name,
		}
	}

	if reImagePull.MatchString(haystack) {
		evidence = append(evidence, "Found ImagePullBackOff / authorization failure in pod lifecycle events.")
		return &TriageResult{
			Category:            CategoryImagePullFailure,
			ConfidenceScore:     0.95,
			RootCauseSummary:    "Kubernetes cannot pull the canary image tag or authentication with registry failed.",
			SuggestedRemediation: "Verify container registry URL, image tag tag existence in ECR/Harbor, and imagePullSecrets configuration.",
			TriggeringEvidence:  evidence,
			AffectsPod:          pod.Name,
		}
	}

	if reProbe.MatchString(haystack) {
		evidence = append(evidence, "Found readiness/liveness health check probe timeouts.")
		return &TriageResult{
			Category:            CategoryProbeFailure,
			ConfidenceScore:     0.90,
			RootCauseSummary:    "Readiness/Liveness probe failed to receive 200 OK within timeout threshold.",
			SuggestedRemediation: "Inspect /healthz endpoint latency, adjust initialDelaySeconds, or check for blocking initialization calls.",
			TriggeringEvidence:  evidence,
			AffectsPod:          pod.Name,
		}
	}

	if reSecret.MatchString(haystack) {
		evidence = append(evidence, "Missing Secret or ConfigMap reference detected in pod events.")
		return &TriageResult{
			Category:            CategoryConfigSecretMissing,
			ConfidenceScore:     0.92,
			RootCauseSummary:    "Pod failed to start because required Secret or ConfigMap volume mount is missing.",
			SuggestedRemediation: "Verify Kubernetes Secrets / AWS Secrets Manager CSI driver syncing before deployment.",
			TriggeringEvidence:  evidence,
			AffectsPod:          pod.Name,
		}
	}

	if reDatabase.MatchString(haystack) {
		evidence = append(evidence, "Database connection error strings detected in application standard error.")
		return &TriageResult{
			Category:            CategoryDatabaseConnectionErr,
			ConfidenceScore:     0.88,
			RootCauseSummary:    "Canary instance failed to establish active database pool connection.",
			SuggestedRemediation: "Check database connection pool limits, firewall rules, and credentials rotation state.",
			TriggeringEvidence:  evidence,
			AffectsPod:          pod.Name,
		}
	}

	if reNetwork.MatchString(haystack) {
		evidence = append(evidence, "Network dial/timeout error encountered during outbound service communication.")
		return &TriageResult{
			Category:            CategoryNetworkConnectionRef,
			ConfidenceScore:     0.85,
			RootCauseSummary:    "Downstream dependency connection refused or DNS resolution failed.",
			SuggestedRemediation: "Check CoreDNS health, network policies, and downstream microservice availability.",
			TriggeringEvidence:  evidence,
			AffectsPod:          pod.Name,
		}
	}

	if pod.RestartCount > 1 || reCrashLoop.MatchString(haystack) {
		evidence = append(evidence, "Multiple pod restarts or runtime panic traceback detected.")
		return &TriageResult{
			Category:            CategoryCrashLoopBackOff,
			ConfidenceScore:     0.85,
			RootCauseSummary:    "Application encountered unhandled exception / panic during startup.",
			SuggestedRemediation: "Inspect stack trace in pod stderr logs and fix initialization regression.",
			TriggeringEvidence:  evidence,
			AffectsPod:          pod.Name,
		}
	}

	return &TriageResult{
		Category:            CategoryUnknown,
		ConfidenceScore:     0.40,
		RootCauseSummary:    "Unclassified anomaly detected in canary rollout.",
		SuggestedRemediation: "Perform detailed log analysis or query LLM-assisted triage engine.",
		TriggeringEvidence:  []string{strings.TrimSpace(haystack)},
		AffectsPod:          pod.Name,
	}
}
