.PHONY: web backend server run test check-repository clean macos

GO_BUILD_FLAGS :=
GO_TEST_FLAGS :=
ifeq ($(shell go env GOOS),darwin)
GO_BUILD_FLAGS += -ldflags='-linkmode=external'
GO_TEST_FLAGS += -ldflags='-linkmode=external'
endif

web:
	cd web && npm install && npm run build
backend:
	mkdir -p build
	go build $(GO_BUILD_FLAGS) -o build/teampulse-server ./cmd/teampulse
server: web backend
run: server
	./build/teampulse-server
test: check-repository web
	go test $(GO_TEST_FLAGS) ./...
check-repository:
	./scripts/check-no-local-data.sh
macos: server
	cd desktop/macos && swift build -c release
clean:
	rm -rf build web/node_modules desktop/macos/.build
