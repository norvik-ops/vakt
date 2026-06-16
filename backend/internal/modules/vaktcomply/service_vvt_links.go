// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S88-9: VVT→ISO/TOM-Control-Verknüpfung. Links a Vakt Privacy processing
// activity (Art. 30 VVT) to one or more compliance controls so an auditor sees
// "this processing is covered by control X" — and the reverse. vvt_id is an
// opaque key (no import of the vaktprivacy package, no cross-schema read).

package vaktcomply

import (
	"context"
	"fmt"
	"time"
)

// VVTControlLink is an n:m link between a VVT entry and a control.
type VVTControlLink struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	VVTID     string    `json:"vvt_id"`
	VVTName   string    `json:"vvt_name"`
	ControlID string    `json:"control_id"`
	CreatedAt time.Time `json:"created_at"`
}

// LinkVVTToControlInput is the validated create payload.
type LinkVVTToControlInput struct {
	VVTID     string `json:"vvt_id" validate:"required"`
	VVTName   string `json:"vvt_name" validate:"max=255"`
	ControlID string `json:"control_id" validate:"required,uuid"`
}

// LinkVVTToControl creates an idempotent link (org-scoped). Verifies the control
// belongs to the org before linking.
func (s *Service) LinkVVTToControl(ctx context.Context, orgID string, in LinkVVTToControlInput) (*VVTControlLink, error) {
	var owns bool
	if err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM ck_controls WHERE id=$1::uuid AND org_id=$2)`,
		in.ControlID, orgID).Scan(&owns); err != nil {
		return nil, fmt.Errorf("verify control: %w", err)
	}
	if !owns {
		return nil, fmt.Errorf("control not found")
	}
	var l VVTControlLink
	err := s.db.QueryRow(ctx, `
		INSERT INTO ck_vvt_control_links (org_id, vvt_id, vvt_name, control_id)
		VALUES ($1, $2, $3, $4::uuid)
		ON CONFLICT (org_id, vvt_id, control_id) DO UPDATE SET vvt_name = EXCLUDED.vvt_name
		RETURNING id::text, org_id::text, vvt_id, vvt_name, control_id::text, created_at`,
		orgID, in.VVTID, in.VVTName, in.ControlID).
		Scan(&l.ID, &l.OrgID, &l.VVTID, &l.VVTName, &l.ControlID, &l.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("link vvt to control: %w", err)
	}
	return &l, nil
}

// UnlinkVVTFromControl removes a link by id (org-scoped).
func (s *Service) UnlinkVVTFromControl(ctx context.Context, orgID, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM ck_vvt_control_links WHERE id=$1::uuid AND org_id=$2`, id, orgID)
	if err != nil {
		return fmt.Errorf("unlink vvt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("link not found")
	}
	return nil
}

// ListLinksForControl returns the VVT links attached to a control (reverse view).
func (s *Service) ListLinksForControl(ctx context.Context, orgID, controlID string) ([]VVTControlLink, error) {
	return s.queryVVTLinks(ctx,
		`WHERE org_id=$1 AND control_id=$2::uuid ORDER BY created_at DESC`, orgID, controlID)
}

// ListLinksForVVT returns the controls linked to a VVT entry.
func (s *Service) ListLinksForVVT(ctx context.Context, orgID, vvtID string) ([]VVTControlLink, error) {
	return s.queryVVTLinks(ctx,
		`WHERE org_id=$1 AND vvt_id=$2 ORDER BY created_at DESC`, orgID, vvtID)
}

func (s *Service) queryVVTLinks(ctx context.Context, where string, args ...any) ([]VVTControlLink, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id::text, org_id::text, vvt_id, vvt_name, control_id::text, created_at
		 FROM ck_vvt_control_links `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("list vvt links: %w", err)
	}
	defer rows.Close()
	out := []VVTControlLink{}
	for rows.Next() {
		var l VVTControlLink
		if err := rows.Scan(&l.ID, &l.OrgID, &l.VVTID, &l.VVTName, &l.ControlID, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan vvt link: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
