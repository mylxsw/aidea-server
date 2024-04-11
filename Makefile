Version := $(shell date "+%Y%m%d%H%M")
GitCommit := $(shell git rev-parse HEAD)
DIR := $(shell pwd)
LDFLAGS := -s -w -X main.Version=$(Version) -X main.GitCommit=$(GitCommit) -X main.DEBUG=true

build: doc
	go build -race -ldflags "$(LDFLAGS)" -o build/debug/aidea-server main.go

build-release: doc
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o build/release/aidea-server-linux main.go
	GOOS=linux GOARCH=arm go build -ldflags "$(LDFLAGS)" -o build/release/aidea-server-linux-arm main.go
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o build/release/aidea-server-darwin main.go
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o build/release/aidea-server.exe main.go

build-linux: doc
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o build/release/aidea-server-linux main.go

# Update swagger documentation
# Please install swag before execution, installation method: `go install github.com/swaggo/swag/cmd/swag@latest`
# Reference: https://github.com/swaggo/gin-swagger
doc:
	swag init

orm:
	# https://github.com/mylxsw/eloquent
	eloquent gen --source 'pkg/repo/model/*.yaml'
	gofmt -s -w pkg/repo/model/*.go

.PHONY: build build-release orm build-linux doc
