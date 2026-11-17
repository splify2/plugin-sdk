# Steer provider API v1 — проект контракта для Steer 3.0.0

Статус: архитектурное решение, не опубликованный ABI. Заголовок, bindings и host
должны пройти указанные ниже проверки до объявления compat axis. Старые
`steer-api`, `steer-abi`, `steer-crypto`, package IDs и net1 не переопределяются.

## 1. Регистрация, версии и исполняемый формат

Новая ось `steer-provider-api` описывает семантику provider API. Major означает
несовместимость; minor — добавленные необязательные tail-поля/функции, которых
старый caller не читает. Значения enum зарезервированы и не переиспользуются.
Версия пакета, версия спеки, management API и provider ABI независимы.

Новые native providers распространяются только в manifest schema 2 после
реализации loader/linter: schema 1 сегодня допускает неизвестные поля, потому
новое обязательное поведение нельзя прятать в schema 1. Старый installer должен
отказать на schema 2 до hooks/активации/изменения installed state. Существующий
installer сначала скачивает и распаковывает проверенный архив в staging; эти
временные записи допустимы и не считаются установкой. Schema 2 сохраняет существующие
поля идентичности и зависимостей и добавляет обязательные descriptors providers;
legacy schema 1 продолжает работать отдельным путём. Формат архива `.s2p` и
подписи не меняется только ради номера manifest schema.

Защита старого installer не равна защите старого runtime при ручном размещении
каталога. В рабочем дереве `src/kinds/plugin.c:plugin_load` добавлен отказ при
schema != 1 (включая отсутствие/неверный тип) и наличии `steer-provider-api`:
новый provider не регистрируется как legacy kind/command/service. Изменение
ещё не собрано и не проверено исполнением. Общий registry loader используется
для каталогов и embedded manifests. Отдельный `src/daemon/plugso.c` читает
`so.load`; перед dlopen теперь проверяет существующий манифест тем же helper
`src/lib/plugmanifest.h`. Legacy bare-.so API пока допускает отсутствие
манифеста для совместимости. Это исключение запрещено переносить в новый
provider loader: он обязан требовать проверенный schema2 package. Проверки
нового `.so` пути добавлены в `tests/abimatch.sh`, но также не запускались.
В host gate A3 обязательно добавить schema/required-axes проверку во все пути
загрузки (installed, builtin, direct directory), до dlopen/hooks/registration.
Новый package нельзя объявлять безопасным для старого runtime в обход manager.

Descriptor содержит ID, api range, execution modes, target ABI, directions,
flow classes, supported combinations, options schema digest и host capabilities.
Target разделяет OS/CPU/ABI/libc/endianness; `arch: all` запрещён для native кода.
Используемые комбинации transport/security задаются допустимыми tuples, а не
декартовым произведением отдельных списков. Права не выводятся из имени provider.

Entrypoint имеет одно экспортируемое имя `steer_provider_query_v1`; получает
versioned host table и выдаёт immutable provider table. Статическая регистрация
вызывает тот же entrypoint. Dynamic lookup/constructors не заменяют проверку
манифеста, подписи и размеров таблиц. Регистрация не открывает сеть и не запускает
фоновые потоки. Никакого fallback на legacy ABI при неуспешной проверке.

## 2. ABI и управление памятью

Граница C ABI: `uint32_t` для tags/status/length таблиц, `uint64_t` для счётчиков,
opaque pointer handles, spans с pointer + length. Pointer width/alignment и
calling convention определяются target ABI; это не сериализуемый wire format.
`size` — количество доступных байт таблицы, `version` — номер layout. Host
проверяет минимальный обязательный prefix до чтения поля. Неизвестный required
capability — отказ; необязательная NULL function — unsupported.

Нет C `bool`, bitfields, compiler-packed structs, `long`, `sockaddr`, `fd`,
`FILE*`, внутренних net1 structs или Rust/C++ containers. Native handle внутри
платформенного адаптера не экспортируется протоколу. Callback userdata — opaque
pointer, живущий до завершения операции. Rust использует `repr(C)`, panic не
пересекает FFI; abort запрещён в штатной ошибке протокола.

Borrowed span существует только до возврата текущего вызова. Для async данных
используются host-owned refcounted buffers: allocate → retain → release;
провайдер не вызывает чужой `free`. Каждый успешный transfer ownership явно
указан контрактом операции. Получатель не освобождает borrowed metadata.
Secret buffer имеет отдельный restricted handle, не попадает в tracing/dumps.

## 3. Потоки исполнения и асинхронность

Один flow прикреплён к одному event-loop shard до закрытия. Callbacks данного
flow сериализованы на этом shard; provider не использует глобальный lock для
всех flows. Cross-shard work передаётся bounded queue. Blocking DNS/filesystem,
certificate reload и тяжёлая работа выполняются в bounded worker pool, результат
возвращается на исходный shard. Провайдеру запрещено создавать безлимитные threads.

Async start возвращает либо `REJECTED(error)` (callback не будет), либо `PENDING`
с operation handle (ровно один completion, всегда отложенный, не inline).
Operation terminal result: success, error или cancelled. Cancel — запрос, не
гарантия победы над уже совершившейся операцией. При гонке success/cancel host
выдаёт только один completion. Handle нельзя повторно использовать до terminal
completion/release. Cancel idempotent; destroy допускается только после barrier
всех операций. После destroy callbacks невозможны.

Operation handle и его API — неразделимая пара владельца операции. Provider
может оборачивать внутреннюю host operation собственной задачей, чтобы сначала
остановить IO и освободить временные ресурсы, затем выдать terminal completion.
Host вызывает cancel/release только через возвращённую таблицу; opaque handle
нельзя передавать в другой operation service или трактовать как его внутренний
объект. Выбор CANCELLED в очереди сам по себе не доказывает остановку реального IO.

Ошибки имеют stable code и transient flag: unsupported, invalid, auth_failed,
policy_denied, resource_exhausted, timeout, cancelled, io, protocol, internal.
Provider не подменяет auth_failed direct route и не помещает секреты в message.
Диагностический текст не служит машинным интерфейсом.

## 4. Lifecycle configuration generation

| Шаг | Обязательство |
|---|---|
| validate | Pure проверка конфигурации/schema/capability; без сетевых эффектов |
| prepare | Allocate state, reserve resources; сокет не принимает новые flows |
| activate | Host публикует generation; provider начинает работу на разрешённых ресурсах |
| quiesce | Новые accepted flows прекращаются; старые продолжаются |
| drain | Завершение flows до monotonic deadline; затем cancellation |
| destroy | После op/flow barrier освободить все handles; повторное освобождение — ошибка host |

Host владеет listener, его readiness и generation routing. Provider владеет
протокольной сессией поверх выданного transport. Prepare failure вызывает unwind
в обратном порядке. Activation не выполняет произвольную необратимую работу;
внешние device mutations идут через journal core. Pending session до auth имеет
time/byte/connection budgets и не публикует application flow.

## 5. Stream, datagram и accepted flow

`accept_flow` передаёт: listener/inbound ID, generation, authenticated principal
handle (либо explicit anonymous), source endpoint, requested destination,
flow class и protocol attributes с проверенной схемой. Peer не может подставить
произвольный principal; provider получает его после host auth verification.
Host отвечает route accepted/rejected, держит references и управляет relay.

Endpoint — tagged IPv4/IPv6/domain + uint16 port; IPv6 scope обрабатывается
платформенным адаптером и явно недоступен, если контракт его не поддерживает.
Domain проходит bounded validation/IDNA policy; исходное доменное назначение
сохраняется до routing/DNS. Connection address не равен requested destination.

Streams имеют read/write, shutdown_write, cancel и close. Один pending read и
один pending write на stream, если явно не negotiated иначе. Write completion
сообщает принятое число байтов, допускает partial write; EOF на чтении не
закрывает запись. Нулевой read означает EOF, не временное отсутствие данных.
Закрытие не превращает TCP half-close в потерю buffered response.

Datagram association передаёт целые сообщения с source/destination, без concat
или partial success. Слишком длинное сообщение отвергается явно; truncation
не выдаётся за успех. Ограничены queued bytes/messages, количество associations,
reassembly bytes/fragments и idle lifetime. При перегрузке UDP drop считается
в telemetry; stream чтение останавливается до low watermark. Неавторизованный
UDP sender не может использовать чужую SOCKS association.

Device provider имеет отдельный interface/peer/packet contract. Kernel WireGuard
на Linux управляется broker через netlink, не через stream codecs. Его lifecycle
включает device ownership, peer public keys/allowed IPs, route ownership и cleanup.
Feature WARP создаёт декларативные ресурсы/WG или MASQUE dependencies; это не
универсальный self-hosted WARP inbound. Сторонние registration services не
переносятся в ядро под видом протокола.

## 6. Host services и ограничения

Host tables разделены на resource/time/log, transport, TLS, QUIC, DNS, policy,
auth, feature/device control. Provider запрашивает только нужные tables/caps.
Core не требует загрузки QUIC при использовании только SOCKS/TCP. Network
runtime может состоять из выборочно поставляемых core libraries с общей
сборкой/ABI; владение core не означает обязательный монолитный мегабинарник.

Transport APIs поддерживают direct dial и dial через routing context. Протокол
не вызывает бесконтрольно connect/getaddrinfo, обходя outbound policy. Bootstrap
DNS и dial dependency graph проходят cycle detection. TLS libraries и crypto
primitives централизованы, Reality-specific decisions остаются у provider.
TLS extension API проектируется по фактическим существующим Reality call sites
до начала этого inbound, а не объявляется универсальной передачей TLS internals.

Нативный trusted provider обладает полномочиями процесса: permissions table —
политика интерфейса, не security sandbox. Недоверенный native package запрещён
в shared-process профиле; optional isolated execution требует отдельного
transport adapter и OS sandbox проверки. Подпись устанавливает источник, не
доказывает отсутствие уязвимостей.

## 7. Приёмка ABI перед первым SOCKS inbound

1. C/C++ header compilation и Rust FFI layout checks на 32/64-bit targets.
2. Host reject: short table, unknown major/required cap, wrong target/schema,
   missing functions, unsupported tuple, несовместимый legacy plugin.
3. Mock lifecycle fault injection на каждом шаге; zero leaked handles после unwind.
4. Cancel/success гонки, deferred callbacks, unload barrier, callback after close.
5. Bounded read/write/UDP pressure, partial write и half-close; проверка лимитов.
6. Fuzz входных frames/options/metadata с ASan/UBSan; тесты не используют сеть production.
7. Первый native SOCKS provider через реальные host routes и upstream client;
   прохождение mock не означает server support.

Первые headers/bindings замораживаются только после двух независимых consumers
(C SOCKS и Rust test provider), полноценного host и measured resource baseline.
До этого всё находится в draft namespace и не повышает stable SDK version.
