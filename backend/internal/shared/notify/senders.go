// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package notify

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/shared/httputil"
)

// DefaultSenders builds the production Sender registry from cfg: SMTP for email,
// an SSRF-guarded HTTP client for Slack/Teams/generic webhooks. The worker wires
// this into DeliverHandler at startup. A Channel with no entry here is delivered
// by no adapter and the handler fails the task loudly (see DeliverHandler).
func DefaultSenders(cfg *config.Config) map[Channel]Sender {
	smtpSnd := &smtpSender{
		host: cfg.SMTPHost,
		port: cfg.SMTPPort,
		user: cfg.SMTPUser,
		pass: cfg.SMTPPass,
		from: cfg.SMTPFrom,
	}
	// allowPrivate=true mirrors the alerting service: this is a self-hosted
	// product and on-prem Slack/webhook receivers on private ranges are
	// legitimate. The guarded client still re-resolves at dial time to close the
	// DNS-rebinding TOCTOU.
	httpSnd := &httpSender{client: httputil.GuardedClient(10*time.Second, true)}
	return map[Channel]Sender{
		ChannelEmail:   smtpSnd,
		ChannelSlack:   httpSnd,
		ChannelTeams:   httpSnd,
		ChannelWebhook: httpSnd,
	}
}

// stripCRLF removes carriage returns and newlines to prevent SMTP header
// injection when a Message field is interpolated into a MIME header.
func stripCRLF(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}

// smtpSender delivers a Message as a plain-text e-mail over SMTP. Target is the
// recipient address.
type smtpSender struct {
	host, port, user, pass, from string
}

func (s *smtpSender) Send(_ context.Context, msg Message) error {
	if s.host == "" {
		return fmt.Errorf("smtp: no host configured (VAKT_SMTP_HOST)")
	}
	if msg.Target == "" {
		return fmt.Errorf("smtp: empty recipient")
	}
	from := s.from
	if from == "" {
		from = "noreply@" + s.host
	}
	to := stripCRLF(msg.Target)
	subject := stripCRLF(msg.Title)
	body := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		stripCRLF(from), to, subject, msg.Body)

	client, closeClient, err := s.open()
	if err != nil {
		return err
	}
	defer closeClient()

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp RCPT: %w", err)
	}
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := wc.Write([]byte(body)); err != nil {
		_ = wc.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	return wc.Close()
}

// open dials an SMTP connection, negotiating TLS according to the port, and
// authenticates when a user is configured. Mirrors vaktaware/smtp_sender.go.
func (s *smtpSender) open() (*smtp.Client, func(), error) {
	addr := net.JoinHostPort(s.host, s.port)
	switch s.port {
	case "587": // STARTTLS
		conn, err := smtp.Dial(addr)
		if err != nil {
			return nil, nil, fmt.Errorf("smtp dial: %w", err)
		}
		if err := conn.StartTLS(&tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}); err != nil {
			_ = conn.Close()
			return nil, nil, fmt.Errorf("starttls: %w", err)
		}
		if err := s.auth(conn); err != nil {
			_ = conn.Close()
			return nil, nil, err
		}
		return conn, func() { _ = conn.Quit() }, nil
	case "465": // implicit TLS
		tlsConn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12})
		if err != nil {
			return nil, nil, fmt.Errorf("smtp tls dial: %w", err)
		}
		c, err := smtp.NewClient(tlsConn, s.host)
		if err != nil {
			_ = tlsConn.Close()
			return nil, nil, fmt.Errorf("smtp client: %w", err)
		}
		if err := s.auth(c); err != nil {
			_ = c.Close()
			return nil, nil, err
		}
		return c, func() { _ = c.Quit() }, nil
	default: // plain / port 25 (Mailpit dev)
		conn, err := smtp.Dial(addr)
		if err != nil {
			return nil, nil, fmt.Errorf("smtp dial: %w", err)
		}
		if err := s.auth(conn); err != nil {
			_ = conn.Close()
			return nil, nil, err
		}
		return conn, func() { _ = conn.Quit() }, nil
	}
}

func (s *smtpSender) auth(c *smtp.Client) error {
	if s.user == "" {
		return nil
	}
	if err := c.Auth(smtp.PlainAuth("", s.user, s.pass, s.host)); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	return nil
}

// httpSender delivers a Message as a JSON HTTP POST to Target (a webhook URL).
// The body shape is chosen per channel so Slack and Teams render it natively.
type httpSender struct {
	client *http.Client
}

func (h *httpSender) Send(ctx context.Context, msg Message) error {
	if msg.Target == "" {
		return fmt.Errorf("webhook: empty target URL")
	}
	body, err := renderWebhookBody(msg)
	if err != nil {
		return fmt.Errorf("webhook marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, msg.Target, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook post: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// renderWebhookBody produces the JSON payload for the given channel. Slack and
// Teams both accept a top-level "text" field for a simple message; the generic
// webhook gets the structured fields.
func renderWebhookBody(msg Message) ([]byte, error) {
	text := msg.Title
	if msg.Body != "" {
		text = msg.Title + "\n" + msg.Body
	}
	switch msg.Channel {
	case ChannelSlack, ChannelTeams:
		return json.Marshal(map[string]string{"text": text})
	default:
		return json.Marshal(map[string]string{
			"title": msg.Title,
			"body":  msg.Body,
		})
	}
}
