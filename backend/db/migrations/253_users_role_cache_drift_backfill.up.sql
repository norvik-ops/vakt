-- ESK-13 / REV-ESK13 §2.1+§2.2 — den Rest der Rollen-Drift aufräumen.
--
-- Vorgeschichte: users.role ist ein Cache, org_members.role_id (→ roles.name) die
-- Quelle der Wahrheit (ADR-0074/ADR-0077, Migration 249). Bis ESK-13 schrieb
-- PATCH /admin/users/:id/role aber AUSSCHLIESSLICH den Cache. Jede Rollenänderung
-- vor diesem Commit hinterliess deshalb ein auseinanderlaufendes Paar:
--
--   Beförderung  -> org_members = Viewer, users.role = 'admin'
--   Herabstufung -> org_members = Admin,  users.role = 'viewer'
--
-- Migration 249 hat nur die erste Form erwischt, und auch die nur teilweise: sie
-- lief einmalig, Drift ist danach weiter entstanden. Der Code-Fix schliesst die
-- Quelle (die Rollenänderung schreibt jetzt beide Spalten in einer Transaktion)
-- und stellt usermgmt.requireAdmin — den letzten Leser des Caches, der eine
-- Autorisierungsentscheidung traf — auf org_members um. Danach autorisiert
-- users.role NICHTS mehr. Diese Migration räumt die Werte auf, die liegen
-- geblieben sind.
--
-- RICHTUNG: ausschliesslich abwärts. Das ist die Regel aus 249 und keine
-- Vorsicht ohne Grund:
--
--   * Ein Backfill, der users.role auf 'admin' HOCHsetzt, weil org_members Admin
--     sagt, würde die Herabstufungs-Drift genau falsch herum auflösen — der
--     Betreiber wollte diesen Nutzer ja herabstufen. Und users.role ist global,
--     org_members org-gebunden: wer in Org A Admin ist, bekäme den 'admin'-Cache
--     auch für Org B. Genau die Konstellation, aus der D24-1 entstand.
--   * Ein zu niedriger Cache-Wert kann nach dem Code-Fix niemandem etwas
--     wegnehmen (er autorisiert nichts mehr) und wäre, sollte ihn je wieder
--     jemand lesen, fail-closed. Ein zu hoher wäre fail-open.
--
-- Ableitung wie in 249: die HÖCHSTE org_members-Rolle über ALLE Orgs des Nutzers,
-- weil users.role eine einzelne globale Spalte ist. Waisen ohne org_members-Zeile
-- bleiben unangetastet (die Subquery liefert für sie keine Zeile).
--
-- WAS DIESE MIGRATION BEWUSST NICHT TUT:
--
--   * Sie setzt NIEMANDEN in org_members auf Admin. Auf Altbestand kann eine Org
--     existieren, die gar keinen org_members-Admin mehr hat (der alte
--     Last-Admin-Schutz zählte users.role und schützte einen Admin mit
--     'viewer'-Cache nicht gegen DELETE). Eine solche Org verliert mit dem
--     Code-Fix ihre Nutzerverwaltung — sie hatte allerdings schon vorher auf
--     KEINEM anderen Modul einen Admin-Claim, weil die Claims aus org_members
--     kommen. Sie still zu reparieren hiesse, aus einem Cache-Wert einen
--     plattformweiten Admin-Claim zu erzeugen; das wäre schlimmer als der
--     Ist-Zustand. Der Betreiber findet solche Orgs mit:
--
--       SELECT o.id, o.slug FROM organizations o
--       WHERE NOT EXISTS (SELECT 1 FROM org_members om JOIN roles r ON r.id = om.role_id
--                         WHERE om.org_id = o.id AND r.name = 'Admin');
--
--   * Sie bricht auch nicht ab, wenn sie solche Orgs findet. Die API startet mit
--     AUTO_MIGRATE=true (infra/server/docker-compose.yml) — eine Migration, die
--     wegen eines Datenzustands RAISE EXCEPTION wirft, ist dort ein Ausfall des
--     ganzen Dienstes, nicht eine Warnung an den Betreiber. golang-migrate über
--     den pgx5-Treiber reicht ausserdem weder NOTICE noch WARNING durch, ein
--     RAISE WARNING wäre also folgenlose Dekoration.

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
  AND u.role <> derived.simple_role
  -- Nur abwärts: admin(3) > editor(2) > viewer(1). Die Gegenrichtung bleibt
  -- absichtlich stehen — siehe RICHTUNG oben.
  AND CASE u.role
          WHEN 'admin'  THEN 3
          WHEN 'editor' THEN 2
          ELSE 1
      END
    > CASE derived.simple_role
          WHEN 'admin'  THEN 3
          WHEN 'editor' THEN 2
          ELSE 1
      END;
