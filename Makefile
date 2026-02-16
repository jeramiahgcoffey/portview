.PHONY: build test lint run clean

build:
	go build -o bin/portview ./cmd/portview

test:
	go test ./...

lint:
	golangci-lint run

run:
	go run ./cmd/portview

clean:
	rm -rf bin/
