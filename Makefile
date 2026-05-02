.PHONY: build test e2e e2e-visible docs
include dev/local.env

build:
	go build -o bin/invito ./cmd/invito

test:
	go test ./...

e2e:
	go test -tags e2e -v -timeout 120s ./e2e/

e2e-visible:
	CHROME_HEADLESS=0 go test -tags e2e -v -timeout 120s ./e2e/

up:
	docker compose up -d mailpit nextcloud dex
	go run ./cmd/invito

down:
	docker compose down --remove-orphans

docs:
	docker compose --profile docs up docs
