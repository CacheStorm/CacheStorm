# CacheStorm Standalone Dockerfile
FROM golang:1.26.9-alpine AS builder

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
# The binary is started with --config /etc/cachestorm/cachestorm.yaml and exits
# immediately if that file is absent, so it has to be baked in -- not left to a
# volume mount. Same COPY the root Dockerfile uses.
COPY config/cachestorm.example.yaml /etc/cachestorm/cachestorm.yaml
ENV CACHESTORM_CONFIG=/etc/cachestorm/cachestorm.yaml

EXPOSE 6379 8080

VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["./cachestorm"]
CMD ["--config", "/etc/cachestorm/cachestorm.yaml"]
