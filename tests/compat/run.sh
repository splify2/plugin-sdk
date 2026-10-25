#!/bin/sh
# Тест совместимости плагина с splify2-api: `sh tests/compat/run.sh <каталог плагина> [версия хоста, 1.0]`.
# Версия — каталог заглушки tests/compat/splify2-<версия>/; без неё — 1.0. Код возврата 0 — плагин подключится.
set -u
HERE=$(cd "$(dirname "$0")" && pwd)
V="${2:-1.0}"
H="$HERE/splify2-$V/host.py"
[ -f "$H" ] || { echo "нет заглушки хоста splify2-$V" >&2; exit 2; }
[ -d "${1:-}" ] || { echo "использование: run.sh <каталог плагина> [версия]" >&2; exit 2; }
exec python3 "$H" "$1"
