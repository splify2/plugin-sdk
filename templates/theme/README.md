# Шаблон: тема

Тема — только значения переменных панели: `--sp-*` и семейства `--an-*` ([docs/host-api.md](../../docs/host-api.md),
«Темы»). Кода нет, поэтому `arch: all`. `--sp-bleed`, `--sp-inset-l` и `--sp-inset-r` ставит сама панель —
`s2p lint` не даёт их задать.

```sh
s2p keygen -o dev
s2p build . --key dev.sec -o dist
s2p verify dist/example-theme-0.1.0-all.s2p --pub dev.pub
```
