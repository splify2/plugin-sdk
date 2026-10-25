#!/bin/sh
# Сборка инструмента s2p: один статический файл без зависимостей (Go 1.21+, сеть не нужна).
#
#   sh build.sh                         → bin/s2p
#   GOOS=darwin GOARCH=arm64 sh build.sh → bin/s2p для другой платформы
#   S2P_OUT=/usr/local/bin sh build.sh   → в другой каталог
#
# Схемы, заголовки steer-abi, evline и шаблоны вшиваются в файл (go:embed): рядом с ним дерево SDK не нужно.
set -eu
cd "$(dirname "$0")"
OUT="${S2P_OUT:-bin}"
mkdir -p "$OUT"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$OUT/s2p" ./cmd/s2p
echo "$OUT/s2p: $("$OUT/s2p" version 2>/dev/null || echo собран для другой платформы)"
