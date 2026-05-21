// Package safego implementiert die Goroutine-Lifecycle- und Panic-Eskalations-
// Regel aus ADR-0018.
//
// Jede Goroutine in backend/internal/ läuft über Run(ctx, name, fn):
//   - Sie erbt einen Parent-Context (kein context.Background() in den Args).
//   - Sie hat einen Namen, der bei Panic im Log und im OTel-Span auftaucht.
//   - Sie kapselt defer-recover mit Stack-Capture und eskaliert nach
//     zerolog + OpenTelemetry-Error-Span. Sentry-Compat kommt in Sprint 15
//     (S15-14) hinzu, ohne dass dieser Helper API-mäßig geändert werden muss.
//
// Direkte `go func() { ... }()`-Aufrufe in internal/ sind ab ADR-0018
// Accepted verboten (Ausnahme: Test-Code und dieses Paket selbst). Ein
// golangci-lint-forbidigo-Eintrag blockt neue Verstöße.
package safego

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync/atomic"

	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// panicHandler ist der optionale Sentry/3rd-party-Hook (S15-14). Wenn der
// Aufrufer (cmd/api/main.go, cmd/worker/main.go) einen Sentry-Init macht, ruft
// er SetPanicHandler mit einer Funktion auf, die das Sentry-Event verschickt.
// safego.Run feuert dann den Hook zusätzlich zu zerolog + OTel. Default: nil.
//
// atomic.Pointer hält den Hook thread-safe; SetPanicHandler darf jederzeit
// laufen, auch nach dem Start (z.B. wenn DSN aus Live-Config kommt).
var panicHandler atomic.Pointer[func(err error, goroutineName string, stack []byte)]

// SetPanicHandler installiert den Sentry-/3rd-party-Panic-Hook. Aufruf mit nil
// entfernt einen vorher gesetzten Hook.
func SetPanicHandler(fn func(err error, goroutineName string, stack []byte)) {
	if fn == nil {
		panicHandler.Store(nil)
		return
	}
	panicHandler.Store(&fn)
}

// Run startet fn als Goroutine.
//
// Pflichten:
//   - ctx ist der Parent-Context (Request-, Worker-Job-, Server-Lifecycle-).
//     Nutze niemals context.Background() für Aufrufer außerhalb cmd/*.
//   - name ist ein stabiler Identifier (z.B. "evidence_auto.collect",
//     "demo.cleanup.tick"). Er erscheint in Logs und Tracing als Filter-Key.
//   - fn ist der eigentliche Workload. Wenn fn einen Fehler zurückgibt,
//     wird er ge-loggt (Warn), aber NICHT als Panic propagiert.
//
// Eine Panic in fn wird gefangen, mit Stacktrace ge-loggt (Error) und am
// aktiven OpenTelemetry-Span (falls vorhanden) als Error markiert. Das ist
// die Eskalations-Schiene laut ADR-0018; die Goroutine stirbt kontrolliert,
// der restliche Prozess läuft weiter.
//
// Bei nil-fn macht Run nichts (defensiv).
func Run(ctx context.Context, name string, fn func(context.Context) error) {
	if fn == nil {
		return
	}
	go runOnce(ctx, name, fn)
}

// runOnce ist die innere Routine — separat, damit Tests sie synchron
// aufrufen können ohne die Goroutine-Race-Lottery.
func runOnce(ctx context.Context, name string, fn func(context.Context) error) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			err := fmt.Errorf("panic in safego goroutine %q: %v", name, r)
			log.Error().
				Str("goroutine", name).
				Bytes("stack", stack).
				Interface("panic", r).
				Msg("safego: recovered from panic — goroutine terminated")
			if span := trace.SpanFromContext(ctx); span.IsRecording() {
				span.RecordError(err, trace.WithStackTrace(true))
				span.SetStatus(codes.Error, fmt.Sprintf("panic in %s", name))
			}
			// S15-14: Sentry-/3rd-party-Hook, falls registriert.
			if hook := panicHandler.Load(); hook != nil {
				(*hook)(err, name, stack)
			}
		}
	}()
	if err := fn(ctx); err != nil {
		// Reguläre Errors sind kein Anlass für Stack-Capture; nur Warn-Log.
		// Wer Errors als Panic eskaliert haben will, soll innerhalb von fn
		// panic(err) aufrufen.
		log.Warn().
			Err(err).
			Str("goroutine", name).
			Msg("safego: goroutine exited with error")
	}
}
