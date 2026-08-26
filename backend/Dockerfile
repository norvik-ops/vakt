FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG APP_VERSION=dev
ARG TARGETARCH

# VAKT_BILLING steuert, ob das Fehlen von cmd/billing ein Fehler ist.
#   auto     (Default) — vorhanden wird gebaut, fehlend wird gemeldet. Das ist
#              der Mirror-Fall und der Fall jedes Kundenbaus.
#   required          — fehlend bricht den Bau ab. Das ist der private
#              Release-Bau und der interne ISMS-Billing-Container.
#
# Warum es diesen Schalter gibt (2026-08-02, REV-W1B N3): der bewachte Bau
# weiter unten hat den Kundenbau repariert, aber auf der privaten Seite eine
# Bau-Zeit-Pruefung verloren. Verschwindet cmd/billing dort durch eine
# Umbenennung, baut das Dockerfile still durch (rc=0), und der Billing-Container
# startet aus einem Image ohne /billing. Ein `echo` im Build-Log ersetzt das
# nicht. Die Pruefung steht VOR dem teuren Go-Bau, damit der Abbruch in Sekunden
# kommt statt nach Minuten.
ARG VAKT_BILLING=auto
RUN if [ "$VAKT_BILLING" = "required" ] && [ ! -d ./cmd/billing ]; then \
      echo "[build] FEHLER: VAKT_BILLING=required, aber ./cmd/billing fehlt." >&2; \
      echo "[build] Dieses Image soll den Billing-Dienst tragen (entrypoint /billing)." >&2; \
      echo "[build] Wurde cmd/billing verschoben oder umbenannt? Oder baut hier" >&2; \
      echo "[build] versehentlich der Public Mirror? Dann VAKT_BILLING=auto setzen." >&2; \
      exit 1; \
    fi
# The four binaries every Vakt instance needs. They go to /out so the runtime
# stage can copy the whole directory in one COPY — see the billing note below.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-s -w -X main.version=${APP_VERSION}" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /out/worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /out/healthcheck ./cmd/healthcheck

# cmd/billing is Norvik's own sales and licensing service — the process that holds
# VAKT_LICENSE_PRIVATE_KEY. It is vendor infrastructure, not part of the product,
# and scripts/build-public-mirror.sh deliberately does not ship it (same reason
# internal/license/generator/ is excluded there).
#
# Until 2026-08-02 this Dockerfile built it unconditionally. In the public mirror
# the directory is absent, so `docker compose build api` died with
# `stat /app/cmd/billing: directory not found` (rc=1) — the customer could not
# compile the product he was told he could keep running independently. Measured
# against the built mirror, not against this repo: here the directory exists and
# every build was green.
#
# So the build follows the tree it was given. Present -> billing is built (the
# private repo, the release pipeline, the internal ISMS container that runs this
# same image with `entrypoint: ["/billing"]`). Absent -> it is not, and the line
# says so out loud rather than passing silently.
#
# Dass "absent" auf der privaten Seite ein Fehler ist, entscheidet VAKT_BILLING
# weiter oben — nicht diese Zeile. Zusaetzlich haelt scripts/check_billing_binary.py
# statisch fest: wer einen /billing-Container ausliefert, muss cmd/billing haben.
RUN if [ -d ./cmd/billing ]; then \
      echo "[build] cmd/billing present — building the vendor billing binary"; \
      CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /out/billing ./cmd/billing; \
    else \
      echo "[build] cmd/billing absent — vendor billing binary NOT built (public mirror build)"; \
    fi

# Pre-create the evidence upload dir (default VAKT_UPLOAD_DIR=./data/uploads,
# which resolves to /data since the distroless image has CWD /). Owned by the
# nonroot UID so a freshly-mounted named volume inherits write permission —
# otherwise Docker creates the mountpoint root-owned and the app (UID 65532)
# cannot create subdirs (mkdir: permission denied).
RUN mkdir -p /data/uploads

FROM gcr.io/distroless/static-debian12

# One COPY for the whole binary directory. This is what makes the billing binary
# optional: a per-binary `COPY --from=builder /billing /billing` fails when the
# builder did not produce it, a directory copy does not. In the private build /out
# holds five binaries (billing included — the internal ISMS billing container runs
# this image with `entrypoint: ["/billing"]`), in the public-mirror build four.
COPY --from=builder /out/ /
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
