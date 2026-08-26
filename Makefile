.PHONY: dev api-local frontend-local stop stop-local test lint build migrate seed seed-local backup public-mirror rotate-key install-hooks test-restore test-backup test-deploy-gate test-backup-wiring test-offsite-order

# Lokale Overrides fuer interne Ops-Ziele (z.B. BILLING_HOST). Gitignored und NICHT
# im oeffentlichen Mirror — Infra-Namen gehoeren nicht ins Kunden-Repo.
-include Makefile.local

# ── Docker-based dev (requires Docker) ─────────────────────────────────────
dev:
	docker compose -f docker-compose.dev.yml up --build

# ── Public Mirror — materialisiert lokal das, was nach norvik-ops/vakt synct
# Verifiziert mit `go build ./...` dass das Mirror kompiliert.
# Output: ./public-mirror/ (gitignored)
public-mirror:
	@./scripts/build-public-mirror.sh

stop:
	docker compose -f docker-compose.dev.yml down

# ── Native dev (requires local Postgres + Redis) ────────────────────────────
# First-time setup: sudo pacman -S postgresql redis
#   sudo -u postgres initdb -D /var/lib/postgres/data
#   sudo systemctl start postgresql redis
#   sudo -u postgres psql -c "CREATE USER vakt WITH PASSWORD 'vakt';;"
#   sudo -u postgres psql -c "CREATE DATABASE vakt OWNER vakt;"
LOCAL_DB  := postgres://vakt:vakt@localhost:5432/vakt?sslmode=disable
LOCAL_ENV := VAKT_DB_URL="$(LOCAL_DB)" \
             VAKT_REDIS_URL="redis://localhost:6379" \
             VAKT_SECRET_KEY="d7463ee089bc65fac0efe91ee13b88413e256de2151228eeebee4787e5d276f7" \
             VAKT_MODULES_ENABLED="vaktscan,vaktcomply,vaktvault,vaktaware,vaktprivacy" \
             AUTO_MIGRATE=true \
             APP_VERSION=0.1.0 \
             VAKT_API_PORT=8080

api-local:
	cd backend && $(LOCAL_ENV) go run ./cmd/api

frontend-local:
	cd frontend && npm run dev

stop-local:
	@pkill -f "go run ./backend/cmd/api" 2>/dev/null || true
	@pkill -f "vite" 2>/dev/null || true
	@echo "stopped"

migrate-local:
	cd backend && VAKT_DB_URL="$(LOCAL_DB)" go run ./cmd/migrate

seed-local:
	cd backend && SEED_ENV=development VAKT_DB_URL="$(LOCAL_DB)" go run ./cmd/seed

# ── Suiten des internen Sites-Stacks (infra/server/) ────────────────────────
#
# `infra/server/` ist der NorvikOps-Sites-Stack, NICHT das Kundenprodukt (CLAUDE.md).
# `scripts/build-public-mirror.sh` spiegelt `infra/` deshalb bewusst nicht — und
# KANN es auch nicht: `vakt-deploy.sh` traegt den internen Deploy-Arbeitspfad,
# `backup-offsite.sh` den internen Monitoring-Hostnamen; beides sind Muster des
# LEAK_PATTERN in genau diesem Skript. Ein `rsync infra/` wuerde den Mirror-Build
# hart abbrechen (gemessen), statt ihn zu reparieren.
#
# Dieses Makefile wird aber 1:1 gespiegelt. Eine harte `bash infra/...`-Zeile in
# `test:` ist damit im Kundenrepo ein garantiertes "No such file or directory" —
# zweimal passiert (#67, dann #76), und beim zweiten Mal starb `make test` an der
# dritten Zeile, bevor die vierte (im Mirror vorhandene und lauffaehige) Suite
# ueberhaupt drankam. `$(wildcard …)` loest genau das: im privaten Repo ist die
# Menge voll, im Mirror leer — und die uebersprungenen werden GEZAEHLT und
# GENANNT, statt still zu verschwinden.
#
# Durchgesetzt wird die Invariante von `scripts/check_mirror_make_test.py`
# (baut den Mirror und prueft `make test` darin auf).
#
# Beide Suiten laufen im privaten CI hart: ci.yml `backend` — vakt-deploy_test.sh
# als "vakt-deploy.sh SSH-Command-Gate (R1-29-O01)", backup-offsite_test.sh als
# "backup-offsite.sh Retention-Reihenfolge (ESK-7)". Ihre Abwesenheit im Mirror
# kostet also keine Durchsetzung, sie nimmt dem Kunden nur einen Test fuer ein
# Skript, das er gar nicht hat.
INTERNAL_TEST_SUITES_EXPECTED := infra/server/scripts/vakt-deploy_test.sh \
                                 infra/server/scripts/backup-offsite_test.sh
INTERNAL_TEST_SUITES         := $(wildcard $(INTERNAL_TEST_SUITES_EXPECTED))
INTERNAL_TEST_SUITES_MISSING := $(filter-out $(INTERNAL_TEST_SUITES),$(INTERNAL_TEST_SUITES_EXPECTED))

# $(call internal_only,<suite>) — fuer die Einzelziele, die GENAU eine dieser
# Suiten sind. Anders als in `test:` ist Ueberspringen hier falsch: wer
# `make test-deploy-gate` tippt, will diese eine Suite und muss erfahren, dass
# es sie in diesem Repo nicht gibt, statt ein stilles OK zu bekommen.
internal_only = $(if $(wildcard $(1)),bash $(1),{ echo "$(1): interner Sites-Stack (infra/server/), wird nicht in den oeffentlichen Mirror gespiegelt — dieses Ziel gibt es hier nicht."; exit 1; })

test:
	cd backend && go test ./...
	cd frontend && npm test
	bash scripts/restore_test.sh
	bash scripts/backup_cron_test.sh
	bash scripts/update_artifacts_test.sh
	bash scripts/backup_restore_wiring_test.sh
	@for t in $(INTERNAL_TEST_SUITES); do echo "bash $$t"; bash "$$t" || exit 1; done
	@echo "interner Sites-Stack: $(words $(INTERNAL_TEST_SUITES))/$(words $(INTERNAL_TEST_SUITES_EXPECTED)) Suiten gelaufen · skipped: $(words $(INTERNAL_TEST_SUITES_MISSING)) $(INTERNAL_TEST_SUITES_MISSING)"

test-restore: ## S89-1: restore.sh hardening shell test (key-leak + tamper checks)
	@bash scripts/restore_test.sh

test-backup: ## S89-4: backup-cron.sh retention + notification shell test
	@bash scripts/backup_cron_test.sh

test-deploy-gate: ## R1-29-O01: vakt-deploy.sh SSH command gate (injection + dispatch)
	@$(call internal_only,infra/server/scripts/vakt-deploy_test.sh)

test-backup-wiring: ## ESK-7: Kunden-Backup-/Restore-Weg gegen echtes Postgres (braucht Docker)
	@bash scripts/backup_restore_wiring_test.sh

test-offsite-order: ## ESK-7: Retention darf nicht vor der Mindestgroessen-Assertion laufen
	@$(call internal_only,infra/server/scripts/backup-offsite_test.sh)

lint:
	cd backend && golangci-lint run ./...
	cd frontend && npm run lint

# Setzt core.hooksPath, statt einzelne Dateien nach .git/hooks/ zu kopieren.
#
# Die alte Fassung kopierte NUR scripts/hooks/pre-commit und liess .githooks/pre-push
# unberuehrt — PROCESS.md verwies aber genau darauf ("aktivieren mit make install-hooks").
# Der pre-push-Hook lief damit bei niemandem, auch nicht auf der Maschine des Autors.
# Zwei Hook-Verzeichnisse nebeneinander waren die Ursache; jetzt gibt es nur noch eines.
#
# core.hooksPath statt cp hat einen zweiten Vorteil: Ein spaeter hinzugefuegter Hook ist
# sofort aktiv, ohne dass jemand install-hooks erneut aufruft. Ein kopierter Hook driftet
# still vom Repo weg — dieselbe Klasse wie server-lokale Ops-Skripte.
install-hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/*
	@echo "Hooks aktiv (core.hooksPath=.githooks): $$(ls .githooks | tr '\n' ' ')"
	@echo "Deaktivieren: git config --unset core.hooksPath"

# Die DoD-Kette aus PROCESS.md P7, in EINEM Befehl — genau das, was .githooks/pre-push
# aufruft. Vorher rief der Hook `make check` gegen ein Target, das es nicht gab: Er haette
# jeden Push mit einem Make-Fehler blockiert, waere er je gelaufen.
#
# Bewusst OHNE `make lint`: golangci-lint ist nicht auf jeder Maschine installiert, und ein
# Hook, der an einem fehlenden Werkzeug scheitert, wird abgeschaltet statt gefixt. Lint
# laeuft in CI und ist dort Merge-Bedingung.
check:
	cd backend && go build ./...
	cd backend && go vet ./...
	@cd backend && u=$$(gofmt -l . | grep -v '^spike/' || true); \
	  if [ -n "$$u" ]; then echo "gofmt noetig:"; echo "$$u" | sed 's/^/  /'; exit 1; fi
	cd backend && go test ./...
	cd frontend && npm test
	cd frontend && npm run build
	$(MAKE) gates
	@echo "✓ DoD gruen"

# AUFNAHMEREGEL (bewusst als Regel formuliert, nicht als Liste): hier laeuft
# JEDES Gate aus ci.yml UND docs.yml, das
#   (1) ohne Netz auskommt,
#   (2) ohne DOCKER auskommt, und
#   (3) kein Werkzeug ausser python3 (+ PyYAML), bash, coreutils und tar braucht.
# Wer ein Gate hinzufuegt, das die Regel erfuellt, gehoert hierher; wer eines
# ausschliesst, nennt unten den Grund.
#
# (2) ist seit 2026-07-30 ein eigenes Kriterium, nicht mehr unter "Werkzeug"
# versteckt: `gates` laeuft aus `make check`, also im pre-push-Hook auf einem
# Entwicklerrechner. Ein Gate, das ohne laufenden Docker-Daemon mit "nicht
# messbar" abbricht, blockiert korrekte Arbeit — dieselbe Begruendung, aus der
# check_image_tags.py (Netz) und `make lint` (golangci-lint) draussen sind. Ein
# Hook, der an einer Umgebung scheitert statt an Code, wird abgeschaltet.
#
# PyYAML ist die einzige Bibliotheks-Abhaengigkeit, und nur check_ci_steps.py
# braucht sie — es gibt keinen YAML-Parser in der Standardbibliothek. Fehlt sie,
# nennt das Gate den `pip install`-Befehl statt zu raten oder zu ueberspringen.
#
# Warum das Target existiert: `make check` ist der pre-push-DoD-Gate
# (.githooks/pre-push) und fuhr KEIN einziges Gate — nur build/vet/gofmt/
# go test/npm build. Ein Gate-Bruch fiel damit erst im PR-CI auf, also nach dem
# Push. PROCESS.md P7 verlangt die Gates auf dem Integrationsstand; lokal vorher
# zu wissen, dass sie halten, kostet 10 Sekunden.
#
# Gruppiert nach Bereich, NICHT in ci.yml-Reihenfolge — die beiden Doku-Gates
# stammen ohnehin aus einem anderen Workflow (docs.yml, required Kontext
# "Doc consistency (drift + links)"), eine gemeinsame Reihenfolge gibt es nicht.
#
# NICHT enthalten — sieben Ausnahmen, jede mit einem Netz-, Docker- oder
# Werkzeuggrund. Alle ausser der letzten laufen in ci.yml, sind also durchgesetzt,
# nur nicht hier:
#   · check_image_tags.py        — fragt die Docker-Hub-Tags-API, braucht Netz.
#   · restore_test.sh            — braucht gpg + openssl.
#   · vakt-deploy_test.sh        — braucht rsync + rrsync.
#   · shellcheck                 — nicht auf jeder Maschine installiert.
#   · check_sqlc_seam.py         — braucht Docker: es generiert mit sqlc in ein
#     Tempdir und nimmt sqlc als Orakel. Ohne Docker Exit 2 („nicht
#     vergleichbar"), ausdruecklich KEIN OK — gemessen: Schicht A (491 Queries,
#     Mengendiff) laeuft trotzdem, nur Schicht B (SQL-Vergleich) faellt aus.
#   · check_sqlc_seam_test.py    — braucht Docker ebenfalls. Gemessen ohne
#     Docker: 6 der 8 Faelle bestehen, aber Fall 1 und 2 (die das echte
#     sqlc-Orakel brauchen) fallen aus -> rc=1. Ein Teil-Selbsttest, der als
#     Ganzes rot ist, taugt nicht fuer den Hook; drum ganz draussen statt
#     bedingt uebersprungen (ein bedingter Skip wuerde Erfolg fuer ungetane
#     Arbeit melden).
#   · backup_restore_wiring_test.sh — braucht einen ECHTEN Compose-Stack
#     (31 docker-compose/run/exec-Aufrufe, Minuten). ACHTUNG: dieses Skript
#     laeuft in KEINEM Workflow, nur in `make test` — es ist damit nirgends
#     durchgesetzt. Das ist ein offener Befund aus PR #76, kein Zustand, den
#     diese Ausnahme gutheisst; es gehoert in den `integration`-Job (der hat
#     Docker), nicht hierher.
# backup_cron_test.sh stand hier bis 2026-07-30 faelschlich mit unter „braucht
# gpg" — es braucht nur `tar` (0,03 s) und ist aufgenommen; das war
# Bequemlichkeit, keine Begruendung. Ebenso neu aufgenommen:
# backup-offsite_test.sh (kein Docker, 0,39 s, gemessen mit docker AUS dem PATH).
#
# EINE benannte Ausnahme von (3): check_mirror_make_test.py braucht `rsync` und
# `make` zusaetzlich zu python3/bash/coreutils — es baut den Public Mirror. Beides
# ist auf jeder Entwicklermaschine und in jedem CI-Image da; und wo nicht, meldet
# das Gate `nicht pruefbar: <werkzeug> fehlt` und endet mit exit 0, statt korrekte
# Arbeit zu blockieren. Es faehrt den go-Build des Mirrors NICHT (Laufzeit) —
# den prueft sync-public-repo.yml. Gemessen: ~2 s.
#
# Die Durchsetzung bleibt ci.yml/docs.yml, nicht dieses Target: hier kann ein
# Gate hoechstens fehlen, es kann nicht heimlich uebergangen werden. Genau
# deshalb faehrt `gates` auch check_ci_steps.py — das Gate, das prueft, dass die
# Workflows ihre Jobs und Steps nicht verlieren.
#
# N-02 (2026-07-30): check_ci_steps_test.py erfuellte die Aufnahmeregel oben
# (kein Netz, kein Docker, nur python3) und stand trotzdem in KEINEM Make-Ziel
# und in KEINEM Workflow — der Regressionstest, der Pruefung E festhaelt, fuhr
# also nirgends. Das ist die Klasse, die PR #77 fuer acht Gates geschlossen hat;
# sie kam hier zum vierten Mal in diesem Lauf zurueck, weil die Aufnahmeregel
# beim Anlegen eines neuen Selbsttests niemand automatisch vorliest. Jetzt
# aufgenommen. Der Gegenstueck-Schritt in ci.yml fehlt noch und gehoert dem
# Reviewer (siehe GATE INVENTORY in scripts/check_ci_steps.py): bis dahin ist
# dieser Selbsttest nur ueber den pre-push-Hook durchgesetzt, nicht als
# required Status-Check.
.PHONY: gates
gates:
	python3 scripts/check_ci_steps.py
	python3 scripts/check_ci_steps_test.py
	bash scripts/ci_status_test.sh
	python3 scripts/check_workflow_exit_capture.py
	python3 scripts/lint-orgid-queries.py --raw-sql
	python3 scripts/lint_orgid_queries_test.py
	python3 scripts/lint-sql-shapes.py
	python3 scripts/check_routes.py
	python3 scripts/check_fe_be_fields.py
	python3 scripts/check_fe_be_fields_test.py
	python3 scripts/check_adr_numbers.py
	python3 scripts/check_adr_numbers_test.py
	python3 scripts/check_defect_ledger.py
	python3 scripts/check_defect_ledger_test.py
	python3 scripts/check_release_gate_test.py
	python3 scripts/check_interface_ratchet.py
	python3 scripts/check_sqlc_frozen.py
	python3 scripts/check_module_isolation.py
	python3 scripts/check_module_isolation_test.py
	python3 scripts/check_openapi_coverage.py
	python3 scripts/check_outbound_security.py
	python3 scripts/check_user_role_insert.py
	python3 scripts/check_response_shape.py
	python3 scripts/check_worker_wiring.py
	python3 scripts/check_evidence_flow.py
	python3 scripts/check_backup_hardening.py
	python3 scripts/check_backup_hardening_test.py
	python3 scripts/check_image_tags_test.py
	python3 scripts/check_fe_csrf.py
	python3 scripts/check-docs.py
	python3 scripts/check-i18n-drift.py
	python3 scripts/check_feature_tiers.py
	python3 scripts/check_feature_tiers.py --selftest
	python3 scripts/check_mirror_make_test.py
	python3 scripts/check_mirror_make_test.py --selftest
	python3 scripts/check_mirror_refs.py
	python3 scripts/check_mirror_refs.py --selftest
	python3 scripts/check_quickstart_secrets.py
	python3 scripts/check_quickstart_secrets.py --selftest
	python3 scripts/check_helm_image_tags.py --teil-a-genuegt
	python3 scripts/check_helm_image_tags.py --selftest
	python3 scripts/check_billing_binary.py
	python3 scripts/check_billing_binary.py --selftest
	python3 scripts/check_upgrade_docs.py
	python3 scripts/check_upgrade_docs.py --selftest
	bash scripts/build_public_mirror_guard_test.sh
	bash scripts/backup_doc_calls_test.sh
	bash scripts/backup_cron_test.sh
	@for t in $(wildcard infra/server/scripts/backup-offsite_test.sh); do echo "bash $$t"; bash "$$t" || exit 1; done
	@n=$$(awk '/^gates:/{f=1;next} f&&/^[^\t]/{exit} f&&/(python3|bash) /{c++} END{print c+0}' Makefile); \
	  echo "✓ Gates gruen ($$n Aufrufe). NICHT hier gelaufen, weil Netz/Docker/Werkzeug:"
	@echo "   check_image_tags.py (Netz) · check_sqlc_seam.py + _test.py (Docker)"
	@echo "   restore_test.sh (gpg) · vakt-deploy_test.sh (rrsync) · shellcheck"
	@echo "   -> alle in ci.yml. backup_restore_wiring_test.sh (Compose) in KEINEM Workflow."
	@echo "   HIER durchgesetzt, aber in KEINEM Workflow (N-02): check_ci_steps_test.py"
	@echo "   -> nur ueber den pre-push-Hook, NICHT als required Status-Check. Die"
	@echo "      fehlende ci.yml-Zeile steht im GATE INVENTORY von check_ci_steps.py."
	@echo "   TEILWEISE gemessen: check_helm_image_tags.py laeuft hier mit"
	@echo "   --teil-a-genuegt. Teil A (Templates UND Helper-Koerper, statisch)"
	@echo "   laeuft immer; Teil B (helm template, 6 Faelle) nur mit helm im PATH"
	@echo "   UND aufgeloesten Subcharts. Ohne das Flag endet der Lauf hier in"
	@echo "   rc=2 'NICHT MESSBAR' — das ist Absicht, es ist kein Erfolg. Im Job"
	@echo "   'helm-chart' (helm + Subcharts vorhanden) gehoert der Aufruf OHNE"
	@echo "   Flag hin; die Zeile steht im GATE INVENTORY von check_ci_steps.py."

build:
	cd backend && go build ./...
	cd frontend && npm run build

migrate:
	cd backend && go run ./cmd/api -migrate

seed:
	cd backend && go run ./cmd/seed

rotate-key: ## Rotate the master encryption key: make rotate-key [NEW_KEY=<hex>]
	@bash scripts/rotate-key.sh

backup: ## Create a timestamped backup archive (PostgreSQL dump + encrypted key)
	@bash scripts/backup.sh .

restore: ## Restore from a backup archive: make restore BACKUP=<file.tar.gz>
	@bash scripts/restore.sh $(BACKUP)

backup-verify: ## Verify backup integrity: VAKT_BACKUP_PASSPHRASE[_FILE]=… make backup-verify BACKUP=<file.tar.gz>
	@bash scripts/backup-verify.sh $(BACKUP)

support-bundle: ## Collect logs + health into a support archive: make support-bundle [TAIL=2000] [SINCE=30m]
	@bash scripts/support-bundle.sh .

## billing: Billing-Admin-Panel im Browser oeffnen (SSH-Tunnel, kein Setup noetig)
##
## Das Panel lauscht auf 127.0.0.1 IM Container — es ist aus dem Internet nicht
## erreichbar, auch wenn jemand die Firewall vergisst. Wer SSH auf den Server hat,
## ist ohnehin drin; es gibt keinen zweiten Login, der falsch gebaut sein koennte.
##
## Browser-/Handy-Zugriff ohne Tunnel braucht Cloudflare Access — siehe
## docs/dev/billing-admin.md. Bis dahin: dieser Befehl.
##
## BILLING_HOST steht NICHT hier drin: Dieses Makefile wird in den oeffentlichen
## Mirror gespiegelt, und der Leak-Guard (scripts/build-public-mirror.sh) bricht den
## Sync ab, sobald ein NorvikOps-Infra-Name darin auftaucht. Genau das ist passiert —
## der Mirror hing fest, und der Fix fuer `docker compose up` erreichte tagelang
## keinen Kunden. Der Hostname gehoert in Makefile.local (gitignored, nicht gespiegelt):
##
##     echo 'BILLING_HOST = mein-server' > Makefile.local
.PHONY: billing
billing:
	@[ -n "$(BILLING_HOST)" ] || { \
		echo "BILLING_HOST ist nicht gesetzt."; \
		echo "  echo 'BILLING_HOST = <host>' > Makefile.local"; \
		exit 1; }
	@echo "→ Tunnel nach $(BILLING_HOST):8099 …"
	@echo "→ Panel:  http://localhost:8099   (Strg-C beendet den Tunnel)"
	@(sleep 2 && (xdg-open http://localhost:8099 2>/dev/null || open http://localhost:8099 2>/dev/null || true)) &
	@ssh -N -L 8099:localhost:8099 $(BILLING_HOST)
