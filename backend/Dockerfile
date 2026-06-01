FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG APP_VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.version=${APP_VERSION}" -o /api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /migrate ./cmd/migrate && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /healthcheck ./cmd/healthcheck

FROM gcr.io/distroless/static-debian12

COPY --from=builder /api /api
COPY --from=builder /worker /worker
COPY --from=builder /migrate /migrate
COPY --from=builder /healthcheck /healthcheck
COPY --from=builder /app/db/migrations /app/db/migrations

# Run as the distroless nonroot user (UID/GID 65532) — never as root.
USER nonroot:nonroot

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
  CMD ["/healthcheck"]

ENTRYPOINT ["/api"]
