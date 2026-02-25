# CacheStorm Standalone Dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /build
COPY . .

RUN go mod download
RUN go build -ldflags="-s -w" -o cachestorm ./cmd/cachestorm

# Runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /build/cachestorm .

# Create data directory
RUN mkdir -p /data

# Default config
ENV CACHESTORM_CONFIG=/etc/cachestorm/cachestorm.yaml

EXPOSE 6379 8080

VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["./cachestorm"]
CMD ["--config", "/etc/cachestorm/cachestorm.yaml"]
