#!/bin/sh
# release.sh TAG: builds uw1go for macOS (a universal .app) and Windows with the
# game from gamedata/UW bundled, and publishes them as TAG's GitHub release.
# Runs on a Mac (the .app needs cgo for both architectures); the repo is
# private because the release carries the game data.
set -e
tag=${1:?usage: release.sh TAG}
out=dist/$tag
rm -rf "$out" && mkdir -p "$out"
go test ./...

app="$out/mac/Ultima Underworld.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
for a in arm64 amd64; do
	CGO_ENABLED=1 GOARCH=$a go build -trimpath -ldflags "-s -w" -o "$out/uw1-$a" ./cmd/uw1
done
lipo -create -output "$app/Contents/MacOS/uw1" "$out/uw1-arm64" "$out/uw1-amd64"
cp -R gamedata/UW "$app/Contents/Resources/UW"
cat >"$app/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
	<key>CFBundleName</key><string>Ultima Underworld</string>
	<key>CFBundleExecutable</key><string>uw1</string>
	<key>CFBundleIdentifier</key><string>de.ruzzoli.uw1go</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>${tag#v}</string>
	<key>LSMinimumSystemVersion</key><string>11.0</string>
	<key>NSHighResolutionCapable</key><true/>
</dict></plist>
EOF
codesign --force --deep -s - "$app"
(cd "$out/mac" && ditto -c -k --keepParent "Ultima Underworld.app" "../uw1go-$tag-macos.zip")

win="$out/uw1go-$tag-windows"
mkdir -p "$win/gamedata"
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H windowsgui" -o "$win/uw1.exe" ./cmd/uw1
cp -R gamedata/UW "$win/gamedata/UW"
(cd "$out" && zip -qr "uw1go-$tag-windows.zip" "uw1go-$tag-windows")

[ -n "$NOPUBLISH" ] && exit 0
gh release create "$tag" --title "$tag" \
	--notes "Ready to play, game included. macOS: unzip and open Ultima Underworld.app (first time: right-click → Open). Windows: unzip and run uw1.exe. F11 toggles fullscreen. Multiplayer: Host/Join a game online, or set the server to none to play over the LAN (port 7781)." \
	"$out/uw1go-$tag-macos.zip" "$out/uw1go-$tag-windows.zip"
