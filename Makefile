.PHONY: all
all: build

build:
	CGO_ENABLED=0 GOOS=linux go build -o bin/loco ./pkgs/loco
	GOOS=windows GOARCH=amd64 go build -o bin/loco.exe ./pkgs/loco

	CGO_ENABLED=0 GOOS=linux go build -o bin/rb pkgs/rb/main.go
	GOOS=windows GOARCH=amd64 go build -o bin/rb.exe pkgs/rb/main.go 

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
