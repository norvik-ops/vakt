-- Sprint 13.1 S13-2: LemonSqueezy-Webhook Replay-Schutz.
-- LemonSqueezy sendet bei Netzwerk-Hangs oder Timeouts denselben Webhook erneut
-- (siehe https://docs.lemonsqueezy.com/help/webhooks). Bisher wurde Idempotenz
-- nur ueber `ON CONFLICT (ls_subscription_id)` in `ls_subscriptions` versucht,
-- was Edge-Cases nicht abdeckt (z.B. order_refunded triggert UPDATE statt
-- INSERT). Diese Tabelle deduped Webhook-Events auf Body-Hash-Ebene, bevor
-- ueberhaupt Business-Logik laeuft.
--
-- event_hash: sha256 des verifizierten Request-Body als Hex-String (64 chars).
-- Body-Hash ist robust gegen Replay (gleicher Body = gleicher Hash) und
-- benoetigt keinen Header, den LemonSqueezy nicht standardmaessig schickt.

CREATE TABLE IF NOT EXISTS lemonsqueezy_webhook_events (
    event_hash    TEXT PRIMARY KEY,
    event_name    TEXT NOT NULL,
    received_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Aufraeumen nach 90 Tagen ist Sache eines optionalen Cron-Jobs (kein Pflicht
-- — die Tabelle waechst langsam, schaetzungsweise < 100 Eintraege/Monat).
CREATE INDEX IF NOT EXISTS idx_ls_webhook_events_received_at
    ON lemonsqueezy_webhook_events (received_at);
