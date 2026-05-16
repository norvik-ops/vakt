UPDATE ck_controls SET description = NULL
WHERE control_id LIKE 'NIS2-%' OR control_id LIKE 'A.%';
