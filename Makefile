.PHONY: all build loco-build rb-build
all: build

# Build metadata injected into pkgs/bigfred/server/version via -ldflags.
# Release tag + tagCommit are added post-build (ELF section) at retag time.
VERSION_PKG := github.com/keskad/loco/pkgs/bigfred/server/version
VERSION_LDFLAGS := -X $(VERSION_PKG).buildCommit=$(shell git rev-parse --short HEAD 2>/dev/null) \
	-X $(VERSION_PKG).buildTime=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

build: loco-build rb-build server-build loadtest-build remote-icmp-build

loco-build:
	CGO_ENABLED=0 GOOS=linux go build -o bin/loco ./pkgs/loco
	GOOS=windows GOARCH=amd64 go build -o bin/loco.exe ./pkgs/loco

rb-build:
	CGO_ENABLED=0 GOOS=linux go build -o bin/rb pkgs/rb/main.go
	GOOS=windows GOARCH=amd64 go build -o bin/rb.exe pkgs/rb/main.go

# --- BigFred: backend (loco-server) ---------------------------------------
# Built from pkgs/bigfred/server. CGO_ENABLED=0 keeps cross-compile working
# because the DB driver is the pure-Go modernc.org/sqlite (see
# pkgs/bigfred/server/repo/db.go).
#
# Persistent data (config, logs, supervisord) lives under BIGFRED_DATA_DIR
# (absolute path; default /data on hub images). Android sets this to the app
# data directory.
.PHONY: server server-build
server:
	go run -ldflags="$(VERSION_LDFLAGS)" ./pkgs/bigfred/server --log-level=debug --http 0.0.0.0:8080

.PHONY: server-telemetry server-build
server-telemetry:
	go run -ldflags="$(VERSION_LDFLAGS)" ./pkgs/bigfred/server --log-level=debug --http 0.0.0.0:8080 --enable-telemetry

server-build:
	CGO_ENABLED=0 GOOS=linux go build -ldflags="$(VERSION_LDFLAGS)" -o bin/loco-server ./pkgs/bigfred/server

.PHONY: loadtest-build remote-icmp-build
loadtest-build:
	CGO_ENABLED=0 GOOS=linux go build -ldflags="$(VERSION_LDFLAGS)" -o bin/loco-server-load-test ./pkgs/bigfred/loadtest

remote-icmp-build:
	CGO_ENABLED=0 GOOS=linux go build -ldflags="$(VERSION_LDFLAGS)" -o bin/bigfred-remote-icmp ./pkgs/bigfred/remote-icmp

# `build-prod` produces the single production binary: it builds the SPA
# (web/dist) and embeds it into loco-server via go:embed (-tags prod), so
# one binary serves both the API and the frontend at "/". `web-build` runs
# first because the go:embed needs web/dist to exist.
.PHONY: build-prod
build-prod: web-build
	CGO_ENABLED=0 go build -tags prod -ldflags="-s -w $(VERSION_LDFLAGS)" -o bin/loco-server ./pkgs/bigfred/server

# Production loco-server for Android arm64 (SPA embedded). Published to GHCR;
# bigfred-android-client pulls ghcr.io/dcc-bigfred/loco-server-android-arm64:main.
.PHONY: android
android: web-build-android
	CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build -tags prod -ldflags="-s -w $(VERSION_LDFLAGS)" \
		-o bin/loco-server-android-arm64 ./pkgs/bigfred/server
	@ls -lh bin/loco-server-android-arm64

# `run-prod` builds the embedded production binary and runs it with
# production defaults: info-level logging, no debug. Override the bind
# address with HTTP_ADDR, e.g. `make run-prod HTTP_ADDR=0.0.0.0:9090`. Set
# BIGFRED_JWT_SECRET in the environment so sessions survive restarts.
HTTP_ADDR ?= 0.0.0.0:8080

.PHONY: run-prod
run-prod: build-prod
	./bin/loco-server --http "$(HTTP_ADDR)" --log-level=info --enable-telemetry

# --- BigFred: frontend (Vite + React + MUI) -------------------------------
# `web-dev` starts Vite on :5173 and proxies /api/v1 to the Go backend
# on :8080 (see web/vite.config.ts). Run `make server` in another
# terminal for the full loop.
#
# Override the dev-server bind address (default localhost), e.g.:
#   make web-dev HOST=0.0.0.0
#   make web-dev HOST=192.168.0.86
HOST ?= localhost

# Rasterize src/icons/*.svg → src/icons/png/*.png (70x70, gitignored).
# Also runs automatically via the Vite plugin on `vite` / `vite build`.
# Needs: rsvg-convert (librsvg), e.g. pacman -S librsvg / apt install librsvg2-bin.
.PHONY: web-icons web-install web-dev web-build web-build-android
web-icons:
	cd web && python3 scripts/rasterize_function_icons.py

web-install:
	cd web && npm install

web-dev:
	cd web && HOST="$(HOST)" npm run dev

web-build:
	cd web && npm ci && npm run build

# SPA with phone capabilities (orange chrome, no remotes menu, no loconet_serial).
web-build-android:
	cd web && npm ci && VITE_ANDROID=1 npm run build

web-check-offline:
	cd web && npm run check:offline

# --- Test / lint targets --------------------------------------------------
ensure-go-junit-report:
	@command -v go-junit-report || (cd /tmp && go install github.com/jstemmer/go-junit-report/v2@latest)

test: ensure-go-junit-report
	go env -w GOTOOLCHAIN=go1.25.0+auto
	export PATH=$$PATH:~/go/bin:$$GOROOT/bin:$$(pwd)/.bin; \
	go test -v ./... -covermode=count -coverprofile=coverage.out 2>&1 | go-junit-report -set-exit-code -out junit.xml -iocopy

.PHONY: test-minisign
test-minisign:
	@command -v minisign >/dev/null 2>&1 || { echo "error: minisign required"; exit 1; }
	GOPROXY=$${GOPROXY:-direct} go test -count=1 github.com/dcc-bigfred/common/internal/minisignsign@latest

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...
