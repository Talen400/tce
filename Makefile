.PHONY: all build test test-short bench install uninstall clean re venv serve run

BINARY := tce
VENV   := .venv
PYTHON := python3
MODEL_DIR := $(HOME)/.cache/tce/models

all: build

build:
	go build -o $(BINARY) .

venv: $(VENV)/bin/activate

$(VENV)/bin/activate: requirements.txt
	$(PYTHON) -m venv $(VENV)
	$(VENV)/bin/pip install -r requirements.txt
	@echo "\n  Venv pronta em $(VENV)/"

serve: venv
	$(VENV)/bin/$(PYTHON) serve.py

run: build
	./$(BINARY) --serve

test:
	go test ./... -v -count=1

test-short:
	go test ./... -short -count=1

test-python: venv
	$(VENV)/bin/python -m pytest tests/ -v

bench:
	go test ./... -bench=. -benchtime=100ms -count=1

install: build
	install -d $(DESTDIR)/usr/local/bin
	install -m 755 $(BINARY) $(DESTDIR)/usr/local/bin/$(BINARY)

uninstall:
	rm -f /usr/local/bin/$(BINARY)

clean:
	rm -f $(BINARY)
	rm -rf $(VENV)
	rm -rf $(MODEL_DIR)
	go clean

distclean: clean
	rm -rf $(MODEL_DIR)

re: clean build
