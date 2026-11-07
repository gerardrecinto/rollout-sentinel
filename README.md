# rollout-sentinel

[![CI](https://github.com/gerardrecinto/rollout-sentinel/actions/workflows/ci.yml/badge.svg)](https://github.com/gerardrecinto/rollout-sentinel/actions/workflows/ci.yml)

A Go simulator for canary release decisions and pod failure triage. It is for platform and release engineers who want to inspect and test the decision logic before building a live integration.

**The CLI uses synthetic metrics, a mock Kubernetes client and canned incident synthesis. It does not query Prometheus, change real traffic or roll back a real deployment.**

[Try the browser demo](https://gerardrecinto.github.io/rollout-sentinel/) or run the same evaluator locally.

## Quick start

Needs Git and Go 1.23 or later. These commands do not need a cluster, cloud account, model key or credentials.

```bash
git clone https://github.com/gerardrecinto/rollout-sentinel.git
cd rollout-sentinel
go build -ldflags="-s -w" -o bin/rollout-sentinel ./cmd/sentinel
./bin/rollout-sentinel check --demo --json
./bin/rollout-sentinel monitor --demo --steps 2
./bin/rollout-sentinel triage --demo --pod example-pod --reason OOMKilled --exit-code 137
```

The healthy sample produces a `PASS` verdict and a simulated traffic advance. Simulation warnings go to stderr, so `check --demo --json` keeps stdout valid JSON. Commands that would otherwise run mock actions require `--demo`; without it, they exit 2 instead of acting as a fake production health gate.

`triage --demo` uses sample logs and events. Its incident synthesis is a fixed test response, not a diagnosis from a connected model. Rule confidence values are heuristic constants, not calibrated probabilities.

## What is implemented

| Component | Current behavior |
| --- | --- |
| Go evaluator | Checks absolute and relative 5xx error rates, p99 latency, latency drift, CPU and memory against configured thresholds. |
| Controller | Fetches snapshots concurrently. Requests a traffic advance on PASS, holds on WARN, and requests rollback on BREACH or ABORT through a client interface. Action errors are returned, not reported as successful changes. |
| Failure classifier | Rules for memory failures, image pulls, probes, missing configuration, database connections, network failures and repeated crashes. |
| Browser demo | Runs the Go evaluator, controller and classifier as WebAssembly. Its inputs and cluster actions are simulated. |
| Container and release workflow | Builds a non-root Linux container and four OS/architecture binaries. The image release workflow signs and verifies its published digest using keyless Cosign. |
| Deployment manifests | Reference Argo CD, Argo Rollouts and Kyverno configuration. These are not a tested production deployment. |

Relative comparisons depend on available baseline data. CPU saturation and latency drift can produce WARN, which holds traffic. Critical error or memory breaches request rollback on that evaluation; a consecutive-failure timer is not implemented. The CLI monitor is a short demonstration loop, not a background controller.

## What is not connected

- No live Prometheus or other metrics client.
- No live Kubernetes client or traffic router.
- No external incident-analysis model client.
- No proven rollback completion time, production service-level agreement or measured fleet throughput.
- `EvaluationInterval` and `MaxConsecutiveFailures` are configuration fields, not an implemented scheduling or consecutive-failure policy.

The interfaces in `pkg/client/` and `pkg/triage/` are extension points. Implementing an interface is only one part of a live integration; authentication, authorization, missing data, timeouts and deployment verification still need testing.

## Tests

```bash
go test -race -coverprofile=coverage.txt ./...
go vet ./...
```

CI builds the binary, runs race tests and vet on Linux and macOS, verifies the demo guard and JSON output, and builds the container. Tests include rejected action requests and missing rollout status. The repository does not currently enforce reproducible latency or RSS benchmark thresholds.

## Installation alternatives

The quick start builds the source you checked out. Prebuilt archives and checksums are listed on [Releases](https://github.com/gerardrecinto/rollout-sentinel/releases). Select the archive for the actual release tag, operating system and architecture. Compare its checksum with that release's `checksums.txt` before using it. No privileged global installation is required to run a downloaded binary from its extraction directory.

## Deployment references

`deploy/` contains Argo CD, Argo Rollouts and Kyverno examples. **Do not apply them as a live canary health gate.** The CLI has no live backend and refuses checks without explicit `--demo`. Adding that flag to a production gate would evaluate synthetic healthy data rather than your service.

The Kyverno policy describes keyless signature admission for this repository's release workflow. Whether it is installed and enforced on a cluster must be verified separately. A signed image is not evidence that its metrics or rollback behavior are production-ready.

## Security and license

Report vulnerabilities privately using [SECURITY.md](SECURITY.md). Avoid including credentials or private workload logs in public reports.

License terms have not been specified in a `LICENSE` file.

## Optional evaluation help

For a scoped paid evaluation or implementation inquiry, see [SUPPORT.md](SUPPORT.md). The CLI currently uses simulated data and mock cluster actions.
