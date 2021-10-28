export GOLDFLAGS=-s -w -extldflags '-zrelro -znow'
export GOFLAGS=-trimpath
export CGO_ENABLED=0

.PHONY: all
all: dist

.PHONY: dist
dist: dist/amd64/tcp4to6 dist/arm64/tcp4to6

.PHONY: dist/amd64/tcp4to6
dist/amd64/tcp4to6:
	GOARCH=amd64 go build -ldflags "$(GOLDFLAGS)" -o $@ ./cmd/tcp4to6

.PHONY: dist/arm64/tcp4to6
dist/arm64/tcp4to6:
	GOARCH=arm64 go build -ldflags "$(GOLDFLAGS)" -o $@ ./cmd/tcp4to6

.PHONY: benchmark
benchmark:
	go test -bench=. ./...

.PHONY: test
test:
	CGO_ENABLED=1 go test -race ./...

.PHONY: lint
lint:
	golangci-lint run ./...
