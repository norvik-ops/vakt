-- Rollback: 2022 IDs → 2013 IDs is not losslessly reversible because the 2022 standard
-- merges multiple 2013 controls into single controls. We restore only A2022-* prefixes for
-- the 11 new-in-2022 controls (which are unambiguous); merged controls stay in 2022 form.
DO $$
BEGIN
    UPDATE ck_controls SET control_id = 'A2022-5.7'  WHERE control_id = 'A.5.7';
    UPDATE ck_controls SET control_id = 'A2022-5.23' WHERE control_id = 'A.5.23';
    UPDATE ck_controls SET control_id = 'A2022-5.30' WHERE control_id = 'A.5.30';
    UPDATE ck_controls SET control_id = 'A2022-7.4'  WHERE control_id = 'A.7.4';
    UPDATE ck_controls SET control_id = 'A2022-8.9'  WHERE control_id = 'A.8.9';
    UPDATE ck_controls SET control_id = 'A2022-8.10' WHERE control_id = 'A.8.10';
    UPDATE ck_controls SET control_id = 'A2022-8.11' WHERE control_id = 'A.8.11';
    UPDATE ck_controls SET control_id = 'A2022-8.12' WHERE control_id = 'A.8.12';
    UPDATE ck_controls SET control_id = 'A2022-8.16' WHERE control_id = 'A.8.16';
    UPDATE ck_controls SET control_id = 'A2022-8.23' WHERE control_id = 'A.8.23';
    UPDATE ck_controls SET control_id = 'A2022-8.28' WHERE control_id = 'A.8.28';

    UPDATE ck_soa_entries SET control_ref = 'A2022-5.7'  WHERE control_ref = 'A.5.7';
    UPDATE ck_soa_entries SET control_ref = 'A2022-5.23' WHERE control_ref = 'A.5.23';
    UPDATE ck_soa_entries SET control_ref = 'A2022-5.30' WHERE control_ref = 'A.5.30';
    UPDATE ck_soa_entries SET control_ref = 'A2022-7.4'  WHERE control_ref = 'A.7.4';
    UPDATE ck_soa_entries SET control_ref = 'A2022-8.9'  WHERE control_ref = 'A.8.9';
    UPDATE ck_soa_entries SET control_ref = 'A2022-8.10' WHERE control_ref = 'A.8.10';
    UPDATE ck_soa_entries SET control_ref = 'A2022-8.11' WHERE control_ref = 'A.8.11';
    UPDATE ck_soa_entries SET control_ref = 'A2022-8.12' WHERE control_ref = 'A.8.12';
    UPDATE ck_soa_entries SET control_ref = 'A2022-8.16' WHERE control_ref = 'A.8.16';
    UPDATE ck_soa_entries SET control_ref = 'A2022-8.23' WHERE control_ref = 'A.8.23';
    UPDATE ck_soa_entries SET control_ref = 'A2022-8.28' WHERE control_ref = 'A.8.28';
END $$;
