package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// ErrRateLimited wird zurueckgegeben wenn der Org-Token-Bucket leer ist.
var ErrRateLimited = errors.New("ai: rate limit exceeded for organization")

// ErrQuotaExceeded wird zurueckgegeben wenn die Tages-Token-Quota fuer die
// Org erschoepft ist (`VAKT_AI_DAILY_TOKEN_LIMIT_PER_ORG`).
var ErrQuotaExceeded = errors.New("ai: daily token quota exceeded for organization")

// CEMonthlyLimit is the number of AI requests allowed per month for Community Edition orgs.
const CEMonthlyLimit = 25

// UsageTracker buendelt Rate-Limit, Tages-Quota, Response-Cache und
// Usage-Persistierung. Der Tracker ist optional: wenn rdb oder db nil ist,
// faellt der jeweilige Pfad auf "always allow" / "no cache" / "no persist"
// zurueck — damit lokale Unit-Tests + initiale Dev-Setups ohne Redis
// weiterhin funktionieren.
//
// Sprint 15 / S15-1, S15-2, S15-3.
type UsageTracker struct {
	rdb              *redis.Client
	db               *pgxpool.Pool
	rateLimitRPM     int
	dailyTokenLimit  int
	cacheTTLSeconds  int
	costPerMTokenIn  int64
	costPerMTokenOut int64
}

// UsageTrackerConfig sammelt die Konstruktor-Parameter, damit die Konfig nicht
// als langer Argument-Vektor uebergeben werden muss.
type UsageTrackerConfig struct {
	RateLimitRPM     int
	DailyTokenLimit  int
	CacheTTLSeconds  int
	CostPerMTokenIn  int64 // micro-EUR pro 1M Input-Tokens
	CostPerMTokenOut int64 // micro-EUR pro 1M Output-Tokens
}

// NewUsageTracker baut einen Tracker. Beide Backends (Redis + Postgres) sind
// optional, der Tracker degradiert geordnet.
func NewUsageTracker(rdb *redis.Client, db *pgxpool.Pool, cfg UsageTrackerConfig) *UsageTracker {
	return &UsageTracker{
		rdb:              rdb,
		db:               db,
		rateLimitRPM:     cfg.RateLimitRPM,
		dailyTokenLimit:  cfg.DailyTokenLimit,
		cacheTTLSeconds:  cfg.CacheTTLSeconds,
		costPerMTokenIn:  cfg.CostPerMTokenIn,
		costPerMTokenOut: cfg.CostPerMTokenOut,
	}
}

// CheckRateLimit prueft einen einfachen Fixed-Window-Counter pro Org.
// Implementierung: Redis INCR + EXPIRE auf 60 s. Wenn count > rateLimitRPM,
// wird ErrRateLimited zurueckgegeben.
//
// Bei rdb == nil oder rateLimitRPM <= 0 ist die Methode no-op.
func (u *UsageTracker) CheckRateLimit(ctx context.Context, orgID string) error {
	if u.rdb == nil || u.rateLimitRPM <= 0 || orgID == "" {
		return nil
	}
	key := fmt.Sprintf("ai:rl:%s:%d", orgID, time.Now().UTC().Unix()/60)
	count, err := u.rdb.Incr(ctx, key).Result()
	if err != nil {
		// Bei Redis-Fehler nicht blockieren (verfuegbarkeitsvorrang).
		log.Warn().Err(err).Str("org_id", orgID).Msg("ai: rate-limit INCR failed — allowing")
		return nil
	}
	if count == 1 {
		// First request in the window — set expiry.
		u.rdb.Expire(ctx, key, 65*time.Second)
	}
	if int(count) > u.rateLimitRPM {
		return ErrRateLimited
	}
	return nil
}

// CheckDailyQuota prueft den Tagestoken-Counter pro Org. Greift auf
// SUM(tokens_in+tokens_out) der Tabelle ai_usage fuer den aktuellen UTC-Tag.
//
// Bei dailyTokenLimit <= 0 oder fehlender DB ist die Methode no-op.
func (u *UsageTracker) CheckDailyQuota(ctx context.Context, orgID string) error {
	if u.db == nil || u.dailyTokenLimit <= 0 || orgID == "" {
		return nil
	}
	var total int
	err := u.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(COALESCE(tokens_in,0) + COALESCE(tokens_out,0)), 0)::int
		FROM ai_usage
		WHERE org_id = $1::uuid
		  AND created_at >= date_trunc('day', NOW() AT TIME ZONE 'UTC')
	`, orgID).Scan(&total)
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ai: quota lookup failed — allowing")
		return nil
	}
	if total >= u.dailyTokenLimit {
		return ErrQuotaExceeded
	}
	return nil
}

// CacheKey baut einen stabilen sha256-Hash aus Model + Prompt-Inhalt fuer
// das Response-Caching. Inputs sind die gleichen Bytes, die an das LLM
// gehen — deterministisch.
func CacheKey(model string, messages []chatMessage) string {
	h := sha256.New()
	h.Write([]byte(model))
	h.Write([]byte{0})
	for _, m := range messages {
		h.Write([]byte(m.Role))
		h.Write([]byte{0})
		h.Write([]byte(m.Content))
		h.Write([]byte{0})
	}
	return "ai:cache:" + hex.EncodeToString(h.Sum(nil))
}

// CacheGet liest eine gecachte Response. Returns ("", false) bei Cache-Miss
// oder wenn Cache deaktiviert ist.
func (u *UsageTracker) CacheGet(ctx context.Context, key string) (string, bool) {
	if u.rdb == nil || u.cacheTTLSeconds <= 0 || key == "" {
		return "", false
	}
	v, err := u.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", false
	}
	return v, true
}

// CacheSet persistiert eine Response im Redis-Cache mit der konfigurierten TTL.
func (u *UsageTracker) CacheSet(ctx context.Context, key, value string) {
	if u.rdb == nil || u.cacheTTLSeconds <= 0 || key == "" {
		return
	}
	if err := u.rdb.Set(ctx, key, value, time.Duration(u.cacheTTLSeconds)*time.Second).Err(); err != nil {
		log.Warn().Err(err).Msg("ai: cache SET failed")
	}
}

// UsageRecord ist der Persistierungs-Record fuer einen einzelnen Call.
type UsageRecord struct {
	OrgID      string
	Model      string
	TokensIn   *int
	TokensOut  *int
	DurationMs int
	Status     string // ok | rate_limited | timeout | provider_error | cache_hit
	RequestID  string
}

// Record persistiert einen Usage-Eintrag inkl. geschaetzter Kosten. Errors
// werden nur ge-loggt — Persistierung darf den Call nicht blockieren.
func (u *UsageTracker) Record(ctx context.Context, r UsageRecord) {
	if u.db == nil || r.OrgID == "" {
		return
	}
	var costMicroEur int64
	if r.TokensIn != nil {
		costMicroEur += int64(*r.TokensIn) * u.costPerMTokenIn / 1_000_000
	}
	if r.TokensOut != nil {
		costMicroEur += int64(*r.TokensOut) * u.costPerMTokenOut / 1_000_000
	}
	_, err := u.db.Exec(ctx, `
		INSERT INTO ai_usage
		  (org_id, model, tokens_in, tokens_out, cost_micro_eur, duration_ms, status, request_id)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8)`,
		r.OrgID, r.Model, r.TokensIn, r.TokensOut, costMicroEur, r.DurationMs, r.Status, r.RequestID,
	)
	if err != nil {
		log.Warn().Err(err).Str("org_id", r.OrgID).Msg("ai: usage record insert failed")
	}
}

// CEMonthlyUsage returns how many AI requests the org has made this calendar month.
// Only successful, non-cached requests are counted (status = 'ok').
// Returns 0 and logs on DB error so callers degrade gracefully.
func (u *UsageTracker) CEMonthlyUsage(ctx context.Context, orgID string) int {
	if u.db == nil || orgID == "" {
		return 0
	}
	var count int
	err := u.db.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM ai_usage
		WHERE org_id = $1::uuid
		  AND status = 'ok'
		  AND created_at >= date_trunc('month', NOW() AT TIME ZONE 'UTC')
	`, orgID).Scan(&count)
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ai: CE monthly usage lookup failed — treating as 0")
		return 0
	}
	return count
}
