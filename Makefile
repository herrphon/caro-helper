.PHONY: all frontend build build-win explore clean

BIN := carohelper

all: build-win

frontend:
	cd frontend && npm install && npm run build

# native build for local testing
build: frontend
	go build -o dist/$(BIN) ./cmd/carohelper

# the artefact that goes on the USB stick
build-win: frontend
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/$(BIN).exe ./cmd/carohelper

# dumps the Smartsheet structure (needs SMARTSHEET_TOKEN in .env)
explore:
	go run ./cmd/explore > docs/smartsheet-layout.md
	@echo "wrote docs/smartsheet-layout.md"

clean:
	rm -rf dist frontend/dist cmd/carohelper/dist
