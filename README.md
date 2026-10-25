# plugin-sdk

Спецификации платформы плагинов splify2 и steer (этап 0): манифест, индекс репозитория, архив `.s2p`,
API ядра для плагинов, ABI `.so`, host API панели.

| Документ | Что |
|---|---|
| [docs/manifest.md](docs/manifest.md) · [schema/plugin.schema.json](schema/plugin.schema.json) | манифест `plugin.json` |
| [docs/index.md](docs/index.md) · [schema/index.schema.json](schema/index.schema.json) | индекс репозитория, отбор версии |
| [docs/s2p.md](docs/s2p.md) | архив, подпись, раскладка, установка, откат, автообновление |
| [docs/steer-api.md](docs/steer-api.md) | steer-api v1: программы, evline, хуки, сокет, nft, ресурсы |
| [docs/steer-abi.md](docs/steer-abi.md) · [include/steer/plugin.h](include/steer/plugin.h) | steer-abi v1: `.so` в `steerd` |
| [docs/host-api.md](docs/host-api.md) · [packages/plugin-api](packages/plugin-api) | host API панели, типы `@splify2/plugin-api` |
| [docs/spec-fields.md](docs/spec-fields.md) | новые поля модели и спеки — договор агентов ядра |
| [docs/migration-2.1.md](docs/migration-2.1.md) | переход на steer-core 2.1.0 |
| [docs/open.md](docs/open.md) | выборы без решения владельца |
| [examples/](examples) | образцы манифестов и индекса |
