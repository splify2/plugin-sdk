# Шаблон: интерфейс на `@splify2/plugin-api`

Карточка плагина во вкладке «Плагины» панели splify2: слот `pluginSettings`, компоненты панели `host.ui`,
вызов своего метода `host.rpc.call` (rpc — shell-скрипт `rpc/hello`), строки из `i18n/<язык>.json`.
Сборка — vite, один ES-модуль `ui/index.js` и `ui/style.css`; `preact` общий с панелью.

```sh
npm install                      # пока пакет не опубликован: npm install -D <путь>/plugin-sdk/packages/plugin-api
s2p keygen -o dev
s2p build . --key dev.sec -o dist    # запускает npm run build, кладёт результат в ui/
s2p verify dist/example-ui-0.1.0-all.s2p --pub dev.pub
```

Проверка типов: `npm run check`. Исходники интерфейса — `ui-src/` (в архив не попадают), `s2p.json` —
шаг сборки: команда получает каталог результата в `S2P_UI_OUT`.
