-- 059 rollback: drop SBOM and EOL component tracking tables
DROP TABLE IF EXISTS vb_eol_cache;
DROP TABLE IF EXISTS vb_components;
DROP TABLE IF EXISTS vb_sboms;
