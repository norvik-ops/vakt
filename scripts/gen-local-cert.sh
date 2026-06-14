#!/usr/bin/env bash
# Generates a self-signed TLS certificate for local development.
# Output: nginx/certs/localhost.crt + localhost.key
# Usage:  ./scripts/gen-local-cert.sh

set -euo pipefail

CERT_DIR="$(cd "$(dirname "$0")/.." && pwd)/nginx/certs"
mkdir -p "$CERT_DIR"

if [[ -f "$CERT_DIR/localhost.crt" && -f "$CERT_DIR/localhost.key" ]]; then
	echo "Certificate already exists at $CERT_DIR — skipping."
	echo "Delete $CERT_DIR/localhost.{crt,key} to regenerate."
	exit 0
fi

# Try mkcert first (produces browser-trusted certs on dev machines)
if command -v mkcert &>/dev/null; then
	echo "mkcert found — generating locally-trusted certificate..."
	mkcert -install 2>/dev/null || true
	mkcert -cert-file "$CERT_DIR/localhost.crt" \
		-key-file "$CERT_DIR/localhost.key" \
		localhost 127.0.0.1 ::1
	echo "Done. Certificate trusted by your browser automatically."
else
	echo "mkcert not found — generating self-signed certificate via openssl."
	echo "(Browser will show a warning — accept it once, or install mkcert for trust.)"
	openssl req -x509 -nodes -days 825 \
		-newkey rsa:2048 \
		-keyout "$CERT_DIR/localhost.key" \
		-out "$CERT_DIR/localhost.crt" \
		-subj "/CN=localhost" \
		-addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
	echo "Done."
fi

echo ""
echo "Certificate: $CERT_DIR/localhost.crt"
echo "Key:         $CERT_DIR/localhost.key"
echo ""
echo "Start with HTTPS:"
echo "  docker compose -f docker-compose.yml -f docker-compose.tls.yml up -d"
