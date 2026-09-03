# syntax=docker/dockerfile:1
#
# Labuda backend runtime image.
#
# Build context: backend/.
#
# The build targets ONLY the production runtime and the migration runner.
# `go build ./...` is intentionally NOT used: the repository contains a
# non-runtime dev tool (cmd/verify_partial_refund_semantics) with an unrelated
# compile failure.

# --- Builder stage -----------------------------------------------------------
FROM golang:1.25.5 AS builder

WORKDIR /app/backend

# Cache module downloads before copying the rest of the source.
COPY go.mod go.sum ./
RUN go mod download

# Copy the remainder of the backend source (respects backend/.dockerignore).
COPY . .

# Production HTTP server (entrypoint: cmd/core_server/main.go).
# Statically linked so the minimal runtime image needs no libc/go toolchain.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bin/labuda-backend ./cmd/core_server

# Migration runner (equivalent to `go run ./cmd/migrate`, compiled for the
# minimal runtime image which has no Go toolchain).
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bin/labuda-migrate ./cmd/migrate

# --- Runtime stage -----------------------------------------------------------
FROM alpine:3.20

# Minimal runtime: CA certificates for outbound TLS (Firebase, Midtrans, S3)
# plus a non-root user. No build toolchain, no source, no secrets.
RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app

WORKDIR /app/backend

# Non-root runtime user.
USER app

# Runtime binaries (statically linked, CGO disabled).
COPY --from=builder --chown=app:app /app/backend/bin/labuda-backend ./bin/labuda-backend
COPY --from=builder --chown=app:app /app/backend/bin/labuda-migrate ./bin/labuda-migrate

# Migration chain required by ./bin/labuda-migrate at startup.
COPY --from=builder --chown=app:app /app/backend/migrations ./migrations

# Startup wrapper: materializes the Firebase credential file, then runs
# migrations and starts the server. See docker-entrypoint.sh.
COPY --chown=app:app docker-entrypoint.sh ./docker-entrypoint.sh
RUN chmod +x ./docker-entrypoint.sh

# The application reads PORT (injected by the environment) and binds on all interfaces.
# No hard-coded port is set here.
ENV GIN_MODE=release

CMD ["./docker-entrypoint.sh"]