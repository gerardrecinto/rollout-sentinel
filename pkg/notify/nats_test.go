package notify

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestNATSPublisherLive is a smoke test against a real NATS server. It's
// skipped by default so `go test ./...` never fails or hangs on a machine
// without NATS running. Point NATS_TEST_URL at a running server to exercise
// it, for example:
//
//	docker run -p 4222:4222 nats:latest
//	NATS_TEST_URL=nats://127.0.0.1:4222 go test ./pkg/notify/... -run TestNATSPublisherLive -v
func TestNATSPublisherLive(t *testing.T) {
	url := os.Getenv("NATS_TEST_URL")
	if url == "" {
		t.Skip("NATS_TEST_URL not set, skipping live NATS smoke test")
	}

	pub, err := NewNATSPublisher(url)
	if err != nil {
		t.Fatalf("connecting to nats at %s: %v", url, err)
	}
	defer pub.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	step := StepEvent{
		Timestamp: time.Now(),
		Service:   "notify-live-test",
		Namespace: "test",
		Weight:    10,
		Verdict:   "PASS",
		Summary:   "live smoke test step event",
	}
	if err := pub.PublishStep(ctx, step); err != nil {
		t.Fatalf("PublishStep: %v", err)
	}

	rollback := RollbackEvent{
		Timestamp: time.Now(),
		Service:   "notify-live-test",
		Namespace: "test",
		Weight:    20,
		Verdict:   "BREACH",
		Reason:    "live smoke test rollback event",
	}
	if err := pub.PublishRollback(ctx, rollback); err != nil {
		t.Fatalf("PublishRollback: %v", err)
	}
}
