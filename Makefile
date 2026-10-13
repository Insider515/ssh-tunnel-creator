BINARY  := sshtunnel
APPNAME := SSH Tunnel
APPID   := com.geekproxy.sshtunnel
PKG     := ./cmd/sshtunnel
DIST    := dist
VERSION := 1.0.0
GO_BIN  := $(shell go env GOBIN)
ifeq ($(GO_BIN),)
GO_BIN  := $(shell go env GOPATH)/bin
endif

.PHONY: all test vet run build icon tools mac mac-dmg windows linux clean

# all runs static analysis, tests, and a build for the current OS.
all: vet test build

# vet runs the Go static analyzer.
vet:
	go vet ./...

# test runs all tests with the race detector.
test:
	go test -race -timeout 120s ./...

# run launches the application from source.
run:
	go run $(PKG)

# build compiles the application for the current OS into dist.
build:
	mkdir -p $(DIST)
	go build -trimpath -o $(DIST)/$(BINARY) $(PKG)

# icon regenerates assets/icon.png.
icon:
	go run ./tools/genicon

# tools installs Fyne packagers: fyne for .app/.exe and fyne-cross for Linux in Docker.
tools:
	go install fyne.io/tools/cmd/fyne@latest
	go install github.com/fyne-io/fyne-cross@latest

# mac builds a universal binary (Apple Silicon + Intel) and an .app bundle.
mac:
	mkdir -p $(DIST)/mac
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -trimpath -o $(DIST)/mac/$(BINARY)-arm64 $(PKG)
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -trimpath -o $(DIST)/mac/$(BINARY)-amd64 $(PKG)
	lipo -create -output $(DIST)/mac/$(BINARY) $(DIST)/mac/$(BINARY)-arm64 $(DIST)/mac/$(BINARY)-amd64
	rm -rf "$(DIST)/mac/$(APPNAME).app"
	PATH="$(GO_BIN):$$PATH" fyne package --os darwin --name "$(APPNAME)" --app-id $(APPID) --app-version $(VERSION) \
		--icon assets/icon.png --exe $(DIST)/mac/$(BINARY) --release=false
	mv "$(APPNAME).app" "$(DIST)/mac/"

# mac-dmg builds a distributable macOS image with first-launch instructions.
mac-dmg: mac
	rm -f "$(DIST)/$(BINARY)-$(VERSION)-macos.dmg"
	rm -rf "$(DIST)/mac-dmg"
	mkdir -p "$(DIST)/mac-dmg"
	cp -R "$(DIST)/mac/$(APPNAME).app" "$(DIST)/mac-dmg/"
	cp macos/FIRST-LAUNCH.txt "$(DIST)/mac-dmg/"
	hdiutil create -volname "$(APPNAME)" -srcfolder "$(DIST)/mac-dmg" \
		-ov -format UDZO "$(DIST)/$(BINARY)-$(VERSION)-macos.dmg"

# windows builds a Windows x64 executable from macOS with mingw-w64.
# The -H windowsgui flag suppresses the console window.
windows:
	mkdir -p $(DIST)/windows
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
		go build -trimpath -ldflags "-H windowsgui" -o $(DIST)/windows/$(BINARY).exe $(PKG)

# linux cross-compiles for Ubuntu x64 in Docker with fyne-cross.
# GOTOOLCHAIN=auto is required because the fyne-cross image uses Go 1.25 with
# GOTOOLCHAIN=local, while golang.org/x/crypto requires Go 1.26.0 or newer.
# The resulting archive is placed in fyne-cross/dist/linux-amd64.
linux:
	PATH="$(GO_BIN):$$PATH" fyne-cross linux -arch=amd64 -app-id $(APPID) -app-version $(VERSION) \
		-icon assets/icon.png -name "$(BINARY)" -env GOTOOLCHAIN=auto $(PKG)
	mkdir -p $(DIST)/linux
	cp fyne-cross/dist/linux-amd64/* $(DIST)/linux/

# clean removes build outputs.
clean:
	rm -rf $(DIST) fyne-cross "$(APPNAME).app"
