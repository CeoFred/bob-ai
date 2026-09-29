.PHONY: all build build-web build-backend test run dev sysinfo clean service-install service-uninstall

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
	@echo "==> Running Bob in foreground dev mode..."
	go run cmd/bob/main.go

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
	rm -rf bin web/dist
