# Skalierung & Sizing

Wie Vakt mit wachsender Nutzerzahl umgeht — Single-Instance-Grenzen, wann vertikal
vs. horizontal skaliert wird, und welche Voraussetzungen Multi-Instance erfüllen muss.

Verwandt: [`pgbouncer.md`](pgbouncer.md) (Connection-Pooling), [`redis-ha.md`](redis-ha.md)
(Redis-Hochverfügbarkeit). Diese Doku dupliziert beide nicht, sondern ergänzt die
**App-Statelessness** und die Sizing-Empfehlung.

## Single-Instance-Grenzen

Eine einzelne Instanz (1× API + 1× Worker + Postgres + Redis) deckt den typischen
KMU-Fall locker ab:

| Ressource | Empfehlung | Anmerkung |
|-----------|-----------|-----------|
| vCPU | 4 | API-Hotpaths sind server-seitig ~30–60 ms |
| RAM | 8 GB | **Haupttreiber ist Ollama (~4,5 GB)**, nicht die App |
| Parallelnutzer | 5–50 | bequem auf 4 vCPU / 8 GB |

Ohne lokale KI (`VAKT_AI_PROVIDER=disabled`, Ollama-Container gestoppt) sinkt der
RAM-Bedarf auf ~2–3 GB — die Go-App selbst ist schlank.

## Vertikal vs. horizontal

- **Bis ~100 Parallelnutzer: vertikal skalieren.** Mehr vCPU/RAM auf einer Instanz ist
  einfacher zu betreiben und reicht für nahezu alle Self-Hosted-KMU.
- **Darüber: horizontal** (mehrere API-Replicas hinter einem Load-Balancer) — **aber erst
  nachdem** die Notifications-SSE auf Push umgestellt ist (siehe nächster Abschnitt).

## Voraussetzung für Multi-Instance: SSE-Push (S98-5)

Der Notifications-SSE-Stream nutzt seit S98-5 **Redis Pub/Sub** (Push) statt eines
2-s-DB-Polls. Das ist die Bedingung für horizontale Skalierung: Beim alten Poll hätte
**jede** API-Replica die DB-Grundlast pro offenem Tab multipliziert. Mit Pub/Sub bleibt
die DB-Last **O(Events)** statt O(Nutzer × Replicas).

Fällt Redis aus, fällt der Stream automatisch auf den 2-s-Poll zurück (kein harter Bruch),
aber für Multi-Instance-Dauerbetrieb sollte Redis hochverfügbar sein → [`redis-ha.md`](redis-ha.md).

## Stateless-Checkliste

Vakt hält **keinen** Sitzungs-State im Prozess-Memory. Alles, was zwischen Requests
überlebt, liegt extern:

| State | Wo | Konsequenz für LB |
|-------|----|--------|
| Auth-Sessions | Paseto-Token (clientseitig) | kein Sticky-Session nötig |
| Rate-Limiter | Redis | geteilt über alle Replicas |
| Cache (Dashboard-Aggregate, Score) | Redis | geteilt, mit Invalidierung |
| Notifications | Postgres + Redis Pub/Sub | Push an alle Replicas |
| Datei-Uploads (Evidence) | Volume `uploads_data` | **gemeinsames Volume / Objektspeicher** mounten |

**Load-Balancer:** Sticky-Sessions sind nicht nötig. Einzige Beachtung: SSE-Verbindungen
(`/notifications/stream`) sind langlebig — Idle-Timeout des LB großzügig setzen und Buffering
deaktivieren (analog `X-Accel-Buffering: no`, siehe `reverse-proxy.md`).

## Sizing-Tabelle je Firmengröße

| Mitarbeiter | vCPU / RAM | DB | Topologie |
|-------------|-----------|----|-----------| 
| bis 100 | 4 / 8 GB | Single Postgres | Single-Instance |
| bis 500 | 8 / 16 GB | Postgres + pgBouncer | Single-Instance (vertikal), optional 2× API |
| 1000+ | 2–3× API (4 / 8 GB je) | Postgres (ggf. Read-Replica) + pgBouncer + Redis-Sentinel | Horizontal hinter LB, SSE-Push aktiv |

RAM-Zahlen **mit** lokaler KI (Ollama). Ohne KI jeweils ~4,5 GB abziehen.

## Worker-Skalierung

Der Asynq-Worker ist aktuell als Single-Instance ausgelegt (Scheduler + Processor in
einem Prozess). Für sehr große Installationen ist horizontale Worker-Skalierung
(Leader-Election oder separater Scheduler-Pod) als v1.0-Track-Item vorgemerkt
(OPS-H03 im Backlog) — für KMU-Lasten nicht erforderlich.
