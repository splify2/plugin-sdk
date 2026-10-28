# steer-api v1: ядро steer для плагинов

Версия **1.0**. Тег совместимости — `compat.steer-api` ([manifest.md](manifest.md)). Ядро сообщает свою
версию API в `steer version --json` (`"api":"1.0"`) и в переменной `STEER_API` каждой программе
плагина. Правило версий: **minor добавляет** (новое событие, поле, подстановку, слот, команду), **major
ломает**. Плагин `^1.x` исполняется ядром с API `1.y`, если `y ≥ x`.

Что входит в steer-api:

1. как ядро запускает программы плагина: argv, окружение, подстановки, файл `{conf}`, коды выхода;
2. линия событий evline от помощника к ядру и `hello` с версиями;
3. хуки спеки: `check`, `expand`, `pre_apply`;
4. события сокета и команды сокета для плагинов;
5. карта nft: базовые цепочки ядра, слоты, формат файлов nft плагина;
6. ресурсы;
7. регистрация видов, подкоманд, служб и хуков из манифеста.

`.so` в процессе `steerd` — отдельный ABI, [steer-abi.md](steer-abi.md). Адреса `файл:строка` — steer `9f0e1fd`.

## 1. Программы плагина

### 1.1 Виды запуска

| Что | Кто запускает | argv | stdin / stdout | Срок |
|---|---|---|---|---|
| помощник выхода (`kinds[].helper`) | супервизор `steerd` (`--supervise`), на каждый выход вида | `argv` манифеста; умолчание `[<cmd>, "{out}", "--conf", "{conf}"]` | — / — (журнал — stderr) | живёт, пока выход в спеке |
| служба (`services[]`) | тот же супервизор, одна на роутер | `argv` манифеста | — / — | живёт, пока плагин включён |
| подкоманда (`commands[]`) | `steer <name> …` (exec вместо себя) или сокет «ребёнком» | `argv` манифеста, затем слова человека | терминал / вывод команды | `ctl.timeout_s` у сокета |
| `kinds[].check` | шаг подготовки спеки | `argv` манифеста | выход JSON / — | `timeout_s`, умолчание 10 с |
| `kinds[].expand` | шаг подготовки спеки | `argv` манифеста | выход JSON / фрагмент JSON | `timeout_s`, умолчание 10 с |
| `hooks.pre_apply` | шаг подготовки спеки | `argv` манифеста | спека JSON / спека JSON | `timeout_s`, умолчание 30 с |
| `hooks.event` | демон, на событие из списка | `argv` манифеста | строка события / — | `timeout_s`, умолчание 30 с |
| `scripts.*` | менеджер | `[<версия>]` | — / — | 120 с |

`argv[0]` — полный путь программы: `/usr/lib/steer/plugins/<id>/<prog>`. Супервизор сейчас ставит
`argv[0]` помощника-модуля в `<каталог>/steer` (`helper_argv0`, `steer/src/daemon/helpers.c:312-319`),
чтобы процесс выглядел как `steer vless out`; у плагинов `argv[0]` — настоящий путь, а имя в списке
процессов — `<prog> <cmd> <выход>`.

Как сегодня: помощник-подкоманда получает `<cmd> <выход> --spec <путь> [--state-dir <каталог>]`
(`helper_argv`, `steer/src/daemon/helpers.c:266-321`), помощник-программа — `prog` и два слова `arg[]`
фиксированной длины (`struct kind_helper`, `steer/src/kinds/kind.h:96-110`; `struct helper`,
`steer/src/daemon/helpers.h:83-119`). Для плагинов **argv и окружение — любой длины**: фиксированные
`cmd[16]`, `prog[16]`, `arg[2][256]`, `env[32]` — техдолг по правилам хаба, ядро 2.1 их снимает.

### 1.2 Окружение

Каждой программе плагина:

| Переменная | Значение |
|---|---|
| `STEER_API` | версия API ядра, `1.0` |
| `STEER_CORE_VERSION` | версия ядра, `2.1.0` |
| `STEER_PLUGIN_ID`, `STEER_PLUGIN_VERSION` | свои id и версия |
| `STEER_PLUGIN_DIR` | `/usr/lib/steer/plugins/<id>` |
| `STEER_PLUGIN_CONF` | `/etc/steer/plugins/conf/<id>` (создан, пишется) |
| `STEER_PLUGIN_STATE` | `<state>/plugin/<id>` (tmpfs, создан, пишется) |
| `STEER_ETC_DIR`, `STEER_STATE_DIR` | каталоги ядра (`/etc/steer`, `/var/lib/steer` на OpenWrt, `steer/src/platform/openwrt.c:19-26`) |
| `STEER_CTL_SOCK` | путь сокета `steerd` (`/etc/steer/steer.sock`, `openwrt.c:59`) |
| `STEER_PLATFORM` | `openwrt`, `android`, `linux` |
| `STEER_ARCH` | пакетная архитектура |
| `LANG` | язык человека из настроек splify2 (`ru`, `en`), если известен |

Помощникам выходов и службам дополнительно:

| Переменная | Значение |
|---|---|
| `STEER_EVENT_FD` | `3` — конец записи трубы событий (как сейчас: `steer/src/daemon/supd.c:1214`) |
| `STEER_SUPD` | `<pid>:<время старта>:<каталог состояния>` — чей это помощник; по нему помощник проверяет, жив ли демон, а новый демон гасит сирот (`supd.c:70-84`) |
| `STEER_OUT` | имя выхода (у службы нет) |

И как сейчас: `PR_SET_PDEATHSIG(SIGTERM)` до `exec` (`child_arm`, `supd.c:648-652`), `SIGPIPE` по
умолчанию (у модулей ядра он выключен в `steer_module_main`, `steer/src/cli/modcmd.c:117-163` —
плагин решает сам). Переменные из `env` манифеста добавляются после этих и не могут их заменить.

### 1.3 Подстановки

В `argv` манифеста, в файлах nft и шаблонах nft вида:

| Подстановка | Где | Что |
|---|---|---|
| `{out}` | помощник, `check`, `expand`, nft вида | имя выхода |
| `{conf}` | помощник | путь файла настройки выхода `<state>/plugin/<id>/<выход>.json` (раздел 1.4) |
| `{plugin_dir}`, `{plugin_conf}`, `{plugin_state}` | везде | как переменные окружения |
| `{mark}` | везде | маска выделенных плагину бит метки (`resources.mark_bits`), `0x…`; `{mark0}`, `{mark1}`… — отдельные биты |
| `{queue0}`, `{queue1}`… | везде | выделенные номера NFQUEUE |
| `{port0}`, `{port1}`… | везде | выделенные плагину порты |
| `{out_mark}`, `{out_table}`, `{out_port0}`… | помощник, nft вида | метка выхода (значение поля steer), его таблица, порты выхода |
| `{steer_mask}` | nft | маска поля метки steer (`plat()->mark_mask`, `steer/src/model/marks.h:71`) |
| `{tcp_port}` | помощник, nft вида | порт слушателя TCP выхода с `tcp-redir` (`out_tcp_port`, `marks.h:314-317`) |

Неизвестная подстановка — отказ установки (`s2p lint` ловит раньше). `{{` и `}}` — буквальные скобки.

### 1.4 Файл настройки выхода `{conf}`

Помощник плагина **не читает спеку**. Сейчас помощник получает `--spec` и сам разбирает всю спеку
libsteer-ом (`load_nodes` → `load_spec`, `steer/src/proto/vless/vlmain.c:66-88`), выводит метку
сокетов из реестра (`underlay_setup`, `vlmain.c:98-105`). Плагин с libsteer не связан, поэтому всё,
что помощнику нужно знать о своём выходе, ядро кладёт в файл перед запуском:

```json
{
  "api": "1.0",
  "plugin": { "id": "vless", "version": "2.1.1" },
  "output": {
    "name": "nl",
    "kind": "tunnel:vless",
    "keys": { "subscription": "/etc/steer/sub/nl", "nodes": [2, 3], "transport": ["tcp"] },
    "device": "vr-nl",
    "mark": "0x00300000",
    "mark_mask": "0x0ff00000",
    "table": 302,
    "ipv6": true,
    "on_fail": "drop",
    "over": { "name": "wg0", "mark": "0x00100000" },
    "sock_mark": "0x00100000",
    "sock_mark_required": true,
    "tcp_port": 9482,
    "ports": [],
    "queues": []
  },
  "paths": {
    "etc": "/etc/steer", "state": "/var/lib/steer", "lists": "/etc/steer/lists",
    "plugin": "/usr/lib/steer/plugins/vless", "conf": "/etc/steer/plugins/conf/vless",
    "plugin_state": "/var/lib/steer/plugin/vless",
    "fakeip": "/var/lib/steer/fakeip.state"
  },
  "platform": "openwrt"
}
```

| Поле | Что | Откуда сейчас |
|---|---|---|
| `output.keys` | ключи вида после проверки схемы; пути `x-path` уже абсолютные; секреты — как есть (файл `0600`) | `struct out_keys` (`steer/src/model/spec.h:426-451`) |
| `device` | имя устройства, которое помощник поднимает (TUN) | `output.device` (`spec.h:502-505`) |
| `mark`, `mark_mask`, `table` | метка выхода и таблица | `output.mark`, `output.table` (`spec.h:536-537`), реестр (`steer/src/model/registry.c`) |
| `ipv6` | несёт ли выход IPv6 после `ipv6: off` и донора | `out_caps` и `v6_denied` (`spec.h:546-551`) |
| `over` | подложка: через какой выход идёт трафик самого туннеля; `null` — напрямую | `output.over` (`spec.h:517-520`) |
| `sock_mark`, `sock_mark_required` | метка, которую помощник ставит **своим** сокетам к узлам (`SO_MARK`); `required` — отказ `SO_MARK` при `over` есть отказ, а не «пойдём напрямую» | `out_underlay_mark` и `transport_set_sock_mark` (`vlmain.c:98-105`; `steer/src/proto/transport/trdial.c:209-212`) |
| `tcp_port` | порт слушателя TCP (вид с `tcp-redir`) | `out_tcp_port` (`marks.h:314-317`) |
| `ports`, `queues` | ресурсы выхода (`kinds[].resources`) | — |
| `paths.fakeip` | таблица fake-IP резолвера ядра, только чтение: строки `домен⇥поддельный IPv4⇥настоящий IPv4 или -[⇥настоящий IPv6]`, файл переписывается целиком (`rename`). По ней туннель узнаёт имя и настоящий адрес за поддельным | `<state>/fakeip.state` (`steer/src/tunnel/stack.c`, `fake_load`) |

- Файл пишется атомарно (временный + `rename`) до запуска помощника, права `0600`.
- **Подпись перезапуска** — хеш содержимого `{conf}`, версии плагина и версий плагинов-библиотек из его `requires.plugins`. Сменилась — помощник
  гасится и поднимается сразу, без паузы (как сейчас подпись `helper_sig`,
  `steer/src/daemon/helpers.c:49-68`, и сверка `helpers_merge`, `helpers.h:134-137`).
- Новые поля в `{conf}` добавляются с minor API; помощник пропускает неизвестные.

### 1.5 Коды выхода и вывод

- `0` — удача. `2` — отказ: причина для человека — последняя непустая строка stderr (как отказы
  подкоманд ядра: «код 2, причина в `stderr`», `steer/docs/ctl.md:558`). `3` — «не готово, повторить
  позже» (только хуки: ядро повторит на следующей подготовке). Прочие коды и сигналы — сбой
  плагина: «плагин vless: check упал (код 139)».
- stderr помощников и служб идёт в журнал ядра с приставкой `steer[<id>]`. Строка журнала — до
  4 КиБ, длиннее режется с пометкой.

## 2. evline: события помощника

Формат — как сейчас (`steer/src/lib/evline.h:6-23`): **одна строка JSON на событие**, `\n` в конце,
один `write()`. Строка не длиннее `PIPE_BUF` (4096 на Linux) — это предел атомарной записи в трубу, а не
выбранное число (`evline.h:6-9`). Объект плоский: значения — строки и целые. Труба `O_NONBLOCK`: если она
полна, событие теряется, помощник не ждёт. События пишутся только на смене состояния.

SDK даёт реализацию для программ (`libs2p-evline.a`, без зависимостей); писать строки можно и самому.

### 2.1 `hello`

**Первое** событие помощника и службы:

```json
{"ev":"hello","api":"1.0","ver":"2.1.1","mod":"vless"}
```

- `api` — версия steer-api, которой говорит программа (major.minor); `ver` — версия плагина; `mod` — id.
- Ядро принимает, если major `api` равен своему и minor не больше своего. Иначе — гасит процесс и
  пишет причину в `status` выхода и в журнал: «плагин vless говорит steer-api 2.0, ядро — 1.3».
- Первое сообщение не `hello`, или `hello` без `api` — отказ, как сейчас у модулей (`mod_hello` и
  ветка в `ev_line`, `steer/src/daemon/supd.c:323-340, 408-422`).
- **Сверка `ver` с версией ядра снимается** (сейчас — `strcmp(ver, steer_engine_version())`,
  `supd.c:332`). Для временной совместимости бинарников модулей 2.0.x ядро 2.1 не держит: модули
  сняты вместе с пакетами (см. [migration-2.1.md](migration-2.1.md)).
- Срок `hello` — `timeout_s` помощника/службы, умолчание 10 с. Не пришёл — процесс гасится, перезапуск
  по обычной паузе.

### 2.2 События

Нынешние события (`evline.h:24-87`) остаются с тем же смыслом:

| ev | Поля | Что делает ядро |
|---|---|---|
| `up` | `watch` (0/1), `dev` | выход готов; `dev` — устройство, к которому ядро привязывает маршрут выхода (`route_up`, `supd.c:244`); `watch:1` — помощник сам следит за узлом, его `up`/`down` и есть приговор здоровью |
| `down` | `why` | выход отказал; причина для человека |
| `node` | `n`, `total` | перебор узлов: проверяется узел `n` из `total` |
| `nonode` | `node`, `total` | выбранного узла нет в подписке |
| `excluded` | `total` | все кандидаты исключены |
| `active` | `nodes` (номера через запятую) | активные живые узлы пула |
| `health` | `dc`, `media`, `domain`, `cool` | путь отставлен (мост tgws) |
| `member` | `out`, `ok` (−1/0/1) | приговор члену группы |

Изменения v1:

- **`member` и `revive` — для выходов, которыми владеет помощник** (поле `owner`,
  [spec-fields.md](spec-fields.md)), а не только для членов группы `kind: warp` (сейчас проверка
  `o->warp_of` — `supd.c:350, 369`). Чужой выход — отказ строкой в журнал, как сейчас.
- **`revive`** `{out, fresh}` — новое имя события `awg` (`evline.h:75-81`, `ev_awg` в `supd.c:346-358`):
  «файл выхода `out` переписан, перенастрой устройство», `fresh:1` — с новой сессией. Ядро 2.1
  принимает оба имени; `awg` уйдёт в API 2.0.
- **`state`** `{}` — помощник переписал свой файл состояния `<state>/plugin/<id>/<выход>.status.json`
  (атомарно, `rename`). Ядро перечитывает его и вкладывает в `status` выхода как `outputs.<имя>.plugin`
  ([spec-fields.md](spec-fields.md)). Файл — JSON любой длины: строка события его не несёт. Это
  замена `warp_emit`, который читает `<state>/warp-<группа>/state` (`steer/src/daemon/status.c:143-183`).
- **`ready`** `{}` — служба готова (у службы нет выхода, `up` ей не по смыслу).
- **Число полей не ограничено.** Сейчас разбор берёт не больше шести полей и ключи до 15 знаков
  (`EVLINE_MAXFIELDS`, `EVLINE_KEYLEN`, `evline.h:126-127`), лишнее отбрасывает молча. Ядро 2.1 разбирает
  любое число полей в пределах строки; ключ — до длины строки.
- Неизвестное событие ядро пропускает (а не гасит помощника): плагин, написанный под API 1.4, на ядре
  1.3 не ставится, но поле или событие сверх известного не повод падать.

## 3. Хуки спеки

### 3.1 Шаг подготовки спеки

Хуки исполняются в одном месте — **шаге подготовки спеки** (`spec_prepare`). Его зовут:

- `check` и `apply` (ребёнок `apply-plan` сокета, `steer/src/daemon/ctl.c:1414-1430, 1612-1646`);
- `reload` и SIGHUP;
- старт демона (`--apply`, до цикла событий);
- `steer plugin` после установки, обновления, отключения.

Порядок внутри шага:

1. разбор спеки человека; выход вида плагина — по схеме ключей (`kinds[].keys`);
2. `kinds[].check` для каждого выхода вида плагина;
3. `kinds[].expand` для каждого выхода вида с разворотом;
4. `pre_apply` плагинов — в порядке зависимостей, затем по `id`; каждый видит результат предыдущего;
5. разбор и проверки ядра по итоговой спеке (как сейчас `load_spec`).

Итог — **подготовленная спека**: `/etc/steer/plugins/cache/spec.prepared.json` и ключ
`spec.prepared.key` = sha256(байты спеки человека, версии и `MANIFEST` всех включённых плагинов с
хуками). **`load_spec` хуков не исполняет**: спеку читают ещё десяток мест — `status`, `diag`,
`explain`, сторож, супервизор (`steer/src/daemon/state.c:52`, `status.c:537`, `failover.c:3378`,
`supervise.c:68` и другие), и fork/exec в каждом из них был бы и медленным, и разным по времени.
`load_spec` берёт подготовленную спеку, если ключ совпал; не совпал (спеку правили руками,
плагин сменился без подготовки) — читает спеку человека, а выходы видов с разворотом и все правки
`pre_apply` считает **неготовыми**: такие выходы уходят в `on_fail` с причиной «спека не подготовлена —
нужен reload», и демон сам заказывает `reload`.

`steer spec convert` и печать спеки (`v2print`) читают **спеку человека**, не подготовленную: выходы,
заведённые разворотом, в спеку человека не печатаются (как сейчас `warp_of`,
`steer/src/model/v2print.c:557`).

Хуки обязаны быть **детерминированными**: одинаковый вход — одинаковый выход. Сеть, время, случайность
— запрещены (у них свои места: помощник и служба). Ядро кэширует ответы хуков по sha256 входа в
`/etc/steer/plugins/cache/<id>/`.

### 3.2 `check`

stdin:

```json
{"api":"1.0","output":{"name":"nl","kind":"tunnel:vless","keys":{…}},
 "spec":{"etc":"/etc/steer","outputs":["nl","de","wg0"]},"platform":"openwrt"}
```

Код 0 — годится; 2 — отказ, причина — stderr, ядро печатает её после `outputs.<имя>: ` (как отказ вида,
`steer/src/kinds/kind.h:155-169`). stdout — необязательный JSON `{"warnings":["…"]}`: предупреждения
`apply` и `diag`.

### 3.3 `expand`

Разворот выхода в выходы видов ядра — обобщение `p_warp` (`steer/src/model/v2.c:1133-1265`), где
`kind: warp` превращается в группу и N членов `awg`/`warp-masque` с `warp_of`.

stdin — как у `check`. stdout:

```json
{
  "self": { "kind": "group", "pick": "latency", "members": ["w.1", "w.2"], "on_fail": "drop" },
  "outputs": {
    "w.1": { "kind": "awg", "conf": "/etc/steer/warp/w-1.conf", "ipv6": "nat" },
    "w.2": { "kind": "awg", "conf": "/etc/steer/warp/w-2.conf", "ipv6": "nat" }
  },
  "helper": true
}
```

- `self` — чем выход становится: запись выхода спеки v2 любого вида ядра или вида другого плагина
  (кроме вида с разворотом — разворот однократный). Имя остаётся прежним, правила спеки, ведущие в
  выход, не меняются.
- `outputs` — новые выходы. Имена — `<выход>.<что угодно>` (`[A-Za-z0-9_.-]`, не длиннее 31 знака —
  предел имени выхода в сокете, `ctl.md:57-58`). Каждый получает `owner = {plugin, out}`.
- `helper: true` — помощник вида (`kinds[].helper`) нужен и развёрнутому выходу: он ведёт членов
  (`member`, `revive`), как `steer-warpbox` ведёт группу `kind: warp` (`steer/src/kinds/group.c:359-379`).
- Ядро проверяет фрагмент тем же разбором v2, что спеку человека; ошибка во фрагменте — отказ с именем
  плагина: «плагин warpbox: разворот w — outputs.w.1: …».

### 3.4 `pre_apply`

stdin — подготовленная до этого места спека в JSON (форма v2, как `steer spec convert --json`), stdout —
спека целиком или код 2 с причиной. Плагин может всё (решение В6: правка спеки), но:

- выходы, правила, списки, которые добавил хук, получают `owner = {plugin}`; человек видит их в
  интерфейсе как «добавлено плагином X» и правит через плагин;
- ядро пишет в ответ `apply` и в `explain`, какой хук что изменил:
  `"plugins":{"pre_apply":[{"id":"x","added":["rules[4]"],"changed":["outputs.nl.on_fail"]}]}`;
- хук, вернувший спеку, которая не разбирается, — отказ `apply` с именем плагина; спека человека при
  этом не теряется (она уже лежит, хук правит только подготовленную).

### 3.5 Вид без плагина

Решение В5:

| Когда | Что |
|---|---|
| старт демона, `reload`, SIGHUP, подготовка после смены плагинов | выход неизвестного или отключённого вида разбирается как **недоступный**: своя метка и таблица, устройства нет, `failed`, его `on_fail`; в `status` — `unavail` с причиной; остальные выходы работают |
| `apply` и `check` из интерфейса или сокета | отказ всей спеки: «outputs.nl: нужен плагин vless — поставить» (плагин найден по `provides.kinds` индекса или семени) или «неизвестный kind foo» |

Сейчас отсутствующий модуль — отказ всей спеки при любом чтении (`if (kd->absent) return fail(…)`,
`steer/src/model/v2.c:761`), а тексты отказа зашиты в реестре (`steer/src/kinds/kind.c:47-61`).

## 4. Сокет `steerd`

### 4.1 События

Протокол — `steer/docs/ctl.md:483-606`: `subscribe`, строка JSON на событие, `"v":1`, неизвестные события
и поля пропускаются. Изменения v1:

- **Подписчиков сколько угодно.** Предел в восемь соединений снят (в ядре `src/daemon/ctl.c`: подписчик не занимает
  место среди запросов, `CTL_CLIENTS_MAX` его не считает); служба плагина, подписанная на события, — обычный
  потребитель. Очередь подписчика (32 КиБ, `CTL_SUBQ_MAX`) остаётся: она защищает демон от зависшего читателя, а не
  ограничивает число.
- Новое событие **`plugin`**:
  ```json
  {"v":1,"ev":"plugin","id":"vless","state":"enabled","version":"2.1.1","from":"2.1.0","why":null,"by":"auto"}
  ```
  `state`: `installed`, `enabled`, `disabled`, `failed`, `removed`, `updated`, `rolled-back`; `by`:
  `human`, `auto`, `migrate`, `core-upgrade`.
- `helper-up` и `helper-down` получают поле `plugin` (id), если помощник — плагин; `helper` у службы —
  `service:<имя>`.
- Новое событие **`plugin-state`** `{id, out}` — помощник прислал `state` (раздел 2.2): интерфейс
  перечитывает `status` выхода.

### 4.2 Команды сокета для менеджера

Все — изменяющие (в очереди с `apply`), кроме `plugin-list`. Ответ — как у остальных команд
(`ctl.md:68-96`), в `stdout` — JSON.

| Команда | Слова | Что |
|---|---|---|
| `plugin-list` | — | стоящие плагины, их состояние, причины; доступные обновления из кэша индекса |
| `plugin-search` | `[слово]` | каталог всех репозиториев |
| `plugin-install` | `<id> [версия]` | установка (ребёнок, срок 600 с) |
| `plugin-install-file` | тело — `.s2p` | установка из файла (до 64 МиБ) |
| `plugin-update` | `[id]` | обновление одного или всех |
| `plugin-remove` | `<id> [force] [purge]` | снятие |
| `plugin-enable`, `plugin-disable` | `<id>` | включить, отключить |
| `plugin-reload` | — | перечитать манифесты (реестр), затем подготовка спеки и `reload` |
| `plugin-repo-add` | `<url> <номер ключа>`, тело — открытый ключ | подключить репозиторий |
| `plugin-repo-del` | `<имя>` | отключить репозиторий |
| `plugin-plan` | `[core <версия>]` | план обновления ядра и плагинов без установки (PLUGINS.md 6.2) |
| `plugin-migrate` | — | переход на 2.1 ([migration-2.1.md](migration-2.1.md)) |

Клиент `steer plugin <действие> … [--json]` — те же действия через сокет, а без демона — сам.

### 4.3 Подкоманды плагинов через сокет

Подкоманда с `ctl` в манифесте получает строку в таблице сокета наравне с `vless-nodes` и `vless-probe`
(сейчас — постоянные строки `CTL_CMDS`, `steer/src/daemon/ctl.c:638-664`): слова проверяются по `args`
(`name`, `int`, `target`, `file`, `word` — те же правила, что `ctl.md:55-60`), ребёнок исполняет
`<plugin>/<prog> <argv…> <слова…>`, срок — `timeout_s`. Команда, которую плагин назвал так же, как
команда ядра, не регистрируется (отказ установки).

## 5. nft

### 5.1 Карта ядра

Ядро ставит одну таблицу `inet steer` (имя — `STEER_NFT_TABLE`, умолчание `steer`,
`steer/src/model/marks.h:228-241`) целиком, одной транзакцией. Её базовые цепочки (`ir_base_chain_add`):

| Цепочка | Тип, хук | Приоритет | Где | Что |
|---|---|---|---|---|
| `ingress_mark` | filter, ingress (netdev) | filter+10 | `steer/src/compile/generate.c:1058` | разметка на ingress устройств раздачи |
| `prerouting_raw` | filter, prerouting | raw+0 | `generate.c:1741` | traceroute для доменных каналов |
| `prerouting_mark` | filter, prerouting | mangle+1 | `generate.c:1071` | **разметка каналов** |
| `prerouting_failopen` | filter, prerouting | mangle+2 | `generate.c:1119` | пуск напрямую упавших выходов |
| `forward_v6` | filter, forward | mangle+0 | `generate.c:1284` | IPv6 по выходам |
| `prerouting_dns` | nat, prerouting | dstnat−2 | `generate.c:1529` | DNS в резолвер |
| `prerouting_tcp_redir` | nat, prerouting | dstnat−1 | `generate.c:2189` | TCP выходов `tcp-redir` к слушателю |
| `prerouting_dnat` | nat, prerouting | dstnat+0 | `generate.c:1667` | fake-IP и подмена порта |
| `tgws_redirect` | nat, prerouting | dstnat+1 | `steer/src/kinds/tgws.c:120` | мост tgws (уходит в плагин) |
| `prerouting_restore` | filter, prerouting | dstnat+10 | `generate.c:1146` | восстановление метки из conntrack |
| `output_mark` | route, output | mangle+1 | `generate.c:1902` | каналы самого устройства (только при `local_channels`) |
| `output_tcp_redir` | nat, output | dstnat−1 | `generate.c:2189` | то же для своего трафика |
| `output_dns` | nat, output | dstnat+0 | `generate.c:1820` | DNS самого устройства |
| `postrouting_guard`, `postrouting_fakeguard` | filter, postrouting | filter+0 | `generate.c:1427, 1635` | запреты утечек |
| `postrouting_nat4`, `postrouting_nat6` | nat, postrouting | srcnat+0 | `generate.c:2118, 2085` | masquerade выходов |
| `postrouting_down` | filter, postrouting | srcnat+10 | `generate.c:1185` | отказ трафику упавших выходов |
| `input_down` | filter, input | filter+10 | `generate.c:1979` | то же для входящего |
| виды `block` | filter, хук вида | filter−5 | `steer/src/kinds/block.c:21` | выход «Блок» |

`output_mark` есть только на платформе с каналами самого устройства: на роутере `local_channels = 0`
(`steer/src/platform/openwrt.c:90`), если не задан `STEER_ROUTER_SELF=1` (`steer/src/platform/platform.c:119-147`).

Карту отдаёт команда `steer nft-map --json` (новая): имя таблицы, цепочки с хуком и приоритетом
числом, поле метки steer, слоты. Плагин со **своей таблицей** (как recheck сейчас: `inet recheck`,
`recheck/integration/openwrt/root/usr/lib/recheck/recheck.sh:82-125`) встаёт «до разметки steer» или
«после» по этой карте, а не угадывая числа.

### 5.2 Слоты

Когда правилам плагина нужны метки steer и та же транзакция, что у набора steer, плагин кладёт их в
**слоты**. Слот — базовая цепочка ядра `plugin_<слот>` в таблице steer; ядро заводит её, только если
слот занят хоть одним плагином, и пишет в неё `jump plugin_<id>_<слот>` по плагинам в порядке
зависимостей, затем по `id`.

| Слот | Тип, хук | Приоритет | Место относительно цепочек ядра |
|---|---|---|---|
| `prerouting-raw` | filter, prerouting | raw+1 | сразу после `prerouting_raw` |
| `prerouting-pre` | filter, prerouting | mangle+0 | **до разметки** `prerouting_mark` (mangle+1): исключить трафик из разметки, поставить свою метку |
| `prerouting-post` | filter, prerouting | mangle+3 | **после разметки** и `prerouting_failopen` (mangle+2): действовать по метке выхода |
| `prerouting-dnat` | nat, prerouting | dstnat+1 | после fake-IP и подмены порта (dstnat+0); место `tgws_redirect` |
| `forward` | filter, forward | filter−5 | после `forward_v6` (mangle), до fw4 (filter+0) |
| `input` | filter, input | filter+0 | до `input_down` (filter+10) |
| `output-raw` | filter, output | raw+1 (−299) | **до conntrack** (−200): `notrack` на пакеты, которые плагин сам породил (с API 1.2, [steer-api-1.2.md](steer-api-1.2.md)) |
| `output-pre` | route, output | mangle+0 | до `output_mark` (mangle+1) |
| `output-post` | route, output | mangle+2 | после `output_mark` |
| `postrouting-pre` | filter, postrouting | filter−5 | до запретов утечек (filter+0) |
| `postrouting-post` | filter, postrouting | srcnat+20 | **после NAT** (srcnat+0) и `postrouting_down` (srcnat+10): очередь NFQUEUE по адресу после masquerade |

Приоритеты — постоянные числа API v1: новый слот добавляется с minor, перенос слота — major.

### 5.3 Файл nft плагина

`steer.nft[]` — файлы плагина, `kinds[].nft[]` — шаблоны на каждый выход вида. Формат — подмножество
синтаксиса nft:

```
set private4 {
  type ipv4_addr; flags interval
  elements = { 10.0.0.0/8, 192.168.0.0/16 }
}
chain postrouting-post {
  meta mark & {steer_mask} != 0 return
  iifname @lan_dev oifname @wan_dev ip daddr != @private4 counter queue flags bypass to {queue0}
}
```

- На верхнем уровне — только `set`, `map` и `chain <слот>`. Ядро переименовывает: набор `X` →
  `p_<id>_X` (и ссылки `@X` в правилах этого файла), `chain <слот>` → `plugin_<id>_<слот>` (обычная
  цепочка, без хука). Таблицу, хук, приоритет плагин не пишет.
- Шаблон вида (`kinds[].nft`) разворачивается на каждый выход вида с подстановками выхода; правила
  всех выходов одного вида ложатся в одну цепочку подряд, в порядке спеки (как `kind_emit_all`,
  `steer/src/kinds/kind.c:137-144`). Пример — мост tgws (`tgws_emit`, `steer/src/kinds/tgws.c:115-128`):
  ```
  chain prerouting-dnat {
    meta mark and {steer_mask} == {out_mark} tcp dport { 443, 80, 5222 } counter redirect to :{out_port0}
  }
  ```
- Наборы ядра, которые плагин может читать: `@lan_dev` — устройства раздачи спеки (`spec.lan_dev`,
  `steer/src/model/spec.h:831-832`), `@wan_dev` — устройства маршрутов по умолчанию таблицы `main`
  (новый набор: в ядре понятия WAN сейчас нет; ведёт его ядро по netlink, см. [open.md](open.md), О-9).
- Правила плагина вставляются в текст набора правил steer и проходят ту же загрузку `nft -f` одной
  транзакцией. Ошибка в файле плагина — отказ `apply` с именем плагина и строкой файла; спека человека
  не теряется. Отключённый или упавший плагин — его цепочки и `jump` не ставятся.

## 6. Ресурсы

Ядро выдаёт по `resources` манифеста и `kinds[].resources` и хранит выдачу в
`/etc/steer/plugins/alloc.json` (номера постоянны между перезагрузками и обновлениями плагина):

| Ресурс | Откуда | Что исключается |
|---|---|---|
| биты метки | биты 0–31 вне поля steer | поле steer (`marks.h:69-71`), бит мини-сборки tgws 28 (`marks.h:49-61`), mwan3 8–13 и Tailscale/pbr 16–23 (`STEER_MARK_NEIGHBOURS`, `marks.h:123`), уже выданные |
| очереди NFQUEUE | от 4300 вверх | занятые в наборе правил (`nft list ruleset`, разбор `queue … to N`), уже выданные |
| порты | от 10480 вверх | ряды ядра 8480… и 9480… (`steer/src/profile/profile.c:19-23`), занятые сокетами (`/proc/net/{tcp,udp}{,6}`), уже выданные |
| таблицы | ряд реестра выше выходов | `steer/src/model/registry.c:17-28` |

Пределов числа нет: ресурс кончается только вместе с номерами, и отказ называет, сколько всего и кем
занято («бит метки: свободных нет — 29–31 у recheck, 0–7 у foo»). Снятие плагина освобождает его
номера (кроме `--keep-alloc` при переустановке).

## 7. Регистрация из манифеста

При старте `steer`/`steerd` и по `plugin-reload` ядро читает `/usr/lib/steer/plugins/*/plugin.json`
включённых плагинов (`state.json`) и строит реестр. Ядро **само сверяет теги** (`compat`): несовместимый
плагин, даже положенный руками, не регистрируется, причина — в `steer plugin status` и событии `plugin`.

| Из манифеста | Во что | Чем заменяет сегодняшнее |
|---|---|---|
| `steer.commands[]` | таблица «подкоманда → (плагин, prog, argv)»; запуск — `execv` | `CMD_MOD` (`steer/src/lib/module.c:26-51`), слабые `cmd_*` и `modcmd_builtin` (`steer/src/cli/modcmd.c:48-92`), `absent` с именами пакетов (`modcmd.c:96-116`) |
| `steer.kinds[]` | записи реестра видов: общий вид `plugin-kind` (схема ключей, caps, помощник, хуки, nft) или допуск вида ядра (`impl: core`) | реестр на слабых ссылках и записи отказа (`steer/src/kinds/kind.c:29-82`), `module_ok` (`kind.c:100-109`), перечень видов в тексте отказа (`steer/src/model/v2.c:757-759`) |
| `kinds[].helper`, `services[]` | состав помощников супервизора | `kind_ops.helper` (`kind.h:207-209`), `steer_cmd_module` в `helpers_plan` (`steer/src/daemon/helpers.c:96-106`) |
| `commands[].ctl` | строки таблицы сокета | `CTL_CMDS` (`ctl.c:638-664`) |
| `steer.nft`, `kinds[].nft` | цепочки слотов и наборы | `kind_ops.emit` вида tgws (`tgws.c:115-128`) |
| `steer.hooks` | шаг подготовки спеки, события | `p_warp` (`v2.c:1133-1265`) |

Порядок реестра видов: сначала виды ядра в прежнем порядке (`kind.c:65-82` — от него зависит порядок
правил видов и проверок `diag`), затем виды плагинов по `id` плагина и порядку в манифесте. Порядок
`kind_emit_all` (`kind.c:137-144`) тем самым не зависит от порядка установки.

Конфликты имён (вид, подкоманда, служба, `id`) проверяет менеджер при установке; если они всё же
случились (плагины положили руками), регистрируется плагин, стоявший раньше, второй — `failed` с
причиной.
