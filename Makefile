.PHONY: dev api-local frontend-local stop stop-local test lint build migrate seed seed-local backup public-mirror rotate-key install-hooks

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

test:
	cd backend && go test ./...
	cd frontend && npm test
	bash scripts/restore_test.sh
	bash scripts/backup_cron_test.sh

test-restore: ## S89-1: restore.sh hardening shell test (key-leak + tamper checks)
	@bash scripts/restore_test.sh

test-backup: ## S89-4: backup-cron.sh retention + notification shell test
	@bash scripts/backup_cron_test.sh

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
	cd frontend && npm run build
	@echo "✓ DoD gruen"

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

backup-verify: ## Verify backup integrity without restoring: make backup-verify BACKUP=<file.tar.gz>
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
