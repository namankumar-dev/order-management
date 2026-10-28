.PHONY: tidy build run test race cover lint vuln docker up down loadtest

tidy:
	go mod tidy
	cd product-service && go mod tidy

build:
	go build -o bin/order-api .

run:
	go run .

test:
	go test ./...

race:
	go test -race -count=1 ./...

cover:
	go test -race -coverprofile=coverage.out -coverpkg=./... ./...
	go tool cover -func=coverage.out | tail -n 1

lint:
	golangci-lint run ./...

vuln:
	govulncheck ./...

docker:
	docker build --build-arg VERSION=local -t order-api:local .

up:
	docker compose up --build -d

down:
	docker compose down -v

loadtest:
	go test -race -count=1 -run TestHundredConcurrentOrderRequests -v ./tests/
