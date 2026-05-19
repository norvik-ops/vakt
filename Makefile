.PHONY: dev api-local frontend-local stop stop-local test lint build migrate seed seed-local backup

# ── Docker-based dev (requires Docker) ─────────────────────────────────────
dev:
	docker compose -f docker-compose.dev.yml up --build

stop:
	docker compose -f docker-compose.dev.yml down

# ── Native dev (requires local Postgres + Redis) ────────────────────────────
# First-time setup: sudo pacman -S postgresql redis
#   sudo -u postgres initdb -D /var/lib/postgres/data
#   sudo systemctl start postgresql redis
#   sudo -u postgres psql -c "CREATE USER sechealth WITH PASSWORD 'sechealth';;"
#   sudo -u postgres psql -c "CREATE DATABASE sechealth OWNER sechealth;"
LOCAL_DB  := postgres://sechealth:sechealth@localhost:5432/sechealth?sslmode=disable
LOCAL_ENV := VAKT_DB_URL="$(LOCAL_DB)" \
             VAKT_REDIS_URL="redis://localhost:6379" \
             VAKT_SECRET_KEY="d7463ee089bc65fac0efe91ee13b88413e256de2151228eeebee4787e5d276f7" \
             VAKT_MODULES_ENABLED="secpulse,secvitals,secvault,secreflex,secprivacy" \
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

backup: ## Create a timestamped backup archive (PostgreSQL dump + encrypted key)
	@bash scripts/backup.sh .

restore: ## Restore from a backup archive: make restore BACKUP=<file.tar.gz>
	@bash scripts/restore.sh $(BACKUP)

backup-verify: ## Verify backup integrity without restoring: make backup-verify BACKUP=<file.tar.gz>
	@bash scripts/backup-verify.sh $(BACKUP)
