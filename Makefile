.PHONY: all build test bench run cli clean re

BINARY := tce

all: build

build:
	go build -o $(BINARY) .

run: build
	./$(BINARY)

cli: build
	./$(BINARY) --cli

test:
	go test ./... -v -count=1

bench:
	go test ./... -bench=. -benchtime=100ms -count=1

clean:
	rm -f $(BINARY)
	go clean

re: clean build
