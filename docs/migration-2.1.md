# Переход на steer-core 2.1.0: три пакета и плагины

Решения 7–9, В4, В5, В10: пакетов apk/ipk остаётся три — `steer-core`, `steer-box-connector`, `splify2`;
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
| `steer-box-connector` | sing-box на libsteer, свой установщик модулей (`steer-box-connector/files/usr/libexec/rpcd/steer-box`) | пересобран под 2.1.0 (связан с `libsteer.so.<версия>` — зависимость `steer-core` той же версии, `steer-box-connector/build.sh:69, 87`); модули ставит `steer plugin ensure` |
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
настройки (`conf/tgws/settings.json` по `settings.schema.json`); переноса нет (`scripts.migrate` у плагина нет).

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
- **Коннектор 2.1.0**: зависит от `steer-core` той же версии (как сейчас); свой установщик модулей
  (`files/usr/libexec/rpcd/steer-box:110-330`) заменяется `steer plugin ensure vless hysteria2 proxy`;
  перечень нужного даёт его перевод конфига (`steer-box-connector/src/translate.c:1966-1968` — сейчас
  печатает имена пакетов `steer-vless`…, станет — имена плагинов).

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

**Как сделано (steer 2.1, ветка `tgws-plugin`).** Вариантов `full`/`lite` и мини-ядра `standalone` у плагина нет:
мини-сборка `stgws` (профиль `tgws`) из дерева steer снята, плагин один — статическая программа под супервизором
`steerd`, так что на роутере нужен `steer-core`. Бит 28 и таблица 316 мини-сборки плагинам не выдаются, пока
прежний пакет `stgws` может стоять рядом. Переходного выпуска пакета `stgws` (правка brb) нет: код brb этот заход
не видел (каталога `/root/brb` на машине разработки нет). Что осталось — в отчёте хаба: переходный пакет stgws,
перенос его настроек в `conf/tgws/settings.json`, подбор путей ДЦ (`tgws pick` из brb) как задача плагина.

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
6. Коннектор: podkop с vless + `steer-core` 2.1.0 + коннектор 2.1.0.
7. splify2 26.10 → «Обновить» → splify2 26.11 + ядро 2.1.0 одной транзакцией.
8. Откат на 2.0.5.
9. stgws рядом с `steer-core` 2.0.5 → 2.1.0: настройки перенесены, `tgws` встал, `stgws` снят, Telegram
   идёт в мост, бит 28 и таблица 316 свободны.
10. stgws один на образе 4/32 (`mipsel_24kc`) → переходный пакет: `tgws@lite` встал, init работает, фоновое
    обновление и откат по здоровью проверены подменой индекса на стенде.
