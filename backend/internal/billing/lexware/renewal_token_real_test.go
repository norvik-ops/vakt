// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package lexware

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/billing/licensing"
)

// Diese Datei prueft den PAYLOAD der Schluessel, die die Erneuerungspfade
// ausstellen — nicht den Statuscode.
//
// Genau diese Luecke hat den Defekt verdeckt: getlicense_states_real_test.go
// belegt, WELCHEN Code GetLicense je Abo-Zustand liefert, und license_test.go
// belegt, dass ein Token einen Sign->parse-Rundlauf ueberlebt. Zwischen beiden
// lag der Fehler: der Server signierte den Erneuerungsschluessel ohne den
// Renewal-Token, persistierte ihn, und die Instanz konnte danach nie wieder
// selbst einen Schluessel holen — sie ging nach Ablauf dunkel, obwohl bezahlt.
// Beide Tests blieben dabei gruen.
//
// Gegen die migrierte Test-DB:
//
//	VAKT_DB_URL=postgres://vakt:vakt@127.0.0.1:15511/vakt_test?sslmode=disable \
//	  go test -run 'Renewal' -v ./internal/billing/lexware/

// ── Harness ──────────────────────────────────────────────────────────────────

// testSigningKey liefert einen echten ECDSA-Signierschluessel im PEM-Format,
// wie VAKT_LICENSE_PRIVATE_KEY ihn traegt.
func testSigningKey(t *testing.T) string {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
}

// tokenInKey liest den Renewal-Token AUS DEM SCHLUESSEL — genau so, wie
// license.AutoRefresher.currentToken() es beim Kunden tut. Die Signatur wird
// hier absichtlich nicht geprueft: geprueft wird, WAS im Payload steht.
func tokenInKey(t *testing.T, key string) string {
	t.Helper()
	require.NotEmpty(t, key, "kein Schluessel geliefert")
	parts := strings.Split(key, ".")
	require.Len(t, parts, 2, "Schluessel hat nicht die Form payload.signature: %q", key)
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	var p struct {
		RenewalToken string `json:"rt"`
		Org          string `json:"org"`
		Exp          *int64 `json:"exp"`
	}
	require.NoError(t, json.Unmarshal(raw, &p))
	return p.RenewalToken
}

// smtpSink ist ein minimaler SMTP-Server, der die Mails einsammelt statt sie zu
// verschicken. mailExpiringOnce speichert den neuen Schluessel NUR, wenn der
// Versand geklappt hat ("pretending they do would leave them locked out with a
// key only we can see") — ohne echten Versand laeuft der Sweep also nie zu Ende
// und ein Test ohne SMTP wuerde am Defekt vorbeimessen.
type smtpSink struct {
	addr string
	mu   sync.Mutex
	msgs []string
}

func newSMTPSink(t *testing.T) *smtpSink {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &smtpSink{addr: ln.Addr().String()}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(conn)
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

func (s *smtpSink) serve(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }

	w("220 sink.invalid ESMTP")
	var body strings.Builder
	inData := false
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		if inData {
			if line == "." {
				inData = false
				s.mu.Lock()
				s.msgs = append(s.msgs, body.String())
				s.mu.Unlock()
				body.Reset()
				w("250 2.0.0 Ok")
				continue
			}
			body.WriteString(line + "\n")
			continue
		}

		switch {
		case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
			w("250-sink.invalid")
			w("250 SIZE 35882577")
		case strings.HasPrefix(line, "MAIL FROM"), strings.HasPrefix(line, "RCPT TO"):
			w("250 2.0.0 Ok")
		case line == "DATA":
			inData = true
			w("354 End data with <CR><LF>.<CR><LF>")
		case line == "QUIT":
			w("221 2.0.0 Bye")
			return
		case line == "RSET":
			w("250 2.0.0 Ok")
		default:
			w("250 2.0.0 Ok")
		}
	}
}

func (s *smtpSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.msgs)
}

func (s *smtpSink) last() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.msgs) == 0 {
		return ""
	}
	return s.msgs[len(s.msgs)-1]
}

// renewalHandler baut einen Handler mit echtem Signierschluessel und dem
// SMTP-Sink als Zustellweg.
func renewalHandler(t *testing.T, pool *pgxpool.Pool, sink *smtpSink) *Handler {
	t.Helper()
	host, port, err := net.SplitHostPort(sink.addr)
	require.NoError(t, err)
	cfg := licensing.SMTPConfig{Host: host, Port: port, From: "billing@example.invalid"}
	iss := licensing.NewIssuer(testSigningKey(t), cfg)
	return NewHandler(pool, nil, iss, cfg, "https://billing.test", "ops@example.invalid")
}

// seedPaidLicence legt die Lage nach, in der die Erneuerung greift: ein
// bezahltes Abo, eine bezahlte Rechnung bis periodEnd, und eine Lizenzzeile,
// deren Schluessel FRUEHER auslaeuft als der Anspruch reicht.
//
// Das ist keine Kunstlage: MSP-Sitzzeilen und Freilizenzen landen genau dort,
// weil settle() nur die Lizenzzeile der eigenen Firma nachzieht.
func seedPaidLicence(t *testing.T, pool *pgxpool.Pool, interval string, periodEnd, keyExpires time.Time) (subID, token string) {
	t.Helper()
	ctx := context.Background()

	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO billing_quote_requests (company_name, email, approval_token_hash, status, product, interval, paid_at)
		VALUES ('ZZZ Renewal Probe', 'zzz-renewal@example.invalid', 'not-a-real-hash', 'paid', 'pro', $1, NOW())
		RETURNING id::text`, interval).Scan(&subID))

	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM billing_invoices WHERE subscription_id = $1::uuid`, subID)
		_, _ = pool.Exec(c, `DELETE FROM billing_licenses WHERE subscription_id = $1::uuid`, subID)
		_, _ = pool.Exec(c, `DELETE FROM billing_quote_requests WHERE id = $1::uuid`, subID)
	})

	addPaidInvoice(t, pool, subID, periodEnd)

	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO billing_licenses (subscription_id, org_name, license_key, expires_at, kind)
		VALUES ($1::uuid, 'ZZZ Renewal Probe', 'placeholder', $2, 'full')
		RETURNING renewal_token::text`, subID, keyExpires).Scan(&token))
	return subID, token
}

// addPaidInvoice bucht eine bezahlte Periode. Der Anspruch (Entitlement) ist
// MAX(period_end) + Grace, also verschiebt genau das den Anspruch nach vorn.
func addPaidInvoice(t *testing.T, pool *pgxpool.Pool, subID string, periodEnd time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO billing_invoices (subscription_id, lexware_invoice_id, status,
		                              net_amount_cents, gross_amount_cents, tax_amount_cents,
		                              period_start, period_end, paid_at)
		VALUES ($1::uuid, 'ZZZ-'||gen_random_uuid()::text, 'paid', 29900, 29900, 0,
		        ($2::timestamptz - INTERVAL '30 days')::date, $2::timestamptz::date, NOW())`,
		subID, periodEnd)
	require.NoError(t, err)
}

// signCurrentKey legt den Schluessel ab, den der Kunde HAELT — mit Token, so wie
// settle() und seats.Issue() ihn ausstellen. Ohne diesen Ausgangszustand koennte
// der Test nicht zeigen, dass die Erneuerung den Token VERLIERT.
func signCurrentKey(t *testing.T, pool *pgxpool.Pool, h *Handler, token, org, interval string, expires time.Time) string {
	t.Helper()
	key, err := h.issuer.SignUntil(licensing.ForLicence(token, org, "zzz-renewal@example.invalid", interval), expires)
	require.NoError(t, err)
	require.Equal(t, token, tokenInKey(t, key), "Ausgangsschluessel muss den Token tragen")
	_, err = pool.Exec(context.Background(),
		`UPDATE billing_licenses SET license_key = $2 WHERE renewal_token = $1::uuid`, token, key)
	require.NoError(t, err)
	return key
}

func storedKey(t *testing.T, pool *pgxpool.Pool, token string) (string, time.Time) {
	t.Helper()
	var key string
	var expires time.Time
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT license_key, expires_at FROM billing_licenses WHERE renewal_token = $1::uuid`, token).
		Scan(&key, &expires))
	return key, expires
}

// ── K4-01: GetLicense ────────────────────────────────────────────────────────

// TestRenewalKeepsTheRenewalTokenInTheKey ist der Test, der gefehlt hat.
//
// Er prueft den Payload des Schluessels, den GetLicense ZURUECKGIBT, und den, der
// dabei in billing_licenses landet. Ohne den Token im Schluessel liefert
// AutoRefresher.currentToken() "" — der Refresher setzt renewalFailing und ruft
// nie wieder an. Die Auto-Erneuerung schaltete sich also bei der ERSTEN
// Erneuerung selbst ab, und die Ersatz-Mail (siehe unten) heilte das nicht,
// weil sie den Token ebenfalls nicht trug.
func TestRenewalKeepsTheRenewalTokenInTheKey(t *testing.T) {
	pool := statesPool(t)
	sink := newSMTPSink(t)
	h := renewalHandler(t, pool, sink)

	// Anspruch reicht 100 Tage, der gehaltene Schluessel laeuft in 10 Tagen aus:
	// unter RenewBelowDays (60) und limit.After(expires) — der Neusignier-Zweig
	// greift. Das ist die MSP-Sitz-/Freilizenz-Lage.
	keyExpires := time.Now().Add(10 * 24 * time.Hour)
	_, token := seedPaidLicence(t, pool, "year", time.Now().Add(100*24*time.Hour), keyExpires)
	before := signCurrentKey(t, pool, h, token, "ZZZ Renewal Probe", "year", keyExpires)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing/license", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	require.NoError(t, h.GetLicense(e.NewContext(req, rec)))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var out struct{ Key string }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))

	// Nicht-Vakuitaet: der Zweig muss wirklich gefeuert haben. Waere der
	// Schluessel unveraendert, wuerde die Token-Zusicherung nichts beweisen.
	require.NotEqual(t, before, out.Key,
		"GetLicense hat NICHT neu signiert — dann prueft dieser Test den Erneuerungspfad nicht")

	assert.Equal(t, token, tokenInKey(t, out.Key),
		"der erneuerte Schluessel traegt den Renewal-Token nicht — die Instanz kann nie wieder selbst einen holen und geht nach Ablauf dunkel, obwohl bezahlt")

	stored, _ := storedKey(t, pool, token)
	assert.Equal(t, out.Key, stored, "ausgelieferter und persistierter Schluessel muessen identisch sein")
	assert.Equal(t, token, tokenInKey(t, stored),
		"der in billing_licenses persistierte Schluessel traegt den Renewal-Token nicht — auch jeder spaetere Abruf liefert dann einen tokenlosen Schluessel")
}

// ── K4-01 + K4-02: der Ablauf-Sweep ──────────────────────────────────────────

// TestExpiringSweepMailsTheTokenAndOnlyOnce deckt beide Defekte an einer Stelle,
// weil sie im selben Schleifendurchlauf sitzen:
//
//   - K4-01: der gemailte und persistierte Schluessel muss den Token tragen.
//   - K4-02: die Aktion des Sweeps muss seine Auswahlbedingung AUFLOESEN. Sie tat
//     es nicht: gewaehlt wird expires_at < NOW()+35d, gesetzt wird expires_at :=
//     Periodenende+Grace, beim Monatsplan 33-36 Tage entfernt — also weiter unter
//     dem Cutoff. Alle 12 h eine Mail, ~58 im Monat, an einen Kunden, bei dem
//     nichts fehlerhaft ist.
func TestExpiringSweepMailsTheTokenAndOnlyOnce(t *testing.T) {
	pool := statesPool(t)
	sink := newSMTPSink(t)
	h := renewalHandler(t, pool, sink)
	ctx := context.Background()

	// Monatskunde: Periode laeuft in 30 Tagen aus, Grace 5 -> Anspruch ~35 Tage.
	// Genau die Lage, in der der Cutoff (35 Tage) dauerhaft wahr bleibt.
	periodEnd := time.Now().Add(30 * 24 * time.Hour)
	_, token := seedPaidLicence(t, pool, "month", periodEnd, time.Now().Add(10*24*time.Hour))
	signCurrentKey(t, pool, h, token, "ZZZ Renewal Probe", "month", time.Now().Add(10*24*time.Hour))

	// Lauf 1 — hier ist eine Mail faellig: der Kunde haelt einen Schluessel, der
	// 25 Tage vor seinem Anspruch endet.
	h.mailExpiringOnce(ctx)
	require.Equal(t, 1, sink.count(), "der Sweep muss mailen, wenn ein spaeterer Schluessel verfuegbar ist")

	mailed := sink.last()
	stored, storedExpiry := storedKey(t, pool, token)
	assert.Contains(t, mailed, stored, "die Mail muss genau den Schluessel enthalten, der gespeichert wurde")
	assert.Equal(t, token, tokenInKey(t, stored),
		"der Ersatzschluessel aus der Mail traegt den Renewal-Token nicht — damit heilt auch die Mail den tokenlosen Zustand nicht")
	assert.WithinDuration(t, entitlementOf(periodEnd, 5), storedExpiry, time.Minute,
		"expires_at muss auf den Anspruch (Periodenende + Grace) gesetzt werden")

	// Lauf 2, 12 h spaeter: der Zustand ist unveraendert, es gibt nichts Neues zu
	// geben. Der Sweep darf NICHT erneut mailen. Vor dem Fix mailte er hier wieder
	// — und danach bei jedem der ~58 Laeufe im Monat.
	h.mailExpiringOnce(ctx)
	assert.Equal(t, 1, sink.count(),
		"der Sweep mailt denselben Schluessel erneut: seine Aktion loest die Auswahlbedingung nicht auf (~58 Mails/Monat an einen Kunden, bei dem nichts fehlerhaft ist)")

	// Gegenprobe — ohne die waere der Guard oben von blosser Stille nicht zu
	// unterscheiden, und Stille ist hier schlimmer als Spam: sobald der Anspruch
	// WIRKLICH weiterwandert (naechste Rechnung bezahlt), muss GENAU EINE Mail
	// rausgehen.
	newPeriodEnd := periodEnd.AddDate(0, 1, 0)
	addPaidInvoice(t, pool, mustSubID(t, pool, token), newPeriodEnd)

	h.mailExpiringOnce(ctx)
	require.Equal(t, 2, sink.count(),
		"nach einer bezahlten Verlaengerung MUSS der Sweep den neuen Schluessel mailen — ein Guard, der das unterdrueckt, ersetzt Spam durch Stille")

	stored2, expiry2 := storedKey(t, pool, token)
	assert.Equal(t, token, tokenInKey(t, stored2), "auch der Folgeschluessel traegt den Token")
	assert.WithinDuration(t, entitlementOf(newPeriodEnd, 5), expiry2, time.Minute,
		"expires_at muss auf den neuen Anspruch nachziehen")

	// Und danach wieder Ruhe.
	h.mailExpiringOnce(ctx)
	assert.Equal(t, 2, sink.count(), "nach dem Nachziehen muss der Sweep wieder still sein")
}

// entitlementOf rechnet den Anspruch so nach, wie Entitlement() ihn bildet:
// billing_invoices.period_end ist eine DATE-Spalte, also Mitternacht UTC, und der
// Grace des Plans kommt in Tagen dazu. Ein Test, der stattdessen die Wanduhrzeit
// des Seeds nimmt, liegt um bis zu 24 h daneben.
func entitlementOf(periodEnd time.Time, graceDays int) time.Time {
	d := periodEnd.UTC()
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, graceDays)
}

func mustSubID(t *testing.T, pool *pgxpool.Pool, token string) string {
	t.Helper()
	var subID string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT subscription_id::text FROM billing_licenses WHERE renewal_token = $1::uuid`, token).Scan(&subID))
	return subID
}

// TestExpiringSweepStaysSilentWhenNothingIsPaid ist die andere Richtung des
// Guards: laeuft der Anspruch selbst ab, gibt es keinen spaeteren Schluessel —
// dann darf der Sweep nicht mailen, und das war auch vorher so. Der Test haelt
// fest, dass der neue Guard diese Grenze nicht verschoben hat.
func TestExpiringSweepStaysSilentWhenNothingIsPaid(t *testing.T) {
	pool := statesPool(t)
	sink := newSMTPSink(t)
	h := renewalHandler(t, pool, sink)

	// Anspruch schon abgelaufen: Periodenende vor 20 Tagen, Grace 5.
	_, token := seedPaidLicence(t, pool, "month", time.Now().Add(-20*24*time.Hour), time.Now().Add(5*24*time.Hour))
	signCurrentKey(t, pool, h, token, "ZZZ Renewal Probe", "month", time.Now().Add(5*24*time.Hour))

	h.mailExpiringOnce(context.Background())
	assert.Equal(t, 0, sink.count(),
		"ein Kunde, dessen bezahlte Periode vorbei ist, bekommt keinen neuen Schluessel — das IST die Abschaltung")
}

// TestSweepQueryFindsWhatTheGuardMustDecide belegt, dass der Guard im Go-Code
// sitzt und nicht in der Query: die Auswahl liefert die Zeile weiterhin, der
// Fortschritts-Guard entscheidet danach. Sonst koennte ein spaeterer Umbau den
// Guard in die Query verschieben und die Auswahl unbemerkt verengen.
func TestSweepQueryFindsWhatTheGuardMustDecide(t *testing.T) {
	pool := statesPool(t)
	sink := newSMTPSink(t)
	h := renewalHandler(t, pool, sink)

	periodEnd := time.Now().Add(30 * 24 * time.Hour)
	_, token := seedPaidLicence(t, pool, "month", periodEnd, time.Now().Add(10*24*time.Hour))
	signCurrentKey(t, pool, h, token, "ZZZ Renewal Probe", "month", time.Now().Add(10*24*time.Hour))
	h.mailExpiringOnce(context.Background())

	// Nach dem Lauf ist expires_at == Anspruch == ~35 Tage, also weiter UNTER dem
	// 35-Tage-Cutoff: die Auswahlbedingung ist nach wie vor wahr. Genau deshalb
	// braucht es den Fortschritts-Guard und nicht einen groesseren Cutoff.
	_, expiry := storedKey(t, pool, token)
	cutoff := time.Now().Add(35 * 24 * time.Hour)
	require.True(t, expiry.Before(cutoff),
		"Erwartung des Befunds: der Anspruch eines Monatskunden liegt IMMER unter dem 35-Tage-Cutoff (%s < %s)",
		expiry.Format(time.RFC3339), cutoff.Format(time.RFC3339))

	var selected int
	require.NoError(t, pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM billing_licenses bl
		  JOIN billing_quote_requests s ON s.id = bl.subscription_id
		 WHERE bl.renewal_token = $1::uuid
		   AND bl.revoked_at IS NULL AND bl.license_key <> '' AND bl.kind <> 'trial'
		   AND s.status = 'paid' AND s.cancelled_at IS NULL
		   AND bl.expires_at < $2`, token, cutoff).Scan(&selected))
	assert.Equal(t, 1, selected,
		"die Zeile wird weiterhin ausgewaehlt — der Fortschritt muss also nach der Auswahl entschieden werden")
	_ = fmt.Sprint(sink.count())
}
