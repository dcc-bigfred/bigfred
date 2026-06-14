## 8. Makefile Additions

```makefile
web-install:
	cd web && npm ci

web-build:
	cd web && npm run build

web-dev:
	cd web && npm run dev

server:
	go run ./pkgs/bigfred/server

server-build:
	CGO_ENABLED=0 go build -o bin/loco-server ./pkgs/bigfred/server

scripts-executor:
	go run ./pkgs/scripts-executor

scripts-executor-build:
	CGO_ENABLED=0 go build -o bin/loco-scripts-executor ./pkgs/scripts-executor

# `server-build` and `scripts-executor-build` are produced from the
# same Go module, so they always share the protocol types defined in
# pkgs/bigfred/server/executor. CI must build BOTH in the same pipeline step
# to prevent a wire-protocol drift between the two binaries.
all-build: server-build scripts-executor-build
```
