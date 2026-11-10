# splify2-api 1.3 — что добавлено к 1.2

splify2 26.11 говорит splify2-api **1.3**. Прибавка к [splify2-api.md](splify2-api.md), [splify2-api-1.1.md](splify2-api-1.1.md)
и [splify2-api-1.2.md](splify2-api-1.2.md): плагин, написанный по 1.0–1.2, работает без изменений. Плагину, которому
нужно новое, пишут `compat.splify2-api: "^1.3"`.

Нужда: выход `kind: awg` — туннель WireGuard или AmneziaWG, который ядро steer создаёт само по файлу
`/etc/steer/awg/<имя>.conf` (плагин ядра `awg`). Файл попадает на роутер через панель, и ей нужны методы, которые
его проверят, положат и уберут.

## Методы `awg_list`, `awg_put`, `awg_del`

Три строки `methods.tsv`, все вида `frame`: плагин вправе звать их, обернуть или заменить (`methods.override`), имя
занять нельзя.

| Метод | Доступ | Параметры | Что делает |
|---|---|---|---|
| `awg_list` | чтение | — | перечисляет файлы туннелей в `/etc/steer/awg/` |
| `awg_put` | запись | `name`, `text` | проверяет конфигурацию и кладёт файл `/etc/steer/awg/<name>.conf` с правами 0600 |
| `awg_del` | запись | `name` | убирает файл туннеля |

### `awg_list`

```jsonc
{ "ok": true, "plugin_ready": true, "dir": "/etc/steer/awg",
  "tunnels": [
    { "name": "home", "conf": "/etc/steer/awg/home.conf", "endpoint": "198.51.100.7:51820",
      "addresses": ["10.8.0.2/32"], "amnezia": false, "ipv6": "off", "outputs": ["home"] }
  ] }
```

- `plugin_ready` — плагин ядра `awg` включён: без него выход `kind: awg` не применится.
- `amnezia` — в файле есть параметры AmneziaWG; `ipv6` — `"nat"` или `"off"`.
- `outputs` — имена выходов спеки, которые используют файл.

### `awg_put`

Параметры: `name` — имя будущего выхода (буквы, цифры, `_`, `-`, `.`, до 31 байта, не с точки); `text` — конфигурация
в формате wg-quick или awg-quick.

```jsonc
{ "ok": true, "name": "home", "conf": "/etc/steer/awg/home.conf", "endpoint": "198.51.100.7:51820",
  "addresses": ["10.8.0.2/32"], "amnezia": false, "ipv6": "off",
  "checked": true, "plugin": "awg", "plugin_ready": true,
  "output": { "kind": "awg", "on_fail": "drop", "ipv6": "off" } }
```

- Метод проверяет форму конфигурации и, если плагин `awg` включён, просит ядро проверить файл (`steer apply --dry-run`
  над спекой с одним выходом). `checked` — проверка ядром была; без плагина (`plugin_ready: false`) файл кладётся по
  проверке формы, `checked: false`.
- Закрытый ключ не возвращается и не пишется в журнал.
- **Спеку метод не пишет.** Выход добавляет тот, кто звал: объект `output` ответа кладётся в спеку под именем `name`
  через `spec_set`. У спеки есть ревизия и черновик панели — метод, который пишет спеку мимо черновика, разошёлся бы
  с ним.

### `awg_del`

Параметр `name`. Убирает файл. Пока файл использует выход сохранённой или применённой спеки, метод отказывает с кодом
`awg.in_use` и называет эти выходы: сначала выход убирают из спеки (`spec_set`) и применяют её.

## `xsteer_link_put` создаёт интерфейс сам

Строка `methods.tsv` не менялась, изменилось поведение. Если интерфейса `<iface>` нет, метод сам создаёт раздел
`network.<iface>` с `proto=xsteer` и добавляет в ответ `created: true` — раньше он отказывал с кодом
`xs.not_xsteer_create`. Интерфейс, который есть и у которого другой `proto`, метод по-прежнему не трогает: отказ
`xs.not_xsteer`.

## Версии

- `PLUGIN_SPLIFY2_API` хоста — `1.3.0`.
- Строка `tests/compat/matrix.json`: SDK 1.6.0 — splify2 26.11.0 — splify2-api 1.3.0, host-api 1.0.0, steer-api 1.5.0
  (заглушка `splify2-1.3`).
- `s2p lint` сверяет `compat.splify2-api` с 1.3.0.
