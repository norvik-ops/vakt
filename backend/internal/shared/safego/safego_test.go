package safego

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRun_RunsFunctionAsynchronously prueft, dass Run die uebergebene
// Funktion in einer Goroutine ausfuehrt und sofort zurueckkehrt.
func TestRun_RunsFunctionAsynchronously(t *testing.T) {
	var ran atomic.Bool
	done := make(chan struct{})
	Run(context.Background(), "test_basic", func(_ context.Context) error {
		ran.Store(true)
		close(done)
		return nil
	})

	select {
	case <-done:
		if !ran.Load() {
			t.Fatal("function did not execute")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("function did not execute within 2s")
	}
}

// TestRun_RecoversFromPanic ist der ADR-0018-Kerntest: eine Panic darf den
// Prozess NICHT mitreissen — Run muss sie fangen, loggen und die Goroutine
// kontrolliert beenden lassen.
func TestRun_RecoversFromPanic(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	// Wir nutzen runOnce direkt, damit der Test synchron ist und
	// Race-Detection deterministisch.
	go func() {
		defer wg.Done()
		runOnce(context.Background(), "test_panic", func(_ context.Context) error {
			panic("boom — should be caught by safego")
		})
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// expected — runOnce kehrte zurueck statt den Test zu killen
	case <-time.After(2 * time.Second):
		t.Fatal("runOnce did not recover from panic within 2s")
	}
}

// TestRun_PropagatesContextCancellation zeigt, dass der Parent-Context
// inner-fn sichtbar bleibt — wenn der Aufrufer cancelt, kann fn das sehen.
func TestRun_PropagatesContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gotCancellation := make(chan struct{})
	Run(ctx, "test_ctx_propagation", func(c context.Context) error {
		<-c.Done()
		close(gotCancellation)
		return c.Err()
	})

	cancel()

	select {
	case <-gotCancellation:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine did not see parent-context cancellation")
	}
}

// TestRun_LogsErrorWithoutPanic dokumentiert: zurueckgegebene Fehler sind
// kein Anlass fuer Panic-Recovery, sondern werden als Warn ge-loggt. fn
// kehrt regulaer zurueck.
func TestRun_LogsErrorWithoutPanic(t *testing.T) {
	done := make(chan struct{})
	go func() {
		runOnce(context.Background(), "test_err_return", func(_ context.Context) error {
			return errors.New("regular error, not a panic")
		})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runOnce did not return for regular error within 2s")
	}
}

// TestRun_NilFunctionIsNoop verhindert, dass ein versehentlicher nil-fn
// zu einer NilPointerPanic im Helper selbst fuehrt.
func TestRun_NilFunctionIsNoop(t *testing.T) {
	// kein Panic erwartet
	Run(context.Background(), "test_nil", nil)
	time.Sleep(50 * time.Millisecond)
}
