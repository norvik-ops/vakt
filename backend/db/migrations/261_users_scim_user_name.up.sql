-- R1-14-D08 — der SCIM-Anmeldename bekommt eine eigene Spalte.
--
-- Bis hierher hatte Vakt nur users.email, und die SCIM-Bereitstellung schrieb
-- dort den userName hinein: das SCIM-Attribut emails[] wurde eingelesen und
-- verworfen. Ein Identitaetsanbieter, der Anmeldename und Postadresse getrennt
-- fuehrt — bei Entra ID sind das userPrincipalName und mail, der Regelfall —
-- legte damit einen Nutzer an, dessen E-Mail-Spalte ein Anmeldename ist. Jede
-- Vakt-Mail (Passwort zuruecksetzen, Digest, Kampagne) ging danach an diesen
-- Anmeldenamen.
--
-- Beides in eine Spalte zu legen geht nicht: users.email ist der lokale
-- Anmeldebezeichner UND die Zustelladresse, und der SCIM-userName ist der
-- Schluessel, unter dem der Identitaetsanbieter seinen Nutzer wiederfindet.
-- Faellt der weg, legt der naechste Abgleich einen zweiten Nutzer an.
--
-- Die Spalte ist bewusst NULLbar: lokal angelegte Nutzer haben keinen
-- SCIM-Anmeldenamen, und ein leerer String waere ein Wert, ueber den der
-- eindeutige Index dann stolpert.
ALTER TABLE users ADD COLUMN IF NOT EXISTS scim_user_name TEXT;

-- Teil-Index: eindeutig unter den Nutzern, die einen SCIM-Anmeldenamen tragen.
-- Ohne das koennten zwei Abgleiche denselben Namen auf zwei Zeilen legen, und
-- die Suche unten wuerde je nach Reihenfolge mal die eine, mal die andere
-- treffen.
CREATE UNIQUE INDEX IF NOT EXISTS users_scim_user_name_key
    ON users (scim_user_name)
    WHERE scim_user_name IS NOT NULL;

-- Bestand: bis heute WAR die E-Mail der userName, weil die Bereitstellung nichts
-- anderes geschrieben hat. Fuer bereits per SCIM angelegte Nutzer ist die
-- Uebernahme der E-Mail also die richtige Antwort und nicht geraten — sie stellt
-- genau den Wert her, den der Identitaetsanbieter geschickt hat. Lokale Nutzer
-- bleiben unberuehrt (NULL), damit der Teil-Index sie nicht erfasst.
UPDATE users
   SET scim_user_name = email
 WHERE scim_provisioned = TRUE
   AND scim_user_name IS NULL;
