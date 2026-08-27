# Stage 1: Build static Go binary
FROM golang:1.23-alpine AS builder

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum* ./
RUN go mod download || true

# Copy source
COPY . .

# Compile statically linked binary with stripped symbols
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.Version=v1.0.0" \
    -trimpath \
    -o /bin/rollout-sentinel ./cmd/sentinel

# Stage 2: Minimal non-root runtime image
FROM alpine:3.20 AS runner

WORKDIR /app

# Non-root user setup for security compliance
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D -s /bin/sh appuser && \
    apk --no-cache add ca-certificates tzdata

COPY --from=builder /bin/rollout-sentinel /usr/local/bin/rollout-sentinel

USER appuser

ENTRYPOINT ["rollout-sentinel"]
CMD ["--help"]
