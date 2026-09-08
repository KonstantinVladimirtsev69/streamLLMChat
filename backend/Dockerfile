# ==============================================================================
# Backend Multi-Stage Dockerfile (Dokploy Compatible)
# Build Context: ./backend
# ==============================================================================

# Stage 1: Build binary
FROM golang:1.27-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Compile optimized static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./main.go

# Stage 2: Minimal runtime image
FROM alpine:3.21 AS runner

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 10001 -S appuser && \
    adduser -u 10001 -S appuser -G appuser

COPY --from=builder /app/server /app/server

USER appuser

EXPOSE 8080
ENV PORT=8080

CMD ["/app/server"]
