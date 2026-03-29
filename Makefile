APP_NAME := voyah-free-update-fix
CMD_PATH := ./cmd/voyah-free-update-fix
BUILD_DIR := ./bin

GO ?= go

.PHONY: build build-mac build-win build-all test test-integration clean

build: build-all

build-mac:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 $(CMD_PATH)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)-darwin-arm64 $(CMD_PATH)

build-win:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe $(CMD_PATH)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)-windows-arm64.exe $(CMD_PATH)

build-all: build-mac build-win

test:
	$(GO) test ./...

test-integration:
	$(GO) test -tags=integration ./tests/...

clean:
	rm -rf $(BUILD_DIR)

lint:
	@golangci-lint run ./...
	
