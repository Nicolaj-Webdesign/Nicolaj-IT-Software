#!/usr/bin/env sh
# Baut Agent und Server. Ergebnis im Ordner dist/
# Voraussetzung: Go 1.24 oder neuer (https://go.dev/dl/)
#
#   ./build.sh          alles bauen (Agent, Server, Prüfsummen)
#   ./build.sh agent    nur den Windows-Agent
#   ./build.sh server   nur die Server (enthalten den Agent aus dist/)
#   ./build.sh sums     nur dist/SHA256SUMS.txt erzeugen
#
# Versionsnummer: Umgebungsvariable VERSION (in GitHub Actions aus dem Tag), sonst Datei VERSION.
set -e
cd "$(dirname "$0")"
VERSION=${VERSION:-$(tr -d ' \r\n' < VERSION)}
MODE=${1:-all}
export CGO_ENABLED=0
mkdir -p dist

agent() {
  echo "→ Agent $VERSION (Windows) …"
  go run ./tools/winres -version "$VERSION" -o agent/rsrc_windows_amd64.syso # Symbol + Dateiinfos
  GOOS=windows GOARCH=amd64 go build -trimpath \
    -ldflags "-s -w -H windowsgui -X main.agentVersion=$VERSION -X main.buildMarker=NRMM-AGENT-VERSION:$VERSION:END -X main.defaultServer=${DEFAULT_SERVER:-}" \
    -o dist/nicolaj-agent.exe ./agent
}

server() {
  # Der Server liefert den Agent mit aus (Installer-Download, Updates).
  cp dist/nicolaj-agent.exe server/assets/nicolaj-agent.exe
  echo "→ Server $VERSION (Windows, Linux, Raspberry Pi) …"
  LDF="-s -w -X main.serverVersion=$VERSION"
  GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$LDF" -o dist/nicolaj-rmm-server.exe ./server
  GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags "$LDF" -o dist/nicolaj-rmm-server-linux-amd64 ./server
  GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags "$LDF" -o dist/nicolaj-rmm-server-linux-arm64 ./server
}

sums() {
  echo "→ Prüfsummen …"
  cd dist
  rm -f SHA256SUMS.txt SHA256SUMS.txt.sig
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum nicolaj-agent.exe nicolaj-rmm-server* > SHA256SUMS.txt
  else
    shasum -a 256 nicolaj-agent.exe nicolaj-rmm-server* > SHA256SUMS.txt
  fi
  cd ..
}

case "$MODE" in
  agent)  agent ;;
  server) server ;;
  sums)   sums ;;
  all)    agent; server; sums ;;
  *)      echo "Unbekannt: $MODE (agent|server|sums|all)"; exit 1 ;;
esac

echo "Fertig ($VERSION):"
ls -lh dist/
