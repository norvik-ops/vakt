#!/usr/bin/env bash
# Kernwege — der automatische Pruefer der Ziellinie (PROCESS.md P7c, docs/launch-gate.md).
#
# Startet eine FRISCHE Vakt-Instanz aus dem aktuellen Arbeitsbaum (echte Images aus
# backend/Dockerfile und frontend/Dockerfile, echtes Postgres, echter Frontend-Build,
# Start ueber das Kunden-docker-compose.yml) und faehrt die Playwright-Tests in
# frontend/kernwege/ dagegen. Kein Demo-Seed: der erste Admin entsteht ueber /setup,
# wie beim Kunden.
#
# Aufruf:  make kernwege            (alles: bauen, starten, testen, abraeumen)
#          scripts/kernwege/kernwege.sh [playwright-args…]
#
# Umgebung (alle optional):
#   KW_PROJECT      Compose-Projektname           (vakt-kw)
#   KW_HTTP_PORT    Port auf 127.0.0.1 fuer caddy (18480)
#   KW_MAIL_PORT    Port auf 127.0.0.1 fuer Mailpit (18425)
#   KW_KEEP=1       Stack nach dem Lauf NICHT abraeumen (Fehlersuche). Abraeumen dann mit
#                   `scripts/kernwege/kernwege.sh down`.
#   KW_SKIP_BUILD=1 vorhandene Images wiederverwenden (nur lokal sinnvoll)
#   KW_UPDATE=0     die Update-Phase (Kernweg 2b) auslassen — wird dann als
#                   uebersprungen gezaehlt, nicht als gruen.
#   KW_FROM_TAG     Ausgangsversion der Update-Phase (Default: letzter v*-Tag vor HEAD)
#
# Exit 0 nur, wenn jeder Test so ausging wie erwartet (gruen, oder rot mit test.fail()
# fuer einen bekannten Befund). Die Zusammenfassung zaehlt gruen / erwartet-rot /
# uebersprungen getrennt (frontend/kernwege/reporter.ts).
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
# Go liegt lokal oft ausserhalb des PATH (wie im pre-push-Hook).
for p in "$HOME/.local/go/bin" /usr/local/go/bin; do
  [[ -d $p ]] && PATH="$p:$PATH"
done
export PATH
PROJECT=${KW_PROJECT:-vakt-kw}
export KW_HTTP_PORT=${KW_HTTP_PORT:-18480}
export KW_MAIL_PORT=${KW_MAIL_PORT:-18425}
STATE_DIR=${KW_STATE_DIR:-${TMPDIR:-/tmp}/vakt-kernwege-$PROJECT}
export KW_DIR=$STATE_DIR/run
export KW_ENV_FILE=$KW_DIR/.env
export KW_API_IMAGE=vakt-kw-api:testkey
RESULTS=$ROOT/frontend/kernwege-results
KW_TAG=kernwege

log() { printf '\n\033[1m[kernwege] %s\033[0m\n' "$*"; }
die() { printf '[kernwege] FEHLER: %s\n' "$*" >&2; exit 1; }

compose() {
  docker compose -p "$PROJECT" \
    -f "$ROOT/docker-compose.yml" -f "$ROOT/scripts/kernwege/compose.kernwege.yml" \
    --env-file "$KW_ENV_FILE" "$@"
}

# set_env KEY VALUE — ersetzt eine vorhandene Zeile in der .env oder haengt sie an.
set_env() {
  local k=$1 v=$2
  if grep -q "^$k=" "$KW_ENV_FILE"; then
    # Werte hier sind hex/URLs ohne '|' — '|' als Trenner ist sicher.
    sed -i "s|^$k=.*|$k=$v|" "$KW_ENV_FILE"
  else
    printf '%s=%s\n' "$k" "$v" >>"$KW_ENV_FILE"
  fi
}

down() {
  if [[ -f $KW_ENV_FILE ]]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
    # Nebeninstanzen aus Kernweg 4 (Restore) und 2 (Update) — die Tests raeumen selbst
    # ab; das hier greift nur, wenn ein Lauf mittendrin abgebrochen wurde.
    local extra
    for extra in "$PROJECT-restore" "$PROJECT-update"; do
      docker compose -p "$extra" -f "$ROOT/docker-compose.yml" -f "$ROOT/scripts/kernwege/compose.kernwege.yml" \
        --env-file "$KW_ENV_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
    done
  fi
  rm -rf "$STATE_DIR"
}

# ── 1. Installation nach der oeffentlichen Anleitung (README „Quick Start") ─────
# Die Befehle werden aus dem README GELESEN, nicht abgeschrieben: aendert jemand die
# Anleitung, laeuft der Pruefer die neue Fassung. Ausgefuehrt werden die Zeilen, die
# die .env erzeugen (cp, sed). `git clone`/`cd` entfallen (wir SIND der Klon),
# `docker compose up -d` ersetzt Schritt 3 unten — mit dem Overlay und lokal gebauten
# Images statt der veroeffentlichten.
install_env() {
  # Frisch heisst frisch: ein Stack aus einem vorigen Lauf (KW_KEEP=1) samt Volumes weg.
  down
  mkdir -p "$KW_DIR/licstub" "$RESULTS"
  cp "$ROOT/.env.example" "$KW_DIR/.env.example"
  local block
  block=$(awk '/^### Local \(development \/ testing\)/{f=1} f&&/^```bash/{b=1;next} b&&/^```/{exit} b' "$ROOT/README.md")
  [[ -n $block ]] || die "README.md: Quick-Start-Block (### Local …, \`\`\`bash) nicht gefunden — Anleitung geaendert? Harness anpassen."
  local lines
  lines=$(printf '%s\n' "$block" | grep -E '^(cp|sed) ' || true)
  local n
  n=$(printf '%s\n' "$lines" | grep -c . || true)
  [[ $n -ge 4 ]] || die "README.md: erwartet cp + 3× sed im Quick Start, gefunden $n Zeilen."
  (cd "$KW_DIR" && bash -euo pipefail -c "$lines")
  printf '%s\n' "$lines" >"$KW_DIR/readme-steps.txt"
  log "Anleitung aus README.md ausgefuehrt ($n Zeilen) → $KW_ENV_FILE"

  # Abweichungen fuer den Pruefer — jede mit Grund (siehe compose.kernwege.yml):
  set_env VAKT_TAG "$KW_TAG"                                 # lokal gebaute Images, nie :latest ueberschreiben
  set_env VAKT_AI_PROVIDER disabled                          # kein Ollama-Download
  set_env VAKT_SMTP_HOST mailpit                             # Mail-Senke statt echtem Versand
  set_env VAKT_SMTP_PORT 1025
  set_env VAKT_SMTP_FROM kernwege@vakt.test
  set_env VAKT_FRONTEND_URL "http://localhost:$KW_HTTP_PORT" # Links in Mails zeigen auf die Testinstanz
  set_env VAKT_LICENSE_REFRESH_URL http://licstub:8000       # Verlaengerung gegen den Stub
  set_env VAKT_EPSS_ENABLED false                            # keine Feeds aus dem Netz
  set_env VAKT_BSI_FEED_ENABLED false
  set_env VAKT_EOL_CHECK_ENABLED false
  set_env VAKT_UPDATE_CHECK false
  KW_RENEWAL_TOKEN="kw-$(openssl rand -hex 12)"
  export KW_RENEWAL_TOKEN
  set_env KW_RENEWAL_TOKEN "$KW_RENEWAL_TOKEN"
  set_env KW_HTTP_PORT "$KW_HTTP_PORT"
  set_env KW_MAIL_PORT "$KW_MAIL_PORT"
  set_env KW_DIR "$KW_DIR"
  set_env KW_ENV_FILE "$KW_ENV_FILE"
  set_env KW_API_IMAGE "$KW_API_IMAGE"
}

# ── 2. Test-Lizenzschluessel (Kernweg 1) ───────────────────────────────────────
# Frisches ECDSA-P-256-Paar pro Lauf. Den Pro-Schluessel praegt das offizielle
# Werkzeug internal/license/generator — derselbe Weg wie beim Verkauf, nur mit dem
# Test-Schluessel. Verlaengerungs-Schluessel (brauchen `rt` und ein iat in der
# Vergangenheit) praegt frontend/kernwege/lib/license.ts im Test selbst.
make_license_keys() {
  # Mit KW_SKIP_BUILD=1 bleibt das API-Image vom letzten Bau — dann muss auch das
  # Schluesselpaar dasselbe sein, sonst verwirft die Instanz jeden Test-Schluessel.
  local cache=${TMPDIR:-/tmp}/vakt-kernwege-keys-$PROJECT
  if [[ ${KW_SKIP_BUILD:-0} == 1 && -f $cache/license-priv.pem ]]; then
    cp "$cache/license-priv.pem" "$cache/license-pub.pem" "$KW_DIR/"
  else
    openssl ecparam -name prime256v1 -genkey -noout -out "$KW_DIR/license-priv.pem" 2>/dev/null
    openssl ec -in "$KW_DIR/license-priv.pem" -pubout -out "$KW_DIR/license-pub.pem" 2>/dev/null
    mkdir -p "$cache" && chmod 700 "$cache"
    cp "$KW_DIR/license-priv.pem" "$KW_DIR/license-pub.pem" "$cache/"
  fi
  local pro_features="eu_ai_act,cra,ai_advisor,audit_pdf,sso,api_access,vaktaware_advanced,vaktscan_advanced,vaktvault_advanced,vaktprivacy_advanced,bsi_grundschutz,granular_permissions,supplier_portal,nis2_reporting,saml_auth,agent_write_tools,scim_provisioning,siem_export,multi_framework"
  local expires
  expires=$(date -u -d '+400 days' +%Y-%m-%d)
  if [[ ! -d $ROOT/backend/internal/license/generator ]]; then
    # Der Public Mirror liefert den Generator nicht aus (build-public-mirror.sh).
    # Dann gibt es keinen Pro-Schluessel, und Kernweg 1 zaehlt als uebersprungen.
    log "internal/license/generator fehlt (Public Mirror?) — Kernweg 1 wird uebersprungen"
    return
  fi
  (cd "$ROOT/backend" && go run ./internal/license/generator \
      --key "$KW_DIR/license-priv.pem" --org "Kernwege GmbH" --tier pro \
      --features "$pro_features" --expires "$expires") >"$KW_DIR/license-pro.key"
  [[ -s $KW_DIR/license-pro.key ]] || die "Lizenz-Generator hat keinen Schluessel geliefert"
  log "Test-Lizenz erzeugt (Pro, bis $expires, internal/license/generator)"
}

# ── 3. Bauen und starten ────────────────────────────────────────────────────────
build_images() {
  if [[ ${KW_SKIP_BUILD:-0} == 1 ]] && docker image inspect "$KW_API_IMAGE" >/dev/null 2>&1; then
    log "KW_SKIP_BUILD=1 — vorhandene Images werden wiederverwendet"
    return
  fi
  log "Images aus dem Arbeitsbaum bauen (backend/Dockerfile, frontend/Dockerfile, Dockerfile.scanners)"
  compose build migrate vakt-scanners
  # Frontend ohne node_modules/Testartefakte im Build-Kontext: frontend/ hat keine
  # .dockerignore, `COPY . .` wuerde lokale node_modules (oder einen Symlink darauf)
  # ueber die im Image installierten legen. Der Kontext ist ein tar des Arbeitsbaums —
  # uncommittete Aenderungen sind also dabei (gewollt: der Pruefer prueft, was da ist).
  tar -C "$ROOT/frontend" --exclude=./node_modules --exclude=./kernwege-results \
      --exclude=./test-results --exclude=./playwright-report -cf - . \
    | docker build -q -f Dockerfile -t "ghcr.io/norvik-ops/vakt-frontend:$KW_TAG" - >/dev/null
  log "API-Binary mit Test-Lizenzschluessel (scripts/kernwege/api-testkey.Dockerfile)"
  docker build -q -f "$ROOT/scripts/kernwege/api-testkey.Dockerfile" \
    --build-arg BASE="ghcr.io/norvik-ops/vakt-api:$KW_TAG" \
    --build-arg KW_PUBKEY="$(cat "$KW_DIR/license-pub.pem")" \
    --build-arg APP_VERSION="kernwege" \
    -t "$KW_API_IMAGE" "$ROOT/backend" >/dev/null
}

start_stack() {
  log "Stack starten (Projekt $PROJECT, http://localhost:$KW_HTTP_PORT)"
  compose up -d --no-build caddy worker mailpit licstub
  local i
  for i in $(seq 1 90); do
    if curl -fsS "http://localhost:$KW_HTTP_PORT/health" >/dev/null 2>&1 \
       && curl -fsS "http://localhost:$KW_HTTP_PORT/login" >/dev/null 2>&1; then
      log "Instanz bereit nach $((i * 2)) s"
      return
    fi
    sleep 2
  done
  compose ps
  compose logs --tail 80 api migrate caddy || true
  die "Instanz wurde nicht bereit (120 s)"
}

# Ausgangsversion fuer das Update (Kernweg 2b): letzter v*-Tag vor HEAD. Leer =
# Update-Test wird als uebersprungen gezaehlt (nie still gruen).
from_tag() {
  [[ ${KW_UPDATE:-1} == 0 ]] && return 0
  if [[ -n ${KW_FROM_TAG:-} ]]; then printf '%s' "$KW_FROM_TAG"; return 0; fi
  git -C "$ROOT" describe --tags --abbrev=0 --match 'v*' HEAD 2>/dev/null || true
}

run_playwright() {
  local rc=0
  local from
  from=$(from_tag)
  (
    cd "$ROOT/frontend"
    KW_PROJECT="$PROJECT" KW_FROM_TAG="$from" \
    KW_BASE_URL="http://localhost:$KW_HTTP_PORT" \
    KW_MAIL_URL="http://localhost:$KW_MAIL_PORT" \
    KW_COMPOSE="docker compose -p $PROJECT -f $ROOT/docker-compose.yml -f $ROOT/scripts/kernwege/compose.kernwege.yml --env-file $KW_ENV_FILE" \
      npx playwright test -c playwright.kernwege.config.ts "$@"
  ) || rc=$?
  return $rc
}

collect_logs() {
  compose logs --no-color api worker migrate caddy licstub >"$RESULTS/stack.log" 2>&1 || true
}

# Typpruefung der Tests VOR dem teuren Bau: Playwright transpiliert nur, ein
# Tippfehler faellt sonst erst nach Minuten mitten im Lauf auf. ESLint ignoriert
# kernwege/ (wie e2e/), deshalb hier strikt.
typecheck() {
  (cd "$ROOT/frontend" && npx tsc --noEmit --module esnext --target es2022 --moduleResolution bundler \
    --strict --skipLibCheck --noUnusedLocals --types node --allowImportingTsExtensions \
    kernwege/*.ts kernwege/lib/*.ts playwright.kernwege.config.ts) || die "Typfehler in frontend/kernwege/"
}

main() {
  local t0=$SECONDS
  typecheck
  install_env
  make_license_keys
  build_images
  start_stack
  local rc=0
  run_playwright "$@" || rc=$?
  collect_logs
  log "Laufzeit gesamt: $((SECONDS - t0)) s — Protokoll: frontend/kernwege-results/"
  return $rc
}

case "${1:-}" in
  down) down; exit 0 ;;
  up)   shift; install_env; make_license_keys; build_images; start_stack
        log "Stack laeuft. Tests: KW_STATE_DIR=$STATE_DIR scripts/kernwege/kernwege.sh test — abraeumen: … down"; exit 0 ;;
  test) shift; [[ -f $KW_ENV_FILE ]] || die "kein laufender Stack — erst 'up'"
        KW_RENEWAL_TOKEN=$(sed -n 's/^KW_RENEWAL_TOKEN=//p' "$KW_ENV_FILE"); export KW_RENEWAL_TOKEN
        run_playwright "$@"; exit $? ;;
esac

if [[ ${KW_KEEP:-0} != 1 ]]; then
  trap down EXIT
fi
main "$@"
