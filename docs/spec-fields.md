# Новые поля модели и спеки: договорённость для агентов ядра

Этот файл — **договор**, чтобы параллельные агенты ядра (модель и виды; демон, супервизор и status;
менеджер; упаковка) не столкнулись в `steer/src/model/spec.h` и на границах между собой. Кто что
правит — колонка «Хозяин»: остальные только читают через названные функции. Адреса — steer `9f0e1fd`.

Агенты (как в PLUGINS.md 11): **А-модель** — `src/model/`, `src/kinds/`, `spec.h`; **А-демон** —
`src/daemon/` (supd, helpers, status, ctl, apply); **А-менеджер** — менеджер плагинов (`src/plugin/`,
новый каталог); **А-упаковка** — `build.sh`, `build/`.

## 1. `struct output` (`steer/src/model/spec.h:492-555`)

| Поле | Было | Стало | Хозяин | Читают |
|---|---|---|---|---|
| владелец | `char warp_of[32]` — выход заведён разворотом `kind: warp` (`spec.h:521-523`; пишет `v2.c:1262`) | `const char *owner_plugin; const char *owner_out;` — в арене спеки; `NULL` — выход человека. `owner_out` — выход, развёрнутый хуком `expand`; `NULL` у добавленного `pre_apply` | А-модель | `supd.c:350, 369` (А-демон), `v2print.c:557`, `awg.c:1333` (А-модель), status (А-демон) |
| недоступность | — | `const char *unavail;` — причина для человека («нужен плагин vless», «плагин vless отключён», «спека не подготовлена — нужен reload»); `const char *unavail_plugin;` — id, если известен | А-модель | status, diag, apply, сторож |
| вид плагина | вид — `const struct kind_ops *kind` (`spec.h:501`) | **без изменений**: у каждого вида плагина своя запись `struct kind_ops`, которую реестр строит при загрузке (куча, живёт до `plugin-reload`) | А-модель | все |
| настройка вида плагина | union видов (`spec.h:524-535`) | новый член `struct plugin_out plg;` (ниже) | А-модель | А-демон (помощник, `{conf}`) |

```c
/* spec.h — настройка выхода вида плагина. Строки — в арене спеки. */
struct plugin_out {
    const struct plugin_kind *pk;   /* запись реестра: плагин, схема, caps, помощник, хуки (kinds/plugin.h) */
    const char *keys_json;          /* ключи вида после схемы, пути x-path абсолютные */
    uint32_t *ports;  size_t ports_n;   /* ресурсы выхода (alloc.json) */
    uint32_t *queues; size_t queues_n;
};
```

Функции — вместо прямого чтения полей (объявляет А-модель в `spec.h`, сразу, первым коммитом, даже
заглушками):

```c
int out_owned_by(const struct output *o, const char *plugin, const char *out); /* out NULL — любой выход плагина */
const char *out_owner_plugin(const struct output *o);   /* NULL — выход человека */
const char *out_unavail(const struct output *o);        /* NULL — доступен */
const struct plugin_kind *out_plugin_kind(const struct output *o); /* NULL — вид ядра */
const char *out_plugin_keys(const struct output *o);    /* JSON или NULL */
```

## 2. `struct group_cfg` (`spec.h:354-420`)

| Поле | Было | Стало | Хозяин |
|---|---|---|---|
| `warp` | `struct { on, transport, xcc_set, count, cc, cc_n, xcc, xcc_n } warp` (`spec.h:384-398`) | **удаляется**: ключи `kind: warp` — ключи плагина warpbox; группа, в которую он развернётся, — обычная группа | А-модель |
| `enum warp_transport` | `spec.h:350-352` | удаляется | А-модель |
| помощник группы | `group_helper` при `g->warp.on` (`steer/src/kinds/group.c:359-379`) | помощник **вида плагина** с `helper: true` в ответе `expand` (steer-api.md, 3.3); подпись — хеш `{conf}` | А-модель, А-демон |

Пока warpbox — плагин 2.1 с разворотом в ядре (переходный шаг), `p_warp` (`steer/src/model/v2.c:1133-1265`)
переезжает в хук `expand` плагина; в ядре его не остаётся.

## 3. Виды ядра

| Вид | Было | Стало |
|---|---|---|
| `warp-masque` (`kind_warpm`, `group.c:381-405`; вне реестра, разбор находит сам, `v2.c:748-752`) | член WARP с TUN модуля warpbox | **`plugin-device`** — общий вид «устройство держит процесс владельца»: caps `device mark ctmark engine-owned over ipv6`, устройство — имя из ключа `device` или выведенное, как у awg. Принимается только в подготовленной спеке и только с `owner` |
| `vless`, `hysteria2`, `trojan`, `shadowsocks`, `socks`, `http`, `vmess`, `xsteer` (`kind.c:30-39, 48-61`) | записи видов libsteer, доступность — по файлу модуля | **уходят из ядра**: виды плагинов `tunnel:<протокол>` и `xsteer` |
| `tgws` | вид ядра с `emit` (`tgws.c:115-140`) | вид плагина `tgws` с шаблоном nft в слоте `prerouting-dnat` |
| `awg` | вид ядра | остаётся в ядре; доступен, если стоит и включён плагин `awg` (`impl: core`, решение В2) |
| `direct`, `interface`, `group`, `block` | ядро | ядро; `interface` с `obfs` берёт помощника `obfs` у плагина `obfs` по имени подкоманды |
| недоступный | — | разбор выхода неизвестного/отключённого вида при старте и `reload` (steer-api.md, 3.5): вид `unavail` (caps `device mark`), `failed = 1`, `on_fail` из спеки; в `status` — его прежнее имя вида |

## 4. `struct out_keys` и ключи v2

| Что | Было | Стало | Хозяин |
|---|---|---|---|
| `opts_file` / `strategy` (`spec.h:430`, `v2.c:629`, `v1.c:332, 482`) и `struct zapret_cfg` (`spec.h:316-323`, член `zp` `:532`) | файл настройки внешнего модуля DPI (recheck) | удаляются: recheck — служба плагина, своих ключей выхода нет (В7) | А-модель |
| ключи туннелей `subscription`, `nodes`, `transport`, `insecure`, `exclude*`, `only`, пул (`v2.c:611-631, 651-656`) и поля `out_keys` под них (`spec.h:426-451`) | общий разбор ядра | ключи вида плагина по его схеме; ядро их не разбирает, а отдаёт `keys_json` | А-модель |
| `stream`, `stream_port` (xsteer), `domain` (tgws), `conf` у xsteer | ключи ядра | ключи плагинов `xsteer`, `tgws` | А-модель |
| `conf` у awg | ключ ядра | остаётся (awg в ядре) | — |

## 5. Помощники (`steer/src/kinds/kind.h:96-110`, `steer/src/daemon/helpers.h:83-119`)

| Поле | Было | Стало | Хозяин |
|---|---|---|---|
| `kind_helper.cmd[16]`, `prog[16]`, `arg[2][256]`, `env[32]` | фиксированные массивы | `char *cmd; char *prog; char **argv; size_t argc; char **envv; size_t envc; const char *plugin; char *conf_path;` — любой длины | А-модель (kind.h) |
| `helper.cmd[16]`, `name[32]`, `prog[16]`, `arg[2][256]`, `env[32]`, `module`, `mver[32]` | то же + признак модуля и его версия | те же указатели, `plugin` (id), `api[16]` из `hello` вместо `mver`; `module` уходит | А-демон (helpers.h) |
| передача состава | ребёнок `supervise` пишет записи `struct helper` в трубу байтами (`steer/src/daemon/supervise.c:79-85`) — с указателями так нельзя | сериализация состава строками JSON (`helpers_plan_write` / `helpers_plan_read`) | А-демон |
| версия модуля | `hello.ver` сверяется с версией ядра (`supd.c:323-340`) | `hello.api` сверяется со steer-api (steer-api.md, 2.1) | А-демон |

## 6. `struct spec` (`spec.h:797-…`)

| Поле | Что | Хозяин |
|---|---|---|
| `int prepared;` | спека прочитана из подготовленной (`spec.prepared.json` с совпавшим ключом) | А-модель |
| `char prepared_key[65];` | sha256 ключа подготовки (это хеш, длина — свойство sha256) | А-модель |

`load_spec` получает режим: `LOAD_PREPARED` (умолчание — все места чтения спеки) и `LOAD_HUMAN`
(`steer spec convert`, печать, `v2print`). Шаг подготовки — `int spec_prepare(const char *path, struct
err *e)` в `src/plugin/prepare.c` (А-менеджер), зовут его `apply-plan`, `check`, `reload`, старт демона
(А-демон).

## 7. Спека в файле (v2)

| Где | Поле | Где разрешено | Смысл |
|---|---|---|---|
| `outputs.<n>.kind` | имя вида плагина (`warp`, `xsteer`, `tgws`) или `tunnel` + `protocol: <протокол плагина>` | везде | вид из реестра плагинов |
| `outputs.<n>.owner` | `{plugin, out?}` | **только подготовленная спека** | кто завёл выход; в спеке человека — отказ «owner пишет только плагин» |
| `rules[i].owner`, `lists.<n>.owner`, `clients.<n>.owner` | `{plugin}` | только подготовленная | добавлено хуком `pre_apply` |
| `outputs.<n>.kind: plugin-device` | — | только подготовленная, с `owner` | устройство держит процесс владельца |

Раздела `plugins:` в спеке нет: что стоит и включено — `state.json` менеджера, настройки плагина —
`conf/<id>/` ([open.md](open.md), О-14). Плагинные виды есть только в спеке v2; спека v1 (JSON до 2.0.0)
плагинных видов не знает — переход см. [migration-2.1.md](migration-2.1.md).

## 8. Выводы ядра (JSON)

| Где | Поле | Хозяин |
|---|---|---|
| `steer status` → `outputs.<n>.owner` | `{"plugin":"warpbox","out":"w"}` | А-демон |
| `outputs.<n>.unavail` | `{"why":"…","plugin":"vless"}` | А-демон |
| `outputs.<n>.plugin` | `{"id":"warpbox","version":"1.4.0","state":{…}}` — `state` — файл `<state>/plugin/<id>/<выход>.status.json` или `steer_kind_v1.status`; замена `group.warp` (`status.c:143-183, 268`) | А-демон |
| `outputs.<n>.kind` | прежнее имя (`vless` у туннеля), как `out_kind_shown` (`kind.h:265-269`) | — |
| `status.plugins` | `[{"id","version","state","why"}]` — сводка | А-демон |
| `steer version --json` | `{"core","api","abi","rev","build"}` | А-демон |
| команда сокета `helper` | `module`, `module_ver` (`steer/docs/ctl.md:579-583`) → `plugin`, `plugin_api`; `rejected` и `last_down` — как были | А-демон |
| ответ `apply` | `"plugins":{"pre_apply":[…],"expand":[…]}` — что изменили хуки | А-менеджер (подготовка), А-демон (ответ) |
| событие `plugin`, `plugin-state` | steer-api.md, 4.1 | А-менеджер, А-демон |
| `steer nft-map --json` | steer-api.md, 5.1 | А-модель (компилятор) |

## 9. Порядок

1. А-модель первым коммитом кладёт в `spec.h` поля и функции разделов 1, 6 (с заглушками) и
   `kinds/plugin.h` с `struct plugin_kind` — остальные агенты строятся на них.
2. А-демон переводит `helper`/`kind_helper` (раздел 5) — после шага 1, в одной ветке с А-моделью по
   `kind.h`.
3. Удаление `group_cfg.warp`, `warp_of`, `zapret_cfg`, видов протоколов — последним, когда плагины
   warpbox и протоколов собраны и проходят стенды.
