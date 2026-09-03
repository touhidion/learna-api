# syntax=docker/dockerfile:1

# ---- build ------------------------------------------------------------------
FROM golang:1.24-alpine AS build

WORKDIR /src

# Dependencies are copied first so a source-only change does not re-download
# the module cache.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

# CGO_ENABLED=0 produces a static binary that runs on scratch.
# -trimpath keeps build paths out of the binary; -s -w drops the symbol table.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/learna-api \
        ./cmd/server

# ---- runtime ----------------------------------------------------------------
FROM alpine:3.21 AS runtime

# ca-certificates: outbound TLS to Cloudinary.
# tzdata: correct timestamps for certificate issue dates.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 -h /app learna

WORKDIR /app

COPY --from=build /out/learna-api /app/learna-api

# Migrations are embedded in the binary; nothing else needs to ship.
USER learna

EXPOSE 8080

ENV APP_ENV=production \
    SERVER_PORT=8080

# The health endpoint reports database connectivity, so this probe covers more
# than "the process is alive".
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/live >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/learna-api"]
