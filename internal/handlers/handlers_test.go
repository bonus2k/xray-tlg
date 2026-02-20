package handlers

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestShortenFileName(t *testing.T) {
	name := "very-very-long-config-file-name.json"
	short := shortenFileName(name)

	if len(short) >= len(name) {
		t.Fatalf("expected shortened name, got %q", short)
	}
}

func TestMakeCopyFileCallbackData(t *testing.T) {
	if got := makeCopyFileCallbackData("cfg.json"); got != "cp_cfg.json" {
		t.Fatalf("unexpected callback data: %s", got)
	}
}

func TestRoundDurationToSeconds(t *testing.T) {
	if got := roundDurationToSeconds(1500 * time.Millisecond); got != 2*time.Second {
		t.Fatalf("unexpected rounded duration: %s", got)
	}

	if got := roundDurationToSeconds(-time.Second); got != 0 {
		t.Fatalf("negative duration should return zero, got: %s", got)
	}
}

func TestAcquireCommandLock(t *testing.T) {
	h := &Handler{
		logger:      zap.NewNop(),
		lockTimeout: 90 * time.Second,
	}

	release, err := h.acquireCommandLock("speedtest")
	if err != nil {
		t.Fatalf("unexpected lock error: %v", err)
	}

	if _, err := h.acquireCommandLock("restart"); err == nil {
		t.Fatal("expected busy lock error")
	}

	release()

	if _, err := h.acquireCommandLock("restart"); err != nil {
		t.Fatalf("expected lock after release, got: %v", err)
	}
}
