# host API v1: интерфейс плагина в панели splify2

Версия **1.0.0**. Тег совместимости — `compat.host-api`. Типы — npm-пакет `@splify2/plugin-api`
([`packages/plugin-api/src/index.d.ts`](../packages/plugin-api/src/index.d.ts)): версия пакета равна
версии host API. Кроме типов в нём константа `HOST_API_VERSION` и сборка на vite с общим preact
(`@splify2/plugin-api/vite`). Адреса `файл:строка` — splify2 `91ac6d2`.

Решение 2: **UI плагина работает в том же контексте, что панель; доверие — по подписи**. ES-модуль
грузится прямо в панель и получает объект `host`.

## Как панель устроена сейчас

| Что | Где | Что из этого следует |
|---|---|---|
| Загрузка под LuCI: `home.js` берёт `build-id.txt`, ставит `<link id="splify2-app-css…">` и `<script type=module src=splify-index.js?v>`, по `onload` зовёт `window.__splifyMount(el)`; `window.luci_rpc = rpc` | `luci/htdocs/luci-static/resources/view/splify2/home.js:13, 45-51, 68-80, 93-117` | плагины грузятся после монтирования, тем же путём `import()` |
| Панель монтируется в shadow root, стили — копией `<link>` в корень тени | `ui/src/main.tsx:98-123, 125-158` | CSS плагина в `document.head` не попадёт: только `host.styles` |
| Тёмный режим — по яркости фона LuCI, класс `.dark` на `<html>` | `ui/src/main.tsx:31-42, 146-149` | темы — переменные, а не свой режим |
| `react → preact/compat` | `ui/vite.config.ts:107-110` | один preact на всех, иначе хуки ломаются |
| Сборка — ES-модули, чанки по белому списку имён | `ui/vite.config.ts:113-170`, `ui/scripts/check-dist.mjs:18` | плагины грузятся мимо `check-dist` — это код вне сборки хоста |
| Разделы — закрытое объединение `SectionId`; Rail — `ITEMS` и `extra` | `ui/src/lib/sections.ts:5`, `ui/src/components/Rail.tsx:29-37, 52-74` | вкладка «Плагины» — новый встроенный раздел; плагины в Rail не попадают (исключение — `railSection`) |
| Единственная точка расширения — `ExtraSection {id,label,icon,render(live)}`, её использует только десктоп | `ui/src/components/Console.tsx:50-55, 107, 212, 270`; `desktop/renderer/main.tsx:48` | уходит: WarpBox и ReCheck живут в карточках вкладки «Плагины» |
| Переход между разделами — событие `splify:section` | `ui/src/components/Console.tsx:118-128` | `host.nav.open` |
| Экраны VPN и настроек — закрытые объединения | `ui/src/components/sections/Vpn.tsx:24`, `Settings.tsx:28` | плагины туда не встраиваются: их настройки — в карточке плагина |
| Плитки «Доски» собраны руками, `WarpTile` читает `st.group.warp` | `ui/src/components/sections/Home.tsx:1124-1146, 1311` | слот `boardTile` |
| Блок «Куда»: часть WARP вшита, `WarpPill` | `ui/src/components/board/RuleTarget.tsx:74-79, 135-138, 203-211, 300-308`; модель `ui/src/lib/ruleout.ts:38-54` | `outputKind.ruleTarget` |
| Мастер: ветка `'warp'`, проверка `steer-warpbox` | `ui/src/components/board/SetupWizard.tsx:23, 70-75, 125-129` | слот `setupVia` |
| Значки: группы и порядок | `ui/src/lib/badges.ts:31-44, 380-402` | слот `badge` |
| Виды: `OutputKind` без `warp`, приведения `'warp' as …` | `ui/src/lib/model.ts:17-26` | вид — строка, известные — из реестра |
| Неизвестный вид и ключи сохраняются в `extra` туда и обратно | `ui/src/lib/specv2.ts:148-163, 268-271, 742-756` | основа `OutputV2` |
| rpc — всегда объект `splify2`; ответ `{ok, error?}`; `deadline` | `ui/src/lib/rpc.ts:102-116, 136-151, 177` | `host.rpc` → `splify2.p.<id>` |
| Живые данные — хук `useLive()`, сигналов нет | `ui/src/lib/live.ts:65-138, 201` | `host.core.useLive` — хук |
| Правка спеки — `pending.edit(next)`, применение само через 800 мс | `ui/src/lib/pending.ts:318-331, 699`; `ui/src/lib/autoapply.ts:21-23` | `host.core.edit` |
| Строки — один словарь `ru`, `S.ns.key`; простые строки вклеиваются при сборке | `ui/src/copy/index.ts:3-6`; `ui/vite.config.ts:28-78` | у плагина свой словарь; переводы хоста — плагины `type: language` |
| Модули — `ModulesCard`, `STEER_MODULES`, `moduleFor` | `ui/src/components/ModulesCard.tsx:17-40`; `ui/src/lib/engine.ts:125-151` | вкладка «Плагины» |
| Карточка правила: имя открывает правило, выключатель, удаление, строка состояния; открытое правило — `RuleEditor` на месте списка, у десктопа черновиком до «Сохранить» | `ui/src/components/sections/Home.tsx:716-801`; `ui/src/components/tabs/RulesTab.tsx:134-160, 342, 368-380` | тот же приём у карточки плагина и слота `pluginSettings` |

## Загрузка

1. После монтирования панель зовёт `splify2.plugins_list` (бэкенд читает `steer plugin list --json`).
2. Для каждого включённого плагина с `ui`, у которого `compat.host-api` выполняется версией панели:
   - ставит `globalThis.__splify2_host = { preact }` (один раз, до первого `import()`);
   - подключает `ui.css` в shadow root;
   - `await import('/luci-static/resources/splify2/plugins/<id>/<entry>?v=<версия плагина>')`;
   - зовёт `activate(host)`.
3. Несовместимый плагин не грузится; раздел «Плагины» показывает причину: «интерфейс warpbox 1.4
   требует host-api ^1.2, у панели 1.0».
4. `?safe=1` в адресе панели — загрузка без плагинов.

Под десктопом код приходит с роутера архивом, главный процесс сам проверяет `MANIFEST.sig` и отдаёт
файлы через свой протокол `splify-plugin://` (CSP `script-src 'self'`, `desktop/renderer/index.html:5`;
PLUGINS.md 5.3). `host.platform = 'desktop'`, `host.rpc` идёт через `desk.api.call`
(`desktop/renderer/main.tsx:28-37`) в `/v1/plugin/<id>/<метод>`.

## Вкладка «Плагины»

Решение владельца (2026-10-25): в панели — вкладка «Плагины», встроенный раздел Rail (`plugins`, между
«Диагностикой» и «Настройками»). Она заменяет `ModulesCard`. Своих разделов в Rail у плагинов нет:
WarpBox и ReCheck живут внутри этой вкладки.

**Каталог и репозитории** (верх вкладки):
- поиск по всем подключённым репозиториям, карточка найденного плагина: описание, версия, что он
  добавит (виды, права), репозиторий; «Установить» — задание `plugin_install` с ходом и отказом числами
  (место, совместимость);
- репозитории: список, «Добавить репозиторий» (адрес и ключ; номер ключа показывается),
  «Убрать»; у стороннего — пометка «не официальный, splify2 за него не отвечает» (В6).

**Установленные** — карточки по образцу карточки правила (`RuleCard`,
`ui/src/components/sections/Home.tsx:716-801`):

| Часть карточки правила | У карточки плагина |
|---|---|
| имя — кнопка, открывает правило (`onOpen`, `Home.tsx:777`) | имя — кнопка, раскрывает карточку в настройки плагина |
| выключатель `Switch` справа сверху (`Home.tsx:779`) | включить/выключить плагин (`plugin_enable`/`plugin_disable`) |
| строка «куда ведёт» бейджем (`Home.tsx:782-787`) | версия, репозиторий, пометка стороннего; доступное обновление |
| удаление корзиной с подтверждением (`Home.tsx:789-798`, `useConfirm`) | «Удалить» с подтверждением; если вид плагина есть в спеке — отказ называет выходы |
| строка состояния в подвале (`Home.tsx:799`) | `summary(live)` плагина или общая строка хоста: состояние и причина (`failed`, `pending`, «нужен пакет…») |

Раскрытая карточка — как открытое правило: одна за раз, на месте списка, со стрелкой назад
(`RulesTab.tsx:134, 342, 368-380`); на десктопе правки спеки — черновиком до «Сохранить», как
`defer` у `RuleEditor` (`ctx.deferred`). Содержимое — слот `pluginSettings` плагина; под ним хост
сам добавляет общий блок: версия и «Обновить», автообновление и закрепление версии, права, системные
пакеты «поставлено для плагина». У плагина без `ui` раскрытая карточка — только общий блок.

## Модуль плагина

```ts
import type { PluginModule } from '@splify2/plugin-api'

const plugin: PluginModule = {
  activate(host) {
    const { h } = host.preact
    host.slots.pluginSettings({
      // строка на свёрнутой карточке: состояние и действие, без техники
      summary: live => {
        const st = live.status?.plugins as { id: string; state: string }[] | undefined
        const me = st?.find(p => p.id === 'recheck')
        return me?.state === 'enabled'
          ? { text: host.i18n.t('summary.on'), tone: 'ok' }
          : { text: host.i18n.t('summary.off'), tone: 'idle' }
      },
      // раскрытая карточка: настройки и состояние ReCheck (rpc status/set/forget)
      render: ctx => h(ReCheckSettings, { host, live: ctx.live, onClose: ctx.close }),
    })
  },
}
export default plugin
```

- Экспорт — `default` или именованные `activate`/`deactivate`.
- `preact`: сборщик SDK (`@splify2/plugin-api/vite`, `splify2Plugin()`) подменяет `preact`, `preact/hooks`, `preact/jsx-runtime`, `react`, `react-dom`,
  `react/jsx-runtime` на прокладку, которая берёт всё из `globalThis.__splify2_host.preact`. Свой
  экземпляр preact сломал бы хуки. Import map не годится: под LuCI страница чужая, вставить его раньше
  модулей LuCI нельзя.
- Иконки — плагин несёт свои (lucide-иконки — маленькие компоненты на том же preact).
- Все регистрации снимаются сами при отключении плагина.

## Объект `host`

| Поле | Что |
|---|---|
| `apiVersion`, `hostVersion`, `platform` | версии и где работает |
| `plugin` | `id`, `version`, `repo`, `trust` (`official`, `third-party`, `unsigned`), `base` — адрес каталога `ui/` |
| `preact` | `h`, `Fragment`, `render`, `hooks` — тот же экземпляр, что у панели |
| `ui` | компоненты панели (ниже) |
| `rpc` | `call`, `job`, `deadline` — объект ubus `splify2.p.<id>` |
| `core` | `useLive`, `getLive`, `useSpec`, `edit`, `on`, `plugins` |
| `i18n` | `lang`, `t`, `plural`, `onChange` |
| `styles` | `add(css)`, `link(path)` — в shadow root |
| `slots` | точки встраивания (ниже) |
| `themes` | `register`, `current` |
| `nav` | `open(section)`, `openPlugin(id?)` — вкладка «Плагины» с раскрытой карточкой |
| `prefs` | настройки интерфейса плагина на роутере |
| `notify`, `confirm` | сообщение и подтверждение панели |

### rpc

`host.rpc.call(method, params)` → объект `splify2.p.<id>`, метод из `rpc.methods` манифеста. Права — ACL
`splify2-p-<id>` из `access` (read/write), как у любого `luci-app-*`. Ответ — JSON программы как есть;
`{ok:false, error}` и ошибки ubus приходят как `RpcError` с `kind`: `denied`, `timeout`, `unavailable`,
`failed`. У `RpcError` из отказа метода есть ещё `code` (машинное слово ответа) и `args` (подстановки `{имя}`), если метод их
назвал: текст `message` хост уже подставил на языке каркаса, а плагину со своим переводом достаточно `code` и `args`
([splify2-api.md](splify2-api.md#тексты-для-человека)). `job()` — для методов с `job: true`: ждёт задания, промежуточные ответы — в `onProgress`.

### core

- `useLive()` — хук на тот же опрос, что у панели; `getLive()` — снимок. Гарантированные поля
  `Live`/`Status`/`OutputStatus` — в типах; остальное панель может менять между minor.
- `useSpec()` — сохранённая спека v2 в форме JSON (`SpecV2`, как её разбирает ядро), `dirty`, `applying`.
- `edit(fn)` — правка копии; панель переводит её в свою модель, кладёт в черновик (`pending.edit`) и
  применяет сама. Неизвестные виды и ключи доходят до ядра без потерь (`extra`,
  `ui/src/lib/specv2.ts:268-271, 742-756`).
- `on(ev, cb)` — события сокета `steerd` (`applied`, `switched`, `failed`, `helper-up`, `helper-down`,
  `plugin`, `plugin-state` и остальные из `steer/docs/ctl.md:500-512` и [steer-api.md](steer-api.md), 4.1).
- `status.outputs.<имя>` несёт поля плагинов: `owner`, `unavail`, `plugin` ([spec-fields.md](spec-fields.md)).

### Слоты

| Слот | Заменяет | Что регистрирует |
|---|---|---|
| **`pluginSettings`** — главный | `desktop/renderer/main.tsx:48` (`EXTRA = [warpSection, recheckSection]`), `ModulesCard` | содержимое раскрытой карточки плагина во вкладке «Плагины» (`render(ctx)`) и строка на свёрнутой (`summary(live)`) |
| `boardTile` | плитки `Home.tsx:1124-1146`, `WarpTile` (`:1311`) | плитка «Доски»: `when(live)`, `render(live)` |
| `outputKind` | `OutputKind` (`model.ts:17-19`), `WarpPill` и часть WARP блока «Куда» | вид выхода: `create`, `editor`, `summary`, `ruleTarget`, `hidesOwned` |
| `setupVia` | `Via` мастера (`SetupWizard.tsx:23`) | ветка «через что» мастера |
| `badge` | `ProtoId`/`FEATURE_ORDER` (`badges.ts:26-44, 380-402`) | значок выхода в группе и порядке панели |
| `diag` | — | проверка в «Диагностике» с приговором `ok/note/warn/fail` (как `DiagCheck`, `ui/src/lib/live.ts:26`) |
| `railSection` — **исключение** | `ExtraSection` (`Console.tsx:50-55`) | свой раздел в Rail; работает, только если манифест объявил `"ui": {"rail": true}` и человек не снял «показывать в полосе» в карточке плагина |

**Встроенные разделы и экраны панели регистрируются тем же API** — внешнего у них нет, закрытые
объединения уходят. По умолчанию плагин в Rail не попадает: WarpBox и ReCheck живут внутри вкладки
«Плагины».

`render` каждого слота обёрнут предохранителем: ошибка отрисовки показывает «Плагин X не
отрисовался — отключить», панель живёт. Ошибка в `activate` отключает плагин в этой сессии.

### Компоненты

`host.ui` — компоненты панели с теми же свойствами, что внутри (`ui/src/components/ui/*`,
`formbits.tsx`): `Button`, `Badge`, `Card`, `CardContent`, `Switch`, `Hint`, `ScreenHeader`, `CardHead`,
`Block`, `Group`, `TapRow`, `KV`, `ToggleRow`, `Segmented`, `Empty`, `FieldRow`, `Field`, `Chip`,
`FileButton`, `DangerButton`, `Meter`, `Spark`, `StateLine`, `Flag`, класс поля `inputClass`. Свойства —
в типах. Свои компоненты плагин рисует на переменных темы, чтобы светлый и тёмный режим работали сами.

### Темы

- Тема — набор значений переменных. Панель берёт `--sp-*` (`ui/src/index.css:48-116` — светлая, `:118-153`
  — тёмная, значки `:775-783`), которые ссылаются на токены дизайн-системы `--an-*`
  (`ui/andromeda/tokens/colors.css`: семейства `--an-accent*`, `--an-success|warn|danger*`,
  `--an-grey-*`, `--an-surface-*`, `--an-border*`, `--an-text*`, `--an-cat-1..7`, `--an-control-*`,
  `--an-glass-*`, `--an-font-*`, `--an-space-*`, `--an-radius-*`, `--an-shadow-*`, `--an-dur-*`).
- Плагин-тема — `themes[]` в манифесте, JSON без JS: `{name, light, dark?, css?}`. Плагин с кодом может
  звать `host.themes.register` сам.
- У хоста появляется переключатель «Тема» и режим `auto | light | dark`; `auto` — как сейчас, по фону
  LuCI (`ui/src/main.tsx:31-42`) или системе на десктопе.
- `--sp-bleed`, `--sp-inset-l`, `--sp-inset-r` ставит сама панель (`main.tsx:65, 76-77`) — тема их не
  задаёт.

### i18n

- Словарь плагина — `i18n/<язык>.json` (плоский объект «ключ → строка», подстановки `{имя}`).
  `host.i18n.t(key)` ищет в языке человека, затем в `ru`, затем отдаёт ключ. Строк хоста плагин не
  видит и от их ключей не зависит.
- **Переводы хоста — тоже плагины** (решение В9): `type: language`, `i18n.host.<язык>` — словарь строк
  панели. В стандартной поставке splify2 — `ru` (вшит в сборку) и плагин `lang-en`, который ставится
  вместе с splify2 из его семени. Для этого панель меняет сборку строк: сейчас простые строки
  вклеиваются в код при сборке и из `ru.ts` вырезаются (`ui/vite.config.ts:28-72`), а функции-строки
  (`ru.ts:30`, `plural` `:12`) остаются кодом. Словарь хоста для плагина языка — JSON с ключами
  `ns.key`; функции записываются шаблоном с `{0}` и формами множественного числа. Версия словаря
  хоста — `hostVersion`; `i18n.host.<язык>.host` в манифесте говорит, к каким версиям он подходит.
- Тексты — по правилу хаба: состояние и действие, без технических пояснений; `s2p lint` проверяет
  словари на технические слова.

### Изоляция

Код плагина исполняется в контексте панели: у него есть всё, что есть у панели, включая сессию LuCI
(`window.luci_rpc`). Ограничивает его только подпись ([s2p.md](s2p.md)) — права официального и
стороннего репозитория одинаковы (В6), интерфейс помечает сторонний «не официальный, splify2 за него
не отвечает».

## Совместимость

- **minor добавляет** (новый слот, поле `host`, компонент, необязательное свойство), **major ломает**.
  Плагин `^1.2` на панели `1.0` не грузится.
- Гарантированы только типы пакета `@splify2/plugin-api` той же major. Внутренности панели (`S`,
  `pending`, `rpc`, классы Tailwind) — не API.
- Панель пишет свою версию host API в `build-id.txt` и отдаёт её в `plugins_list`.
