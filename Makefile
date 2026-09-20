.PHONY: all build build-web test run clean

all: build

build-web:
	cd web && npm run build

build: build-web
	go build -o bin/broffice ./cmd/broffice

test:
	go test -v ./...

run: build
	./bin/broffice

clean:
	rm -rf bin/ cmd/broffice/dist web/dist
