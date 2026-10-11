BINARY  := sshtunnel
APPNAME := SSH Tunnel
APPID   := com.geekproxy.sshtunnel
PKG     := ./cmd/sshtunnel
DIST    := dist
VERSION := 1.0.0

.PHONY: all test vet run icon tools mac mac-dmg windows linux clean

# all — проверка кода и сборка под текущую ОС.
all: vet test mac

# vet — статическая проверка компилятором.
vet:
	go vet ./...

# test — все тесты с детектором гонок.
test:
	go test -race -timeout 120s ./...

# run — запуск приложения из исходников.
run:
	go run $(PKG)

# icon — перерисовка assets/icon.png генератором.
icon:
	go run ./tools/genicon

# tools — установка упаковщиков Fyne: fyne для .app/.exe, fyne-cross для Linux в Docker.
tools:
	go install fyne.io/tools/cmd/fyne@latest
	go install github.com/fyne-io/fyne-cross@latest

# mac — универсальный бинарник (Apple Silicon + Intel) и бандл .app.
mac:
	mkdir -p $(DIST)/mac
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -trimpath -o $(DIST)/mac/$(BINARY)-arm64 $(PKG)
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -trimpath -o $(DIST)/mac/$(BINARY)-amd64 $(PKG)
	lipo -create -output $(DIST)/mac/$(BINARY) $(DIST)/mac/$(BINARY)-arm64 $(DIST)/mac/$(BINARY)-amd64
	rm -rf "$(DIST)/mac/$(APPNAME).app"
	fyne package --os darwin --name "$(APPNAME)" --app-id $(APPID) --app-version $(VERSION) \
		--icon assets/icon.png --exe $(DIST)/mac/$(BINARY) --release=false
	mv "$(APPNAME).app" "$(DIST)/mac/"

# mac-dmg — образ для распространения на macOS, собирается из готового .app.
mac-dmg: mac
	rm -f "$(DIST)/$(BINARY)-$(VERSION)-macos.dmg"
	hdiutil create -volname "$(APPNAME)" -srcfolder "$(DIST)/mac/$(APPNAME).app" \
		-ov -format UDZO "$(DIST)/$(BINARY)-$(VERSION)-macos.dmg"

# windows — .exe под Windows x64, собирается с этого мака компилятором mingw-w64.
# Флаг -H windowsgui убирает окно консоли при запуске.
windows:
	mkdir -p $(DIST)/windows
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
		go build -trimpath -ldflags "-H windowsgui" -o $(DIST)/windows/$(BINARY).exe $(PKG)

# linux — сборка под Ubuntu x64 в контейнере Docker через fyne-cross.
# GOTOOLCHAIN=auto передаётся в контейнер потому, что в образе fyne-cross
# стоит Go 1.25 с GOTOOLCHAIN=local, а golang.org/x/crypto требует go >= 1.26.0.
# Готовый архив кладётся в fyne-cross/dist/linux-amd64.
linux:
	fyne-cross linux -arch=amd64 -app-id $(APPID) -app-version $(VERSION) \
		-icon assets/icon.png -name "$(BINARY)" -env GOTOOLCHAIN=auto $(PKG)
	mkdir -p $(DIST)/linux
	cp fyne-cross/dist/linux-amd64/* $(DIST)/linux/

# clean — удаление результатов сборки.
clean:
	rm -rf $(DIST) fyne-cross "$(APPNAME).app"
