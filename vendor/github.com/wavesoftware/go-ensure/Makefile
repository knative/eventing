PROJECT_DIR   = $(shell readlink -f .)
BUILD_DIR     = "$(PROJECT_DIR)/build"

GO           ?= go
RICHGO       ?= rich$(GO)

.PHONY: default
default: binaries

.PHONY: builddeps
builddeps:
	@GO111MODULE=off $(GO) get github.com/kyoh86/richgo
	@GO111MODULE=off $(GO) get github.com/mgechev/revive

.PHONY: builddir
builddir:
	@mkdir -p build

.PHONY: clean
clean: builddeps
	@echo "🛁 Cleaning"
	@rm -frv $(BUILD_DIR)

.PHONY: check
check: builddeps
	@echo "🛂 Checking"
	revive -config revive.toml -formatter stylish ./...

.PHONY: test
test: builddir check
	@echo "✔️ Testing"
	$(RICHGO) test -v -covermode=count -coverprofile=build/coverage.out ./...
