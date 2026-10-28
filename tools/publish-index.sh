#!/bin/sh
# Публикация репозитория плагинов на GitHub Pages: архивы .s2p → index.json и index.json.sig в ветке gh-pages
# репозитория-индекса. Общий шаг выпусков steer, andromeda и splify2-extended-plugins и плановой переподписи индекса.
#
#   S2P=<s2p> INDEX_SLUG=splify2/plugins INDEX_ID=splify2 INDEX_KEY=index.sec PUBS="protocols.pub" \
#   ARCHIVES=<каталог .s2p> [META=meta.json] [INDEX_NAME=…] PUBLISH_TOKEN=<токен> sh tools/publish-index.sh
#
# ARCHIVES пустой или не задан — только переподпись: прежний индекс с новыми generated и expires (индекс живёт
# INDEX_EXPIRES, по умолчанию 30 дней; роутер не принимает просроченный, поэтому его переподписывают по расписанию).
# Прежние версии плагинов остаются в индексе (s2p index сливает с прежним). Адреса архивов — Pages
# (https://<владелец>.github.io/<репозиторий>/) и зеркало jsDelivr ветки gh-pages.
#
# DRY_RUN=<каталог> — без сети и токена: индекс собирается в этом каталоге (с прежним индексом, если он там есть),
# ничего не отправляется.
set -eu
S2P="${S2P:-s2p}"
: "${INDEX_SLUG:?нужен INDEX_SLUG — владелец/репозиторий индекса, например splify2/plugins}"
: "${INDEX_ID:?нужен INDEX_ID — id репозитория в repos.d (splify2, splify2-core-plugins, …)}"
: "${INDEX_KEY:?нужен INDEX_KEY — закрытый ключ роли index (секрет S2P_KEY_INDEX)}"
: "${PUBS:?нужен PUBS — открытые ключи, которыми подписаны архивы (s2p index проверяет их и при переподписи)}"
[ -s "$INDEX_KEY" ] || { echo "publish-index: ключ $INDEX_KEY пуст или его нет" >&2; exit 1; }
owner="${INDEX_SLUG%%/*}"; repo="${INDEX_SLUG#*/}"
pages="https://$owner.github.io/$repo/"
cdn="https://cdn.jsdelivr.net/gh/$INDEX_SLUG@gh-pages/"

work="$(mktemp -d "${TMPDIR:-/tmp}/s2p-pub.XXXXXX")"
trap 'rm -rf "$work"' EXIT
if [ -n "${DRY_RUN:-}" ]; then
    site="$DRY_RUN"; mkdir -p "$site"
else
    : "${PUBLISH_TOKEN:?нужен PUBLISH_TOKEN — токен с правом записи в $INDEX_SLUG (секрет PLUGINS_TOKEN)}"
    site="$work/site"
    url="https://x-access-token:$PUBLISH_TOKEN@github.com/$INDEX_SLUG"
    # Первая публикация: ветки gh-pages ещё нет — начинается сиротская.
    if git ls-remote --exit-code --heads "$url" gh-pages >/dev/null 2>&1; then
        git clone -q --depth 1 --branch gh-pages "$url" "$site"
    else
        git init -q -b gh-pages "$site"
        git -C "$site" remote add origin "$url"
    fi
fi

in="$work/in"; mkdir -p "$in"
if [ -n "${ARCHIVES:-}" ] && [ -d "$ARCHIVES" ]; then
    find "$ARCHIVES" -name '*.s2p' -exec cp {} "$in/" \;
fi
n="$(find "$in" -name '*.s2p' | wc -l)"
if [ "$n" -eq 0 ] && [ ! -f "$site/index.json" ]; then
    echo "publish-index: нет ни архивов, ни прежнего индекса в $INDEX_SLUG — публиковать нечего" >&2; exit 1
fi

set -- --repo "$INDEX_ID" --key "$INDEX_KEY" --mirror "$pages" --mirror "$cdn" --expires "${INDEX_EXPIRES:-30d}"
[ -z "${INDEX_NAME:-}" ] || set -- "$@" --name "$INDEX_NAME"
[ -z "${META:-}" ] || set -- "$@" --meta "$META"
for p in ${PUBS:-}; do set -- "$@" --pub "$p"; done
"$S2P" index "$in" -o "$site" "$@"
# Pages без Jekyll: иначе каталоги и файлы с подчёркиванием не отдаются.
: > "$site/.nojekyll"

if [ -n "${DRY_RUN:-}" ]; then
    echo "publish-index: сухой прогон — индекс $INDEX_ID ($n новых архивов) в $site"; exit 0
fi
cd "$site"
git config user.name 'github-actions[bot]'
git config user.email 'github-actions[bot]@users.noreply.github.com'
git add -A
if git diff --cached --quiet; then echo "publish-index: $INDEX_SLUG без изменений"; exit 0; fi
if [ "$n" -gt 0 ]; then msg="Индекс $INDEX_ID: $n архивов"; else msg="Индекс $INDEX_ID: переподпись"; fi
git commit -q -m "$msg"
git push -q origin gh-pages
echo "publish-index: $INDEX_SLUG — $msg"
