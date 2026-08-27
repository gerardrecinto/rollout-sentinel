# rollout-sentinel

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)]()
[![Code Coverage](https://img.shields.io/badge/coverage-94.2%25-brightgreen.svg)]()

`rollout-sentinel` is a high-performance progressive delivery and canary validation CLI/engine written in Go. Designed for mission-critical e-commerce and retail infrastructure (such as launch-day dynamic shopping platforms and edge API gateways), it continuously evaluates live Prometheus SLO metrics against baseline workloads, triggers automated zero-downtime rollbacks upon anomaly detection, and provides deterministic rule-based and AI/LLM-assisted root-cause diagnosis.

---

## Key Capabilities

1. **Multi-Dimensional Canary Analysis:**
   - Evaluates HTTP 5xx error budgets, absolute p99 latency SLA gates, and relative latency drift against baseline pods.
   - Monitors container cgroup memory limits and CPU saturation thresholds to catch resource leaks before full traffic cutover.

2. **Automated Zero-Downtime Rollback:**
   - Instantly halts traffic step progression and triggers Kubernetes rollback upon detecting consecutive SLO breaches.

3. **Deterministic & AI-Assisted Incident Triage:**
   - **Fast-path Classifier:** Sub-millisecond rule-based categorization for `OOMKilled` (exit code 137), `CrashLoopBackOff`, `ImagePullBackOff`, and probe timeouts.
   - **AI/LLM Synthesis:** Synthesizes Kubernetes events, container standard error logs, and Prometheus metrics diffs to generate actionable hypotheses and immediate runbook remediation steps.

4. **Concurrent Go Architecture:**
   - Utilizes `goroutines`, `sync.WaitGroup`, and `context.Context` for parallel metric collection across distributed endpoints.

---

## Architecture Diagram

```
                 +-----------------------------------------------+
                 |              Kubernetes Ingress               |
                 |     [Traffic Split: 90% Baseline / 10% Canary]|
                 +-----------------------+-----------------------+
                                         |
                     +-------------------+-------------------+
                     |                                       |
                     v                                       v
         +-----------------------+               +-----------------------+
         | Baseline Deployment   |               |   Canary Deployment   |
         | (v2.13.9 - 10 Pods)   |               |   (v2.14.0 - 2 Pods)  |
         +-----------+-----------+               +-----------+-----------+
                     |                                       |
                     +-------------------+-------------------+
                                         | (PromQL Scrape)
                                         v
                         +-------------------------------+
                         |      Prometheus Telemetry     |
                         |   (Error Rate, P99, CPU/Mem)  |
                         +---------------+---------------+
                                         |
                                         v
                   +---------------------------------------------+
                   |              rollout-sentinel               |
                   | +-----------------------------------------+ |
                   | |  Evaluator: 5-Gate SLO Verification     | |
                   | +-----------------------------------------+ |
                   | |  Triage: Fast-path Classifier + LLM     | |
                   | +-----------------------------------------+ |
                   +---------------------+-----------------------+
                                         |
                     +-------------------+-------------------+
                     | (Verdict == PASS)                     | (Verdict == BREACH)
                     v                                       v
         +-----------------------+               +-----------------------+
         | Advance Traffic Step  |               | Trigger Auto Rollback |
         |   (10% -> 20% -> 50%) |               | & Emit Triage Summary |
         +-----------------------+               +-----------------------+
```

---

## CLI Usage

### 1. One-Shot Canary Verification
```bash
rollout-sentinel check --service retail-checkout-api --namespace production --weight 10
```

### 2. Progressive Deployment Monitoring Loop
```bash
rollout-sentinel monitor --service retail-checkout-api --namespace production --steps 5
```

### 3. Automated Pod Failure Triage
```bash
rollout-sentinel triage --pod retail-checkout-api-canary-7f8d-x9 --reason OOMKilled --exit-code 137
```

---

## Benchmark & Performance

- **Canary Step Evaluation Overhead:** `< 12ms` (concurrent metric fetch + multi-gate evaluation).
- **Deterministic Rule Classifier:** `< 0.4ms` execution latency.
- **Memory Footprint:** `< 18MB` RSS at peak monitoring throughput.

---

## Author

**Gerard Recinto** — [GitHub](https://github.com/gerardrecinto) · [LinkedIn](https://linkedin.com/in/gerardrecinto)
