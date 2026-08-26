-- R1-14cA-11 — die Einladung darf dieselben Rollennamen tragen wie der
-- Rollenwechsel.
--
-- Bis hierher sprachen benachbarte Routen zwei unvereinbare Vokabulare: die
-- Einladung nahm nur admin|editor|viewer, der Rollenwechsel die Plattformnamen
-- (Admin, SecurityAnalyst, Viewer, AuditorReadOnly, InternalAuditor). Wer das
-- eine Vokabular kannte, scheiterte an der anderen Route.
--
-- Die Spalte hielt die drei kleingeschriebenen Werte per CHECK fest. Das war
-- nicht bloss eine Eingabepruefung: haette man nur die Eingabepruefung im Code
-- geweitet, waere die Einladung mit "Admin" an genau diesem CHECK gescheitert —
-- SQLSTATE 23514, aus Sicht des Aufrufers ein 500 ohne Erklaerung.
--
-- ZWEI Werte NICHT abzubilden waere die schlechtere Antwort gewesen:
-- AuditorReadOnly und InternalAuditor haben im alten Vokabular kein Gegenstueck
-- (beide fielen auf "viewer"). Eine Einladung fuer InternalAuditor haette dann
-- ausgesehen wie ein Erfolg und einen Betrachter angelegt — eine stille
-- Herabstufung, und genau die Form, die dieser Lauf mehrfach gefunden hat.
--
-- Die drei alten Werte bleiben erlaubt: es gibt Bestandszeilen, und ein
-- Client, der sie schickt, soll weiter funktionieren. Der Code schreibt ab
-- jetzt den Plattformnamen (CreateInvitation normalisiert), die alten Werte
-- sind also nur noch Lesevokabular.
ALTER TABLE user_invitations DROP CONSTRAINT IF EXISTS user_invitations_role_check;

ALTER TABLE user_invitations ADD CONSTRAINT user_invitations_role_check
    CHECK (role IN (
        -- Plattformrollen (roles.name) — was der Code ab jetzt schreibt
        'Admin', 'SecurityAnalyst', 'Viewer', 'AuditorReadOnly', 'InternalAuditor',
        -- Altbestand und alte Clients
        'admin', 'editor', 'viewer'
    ));
