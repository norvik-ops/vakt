-- Ein Direct-SAML-Subject ('saml:<org_id>:<NameID>', Provider 'saml_direct')
-- gehört genau einem Konto. Bisher hielt das nur der Code ein: Das Hochstufen
-- eines Altkontos (provisionSAMLUser) prüfte bis 2026-11-10 nur Konten, die
-- Mitglied der Org sind. Trug ein ausgetretenes Konto das Subject, bekam ein
-- zweites Konto dasselbe Subject. Die Subject-Suche in provisionOIDCUser hat kein
-- ORDER BY, welches Konto ein Login traf, war also nicht festgelegt. Dieser
-- Index macht die Regel zur Eigenschaft der Datenbank.
--
-- Schritt 0: Sperre. Die Transaktion stellt nicht golang-migrate, sondern
-- Postgres: Der Treiber schickt die ganze Datei als EINE mehranweisige Abfrage
-- über das einfache Protokoll, und Postgres führt sie als implizite Transaktion
-- aus. Nur deshalb sind SET LOCAL und LOCK TABLE hier gültig. Mit
-- x-multi-statement=true (Anweisungen einzeln) schlüge LOCK laut mit 25P01 fehl.
-- Die Tabellensperre hält bis zum Ende der Abfrage und lässt Lesen zu, blockiert
-- aber jedes INSERT/UPDATE auf users. Zwischen Bereinigung und CREATE UNIQUE
-- INDEX kann so keine neue Dublette entstehen.
-- lock_timeout: Kommt die Sperre nicht binnen 5 s (lange Transaktion auf users),
-- bricht die Abfrage ab, statt Logins unbegrenzt hinter sich zu stauen. Postgres
-- rollt alles zurück, das Schema bleibt auf Stand 270. golang-migrate setzt aber
-- dirty=true, und die API endet beim Start per log.Fatal (cmd/api/main.go) in
-- einer Neustart-Schleife. Rückweg: per psql
--   UPDATE schema_migrations SET version = 270, dirty = false;
-- danach die API neu starten; sie wendet 271 erneut an.
SET LOCAL lock_timeout = '5s';
LOCK TABLE users IN SHARE ROW EXCLUSIVE MODE;

-- Schritt 1: Bereinigung, damit der Index auf Bestand mit Dubletten nicht
-- scheitert. Ein Merge nach main rollt unbeaufsichtigt aus; ein Abbruch hielte
-- den Start der API an, bis jemand von Hand eingreift. Deshalb bereinigen statt
-- abbrechen. Je Subject behält genau ein Konto das Subject, deterministisch in
-- dieser Rangfolge:
--   1. Mitglied der Org aus dem Subject (Präfix 'saml:<org_id>:', dieselbe Form
--      wie directSAMLSubject in oidc.go und Migration 270). Nur ein Mitglied
--      findet der Direct-SAML-Login über das Subject überhaupt.
--   2. aktives Konto (is_active) vor deaktiviertem,
--   3. älteres Konto (created_at, dann id).
-- Sonst behielte ein ausgetretenes, älteres Konto das Subject, und das aktive
-- Mitglied liefe beim nächsten Login per JIT gegen den Index: ausgesperrt.
-- Die übrigen Konten verlieren Subject UND Provider (beide NULL), wie beim
-- Anonymisieren in account.go. Gelöscht wird nichts: Konto, Mitgliedschaft und
-- Daten bleiben. Entkoppelte Konten melden sich nur noch per Passwort an,
-- sofern eines gesetzt ist und das Konto aktiv ist. Ein Direct-SAML-Login mit
-- derselben NameID landet im behaltenden Konto.
-- Bewusst NICHT zurück auf die Altform (rohe NameID, Provider 'saml'): Diese
-- Form trifft ein Casdoor-SAML-Login mit gleichem Subject (der Grund für
-- Migration 270). Die Bereinigung würde die Lücke wieder öffnen.
-- Nicht umkehrbar: Die .down.sql entfernt nur den Index. Welches Subject ein
-- bereinigtes Konto trug, steht danach nicht mehr in der Tabelle. Ein
-- Protokoll der bereinigten Konten gibt es nicht: Das RAISE NOTICE unten sieht
-- nur, wer die Datei per psql ausführt. pgx verwirft NOTICEs ohne OnNotice, im
-- Log von golang-migrate bzw. der API erscheint die Zahl also nicht.
--
-- Schritt 2: der Index. Ohne CONCURRENTLY: users ist klein (ein Kunde, eine
-- Org), und CONCURRENTLY läuft nur ohne Transaktion, also ohne die Sperre aus
-- Schritt 0. Kein NOW() im WHERE des Index (42P17); das Prädikat ist eine
-- Konstante.
DO $$
DECLARE
    n integer;
BEGIN
    WITH ranked AS (
        SELECT u.id,
               row_number() OVER (
                   PARTITION BY u.oidc_subject
                   ORDER BY EXISTS (
                                SELECT 1 FROM org_members om
                                WHERE om.user_id = u.id
                                  AND left(u.oidc_subject, 42) = 'saml:' || om.org_id::text || ':'
                            ) DESC,
                            u.is_active DESC,
                            u.created_at ASC,
                            u.id ASC) AS rn
        FROM users u
        WHERE u.oidc_provider = 'saml_direct'
          AND u.oidc_subject IS NOT NULL
    )
    UPDATE users u
    SET oidc_subject  = NULL,
        oidc_provider = NULL,
        updated_at    = NOW()
    FROM ranked r
    WHERE u.id = r.id AND r.rn > 1;
    GET DIAGNOSTICS n = ROW_COUNT;
    -- Nur bei Ausführung per psql sichtbar (siehe oben), nicht im API-Log.
    IF n > 0 THEN
        RAISE NOTICE '271: % Konto/Konten mit doppeltem Direct-SAML-Subject entkoppelt (Mitglied der Org, aktiv, aelter behaelt das Subject)', n;
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_saml_direct_subject
    ON users (oidc_subject)
    WHERE oidc_provider = 'saml_direct';
