//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
)

// TestPolicyAcceptanceCampaign_OrgScope is the regression guard for R1-16-V1 /
// R1-24-RT02 (CRITICAL, CROSS_ORG_LEAK).
//
// GetCKPolicyAcceptanceCampaignStats and ListCKPolicyAcceptanceRequests were
// scoped only by `WHERE campaign_id = $1::uuid` — no org_id predicate on any
// layer. Two lint gates allow-listed them with the rationale "caller verifies
// campaign org ownership", but NO such caller existed: an org-2 viewer who put
// an org-1 campaign_id in the URL read org-1's stats and, worse, the per-request
// list including recipient_email + recipient_name (employee PII), byte-identical
// to what the owner saw. The identical vaktaware twin was already fixed as
// R-H25 (S131-C4); this one stayed open.
//
// The fix adds `AND org_id = $2::uuid` to both queries (the requests table owns
// org_id, migration 068) and threads orgID from the handler context through the
// service and repository. This test drives the repository with TWO real orgs
// against live Postgres and proves org B cannot read org A's campaign — the
// stats collapse to zero and the request list is empty, and specifically org A's
// recipient_email does NOT appear in org B's response.
//
// Non-vacuity: reverting the org_id predicate turns the cross-org assertions red
// (org B would see total=1 and org A's recipient_email).
func TestPolicyAcceptanceCampaign_OrgScope(t *testing.T) {
	pool, cleanup := bootPostgres(t)
	defer cleanup()
	ctx := context.Background()

	repo := policy.NewRepository(pool)

	// Two independent orgs.
	newOrg := func(slug string) string {
		var orgID string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO organizations (name, slug) VALUES ($1, $1) RETURNING id::text`, slug).Scan(&orgID))
		return orgID
	}
	orgA := newOrg("pac-a")
	orgB := newOrg("pac-b")

	// Seed org A: policy → campaign → one accepted + one pending request, the
	// pending one carrying a distinctive recipient_email that must never leak.
	const secretEmail = "victim@org-a.example"
	seedCampaign := func(orgID, slug, recipientEmail string, accepted bool) string {
		var policyID string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO ck_policies (org_id, title) VALUES ($1::uuid, $2)
			 RETURNING id::text`, orgID, slug+"-policy").Scan(&policyID))
		var campaignID string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO ck_policy_acceptance_campaigns (org_id, policy_id, name)
			 VALUES ($1::uuid, $2::uuid, $3) RETURNING id::text`, orgID, policyID, slug+"-camp").Scan(&campaignID))
		acceptedAt := "NULL"
		if accepted {
			acceptedAt = "now()"
		}
		// pending request with the sensitive email
		_, err := pool.Exec(ctx,
			`INSERT INTO ck_policy_acceptance_requests
			   (campaign_id, org_id, recipient_email, recipient_name, token_hash, accepted_at)
			 VALUES ($1::uuid, $2::uuid, $3, 'Victim Name', $4, `+acceptedAt+`)`,
			campaignID, orgID, recipientEmail, slug+"-token-1")
		require.NoError(t, err)
		return campaignID
	}

	campaignA := seedCampaign(orgA, "a", secretEmail, false)
	// Give org B its own campaign so the tables are non-empty for both orgs.
	seedCampaign(orgB, "b", "someone@org-b.example", true)

	t.Run("owner_sees_own_stats", func(t *testing.T) {
		stats, err := repo.GetCampaignStats(ctx, orgA, campaignA)
		require.NoError(t, err)
		assert.Equal(t, 1, stats.Total, "org A must see its own campaign's single request")
		assert.Equal(t, 1, stats.Pending)
	})

	t.Run("owner_sees_own_requests", func(t *testing.T) {
		reqs, err := repo.ListAcceptanceRequests(ctx, orgA, campaignA)
		require.NoError(t, err)
		require.Len(t, reqs, 1)
		assert.Equal(t, secretEmail, reqs[0].RecipientEmail)
	})

	t.Run("crossOrg_stats_are_zero", func(t *testing.T) {
		stats, err := repo.GetCampaignStats(ctx, orgB, campaignA)
		require.NoError(t, err)
		assert.Equal(t, 0, stats.Total, "org B must NOT see org A's campaign stats")
		assert.Equal(t, 0, stats.Accepted)
		assert.Equal(t, 0, stats.Pending)
	})

	t.Run("crossOrg_requests_are_empty_and_no_pii_leaks", func(t *testing.T) {
		reqs, err := repo.ListAcceptanceRequests(ctx, orgB, campaignA)
		require.NoError(t, err)
		assert.Empty(t, reqs, "org B must NOT read org A's per-recipient requests")
		for _, r := range reqs {
			assert.NotEqual(t, secretEmail, r.RecipientEmail,
				"org A's recipient PII must never appear in org B's response")
		}
	})
}
