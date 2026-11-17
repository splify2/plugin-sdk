# Первый draft C header provider API

Статус: декларации foundation, не работающий provider host и не frozen ABI.
Изолированная реализация buffer service находится в соседнем checkout
`steer/experimental/provider/`; она не подключена к production; standalone C fixtures прошли strict C11 и ASan/UBSan. Это ещё не host transport или provider lifecycle.
В том же модуле добавлена базовая host table: выдача buffer service, время и
журнал через переданные host callbacks. Clock/log backend задаёт вызывающий;
реальных сетевых сервисов, loader и lifecycle orchestration пока нет.
`steer/experimental/provider/lifecycle_fixture.c` соединяет эти primitives с
тестовым C provider через query/prepare/activate/quiesce/drain/destroy. Это
исходник вручную управляемого fixture, не production orchestration; он прошёл standalone strict C11 и ASan/UBSan.
Источник решения — [provider-v1-draft.md](provider-v1-draft.md).
Заголовок: `include/steer/draft/provider_v1.h`. Legacy `plugin.h`, net1,
stable manifest/CLI и объявляемые версии совместимости не изменены.

## Что выражено

- Fixed-width tags/errors, borrowed spans, opaque typed handles, C linkage и
  явная calling convention. Таблицы имеют `size` и draft-local layout `version`;
  этот номер не объявляет поддержку `steer-provider-api` у ядра.
- Descriptor с package/provider identity, API range, OS/CPU/ABI/libc/endianness,
  комбинациями direction/flow/transport/security, execution modes, required
  capabilities и digest схемы options. Pointer arrays исключают предположение
  о неизменном stride расширяемых tuple records.
  Target передаётся указателем и проверяется отдельно: его tail extension
  не сдвигает поля descriptor. Неверсионированные value records имеют явно
  фиксированный layout; их расширение требует новой версии содержащей таблицы.
- Tagged IP/domain endpoints без fd/sockaddr, IPv6 scope пока unsupported.
- Базовая host table: получение versioned tables, monotonic clock и bounded log.
  Одна конкретная service table — refcounted host buffers. Secret handle отдельно,
  `retain` возвращает ошибку без изменения ownership при переполнении счётчика;
  успешная передача дополнительного reference требует результата OK.
  secret access не реализован и не предоставлен через ordinary buffer service.
- Lifecycle validate/prepare/activate/quiesce/drain/destroy. Pure validate и
  query; prepare не публикует listeners; уничтожение после barrier.
- Async start REJECTED/PENDING, deferred exactly-once completion, cancel race,
  operation ownership/release, userdata/module lifetime и callback shard.
  Prepare completion передаёт instance; ошибка требует полного unwind.

`drain` barrier не включает собственную drain operation: callback сначала
возвращается, затем host освобождает operation и только после этого instance.
Deadline не даёт права уничтожить instance с ещё возможным callback.
Drain завершается сразу по достижении barrier; deadline — граница принудительной
отмены, а не обязательное ожидание до указанного времени.
Borrowed metadata/config не удерживаются после возврата start; pending code
должен скопировать данные. Передача host buffers в IO будет определена отдельно;
здесь writable view допустим лишь при единственном reference. Это явное
исключение из call-borrowed spans: view переживает возврат `writable` и
прекращает действовать перед retain/share/release/IO submission или следующим
writable/mutating service call соответствующего buffer.
REJECTED от drain не считается barrier: host сохраняет instance для recovery.

## Что намеренно не реализовано

Это foundation, а не полный dataplane contract. Stream/datagram/listener/principal/
route handles зарезервированы, но их data service tables ещё не объявлены.
Нет accept_flow, open/read/write/shutdown/close, datagram IO/reassembly, auth,
policy routing, DNS, TLS/QUIC, device/feature operations, listener handover,
secret access, scheduler API или перечня согласованных capability IDs.
Нельзя регистрировать descriptor stream/device tuple и на этом основании
сообщать о доступности такого протокола: отсутствуют host services и consumers.

Draft Rust FFI mirror добавлен в `experimental/provider-rust/` без dependencies
и с `no_std`: декларации и исходники проверок offsets/signatures. Они не
компилировались; сравнение C/Rust layouts на целевых архитектурах ещё не выполнено.
Для этого подготовлены независимые `layout.c` и `src/bin/layout.rs`, выводящие
размеры, выравнивание, offsets и числовые ABI-константы. `compare_layout.py`
сравнивает сохранённые отчёты; C emitter выдал 147 записей, Rust/comparator не запускались.
Изолированная completion queue в `steer/experimental/provider/operations.c`
реализует синтетические операции; она не отменяет реальные IO и не реализует
barrier уничтожения provider. Её standalone tests прошли strict C11 и ASan/UBSan.

Нет manifest schema2, loader/query implementation, generation journal,
проверенных межъязыковых layout fixtures, сборочных export scripts,
изолированного process transport или реального SOCKS provider. Вариант native
trusted mode обозначен; process/static packaging пока не реализованы. Target
пока декларативный descriptor, а не подтверждение переносимости Windows/mobile.
Критерии required prefixes/capability IDs и все numeric tags требуют совместной
проверки двух независимых consumers перед ABI freeze.

Запрет на сеть в query не предотвращает constructors динамической библиотеки:
manifest/target/signature/axis gate должен происходить до dlopen. Header не
реализует sandbox для native кода и не исправляет legacy directory loader.

## Проверка этого изменения

Отдельное экспериментальное расширение `include/steer/draft/stream_v1.h`
добавляет opaque stream read/write/half-close/close/release. Linux host в
`steer/experimental/provider/stream.c` использует существующий `loop.c`,
buffers и deferred operations; socketpair tests прошли strict C11 и ASan/UBSan.
Основной provider layout и `host.get_table` не изменены: расширение пока
получается через явный experimental getter. Это не listener, resolver,
route/dial service, загрузчик provider или поддержка TLS/QUIC. Rust mirror
этого расширения пока отсутствует. Подробности — `stream.md` рядом с host.

2026-11-17: WSL Ubuntu, x86_64-linux-gnu, 64-bit, GCC 15.2.0
(Ubuntu 15.2.0-16ubuntu1). Все пять C fixtures (buffers, operations,
admission, host, lifecycle) прошли strict C11 с -Wall -Wextra -Werror -pedantic
и отдельный ASan/UBSan запуск с leak detection: exit 0, без diagnostics.
Команды и границы: [provider README](../../steer/experimental/provider/README.md).
C layout emitter скомпилирован теми же strict flags, выдал 147 записей.
Rust/cargo отсутствуют в проверенной Windows/WSL среде; toolchain не установлен.
Cargo tests и независимое сравнение C/Rust layouts остаются непроверенными.
Другие ОС, 32-bit, C++, реальные IO/races и production integration не проверены.
Эти проверки не означают прохождение gate A3 или выпуск нового API.
