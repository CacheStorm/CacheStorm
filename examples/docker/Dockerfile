# CacheStorm Standalone Dockerfile
FROM golang:1.26.9-alpine AS builder

WORKDIR /build
COPY . .

RUN go mod download
RUN go build -ldflags="-s -w" -o cachestorm ./cmd/cachestorm

# Runtime image
# Pinned, not :latest, so a rebuild months from now resolves the same base.
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

# Non-root runtime user, same uid/gid and shape as the root Dockerfile.
RUN addgroup -g 1000 cachestorm && \
    adduser -u 1000 -G cachestorm -s /bin/sh -D cachestorm

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

# Hand ownership to the runtime user AFTER the config COPY, which is what
# creates /etc/cachestorm. /data is a VOLUME mount point the process writes to
# once privileges are dropped.
RUN chown -R cachestorm:cachestorm /app /data /etc/cachestorm

# Drop privileges. Everything above ran as root; the service does not need it.
USER cachestorm

# 6380, not the Redis-default 6379: that is what the baked config binds.
EXPOSE 6380 8080

VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["./cachestorm"]
CMD ["--config", "/etc/cachestorm/cachestorm.yaml"]
