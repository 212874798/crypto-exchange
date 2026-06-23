build:
	go build -o bin/exchange ./cmd/server

run: build
	./bin/exchange

test:
	go test -v ./...

	