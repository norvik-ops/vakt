// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"context"
	"fmt"
)

// GetSoAEntries returns all controls for the org's frameworks with SoA metadata.
func (s *Service) GetSoAEntries(ctx context.Context, orgID string) ([]SoAEntry, error) {
	rows, err := s.db.Query(ctx, `
		SELECT
			c.id::text,
			f.name AS framework_name,
			c.domain,
			c.title,
			COALESCE(c.soa_applicable, true)         AS applicable,
			COALESCE(c.manual_status, 'not_started') AS status,
			COALESCE(c.soa_justification_yes, '')    AS just_yes,
			COALESCE(c.soa_justification_no, '')     AS just_no
		FROM ck_controls c
		JOIN ck_frameworks f ON f.id = c.framework_id AND f.org_id = c.org_id
		WHERE c.org_id = $1::uuid
		ORDER BY f.name, c.domain, c.title
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("get soa entries: %w", err)
	}
	defer rows.Close()

	var entries []SoAEntry
	for rows.Next() {
		var e SoAEntry
		if err := rows.Scan(
			&e.ControlID, &e.FrameworkName, &e.Domain, &e.Title,
			&e.Applicable, &e.Status,
			&e.JustificationApplicable, &e.JustificationNotApplicable,
		); err != nil {
			return nil, fmt.Errorf("scan soa entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// UpdateSoAApplicability sets the applicability and justification for a control.
func (s *Service) UpdateSoAApplicability(ctx context.Context, orgID, controlID string, applicable bool, justYes, justNo string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE ck_controls
		SET soa_applicable          = $1,
		    soa_justification_yes   = $2,
		    soa_justification_no    = $3
		WHERE id = $4::uuid AND org_id = $5::uuid
	`, applicable, justYes, justNo, controlID, orgID)
	return err
}
