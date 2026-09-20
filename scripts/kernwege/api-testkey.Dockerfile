# Kernwege: das API-Image des Kunden-Stacks, nur das /api-Binary mit einem
# TEST-Lizenzschluessel gebaut (Kernweg 1, Kauf & Lizenz).
#
# Warum: internal/license/license.go vertraut genau einem fest einkompilierten
# oeffentlichen Schluessel. Der passende private Schluessel liegt beim Verkaufsdienst
# und ist fuer Tests tabu. kernwege.sh erzeugt deshalb bei jedem Lauf ein frisches
# ECDSA-P-256-Paar, und dieses Image setzt den oeffentlichen Teil per
# `-ldflags -X` in publicKeyPEM — dieselbe Variable, die die Go-Tests austauschen
# (license_test.go). Kein Produktcode wird geaendert, kein Schluessel committet.
#
# Alles andere (Runtime-Image, Migrationen, worker, migrate, healthcheck) ist das
# unveraenderte Image aus backend/Dockerfile: BASE ist dessen frisch gebautes Tag.
ARG BASE
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETARCH
ARG APP_VERSION=dev
ARG KW_PUBKEY
RUN test -n "$KW_PUBKEY" && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
      -ldflags="-s -w -X main.version=${APP_VERSION} -X 'github.com/matharnica/vakt/internal/license.publicKeyPEM=${KW_PUBKEY}'" \
      -o /out/api ./cmd/api

FROM ${BASE}
COPY --from=builder /out/api /api
