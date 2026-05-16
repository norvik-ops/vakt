FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12

COPY --from=builder /api /api
COPY --from=builder /worker /worker
COPY --from=builder /migrate /migrate
COPY --from=builder /app/db/migrations /app/db/migrations
# Copy busybox wget from the Alpine builder so the HEALTHCHECK has a shell tool.
# distroless/static contains no shell or utilities by default.
COPY --from=builder /bin/busybox /bin/busybox

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
  CMD ["/bin/busybox", "wget", "-qO-", "http://localhost:8080/health/ready"]

ENTRYPOINT ["/api"]
