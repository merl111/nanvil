.PHONY: build nanvil ncast darwin windows test vet fmt deps clean sync-docs website

GO_VERSION ?= 1.25
# gnark-crypto ships 160+ byte filenames; some $HOME filesystems cap NAME_MAX at 143.
GOMODCACHE ?= /tmp/nanvil-gomod-$(shell id -u)
export GOMODCACHE
export GOTOOLCHAIN ?= local

GO_BUILD = CGO_ENABLED=0 go build -trimpath
NANVIL_BIN = ./bin/nanvil$(shell GOMODCACHE=$(GOMODCACHE) GOTOOLCHAIN=$(GOTOOLCHAIN) go env GOEXE)
NCAST_BIN = ./bin/ncast$(shell GOMODCACHE=$(GOMODCACHE) GOTOOLCHAIN=$(GOTOOLCHAIN) go env GOEXE)
EMBEDDED_DOCS = pkg/nanvil/explorer/embedded-docs

sync-docs:
	@echo "=> Syncing docs into explorer embed"
	@mkdir -p $(EMBEDDED_DOCS)
	@rsync -a --delete docs/ $(EMBEDDED_DOCS)/

website:
	@echo "=> Building GitHub Pages site"
	@go run ./cmd/buildsite/
	@touch website/dist/.nojekyll

build: sync-docs nanvil ncast

nanvil:
	@echo "=> Building nanvil"
	@$(GO_BUILD) -o $(NANVIL_BIN) ./cmd/nanvil/

ncast:
	@echo "=> Building ncast"
	@$(GO_BUILD) -o $(NCAST_BIN) ./cmd/ncast/

darwin-amd64:
	@echo "=> Building darwin/amd64"
	@GOOS=darwin GOARCH=amd64 $(GO_BUILD) -o ./bin/nanvil-darwin-amd64 ./cmd/nanvil/
	@GOOS=darwin GOARCH=amd64 $(GO_BUILD) -o ./bin/ncast-darwin-amd64 ./cmd/ncast/

darwin-arm64:
	@echo "=> Building darwin/arm64"
	@GOOS=darwin GOARCH=arm64 $(GO_BUILD) -o ./bin/nanvil-darwin-arm64 ./cmd/nanvil/
	@GOOS=darwin GOARCH=arm64 $(GO_BUILD) -o ./bin/ncast-darwin-arm64 ./cmd/ncast/

darwin: darwin-amd64 darwin-arm64

windows-amd64:
	@echo "=> Building windows/amd64"
	@GOOS=windows GOARCH=amd64 $(GO_BUILD) -o ./bin/nanvil-windows-amd64.exe ./cmd/nanvil/
	@GOOS=windows GOARCH=amd64 $(GO_BUILD) -o ./bin/ncast-windows-amd64.exe ./cmd/ncast/

windows: windows-amd64

deps:
	@CGO_ENABLED=0 go mod download
	@CGO_ENABLED=0 go mod tidy -v

test: sync-docs
	@go test ./pkg/nanvil/... ./integration/... -race -count=1 -timeout 120s

vet:
	@go vet ./cmd/... ./pkg/nanvil/... ./pkg/ncast/... ./integration/...

fmt:
	@gofmt -l -w -s $$(find ./cmd ./pkg/nanvil ./pkg/ncast ./integration -type f -name '*.go')

clean:
	@rm -rf bin/
