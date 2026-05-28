.PHONY: all
all: build

build:
	CGO_ENABLED=0 GOOS=linux go build -o bin/loco ./pkgs/loco
	GOOS=windows GOARCH=amd64 go build -o bin/loco.exe ./pkgs/loco

	CGO_ENABLED=0 GOOS=linux go build -o bin/rb pkgs/rb/main.go
	GOOS=windows GOARCH=amd64 go build -o bin/rb.exe pkgs/rb/main.go

# --- BigFred: backend (loco-server) ---------------------------------------
# Built from pkgs/server. CGO_ENABLED=0 keeps cross-compile working
# because the DB driver is the pure-Go modernc.org/sqlite (see
# pkgs/server/repo/db.go).
.PHONY: server server-build
server:
	go run ./pkgs/server -- --log-level=debug

server-build:
	CGO_ENABLED=0 GOOS=linux go build -o bin/loco-server ./pkgs/server

# --- BigFred: frontend (Vite + React + MUI) -------------------------------
# `web-dev` starts Vite on :5173 and proxies /api/v1 to the Go backend
# on :8080 (see web/vite.config.ts). Run `make server` in another
# terminal for the full loop.
.PHONY: web-install web-dev web-build
web-install:
	cd web && npm install

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

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
