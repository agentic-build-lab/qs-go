.PHONY: build test verify

build:
	go build -trimpath -o bin/qsgo ./cmd/qsgo

test:
	go test ./... -count=1

verify: test build
	./bin/qsgo version
	./bin/qsgo normalize 'a%5Bb%5D=c&list%5B%5D=1&list%5B%5D=2'
