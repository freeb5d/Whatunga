.PHONY: build test run-status run-watch run-serve fmt vet

build:
	go build -o bin/whatunga ./cmd/whatunga

test:
	go test ./... -v

fmt:
	gofmt -l -w .

vet:
	go vet ./...

run-status: build
	./bin/whatunga status -config config.yaml

run-watch: build
	./bin/whatunga watch -config config.yaml

run-serve: build
	./bin/whatunga serve -config config.yaml
