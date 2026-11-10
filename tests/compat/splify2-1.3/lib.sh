#!/bin/sh
# Заглушка библиотеки для shell-плагинов (splify2-api 1.1, docs/splify2-api.md, раздел 7): то же, что у хоста, — jshn,
# s2_fail, s2_fail_code, s2_t, s2_setting, — и ничего сверх. Словарей языка в заглушке нет: s2_t и s2_fail_code отдают английский
# текст вызова с подставленными {именами}; в ответе, как у хоста, ещё code и args. Окружения каркаса здесь нет: плагин, который за ним тянется, в заглушке не
# заработает. Путь к файлу — SPLIFY2_LIB; jshn (в заглушке — python-реализация рядом) на роутере даёт libubox.
. "${SPLIFY2_LIB%/*}/jshn.sh"

s2_fail() {  # ТЕКСТ
    json_init
    json_add_boolean ok 0
    json_add_string error "$1"
    json_dump
    exit 0
}

# Подстановка {имя} -> значение: s2_subst ТЕКСТ [ИМЯ ЗНАЧЕНИЕ]...
s2_subst() {
    _s2s_t="$1"; shift
    while [ $# -ge 2 ]; do
        _s2s_n="{$1}"; _s2s_out=""
        while case "$_s2s_t" in *"$_s2s_n"*) true ;; *) false ;; esac; do
            _s2s_out="$_s2s_out${_s2s_t%%"$_s2s_n"*}$2"
            _s2s_t="${_s2s_t#*"$_s2s_n"}"
        done
        _s2s_t="$_s2s_out$_s2s_t"
        shift 2
    done
    printf '%s' "$_s2s_t"
}

s2_t() {  # КОД ТЕКСТ [ИМЯ ЗНАЧЕНИЕ]...
    _s2t_d="$2"; shift 2
    s2_subst "$_s2t_d" "$@"
}

s2_fail_code() {  # КОД ТЕКСТ [ИМЯ ЗНАЧЕНИЕ]...
    _s2f_c="$1"; _s2f_d="$2"; shift 2
    _s2f_x="$(s2_subst "$_s2f_d" "$@")"
    json_init
    json_add_boolean ok 0
    json_add_string error "$_s2f_x"
    json_add_string code "$_s2f_c"
    json_add_object args
    while [ $# -ge 2 ]; do json_add_string "$1" "$2"; shift 2; done
    json_close_object
    json_dump
    exit 0
}

s2_setting() {  # КЛЮЧ
    "$SPLIFY2_HOST" settings get "$1"
}
