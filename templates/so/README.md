# Шаблон: плагин `.so`

Код вида внутри процесса `steerd` по `steer-abi` v1. Берите `.so`, только когда отдельная программа не
подходит (правила в горячем пути, `emit`, `health` без fork): падение `.so` роняет `steerd`, а лицензия
плагина должна быть совместима с GPL-3.0 ядра. По умолчанию вид — программа ([../helper](../helper)).

```sh
s2p keygen -o dev
s2p build . --key dev.sec -o dist                   # .so под 10 архитектур (zig, musl)
s2p build . --arch aarch64_cortex-a53 --key dev.sec -o dist
s2p lint dist/example-so-0.1.0-aarch64_cortex-a53.s2p
```

`s2p lint` проверяет у `.so`: ровно один экспорт `steer_plugin_v1_entry`, нет `DT_NEEDED` на `libsteer*`,
машина ELF совпадает с архитектурой архива.
