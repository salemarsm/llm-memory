package memory

import (
	"context"
	"testing"
	"time"
)

func TestSignals(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	t.Run("create and get", func(t *testing.T) {
		sig, err := store.CreateSignal(ctx, AgentSignal{
			Project:    "test-project",
			Kind:       SignalKindNotice,
			OwnerAgent: "claude-code",
			Payload:    "refactoring auth package",
		})
		if err != nil {
			t.Fatal(err)
		}
		if sig.ID == "" {
			t.Fatal("expected non-empty ID")
		}
		if sig.Status != SignalStatusActive {
			t.Fatalf("expected active, got %s", sig.Status)
		}

		got, err := store.GetSignal(ctx, sig.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Payload != sig.Payload {
			t.Fatalf("payload mismatch: %q vs %q", got.Payload, sig.Payload)
		}
	})

	t.Run("lease requires expires_at", func(t *testing.T) {
		_, err := store.CreateSignal(ctx, AgentSignal{
			Project:    "test-project",
			Kind:       SignalKindLease,
			OwnerAgent: "claude-code",
			Payload:    "locking auth/middleware.go",
		})
		if err == nil {
			t.Fatal("expected error for lease without expires_at")
		}
	})

	t.Run("lease with expires_at succeeds", func(t *testing.T) {
		exp := time.Now().Add(5 * time.Minute).UTC()
		sig, err := store.CreateSignal(ctx, AgentSignal{
			Project:    "test-project",
			Kind:       SignalKindLease,
			OwnerAgent: "claude-code",
			Payload:    "locking auth/middleware.go",
			ExpiresAt:  &exp,
		})
		if err != nil {
			t.Fatal(err)
		}
		if sig.ExpiresAt == nil {
			t.Fatal("expected ExpiresAt to be set")
		}
	})

	t.Run("list active signals", func(t *testing.T) {
		sigs, err := store.ListSignals(ctx, SignalQuery{Project: "test-project"})
		if err != nil {
			t.Fatal(err)
		}
		if len(sigs) < 2 {
			t.Fatalf("expected at least 2 signals, got %d", len(sigs))
		}
		for _, s := range sigs {
			if s.Status != SignalStatusActive {
				t.Fatalf("list returned non-active signal: %s", s.Status)
			}
		}
	})

	t.Run("acknowledge transitions status", func(t *testing.T) {
		sig, _ := store.CreateSignal(ctx, AgentSignal{
			Project:    "test-project",
			Kind:       SignalKindBlocker,
			OwnerAgent: "codex",
			Payload:    "waiting for review",
		})
		updated, err := store.UpdateSignalStatus(ctx, sig.ID, SignalStatusAcknowledged)
		if err != nil {
			t.Fatal(err)
		}
		if updated.Status != SignalStatusAcknowledged {
			t.Fatalf("expected acknowledged, got %s", updated.Status)
		}
	})

	t.Run("resolve sets resolved_at", func(t *testing.T) {
		sig, _ := store.CreateSignal(ctx, AgentSignal{
			Project:    "test-project",
			Kind:       SignalKindHandoff,
			OwnerAgent: "claude-code",
			Payload:    "stopped at step 3",
		})
		updated, err := store.UpdateSignalStatus(ctx, sig.ID, SignalStatusResolved)
		if err != nil {
			t.Fatal(err)
		}
		if updated.ResolvedAt == nil {
			t.Fatal("expected ResolvedAt to be set on resolve")
		}
	})

	t.Run("expire stale leases", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour).UTC()
		sig, _ := store.CreateSignal(ctx, AgentSignal{
			Project:    "test-project",
			Kind:       SignalKindLease,
			OwnerAgent: "claude-code",
			Payload:    "stale lock",
			ExpiresAt:  &past,
		})

		n, err := store.ExpireStaleSignals(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if n < 1 {
			t.Fatalf("expected at least 1 expired signal, got %d", n)
		}

		got, _ := store.GetSignal(ctx, sig.ID)
		if got.Status != SignalStatusExpired {
			t.Fatalf("expected expired, got %s", got.Status)
		}
	})

	t.Run("not found returns ErrSignalNotFound", func(t *testing.T) {
		_, err := store.GetSignal(ctx, "sig_nonexistent")
		if err != ErrSignalNotFound {
			t.Fatalf("expected ErrSignalNotFound, got %v", err)
		}
	})
}
