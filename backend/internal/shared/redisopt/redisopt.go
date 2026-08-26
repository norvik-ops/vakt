// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package redisopt haelt die EINZIGE Ableitung von VAKT_REDIS_URL zu den
// Verbindungsoptionen von Asynq.
//
// Warum ein eigenes Paket fuer eine Feldabbildung von vier Zeilen:
//
// Vor R1-14b-01 baute jede Aufrufstelle diese Struktur von Hand. Der Worker
// (cmd/worker/main.go) setzte dabei DB, die API (cmd/api/routes.go) an vier
// Stellen nicht. Traegt VAKT_REDIS_URL eine Datenbanknummer — redis://host:6379/1,
// voellig ueblich, wenn man sich in einer bestehenden Redis-Instanz eine eigene
// Datenbank nimmt —, dann schreibt die API ihre Auftraege nach Datenbank 0 und
// der Worker liest aus Datenbank 1. Die Kette bricht dann lautlos: kein Fehler,
// kein Log, kein Wiederholungsversuch. Beide Seiten sind fuer sich gesund, sie
// reden nur nicht miteinander. Live belegt: asynq:queues lag in DB 0, die
// Worker-Registrierung in DB 1, der Trainings-Task lag ungelesen in DB 0.
//
// Verschaerfend lasen drei Asynq-Inspectors (Admin-Jobs, Admin-Health,
// Prometheus) ebenfalls ohne DB. Ein Health-Check, der die falsche Datenbank
// prueft, meldet "gesund" fuer Warteschlangen, die er gar nicht ansieht — das
// ist schlimmer als kein Health-Check.
//
// Die Lehre ist nicht "das Feld nicht vergessen", sondern: eine Struktur, die an
// acht Stellen von Hand gebaut wird, wird an der neunten wieder falsch gebaut.
// Deshalb gibt es die Abbildung genau einmal, und ein Gate
// (cmd/api/asynq_redis_db_coverage_test.go) haelt sie dort.
package redisopt

import (
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// defaultAddr ist die Entwicklungs-Ausweichadresse, wenn VAKT_REDIS_URL leer ist.
const defaultAddr = "localhost:6379"

// Asynq bildet bereits geparste go-redis-Optionen auf Asynq-Verbindungsoptionen ab.
//
// Das ist die einzige Stelle im Baum, die diese Felder aufzaehlt. Wer hier ein
// Feld vergisst, vergisst es ueberall — und genau das ist gewollt: ein Fehler an
// einer Stelle ist auffindbar, derselbe Fehler an acht Stellen ist es nicht.
//
// Uebertragen werden alle Felder, die ueber die Zieldatenbank oder das
// Zustandekommen der Verbindung entscheiden:
//
//   - DB — der Defekt, um den es hier geht.
//   - Username/Password — Redis-ACL bzw. --requirepass (S121-C3/S122-B2).
//   - TLSConfig — rediss://. Asynq verhandelt TLS ausschliesslich, wenn dieses
//     Feld gesetzt ist; die alte Ableitung im Worker liess es fallen, also war
//     jede rediss://-Konfiguration dort de facto Klartext-oder-nichts.
//   - Network — unix-Sockets.
//
// Zeitlimits und Poolgroessen werden bewusst NICHT uebertragen: go-redis und
// Asynq haben dafuer verschiedene Vorgaben, und ein durchgereichter Wert waere
// eine stille Verhaltensaenderung ohne Anlass.
//
// Ein nil-Zeiger ergibt die Entwicklungs-Ausweichadresse — dieselbe Semantik wie
// eine leere URL.
func Asynq(o *redis.Options) asynq.RedisClientOpt {
	if o == nil {
		// redisauth-ok: Ausweichadresse fuer die lokale Entwicklung ohne
		// konfigurierte URL; ein Passwort gibt es hier per Konstruktion nicht.
		// redisdb-ok: ohne URL gibt es keine Datenbanknummer, DB 0 ist richtig.
		return asynq.RedisClientOpt{Addr: defaultAddr}
	}
	return asynq.RedisClientOpt{
		Network:   o.Network,
		Addr:      o.Addr,
		Username:  o.Username,
		Password:  o.Password,
		DB:        o.DB,
		TLSConfig: o.TLSConfig,
	}
}

// AsynqFromURL leitet die Asynq-Optionen direkt aus VAKT_REDIS_URL ab.
//
// Akzeptiert beide Schreibweisen, die in freier Wildbahn vorkommen: die volle
// URL (redis://:pass@host:port/1, rediss://…) und die nackte Adresse (host:port).
// Die Ausweichpfade sind die des Workers vor der Zusammenlegung, damit die
// Umstellung keine Verhaltensaenderung ist:
//
//	""                 → localhost:6379, DB 0
//	geparste URL       → Adresse/Zugangsdaten/DB/TLS aus der URL
//	nackte host:port   → genau diese Adresse, DB 0
func AsynqFromURL(redisURL string) asynq.RedisClientOpt {
	if redisURL == "" {
		return Asynq(nil)
	}
	if parsed, err := redis.ParseURL(redisURL); err == nil {
		return Asynq(parsed)
	}
	// Nackte host:port-Form ohne Schema — unveraendert durchreichen.
	//
	// redisdb-ok: eine nackte Adresse traegt keine Datenbanknummer; DB 0 ist hier
	// die Vorgabe von Redis selbst und kein verlorener Wert.
	// redisauth-ok: sie traegt aus demselben Grund kein Passwort; ein
	// auth-geschuetztes Redis muss als redis://:pass@host konfiguriert werden
	// (die ausgelieferte Compose-Vorgabe), und das nimmt den ParseURL-Zweig.
	return asynq.RedisClientOpt{Addr: redisURL}
}

// GoRedis liefert eine Kopie der Optionen, die gefahrlos an redis.NewClient
// gegeben werden kann.
//
// go-redis fuellt beim ersten Gebrauch Vorgabewerte in die uebergebene Struktur;
// denselben Zeiger an zwei Clients zu geben, verknuepft deren Konfiguration
// stillschweigend. Die Kopie kostet nichts und schliesst das aus.
// Ein nil-Zeiger ergibt nil — der Aufrufer entscheidet, ob "kein Redis" ein
// Fehler ist oder ein zulaessiger Zustand.
func GoRedis(o *redis.Options) *redis.Options {
	if o == nil {
		return nil
	}
	cp := *o
	return &cp
}
