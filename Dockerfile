# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o cachestorm ./cmd/cachestorm

# Final stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -g 1000 cachestorm && \
    adduser -u 1000 -G cachestorm -s /bin/sh -D cachestorm

WORKDIR /app

COPY --from=builder /app/cachestorm /usr/local/bin/cachestorm
COPY config/cachestorm.example.yaml /etc/cachestorm/cachestorm.yaml

RUN mkdir -p /data && chown -R cachestorm:cachestorm /app /data /etc/cachestorm

USER cachestorm

EXPOSE 6379 8080 7946 9090

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["cachestorm"]
CMD ["--config", "/etc/cachestorm/cachestorm.yaml", "--port", "6379", "--http-port", "8080"]
