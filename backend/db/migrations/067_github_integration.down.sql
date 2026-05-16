-- Rollback Migration 067: GitHub Integration
DROP TABLE IF EXISTS integrations_github_checks;
DROP TABLE IF EXISTS integrations_github;
