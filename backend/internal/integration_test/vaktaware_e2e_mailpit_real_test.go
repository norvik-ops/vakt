//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/matharnica/vakt/internal/modules/vaktaware"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/reporting"
)

// TestVaktaware_E2E_MailpitToClickRate is the acceptance test for S127-6, and the
// only honest one: a real mail, sent over real SMTP, fetched back out of a real
// mailbox, its link followed by something holding no token at all.
//
// Everything else in this suite fakes the transport. That is the right trade for a
// unit of logic — but it cannot see the two things that actually broke here, twice,
// in production:
//
//   - the tracking routes were mounted behind auth, so the recipient's browser (which
//     by definition has no session) got a 401 and the click vanished (S127);
//   - the token in the link resolved to nothing, because nothing had stored it, so
//     the click was rejected as invalid (S126, migration 242).
//
// Both were invisible to every test in the project and both were found by a person
// clicking a link. So this test clicks the link.
//
// The public routes are mounted here exactly as cmd/api/routes.go mounts them: on a
// group with NO auth middleware. If someone ever "tidies" them back under `protected`,
// this test is what says no.
func TestVaktaware_E2E_MailpitToClickRate(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()

	// ── A real mail server ────────────────────────────────────────────────────
	mp, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        imageMailpit,
			ExposedPorts: []string{"1025/tcp", "8025/tcp"},
			// Readiness has to cover BOTH ports this test uses, and it used to cover
			// only one: `ForListeningPort("8025/tcp")` — while the very next thing
			// the test does is dial **1025** over SMTP. That smtpd came up at all
			// was never asserted. That is the whole reason for the change.
			//
			// What the old check did NOT get wrong, contrary to an earlier version
			// of this comment: `ForListeningPort` is not merely an external dial.
			// wait/host_port.go does an external dial AND an internalCheck that
			// shells into the container (`cat /proc/net/tcp* | … || nc -vz … ||
			// </dev/tcp/…`), and mailpit ships /bin/sh with a readable /proc — so
			// 8025 really was verified as bound inside the container.
			//
			// Net effect of the swap, stated plainly because it is not a pure win:
			// the 1025 leg GAINS that internal check (it is a ForListeningPort), and
			// the 8025 leg LOSES it (HTTPStrategy has no internal check — it issues
			// a real request against the mapped port). For 8025 that trade is worth
			// it: a bound port that does not yet answer /readyz with 200 is not a
			// usable API, and an HTTP status cannot be produced by docker-proxy.
			//
			// WithDeadline rather than WithStartupTimeout: the latter is deprecated
			// on MultiStrategy and delegates here anyway. The 60s covers both legs.
			WaitingFor: wait.ForAll(
				wait.ForListeningPort("1025/tcp"),
				wait.ForHTTP("/readyz").WithPort("8025/tcp").
					WithStatusCodeMatcher(func(status int) bool { return status == http.StatusOK }),
			).WithDeadline(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Skipf("integration: cannot start mailpit (%v)", err)
	}
	defer func() { _ = mp.Terminate(ctx) }()

	smtpHost, err := mp.Host(ctx)
	require.NoError(t, err)
	smtpPort, err := mp.MappedPort(ctx, "1025")
	require.NoError(t, err)
	apiPort, err := mp.MappedPort(ctx, "8025")
	require.NoError(t, err)

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ('aware-e2e@acme.test') RETURNING id::text`).Scan(&userID))

	// ── The public tracking surface, mounted the way production mounts it ──────
	//
	// No auth middleware, no CSRF: the recipient of a phishing simulation has
	// neither a token nor a csrf cookie. That is the whole point, and it is what
	// S127 had to fix.
	var srv *httptest.Server
	e := echo.New()
	repo := vaktaware.NewRepository(pool)

	// The service needs to know the URL it is embedding in the mail, and that URL
	// has to be the one this test can actually reach — so the server is created
	// first and the service is built around its address.
	srv = httptest.NewServer(e)
	defer srv.Close()

	svc := vaktaware.NewService(pool, vaktaware.SMTPConfig{
		Host:   smtpHost,
		Port:   smtpPort.Port(),
		From:   "it-security@acme.test",
		AppURL: srv.URL,
	})
	vaktaware.RegisterPublic(e.Group("/api/v1/vaktaware"), vaktaware.NewHandler(svc), passThroughMW)

	// ── A campaign, sent for real ─────────────────────────────────────────────
	campaignID, _ := seedCampaign(t, ctx, pool, repo, orgID, userID, true, false)
	_, err = pool.Exec(ctx, `UPDATE sr_targets SET is_bounced = false WHERE org_id = $1::uuid`, orgID)
	require.NoError(t, err)

	// NOTE (R1-INT-02 follow-up, 2026-07-30): this assertion is VACUOUS as far as
	// delivery goes, and its message used to claim otherwise ("the campaign must go
	// out over real SMTP"). SendCampaignEmails swallows the SMTP error —
	// vaktaware/service.go only log.Error()s a non-nil sendErr, then calls
	// SetCampaignCompleted and returns nil. So a total delivery failure
	// ("smtp dial: EOF", every mail lost) still returns nil and this line passes;
	// the test then proceeds to poll Mailpit and fails later, somewhere else.
	// What actually proves delivery here is the mail being read back out of the
	// mailbox further down, not this call — so the message now says only what the
	// call can actually establish. A comment is not enforcement; the string is the
	// part a future reader sees when it fails.
	//
	// Reported as its own finding (swallowed error + campaign stamped 'completed'
	// + emails_sent counting pre-send events = a plausible-looking zero that flows
	// into Comply as evidence). Deliberately NOT fixed here: it is a product
	// defect in vaktaware, not a test-infrastructure one, and this commit owns
	// test files only.
	require.NoError(t, svc.SendCampaignEmails(ctx, orgID, campaignID),
		"SendCampaignEmails must not return an error — note this does NOT prove "+
			"delivery, it swallows SMTP failures; the mailbox read below is the proof")

	// ── Read the mail back out of the mailbox ─────────────────────────────────
	mailAPI := "http://" + smtpHost + ":" + apiPort.Port()
	body := fetchFirstMailBody(t, mailAPI, 2)

	// Mailpit returns newest-first, and both recipients were mailed — so this is
	// whichever of the two arrived last, not necessarily Alice. What matters is that
	// the template was rendered with a real recipient's data, not that a particular
	// one happened to be on top.
	assert.Regexp(t, `Hallo (Alice|Bob)`, body, "the template must have been rendered for the recipient")

	m := regexp.MustCompile(`/api/v1/vaktaware/t/([0-9a-f-]{36})`).FindStringSubmatch(body)
	require.Len(t, m, 2, "the delivered mail must carry a click link on /t/ — body:\n%s", body)
	token := m[1]

	// ── The click. No token, no cookie, no session — just a browser. ──────────
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/vaktaware/t/"+token, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode,
		"the recipient has no session — a 401 here is exactly the bug S127 fixed, and it would mean "+
			"Vakt Aware measures nothing again")

	// And the open pixel, which the mail client fetches on its own.
	pixReq, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/vaktaware/track/"+token, nil)
	require.NoError(t, err)
	pixResp, err := http.DefaultClient.Do(pixReq)
	require.NoError(t, err)
	defer func() { _ = pixResp.Body.Close() }()
	assert.Equal(t, http.StatusOK, pixResp.StatusCode, "the open pixel must load without a session too")

	// ── Did any of it count? ──────────────────────────────────────────────────
	stats, err := svc.GetCampaignStats(ctx, orgID, campaignID)
	require.NoError(t, err)
	assert.True(t, stats.TrackingMeasured)
	assert.Equal(t, 2, stats.EmailsSent, "both recipients were mailed")
	assert.Equal(t, 1, stats.Clicks, "the click that was actually made must be the click that is counted")
	assert.Equal(t, 1, stats.Opens)
	assert.InDelta(t, 50.0, stats.ClickRate, 0.01)

	// ── And does it reach the auditor? ────────────────────────────────────────
	snap := reporting.CalculateKPIsForOrg(ctx, pool, orgID)
	require.NotNil(t, snap.PhishingClickRate,
		"a completed campaign must produce a phishing click rate for Vakt Comply")
	assert.InDelta(t, 50.0, *snap.PhishingClickRate, 0.01,
		"one of two recipients clicked — this is the number that ends up in an ISO 27001 A.6.3 file")
}

// fetchFirstMailBody waits for `want` messages to land and returns the body of the
// first one. It waits rather than reads once: SendCampaignEmails returns when the
// SMTP conversation is over, which is a moment before Mailpit has finished indexing.
func fetchFirstMailBody(t *testing.T, mailAPI string, want int) string {
	t.Helper()

	var list struct {
		Messages []struct {
			ID string `json:"ID"`
		} `json:"messages"`
	}
	// The send returned before Mailpit necessarily finished indexing; give it a moment.
	for i := 0; i < 40; i++ {
		resp, err := http.Get(mailAPI + "/api/v1/messages") //nolint:noctx // test helper, bounded by the loop
		require.NoError(t, err)
		raw, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &list))
		if len(list.Messages) >= want {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	require.Len(t, list.Messages, want,
		"the mail server received %d of %d campaign mails — one never left the machine", len(list.Messages), want)

	resp, err := http.Get(mailAPI + "/api/v1/message/" + list.Messages[0].ID) //nolint:noctx // test helper
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	var msg struct {
		HTML string `json:"HTML"`
		Text string `json:"Text"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&msg))
	if msg.HTML != "" {
		return msg.HTML
	}
	return msg.Text
}
