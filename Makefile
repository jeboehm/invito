.PHONY: build test e2e e2e-visible

build:
	go build -o bin/invito ./cmd/invito

test:
	go test ./...

e2e:
	go test -tags e2e -v -timeout 120s ./e2e/

e2e-visible:
	CHROME_HEADLESS=0 go test -tags e2e -v -timeout 120s ./e2e/
