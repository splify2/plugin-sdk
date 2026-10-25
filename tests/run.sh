#!/bin/sh
# Стенд SDK плагинов: lint, build, sign, verify, index на шаблонах.
#
#   sh tests/run.sh          полный прогон; сборка C — x86_64, aarch64_cortex-a53, arm и mips
#   FULL=1 sh tests/run.sh   сборка C под все 10 архитектур
#   NO_UI=1 sh tests/run.sh  без npm (шаблон интерфейса и пакет plugin-api)
#
# Нужны: go, python3 (только стенд: враждебные архивы), file, tar, gzip; zig 0.13 или docker с образом
# steer-builder:wolfssl для сборки C; npm и сеть для интерфейса. Чего нет — тесты помечены SKIP.
set -u
HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(dirname "$HERE")
W=$(mktemp -d "${TMPDIR:-/tmp}/s2p-tests.XXXXXX")
trap 'rm -rf "$W"' EXIT
PASS=0; FAIL=0; SKIP=0
LOG="$W/log"

pass() { PASS=$((PASS + 1)); printf '  ok    %s\n' "$1"; }
fail() { FAIL=$((FAIL + 1)); printf '  FAIL  %s\n' "$1"; [ -f "$LOG" ] && sed 's/^/        | /' "$LOG" | tail -${2:-12}; }
skip() { SKIP=$((SKIP + 1)); printf '  SKIP  %s (%s)\n' "$1" "$2"; }
section() { printf '\n== %s\n' "$1"; }

# check ИМЯ КОМАНДА… — ноль
check() { n=$1; shift; if "$@" >"$LOG" 2>&1; then pass "$n"; else fail "$n"; fi; }
# refuse ИМЯ РЕГУЛЯРКА КОМАНДА… — не ноль, и вывод называет причину
refuse() {
    n=$1; re=$2; shift 2
    if "$@" >"$LOG" 2>&1; then fail "$n: команда должна была отказать"
    elif grep -Eq -e "$re" "$LOG"; then pass "$n"
    else fail "$n: отказ не по той причине (ждали /$re/)"; fi
}
same() { cmp -s "$1" "$2"; }

section "сборка s2p"
S="$W/s2p"
if (cd "$ROOT" && go build -o "$S" ./cmd/s2p && go vet ./... ) >"$LOG" 2>&1; then pass "go build, go vet"; else fail "go build"; cat "$LOG"; exit 1; fi
check "go test (semver, пути, подпись, архив)" sh -c "cd '$ROOT' && go test ./cmd/s2p"
cd "$W" || exit 1

have_cc=0
if command -v zig >/dev/null 2>&1 || [ -n "${S2P_ZIG:-}" ]; then have_cc=1
elif command -v docker >/dev/null 2>&1 && docker image inspect "${S2P_BUILDER_IMAGE:-steer-builder:wolfssl}" >/dev/null 2>&1; then have_cc=1; fi

T="$ROOT/templates"
"$S" keygen -o dev >/dev/null; "$S" keygen -o other >/dev/null; "$S" keygen -o idx >/dev/null; "$S" keygen -o rot >/dev/null

section "lint"
for t in helper so ui theme lang; do check "шаблон $t" "$S" lint "$T/$t"; done
for e in "$ROOT"/examples/*/plugin.json; do check "образец $(basename "$(dirname "$e")") по схеме" "$S" lint "$e"; done

mutate() {  # ИМЯ ШАБЛОН ПРАВКА-python — копия шаблона с правкой plugin.json
    rm -rf "m-$1"; cp -r "$T/$2" "m-$1"; python3 - "m-$1/plugin.json" "$3" <<'PY'
import json, sys
p = json.load(open(sys.argv[1])); exec(sys.argv[2]); json.dump(p, open(sys.argv[1], 'w'), ensure_ascii=False, indent=2)
PY
}
mutate nft helper "p['steer']['nft'] = ['nft/x.nft']"; mkdir -p m-nft/nft; printf 'chain forward {\n}\n' > m-nft/nft/x.nft
refuse "lint: файл nft без права nft" 'nft' "$S" lint m-nft
mutate unk helper "p['bogus_axis_ok'] = 1; p['compat'] = {}"
refuse "lint: пустой compat" 'compat' "$S" lint m-unk
mutate dd helper "p['steer']['kinds'][0]['keys'] = '../kinds/x.json'"
refuse "lint: .. в пути манифеста" 'шаблон|\.\.' "$S" lint m-dd
mutate noapi helper "p['compat'] = {'host-api': '^1.0'}"
refuse "lint: вид без оси steer-api" 'steer-api' "$S" lint m-noapi
mutate noprog helper "p['steer']['commands'][0]['prog'] = 'bin/missing'"
refuse "lint: нет файла программы" 'нет в плагине' "$S" lint m-noprog
mutate subst helper "p['steer']['kinds'][0]['helper']['argv'] = ['example', '{outt}']"
refuse "lint: неизвестная подстановка" 'подстановка' "$S" lint m-subst
mutate sul so "p['license'] = 'LicenseRef-proprietary'"
refuse "lint: .so с несовместимой лицензией" 'license' "$S" lint m-sul
mutate sox so "p['permissions'] = []"
refuse "lint: .so без права so" '"so"' "$S" lint m-sox
mutate keep helper "p['keep'] = ['/etc/example']"
refuse "lint: keep без права fs" 'fs:' "$S" lint m-keep
mutate theme-bad theme "p['themes'][0]['file'] = 'themes/dusk.json'"
python3 - <<'PY'
import json
f = 'm-theme-bad/themes/dusk.json'
t = json.load(open(f)); t['light']['--sp-bleed'] = '0'; t['light']['color'] = 'red'; json.dump(t, open(f, 'w'))
PY
refuse "lint: тема задаёт --sp-bleed и не переменную" 'не переменная|ставит сама' "$S" lint m-theme-bad
rm -rf m-exec; cp -r "$T/ui" m-exec; chmod -x m-exec/rpc/hello
refuse "lint: rpc не исполняемый" 'не исполняемый' "$S" lint m-exec
rm -rf m-lang; cp -r "$T/lang" m-lang; echo '{"a": 5}' > m-lang/i18n/host/de.json
refuse "lint: словарь с не строкой" 'не строка' "$S" lint m-lang
rm -rf m-bare; mkdir -p m-bare/ui; cp "$T/ui/plugin.json" m-bare/; cp -r "$T/ui/i18n" "$T/ui/rpc" m-bare/
printf 'import {h} from "preact";\nexport default {activate(){}}\n' > m-bare/ui/index.js; echo > m-bare/ui/style.css
refuse "lint: UI тянет preact отдельным import" 'preact' "$S" lint m-bare
if [ -z "${W##*/s2p-tests*}" ]; then :; fi

section "build: детерминизм и состав"
# theme и lang — без компилятора; подписанные и неподписанные
for t in theme lang; do
    check "build $t" "$S" build "$T/$t" --key dev.sec -o out1
    check "build $t второй раз" "$S" build "$T/$t" --key dev.sec -o out2
done
check "одинаковый вход — одинаковые байты (theme)" same out1/example-theme-0.1.0-all.s2p out2/example-theme-0.1.0-all.s2p
check "одинаковый вход — одинаковые байты (lang)" same out1/lang-de-26.11.0-all.s2p out2/lang-de-26.11.0-all.s2p
# смена каталога, прав владельца и времени не меняет байты
cp -r "$T/theme" theme-copy; touch -d '2001-01-01' theme-copy/plugin.json
"$S" build theme-copy --key dev.sec -o out3 >/dev/null 2>&1
check "другой каталог и время файлов — те же байты" same out1/example-theme-0.1.0-all.s2p out3/example-theme-0.1.0-all.s2p
check "gzip: целый поток" gzip -t out1/example-theme-0.1.0-all.s2p
tar tzvf out1/example-theme-0.1.0-all.s2p > tar.lst 2>&1
check "первые три записи: plugin.json, MANIFEST, MANIFEST.sig" sh -c "sed -n '1,3p' tar.lst | awk '{print \$NF}' | tr '\n' ' ' | grep -q '^plugin.json MANIFEST MANIFEST.sig \$'"
check "владелец 0/0, время 0" sh -c "! grep -v ' 0/0 ' tar.lst | grep -q . && ! grep -v '1970-01-01' tar.lst | grep -q ."
tar xzOf out1/example-theme-0.1.0-all.s2p MANIFEST > MANIFEST.txt
check "MANIFEST: заголовок и sha256 busybox/coreutils" sh -c "head -1 MANIFEST.txt | grep -q '^s2p 1 example-theme 0.1.0 all\$' && rm -rf x1 && mkdir x1 && tar xzf out1/example-theme-0.1.0-all.s2p -C x1 && tail -n +2 MANIFEST.txt | (cd x1 && while read -r sum size mode path; do echo \"\$sum  \$path\" | sha256sum -c - >/dev/null || exit 1; [ \"\$(wc -c < \"\$path\")\" = \"\$size\" ] || exit 1; done)"
refuse "build без ключа и без --unsigned" '--key' "$S" build "$T/theme" -o out4
check "build --unsigned" "$S" build "$T/theme" --unsigned -o out4
refuse "verify неподписанного" 'не подписан' "$S" verify out4/example-theme-0.1.0-all.s2p --pub dev.pub
check "verify неподписанного с --unsigned" "$S" verify out4/example-theme-0.1.0-all.s2p --unsigned

section "sign и verify"
A=out1/example-theme-0.1.0-all.s2p
check "verify своим ключом" "$S" verify $A --pub dev.pub
refuse "verify чужим ключом" 'такого ключа среди доверенных нет' "$S" verify $A --pub other.pub
refuse "verify без доверенных ключей" 'нет доверенных ключей' "$S" verify $A
# порча подписи: меняем один знак base64 внутри MANIFEST.sig, архив собираем заново
python3 - "$A" bad-sig.s2p <<'PY'
import gzip, io, sys, tarfile
raw = gzip.decompress(open(sys.argv[1], 'rb').read())
tf = tarfile.open(fileobj=io.BytesIO(raw)); ents = [(m, tf.extractfile(m).read() if m.isfile() else b'') for m in tf.getmembers()]
out = io.BytesIO()
with tarfile.open(fileobj=out, mode='w', format=tarfile.USTAR_FORMAT) as t:
    for m, d in ents:
        if m.name == 'MANIFEST.sig':
            lines = d.decode().split('\n'); s = list(lines[1]); i = 40; s[i] = 'A' if s[i] != 'A' else 'B'; lines[1] = ''.join(s); d = '\n'.join(lines).encode()
        t.addfile(m, io.BytesIO(d) if m.isfile() else None)
z = io.BytesIO()
with gzip.GzipFile(fileobj=z, mode='wb', mtime=0) as g: g.write(out.getvalue())
open(sys.argv[2], 'wb').write(z.getvalue())
PY
refuse "verify: испорченная подпись" 'не сходится' "$S" verify bad-sig.s2p --pub dev.pub
# подмена файла при той же подписи
python3 "$HERE/craft.py" "$A" craft >/dev/null
refuse "verify: подмена содержимого при старой подписи" 'sha256' "$S" verify craft/tampered-same-size.s2p --pub dev.pub
# ротация: два блока
cp $A rot.s2p
check "sign: второй ключ добавлен (--add)" "$S" sign rot.s2p --key rot.sec --add
check "ротация: старый ключ принимает" "$S" verify rot.s2p --pub dev.pub
check "ротация: новый ключ принимает" "$S" verify rot.s2p --pub rot.pub
check "ротация: оба в списке доверенных" "$S" verify rot.s2p --pub dev.pub --pub rot.pub
refuse "ротация: ни одного знакомого ключа" 'dev|такого ключа' "$S" verify rot.s2p --pub other.pub
grep -c 'untrusted comment' > /dev/null 2>&1 < /dev/null
tar xzOf rot.s2p MANIFEST.sig > rot.sig
check "MANIFEST.sig: два блока usign" sh -c "[ \$(grep -c '^untrusted comment:' rot.sig) -eq 2 ]"
printf '%s\n' "$(sed -n 's/.*key \(................\)$/\1/p' dev.pub | head -1)" > /dev/null
DEVNUM=$(sh -c "head -c 0 /dev/null; tar xzOf $A MANIFEST.sig | head -1 | sed 's/.* //'")
echo "$DEVNUM" > revoked
refuse "verify: ключ в списке отозванных" 'отозван' "$S" verify $A --pub dev.pub --revoked revoked
check "verify: --keys-dir по ролям" sh -c "mkdir -p keys/plugins keys/protocols && cp dev.pub keys/plugins/ && cp other.pub keys/protocols/ && '$S' verify $A --keys-dir keys --role plugins && ! '$S' verify $A --keys-dir keys --role protocols"
check "sign: перезапись подписи (--key) заменяет блоки" sh -c "cp $A re.s2p && '$S' sign re.s2p --key other.sec && '$S' verify re.s2p --pub other.pub && ! '$S' verify re.s2p --pub dev.pub"
check "sign: байты MANIFEST.sig не зависят от времени (Ed25519 детерминирован)" sh -c "cp $A r1.s2p && cp $A r2.s2p && '$S' sign r1.s2p --key other.sec && '$S' sign r2.s2p --key other.sec && cmp r1.s2p r2.s2p"
if command -v openssl >/dev/null 2>&1; then
    # независимая проверка формата: подпись и ключ — настоящий Ed25519 (RFC 8032), без нашего кода
    python3 - "$A" dev.pub <<'PY'
import base64, gzip, io, sys, tarfile
raw = gzip.decompress(open(sys.argv[1], 'rb').read()); tf = tarfile.open(fileobj=io.BytesIO(raw))
open('o.manifest', 'wb').write(tf.extractfile('MANIFEST').read())
sig = base64.b64decode(tf.extractfile('MANIFEST.sig').read().decode().split('\n')[1]); assert sig[:2] == b'Ed' and len(sig) == 74
open('o.sig', 'wb').write(sig[10:])
pub = base64.b64decode(open(sys.argv[2]).read().split('\n')[1]); assert pub[:2] == b'Ed' and len(pub) == 42
open('o.pub.der', 'wb').write(bytes.fromhex('302a300506032b6570032100') + pub[10:])
PY
    check "openssl подтверждает подпись MANIFEST (Ed25519, формат usign)" openssl pkeyutl -verify -pubin -inkey o.pub.der -keyform DER -rawin -in o.manifest -sigfile o.sig
else skip "interop с openssl" "нет openssl"; fi
if command -v usign >/dev/null 2>&1; then
    tar xzOf $A MANIFEST > u.manifest; tar xzOf $A MANIFEST.sig > u.sig
    check "usign -V подтверждает подпись" usign -V -p dev.pub -m u.manifest -x u.sig
else skip "interop с usign" "usign не установлен"; fi

section "враждебные архивы"
check "годный архив для основы принят" "$S" verify "$A" --pub dev.pub
while IFS="$(printf '\t')" read -r f frag; do
    refuse "$f" "$frag" "$S" verify "craft/$f" --pub dev.pub
done < craft/variants.txt
for f in craft/path-*; do
    refuse "lint тоже отказывает: $(basename "$f")" '.' "$S" lint "$f"
done

section "сборка C-плагинов (zig, musl)"
ARCHES="x86_64 aarch64_cortex-a53 arm_cortex-a9 mipsel_24kc"
[ "${FULL:-}" = 1 ] && ARCHES=$("$S" arches | awk '{print $1}')
elf_expect() {  # арх -> шаблон для file
    case $1 in
        x86_64) echo 'x86-64' ;;
        aarch64*) echo 'ARM aarch64' ;;
        arm_*) echo 'ARM, EABI5' ;;
        mipsel*) echo 'MIPS, MIPS32 rel2' ;;
        mips_*) echo 'MIPS, MIPS32 rel2' ;;
    esac
}
if [ $have_cc = 0 ]; then
    skip "сборка helper и so" "нет zig и нет docker-образа steer-builder:wolfssl"
else
    rm -rf cc; mkdir cc
    for a in $ARCHES; do
        check "so [$a]: build" "$S" build "$T/so" --arch "$a" --key dev.sec -o cc
        f=cc/example-so-0.1.0-$a.s2p
        rm -rf "ex-so-$a"; mkdir "ex-so-$a"; tar xzf "$f" -C "ex-so-$a" 2>/dev/null
        so="ex-so-$a/lib/example-so.so"
        want=$(elf_expect "$a")
        check "so [$a]: file — разделяемая библиотека $want" sh -c "file '$so' | grep -q 'shared object' && file '$so' | grep -q '$want'"
        check "so [$a]: verify" "$S" verify "$f" --pub dev.pub
        if command -v readelf >/dev/null 2>&1; then
            check "so [$a]: нет DT_NEEDED libsteer*, SONAME example-so.so" sh -c "! readelf -d '$so' | grep NEEDED | grep -q libsteer && readelf -d '$so' | grep -q 'SONAME.*example-so.so'"
            check "so [$a]: единственный экспорт steer_plugin_v1_entry" sh -c "[ \"\$(readelf --dyn-syms -W '$so' | awk '\$7 != \"UND\" && \$5 != \"LOCAL\" && NR > 3 {print \$8}')\" = steer_plugin_v1_entry ]"
        fi
        check "helper [$a]: build" "$S" build "$T/helper" --arch "$a" --key dev.sec -o cc
        h=cc/example-helper-0.1.0-$a.s2p
        rm -rf "ex-h-$a"; mkdir "ex-h-$a"; tar xzf "$h" -C "ex-h-$a" 2>/dev/null
        check "helper [$a]: file — статический исполняемый $want" sh -c "file 'ex-h-$a/bin/example-helper' | grep -q 'statically linked' && file 'ex-h-$a/bin/example-helper' | grep -q '$want'"
    done
    check "so: x86_64 и aarch64_cortex-a53 — обязательные цели собраны" sh -c "[ -f cc/example-so-0.1.0-x86_64.s2p ] && [ -f cc/example-so-0.1.0-aarch64_cortex-a53.s2p ]"
    # исполняем: x86_64 — прямо, aarch64 — под qemu-user
    check "helper x86_64: подкоманда и evline hello" sh -c "ex-h-x86_64/bin/example-helper example-hello | grep -q 'example-helper 0.1.0' && echo '{}' > c.json && (timeout 2 ex-h-x86_64/bin/example-helper example o --conf c.json 3>ev.out 2>/dev/null; true) && head -1 ev.out | grep -q '^{\"ev\":\"hello\",\"api\":\"1.0\",\"ver\":\"0.1.0\",\"mod\":\"example-helper\"}\$' && sed -n 2p ev.out | grep -q '\"ev\":\"up\"'"
    if command -v qemu-aarch64 >/dev/null 2>&1; then
        check "helper aarch64_cortex-a53 под qemu-user: подкоманда" sh -c "qemu-aarch64 ex-h-aarch64_cortex-a53/bin/example-helper example-hello | grep -q 'example-helper 0.1.0'"
    else skip "helper под qemu-user" "нет qemu-aarch64"; fi
    # повторяемость машинного кода: вторая сборка того же дерева — те же байты
    "$S" build "$T/helper" --arch x86_64 --key dev.sec -o cc2 >/dev/null 2>&1
    check "helper x86_64: пересборка даёт тот же архив" same cc/example-helper-0.1.0-x86_64.s2p cc2/example-helper-0.1.0-x86_64.s2p
    "$S" build "$T/so" --arch aarch64_cortex-a53 --key dev.sec -o cc2 >/dev/null 2>&1
    check "so aarch64_cortex-a53: пересборка даёт тот же архив" same cc/example-so-0.1.0-aarch64_cortex-a53.s2p cc2/example-so-0.1.0-aarch64_cortex-a53.s2p
    # lint ловит чужую архитектуру: архив для aarch64, а в манифесте x86_64 нет
    refuse "build: архитектура не из манифеста" 'не названа' "$S" build "$T/so" --arch riscv64 --key dev.sec -o cc
    # s2p cc напрямую: два варианта
    check "s2p cc --kind so" "$S" cc --arch x86_64 --kind so -o direct/x.so "$T/so/src/example.c"
    check "s2p cc --kind helper --evline" "$S" cc --arch aarch64_cortex-a53 --kind helper --evline -o direct/h "$T/helper/src/main.c"
    check "s2p cc --kind lib (libs2p-evline.a)" "$S" cc --arch x86_64 --kind lib --evline -o direct/libs2p-evline.a
    check "s2p cc --arch all: 10 файлов" sh -c "'$S' cc --arch all --kind helper --evline -o 'direct/all/%a/h' '$T/helper/src/main.c' >/dev/null && [ \$(ls direct/all | wc -l) -eq 10 ]"
    check "libs2p-evline.a — архив ar" sh -c "file direct/libs2p-evline.a | grep -q 'ar archive'"
    # lint машинного кода: DT_NEEDED на libsteer отвергается, на libs2p-* — только при зависимости и с RUNPATH
    mkdir -p stub; printf '__attribute__((visibility("default"))) const void *steer_plugin_v1_entry(unsigned short a, unsigned short b) { (void)a; (void)b; return 0; }\n' > stub/stub.c
    check "заглушки libsteer.so.2 и libs2p-net.so.1 для проверок lint" sh -c "'$S' cc --arch x86_64 --kind so -o stub/libsteer.so.2 stub/stub.c && '$S' cc --arch x86_64 --kind so -o stub/libs2p-net.so.1 stub/stub.c" 
    rm -rf m-needs; cp -r "$T/so" m-needs; cp stub/libsteer.so.2 m-needs/
    python3 - m-needs/s2p.json <<'PY'
import json, sys
c = json.load(open(sys.argv[1])); c['cc'][0]['link'] = ['libsteer.so.2']; json.dump(c, open(sys.argv[1], 'w'))
PY
    refuse "build: .so с DT_NEEDED на libsteer отвергнут" 'DT_NEEDED libsteer' "$S" build m-needs --arch x86_64 --key dev.sec -o cc3
    rm -rf m-net; cp -r "$T/helper" m-net; cp stub/libs2p-net.so.1 m-net/
    python3 - m-net <<'PY'
import json, sys
d = sys.argv[1]
c = json.load(open(d + '/s2p.json')); c['cc'][0]['link'] = ['libs2p-net.so.1']; c['cc'][0]['requires'] = ['net1']; json.dump(c, open(d + '/s2p.json', 'w'))
p = json.load(open(d + '/plugin.json')); p['requires'] = {'plugins': {'net1': '^1.0'}}; json.dump(p, open(d + '/plugin.json', 'w'), ensure_ascii=False)
PY
    check "build: помощник на libs2p-net.so.1 с зависимостью net1 проходит" "$S" build m-net --arch x86_64 --key dev.sec -o cc3
    if command -v readelf >/dev/null 2>&1; then
        rm -rf ex-net; mkdir ex-net; tar xzf cc3/example-helper-0.1.0-x86_64.s2p -C ex-net 2>/dev/null
        check "RUNPATH \$ORIGIN/../../net1/lib и загрузчик musl" sh -c "readelf -d ex-net/bin/example-helper | grep -F 'RUNPATH' | grep -qF '\$ORIGIN/../../net1/lib' && readelf -d ex-net/bin/example-helper | grep -q 'libs2p-net.so.1' && readelf -l ex-net/bin/example-helper | grep -q '/lib/ld-musl-x86_64.so.1'"
    fi
    python3 - m-net/plugin.json <<'PY'
import json, sys
p = json.load(open(sys.argv[1])); del p['requires']; json.dump(p, open(sys.argv[1], 'w'), ensure_ascii=False)
PY
    refuse "build: libs2p-* без requires.plugins отвергнут" 'requires.plugins' "$S" build m-net --arch x86_64 --key dev.sec -o cc4
    python3 - m-net <<'PY'
import json, sys
d = sys.argv[1]
p = json.load(open(d + '/plugin.json')); p['requires'] = {'plugins': {'net1': '^1.0'}}; json.dump(p, open(d + '/plugin.json', 'w'), ensure_ascii=False)
c = json.load(open(d + '/s2p.json')); c['cc'][0]['requires'] = []; json.dump(c, open(d + '/s2p.json', 'w'))
PY
    refuse "build: libs2p-* без RUNPATH отвергнут" 'RUNPATH' "$S" build m-net --arch x86_64 --key dev.sec -o cc4
fi

section "index"
# несколько плагинов, несколько версий, несколько архитектур
rm -rf repo-in; mkdir repo-in
cp out1/*.s2p repo-in/
v2() {  # шаблон новая-версия — копия шаблона с другой версией
    rm -rf "v-$1-$2"; cp -r "$T/$1" "v-$1-$2"
    python3 - "v-$1-$2/plugin.json" "$2" <<'PY'
import json, sys
p = json.load(open(sys.argv[1])); p['version'] = sys.argv[2]; json.dump(p, open(sys.argv[1], 'w'), ensure_ascii=False, indent=2)
PY
}
v2 theme 0.2.0; v2 theme 0.3.0
"$S" build v-theme-0.2.0 --key dev.sec -o repo-in >/dev/null 2>&1
"$S" build v-theme-0.3.0 --key dev.sec -o repo-in >/dev/null 2>&1
if [ $have_cc = 1 ]; then cp cc/example-helper-0.1.0-x86_64.s2p cc/example-helper-0.1.0-aarch64_cortex-a53.s2p cc/example-so-0.1.0-x86_64.s2p cc/example-so-0.1.0-aarch64_cortex-a53.s2p repo-in/; fi
cat > meta.json <<'JSON'
{ "plugins": { "lang-de": { "key_role": "plugins", "tags": ["language"] },
               "example-theme": { "key_role": "plugins", "tags": ["theme"], "changelog": { "0.3.0": "новая палитра" } } } }
JSON
IDX="--repo testrepo --name 'Тестовый' --mirror https://example.invalid/plugins/ --pub dev.pub --meta meta.json --expires 30d"
eval "$S" index repo-in -o repo $IDX --key idx.sec --now 2026-11-02T10:00:00Z > "$LOG" 2>&1 && pass "index: собран" || fail "index: собран"
check "index.json и index.json.sig на месте, архивы разложены <id>/<версия>/" sh -c "[ -f repo/index.json ] && [ -f repo/index.json.sig ] && [ -f repo/example-theme/0.3.0/example-theme-0.3.0-all.s2p ]"
check "verify index ключом index" "$S" verify repo/index.json --pub idx.pub --now 2026-11-03T00:00:00Z
check "verify index: архивы лежат и сходятся по sha256 и размеру" "$S" verify repo/index.json --pub idx.pub --base repo --now 2026-11-03T00:00:00Z
refuse "verify index ключом архивов (чужой ключ)" 'такого ключа среди доверенных нет' "$S" verify repo/index.json --pub dev.pub
refuse "verify index: срок вышел" 'срок индекса вышел' "$S" verify repo/index.json --pub idx.pub --now 2027-01-01T00:00:00Z
cp repo/index.json tampered.json; cp repo/index.json.sig tampered.json.sig
sed -i 's/"0.3.0"/"9.9.9"/' tampered.json
refuse "verify index: правка индекса при старой подписи" 'не сходится' "$S" verify tampered.json --pub idx.pub
check "index: версии — три у example-theme, по плагину на каждый id" python3 "$HERE/jsonassert.py" repo/index.json "len(d['plugins']['example-theme']['versions']) == 3 and set(d['plugins']) >= {'example-theme','lang-de'}"
check "index: у новейшей версии changelog и released" python3 "$HERE/jsonassert.py" repo/index.json "d['plugins']['example-theme']['versions'][0]['version'] == '0.3.0' and d['plugins']['example-theme']['versions'][0]['changelog'] == 'новая палитра' and 'released' in d['plugins']['example-theme']['versions'][0]"
check "index: assets.all с path, sha256, size, installed" python3 "$HERE/jsonassert.py" repo/index.json "all(set(a) >= {'path','sha256','size','installed'} for p in d['plugins'].values() for v in p['versions'] for a in v['assets'].values()) and 'all' in d['plugins']['lang-de']['versions'][0]['assets']"
check "index: provides.languages и key_role из meta" python3 "$HERE/jsonassert.py" repo/index.json "d['plugins']['lang-de']['versions'][0]['provides']['languages'] == ['de'] and d['plugins']['lang-de']['key_role'] == 'plugins' and d['plugins']['example-theme']['tags'] == ['theme']"
if [ $have_cc = 1 ]; then
    check "index: две архитектуры у example-helper и example-so; provides.kinds" python3 "$HERE/jsonassert.py" repo/index.json "set(d['plugins']['example-helper']['versions'][0]['assets']) == {'x86_64','aarch64_cortex-a53'} and d['plugins']['example-so']['versions'][0]['provides']['kinds'] == ['example-so']"
fi
# детерминизм индекса
eval "$S" index repo-in -o repo2 $IDX --key idx.sec --now 2026-11-02T10:00:00Z > "$LOG" 2>&1
check "index: тот же вход — те же байты index.json и .sig" sh -c "cmp repo/index.json repo2/index.json && cmp repo/index.json.sig repo2/index.json.sig"
# слияние: новая версия поверх прежнего индекса, прежние остаются
v2 theme 0.4.0; "$S" build v-theme-0.4.0 --key dev.sec -o repo-new >/dev/null 2>&1
eval "$S" index repo-new -o repo $IDX --key idx.sec --now 2026-11-09T10:00:00Z > "$LOG" 2>&1 && pass "index: слияние с прежним (-o уже с index.json)" || fail "index: слияние"
check "index: версии 0.1.0–0.4.0 все на месте" python3 "$HERE/jsonassert.py" repo/index.json "[v['version'] for v in d['plugins']['example-theme']['versions']] == ['0.4.0','0.3.0','0.2.0','0.1.0']"
check "index: плагины, которых нет во входе, не пропали" python3 "$HERE/jsonassert.py" repo/index.json "'lang-de' in d['plugins'] and d['generated'] == '2026-11-09T10:00:00Z'"
refuse "index: время сборки раньше прежнего индекса" 'раньше прежнего' sh -c "$S index repo-new -o repo $IDX --key idx.sec --now 2026-11-01T00:00:00Z"
# та же версия с другим содержимым
v2 theme 0.4.0; echo '{"name":"X","light":{"--sp-card":"#000"}}' > v-theme-0.4.0-b.json; cp v-theme-0.4.0-b.json v-theme-0.4.0/themes/dusk.json
"$S" build v-theme-0.4.0 --key dev.sec -o repo-conflict >/dev/null 2>&1
refuse "index: та же версия, другие байты — отказ" 'другой суммой' sh -c "$S index repo-conflict -o repo $IDX --key idx.sec --now 2026-11-10T00:00:00Z"
# отзыв и удаление
rm -rf repo-empty; mkdir repo-empty
eval "$S" index repo-empty -o repo $IDX --key idx.sec --now 2026-11-10T10:00:00Z --yank example-theme@0.2.0 --why "'палитра не читалась'" --security --drop example-theme@0.1.0 > "$LOG" 2>&1 && pass "index: --yank и --drop" || fail "index: --yank и --drop"
check "index: yanked с причиной и security, 0.1.0 убрана" python3 "$HERE/jsonassert.py" repo/index.json "[v['version'] for v in d['plugins']['example-theme']['versions']] == ['0.4.0','0.3.0','0.2.0'] and d['plugins']['example-theme']['versions'][2]['yanked'] == {'why': 'палитра не читалась', 'security': True}"
# архив чужого ключа и неподписанный в индекс не попадают
mkdir -p repo-foreign; "$S" build "$T/lang" --key other.sec -o repo-foreign >/dev/null 2>&1
refuse "index: архив подписан чужим ключом" 'такого ключа среди доверенных нет' sh -c "$S index repo-foreign -o repo-x $IDX --key idx.sec"
mkdir -p repo-unsigned; "$S" build "$T/lang" --unsigned -o repo-unsigned >/dev/null 2>&1
refuse "index: неподписанный архив" 'не подписан' sh -c "$S index repo-unsigned -o repo-x $IDX --key idx.sec"
refuse "index: без ключа индекса" 'нужны' sh -c "$S index repo-in -o repo-x --repo r --pub dev.pub"

section "s2p new"
for t in helper so ui theme lang; do
    id="my-$t"; [ $t = lang ] && id=lang-fr
    check "new $t" "$S" new "$t" "n-$id" --id "$id"
    check "new $t: lint" "$S" lint "n-$id"
done
check "new: id подставлен в манифест и исходники" sh -c "grep -q '\"my-helper\"' n-my-helper/plugin.json && grep -q 'my-helper' n-my-helper/s2p.json && grep -q 'my_helper\\|my-helper' n-my-helper/src/main.c"
check "new lang: код языка в манифесте и имени файла" sh -c "[ -f n-lang-fr/i18n/host/fr.json ] && grep -q '\"fr\"' n-lang-fr/plugin.json && ! grep -q '\"de\"' n-lang-fr/plugin.json"
check "new: права на rpc/hello" sh -c "[ -x n-my-ui/rpc/hello ]"
refuse "new: каталог уже есть" 'уже есть' "$S" new theme n-my-theme
refuse "new: плохой id" 'нужно' "$S" new theme bad --id 'Bad_ID'
check "new theme → build → verify" sh -c "'$S' build n-my-theme --key dev.sec -o nb >/dev/null 2>&1 && '$S' verify nb/my-theme-0.1.0-all.s2p --pub dev.pub"
check "new lang → build → verify" sh -c "'$S' build n-lang-fr --key dev.sec -o nb >/dev/null 2>&1 && '$S' verify nb/lang-fr-26.11.0-all.s2p --pub dev.pub"
if [ $have_cc = 1 ]; then
    check "new helper → build (x86_64) → verify" sh -c "'$S' build n-my-helper --arch x86_64 --key dev.sec -o nb >/dev/null 2>&1 && '$S' verify nb/my-helper-0.1.0-x86_64.s2p --pub dev.pub"
fi

section "UI: шаблон и @splify2/plugin-api"
if [ "${NO_UI:-}" = 1 ] || ! command -v npm >/dev/null 2>&1; then
    skip "шаблон ui и пакет plugin-api" "NO_UI=1 или нет npm"
else
    PA="$ROOT/packages/plugin-api"
    rm -rf pa && mkdir pa && cp -r "$PA/src" "$PA/scripts" "$PA/package.json" "$PA/tsconfig.json" "$PA/README.md" "$PA/LICENSE" pa/
    if (cd pa && npm install --no-audit --no-fund && npm run build && npm test) >"$LOG" 2>&1; then pass "plugin-api: npm run build, npm test"; else fail "plugin-api: сборка"; fi
    check "plugin-api: npm pack — dist, README, LICENSE, package.json и больше ничего" sh -c "cd pa && npm pack --dry-run --json 2>/dev/null | python3 -c \"import json,sys; f=sorted(x['path'] for x in json.load(sys.stdin)[0]['files']); print(f); sys.exit(0 if f==['LICENSE','README.md','dist/index.d.ts','dist/index.js','dist/vite.d.ts','dist/vite.js','package.json'] else 1)\""
    rm -rf ui-t && "$S" new ui ui-t --id ui-t >/dev/null
    sed -i "s|\"@splify2/plugin-api\": \"^1.0.0\"|\"@splify2/plugin-api\": \"file:$W/pa\"|" ui-t/package.json
    if (cd ui-t && npm install --no-audit --no-fund && npm run check) >"$LOG" 2>&1; then pass "ui: npm install, tsc (типы @splify2/plugin-api)"; else fail "ui: npm install и tsc"; fi
    check "ui: s2p build" "$S" build ui-t --key dev.sec -o uib
    check "ui: verify" "$S" verify uib/ui-t-0.1.0-all.s2p --pub dev.pub
    tar xzOf uib/ui-t-0.1.0-all.s2p ui/index.js > ui.js
    check "ui: бандл без отдельного preact, с globalThis.__splify2_host" sh -c "! grep -Eq 'from *[\"'\\'']preact|from *[\"'\\'']react' ui.js && grep -q '__splify2_host' ui.js && grep -Eq 'export *\\{|export default' ui.js"
    check "ui: ui/style.css в архиве" sh -c "tar tzf uib/ui-t-0.1.0-all.s2p | grep -q '^ui/style.css\$'"
    check "ui: бандл исполняется на подставном хосте" node --input-type=module -e "
globalThis.__splify2_host = { preact: { h: (...a) => a, Fragment: 'F', render() {}, hooks: { useState: v => [v, () => {}] } } };
const m = await import('file://$W/ui.js'); let slot; m.default.activate({ slots: { pluginSettings: s => { slot = s } }, i18n: { t: k => k }, ui: new Proxy({}, { get: () => 'C' }), plugin: { id: 'ui-t', version: '0.1.0' }, rpc: {}, notify() {} });
if (slot.summary().text !== 'summary') process.exit(1); if (!slot.render({ close() {} })) process.exit(1)"
fi

printf '\n== итог: %d прошло, %d упало, %d пропущено\n' "$PASS" "$FAIL" "$SKIP"
[ "$FAIL" -eq 0 ]
