FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG APP_VERSION=dev
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-s -w -X main.version=${APP_VERSION}" -o /api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /migrate ./cmd/migrate && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /healthcheck ./cmd/healthcheck

# Pre-create the evidence upload dir (default VAKT_UPLOAD_DIR=./data/uploads,
# which resolves to /data since the distroless image has CWD /). Owned by the
# nonroot UID so a freshly-mounted named volume inherits write permission —
# otherwise Docker creates the mountpoint root-owned and the app (UID 65532)
# cannot create subdirs (mkdir: permission denied).
RUN mkdir -p /data/uploads

FROM gcr.io/distroless/static-debian12

COPY --from=builder /api /api
COPY --from=builder /worker /worker
COPY --from=builder /migrate /migrate
COPY --from=builder /healthcheck /healthcheck
COPY --from=builder /app/db/migrations /app/db/migrations
COPY --from=builder /app/NOTICE /NOTICE
# Evidence upload dir, owned by the distroless nonroot UID/GID (65532) so a
# fresh named volume mounted at /data is writable by the app.
COPY --from=builder --chown=65532:65532 /data /data

# Run as the distroless nonroot user (UID/GID 65532) — never as root.
USER nonroot:nonroot

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
  CMD ["/healthcheck"]

ENTRYPOINT ["/api"]
