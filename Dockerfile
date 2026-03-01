# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install dependencies first (layer cache)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o notulapro-backend ./main.go

# ── Run stage ─────────────────────────────────────────────────────────────────
FROM alpine:3.20

WORKDIR /app

# Install CA certs for HTTPS calls to Recall.ai
RUN apk --no-cache add ca-certificates

COPY --from=builder /app/notulapro-backend .

EXPOSE 8080

CMD ["./notulapro-backend"]
