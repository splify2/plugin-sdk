# Переход на steer-core 2.1.0: три пакета и плагины

Решения 7–9, В4, В5, В10 и «steer-box-connector тоже плагин» (DIRECTIONS.md хаба): продуктовых пакетов apk/ipk
два — `steer-core` и `splify2`; коннектор — плагин `box-connector`, а пакет `steer-box-connector` остался без
файлов — он только держит имя `sing-box` для podkop и forkop (раздел 6);
все протоколы — плагины из официального репозитория, они ставятся изначально; установка `steer-core`
2.1.0 снимает все прежние пакеты `steer-*`; «семя» лежит в `steer-core`, поэтому переход и первая
установка проходят без сети. Адреса — steer `9f0e1fd`.

## 1. Что было и что стало

| Пакет 2.0.5 | Что в нём (`steer/build.sh:410-430, 586-613`) | В 2.1.0 |
|---|---|---|
| `steer-core` | steerd, `steer`, `libsteer.so.2.0.5` (модель, виды, TLS, транспорты, стек), `libsteer-wolfssl.so.5.9.4`, init, hotplug, keep.d | `steer-core` 2.1.0: steerd, `steer`, libsteer **без кода протоколов**, `libsteer-wolfssl`, `libsteer-plugin.so.1`, менеджер плагинов, ключи, семя, служба `steer-plugins`, `keep.d/steer-plugins` |
| `steer-vless` | `/usr/sbin/steer-vless` | плагин `vless` + библиотека `net1` |
| `steer-hysteria2` | `steer-hysteria2` | плагин `hysteria2` + `net1` |
| `steer-proxy` | `steer-proxy` (trojan, shadowsocks, socks, http, vmess) | плагин `proxy` + `net1` |
| `steer-xsteer` | `steer-xsteer` | плагин `xsteer` + `net1` |
| `steer-obfs` | `steer-obfs` | плагин `obfs` |
| `steer-tgws` | `steer-tgws` | плагин `tgws` репозитория `splify2-extended-plugins` (статическая программа со своим TLS; исходник — `steer/plugins/tgws`), раздел 2.3 |
| `steer-warpbox` | `steer-warpbox` | плагин `warpbox` + `awg` + `net1` |
| `steer-extended` | мета-пакет: ядро, vless, xsteer, obfs, tgws (`build.sh:615-630`) | плагины `vless`, `xsteer`, `obfs` (из семени) и `tgws` (из `splify2-extended-plugins`) |
| — (вид `awg` в ядре) | — | плагин `awg` (`impl: core`: зависимости `kmod-amneziawg\|kmod-wireguard` и UI; код вида в ядре, В2) |
| `stgws` (brb, вне дерева steer) | мини-сборка моста Telegram профиля `tgws` (`steer/src/profile/tgws.c:1-33`) | плагин `tgws`; мини-сборки в дереве steer больше нет, раздел 6а |
| `steer-box-connector` | sing-box на libsteer, свой установщик модулей (`steer-box-connector/files/usr/libexec/rpcd/steer-box`) | плагин `box-connector` (статическая программа со своей копией TLS, `compat.steer-api: ^1.3`, пути в системе — `steer.links`) и пакет `steer-box-connector` без файлов: `provides: sing-box`, `steer-core >= 2.1.0`; протоколы ставит `steer plugin ensure` (раздел 6) |
| `splify2` 26.10.x | ставит и снимает `steer-<m>` пакетами (`files/usr/lib/splify2/rpcd/m-engine.sh:388-459, 849-943`) | `splify2` 26.11: раздел «Плагины» над менеджером ядра; семя `lang-en` |

## 2. Пакетный механизм

**Проверено на настоящих менеджерах** (apk 3.0.5 на стенде QEMU OpenWrt 25.12.5; opkg 24.10.2 в
контейнере `openwrt/rootfs:x86-64-24.10.2`; пакеты — настоящие 2.0.5 и 2.1.0, `steer/build.sh`;
сценарии — `steer/tests/pkglayout.sh`, раздел 4).

### 2.1 apk

`steer-core` 2.1.0 объявляет прежние пакеты своими — тот же приём, что при переходе `steer` → `steer-core`:
`provides: steer-vless=2.1.0-r1 …` и `replaces:` тем же списком (`steer-vless steer-hysteria2 steer-proxy
steer-xsteer steer-obfs steer-warpbox steer-tgws steer-extended`). Больше ничего не нужно:
решатель видит, что имена из `world` закрыты ядром, а прежние модули требуют `steer-core=2.0.5-r1`, и снимает их
(«Purging steer-vless …») в той же транзакции. `apk upgrade` из фида с индексом и `apk add steer-core.apk
steer-tgws.apk` дают одно и то же. Варианты, проверенные на apk:

| Вариант | Итог на apk |
|---|---|
| `provides` + `replaces` (выбран) | работает |
| только `provides` (без `replaces`) | тоже работает; `replaces` нужен, чтобы ядро вправе было занять файлы |
| `!steer-vless` и `!steer-vless<2.1.0` в depends ядра, без `provides` | **отказ**: модуль в `world` нечем заменить («breaks: steer-core-2.1.0[!steer-vless]»), апгрейд не идёт |
| те же `!…` вместе с `provides` | работает, но ничего не добавляет к `provides` + `replaces` |
| переходные пустые `steer-<модуль>-2.1.0` в фиде | работают только из фида (пакеты остаются стоять пустыми — чистка отдельным шагом); при закреплениях `world` не помогают («breaks: steer-core-2.1.0[…]» уже у самого ядра) |
| `provider_priority` | не потребовался: провайдер единственный |

Запасной путь (отложенное `steer plugin migrate --remove-old` после `postinst`) **не нужен и не делается**: решатель
снимает модули сам, а звать `apk del` изнутри скрипта нельзя — база заперта.

**Что ломало переход на стенде — закрепление `world`, а не метаданные.** `apk add ФАЙЛ` (apk 3) пишет в
`/etc/apk/world` не `steer-vless`, а `steer-vless><хеш файла`: закрепление за этим файлом. Такой пакет решатель
обязан оставить в базе, и никакие `provides` нового ядра его не заменят — ошибка
`breaks: world[steer-core><хеш>]`, одна и та же с любыми метаданными (проверены все варианты таблицы).
Поэтому:

- установщик splify2 перед `apk add` снимает закрепления (`pkg_world_heal`) — после этого работает `apk add
  steer-core.apk steer-tgws.apk` без репозитория;
- руками при закреплениях: назвать модули в той же команде — `apk add --allow-untrusted steer-core.apk
  steer-tgws.apk steer-vless steer-hysteria2 steer-proxy steer-xsteer steer-obfs steer-warpbox steer-extended` (имя без
  файла снимает закрепление транзакции), или перед установкой убрать закрепления (команда в
  `steer/docs/guide.md`, «Если после неудачной установки apk отказывает во всём»);
- установка ядра из фида (`apk upgrade`, `apk add -u steer-core`) закреплений не создаёт.

**`kmod-tun`.** Прежние модули тянули `kmod-tun`; при их снятии apk в той же транзакции снимает и осиротевший
`kmod-tun` (проверено: `Purging kmod-tun`). Менеджер плагинов поставить пакет из `postinst` не может (база
заперта), так что плагины vless, hysteria2, xsteer остались бы без TUN — а раздел 4 исходил из обратного.
Поэтому `steer-core` 2.1.0 **сам зависит от `kmod-tun`** (apk и opkg) — он остаётся стоять.

### 2.2 opkg

Одних `Provides`/`Replaces` opkg мало: после `opkg install steer-core.ipk` модули 2.0.5 остаются стоять с
нарушенной зависимостью от ядра (`steer-vless 2.0.5` при `steer-core 2.1.0`). Снимает их `Conflicts` вместе
с `Replaces` и `Provides` (так же `steer` → `steer-core`): «Removing package steer-vless …» перед
распаковкой ядра. Поэтому в `.ipk` ядра 2.1.0 `Conflicts` включает имена прежних модулей, а в `.apk` их нет
(там они бы ничего не добавили). Проверено в трёх видах: `opkg install steer-core.ipk steer-tgws.ipk`,
`opkg install steer-core.ipk`, `opkg upgrade` из фида — модули сняты во всех трёх; `preinst` ядра идёт
**до** снятия (журнал opkg: preinst, затем «Removing package»), так что бинарники модулей он ещё видит.
`kmod-tun` остаётся: opkg без `--autoremove` осиротевшее не снимает.

### 2.3 steer-tgws

Решение владельца: **мост Telegram — плагин `tgws` репозитория `splify2-extended-plugins`** («stgws теперь часть
splify2, у него появятся настройки»). Поэтому `steer-core` 2.1.0 заменяет и `steer-tgws`, как прочие пакеты модулей
(apk — `provides`/`replaces`, opkg — `Conflicts`): переход снимает его в той же транзакции. В семени ядра плагина
`tgws` нет — его ставят из репозитория `splify2-extended-plugins` (вкладка «Плагины» splify2 или
`steer plugin install tgws`); до этого выход `kind: tgws` недоступен с причиной «нужен плагин tgws», а `apply` с ним —
отказ. Исходник плагина — в дереве steer (`plugins/tgws`, `src/modules/main_tgws.c`, `src/proto/tgws`): программа
статическая, со своим TLS 1.3/1.2 и wolfSSL (`net1` не отдаёт TLS-соединение), собирает её `steer/build.sh`, архив
`.s2p` — `plugins/build.sh` репозитория `splify2-extended-plugins`. Прежние файлы моста (`/etc/steer/tgws-dc.conf`,
`tgws-domains.lst`, `tgws-route.conf`) плагин не читает: адреса ДЦ, пул доменов, пути ДЦ и туннели WARP — его
настройки (`conf/tgws/settings.json` по `settings.schema.json`). Перенос из микропакета stgws (`/etc/stgws`) делает
`scripts.postinst` плагина — раздел 6а.

## 3. Порядок установки `steer-core` 2.1.0

### 3.1 `pre-upgrade` (apk) / `preinst upgrade` (opkg)

Запомнить, какие модули стояли пакетами: `/etc/steer/plugins/migrate.json`,
`{"packages":["steer-vless","steer-warpbox","steer-obfs"]}` (`steer/build.sh`, `steer.preinst`; пишется только
когда есть что записать). Где искать след, проверено на менеджерах:

- **apk**: решатель снимает модули («Purging») **раньше**, чем зовёт `pre-upgrade` нового ядра, так что бинарников
  уже нет. След остался в `/etc/apk/world`: `steer-vless` и прочие имена стоят там дальше (их закрывает `provides`
  ядра). Скрипт читает `world`;
- **opkg**: `preinst` ядра идёт **до** «Removing package …», бинарники `/usr/sbin/steer-<m>` на месте. Скрипт
  проверяет их наличие (в `world` у opkg нет аналога).

Имена в `world` остаются навсегда, ими нельзя решать при каждом обновлении: человек мог снять плагин. Поэтому
после перехода `migrate` переименовывает файл в `migrate.done.json`, а `preinst` при его наличии ничего не пишет.
`steer-tgws` в список не входит (остаётся пакетом). При первой установке прежнего `steer-core` нет — файла нет.

### 3.2 `post-install` / `post-upgrade` (apk), `postinst` (opkg)

apk при обновлении зовёт `post-upgrade`, а не `post-install` (`steer/build.sh:555-561`) — тело у них одно.

1. `steer plugin migrate` — **сам менеджер, без демона** (у коннектора служба `steer` выключена:
   `steer/build/scripts/steer.postinst:4-8`).
2. Затем — как сейчас: `fw4 reload`, `enable` и `restart` службы `steer` (`steer.postinst:3, 10-11`), или
   у коннектора — перезапуск `sing-box`.

### 3.3 `steer plugin migrate`

1. **Какие плагины нужны.** Объединение трёх списков:
   - **по спеке:** читается спека человека (`/etc/steer/spec.yaml` или `spec.json`), собираются виды
     выходов (`kind`, у туннеля — `tunnel:<protocol>`), для каждого — плагин по `provides.kinds`
     индекса (из сети) или индекса семени (без сети). `warp` → `warpbox` (он тянет `awg`), `awg` →
     `awg`, `interface` с `obfs` → `obfs`;
   - **что стояло пакетами:** `migrate.json` по таблице раздела 1;
   - **по умолчанию:** при первой установке (нет `migrate.json`) — все `default: true` индекса (протоколы).
   Библиотеки (`net1`) и `awg` добавляются по `requires.plugins`.
2. **Откуда.** Есть сеть (индекс официального репозитория скачался и прошёл подпись) — обычный цикл
   установки ([s2p.md](s2p.md)): самая новая совместимая версия. Сети нет, или скачивание конкретного
   плагина не удалось — **из семени** `/usr/share/steer/seed/`: подписанные архивы той же версии, что
   индекс семени. Проверка места — как всегда, с отказом числами.
3. **Спека v1.** Плагинные виды есть только в спеке v2 ([spec-fields.md](spec-fields.md), 7). Если
   сохранена спека v1 (JSON до 2.0.0) с видами протоколов, `migrate` переводит её в v2 (`steer spec
   convert`), прежнюю кладёт в `spec.json.v1` и пишет об этом в `diag`.
4. **Ресурсы и ключи.** Выдача ресурсов в `alloc.json`; порт слушателя TCP туннелей и порт моста tgws
   остаются прежними — они выводятся из места метки выхода (`out_tcp_port`, `out_tgws_port`,
   `steer/src/model/marks.h:303-317`).
5. **Итог** — событие `plugin` с `by: "migrate"` на каждый плагин, строка `diag` «поставлено: vless 2.1.1
   (сеть), warpbox 1.3.0 (семя); не удалось: — », `migrate.json` получает `"done"`. Неудача одного плагина
   не останавливает остальные.
6. Прежние бинарники `/usr/sbin/steer-<m>` принадлежат пакетам и уходят вместе с ними; ядро 2.1 их не
   ищет (таблицы `CMD_MOD` больше нет).

**Что сделано в 2.1.0 (`steer plugin migrate`, `src/pluginmgr/pmmain.c`).** Список плагинов — объединение
(а) видов выходов спеки (`spec.yaml` и `spec.json`; `tunnel` — по `protocol`, остальные — по имени вида) через
`provides.kinds` индекса **семени**, а для видов без записи в индексе — `warp` → `warpbox`, `awg` → `awg`,
`interface` с ключом `obfs` → `obfs`; (б) `migrate.json`; (в) при отсутствии и спеки, и `migrate.json` — все
`default: true` семени. Каждый плагин: `ensure` (сеть, не вышло — семя); неудача одного остальные не останавливает,
итог — строки «поставлен …» и «не удалось: …», код 1 при любой неудаче (тогда `migrate.json` остаётся для повтора).
Зависимости — `requires.plugins` (`net1`, у `warpbox` — `awg`). Работает без демона. **Ещё нет:** перевод спеки v1
(п. 3), событие `plugin` с `by: "migrate"`, строка `diag`, повтор при подъёме WAN (п. 3.4) и `migrate.json` с
версией «from».

### 3.4 Пока плагин не встал

Решение В5: вид без плагина при старте и `reload` — выход в `on_fail`, остальное работает. Между
установкой `steer-core` и концом `migrate` демон не перезапускается (перезапуск — после `migrate`), так
что окна «спека с неизвестными видами» в обычном пути нет. Если `migrate` не поставил плагин (нет места,
битое семя), его выходы стоят в `on_fail` с причиной «нужен плагин vless — ставится», а менеджер
повторяет установку при подъёме WAN и раз в 10 минут с растущей паузой до часа.

## 4. Без сети и с сетью

| | С сетью | Без сети |
|---|---|---|
| Индекс | `splify2/plugins`, подпись ключом `index` | индекс семени, та же подпись |
| Версии | самые новые совместимые | версии семени (выпуск протоколов на момент выпуска ядра) |
| Системные пакеты (`kmod-tun`, `kmod-amneziawg`…) | `apk add` / `opkg install` из фидов | **уже стоят**: `kmod-tun` держит `steer-core` 2.1.0 (раздел 2.1; без этой зависимости apk снял бы его вместе с модулями). При первой установке без сети — плагин ставится, но не включается (`pending`, причина «нужен пакет kmod-tun — нет сети»), и менеджер довершает при появлении сети |
| После | — | первая связь — обычное фоновое обновление из сети |

## 5. Семя

- Состав: все протоколы `default: true` (`vless`, `hysteria2`, `proxy`, `xsteer`, `obfs`, `tgws`,
  `warpbox`), их библиотека `net1`, `awg`, а также `tgws@lite` — на случай нехватки места под `full`; `index.json` и `index.json.sig` — снимок индекса на момент
  выпуска; всё — под архитектуру пакета.
- Размер: с переездом стека в `net1` семя больше прежней оценки PLUGINS.md 7.4 (≈0,57 МБ без сжатия):
  теперь в него входит и стек с TLS/QUIC и копией wolfSSL — порядка 1,1 МБ libsteer и 0,67 МБ wolfSSL на
  x86-64 сегодня (`steer/build/libs/x86_64/`), в gzip заметно меньше. Точное число — после сборки `net1`;
  решение «не экономить флеш» (CLAUDE.md хаба) такую цену допускает.
- Собирает семя конвейер выпуска steer из тех же `.s2p`, что публикуются в индекс (PLUGINS.md 7.5).

## 6. splify2 и коннектор

- **splify2 26.11** зависит от `steer-core >= 2.1.0`; `m-engine.sh` (`STEER_MODULES`, `spec_mods`,
  `steer_module_add/del`, `m-engine.sh:388-459, 849-943`) заменяется вызовами `steer plugin … --json`;
  `ModulesCard` (`ui/src/components/ModulesCard.tsx:17-80`) — разделом «Плагины». Своё семя splify2 —
  плагин `lang-en` (`/usr/share/splify2/seed/`), `postinst` ставит его `steer plugin install --file`.
- **splify2 26.10 на ядре 2.1** не годится: он ставит пакеты `steer-<m>`, которых больше нет. Поэтому
  `steer-core` 2.1.0 объявляет `conflicts: splify2<26.11` (apk `!splify2<26.11`, opkg `Conflicts:
  splify2 (<< 26.11)`), а обычный путь — «Обновить» в splify2 26.10, который ставит splify2 26.11 и
  ядро вместе. Проверить на стенде. Довод — [open.md](open.md), О-15.
- **Коннектор 2.1.0 — плагин ядра `box-connector`** (решение владельца: «steer-box-connector тоже плагин ядра,
  пакетов остаётся два»). podkop и forkop — апстрим, их код не меняется; совместимость — со стороны коннектора:
  - **программа** `bin/sing-box` — статическая, со своей копией TLS 1.3, REALITY, HTTP/2, корней, разбора `.srs` и
    wolfSSL из дерева steer (`steer/build/ext-build.sh --static`, `EXT_TLS_SRC`), как плагин `tgws`: связь с
    `libsteer.so.<версия>` привязала бы плагин к одной версии ядра, а тега «=ядро» нет (В10);
  - **пути, которые ищут podkop и forkop** (`/usr/bin/sing-box`, `/etc/init.d/sing-box`, `/etc/config/sing-box`),
    страница LuCI коннектора и правило fw4 — `steer.links` ([steer-api-1.3.md](steer-api-1.3.md)): ссылки в каталог
    плагина, настройки — копией (uci правит файл);
  - **имя `sing-box` в базе пакетов.** podkop зависит от пакета `sing-box` (podkop 0.7.24: `Depends: libc, sing-box,
    curl, jq, kmod-nft-tproxy, coreutils-base64, bind-dig`); прежде имя давал пакет коннектора (`provides`). Взять
    его `steer-core` не может: **apk 3 считает provides имени его владельцем** — с `provides: sing-box` (с версией и
    без, с `provider_priority` и без) установка ядра снимает настоящий sing-box у каждого, кто его ставил, и не
    даёт поставить его рядом («conflicts: steer-core[sing-box]»; проверено на apk 3.0.8,
    `steer-box-connector/tests/standin.sh`, «почему не ядро»). Этого нельзя: чужой пакет — чужая настройка, а
    `steer-core` стоит у всех пользователей splify2. Поэтому имя держит **пакет `steer-box-connector` без файлов**
    (`noarch` / `all`): `provides: sing-box=<версия>`, `!sing-box !sing-box-tiny !sing-box-extended` (opkg —
    `Conflicts`, `Replaces`, `Provides`), зависимость `steer-core >= 2.1.0`. Его скрипт ставит плагин:
    `steer plugin ensure box-connector --pkg steer-box-connector` (изнутри транзакции пакета в базе ещё нет —
    [steer-api-1.3.md](steer-api-1.3.md)), снятие пакета снимает и плагин. Плагин требует его
    (`requires.system`), так что `steer plugin install box-connector` без пакета отказывает, и путь
    `/usr/bin/sing-box` без имени `sing-box` в базе не остаётся;
  - **переход с 2.0.x** — тот же пакет `steer-box-connector` новой версии: `apk upgrade` или `apk add` файлов ядра
    и пакета одной командой; прежний бинарник уходит с прежним пакетом раньше, чем скрипт нового ставит плагин.
    Иначе не выходит: в транзакции, где снимается пакет 2.0.x, имя `sing-box` кто-то должен держать, а `steer-core`
    не может (выше). Одно ядро без пакета apk не ставит (`breaks: steer-box-connector-2.0.x[steer-core=2.0.x]`) —
    podkop не остаётся без sing-box молча;
  - свой установщик модулей (`files/usr/libexec/rpcd/steer-box:110-330`) заменён `steer plugin ensure … --job`;
    перечень нужного даёт перевод конфига (`sing-box box-needs` печатает id плагинов `vless`, `hysteria2`, `proxy`).
  Проверено на настоящих менеджерах (`steer-box-connector/tests/standin.sh`): apk 3 — замена sing-box из фида,
  `apk add podkop` и `apk upgrade` при стоящем плагине не тянут настоящий sing-box, переход 2.0.5 → 2.1 файлами и
  из фида, обратный путь `apk add sing-box '!steer-box-connector'`; opkg 24.10 — метаданные пакета и плагин из
  файла. На стенде QEMU с podkop и forkop — ещё нет.

## 6а. stgws — микропакет моста Telegram

Решение владельца (2026-10-25): **stgws становится плагином `tgws` и обновляется в фоне**, как прочие
протоколы. Сейчас его не обновить: это отдельный пакет из `/root/brb` (форк Zapret-Manager) со своим
бинарником мини-сборки, а у его проверки обновлений — «вечное доступно обновление» (`TELEGRAM_REVIEW.md`
хаба, раздел про доставку). Код и упаковку brb этот заход не читал (каталог недоступен); всё, что ниже
сказано о пакете stgws, сверить с `/root/brb` при работе.

Что известно из дерева steer:

- бинарник `stgws` — профиль `tgws` ядра: перехват Telegram и мост, без туннелей и резолвера
  (`steer/src/profile/tgws.c:1-33`, сборка — `steer/build.sh:350-366`);
- он ставится и рядом с полным ядром, поэтому у него всё своё: бит метки 28 вместо поля `0x0ff00000`,
  таблицы с 316, свой файл имён таблиц `stgws.conf`, порты моста с 8490 (`tgws.c:24-33`;
  `steer/src/model/marks.h:49-61`).

**Как сделано (steer 2.1, ветки `tgws-plugin` и `tgws-2`).** Вариантов `full`/`lite` и мини-ядра `standalone` у
плагина нет: мини-сборка `stgws` (профиль `tgws`) из дерева steer снята, плагин один — статическая программа под
супервизором `steerd`, так что на роутере нужен `steer-core`. Поля профиля мини-сборки сняты вместе с ней: бит метки
28 ядро больше не бережёт (плагинам он выдаётся), ряд таблиц идёт с 300 подряд (выход, которому прежняя раздача дала
317 и дальше, держит таблицу из реестра), раздачи мест сверху (`STEER_MARK_ORDER=top`) нет.

**Переход со stgws** — `scripts.postinst` плагина (`steer/plugins/tgws/scripts/postinst`), при первой установке
плагина, если есть `/etc/stgws`:

| Файл stgws | Что | Куда |
|---|---|---|
| `tgws-domains.lst` | пул доменов | `domains` (первое слово строки) |
| `domain` | домен по умолчанию | первым в `domains` |
| `tgws-dc.conf` | дополнение таблицы ДЦ | `dc_addresses` |
| `warp.lst` | туннели WARP | `warp` |
| `route.conf` | карта прежнего подбора | не переносится: её заменяет настройка `pick` (мост мерит пути сам, в фоне, раз в 6 ч, карта подменяется без перезапуска) |

Строки, не подходящие под схему настроек, пропускаются и называются в выводе; уже записанные настройки плагина не
трогаются. Служба микропакета (`/etc/init.d/tgws`) останавливается и снимается с автозапуска — два моста
перехватывали бы одни и те же соединения; сам пакет остаётся (его ставил человек), снимает его человек
(`apk del tgws` / `opkg remove tgws`). Выход `kind: tgws` в спеке ядра плагин не заводит: спеку пишет панель
(домен выхода — домен по умолчанию stgws, он же первый в пуле). Переходного выпуска пакета stgws (правка brb) нет:
каталога `/root/brb` на машине разработки нет, формат его файлов взят из `tech/04-stgws.md` хаба.

## 7. Откат на 2.0.5

`apk add steer-core=2.0.5 steer-vless=2.0.5 …` (или opkg с файлами) — обычная установка прежних
пакетов. Каталог `/usr/lib/steer/plugins` и `/etc/steer/plugins` ядро 2.0.5 не читает: они лежат без
дела. Спека v2 с протоколами годится и для 2.0.5 (формат спеки тот же). Спека, переведённая из v1,
остаётся v2 — 2.0.5 её читает.

## 8. Стенды перехода (QEMU, OpenWrt 25.12.5 x86-64 и 24.10 для opkg)

1. 2.0.5 с `steer-extended` и выходами vless, xsteer, tgws → 2.1.0 с сетью: модули сняты, плагины стоят,
   выходы подняты, `diag` чистый.
2. То же без сети (WAN отключён): плагины из семени, затем подключение WAN — фоновое обновление.
3. 2.0.5 с `steer-warpbox` и `kind: warp` → 2.1.0: группа развёрнута хуком плагина, члены на месте,
   регистрации `/etc/steer/warp/` целы.
4. Первая установка 2.1.0 на чистый роутер без сети: протоколы из семени, `kmod-tun` нет — `pending`.
5. Нет места: отказ числами, выходы в `on_fail`, остальное работает.
6. Коннектор: podkop с vless + `steer-core` 2.1.0 + пакет `steer-box-connector` 2.1.0 + плагин `box-connector`.
7. splify2 26.10 → «Обновить» → splify2 26.11 + ядро 2.1.0 одной транзакцией.
8. Откат на 2.0.5.
9. stgws рядом с `steer-core` 2.0.5 → 2.1.0: настройки перенесены, `tgws` встал, `stgws` снят, Telegram
   идёт в мост, бит 28 и таблица 316 свободны.
10. stgws один на образе 4/32 (`mipsel_24kc`) → переходный пакет: `tgws@lite` встал, init работает, фоновое
    обновление и откат по здоровью проверены подменой индекса на стенде.
