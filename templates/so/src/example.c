/* Плагин .so для steerd по steer-abi v1 (docs/steer-abi.md, include/steer/plugin.h).
 *
 * Правила: связи с libsteer нет — всё от ядра приходит таблицей host_v1 в init; наружу виден один
 * символ steer_plugin_v1_entry; функции не блокируют (сеть и имена — через host_v1); падение .so
 * роняет steerd, поэтому только то, что нужно в процессе ядра. Остальное — программа-помощник. */
#include <stdio.h>
#include <string.h>

#include "steer/plugin.h"

#ifndef PLUGIN_ID
#define PLUGIN_ID "example-so"
#endif
#ifndef PLUGIN_VERSION
#define PLUGIN_VERSION "0.0.0"
#endif

static const struct steer_host_v1 *api;
static struct steer_host *self;

static int p_init(struct steer_host *h, const struct steer_host_v1 *host, char *why, size_t n) {
    (void)why; (void)n;
    self = h;
    api = host;
    api->log(self, STEER_LOG_INFO, "%s %s загружен, ядро %s, steer-api %s",
             PLUGIN_ID, PLUGIN_VERSION, api->core_version, api->api_version);
    return 0;   /* не 0 — плагин failed с причиной why */
}

static void p_fini(struct steer_host *h) { (void)h; }

/* Проверка выхода после схемы ключей: 0 — годится, -1 — отказ с причиной для человека. */
static int k_check(struct steer_host *h, const struct steer_spec *spec, const struct steer_out *out,
                   char *why, size_t n) {
    (void)h; (void)spec;
    const char *keys = api->out_keys_json(out);
    if (keys && strstr(keys, "\"greeting\":\"\"")) {
        snprintf(why, n, "приветствие не может быть пустым");
        return -1;
    }
    return 0;
}

/* Правила вида: host->nft_rule(h, "<слот>", "<правило>") — нужны право nft и слоты из steer-api.md. */
static void k_emit(struct steer_host *h, const struct steer_spec *spec, const struct steer_out *out) {
    (void)h; (void)spec; (void)out;
}

static const struct steer_kind_v1 kind = {
    .size = sizeof kind,
    .name = "example-so",
    .caps = 0,
    .check = k_check,
    .emit = k_emit,
};

static const struct steer_kind_v1 *const kinds[] = { &kind, NULL };

static const struct steer_plugin_v1 plugin = {
    .size = sizeof plugin,
    .abi_major = STEER_ABI_MAJOR,
    .abi_minor = STEER_ABI_MINOR,
    .fingerprint = STEER_ABI_FINGERPRINT,
    .id = PLUGIN_ID,
    .version = PLUGIN_VERSION,
    .init = p_init,
    .fini = p_fini,
    .kinds = kinds,
};

/* Единственный экспорт. Ядро ABI 1.y принимает плагин 1.x при x <= y. */
__attribute__((visibility("default")))
const struct steer_plugin_v1 *steer_plugin_v1_entry(uint16_t host_major, uint16_t host_minor) {
    if (host_major != STEER_ABI_MAJOR || host_minor < STEER_ABI_MINOR) return NULL;
    return &plugin;
}
