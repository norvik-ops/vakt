// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package admin

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/billing/lexware"
	"github.com/matharnica/vakt/internal/billing/portal"
)

type Handler struct {
	db      *pgxpool.Pool
	seats   *lexware.Seats
	billing *lexware.Handler // for the Lexware reconciliation
	baseURL string
}

func NewHandler(db *pgxpool.Pool, seats *lexware.Seats, billing *lexware.Handler, baseURL string) *Handler {
	return &Handler{db: db, seats: seats, billing: billing, baseURL: baseURL}
}

func Register(e *echo.Echo, h *Handler, cfg Config) {
	g := e.Group("", RequireCloudflareAccess(cfg), setCSRF, requireCSRF)

	g.GET("/", h.Dashboard)
	g.GET("/subscriptions", h.Subscriptions)
	g.GET("/invoices", h.Invoices)
	g.GET("/licences", h.Licences)
	g.GET("/lexware", h.LexwareCheck)
	g.GET("/subscriptions/:id", h.Subscription)
	g.GET("/new", h.NewSubscriptionForm)
	g.GET("/invoices/:id/pdf", h.InvoicePDF)

	g.POST("/new", h.CreateSubscription)
	g.POST("/subscriptions/:id/approve", h.ApproveSubscription)
	g.POST("/subscriptions/:id/notes", h.SaveNotes)
	g.POST("/subscriptions/:id/resend", h.ResendKey)
	g.POST("/invoices/:id/remind", h.SendReminder)
	g.POST("/subscriptions/:id/cancel", h.CancelSubscription)
	g.POST("/subscriptions/:id/seat", h.IssueSeat)
	g.POST("/subscriptions/:id/portal", h.CreatePortalLink)
	g.POST("/subscriptions/:id/revoke", h.RevokeLicence)
}

// setCSRF hands out the double-submit token. SameSite=Strict on top, so the cookie
// is not even sent on a cross-site POST — belt and braces, because this panel can
// sign licences.
func setCSRF(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if ck, err := c.Cookie(csrfCookie); err == nil && ck.Value != "" {
			c.Set("csrf", ck.Value)
			return next(c)
		}
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return err
		}
		tok := hex.EncodeToString(b)
		c.SetCookie(&http.Cookie{
			Name: csrfCookie, Value: tok, Path: "/",
			HttpOnly: false, // the form has to read it
			SameSite: http.SameSiteStrictMode,
			Secure:   c.Scheme() == "https",
		})
		c.Set("csrf", tok)
		return next(c)
	}
}

// ── shared row types ─────────────────────────────────────────────────────────

type subRow struct {
	ID           string
	Company      string
	Email        string
	Plan         string
	Quantity     int
	SeatsUsed    int
	Status       string // bezahlt | läuft aus | abgelaufen | gekündigt | angefragt
	NextInvoice  string
	OpenInvoices int
	MRRCents     int64
	Notes        string
}

type invoiceRow struct {
	SubID     string
	Company   string
	LexwareID string
	Period    string
	Amount    string
	Paid      bool
	PaidOn    string
	Overdue   bool
	Reminded  string
}

type licenceRow struct {
	SubID    string
	Company  string
	OrgName  string
	Kind     string
	Status   string
	Expires  string
	Expired  bool
	LastSeen string
	Revoked  bool
	Key      string
	Token    string
	Note     string
}

// loadSubs is the one query behind the overview and the subscription list. One
// query, one shape: a second one would drift, and the number on the dashboard would
// stop matching the list you get when you click through to it.
func (h *Handler) loadSubs(c echo.Context) ([]subRow, error) {
	rows, err := h.db.Query(c.Request().Context(), `
		SELECT s.id, s.company_name, s.email, s.product, s.interval, s.quantity, s.status,
		       s.next_invoice_at, s.cancelled_at, s.notes,
		       (SELECT count(*) FROM billing_invoices bi
		         WHERE bi.subscription_id = s.id AND bi.status = 'open'),
		       -- DISTINCT: a normal customer gets TWO keys for ONE organisation (the
		       -- 45-day trial with the invoice, the full one on payment). Counting rows
		       -- would show them as "2 / 1 seats used".
		       (SELECT count(DISTINCT bl.org_name) FROM billing_licenses bl
		         WHERE bl.subscription_id = s.id)
		  FROM billing_quote_requests s
		 ORDER BY s.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []subRow
	for rows.Next() {
		var r subRow
		var product, interval string
		var next, cancelled *time.Time
		if err := rows.Scan(&r.ID, &r.Company, &r.Email, &product, &interval, &r.Quantity,
			&r.Status, &next, &cancelled, &r.Notes, &r.OpenInvoices, &r.SeatsUsed); err != nil {
			return nil, err
		}
		r.Plan = product + "/" + interval
		r.NextInvoice = dayp(next)

		switch {
		case cancelled != nil:
			r.Status = string(lexware.StatusCancelled)
		case r.Status == "requested":
			r.Status = "angefragt"
		case r.Status == "paid":
			if r.OpenInvoices > 0 {
				r.Status = string(lexware.StatusExpiring)
			} else {
				r.Status = string(lexware.StatusPaid)
			}
			// "läuft aus" zählt MIT: Der Kunde ist noch in dem Zeitraum, den er bezahlt
			// hat — nur seine Verlängerung steht aus. Ihn aus dem MRR zu nehmen, sobald
			// die Folgerechnung raus ist, würde die Zahl jeden Monat kurz einbrechen
			// lassen, ohne dass sich irgendetwas geändert hätte.
			if p, err := lexware.PlanFor(product, interval); err == nil {
				cents := p.TotalCents(r.Quantity)
				if interval == "year" {
					cents /= 12 // auf den Monat normalisiert, sonst läse ein Jahresplan als 2.990 € MRR
				}
				r.MRRCents = cents
			}
		case r.Status == "approved":
			r.Status = "Rechnung raus"
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Übersicht ────────────────────────────────────────────────────────────────

type dashboardData struct {
	page
	MRR      string
	Active   []subRow
	Pending  []subRow
	Warnings []string
}

func (h *Handler) Dashboard(c echo.Context) error {
	subs, err := h.loadSubs(c)
	if err != nil {
		return err
	}
	d := dashboardData{page: h.chrome(c, "Übersicht", "overview")}

	var mrr int64
	for _, s := range subs {
		mrr += s.MRRCents
		switch s.Status {
		case "angefragt":
			d.Pending = append(d.Pending, s)
		case string(lexware.StatusPaid), string(lexware.StatusExpiring):
			d.Active = append(d.Active, s)
		}

		// Loud on the two states that cost real money if nobody looks.
		if s.Status == "Rechnung raus" && s.OpenInvoices > 0 {
			d.Warnings = append(d.Warnings, fmt.Sprintf(
				"%s — Rechnung ist raus, Geld ist nicht da. Der 45-Tage-Schlüssel läuft irgendwann aus.", s.Company))
		}
		if s.Quantity > s.SeatsUsed && s.Status == string(lexware.StatusPaid) {
			d.Warnings = append(d.Warnings, fmt.Sprintf(
				"%s — %d von %d Plätzen vergeben. %d Lizenzen sind bezahlt, aber nie ausgestellt worden.",
				s.Company, s.SeatsUsed, s.Quantity, s.Quantity-s.SeatsUsed))
		}
	}
	// Ein Storno, der bereits einen Lizenzschluessel produziert hat, gehoert auf die
	// ERSTE Seite — nicht auf eine, die man vielleicht nie oeffnet. Der Abgleich laeuft
	// ohnehin alle 6 h im Hintergrund; hier wird nur gelesen, was er zuletzt fand.
	if h.billing != nil {
		if drifts, err := h.billing.Reconcile(c.Request().Context()); err == nil {
			for _, x := range drifts {
				if x.Severe {
					d.Warnings = append(d.Warnings, fmt.Sprintf(
						"Lexware-Abgleich: %s (%s, %s) — %s", x.Kind, x.Company, x.Amount,
						"siehe „Abgleich“, das braucht eine Entscheidung."))
				}
			}
		}
	}

	d.MRR = eur(mrr)
	return c.Render(http.StatusOK, "dashboard.html", d)
}

// ── Abos ─────────────────────────────────────────────────────────────────────

type listData struct {
	page
	Q    string
	Subs []subRow
}

func (h *Handler) Subscriptions(c echo.Context) error {
	subs, err := h.loadSubs(c)
	if err != nil {
		return err
	}
	d := listData{page: h.chrome(c, "Abos", "subs"), Q: strings.TrimSpace(c.QueryParam("q"))}
	for _, s := range subs {
		if d.Q == "" || matches(d.Q, s.Company, s.Email, s.Plan) {
			d.Subs = append(d.Subs, s)
		}
	}
	return c.Render(http.StatusOK, "subscriptions.html", d)
}

// ── Rechnungen ───────────────────────────────────────────────────────────────

type invoicesData struct {
	page
	Q        string
	Open     []invoiceRow
	Paid     []invoiceRow
	OpenSum  string
	PaidSum  string
	Overdues int
}

func (h *Handler) Invoices(c echo.Context) error {
	rows, err := h.db.Query(c.Request().Context(), `
		SELECT i.subscription_id, s.company_name, i.lexware_invoice_id,
		       i.period_start, i.period_end, i.net_amount_cents, i.status, i.paid_at,
		       i.created_at, i.reminded_at
		  FROM billing_invoices i
		  JOIN billing_quote_requests s ON s.id = i.subscription_id
		 WHERE i.status <> 'voided'
		 ORDER BY i.created_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	d := invoicesData{page: h.chrome(c, "Rechnungen", "invoices"), Q: strings.TrimSpace(c.QueryParam("q"))}
	var openSum, paidSum int64

	for rows.Next() {
		var r invoiceRow
		var from, to, created time.Time
		var cents int64
		var status string
		var paid, reminded *time.Time
		if err := rows.Scan(&r.SubID, &r.Company, &r.LexwareID, &from, &to, &cents,
			&status, &paid, &created, &reminded); err != nil {
			return err
		}
		r.Period = day(from) + " – " + day(to)
		r.Amount = eur(cents)
		r.Paid = status == "paid"
		r.PaidOn = dayp(paid)
		r.Reminded = dayp(reminded)

		if d.Q != "" && !matches(d.Q, r.Company, r.LexwareID) {
			continue
		}
		if r.Paid {
			paidSum += cents
			d.Paid = append(d.Paid, r)
		} else {
			// Overdue = the invoice's own payment term has run out. 14 days is the
			// longest term the plans print; older than that is genuinely late,
			// not merely "recent".
			r.Overdue = time.Since(created) > 14*24*time.Hour
			if r.Overdue {
				d.Overdues++
			}
			openSum += cents
			d.Open = append(d.Open, r)
		}
	}
	d.OpenSum, d.PaidSum = eur(openSum), eur(paidSum)
	return c.Render(http.StatusOK, "invoices.html", d)
}

// ── Lizenzen ─────────────────────────────────────────────────────────────────

type licencesData struct {
	page
	Q        string
	Licences []licenceRow
}

// Licences lists every key ever issued, searchable.
//
// This is the screen for "customer X is on the phone, which key do they have?".
// Before the ledger existed, the only copy of a customer's key was the mail we sent
// them — if they lost it, so had we.
func (h *Handler) Licences(c echo.Context) error {
	rows, err := h.db.Query(c.Request().Context(), `
		SELECT l.subscription_id, s.company_name, l.org_name, l.kind, l.license_key,
		       l.expires_at, l.last_seen_at, l.revoked_at, l.renewal_token, l.note
		  FROM billing_licenses l
		  JOIN billing_quote_requests s ON s.id = l.subscription_id
		 ORDER BY l.created_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	d := licencesData{page: h.chrome(c, "Lizenzen", "licences"), Q: strings.TrimSpace(c.QueryParam("q"))}
	for rows.Next() {
		var r licenceRow
		var expires time.Time
		var lastSeen, revoked *time.Time
		if err := rows.Scan(&r.SubID, &r.Company, &r.OrgName, &r.Kind, &r.Key,
			&expires, &lastSeen, &revoked, &r.Token, &r.Note); err != nil {
			return err
		}
		r.Expires = day(expires)
		r.Expired = expires.Before(time.Now())
		r.Revoked = revoked != nil
		// "Nie gesehen" is NOT "unused". The instance only reports in when the key is
		// close to expiry and auto-renewal is on — saying "unused" would be a claim we
		// cannot support and the customer cannot disprove.
		if lastSeen != nil {
			r.LastSeen = day(*lastSeen)
		} else {
			r.LastSeen = "—"
		}
		if st, _, err := lexware.LicenceStatus(c.Request().Context(), h.db, r.Token); err == nil {
			r.Status = string(st)
		}
		if d.Q == "" || matches(d.Q, r.Company, r.OrgName, r.Key) {
			d.Licences = append(d.Licences, r)
		}
	}
	return c.Render(http.StatusOK, "licences.html", d)
}

// ── Abo-Detail ───────────────────────────────────────────────────────────────

type subDetail struct {
	page
	Sub       subRow
	Invoices  []invoiceRow
	Licences  []licenceRow
	SeatsLeft int
}

func (h *Handler) Subscription(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")

	subs, err := h.loadSubs(c)
	if err != nil {
		return err
	}
	d := subDetail{page: h.chrome(c, "Abo", "subs")}
	found := false
	for _, s := range subs {
		if s.ID == id {
			d.Sub, found = s, true
			break
		}
	}
	if !found {
		return c.String(http.StatusNotFound, "Abo nicht gefunden")
	}
	d.Title = d.Sub.Company

	rows, err := h.db.Query(ctx, `
		SELECT lexware_invoice_id, period_start, period_end, net_amount_cents, status,
		       paid_at, created_at, reminded_at
		  FROM billing_invoices
		 WHERE subscription_id = $1 AND status <> 'voided'
		 ORDER BY period_start DESC`, id)
	if err != nil {
		return err
	}
	for rows.Next() {
		var r invoiceRow
		var from, to, created time.Time
		var cents int64
		var status string
		var paid, reminded *time.Time
		if err := rows.Scan(&r.LexwareID, &from, &to, &cents, &status, &paid, &created, &reminded); err != nil {
			continue
		}
		r.Period, r.Amount = day(from)+" – "+day(to), eur(cents)
		r.Paid, r.PaidOn = status == "paid", dayp(paid)
		r.Reminded = dayp(reminded)
		r.Overdue = !r.Paid && time.Since(created) > 14*24*time.Hour
		d.Invoices = append(d.Invoices, r)
	}
	rows.Close()

	if st, err := h.seats.State(ctx, id); err == nil {
		d.SeatsLeft = st.Free
		for _, l := range st.Licences {
			r := licenceRow{
				OrgName: l.OrgName, Kind: l.Kind, Key: l.Key, Note: l.Note,
				Token: l.RenewalToken, Revoked: l.Revoked, Status: string(l.Status),
				Expires: day(l.ExpiresAt), Expired: l.ExpiresAt.Before(time.Now()),
				LastSeen: dayp(l.LastSeen),
			}
			d.Licences = append(d.Licences, r)
		}
	}
	return c.Render(http.StatusOK, "subscription.html", d)
}

// ── Aktionen ─────────────────────────────────────────────────────────────────

func (h *Handler) CancelSubscription(c echo.Context) error {
	id := c.Param("id")
	if err := lexware.Cancel(c.Request().Context(), h.db, id); err != nil {
		return redirect(c, id, "Konnte nicht kündigen: "+err.Error())
	}
	log.Warn().Str("subscription_id", id).Str("by", requestEmail(c)).
		Msg("billing admin: subscription cancelled")
	return redirect(c, id, "Gekündigt. Keine weiteren Rechnungen; der aktuelle Schlüssel läuft von allein aus.")
}

func (h *Handler) IssueSeat(c echo.Context) error {
	id := c.Param("id")
	lic, err := h.seats.Issue(c.Request().Context(), id,
		strings.TrimSpace(c.FormValue("org_name")), strings.TrimSpace(c.FormValue("email")), requestEmail(c))
	switch {
	case err != nil && lic == nil:
		return redirect(c, id, seatErr(err))
	case err != nil:
		return redirect(c, id, "Schlüssel für „"+lic.OrgName+"“ ausgestellt, aber die Mail ist fehlgeschlagen. Er steht unten in der Liste.")
	}
	return redirect(c, id, "Schlüssel für „"+lic.OrgName+"“ ausgestellt und verschickt.")
}

// CreatePortalLink mints (or rotates) the customer's self-service link. The token is
// shown exactly ONCE, here — only its hash is stored, so a leaked database backup
// cannot hand anyone the portal. Rotating invalidates the old link.
func (h *Handler) CreatePortalLink(c echo.Context) error {
	id := c.Param("id")
	link, err := portal.NewPortalToken(c.Request().Context(), h.db, id, h.baseURL)
	if err != nil {
		return redirect(c, id, "Link konnte nicht erzeugt werden.")
	}
	log.Warn().Str("subscription_id", id).Str("by", requestEmail(c)).
		Msg("billing admin: portal link created/rotated — the old one is now invalid")
	return redirect(c, id, "Neuer Portal-Link (wird NUR JETZT angezeigt, danach nie wieder): "+link)
}

// RevokeLicence stops renewals for one key. It is not a kill switch and the UI says
// so: the signed key stays valid until it expires. A self-hosted instance cannot be
// reached — the same property that makes the product sellable at all.
func (h *Handler) RevokeLicence(c echo.Context) error {
	id := c.Param("id")
	if err := h.seats.Revoke(c.Request().Context(), c.FormValue("token"), requestEmail(c)); err != nil {
		return redirect(c, id, "Konnte nicht sperren: "+err.Error())
	}
	return redirect(c, id,
		"Gesperrt. Der Schlüssel wird nicht mehr erneuert und läuft aus — sofort abschalten geht nicht, dafür bräuchte Vakt Phone-Home.")
}

func redirect(c echo.Context, id, flash string) error {
	return c.Redirect(http.StatusSeeOther, "/subscriptions/"+id+"?flash="+urlq(flash))
}

func seatErr(err error) string {
	switch err {
	case lexware.ErrNoSeatsLeft:
		return "Alle Plätze sind vergeben. Für einen weiteren muss der Kunde aufstocken."
	case lexware.ErrOrgNameRequired:
		return "Bitte den Namen der Organisation angeben — er wird in den Schlüssel signiert."
	case lexware.ErrCancelled:
		return "Das Abo ist gekündigt. Kein neuer Schlüssel."
	case lexware.ErrNoSuchSubscription:
		return "Abo nicht gefunden oder noch nicht bezahlt."
	default:
		log.Error().Err(err).Msg("billing admin: seat issue failed")
		return "Das hat nicht geklappt — siehe Log."
	}
}

// matches is the search. Case-insensitive substring across the fields a human would
// actually type: a company name, an e-mail, an invoice number, a licence key.
func matches(q string, fields ...string) bool {
	q = strings.ToLower(q)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

func csrfOf(c echo.Context) string {
	v, _ := c.Get("csrf").(string)
	return v
}

// ── Abgleich mit Lexware ─────────────────────────────────────────────────────

type lexwareData struct {
	page
	Drifts []lexware.Drift
	Severe int
	Err    string
}

// LexwareCheck asks Lexware what it thinks and shows where the two disagree.
//
// Vakt LISTENS for payments. It does not hear a storno — cancelling an invoice in
// Lexware raises no payment event. So an invoice we booked as paid, for which a
// licence key was already signed and mailed, can quietly become void and nothing in
// our database would notice. That is not fixable by listening harder; it needs asking.
func (h *Handler) LexwareCheck(c echo.Context) error {
	d := lexwareData{page: h.chrome(c, "Abgleich mit Lexware", "lexware")}
	if h.billing == nil {
		d.Err = "Lexware ist auf dieser Instanz nicht konfiguriert."
		return c.Render(http.StatusOK, "lexware.html", d)
	}
	drifts, err := h.billing.Reconcile(c.Request().Context())
	if err != nil {
		// Never the raw error: it can carry the API key in a URL or internal paths.
		log.Error().Err(err).Msg("billing admin: Lexware reconciliation failed")
		d.Err = "Lexware antwortet nicht. Siehe Log."
		return c.Render(http.StatusOK, "lexware.html", d)
	}
	d.Drifts = drifts
	for _, x := range drifts {
		if x.Severe {
			d.Severe++
		}
	}
	return c.Render(http.StatusOK, "lexware.html", d)
}

// ── Freigeben ────────────────────────────────────────────────────────────────

// ApproveSubscription turns a request into a finalised invoice + 45-day key, from the
// panel.
//
// I originally left this out on purpose: "a second way to mint a finalised invoice
// under your tax number is attack surface without a benefit". That was before
// Cloudflare Access. The e-mail link is now the WEAKER path — a mail can be forwarded,
// a mailbox can be taken over — while this panel sits behind edge authentication plus
// its own JWT check, with exactly one address allowed. The reasoning changed because
// the facts did.
//
// Both paths call the SAME ApproveRequest. Two implementations would drift, and the way
// they would drift is that one forgets the licence row — after which the customer holds
// a key that can never auto-renew, and nothing can fix it retroactively.
func (h *Handler) ApproveSubscription(c echo.Context) error {
	id := c.Param("id")
	if h.billing == nil {
		return redirect(c, id, "Billing ist auf dieser Instanz nicht konfiguriert.")
	}
	res := h.billing.ApproveRequest(c.Request().Context(), id, requestEmail(c))
	log.Warn().Str("subscription_id", id).Str("by", requestEmail(c)).Bool("ok", res.OK).
		Msg("billing admin: invoice approved from the panel")
	return redirect(c, id, res.Message)
}

// ── Abo von Hand anlegen ─────────────────────────────────────────────────────

type newSubData struct {
	page
	Err string
}

func (h *Handler) NewSubscriptionForm(c echo.Context) error {
	return c.Render(http.StatusOK, "new.html", newSubData{page: h.chrome(c, "Neues Abo", "subs")})
}

// CreateSubscription records a customer who phoned instead of using the form.
//
// Without it, such a sale can only be raised directly in Lexware — and then Vakt does
// not know it exists. No renewal, no key, and it turns up in the reconciliation as
// "nur in Lexware" while every number in this panel is quietly wrong.
func (h *Handler) CreateSubscription(c echo.Context) error {
	if h.billing == nil {
		return c.Redirect(http.StatusSeeOther, "/new?flash="+urlq("Billing ist nicht konfiguriert."))
	}
	qty, _ := strconv.Atoi(c.FormValue("quantity"))
	id, err := h.billing.CreateSubscription(c.Request().Context(), lexware.NewSubscription{
		CompanyName: c.FormValue("company_name"),
		ContactName: c.FormValue("contact_name"),
		Email:       c.FormValue("email"),
		VATID:       c.FormValue("vat_id"),
		Street:      c.FormValue("street"),
		Zip:         c.FormValue("zip"),
		City:        c.FormValue("city"),
		CountryCode: c.FormValue("country_code"),
		Product:     "pro",
		Interval:    c.FormValue("interval"),
		Quantity:    qty,
		Notes:       c.FormValue("notes"),
	}, requestEmail(c))
	if err != nil {
		return c.Redirect(http.StatusSeeOther, "/new?flash="+urlq(err.Error()))
	}
	return redirect(c, id,
		"Abo angelegt. Es steht auf „angefragt“ — es ist noch KEINE Rechnung erstellt. "+
			"Mit „Freigeben“ geht die finalisierte Rechnung samt 45-Tage-Schlüssel raus.")
}

// ── Notizen, Erinnerung, Schlüssel erneut ────────────────────────────────────

func (h *Handler) SaveNotes(c echo.Context) error {
	id := c.Param("id")
	if h.billing == nil {
		return redirect(c, id, "nicht konfiguriert")
	}
	if err := h.billing.SetNotes(c.Request().Context(), id, c.FormValue("notes")); err != nil {
		return redirect(c, id, "Notiz konnte nicht gespeichert werden.")
	}
	return redirect(c, id, "Notiz gespeichert.")
}

func (h *Handler) SendReminder(c echo.Context) error {
	invoiceID := c.Param("id")
	sub := c.FormValue("sub")
	if h.billing == nil {
		return c.Redirect(http.StatusSeeOther, "/invoices")
	}
	if err := h.billing.SendReminder(c.Request().Context(), invoiceID, requestEmail(c)); err != nil {
		return redirect(c, sub, "Erinnerung nicht verschickt: "+err.Error())
	}
	return redirect(c, sub, "Zahlungserinnerung verschickt.")
}

// ResendKey mails an EXISTING key again — the customer deleted the mail. Nothing is
// signed anew: the same key, so a re-send can never quietly change what they hold.
func (h *Handler) ResendKey(c echo.Context) error {
	id := c.Param("id")
	if h.billing == nil {
		return redirect(c, id, "nicht konfiguriert")
	}
	if err := h.billing.ResendKey(c.Request().Context(), c.FormValue("token"),
		strings.TrimSpace(c.FormValue("email")), requestEmail(c)); err != nil {
		return redirect(c, id, "Konnte nicht verschickt werden: "+err.Error())
	}
	return redirect(c, id, "Schlüssel erneut verschickt — derselbe wie zuvor, es wurde nichts neu ausgestellt.")
}

// InvoicePDF streams the invoice straight from Lexware, so looking at a bill does not
// mean opening a second application.
func (h *Handler) InvoicePDF(c echo.Context) error {
	if h.billing == nil {
		return c.String(http.StatusServiceUnavailable, "nicht konfiguriert")
	}
	pdf, err := h.billing.InvoicePDF(c.Request().Context(), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("billing admin: invoice pdf")
		return c.String(http.StatusBadGateway, "Lexware liefert das PDF nicht.")
	}
	c.Response().Header().Set("Content-Disposition", `inline; filename="Rechnung.pdf"`)
	return c.Blob(http.StatusOK, "application/pdf", pdf)
}
