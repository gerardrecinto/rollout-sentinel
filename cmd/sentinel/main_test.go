package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestCLIProcess(t *testing.T) {
	if os.Getenv("SENTINEL_CLI_TEST") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"rollout-sentinel"}, os.Args[i+1:]...)
			break
		}
	}
	main()
	os.Exit(0)
}

func runCLI(args ...string) (string, string, error) {
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=TestCLIProcess", "--"}, args...)...)
	cmd.Env = append(os.Environ(), "SENTINEL_CLI_TEST=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func TestCommandsRequireExplicitDemo(t *testing.T) {
	for _, command := range []string{"check", "monitor", "triage"} {
		t.Run(command, func(t *testing.T) {
			stdout, stderr, err := runCLI(command)
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 2 {
				t.Fatalf("expected exit 2, got %v", err)
			}
			if stdout != "" || !strings.Contains(stderr, "no live backend") {
				t.Fatalf("ambiguous output: stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}

func TestDemoCheckKeepsJSONMachineReadable(t *testing.T) {
	stdout, stderr, err := runCLI("check", "--demo", "--json")
	if err != nil {
		t.Fatalf("demo check failed: %v: %s", err, stderr)
	}
	var result struct {
		Evaluation struct {
			Verdict string `json:"verdict"`
		} `json:"evaluation"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout must remain JSON: %v: %s", err, stdout)
	}
	if result.Evaluation.Verdict != "PASS" || !strings.Contains(stderr, "Simulation only") {
		t.Fatalf("missing demo result or warning: %s %s", stdout, stderr)
	}
}

func TestVersionReportsActualBuildPlatform(t *testing.T) {
	stdout, _, err := runCLI("version")
	if err != nil || !strings.Contains(stdout, runtime.GOOS+"/"+runtime.GOARCH) {
		t.Fatalf("incorrect platform: %q, %v", stdout, err)
	}
}
