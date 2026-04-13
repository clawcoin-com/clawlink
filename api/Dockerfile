FROM golang:1.25-alpine AS builder

WORKDIR /app

# No network access needed — all deps are in vendor/
COPY go.mod go.sum ./
COPY vendor/ vendor/
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -ldflags="-w -s" -o /clawlink-api ./cmd/server

# ── Runtime image ─────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /clawlink-api .

EXPOSE 8080

CMD ["./clawlink-api"]
