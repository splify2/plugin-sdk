<div align="center">

<img src="https://splify2.github.io/assets/img/logo.svg" width="104" alt="">

# plugin-sdk

**Всё, чтобы написать, собрать, подписать и выпустить плагин для splify2 и ядра steer — без SDK OpenWrt**

[![Лицензия](https://img.shields.io/badge/лицензия-MIT-6d5ce7)](LICENSE)
[![Документация](https://img.shields.io/badge/документация-splify2.github.io-a897ff)](https://splify2.github.io/docs/)
[![Telegram](https://img.shields.io/badge/Telegram-чат-2CA5E0?logo=telegram&logoColor=white)](https://t.me/ssplify)

</div>

Плагин — архив `.s2p` с манифестом `plugin.json`: вид выхода или протокол для steer, интерфейс для
панели splify2, тема или перевод. SDK даёт спецификации, инструмент `s2p`, заготовки и пакет типов
`@splify2/plugin-api`.

## Возможности

- **`s2p`** — один статический файл на Go без зависимостей: `lint`, `build`, `sign`, `verify`, `index`, `cc`,
  `new`, `keygen`
- **Одинаковый вход — одинаковые байты**: ustar в одном gzip без времени и владельцев, MANIFEST с SHA-256;
  повторная сборка плагина с машинным кодом даёт тот же архив
- **Подпись usign/signify (Ed25519)** в формате `usign`; несколько блоков в одном файле — для ротации
  ключей. Стенд сверяет подпись независимо через `openssl`, а где есть `usign` — и им
- **C под 10 архитектур OpenWrt через zig (musl)**, без SDK OpenWrt: программа-помощник и `.so` по
  [steer-abi](docs/steer-abi.md)
- **Заготовки**: программа-помощник, `.so`, интерфейс на vite, тема, перевод — каждая собирается в `.s2p`
- **`s2p lint`** смотрит на манифест, файлы, права, теги совместимости, словари и сам машинный код:
  машина ELF, экспорт `.so`, `DT_NEEDED`
- **`s2p index`** собирает и подписывает индекс репозитория: много плагинов, много версий и архитектур,
  отзыв и удаление версий

## Установка

Нужен Go 1.21+. Для C — zig 0.13 (в `PATH` или `S2P_ZIG`) либо docker с образом `steer-builder:wolfssl`.

```sh
git clone https://github.com/splify2/plugin-sdk && cd plugin-sdk
sh build.sh                       # bin/s2p; положите его в PATH
```

## Быстрый старт

```sh
s2p new helper ./myplug           # заготовки: helper, so, ui, theme, lang
cd myplug
s2p keygen -o dev                 # ключ для разработки: dev.sec (секрет) и dev.pub
s2p lint .
s2p build . --key dev.sec         # dist/myplug-0.1.0-<арх>.s2p под архитектуры манифеста
s2p verify dist/myplug-0.1.0-x86_64.s2p --pub dev.pub
```

На роутере с ядром steer 2.1: `steer plugin install --file myplug-0.1.0-<арх>.s2p --unsigned` (без подписи —
только из файла, для разработки) либо подпись вашего ключа, принятого человеком. Подробно — в
[docs/guide.md](docs/guide.md).

Репозиторий плагинов:

```sh
s2p index dist -o repo --repo mine --pub dev.pub --key index.sec --mirror https://example.org/plugins/
s2p verify repo/index.json --pub index.pub --base repo
```

## Что в репозитории

| Путь | Что |
|---|---|
| [docs/guide.md](docs/guide.md) | как написать плагин: от заготовки до выпуска |
| [docs/manifest.md](docs/manifest.md) · [schema/plugin.schema.json](schema/plugin.schema.json) | манифест `plugin.json` |
| [docs/index.md](docs/index.md) · [schema/index.schema.json](schema/index.schema.json) | индекс репозитория, отбор версии |
| [docs/s2p.md](docs/s2p.md) | архив, подпись, раскладка, установка, откат, автообновление |
| [docs/steer-api.md](docs/steer-api.md) | steer-api v1: программы, evline, хуки, сокет, nft, ресурсы |
| [docs/steer-abi.md](docs/steer-abi.md) · [include/steer/plugin.h](include/steer/plugin.h) | steer-abi v1: `.so` в `steerd` |
| [docs/host-api.md](docs/host-api.md) · [packages/plugin-api](packages/plugin-api) | host API панели, типы `@splify2/plugin-api` |
| [docs/spec-fields.md](docs/spec-fields.md) · [docs/migration-2.1.md](docs/migration-2.1.md) | поля спеки для ядра; переход на steer-core 2.1.0 |
| [docs/open.md](docs/open.md) | выборы без решения владельца |
| [cmd/s2p](cmd/s2p) | инструмент `s2p` |
| [include/s2p/evline.h](include/s2p/evline.h) · [lib/evline](lib/evline) | события программы ядру без зависимостей |
| [templates/](templates) | заготовки: `helper`, `so`, `ui`, `theme`, `lang` |
| [examples/](examples) | образцы манифестов и индекса |
| [tests/](tests) | стенд: `sh tests/run.sh` |

## Проверка

```sh
sh tests/run.sh               # lint, build, sign, verify, index; C под x86_64, aarch64_cortex-a53, arm, mips
FULL=1 sh tests/run.sh        # C под все 10 архитектур
NO_UI=1 sh tests/run.sh       # без npm
```

Стенд проверяет детерминизм сборки, порчу подписи, чужой и отозванный ключ, пути `..` и другие изъяны
архива, `file` и `readelf` у собранных `.so` и помощников, индекс из нескольких плагинов и версий.
