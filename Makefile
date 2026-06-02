PORT ?= 9001

.PHONY: dev web-install web-build server build run clean

# Full local dev: install web deps once, build the SPA, run the Go server
# which serves the SPA from web/dist via embed.FS.
dev: web-install web-build run

web-install:
	cd web && npm install

web-build:
	cd web && npm run build

server:
	go build -o bin/server .

build: web-build server

run: web-build
	PORT=$(PORT) go run .

clean:
	rm -rf bin web/dist web/node_modules
