-- Zurueck auf den Stand ohne eigene Spalte fuer den SCIM-Anmeldenamen.
-- Der Index haengt an der Spalte und faellt mit ihr, wird aber ausdruecklich
-- genannt, damit ein Teil-Rueckbau nicht stillschweigend einen Index stehen
-- laesst.
DROP INDEX IF EXISTS users_scim_user_name_key;
ALTER TABLE users DROP COLUMN IF EXISTS scim_user_name;
