// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktaware

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"net/textproto"
	"regexp"
)

// OutboundMail is one fully rendered message, ready for the wire. Everything
// interesting has already happened by the time a message becomes an OutboundMail:
// the template is rendered, the tracking token is minted and embedded, the MIME
// headers are assembled and CRLF-sanitised.
type OutboundMail struct {
	From string
	To   string
	Body []byte
}

// MailSender puts rendered messages on the wire.
//
// This is the seam S126 added, and it is deliberately drawn here rather than
// deeper: everything that decides *what* gets sent — which targets, which
// template, which tracking token, which MIME body, and afterwards which campaign
// is marked complete and which evidence is collected — is ordinary logic that a
// test can drive against a real database. Only the socket cannot be. Before this,
// SendCampaignEmails dialled net/smtp inline, which made the entire core flow of
// Vakt Aware unreachable by any test. That is not a coincidence: this module has
// produced more born-broken bugs than any other, and every one of them was found
// by a person clicking through a live stack.
//
// House pattern (internal/services/scim/service.go): a narrow interface, a
// hand-written fake, no mock library, and the constructor still takes the
// concrete config.
// DeliveryResult is the outcome for ONE message, in the order the messages were
// handed over.
//
// The interface used to return only a count. A count cannot say WHICH recipient
// was rejected, so the caller had no way to take back the „sent" event it had
// already written for that person — the campaign reported „3 versendet" while
// the mail server had accepted one (L2-01). Per-message results are what makes
// that reconciliation possible at all.
type DeliveryResult struct {
	// Delivered is true when the mail server accepted the message.
	Delivered bool
	// Err is the rejection, nil when Delivered.
	Err error
	// Permanent marks a 5xx rejection: the address is wrong, not the moment.
	// A retry cannot help, and the address should not be mailed again (L1-06).
	Permanent bool
}

// MailSender puts rendered messages on the wire.
type MailSender interface {
	// Send delivers every message and reports the outcome per message, plus the
	// first failure encountered (nil when all went out). The returned slice has
	// one entry per input message, in the same order.
	//
	// One bad recipient must not abort the rest: a phishing simulation with one
	// stale address still has to reach the other two hundred people, and a
	// campaign that silently delivered to nobody would report a 0% click rate
	// that looks exactly like a well-trained workforce.
	Send(ctx context.Context, msgs []OutboundMail) (results []DeliveryResult, err error)
}

// smtpSender is the production MailSender: one connection, all messages.
type smtpSender struct {
	cfg SMTPConfig
}

func (s *smtpSender) Send(_ context.Context, msgs []OutboundMail) ([]DeliveryResult, error) {
	if len(msgs) == 0 {
		return nil, nil
	}

	results := make([]DeliveryResult, len(msgs))

	client, closeClient, err := s.open(msgs[0].From)
	if err != nil {
		// The connection never came up, so NOTHING was delivered. Saying so per
		// message is what lets the caller take back every „sent" event instead of
		// reporting a campaign that nobody received as complete.
		openErr := fmt.Errorf("smtp open: %w", err)
		for i := range results {
			results[i] = DeliveryResult{Err: openErr}
		}
		return results, openErr
	}
	defer closeClient()

	var firstErr error
	for i, m := range msgs {
		sendErr := sendViaClient(client, m.From, m.To, m.Body)
		if sendErr != nil {
			results[i] = DeliveryResult{Err: sendErr, Permanent: isPermanentSMTPError(sendErr)}
			if firstErr == nil {
				firstErr = sendErr
			}
			continue
		}
		results[i] = DeliveryResult{Delivered: true}
	}
	return results, firstErr
}

// isPermanentSMTPError reports whether the server answered with a 5xx code —
// „this address is wrong", as opposed to 4xx „not right now".
//
// net/smtp surfaces the reply as *textproto.Error with the numeric code intact,
// which is the reliable path. The string fallback exists because a few of the
// client's own wrappers lose the type; matching on a leading 5 of a three-digit
// code is narrow enough not to misread a dial error as a rejection.
func isPermanentSMTPError(err error) bool {
	var protoErr *textproto.Error
	if errors.As(err, &protoErr) {
		return protoErr.Code >= 500 && protoErr.Code < 600
	}
	return permanentCodeRe.MatchString(err.Error())
}

var permanentCodeRe = regexp.MustCompile(`\b5\d\d\b`)

// open dials an authenticated SMTP connection and returns the client plus a
// close function.
func (s *smtpSender) open(_ string) (*smtp.Client, func(), error) {
	addr := net.JoinHostPort(s.cfg.Host, s.cfg.Port)

	var client *smtp.Client

	switch s.cfg.Port {
	case "587": // STARTTLS
		conn, err := smtp.Dial(addr)
		if err != nil {
			return nil, nil, fmt.Errorf("smtp dial: %w", err)
		}
		if err := conn.StartTLS(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			_ = conn.Close()
			return nil, nil, fmt.Errorf("starttls: %w", err)
		}
		if s.cfg.User != "" {
			auth := smtp.PlainAuth("", s.cfg.User, s.cfg.Pass, s.cfg.Host)
			if err := conn.Auth(auth); err != nil {
				_ = conn.Close()
				return nil, nil, fmt.Errorf("smtp auth: %w", err)
			}
		}
		client = conn

	case "465": // implicit TLS
		tlsConn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12})
		if err != nil {
			return nil, nil, fmt.Errorf("smtp tls dial: %w", err)
		}
		c, err := smtp.NewClient(tlsConn, s.cfg.Host)
		if err != nil {
			_ = tlsConn.Close()
			return nil, nil, fmt.Errorf("smtp client: %w", err)
		}
		if s.cfg.User != "" {
			auth := smtp.PlainAuth("", s.cfg.User, s.cfg.Pass, s.cfg.Host)
			if err := c.Auth(auth); err != nil {
				_ = c.Close()
				return nil, nil, fmt.Errorf("smtp auth: %w", err)
			}
		}
		client = c

	default: // plain / port 25 (Mailpit dev)
		conn, err := smtp.Dial(addr)
		if err != nil {
			return nil, nil, fmt.Errorf("smtp dial: %w", err)
		}
		if s.cfg.User != "" {
			auth := smtp.PlainAuth("", s.cfg.User, s.cfg.Pass, s.cfg.Host)
			if err := conn.Auth(auth); err != nil {
				_ = conn.Close()
				return nil, nil, fmt.Errorf("smtp auth: %w", err)
			}
		}
		client = conn
	}

	return client, func() { _ = client.Quit() }, nil
}

// sendViaClient delivers a single message through an already-open SMTP client.
// Each call issues MAIL FROM / RCPT TO / DATA against the existing connection.
func sendViaClient(client *smtp.Client, from, to string, msg []byte) error {
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
	if _, err := wc.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	return wc.Close()
}
