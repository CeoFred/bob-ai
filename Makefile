.PHONY: all build build-web build-backend test run dev dev-web dev-backend sysinfo clean service-install service-uninstall

all: build

build: build-web build-backend

build-web:
	@echo "==> Building React frontend (Vite)..."
	cd web && npm run build

build-backend:
	@echo "==> Compiling Bob Go binary..."
	mkdir -p bin
	go build -o bin/bob cmd/bob/main.go

test:
	@echo "==> Running test suite..."
	go test -v ./...

run: build
	@echo "==> Running Bob..."
	./bin/bob

dev:
	@echo "==> Starting Bob full-stack live-reload dev mode..."
	./scripts/dev.sh

dev-web:
	@echo "==> Starting Vite frontend dev server with HMR..."
	cd web && npm run dev

dev-backend:
	@echo "==> Starting Go backend with auto-reload..."
	air || go run cmd/bob/main.go

sysinfo:
	@echo "==> Inspecting host hardware and recommending LLMs..."
	go run cmd/bob/main.go -sysinfo

service-install: build
	@echo "==> Installing Bob macOS launchd service..."
	./scripts/install-service.sh install

service-uninstall:
	@echo "==> Uninstalling Bob macOS launchd service..."
	./scripts/install-service.sh uninstall

clean:
	@echo "==> Cleaning artifacts..."
	rm -rf bin tmp web/dist

