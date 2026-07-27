-- S132 Spur A — D24-1 (CRIT) + V24-a: Split-Brain-RBAC schließen.
--
-- users.role trug seit Migration 077 DEFAULT 'admin'. usermgmt.requireAdmin liest
-- users.role (nicht org_members) — jeder ohne explizit gesetztes role wurde damit
-- Admin, obwohl org_members ihn korrekt als Viewer/SecurityAnalyst führte. Das ist
-- eine stille Privilege-Escalation über die zweite, denormalisierte Rollenquelle.
--
-- SoT ist org_members.role (via role_id → roles.name); users.role ist nur ein
-- Cache. Entscheidung (ADR-0074): least-privilege Default 'viewer' + jede Insert-
-- Grenze setzt role explizit. Diese Migration flippt den Default und korrigiert
-- Altbestände, die durch den 'admin'-Default falsch hochgestuft wurden.

-- 1) Default auf least-privilege umstellen. Neue Zeilen ohne explizites role sind
--    ab jetzt Viewer, nicht Admin. Der CHECK-Constraint aus 077 bleibt unberührt.
ALTER TABLE users ALTER COLUMN role SET DEFAULT 'viewer';

-- 2) Backfill: Nutzer, die aktuell role='admin' tragen (der verdächtige Default),
--    deren tatsächliche höchste org_members-Rolle über ALLE ihre Orgs aber UNTER
--    Admin liegt, auf die abgeleitete simple Rolle heruntersetzen.
--
--    Warum "höchste Rolle über alle Orgs": users.role ist eine EINZELNE globale
--    Spalte, ein Nutzer kann aber in mehreren Orgs verschieden eingestuft sein.
--    Wer in IRGENDEINER Org Admin ist, bleibt Admin (WHERE greift nicht) — sonst
--    würde der Backfill einen legitimen Admin herabstufen. Nutzer ohne jede
--    org_members-Zeile (Waisen) bleiben unangetastet (Subquery liefert keine
--    Zeile) — das entspricht der abgesteckten Aufgabe; ein Orphan-Admin hat ohnehin
--    keine Org, deren Admin er über den Token-Pfad (org_members) sein könnte.
--
--    Mapping roles.name → users.role (identisch zu usermgmt.platformRoleName,
--    nur invers): Admin→admin, SecurityAnalyst→editor, sonst→viewer.
UPDATE users u
SET role = derived.simple_role
FROM (
    SELECT om.user_id,
           CASE
               WHEN bool_or(r.name = 'Admin')           THEN 'admin'
               WHEN bool_or(r.name = 'SecurityAnalyst') THEN 'editor'
               ELSE 'viewer'
           END AS simple_role
    FROM org_members om
    JOIN roles r ON r.id = om.role_id
    GROUP BY om.user_id
) AS derived
WHERE derived.user_id = u.id
  AND u.role = 'admin'
  AND derived.simple_role <> 'admin';
