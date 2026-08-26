// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// REV-W1D §B — Anmelden und Zugangsprüfung müssen dieselbe pw_version-Quelle lesen.
//
// DER DEFEKT, DEN DIESE DATEI FÄNGT
// ---------------------------------
// `currentPwVersion` stempelt beim Anmelden die Version in den Token.
// `checkPwVersion` (Middleware) prüft sie bei jeder Folgeanfrage und verlangt
// Gleichheit. Die frühere Fassung von `currentPwVersion` las AUSSCHLIESSLICH Redis
// und nahm bei jedem Fehlschlag 0 an — während `checkPwVersion` im selben Fall auf
// PostgreSQL zurückfällt.
//
// Das ausgelieferte Redis läuft mit `allkeys-lru`; Verdrängung ist eingeplant, kein
// Ausnahmefall. Nach dem Verlust des Schlüssels stempelte die Anmeldung 0, die
// Middleware las die echte Version (z. B. 1) aus PostgreSQL — und der Betroffene
// bekam auf JEDE Anfrage dauerhaft 401, obwohl die Anmeldung selbst erfolgreich war.
//
// Vorbestehend, aber der SSO-Fix aus derselben Spur legt den Auslöser auf den
// normalen Weg des Identitätsanbieters: vorher kam nur ein einziger Nutzer über SSO
// herein, jetzt alle.
//
// WAS DIESER TEST PRÜFT
// ---------------------
// Nicht „liest es aus PostgreSQL", sondern die Invariante dahinter: Was die Anmeldung
// stempelt, muss die Zugangsprüfung akzeptieren — auch dann, wenn Redis den Wert nicht
// mehr hat. Ein Test, der nur die Quelle prüft, ginge bei der nächsten Umstellung
// grün und die Invariante trotzdem kaputt.
package auth

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestPwVersionStamp_SurvivesRedisEviction fährt den echten Ablauf: PostgreSQL trägt
// eine Version > 0, Redis kennt den Schlüssel nicht (verdrängt). Die Anmeldung muss
// denselben Wert stempeln, den die Zugangsprüfung anschließend verlangt.
func TestPwVersionStamp_SurvivesRedisEviction(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL nicht gesetzt — dieser Test braucht eine migrierte Postgres (CI setzt sie)")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()

	var orgID, userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('pwstamp', 'pwstamp-'||substr(md5(random()::text),1,8))
		 RETURNING id::text`).Scan(&orgID))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1::uuid`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1::uuid`, orgID)
	})
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, role, pw_version)
		 VALUES ('pwstamp-'||substr(md5(random()::text),1,8)||'@example.test', 'x', 'viewer', 3)
		 RETURNING id::text`).Scan(&userID))

	// Redis kennt den Schlüssel nicht — genau der Zustand nach einer Verdrängung.
	// `s.redis == nil` bildet das ab: beide Wege müssen dann auf PostgreSQL greifen.
	svc := &Service{db: pool}

	stamped := svc.currentPwVersion(ctx, userID)
	require.Equal(t, int64(3), stamped,
		"die Anmeldung muss die Version aus PostgreSQL stempeln; 0 hiesse: der Nutzer kommt herein "+
			"und bekommt danach dauerhaft 401")

	// Die Gegenprobe, die die Invariante wirklich festnagelt: Was gestempelt wurde,
	// muss die Zugangsprüfung akzeptieren.
	// Tote Redis-Verbindung statt nil: die Middleware ruft checkPwVersion nur mit
	// einem nicht-nil Client auf (beide Aufrufstellen pruefen das), nil waere also
	// eine Eingabe, die die Funktion produktiv nie sieht.
	deadRedis := dialFailingRedis(t)
	err = checkPwVersion(ctx, deadRedis, pool, &Claims{UserID: userID, PwVersion: stamped})
	require.NoError(t, err,
		"was die Anmeldung stempelt, muss die Zugangspruefung akzeptieren — sonst sperrt sich "+
			"der Nutzer mit jeder Folgeanfrage selbst aus")

	// Und die Umkehrung: der alte Wert 0 MUSS abgelehnt werden. Ohne diese Zeile
	// waere der Test auch dann gruen, wenn checkPwVersion alles durchliesse.
	err = checkPwVersion(ctx, deadRedis, pool, &Claims{UserID: userID, PwVersion: 0})
	require.Error(t, err,
		"eine veraltete Version darf nicht akzeptiert werden — sonst prueft dieser Test nichts")
}
