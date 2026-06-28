#!/usr/bin/env bash
set -euo pipefail

# S89-4: Backup automation wrapper. Runs backup.sh, verifies the result, pushes
# off-site (opt-in, customer-configured), prunes old archives by retention, and
# notifies on any failure. Intended to be invoked by a scheduler (host cron or
# the optional docker-compose.backup.yml service).
#
# Configuration (env):
#   VAKT_BACKUP_DIR              target dir for archives          (default /backups/vakt)
#   VAKT_BACKUP_RETENTION_DAYS   delete archives older than N days (default 30)
#   VAKT_BACKUP_OFFSITE_CMD      opt-in off-site push command; runs with $ARCHIVE
#                                and $SIG set. Customer-configured, NO Norvik target.
#                                e.g. 'aws s3 cp "$ARCHIVE" s3://my-bucket/ && aws s3 cp "$SIG" s3://my-bucket/'
#   VAKT_BACKUP_NOTIFY_WEBHOOK   POST a JSON {text} here on failure (your own endpoint)
#   VAKT_BACKUP_NOTIFY_CMD       generic failure hook; runs with $MESSAGE set
#   VAKT_INTERNAL_API_URL        URL of the vakt-api internal port  (default http://vakt-api:8081)
#                                Override when backup-cron runs outside the Docker network.
#
# Subcommands:
#   (default)  run the full cycle: backup → verify → off-site → prune
#   prune      run retention only (used by tests)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKUP_DIR="${VAKT_BACKUP_DIR:-/backups/vakt}"
RETENTION_DAYS="${VAKT_BACKUP_RETENTION_DAYS:-30}"

# Load config overrides from the Vakt API (if available). Env vars take
# precedence only when explicitly set; API values fill unset slots.
# ponytail: best-effort curl, never blocks the backup cycle
_load_api_config() {
	local api_url="${VAKT_INTERNAL_API_URL:-http://vakt-api:8081}"
	local secret="${VAKT_SECRET_KEY:-}"
	[ -z "$secret" ] && return 0
	local resp
	resp=$(curl -fsS -m 5 \
		-H "Authorization: Bearer ${secret}" \
		"${api_url}/api/v1/internal/backup-config" 2>/dev/null) || return 0

	_api_val() { printf '%s' "$resp" | grep -o "\"$1\":\"[^\"]*\"" | cut -d'"' -f4; }
	_api_int() { printf '%s' "$resp" | grep -o "\"$1\":[0-9]*" | cut -d':' -f2; }

	local v
	v=$(_api_val "schedule");        [ -n "$v" ] && export VAKT_BACKUP_SCHEDULE="${VAKT_BACKUP_SCHEDULE:-$v}"
	v=$(_api_int "retention_days");  [ -n "$v" ] && [ "$v" -gt 0 ] && RETENTION_DAYS="${v}"
	v=$(_api_val "passphrase");      [ -n "$v" ] && export VAKT_BACKUP_PASSPHRASE="${VAKT_BACKUP_PASSPHRASE:-$v}"
	v=$(_api_val "notify_webhook");  [ -n "$v" ] && export VAKT_BACKUP_NOTIFY_WEBHOOK="${VAKT_BACKUP_NOTIFY_WEBHOOK:-$v}"
	v=$(_api_val "offsite_cmd");     [ -n "$v" ] && export VAKT_BACKUP_OFFSITE_CMD="${VAKT_BACKUP_OFFSITE_CMD:-$v}"
	v=$(_api_val "notify_cmd");      [ -n "$v" ] && export VAKT_BACKUP_NOTIFY_CMD="${VAKT_BACKUP_NOTIFY_CMD:-$v}"
	# Guided backup destination (migration 232)
	v=$(_api_val "backup_dest_type");         [ -n "$v" ] && export BACKUP_DEST_TYPE="${BACKUP_DEST_TYPE:-$v}"
	v=$(_api_val "backup_dest_url");          [ -n "$v" ] && export BACKUP_DEST_URL="${BACKUP_DEST_URL:-$v}"
	v=$(_api_val "backup_dest_user");         [ -n "$v" ] && export BACKUP_DEST_USER="${BACKUP_DEST_USER:-$v}"
	v=$(_api_val "backup_dest_pass");         [ -n "$v" ] && export BACKUP_DEST_PASS="${BACKUP_DEST_PASS:-$v}"
	v=$(_api_val "backup_dest_remote_path");  [ -n "$v" ] && export BACKUP_DEST_REMOTE_PATH="${BACKUP_DEST_REMOTE_PATH:-$v}"
	v=$(_api_val "backup_dest_endpoint");     [ -n "$v" ] && export BACKUP_DEST_ENDPOINT="${BACKUP_DEST_ENDPOINT:-$v}"
	v=$(_api_val "backup_dest_bucket");       [ -n "$v" ] && export BACKUP_DEST_BUCKET="${BACKUP_DEST_BUCKET:-$v}"
	v=$(_api_val "backup_dest_prefix");       [ -n "$v" ] && export BACKUP_DEST_PREFIX="${BACKUP_DEST_PREFIX:-$v}"
	v=$(_api_val "backup_dest_access_key");   [ -n "$v" ] && export BACKUP_DEST_ACCESS_KEY="${BACKUP_DEST_ACCESS_KEY:-$v}"
	v=$(_api_val "backup_dest_secret_key");   [ -n "$v" ] && export BACKUP_DEST_SECRET_KEY="${BACKUP_DEST_SECRET_KEY:-$v}"
	v=$(_api_val "backup_dest_host");         [ -n "$v" ] && export BACKUP_DEST_HOST="${BACKUP_DEST_HOST:-$v}"
	v=$(_api_int "backup_dest_port");         [ -n "$v" ] && [ "$v" -gt 0 ] && export BACKUP_DEST_PORT="${BACKUP_DEST_PORT:-$v}"
	v=$(_api_val "backup_dest_cmd");          [ -n "$v" ] && export BACKUP_DEST_CMD="${BACKUP_DEST_CMD:-$v}"
	echo "→ [backup-cron] config loaded from API"
}
_load_api_config

# _run_offsite: push $1 (archive path) to the configured destination.
# Guided types (nextcloud/s3/sftp) call curl/rclone directly — no eval.
# "custom" and legacy VAKT_BACKUP_OFFSITE_CMD use bash -c (admin choice).
_run_offsite() {
	local archive="$1"
	local dtype="${BACKUP_DEST_TYPE:-none}"

	case "$dtype" in
	nextcloud)
		local base="${BACKUP_DEST_URL%/}"
		local rpath="${BACKUP_DEST_REMOTE_PATH%/}/$(basename "$archive")"
		curl -fsS -m 300 --upload-file "$archive" \
			-u "${BACKUP_DEST_USER}:${BACKUP_DEST_PASS}" \
			"${base}${rpath}"
		;;
	s3)
		RCLONE_S3_PROVIDER="${RCLONE_S3_PROVIDER:-Other}" \
		RCLONE_S3_ACCESS_KEY_ID="$BACKUP_DEST_ACCESS_KEY" \
		RCLONE_S3_SECRET_ACCESS_KEY="$BACKUP_DEST_SECRET_KEY" \
		RCLONE_S3_ENDPOINT="$BACKUP_DEST_ENDPOINT" \
		rclone copy "$archive" ":s3:${BACKUP_DEST_BUCKET}/${BACKUP_DEST_PREFIX}"
		;;
	sftp)
		local obscured
		obscured=$(rclone obscure "${BACKUP_DEST_PASS}")
		RCLONE_SFTP_HOST="$BACKUP_DEST_HOST" \
		RCLONE_SFTP_PORT="${BACKUP_DEST_PORT:-22}" \
		RCLONE_SFTP_USER="$BACKUP_DEST_USER" \
		RCLONE_SFTP_PASS="$obscured" \
		rclone copy "$archive" ":sftp:${BACKUP_DEST_REMOTE_PATH}"
		;;
	custom)
		ARCHIVE="$archive" SIG="${archive}.sig" bash -c "${BACKUP_DEST_CMD}"
		;;
	none|*)
		# Legacy fallback
		if [ -n "${VAKT_BACKUP_OFFSITE_CMD:-}" ]; then
			ARCHIVE="$archive" SIG="${archive}.sig" bash -c "$VAKT_BACKUP_OFFSITE_CMD"
		fi
		;;
	esac
}

notify_failure() {
	local message="$1"
	echo "ERROR: $message" >&2
	if [ -n "${VAKT_BACKUP_NOTIFY_WEBHOOK:-}" ]; then
		# Best-effort; never let a failing notification mask the original failure.
		curl -fsS -m 10 -X POST -H 'Content-Type: application/json' \
			-d "{\"text\":\"Vakt backup failure: ${message}\"}" \
			"$VAKT_BACKUP_NOTIFY_WEBHOOK" >/dev/null 2>&1 || true
	fi
	if [ -n "${VAKT_BACKUP_NOTIFY_CMD:-}" ]; then
		MESSAGE="$message" bash -c "$VAKT_BACKUP_NOTIFY_CMD" || true
	fi
}

prune_old_backups() {
	[ -d "$BACKUP_DIR" ] || return 0
	# Delete archives (and their .sig) older than the retention window.
	find "$BACKUP_DIR" -maxdepth 1 -type f \
		\( -name 'vakt-backup-*.tar.gz' -o -name 'vakt-backup-*.tar.gz.sig' \) \
		-mtime +"$RETENTION_DAYS" -print -delete
}

run_cycle() {
	mkdir -p "$BACKUP_DIR"

	echo "→ [backup-cron] creating backup in $BACKUP_DIR"
	if ! bash "$SCRIPT_DIR/backup.sh" "$BACKUP_DIR"; then
		notify_failure "backup.sh failed"
		exit 1
	fi

	# Newest archive just created.
	local archive
	archive="$(find "$BACKUP_DIR" -maxdepth 1 -name 'vakt-backup-*.tar.gz' -type f -printf '%T@ %p\n' \
		| sort -nr | head -1 | cut -d' ' -f2-)"
	if [ -z "$archive" ]; then
		notify_failure "no archive found after backup"
		exit 1
	fi

	echo "→ [backup-cron] verifying $archive"
	if ! bash "$SCRIPT_DIR/backup-verify.sh" "$archive"; then
		notify_failure "backup-verify.sh failed for $archive"
		exit 1
	fi

	# Off-site push (opt-in; guided or legacy VAKT_BACKUP_OFFSITE_CMD).
	local has_guided="${BACKUP_DEST_TYPE:-none}"
	if [ "$has_guided" != "none" ] && [ -n "$has_guided" ] || [ -n "${VAKT_BACKUP_OFFSITE_CMD:-}" ]; then
		echo "→ [backup-cron] off-site push (type=${has_guided})"
		if ! _run_offsite "$archive"; then
			notify_failure "off-site push failed for $archive"
			# Off-site failure is reported but does not abort retention.
		fi
	fi

	echo "→ [backup-cron] pruning archives older than ${RETENTION_DAYS}d"
	prune_old_backups

	echo "✓ [backup-cron] cycle complete"
}

case "${1:-run}" in
prune) prune_old_backups ;;
run) run_cycle ;;
*)
	echo "Usage: $0 [run|prune]" >&2
	exit 1
	;;
esac
