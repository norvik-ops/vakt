-- Fix: rename po_vvt_entries → po_processing_activities (missing rename from Sprint 68)
-- Migrations 185, 189, 192 and all post-S68 service code reference po_processing_activities.
-- The original table name po_vvt_entries was created in migration 014.
ALTER TABLE IF EXISTS po_vvt_entries RENAME TO po_processing_activities;
