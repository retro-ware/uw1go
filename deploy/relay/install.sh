#!/bin/sh
# install.sh: build uw1relay for the server and install it there as a systemd socket
# service (once, and again after relay changes). Asks for the sudo password on the server.
# nginx's /goplay/uw1relay location comes with goplay's deploy.sh.
set -e
cd "$(dirname "$0")/../.."
host=${HOST:-ruzzoli.de}
B=$(mktemp -d)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags=-s -o "$B/uw1relay" ./cmd/uw1relay
cp deploy/relay/uw1-relay.socket deploy/relay/uw1-relay.service "$B/"
ssh "$host" 'mkdir -p ~/uw1-relay'
scp -q "$B"/* "$host:uw1-relay/"
rm -rf "$B"
ssh -t "$host" 'set -e
	sudo install -m 755 ~/uw1-relay/uw1relay /usr/local/bin/uw1relay
	sudo install -m 644 ~/uw1-relay/uw1-relay.socket ~/uw1-relay/uw1-relay.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl stop uw1-relay.service 2>/dev/null || true
	sudo systemctl enable --now uw1-relay.socket
	systemctl status --no-pager uw1-relay.socket | head -5'
