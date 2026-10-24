# Build stage
FROM golang:1.26.10-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# The release passes the tag so INFO and --version report it; a local build
# reports "dev".
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X github.com/cachestorm/cachestorm/internal/version.Version=${VERSION}" \
    -o cachestorm ./cmd/cachestorm

# Final stage
FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -g 1000 cachestorm && \
    adduser -u 1000 -G cachestorm -s /bin/sh -D cachestorm

WORKDIR /app

COPY --from=builder /app/cachestorm /usr/local/bin/cachestorm
COPY config/cachestorm.example.yaml /etc/cachestorm/cachestorm.yaml

RUN mkdir -p /data && chown -R cachestorm:cachestorm /app /data /etc/cachestorm

USER cachestorm

EXPOSE 6380 8080 7946 9090

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

ENTRYPOINT ["cachestorm"]
CMD ["--config", "/etc/cachestorm/cachestorm.yaml"]
