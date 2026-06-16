.PHONY: dev api-local frontend-local stop stop-local test lint build migrate seed seed-local backup public-mirror rotate-key

# ── Docker-based dev (requires Docker) ─────────────────────────────────────
dev:
	docker compose -f docker-compose.dev.yml up --build

# ── Public Mirror — materialisiert lokal das, was nach norvik-ops/vatk synct
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
