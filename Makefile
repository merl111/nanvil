.PHONY: build nanvil ncast test vet fmt deps clean

GO_VERSION ?= 1.25
NANVIL_BIN = ./bin/nanvil$(shell go env GOEXE)
NCAST_BIN = ./bin/ncast$(shell go env GOEXE)

build: nanvil ncast

nanvil:
	@echo "=> Building nanvil"
	@CGO_ENABLED=0 go build -trimpath -o $(NANVIL_BIN) ./cmd/nanvil/

ncast:
	@echo "=> Building ncast"
	@CGO_ENABLED=0 go build -trimpath -o $(NCAST_BIN) ./cmd/ncast/

deps:
	@CGO_ENABLED=0 go mod download
	@CGO_ENABLED=0 go mod tidy -v

test:
	@go test ./pkg/nanvil/... ./integration/... -race -count=1 -timeout 120s

vet:
	@go vet ./cmd/... ./pkg/nanvil/... ./pkg/ncast/... ./integration/...

fmt:
	@gofmt -l -w -s $$(find ./cmd ./pkg/nanvil ./pkg/ncast ./integration -type f -name '*.go')

clean:
	@rm -rf bin/
