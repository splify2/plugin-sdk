/* Программа-помощник плагина: steer запускает её под супервизором на каждый выход вида example
 * (steer.kinds[].helper в plugin.json), а она рассказывает о себе строками событий (evline).
 *
 *   example-helper example <выход> --conf <файл>   — помощник выхода
 *   example-helper example-hello                   — подкоманда `steer example-hello`
 *
 * Правила: первое событие — hello; дальше up, когда выход готов, и down с причиной для человека,
 * когда нет; журнал — stderr; на SIGTERM — выйти (супервизор остановит и по PDEATHSIG). */
#include <signal.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>

#include "s2p/evline.h"

#ifndef PLUGIN_ID
#define PLUGIN_ID "example-helper"
#endif
#ifndef PLUGIN_VERSION
#define PLUGIN_VERSION "0.0.0"
#endif

static volatile sig_atomic_t stop;

static void on_term(int sig) { (void)sig; stop = 1; }

static int helper(const char *out, const char *conf) {
    s2p_ev_hello(PLUGIN_VERSION, PLUGIN_ID);        /* первым, до всего остального */

    /* Файл {conf} — всё, что помощник знает о своём выходе (docs/steer-api.md, 1.4): JSON с ключами
     * вида, устройством, меткой. Разбор JSON — ваш выбор; здесь только проверяем, что он есть. */
    FILE *f = fopen(conf, "r");
    if (!f) {
        fprintf(stderr, "%s: файл настройки %s не открылся\n", PLUGIN_ID, conf);
        s2p_ev_down("нет файла настройки");
        return 2;
    }
    fseek(f, 0, SEEK_END);
    long n = ftell(f);
    fclose(f);
    fprintf(stderr, "%s: выход %s, настройка %ld байт\n", PLUGIN_ID, out, n);

    s2p_ev_up(0, NULL);                              /* выход готов; watch 0 — здоровье судит сторож */
    while (!stop) pause();
    s2p_ev_down("помощник остановлен");
    return 0;
}

int main(int argc, char **argv) {
    struct sigaction sa;
    memset(&sa, 0, sizeof sa);
    sa.sa_handler = on_term;
    sigaction(SIGTERM, &sa, NULL);
    sigaction(SIGINT, &sa, NULL);

    if (argc == 5 && !strcmp(argv[1], "example") && !strcmp(argv[3], "--conf")) return helper(argv[2], argv[4]);
    if (argc == 2 && !strcmp(argv[1], "example-hello")) {
        puts("Привет от " PLUGIN_ID " " PLUGIN_VERSION);
        return 0;
    }
    /* Код 2 и причина в последней строке stderr — так ядро показывает отказ человеку. */
    fprintf(stderr, "%s: так вызывать нельзя\n", PLUGIN_ID);
    return 2;
}
