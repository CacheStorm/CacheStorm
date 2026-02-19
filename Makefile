.PHONY: build test bench lint docker docker-cluster clean

build:
	go build -o bin/cachestorm ./cmd/cachestorm

test:
	go test ./... -race -count=1

bench:
	go test ./... -bench=. -benchmem

lint:
	golangci-lint run

docker:
	docker build -t cachestorm:latest -f docker/Dockerfile .

docker-cluster:
	docker compose -f docker/docker-compose.yml up

clean:
	rm -rf bin/

install-deps:
	go mod download
	go mod tidy
