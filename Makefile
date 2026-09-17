.PHONY: all frontend build build-win explore ariba-probe ariba-probe-win clean

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

# Ariba session-replay spike (run on Caro's Windows laptop; needs ariba-probe.json)
ariba-probe:
	go run ./cmd/ariba-probe

# standalone Windows exe of the spike (no Go needed on the target laptop)
ariba-probe-win:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/ariba-probe.exe ./cmd/ariba-probe
	@echo "wrote dist/ariba-probe.exe"

clean:
	rm -rf dist frontend/dist cmd/carohelper/dist
