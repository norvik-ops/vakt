-- Entfernt nur den Index. Die Bereinigung aus der .up.sql ist nicht umkehrbar:
-- Das Subject eines entkoppelten Kontos ist danach nicht mehr bekannt.
DROP INDEX IF EXISTS idx_users_saml_direct_subject;
