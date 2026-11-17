# Draft: минимальная спека v3 и plugin lock

Это проверяемый проект формата для этапа A универсального Steer, а не поддержка
runtime. Источник архитектурного решения — `steer/docs/universal-core.md`, §4–5,9.
Текущий манифест, оси API/ABI, команды `s2p` и чтение v1/v2 не изменены.
Не устанавливайте эти документы как конфигурацию действующего ядра.

Схемы: `schema/draft/spec-v3.schema.json` и `schema/draft/plugin-lock.schema.json`.
Проверка изолирована в `cmd/s2p/spec_v3_draft.go`; CLI её пока не вызывает.
JSON Schema проверяет форму, Go дополнительно проверяет ссылки и циклы.
Сейчас вход проверки — JSON. Повтор ключа (включая разные escape-записи одного
имени), несколько JSON values и некорректный JSON отвергаются до проверки схемы.
Лимиты draft entry point: 1 MiB на документ, глубина 64 и 1024 элемента в одном
object/array, включая `options`. YAML parser в этот этап не входит.

## Подмножество спеки

Обязательны `version: 3`, `plugin_remotes`, `plugins`, `inbounds`, `outbounds`,
`routing`. Имена состоят из ASCII букв, цифр, `_ . -`, начинаются с буквы,
цифры или `_`, длина до 63. Это ограничение draft, не изменение имён legacy.

Remote содержит HTTPS URL подписанного индекса и `trust` — имя внешнего корня
доверия. URL не содержит userinfo или fragment; явный порт — 1–65535.
Значение `trust` не добавляет
ключ в trust store. Alias содержит `remote`, `package`, `version` (диапазон
версий существующего SDK). Ссылка `plugin` имеет ровно вид `provider:alias`.
Alias обязан существовать; его remote обязан существовать.

Inbound содержит `plugin`, `listen: {address, port}`, `options: {}`. Port —
1–65535. Адрес пока непустая строка: разрешение/классификация bind address,
IPv4/v6 dual-stack и коллизии TCP/UDP принадлежат будущему host validate.
Outbound — либо `plugin` + `options`, либо `type: direct|block`, либо
`type: group` + `strategy` + непустой уникальный `members`. Стратегии:
`order`, `latency`, `manual`, `balance`, `failover`. Ссылки членов проверяются,
циклы групп запрещены. `plugin` и `type` взаимоисключающие.

Routing содержит `rules` и `default`. Правило — уникальное `name`,
`match: {inbound: [имена]}` и `outbound`. Все ссылки проверяются. Неизвестные
поля отвергаются на каждом общем уровне. DNS, subscriptions, users, secrets,
прочие classifiers и параметры стратегий не поддерживаются этим подмножеством.
`options` намеренно открытый object: их проверит подписанная схема provider;
этот этап не подтверждает корректность параметров или доступность протокола.

## Lock

Draft lock имеет `version: 1`, `target`, `remotes` и `plugins` по явным aliases спеки.
Snapshot `remotes` содержит записи `{url, trust, index_sha256, signer}`.
URL и trust обязаны точно совпадать со спекой, без URL canonicalization.
Snapshot каждого используемого remote обязателен. Snapshot неиспользуемого
remote допустим, если remote существует в спеке и identity совпадает.
Каждая запись содержит `remote`, `package`, точную semver `version`, `target`,
`artifact_sha256`, `index_sha256` (64 строчные hex цифры), `signer`, список
`providers`, `api` (оси → диапазоны) и `dependencies` (aliases).

Проверяется полнота aliases, identity remote/package, version constraint,
одинаковый target, объявление используемых providers, существование dependencies
и отсутствие циклов. `index_sha256` и `signer` пакета должны совпадать со snapshot
его remote. Alias одного remote/package не может фиксировать разные версии,
artifact hashes, provider sets или API ranges: отдельный execution namespace
пока не поддержан. Порядок provider IDs значения не имеет.
В первом v3 namespace package ID глобален, как identity установленного пакета:
один package через разные remote aliases отвергается как `ambiguous origin`,
даже если URL совпадают. Пользователь должен свести aliases этого пакета к
одной remote записи. Это намеренная консервативная политика первого формата;
отдельные package namespaces пока не поддерживаются.
Минимальный draft требует, чтобы транзитивные nodes тоже были явно перечислены
в `plugins` спеки; автоматический dependency resolver ещё не реализован.

Hashes и signer — фиксация ожидаемых значений, не доказательство подписи.
Trust здесь — имя, не проверенный fingerprint ключа.
Проверка байтов архивов/индекса, trust roots, expiry, revocation, antirollback,
direction/capability provider, совместимости API с host и OS/ABI target ещё
предстоит. Нет fetch/install/resolve, сетевых запросов или исполнения пакетов.
Apply должен использовать локальный проверенный lock; этот этап apply не меняет.

## Проверка и миграция

`go test ./cmd/s2p -run 'Test(SpecV3Draft|PluginLockDraft)' -count=1` проверяет
валидный вертикальный пример и отказы формы, ссылок, XOR, ограничений версий,
групп, lock identity/target/digest/provider/dependencies, remote snapshots,
duplicate JSON keys, границ размера/вложенности, портов и exact semver.
Полный Go suite:
`go test ./cmd/s2p -count=1`.

В Windows с установленным MSYS Go требуется корректный GOROOT процесса,
например `$env:GOROOT='C:/msys64/ucrt64/lib/go'` для текущей среды.

Конвертера v1/v2 ещё нет. Будущий inspect/plan создаёт v3 и lock рядом со
старой конфигурацией; автоматическая перезапись legacy запрещена. Перенос net1,
provider ABI, native loader, реальные протоколы и runtime generation lifecycle
не реализованы этим draft и не объявляются совместимыми на основании схем.
