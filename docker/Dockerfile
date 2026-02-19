FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o cachestorm ./cmd/cachestorm

FROM alpine:3.19

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/cachestorm /usr/local/bin/cachestorm
COPY config/cachestorm.example.yaml /etc/cachestorm/cachestorm.yaml

EXPOSE 6380 7946 9090

ENTRYPOINT ["cachestorm"]
CMD ["--config", "/etc/cachestorm/cachestorm.yaml"]
