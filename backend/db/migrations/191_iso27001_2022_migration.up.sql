-- S70-1: ISO 27001:2022 — Migrate 2013 control IDs to 2022 IDs
-- Based on ISO/IEC 27001:2022 Annex C official mapping table.
-- org-specific ck_controls rows that still carry 2013-style IDs or A2022-* prefixes
-- are updated to the canonical 2022 codes. Where multiple 2013 controls map to one
-- 2022 control, we UPDATE the first match and ignore subsequent ones (ON CONFLICT DO NOTHING
-- at re-seed time handles the rest).
DO $$
BEGIN
    -- A2022-* prefixed controls → canonical 2022 IDs
    UPDATE ck_controls SET control_id = 'A.5.7'  WHERE control_id = 'A2022-5.7';
    UPDATE ck_controls SET control_id = 'A.5.23' WHERE control_id = 'A2022-5.23';
    UPDATE ck_controls SET control_id = 'A.5.30' WHERE control_id = 'A2022-5.30';
    UPDATE ck_controls SET control_id = 'A.7.4'  WHERE control_id = 'A2022-7.4';
    UPDATE ck_controls SET control_id = 'A.8.9'  WHERE control_id = 'A2022-8.9';
    UPDATE ck_controls SET control_id = 'A.8.10' WHERE control_id = 'A2022-8.10';
    UPDATE ck_controls SET control_id = 'A.8.11' WHERE control_id = 'A2022-8.11';
    UPDATE ck_controls SET control_id = 'A.8.12' WHERE control_id = 'A2022-8.12';
    UPDATE ck_controls SET control_id = 'A.8.16' WHERE control_id = 'A2022-8.16';
    UPDATE ck_controls SET control_id = 'A.8.23' WHERE control_id = 'A2022-8.23';
    UPDATE ck_controls SET control_id = 'A.8.28' WHERE control_id = 'A2022-8.28';

    -- ISO 27001:2013 → 2022 mapping (Annex C)
    -- A.5 Policies (2013) → A.5.1 (2022)
    UPDATE ck_controls SET control_id = 'A.5.1' WHERE control_id IN ('A.5.1.1', 'A.5.1.2') AND control_id NOT IN (SELECT control_id FROM ck_controls WHERE control_id = 'A.5.1');
    -- A.6 Organisation (2013) → A.5.2, A.5.3, A.5.4, A.5.5, A.5.6, A.5.8
    UPDATE ck_controls SET control_id = 'A.5.2' WHERE control_id = 'A.6.1.1';
    UPDATE ck_controls SET control_id = 'A.5.3' WHERE control_id = 'A.6.1.2';
    UPDATE ck_controls SET control_id = 'A.5.5' WHERE control_id = 'A.6.1.3';
    UPDATE ck_controls SET control_id = 'A.5.8' WHERE control_id = 'A.6.1.5';
    -- A.7 HR (2013) → A.6.x (2022)
    UPDATE ck_controls SET control_id = 'A.6.1' WHERE control_id = 'A.7.1.1';
    UPDATE ck_controls SET control_id = 'A.6.2' WHERE control_id = 'A.7.1.2';
    UPDATE ck_controls SET control_id = 'A.5.4' WHERE control_id = 'A.7.2.1';
    UPDATE ck_controls SET control_id = 'A.6.3' WHERE control_id = 'A.7.2.2';
    UPDATE ck_controls SET control_id = 'A.6.4' WHERE control_id = 'A.7.2.3';
    UPDATE ck_controls SET control_id = 'A.6.5' WHERE control_id = 'A.7.3.1';
    -- A.8 Asset Management (2013) → A.5.9-A.5.14 (2022)
    UPDATE ck_controls SET control_id = 'A.5.9' WHERE control_id IN ('A.8.1.1', 'A.8.1.2');
    UPDATE ck_controls SET control_id = 'A.5.10' WHERE control_id = 'A.8.1.3';
    UPDATE ck_controls SET control_id = 'A.5.11' WHERE control_id = 'A.8.1.4';
    UPDATE ck_controls SET control_id = 'A.5.12' WHERE control_id = 'A.8.2';
    UPDATE ck_controls SET control_id = 'A.7.10' WHERE control_id = 'A.8.3';
    -- A.9 Access Control (2013) → A.5.15-A.5.18, A.8.2-A.8.5 (2022)
    UPDATE ck_controls SET control_id = 'A.5.15' WHERE control_id IN ('A.9.1', 'A.9.1.1', 'A.9.1.2');
    UPDATE ck_controls SET control_id = 'A.5.16' WHERE control_id = 'A.9.2';
    UPDATE ck_controls SET control_id = 'A.5.18' WHERE control_id IN ('A.9.2.1', 'A.9.2.2', 'A.9.2.5');
    UPDATE ck_controls SET control_id = 'A.8.2'  WHERE control_id = 'A.9.2.3';
    UPDATE ck_controls SET control_id = 'A.5.17' WHERE control_id IN ('A.9.4', 'A.9.4.3');
    UPDATE ck_controls SET control_id = 'A.8.3'  WHERE control_id = 'A.9.4.1';
    UPDATE ck_controls SET control_id = 'A.8.5'  WHERE control_id = 'A.9.4.2';
    UPDATE ck_controls SET control_id = 'A.8.18' WHERE control_id = 'A.9.4.4';
    UPDATE ck_controls SET control_id = 'A.8.4'  WHERE control_id = 'A.9.4.5';
    -- A.10 Cryptography (2013) → A.8.24 (2022)
    UPDATE ck_controls SET control_id = 'A.8.24' WHERE control_id IN ('A.10.1', 'A.10.1.1', 'A.10.1.2');
    -- A.11 Physical (2013) → A.7.x (2022)
    UPDATE ck_controls SET control_id = 'A.7.1'  WHERE control_id = 'A.11.1.1';
    UPDATE ck_controls SET control_id = 'A.7.2'  WHERE control_id = 'A.11.1.2';
    UPDATE ck_controls SET control_id = 'A.7.3'  WHERE control_id = 'A.11.1.3';
    UPDATE ck_controls SET control_id = 'A.7.6'  WHERE control_id = 'A.11.1.5';
    UPDATE ck_controls SET control_id = 'A.7.8'  WHERE control_id = 'A.11.2.1';
    UPDATE ck_controls SET control_id = 'A.7.9'  WHERE control_id = 'A.11.2.5';
    UPDATE ck_controls SET control_id = 'A.7.14' WHERE control_id = 'A.11.2.7';
    -- A.12 Operations (2013) → A.8.x (2022)
    UPDATE ck_controls SET control_id = 'A.5.37' WHERE control_id IN ('A.12.1', 'A.12.1.1');
    UPDATE ck_controls SET control_id = 'A.8.32' WHERE control_id = 'A.12.1.2';
    UPDATE ck_controls SET control_id = 'A.8.7'  WHERE control_id = 'A.12.2';
    UPDATE ck_controls SET control_id = 'A.8.13' WHERE control_id IN ('A.12.3', 'A.12.3.1');
    UPDATE ck_controls SET control_id = 'A.8.15' WHERE control_id = 'A.12.4';
    UPDATE ck_controls SET control_id = 'A.8.8'  WHERE control_id IN ('A.12.6', 'A.12.6.1');
    -- A.13 Communications (2013) → A.8.20-A.8.22 (2022)
    UPDATE ck_controls SET control_id = 'A.8.20' WHERE control_id IN ('A.13.1', 'A.13.1.1');
    UPDATE ck_controls SET control_id = 'A.8.21' WHERE control_id = 'A.13.1.2';
    UPDATE ck_controls SET control_id = 'A.8.22' WHERE control_id = 'A.13.1.3';
    UPDATE ck_controls SET control_id = 'A.5.14' WHERE control_id = 'A.13.2.1';
    UPDATE ck_controls SET control_id = 'A.6.6'  WHERE control_id = 'A.13.2.4';
    -- A.14 System Development (2013) → A.8.25-A.8.29 (2022)
    UPDATE ck_controls SET control_id = 'A.8.26' WHERE control_id IN ('A.14.1', 'A.14.1.1', 'A.14.1.2');
    UPDATE ck_controls SET control_id = 'A.8.25' WHERE control_id IN ('A.14.2', 'A.14.2.1');
    UPDATE ck_controls SET control_id = 'A.8.29' WHERE control_id = 'A.14.2.8';
    -- A.15 Supplier (2013) → A.5.19-A.5.22 (2022)
    UPDATE ck_controls SET control_id = 'A.5.19' WHERE control_id = 'A.15.1.1';
    UPDATE ck_controls SET control_id = 'A.5.20' WHERE control_id = 'A.15.1.2';
    UPDATE ck_controls SET control_id = 'A.5.21' WHERE control_id = 'A.15.1.3';
    UPDATE ck_controls SET control_id = 'A.5.22' WHERE control_id IN ('A.15.2.1', 'A.15.2.2');
    -- A.16 Incident (2013) → A.5.24-A.5.28 (2022)
    UPDATE ck_controls SET control_id = 'A.5.24' WHERE control_id IN ('A.16.1', 'A.16.1.1');
    UPDATE ck_controls SET control_id = 'A.6.8'  WHERE control_id = 'A.16.1.2';
    UPDATE ck_controls SET control_id = 'A.5.25' WHERE control_id = 'A.16.1.4';
    UPDATE ck_controls SET control_id = 'A.5.26' WHERE control_id = 'A.16.1.5';
    UPDATE ck_controls SET control_id = 'A.5.27' WHERE control_id = 'A.16.1.6';
    -- A.17 Business Continuity (2013) → A.5.29-A.5.30 (2022)
    UPDATE ck_controls SET control_id = 'A.5.29' WHERE control_id IN ('A.17.1', 'A.17.1.1', 'A.17.1.2', 'A.17.1.3');
    -- A.18 Compliance (2013) → A.5.31-A.5.36 (2022)
    UPDATE ck_controls SET control_id = 'A.5.31' WHERE control_id IN ('A.18.1', 'A.18.1.1');
    UPDATE ck_controls SET control_id = 'A.5.33' WHERE control_id = 'A.18.1.3';
    UPDATE ck_controls SET control_id = 'A.5.34' WHERE control_id = 'A.18.1.4';
    UPDATE ck_controls SET control_id = 'A.5.36' WHERE control_id IN ('A.18.2', 'A.18.2.2');

    -- Remove orphaned 2013 controls that duplicated into the same 2022 ID
    -- (ON CONFLICT scenario: keep only one row per org+framework+control_id)
    DELETE FROM ck_controls a
    USING ck_controls b
    WHERE a.framework_id = b.framework_id
      AND a.org_id = b.org_id
      AND a.control_id = b.control_id
      AND a.id > b.id;

    -- Update SoA entries that reference old control IDs
    UPDATE ck_soa_entries SET control_ref = 'A.5.1'  WHERE control_ref IN ('A.5.1.1', 'A.5.1.2');
    UPDATE ck_soa_entries SET control_ref = 'A.5.7'  WHERE control_ref = 'A2022-5.7';
    UPDATE ck_soa_entries SET control_ref = 'A.5.23' WHERE control_ref = 'A2022-5.23';
    UPDATE ck_soa_entries SET control_ref = 'A.5.30' WHERE control_ref = 'A2022-5.30';
    UPDATE ck_soa_entries SET control_ref = 'A.7.4'  WHERE control_ref = 'A2022-7.4';
    UPDATE ck_soa_entries SET control_ref = 'A.8.9'  WHERE control_ref = 'A2022-8.9';
    UPDATE ck_soa_entries SET control_ref = 'A.8.10' WHERE control_ref = 'A2022-8.10';
    UPDATE ck_soa_entries SET control_ref = 'A.8.11' WHERE control_ref = 'A2022-8.11';
    UPDATE ck_soa_entries SET control_ref = 'A.8.12' WHERE control_ref = 'A2022-8.12';
    UPDATE ck_soa_entries SET control_ref = 'A.8.16' WHERE control_ref = 'A2022-8.16';
    UPDATE ck_soa_entries SET control_ref = 'A.8.23' WHERE control_ref = 'A2022-8.23';
    UPDATE ck_soa_entries SET control_ref = 'A.8.28' WHERE control_ref = 'A2022-8.28';

    -- Update framework control mappings that reference old IDs
    UPDATE ck_framework_control_mappings SET target_control = 'A.5.30' WHERE target_control = 'A2022-5.30';
    UPDATE ck_framework_control_mappings SET target_control = 'A.8.6'  WHERE target_control = 'A.8.6';
    UPDATE ck_framework_control_mappings SET source_control = 'A.5.30' WHERE source_control = 'A2022-5.30';

END $$;
