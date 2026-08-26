// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package redisopt

import (
	"crypto/tls"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Abnahme 1 (Basislinie): eine URL ohne Datenbanknummer verhaelt sich
// unveraendert. Das ist die Konfiguration, die ausgeliefert wird — waere sie
// betroffen, waere R1-14b-01 kein latenter Defekt gewesen, sondern eine
// laufende Stoerung.
func TestAsynqFromURLBaselineUnchanged(t *testing.T) {
	cases := []struct {
		name     string
		url      string
		wantAddr string
		wantPass string
	}{
		{"leere URL faellt auf localhost zurueck", "", "localhost:6379", ""},
		{"URL ohne Datenbanknummer", "redis://redis:6379", "redis:6379", ""},
		{"ausgelieferte Compose-Form mit Passwort", "redis://:geheim@redis:6379", "redis:6379", "geheim"},
		{"nackte host:port-Adresse ohne Schema", "cache.internal:6379", "cache.internal:6379", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AsynqFromURL(tc.url)
			assert.Equal(t, tc.wantAddr, got.Addr)
			assert.Equal(t, tc.wantPass, got.Password)
			assert.Equal(t, 0, got.DB, "ohne Datenbanknummer in der URL ist DB 0 richtig")
		})
	}
}

// Der eigentliche Defekt: eine Datenbanknummer in der URL muss ankommen.
// Kommt sie nicht an, enqueued die API nach DB 0 und der Worker liest aus DB N —
// beide Seiten sind fuer sich gesund und reden nicht miteinander.
func TestAsynqFromURLCarriesDatabaseNumber(t *testing.T) {
	for _, url := range []string{
		"redis://redis:6379/1",
		"redis://:geheim@redis:6379/1",
		"rediss://redis:6380/1",
	} {
		t.Run(url, func(t *testing.T) {
			assert.Equal(t, 1, AsynqFromURL(url).DB)
		})
	}
	assert.Equal(t, 9, AsynqFromURL("redis://redis:6379/9").DB)
}

// Die Ableitung deckt ausserdem die Faelle ab, die der Worker vorher fallen
// liess: Redis-ACL-Benutzername und TLS. Asynq verhandelt TLS ausschliesslich,
// wenn TLSConfig gesetzt ist — eine rediss://-URL ohne dieses Feld ist also
// nicht "TLS mit Vorgaben", sondern gar kein TLS.
func TestAsynqCarriesUsernameAndTLS(t *testing.T) {
	parsed, err := redis.ParseURL("rediss://alice:geheim@redis:6380/2")
	require.NoError(t, err)

	got := Asynq(parsed)
	assert.Equal(t, "alice", got.Username, "Redis-ACL-Benutzername")
	assert.Equal(t, "geheim", got.Password)
	assert.Equal(t, 2, got.DB)
	require.NotNil(t, got.TLSConfig, "rediss:// ohne TLSConfig waere Klartext")
	assert.IsType(t, &tls.Config{}, got.TLSConfig)
}

// Eine Falle von go-redis, festgehalten statt geaendert: "redis:6379" ist KEINE
// nackte Adresse, sondern eine gueltige URL mit dem Schema "redis" und dem
// opaken Rest "6379" — ohne Host. ParseURL nimmt sie an und liefert
// localhost:6379. Wer in docker-compose den Dienstnamen ohne Schema schreibt
// (VAKT_REDIS_URL=redis:6379, naheliegend), landet also still auf localhost.
//
// Das ist das Verhalten des Workers seit jeher; die Zusammenlegung in redisopt
// aendert es NICHT, und dieser Test haelt genau das fest. Wer es spaeter
// begradigen will, aendert damit das Verhalten einer ausgelieferten
// Konfiguration und soll hier rot werden statt es nebenbei mitzunehmen.
func TestBareSchemeLikeHostIsParsedAsAURL(t *testing.T) {
	assert.Equal(t, "localhost:6379", AsynqFromURL("redis:6379").Addr)
	assert.Equal(t, "localhost:6379", AsynqFromURL("rediss:6379").Addr)
	// Ein Host, der kein Redis-Schema ist, faellt korrekt in den Adress-Zweig.
	assert.Equal(t, "cache.internal:6379", AsynqFromURL("cache.internal:6379").Addr)
}

// Ein nil-Zeiger ist derselbe Fall wie eine leere URL — kein Absturz, kein
// stiller Nullwert an einer anderen Stelle.
func TestAsynqNilOptionsFallsBackToDefault(t *testing.T) {
	got := Asynq(nil)
	assert.Equal(t, "localhost:6379", got.Addr)
	assert.Equal(t, 0, got.DB)
}

// GoRedis muss kopieren, nicht durchreichen: go-redis schreibt beim ersten
// Gebrauch Vorgabewerte in die uebergebene Struktur, und zwei Clients auf
// demselben Zeiger teilen sich dann stillschweigend ihre Konfiguration.
func TestGoRedisReturnsACopy(t *testing.T) {
	orig, err := redis.ParseURL("redis://redis:6379/3")
	require.NoError(t, err)

	cp := GoRedis(orig)
	require.NotNil(t, cp)
	assert.NotSame(t, orig, cp, "muss eine Kopie sein, nicht derselbe Zeiger")
	assert.Equal(t, 3, cp.DB)

	cp.DB = 7
	assert.Equal(t, 3, orig.DB, "die Kopie darf nicht auf das Original zurueckschlagen")

	assert.Nil(t, GoRedis(nil))
}
