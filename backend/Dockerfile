# Trivy — statically compiled binary from official slim image (scratch-based)
FROM aquasec/trivy:0.62.0@sha256:cb84170aa1fb6942c11b638969dce0845735c35f40d3bd48851c7f2d83a3c1ae AS trivy

# Nuclei — Go binary, statically compiled (CGO_ENABLED=0)
FROM projectdiscovery/nuclei:v3.4.4@sha256:87bb57b9be70c90a6be8516c3852ca73370139ec32fe4bf6281f067c4b06db4a AS nuclei

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
COPY --from=trivy /usr/local/bin/trivy /usr/local/bin/trivy
COPY --from=nuclei /usr/local/bin/nuclei /usr/local/bin/nuclei

# Run as the distroless nonroot user (UID/GID 65532) — never as root.
USER nonroot:nonroot

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
  CMD ["/healthcheck"]

ENTRYPOINT ["/api"]
