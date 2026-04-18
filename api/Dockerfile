FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy module files first (for better layer caching).
COPY go.mod go.sum ./

# If vendor/ is present, use it for offline/reproducible builds.
# Otherwise, download modules on the fly. Either mode works.
COPY . .
RUN if [ -d vendor ]; then \
        echo "[build] using vendored dependencies"; \
        CGO_ENABLED=0 GOOS=linux go build -mod=vendor -ldflags="-w -s" -o /clawlink-api ./cmd/server; \
    else \
        echo "[build] no vendor/ dir, downloading modules"; \
        CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /clawlink-api ./cmd/server; \
    fi

# ── Runtime image ─────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /clawlink-api .

EXPOSE 8080

CMD ["./clawlink-api"]
