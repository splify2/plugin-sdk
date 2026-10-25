# Шаблон: программа-помощник

Вид выхода `example`, который ведёт маленькая программа на C: супервизор steer запускает её на каждый
выход вида, она говорит с ядром строками событий (`evline`). Так устроены протоколы (`vless`,
`hysteria2`) и службы вроде recheck.

```sh
s2p keygen -o dev                                   # один раз: ключ для разработки
s2p build . --key dev.sec -o dist                   # .s2p под все архитектуры манифеста
s2p build . --arch x86_64 --key dev.sec -o dist     # одна архитектура
s2p verify dist/example-helper-0.1.0-x86_64.s2p --pub dev.pub
```

Нужен zig 0.13 (в `PATH` или `S2P_ZIG`) либо docker с образом `steer-builder:wolfssl`.

Своё имя: `s2p new helper ../myplug` — id берётся из имени каталога.

Что где: `plugin.json` — манифест, `s2p.json` — шаги сборки (в архив не попадает), `src/main.c` — программа,
`kinds/example.schema.json` — ключи вида.
