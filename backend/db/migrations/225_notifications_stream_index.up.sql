-- S98-5: covering index for the notification SSE poll/push query.
-- Query: WHERE org_id = $1 AND created_at > $2 ORDER BY created_at ASC LIMIT 50
-- (org_id, created_at) lets each poll/flush be a sub-ms index range scan.
CREATE INDEX IF NOT EXISTS idx_user_notifications_org_cursor
    ON user_notifications (org_id, created_at);
