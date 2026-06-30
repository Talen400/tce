.PHONY: all build test test-short bench install uninstall clean re

BINARY := tce

all: build

build:
	go build -o $(BINARY) .

test:
	go test ./... -v -count=1

test-short:
	go test ./... -short -count=1

bench:
	go test ./... -bench=. -benchtime=100ms -count=1

install: build
	install -d $(DESTDIR)/usr/local/bin
	install -m 755 $(BINARY) $(DESTDIR)/usr/local/bin/$(BINARY)

uninstall:
	rm -f /usr/local/bin/$(BINARY)

clean:
	rm -f $(BINARY)
	go clean

re: clean build
