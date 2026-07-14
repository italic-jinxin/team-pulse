.PHONY: web server run test clean macos
web:
	cd web && npm install && npm run build
server: web
	go build -o build/teampulse-server ./cmd/teampulse
run: server
	./build/teampulse-server
test:
	go test ./...
	cd web && npm run build
macos: server
	cd desktop/macos && swift build -c release
clean:
	rm -rf build web/node_modules desktop/macos/.build
