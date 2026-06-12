-- S78-8: SoD — Rolle InternalAuditor (Vier-Augen-Prinzip für Audit-Abschluss und
-- Management-Review-Approval). Die Rolle ergänzt das Rollenmodell um eine
-- interne Prüfer-Instanz, die von der rw-Gruppe (Admin/SecurityAnalyst)
-- getrennt ist, damit kein Nutzer seine eigenen Audit-Ergebnisse genehmigen kann.
INSERT INTO roles (id, name, description)
VALUES (gen_random_uuid(), 'InternalAuditor',
        'Internal auditor role — can approve audit-program closures and management reviews (SoD). Assigned separately from rw roles to enforce four-eyes principle.')
ON CONFLICT DO NOTHING;
