BINARY=bin/wings

.PHONY: build run test lint docker clean

build:
	mkdir -p bin
	go build -o $(BINARY) ./cmd/wings

run: build
	./$(BINARY)

test:
	go test ./...

lint:
	golangci-lint run

docker:
	docker build -t petrel-wings:latest .

clean:
	rm -rf bin
