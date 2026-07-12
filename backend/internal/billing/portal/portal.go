// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

// Package portal is the MSP self-service page.
//
// An MSP that bought ten seats should not have to mail us every time it onboards a
// client. It gets a link: see the seats, name the new client, get the key.
//
// It is PUBLIC — an MSP has no Vakt account with us — and guarded solely by a
// 32-byte token that is stored only as a SHA-256 hash. That is deliberate and it is
// bounded: what a stolen portal token can do is burn seats the MSP has ALREADY PAID
// FOR, and nothing else. It cannot mint an eleventh key for ten seats, cannot touch
// another subscription, cannot read anything but this subscription's own seat list,
// and every key it issues is mailed to the MSP's registered address — so they notice.
//
// It is mounted on the billing service, which is the process that holds the signing
// key. That is uncomfortable, and it is why the cap, the hash, the rate limit and
// the notification all exist. A licence-signing oracle with no ceiling would be a
// catastrophe; one that can only spend what someone already bought is a support
// ticket.
package portal

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/billing/lexware"
)

//go:embed templates/*.html
var files embed.FS

type Handler struct {
	db      *pgxpool.Pool
	seats   *lexware.Seats
	tpl     *template.Template
	baseURL string
}

func NewHandler(db *pgxpool.Pool, seats *lexware.Seats, baseURL string) (*Handler, error) {
	t, err := template.ParseFS(files, "templates/portal.html")
	if err != nil {
		return nil, fmt.Errorf("portal: parse template: %w", err)
	}
	return &Handler{db: db, seats: seats, tpl: t, baseURL: baseURL}, nil
}

func Register(g *echo.Group, h *Handler) {
	// Rate limit per IP. The POST signs a licence; without a limit, a leaked token
	// could burn every remaining seat in a second, before anyone read the mail.
	lim := middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{Rate: 0.2, Burst: 10, ExpiresIn: 10 * time.Minute},
		),
	})
	g.GET("/billing/portal/:token", h.Show, lim)
	g.POST("/billing/portal/:token/seat", h.IssueSeat, lim)
}

// resolve maps a portal token to its subscription — constant time, hashed.
func (h *Handler) resolve(ctx context.Context, token string) (string, error) {
	if len(token) != 64 {
		return "", fmt.Errorf("portal: bad token")
	}
	sum := sha256.Sum256([]byte(token))
	want := hex.EncodeToString(sum[:])

	rows, err := h.db.Query(ctx, `
		SELECT id, portal_token_hash FROM billing_quote_requests
		 WHERE portal_token_hash IS NOT NULL AND status = 'paid' AND cancelled_at IS NULL`)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var found string
	for rows.Next() {
		var id, hash string
		if err := rows.Scan(&id, &hash); err != nil {
			continue
		}
		// Compare every candidate, do not break early: an early exit leaks, through
		// timing, roughly where in the table the match sits.
		if subtle.ConstantTimeCompare([]byte(hash), []byte(want)) == 1 {
			found = id
		}
	}
	if found == "" {
		return "", fmt.Errorf("portal: unknown token")
	}
	return found, nil
}

type pageData struct {
	S     *seatView
	Flash string
	Err   string
	Token string
	Base  string // the pretty portal host, so the form posts where the human is
}

type seatView struct {
	Company  string
	Plan     string
	Quantity int
	Used     int
	Free     int
	Licences []licenceView
}

type licenceView struct {
	OrgName  string
	Status   string
	Expires  string
	Expired  bool
	LastSeen string
	Revoked  bool
	Key      string
	Token    string
}

func (h *Handler) view(ctx context.Context, subID string) (*seatView, error) {
	st, err := h.seats.State(ctx, subID)
	if err != nil {
		return nil, err
	}
	v := &seatView{Company: st.Company, Plan: st.Plan, Quantity: st.Quantity,
		Used: st.Used, Free: st.Free}
	for _, l := range st.Licences {
		lv := licenceView{
			OrgName: l.OrgName, Key: l.Key, Token: l.RenewalToken,
			Status:  string(l.Status),
			Expires: l.ExpiresAt.Format("02.01.2006"),
			Expired: l.ExpiresAt.Before(time.Now()),
			Revoked: l.Revoked,
		}
		if l.LastSeen != nil {
			lv.LastSeen = l.LastSeen.Format("02.01.2006")
		} else {
			// Not "never used" — "never told us". The instance only phones home if the
			// customer opted in with VAKT_LICENSE_TOKEN. Saying "unused" here would be
			// a lie the customer could not disprove.
			lv.LastSeen = "keine Rückmeldung"
		}
		v.Licences = append(v.Licences, lv)
	}
	return v, nil
}

func (h *Handler) Show(c echo.Context) error {
	token := c.Param("token")
	subID, err := h.resolve(c.Request().Context(), token)
	if err != nil {
		return c.String(http.StatusNotFound, "Dieser Link ist ungültig oder gehört zu keinem aktiven Abo.")
	}
	v, err := h.view(c.Request().Context(), subID)
	if err != nil {
		return c.String(http.StatusNotFound, "Abo nicht gefunden.")
	}
	return h.tpl.Execute(c.Response(), pageData{
		S: v, Token: token, Base: strings.TrimRight(h.baseURL, "/"),
		Flash: c.QueryParam("flash"), Err: c.QueryParam("err"),
	})
}

func (h *Handler) IssueSeat(c echo.Context) error {
	ctx := c.Request().Context()
	token := c.Param("token")
	subID, err := h.resolve(ctx, token)
	if err != nil {
		return c.String(http.StatusNotFound, "Dieser Link ist ungültig oder gehört zu keinem aktiven Abo.")
	}

	orgName := c.FormValue("org_name")
	sendTo := c.FormValue("email")

	lic, err := h.seats.Issue(ctx, subID, orgName, sendTo, "MSP-Portal")
	if err != nil && lic == nil {
		log.Warn().Err(err).Str("subscription_id", subID).Msg("portal: seat issue refused")
		return c.Redirect(http.StatusSeeOther, h.link(token)+"?err="+urlq(msg(err)))
	}
	if err != nil {
		// Issued, but the mail failed. The key exists — show it rather than hide it.
		return c.Redirect(http.StatusSeeOther, h.link(token)+"?err="+
			urlq("Der Schlüssel für „"+orgName+"\" wurde ausgestellt, aber die E-Mail ist nicht rausgegangen. Er steht unten in der Liste."))
	}
	return c.Redirect(http.StatusSeeOther, h.link(token)+"?flash="+
		urlq("Schlüssel für „"+lic.OrgName+"\" ausgestellt und verschickt."))
}

// link is the URL a HUMAN gets. Caddy maps lizenz.norvikops.de/<token> onto the
// internal /api/v1/billing/portal/<token> route — the customer never sees that path,
// because "here is your licence portal: api.norvikops.de/api/v1/billing/portal/3f2a…"
// is not a link you send anybody.
func (h *Handler) link(token string) string {
	return strings.TrimRight(h.baseURL, "/") + "/" + token
}

func urlq(s string) string { return url.QueryEscape(s) }

func msg(err error) string {
	switch err {
	case lexware.ErrNoSeatsLeft:
		return "Alle Plätze sind vergeben. Für einen weiteren melde dich bei uns — wir stocken auf."
	case lexware.ErrOrgNameRequired:
		return "Bitte den Namen der Organisation angeben. Er wird in den Schlüssel signiert."
	case lexware.ErrCancelled:
		return "Das Abo ist gekündigt. Es werden keine Schlüssel mehr ausgestellt."
	default:
		// Never the raw error: it can carry SQL or internal paths.
		log.Error().Err(err).Msg("portal: seat issue failed")
		return "Das hat nicht geklappt. Schreib uns kurz — wir stellen den Schlüssel von Hand aus."
	}
}

// NewPortalToken mints a link for a subscription and stores only its hash.
// Calling it again rotates the link and invalidates the old one.
func NewPortalToken(ctx context.Context, db *pgxpool.Pool, subID, baseURL string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(token))
	if _, err := db.Exec(ctx,
		`UPDATE billing_quote_requests SET portal_token_hash = $2 WHERE id = $1`,
		subID, hex.EncodeToString(sum[:])); err != nil {
		return "", err
	}
	return strings.TrimRight(baseURL, "/") + "/" + token, nil
}
