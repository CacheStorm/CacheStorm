FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o cachestorm ./cmd/cachestorm

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata wget && \
    adduser -D -u 1000 cachestorm

COPY --from=builder /app/cachestorm /usr/local/bin/cachestorm
COPY config/cachestorm.example.yaml /etc/cachestorm/cachestorm.yaml

RUN mkdir -p /data && chown cachestorm:cachestorm /data

USER cachestorm
EXPOSE 6380 8080 7946 9090
VOLUME /data

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8080/api/health || exit 1

ENTRYPOINT ["cachestorm"]
CMD ["--config", "/etc/cachestorm/cachestorm.yaml"]
