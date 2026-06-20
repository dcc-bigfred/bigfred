.PHONY: all
all: build

build:
	CGO_ENABLED=0 GOOS=linux go build -o bin/loco ./pkgs/loco
	GOOS=windows GOARCH=amd64 go build -o bin/loco.exe ./pkgs/loco

	CGO_ENABLED=0 GOOS=linux go build -o bin/rb pkgs/rb/main.go
	GOOS=windows GOARCH=amd64 go build -o bin/rb.exe pkgs/rb/main.go

# --- BigFred: backend (loco-server) ---------------------------------------
# Built from pkgs/bigfred/server. CGO_ENABLED=0 keeps cross-compile working
# because the DB driver is the pure-Go modernc.org/sqlite (see
# pkgs/bigfred/server/repo/db.go).
.PHONY: server server-build
server:
	go run ./pkgs/bigfred/server --log-level=debug --http 0.0.0.0:8080

.PHONY: server-telemetry server-build
server-telemetry:
	go run ./pkgs/bigfred/server --log-level=debug --http 0.0.0.0:8080 --enable-telemetry

server-build:
	CGO_ENABLED=0 GOOS=linux go build -o bin/loco-server ./pkgs/bigfred/server

# `build-prod` produces the single production binary: it builds the SPA
# (web/dist) and embeds it into loco-server via go:embed (-tags prod), so
# one binary serves both the API and the frontend at "/". `web-build` runs
# first because the go:embed needs web/dist to exist.
.PHONY: build-prod
build-prod: web-build
	CGO_ENABLED=0 go build -tags prod -ldflags="-s -w" -o bin/loco-server ./pkgs/bigfred/server

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

.PHONY: web-install web-dev web-build
web-install:
	cd web && npm install

web-dev:
	cd web && HOST="$(HOST)" npm run dev

web-build:
	cd web && npm ci && npm run build

web-check-offline:
	cd web && npm run check:offline

# --- Test / lint targets --------------------------------------------------
ensure-go-junit-report:
	@command -v go-junit-report || (cd /tmp && go install github.com/jstemmer/go-junit-report/v2@latest)

test: ensure-go-junit-report
	go env -w GOTOOLCHAIN=go1.25.0+auto
	export PATH=$$PATH:~/go/bin:$$GOROOT/bin:$$(pwd)/.bin; \
	go test -v ./... -covermode=count -coverprofile=coverage.out 2>&1 | go-junit-report -set-exit-code -out junit.xml -iocopy

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...
