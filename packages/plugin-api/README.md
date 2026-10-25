# @splify2/plugin-api

host API панели [splify2](https://github.com/splify2/splify2) для интерфейса плагина: **типы**, константа
версии и сборка на vite, где `preact` один на панель и плагин.

Версия пакета равна версии host API. Плагин объявляет её в манифесте — `"compat": {"host-api": "^1.0"}` —
и панель не загрузит интерфейс, если её версия не подходит. Описание API —
[docs/host-api.md](https://github.com/splify2/plugin-sdk/blob/main/docs/host-api.md), «как написать плагин» —
[docs/guide.md](https://github.com/splify2/plugin-sdk/blob/main/docs/guide.md).

## Модуль плагина

```ts
import type { PluginModule } from '@splify2/plugin-api'

const plugin: PluginModule = {
  activate(host) {
    const { h } = host.preact
    host.slots.pluginSettings({
      summary: () => ({ text: host.i18n.t('summary.on'), tone: 'ok' }),
      render: ctx => h('div', null, host.i18n.t('title')),
    })
  },
}
export default plugin
```

`host` приходит в `activate`: `rpc` (методы плагина), `core` (живые данные, спека, события), `slots`, `ui`
(компоненты панели), `i18n`, `themes`, `nav`, `prefs`. Регистрации снимаются сами при отключении плагина.

## Сборка на vite

```ts
// vite.config.ts
import { defineConfig } from 'vite'
import { splify2Plugin } from '@splify2/plugin-api/vite'

export default defineConfig({ plugins: [splify2Plugin()] })
```

`splify2Plugin()` собирает один ES-модуль `index.js` (и `style.css`, если он есть) в каталог из `S2P_UI_OUT`
(его задаёт `s2p build`), а `preact`, `preact/hooks`, `preact/jsx-runtime`, `react`, `react-dom` подменяет
прокладкой на `globalThis.__splify2_host.preact`. Своя копия preact сломала бы хуки панели.
`s2p lint` отвергает модуль, который тянет preact отдельным `import`.

Для JSX в `tsconfig.json`: `"jsx": "react-jsx", "jsxImportSource": "preact"`.

## Что в пакете

| Файл | Что |
|---|---|
| `index.d.ts` | типы host API: `PluginModule`, `Host`, слоты, компоненты, `Live`, `SpecV2` |
| `index.js` | `HOST_API_VERSION` |
| `vite.js`, `vite.d.ts` | `splify2Plugin()` и исходники прокладок `shims` |

## Разработка пакета

```sh
npm install
npm run build   # проверка типов (strict) и dist/
npm test        # версия, прокладки на подставном хосте, конфиг vite
npm pack --dry-run
```

Публикация — только по решению владельца: `npm publish` из каталога пакета (`prepack` соберёт `dist/`).

Лицензия MIT.
