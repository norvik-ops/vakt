// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S88-6: opt-in outbound forwarding of audit events to a customer-configured
// Syslog/SIEM sink (RFC 5424 or CEF over TCP/TLS). Datenschutz: the target is
// configured by the customer (analogous to outgoing webhooks / SMTP) — Vakt
// never sends audit data to Norvik. Default off: with no target configured no
// outbound traffic occurs. The audit write path is NEVER blocked — forwarding is
// asynchronous with a bounded buffer and a drop counter.
package audit

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/httputil"
)

// Forwarder receives audit entries for outbound delivery. Implementations must
// be non-blocking.
type Forwarder interface {
	Forward(e WriteEntry)
}

var globalForwarder atomic.Pointer[forwarderHolder]

type forwarderHolder struct{ f Forwarder }

// SetForwarder installs the process-wide audit forwarder (call once at startup).
// Passing nil disables forwarding.
func SetForwarder(f Forwarder) {
	if f == nil {
		globalForwarder.Store(nil)
		return
	}
	globalForwarder.Store(&forwarderHolder{f: f})
}

func currentForwarder() Forwarder {
	if h := globalForwarder.Load(); h != nil {
		return h.f
	}
	return nil
}

// ── Counters (exposed to the /metrics endpoint, no client library) ─────────

var (
	forwardSent    atomic.Uint64
	forwardDropped atomic.Uint64
	forwardFailed  atomic.Uint64
)

// ForwardStats returns the cumulative forwarding counters for the metrics
// endpoint: sent (delivered), dropped (buffer full), failed (sink error).
func ForwardStats() (sent, dropped, failed uint64) {
	return forwardSent.Load(), forwardDropped.Load(), forwardFailed.Load()
}

// SyslogForwarder delivers audit entries to a Syslog/SIEM sink. Delivery runs on
// a single worker goroutine fed by a bounded channel; when the channel is full
// the entry is dropped (counter incremented) so the audit write path never blocks.
type SyslogForwarder struct {
	target string // host:port
	useTLS bool
	format string // "rfc5424" | "cef"
	ch     chan WriteEntry
}

// SyslogForwarderConfig is read from the environment.
type SyslogForwarderConfig struct {
	Target       string // VAKT_AUDIT_SYSLOG_ADDR (host:port). Empty = disabled.
	Proto        string // VAKT_AUDIT_SYSLOG_PROTO: "tcp" | "tcp+tls"
	Format       string // VAKT_AUDIT_SYSLOG_FORMAT: "rfc5424" | "cef"
	AllowPrivate bool   // VAKT_AUDIT_SYSLOG_ALLOW_PRIVATE
	BufferSize   int
}

// SyslogConfigFromEnv reads the forwarder config from the environment.
func SyslogConfigFromEnv() SyslogForwarderConfig {
	return SyslogForwarderConfig{
		Target:       os.Getenv("VAKT_AUDIT_SYSLOG_ADDR"),
		Proto:        getEnvDefault("VAKT_AUDIT_SYSLOG_PROTO", "tcp"),
		Format:       getEnvDefault("VAKT_AUDIT_SYSLOG_FORMAT", "rfc5424"),
		AllowPrivate: os.Getenv("VAKT_AUDIT_SYSLOG_ALLOW_PRIVATE") == "true",
		BufferSize:   1024,
	}
}

func getEnvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// NewSyslogForwarder validates the config and starts the delivery worker.
// Returns (nil, nil) when no target is configured (forwarding disabled).
func NewSyslogForwarder(cfg SyslogForwarderConfig) (*SyslogForwarder, error) {
	if strings.TrimSpace(cfg.Target) == "" {
		return nil, nil // disabled — opt-in
	}
	if err := httputil.ValidateOutboundHostPort(cfg.Target, cfg.AllowPrivate); err != nil {
		return nil, fmt.Errorf("audit syslog target rejected: %w", err)
	}
	format := strings.ToLower(cfg.Format)
	if format != "rfc5424" && format != "cef" {
		format = "rfc5424"
	}
	bufSize := cfg.BufferSize
	if bufSize <= 0 {
		bufSize = 1024
	}
	f := &SyslogForwarder{
		target: cfg.Target,
		useTLS: strings.Contains(strings.ToLower(cfg.Proto), "tls"),
		format: format,
		ch:     make(chan WriteEntry, bufSize),
	}
	go f.worker()
	log.Info().Str("target", cfg.Target).Str("format", format).Bool("tls", f.useTLS).
		Msg("audit syslog forwarder enabled")
	return f, nil
}

// Forward enqueues an entry for delivery. Non-blocking: drops + counts when full.
func (f *SyslogForwarder) Forward(e WriteEntry) {
	select {
	case f.ch <- e:
	default:
		forwardDropped.Add(1)
	}
}

func (f *SyslogForwarder) worker() {
	for e := range f.ch {
		if err := f.deliver(e); err != nil {
			forwardFailed.Add(1)
			log.Warn().Err(err).Str("target", f.target).Msg("audit forward delivery failed")
			continue
		}
		forwardSent.Add(1)
	}
}

func (f *SyslogForwarder) deliver(e WriteEntry) error {
	msg := f.formatMessage(e)
	conn, err := f.dial()
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write([]byte(msg))
	return err
}

func (f *SyslogForwarder) dial() (net.Conn, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	if f.useTLS {
		return tls.DialWithDialer(&d, "tcp", f.target, &tls.Config{MinVersion: tls.VersionTLS12})
	}
	return d.Dial("tcp", f.target)
}

// formatMessage renders the audit entry in the configured format.
func (f *SyslogForwarder) formatMessage(e WriteEntry) string {
	if f.format == "cef" {
		return formatCEF(e)
	}
	return formatRFC5424(e)
}

const syslogHostname = "vakt"

// formatRFC5424 renders an RFC 5424 syslog line (PRI=134: facility local0,
// severity informational). Structured data carries the audit fields.
func formatRFC5424(e WriteEntry) string {
	ts := e.CreatedAt
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	sd := fmt.Sprintf(
		`[vakt@53595 org="%s" user="%s" action="%s" resource_type="%s" resource_id="%s" ip="%s"]`,
		escapeSD(e.OrgID), escapeSD(e.UserID), escapeSD(e.Action),
		escapeSD(e.ResourceType), escapeSD(e.ResourceID), escapeSD(e.IPAddress),
	)
	msg := e.Action + " " + e.ResourceType
	if e.ResourceName != "" {
		msg += " — " + e.ResourceName
	}
	// <PRI>VERSION TIMESTAMP HOSTNAME APP-NAME PROCID MSGID STRUCTURED-DATA MSG
	return fmt.Sprintf("<134>1 %s %s vakt-audit - AUDIT %s %s\n",
		ts.UTC().Format(time.RFC3339), syslogHostname, sd, msg)
}

// formatCEF renders an ArcSight CEF line over syslog.
func formatCEF(e WriteEntry) string {
	name := e.Action + " " + e.ResourceType
	ext := fmt.Sprintf("act=%s suser=%s src=%s cs1Label=org cs1=%s cs2Label=resourceId cs2=%s",
		escapeCEF(e.Action), escapeCEF(e.UserID), escapeCEF(e.IPAddress),
		escapeCEF(e.OrgID), escapeCEF(e.ResourceID))
	// CEF:Version|Vendor|Product|ProductVersion|SignatureID|Name|Severity|Extension
	return fmt.Sprintf("CEF:0|NorvikOps|Vakt|1.0|%s|%s|3|%s\n",
		escapeCEF(e.ResourceType), escapeCEF(name), ext)
}

func escapeSD(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, `]`, `\]`)
	return s
}

func escapeCEF(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `|`, `\|`)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "=", `\=`)
	return s
}
