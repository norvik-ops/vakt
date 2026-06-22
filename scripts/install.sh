#!/usr/bin/env bash
set -euo pipefail

HEALTH_URL="http://localhost/health/ready"
MAX_WAIT=180
POLL_INTERVAL=2

# ── Swap (Linux only, best-effort, skip if already present) ─────────────────
if [ "$(uname)" = "Linux" ] && [ "$(id -u)" = "0" ]; then
	if ! swapon --show | grep -q .; then
		echo "No swap detected — creating 4 GB swapfile..."
		fallocate -l 4G /swapfile 2>/dev/null || dd if=/dev/zero of=/swapfile bs=1M count=4096 2>/dev/null || true
		if [ -f /swapfile ] && [ "$(stat -c %s /swapfile 2>/dev/null || echo 0)" -ge 1073741824 ]; then
			chmod 600 /swapfile
			mkswap /swapfile
			swapon /swapfile
			grep -qF '/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
			echo "Swap enabled (4 GB, persistent)"
		else
			echo "WARNING: could not create swapfile — continuing without swap"
		fi
	fi
fi

# ── Dependency checks ────────────────────────────────────────────────────────
if ! command -v docker &>/dev/null; then
	echo "ERROR: Docker is not installed. See https://docs.docker.com/get-docker/"
	exit 1
fi

if docker compose version &>/dev/null 2>&1; then
	COMPOSE_CMD="docker compose"
elif docker-compose version &>/dev/null 2>&1; then
	COMPOSE_CMD="docker-compose"
else
	echo "ERROR: docker compose (v2) or docker-compose (v1) is required."
	exit 1
fi

echo "Vakt installer"
echo "---------------------"

# ── .env bootstrap ───────────────────────────────────────────────────────────
if [ ! -f .env ]; then
	if [ ! -f .env.example ]; then
		echo "ERROR: .env.example not found. Are you in the Vakt project root?"
		exit 1
	fi
	cp .env.example .env
	echo "Created .env from .env.example"
fi

# Generate secret key if placeholder or empty
if grep -qE '^VAKT_SECRET_KEY=($|changeme|ERSETZEN_SIE_DIESEN_WERT)' .env; then
	SECRET=$(openssl rand -hex 32)
	# Replace the line in-place (works on both Linux and macOS)
	sed -i.bak "s|^VAKT_SECRET_KEY=.*|VAKT_SECRET_KEY=${SECRET}|" .env
	rm -f .env.bak
	echo "Generated VAKT_SECRET_KEY"
fi

# Generate POSTGRES_PASSWORD if placeholder or empty
if grep -qE '^POSTGRES_PASSWORD=($|changeme|vakt|ERSETZEN_SIE_DIESEN_WERT)' .env; then
	POSTGRES_PASSWORD=$(openssl rand -hex 16)
	sed -i "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=$POSTGRES_PASSWORD/" .env
	echo "Generated POSTGRES_PASSWORD"
fi

# Generate REDIS_PASSWORD if placeholder or empty, and inject into VAKT_REDIS_URL
if grep -qE '^REDIS_PASSWORD=($|changeme|ERSETZEN_SIE_DIESEN_WERT)' .env; then
	REDIS_PASSWORD=$(openssl rand -hex 16)
	sed -i "s/^REDIS_PASSWORD=.*/REDIS_PASSWORD=$REDIS_PASSWORD/" .env
	# Replace ERSETZEN_SIE_DIESEN_WERT in the VAKT_REDIS_URL with the generated password
	sed -i "s|^VAKT_REDIS_URL=redis://:[^@]*@|VAKT_REDIS_URL=redis://:$REDIS_PASSWORD@|" .env
	echo "Generated REDIS_PASSWORD"
fi

# ── Start stack ──────────────────────────────────────────────────────────────
echo "Starting Vakt..."
# Add --profile ai to enable local AI features (requires GPU/significant RAM)
$COMPOSE_CMD up -d

# ── Wait for health check ────────────────────────────────────────────────────
echo "Waiting for API to become healthy (max ${MAX_WAIT}s)..."
elapsed=0
while true; do
	if curl -sf "${HEALTH_URL}" >/dev/null 2>&1; then
		echo ""
		echo "Vakt is running!"
		echo ""
		echo "  Dashboard: http://localhost"
		echo ""
		echo "First-run wizard: http://localhost/setup"
		exit 0
	fi

	if [ "$elapsed" -ge "$MAX_WAIT" ]; then
		echo ""
		echo "ERROR: API did not respond within ${MAX_WAIT}s."
		echo "Check logs: docker compose logs api"
		exit 1
	fi

	printf "."
	sleep "$POLL_INTERVAL"
	elapsed=$((elapsed + POLL_INTERVAL))
done
