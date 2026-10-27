#!/bin/sh
# Тест совместимости плагина:
#   sh tests/compat/run.sh <каталог плагина> [версия хоста, 1.0]       — с splify2-api (заглушка splify2-<версия>/)
#   sh tests/compat/run.sh --steer <каталог плагина> [версия ядра, 2.1] — со стороной ядра steer (steer-<версия>/:
#     манифест, хуки check/expand, hello помощника, --conf у подкоманд; каталог — с собранными программами)
# Код возврата 0 — плагин подключится.
set -u
HERE=$(cd "$(dirname "$0")" && pwd)
if [ "${1:-}" = --steer ]; then
    shift
    V="${2:-2.1}"
    H="$HERE/steer-$V/host.py"
    [ -f "$H" ] || { echo "нет заглушки ядра steer-$V" >&2; exit 2; }
else
    V="${2:-1.0}"
    H="$HERE/splify2-$V/host.py"
    [ -f "$H" ] || { echo "нет заглушки хоста splify2-$V" >&2; exit 2; }
fi
[ -d "${1:-}" ] || { echo "использование: run.sh [--steer] <каталог плагина> [версия]" >&2; exit 2; }
exec python3 "$H" "$1"
