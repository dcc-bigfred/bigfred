.PHONY: all build loco-build
all: build

# Build metadata injected into pkgs/bigfred/server/version via -ldflags.
# Release tag + tagCommit are added post-build (ELF section) at retag time.
VERSION_PKG := github.com/dcc-bigfred/bigfred/pkgs/bigfred/server/version
VERSION_LDFLAGS := -X $(VERSION_PKG).buildCommit=$(shell git rev-parse --short HEAD 2>/dev/null) \
	-X $(VERSION_PKG).buildTime=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

build: loco-build server-build loadtest-build remote-icmp-build

loco-build:
	CGO_ENABLED=0 GOOS=linux go build -o bin/loco ./pkgs/loco

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
	CGO_ENABLED=0 GOOS=linux go build -ldflags="$(VERSION_LDFLAGS)" -o bin/bf ./pkgs/bigfred/bf

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
	@tmpdir=$$(mktemp -d) && cd "$$tmpdir" && \
		go mod init smoke-test && \
		GOPROXY=$${GOPROXY:-direct} go get -t github.com/dcc-bigfred/common/internal/minisignsign@latest && \
		go test -count=1 github.com/dcc-bigfred/common/internal/minisignsign

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

# --- Hub deploy (RO rootfs: binaries live on /data) -----------------------
# Hub runs Dropbear. Older images lack /usr/libexec/sftp-server; -O uses
# legacy scp. Harmless on images that ship openssh sftp-server.
#
#   make deploy-hub
#   make deploy-hub HUB=192.168.0.10
#
# Installs linux/arm64 prod binaries into /data/opt/bigfred/bin (preferred
# over the image copies in /opt) and restarts microinit services.
HUB ?= 192.168.0.1
HUB_USER ?= root
HUB_SSH ?= $(HUB_USER)@$(HUB)
SCP ?= scp
SCP_OPTS ?= -O
SSH ?= ssh
HUB_BIN_DIR ?= /data/opt/bigfred/bin

.PHONY: hub-arm64 deploy-hub
hub-arm64: web-build
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags prod -ldflags="-s -w $(VERSION_LDFLAGS)" \
		-o bin/loco-server-linux-arm64 ./pkgs/bigfred/server
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(VERSION_LDFLAGS)" \
		-o bin/bigfred-remote-icmp-linux-arm64 ./pkgs/bigfred/remote-icmp
	@ls -lh bin/loco-server-linux-arm64 bin/bigfred-remote-icmp-linux-arm64

# Upload next to the target and rename: writing in place fails with ETXTBSY
# once the hub is running the /data copy, and rename(2) swaps the inode
# atomically. dcc-bus runs `bigfred dcc-bus …` from the same binary, so those
# daemons keep the old inode until they are restarted too.
deploy-hub: hub-arm64
	@test -f bin/loco-server-linux-arm64 || { echo "error: bin/loco-server-linux-arm64 missing" >&2; exit 1; }
	@test -f bin/bigfred-remote-icmp-linux-arm64 || { echo "error: bin/bigfred-remote-icmp-linux-arm64 missing" >&2; exit 1; }
	$(SSH) $(HUB_SSH) 'mkdir -p $(HUB_BIN_DIR)'
	$(SCP) $(SCP_OPTS) bin/loco-server-linux-arm64 $(HUB_SSH):$(HUB_BIN_DIR)/.bigfred.new
	$(SCP) $(SCP_OPTS) bin/bigfred-remote-icmp-linux-arm64 $(HUB_SSH):$(HUB_BIN_DIR)/.bigfred-remote-icmp.new
	$(SSH) $(HUB_SSH) 'set -e; \
		cd $(HUB_BIN_DIR); \
		chmod 755 .bigfred.new .bigfred-remote-icmp.new; \
		mv -f .bigfred.new bigfred; \
		mv -f .bigfred-remote-icmp.new bigfred-remote-icmp; \
		command -v setcap >/dev/null && setcap cap_net_raw+ep bigfred-remote-icmp || true; \
		rc=0; \
		microinit restart bigfred || rc=1; \
		microinit restart remote-icmp || rc=1; \
		for p in $$(microinit list 2>/dev/null | grep -o "^dcc-bus-[^ ]*" || true); do \
			echo "restarting $$p"; \
			microinit restart "$$p" || rc=1; \
		done; \
		exit $$rc'
