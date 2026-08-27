#!/usr/bin/env python3
"""Claude-based pull request code review with auto-suggestions.

Reviews Go pull request diffs for concurrency safety (goroutines, data races,
unbuffered channels, mutex locks, context cancellations), error handling, and
Kubernetes/cloud-native best practices. Posts one consolidated PR review comment
with 1-click GitHub suggestion blocks.
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from typing import Literal

from anthropic import Anthropic
from pydantic import BaseModel, Field

MODEL = "claude-sonnet-5"
COMMENT_MARKER = "<!-- rollout-sentinel-claude-pr-review -->"

SYSTEM_PROMPT = """You are a senior Go and Kubernetes platform engineer reviewing a pull request diff \
for rollout-sentinel (a progressive delivery and canary SLO verification CLI in Go 1.23+).

Review the diff, not the whole repository. Only flag issues that the diff introduces or modifies.

Focus on:
1. Concurrency safety: goroutine leaks, data races, unbuffered channel deadlocks, context.Context propagation.
2. Error handling: explicit error checks, no ignored errors, proper error wrapping (%w).
3. Kubernetes & Prometheus reliability: cgroup boundaries, timeout handling, mutex locking.
4. Clean idiomatic Go code.

For each concrete violation, provide the exact file, line number, and a suggested_replacement \
if a direct 1-line or multi-line fix applies (so GitHub suggestion blocks can be generated)."""


class Violation(BaseModel):
    file: str
    line: int
    category: Literal[
        "concurrency-safety", "error-handling", "performance",
        "resource-leak", "code-quality", "other"
    ]
    severity: Literal["low", "medium", "high"]
    message: str
    suggested_replacement: str | None = Field(
        default=None,
        description="Exact replacement code for this line if a direct fix applies; null otherwise."
    )


class PRReview(BaseModel):
    quality_score: int = Field(ge=0, le=100, description="Overall code quality score (100 = flawless)")
    confidence: int = Field(ge=0, le=100)
    summary: str
    violations: list[Violation]


def run_review(diff_text: str) -> PRReview:
    client = Anthropic()
    response = client.messages.parse(
        model=MODEL,
        max_tokens=8000,
        system=[{"type": "text", "text": SYSTEM_PROMPT}],
        messages=[{"role": "user", "content": f"Review this Go PR diff:\n\n```diff\n{diff_text}\n```"}],
        output_format=PRReview,
    )
    return response.parsed_output


def render_comment(review: PRReview, pr_number: int, commit_sha: str) -> str:
    lines = [
        COMMENT_MARKER,
        "## Automated Code Review & Suggestions",
        f"**Quality Score:** `{review.quality_score}/100` | **Confidence:** `{review.confidence}%` | **Commit:** `{commit_sha[:7]}`",
        "",
        review.summary,
        "",
    ]

    if not review.violations:
        lines.append("No violations found. Clean pull request diff.")
        return "\n".join(lines)

    lines.append(f"### Findings ({len(review.violations)})")
    lines.append("")

    for v in review.violations:
        sev_label = f"[{v.severity.upper()}]"
        lines.append(f"#### `{v.file}:{v.line}` {sev_label} `{v.category}`")
        lines.append(f"{v.message}")
        lines.append("")
        if v.suggested_replacement:
            lines.append("```suggestion")
            lines.append(v.suggested_replacement)
            lines.append("```")
            lines.append("")

    return "\n".join(lines)


def post_to_github(body: str, pr_number: int, repo: str):
    gh_token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")
    if not gh_token:
        print("GH_TOKEN not set; skipping GitHub comment post.")
        return

    # Check for existing comment
    list_cmd = ["gh", "api", f"repos/{repo}/issues/{pr_number}/comments"]
    env = dict(os.environ, GH_TOKEN=gh_token)
    res = subprocess.run(list_cmd, capture_output=True, text=True, env=env)
    
    existing_comment_id = None
    if res.returncode == 0:
        try:
            comments = json.loads(res.stdout)
            for c in comments:
                if COMMENT_MARKER in c.get("body", ""):
                    existing_comment_id = c["id"]
                    break
        except Exception:
            pass

    if existing_comment_id:
        update_cmd = [
            "gh", "api", "-X", "PATCH",
            f"repos/{repo}/issues/comments/{existing_comment_id}",
            "-f", f"body={body}"
        ]
        subprocess.run(update_cmd, check=True, env=env)
        print(f"Updated PR comment #{existing_comment_id}")
    else:
        post_cmd = [
            "gh", "api", "-X", "POST",
            f"repos/{repo}/issues/{pr_number}/comments",
            "-f", f"body={body}"
        ]
        subprocess.run(post_cmd, check=True, env=env)
        print("Posted new PR comment.")


def main():
    parser = argparse.ArgumentParser(description="Run Claude PR review on diff")
    parser.add_argument("--diff-file", required=True, help="Path to git diff file")
    parser.add_argument("--pr-number", type=int, default=0, help="PR number")
    parser.add_argument("--repo", default="", help="GitHub owner/repo")
    parser.add_argument("--commit-sha", default="", help="Commit SHA")
    parser.add_argument("--output", default="", help="Output markdown path")
    parser.add_argument("--post", action="store_true", help="Post comment to GitHub PR")
    args = parser.parse_args()

    if not os.path.exists(args.diff_file):
        print(f"Diff file {args.diff_file} does not exist.")
        sys.exit(0)

    diff_text = open(args.diff_file, encoding="utf-8").read().strip()
    if not diff_text:
        print("Empty diff; nothing to review.")
        sys.exit(0)

    print(f"Analyzing diff ({len(diff_text.splitlines())} lines)...")
    review = run_review(diff_text)
    comment_body = render_comment(review, args.pr_number, args.commit_sha)

    if args.output:
        with open(args.output, "w", encoding="utf-8") as fp:
            fp.write(comment_body)
        print(f"Saved review to {args.output}")

    if args.post and args.repo and args.pr_number > 0:
        post_to_github(comment_body, args.pr_number, args.repo)


if __name__ == "__main__":
    main()
