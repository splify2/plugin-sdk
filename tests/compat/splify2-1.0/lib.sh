#!/bin/sh
# Заглушка библиотеки для shell-плагинов (splify2-api 1.0, docs/splify2-api.md, раздел 7): то же, что у хоста, — jshn,
# s2_fail, s2_setting, — и ничего сверх. Окружения каркаса здесь нет: плагин, который за ним тянется, в заглушке не
# заработает. Путь к файлу — SPLIFY2_LIB; jshn (в заглушке — python-реализация рядом) на роутере даёт libubox.
. "${SPLIFY2_LIB%/*}/jshn.sh"

s2_fail() {  # ТЕКСТ
    json_init
    json_add_boolean ok 0
    json_add_string error "$1"
    json_dump
    exit 0
}

s2_setting() {  # КЛЮЧ
    "$SPLIFY2_HOST" settings get "$1"
}
