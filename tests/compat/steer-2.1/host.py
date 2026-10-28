#!/usr/bin/env python3
"""Заглушка ядра steer 2.1 (steer-api 1.3) — тест совместимости плагина со стороной ядра.

    sh tests/compat/run.sh --steer <каталог плагина> [2.1]

Каталог плагина — как в раскладке роутера: plugin.json, схемы ключей видов и собранные программы
(bin/<id>; программе протокола нужна net1 — у раскладки она рядом, ../net1/lib, по RUNPATH). Заглушка
делает то, что делает ядро (docs/steer-api.md, docs/steer-api-1.1.md), и проверяет ответ программы:

  1. compat.steer-api манифеста принимает 1.1;
  2. хук check вида — вход по steer-api 3.2 с ключами из схемы (x-example или простое значение по типу):
     код 0 и пустой stdout;
  3. хук expand — код 0 и JSON с объектом outputs;
  4. помощник вида — {conf} выхода по steer-api 1.4, первое сообщение в STEER_EVENT_FD — hello с api
     того же major и minor не новее 1.1, mod — id плагина; процесс гаснет по SIGTERM;
  5. подкоманды с conf: true — программа принимает --conf (не отвечает отказом про флаг, не 127);
  6. службы steer.services[] (steer-api 1.2) — программа с argv манифеста и выданными ресурсами ({queue0},
     {mark}…), первое событие hello (api не новее 1.2, mod — id плагина), живёт до SIGTERM и гаснет по нему;
  7. пути в системе steer.links (steer-api 1.3) — как их ставит менеджер: path абсолютный и под правом fs:,
     не путь ядра; цель — файл плагина (у программ по ссылке — исполняемый); ссылка, поставленная в
     подставной корень, ведёт в файл плагина, а программа по ней запускается (у link в /usr/bin — с
     `--help` или `version`, код не 126/127).

Код 0 — плагин подключится к ядру 2.1; иначе — строки FAIL с причиной.
"""
import json
import os
import select
import signal
import subprocess
import sys
import tempfile
import time

API = (1, 3)
ABI = (1, 0)  # steer-abi: ось плагинов-.so (docs/steer-abi.md)
HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, ".."))
from semver import range_has  # noqa: E402
fails = []
passes = 0


def ok(what, cond, why=""):
    global passes
    if cond:
        passes += 1
        print("ok   " + what)
    else:
        fails.append(what)
        print("FAIL " + what + (("\n     " + why) if why else ""))


def range_ok(rng, have=None):
    return range_has(rng, have or (API[0], API[1], 0))


def sample(name, sch, tmp):
    """Значение ключа по схеме: x-example, первое из enum, файл для x-path, простое по типу."""
    if "x-example" in sch:
        return sch["x-example"]
    if "default" in sch:
        return sch["default"]
    if "enum" in sch:
        return sch["enum"][0]
    t = sch.get("type")
    if isinstance(t, list):
        t = t[0]
    if sch.get("x-path") or name in ("subscription", "conf", "file"):
        p = os.path.join(tmp, name + ".txt")
        with open(p, "w") as f:
            f.write("")
        return p
    if t == "integer":
        return max(sch.get("minimum", 1), 1)
    if t == "boolean":
        return False
    if t == "array":
        return []
    if t == "object":
        return {}
    return "x"


def keys_of(plugin_dir, kind, tmp):
    rel = kind.get("keys")
    if not rel:
        return {}
    with open(os.path.join(plugin_dir, rel)) as f:
        sch = json.load(f)
    props = sch.get("properties", {})
    out = {}
    for k in sch.get("required", []):
        out[k] = sample(k, props.get(k, {}), tmp)
    return out


def env_for(pid, plugin_dir, state):
    e = dict(os.environ)
    e.update({
        "STEER_API": "%d.%d" % API, "STEER_CORE_VERSION": "2.1.0", "STEER_PLUGIN_ID": pid,
        "STEER_PLUGIN_VERSION": "0", "STEER_PLUGIN_DIR": plugin_dir, "STEER_PLUGIN_STATE": state,
        "STEER_PLUGIN_CONF": state, "STEER_STATE_DIR": state, "STEER_ETC_DIR": state,
        "STEER_PLATFORM": "openwrt",
    })
    return e


def prog_argv(plugin_dir, p, subst):
    argv = [os.path.join(plugin_dir, p["prog"])]
    for a in p.get("argv", []):
        for k, v in subst.items():
            a = a.replace("{" + k + "}", v)
        argv.append(a)
    return argv


def conf_json(pid, out, kind, keys, state, members=None):
    o = {
        "name": out, "kind": kind, "keys": keys, "device": out[:15], "mark": "0x00300000",
        "mark_mask": "0x0ff00000", "table": 302, "ipv6": False, "on_fail": "drop", "over": None,
        "sock_mark": "0x00000000", "sock_mark_required": False, "tcp_port": 0, "ports": [], "queues": [],
    }
    if members is not None:
        o["members"] = members
    return {"api": "%d.%d" % API, "plugin": {"id": pid, "version": "0"}, "output": o,
            "paths": {"etc": state, "state": state, "plugin_state": state, "fakeip": os.path.join(state, "fakeip")},
            "platform": "openwrt"}


def main():
    if len(sys.argv) < 2 or not os.path.isdir(sys.argv[1]):
        print("использование: host.py <каталог плагина>", file=sys.stderr)
        return 2
    d = os.path.abspath(sys.argv[1])
    with open(os.path.join(d, "plugin.json")) as f:
        m = json.load(f)
    pid = m["id"]
    print("steer-2.1: плагин %s %s" % (pid, m.get("version", "?")))
    rng = m.get("compat", {}).get("steer-api", "")
    ok("compat.steer-api «%s» принимает %d.%d" % (rng, API[0], API[1]), bool(rng) and range_ok(rng))
    if "steer-abi" in m.get("compat", {}):
        ok("compat.steer-abi «%s» принимает %d.%d" % (m["compat"]["steer-abi"], ABI[0], ABI[1]), range_ok(m["compat"]["steer-abi"], ABI))
    st = m.get("steer", {})
    tmp = tempfile.mkdtemp(prefix="steer21-")
    for kind in st.get("kinds", []):
        kname = kind["name"]
        keys = keys_of(d, kind, tmp)
        hook_in = json.dumps({"api": "%d.%d" % API, "output": {"name": "t1", "kind": kname, "keys": keys, "common": {}},
                              "spec": {"etc": tmp, "outputs": ["t1"]}, "platform": "openwrt"})
        env = env_for(pid, d, tmp)
        for hook in ("check", "expand"):
            h = kind.get(hook)
            if not h:
                continue
            r = subprocess.run(prog_argv(d, h, {"out": "t1"}), input=hook_in.encode(), capture_output=True,
                               env=env, timeout=h.get("timeout_s", 10))
            err = r.stderr.decode(errors="replace").strip()
            if hook == "check":
                ok("вид %s: хук check принимает годные ключи %s" % (kname, json.dumps(keys, ensure_ascii=False)),
                   r.returncode == 0 and not r.stdout.strip(), "код %d: %s" % (r.returncode, err))
            else:
                try:
                    res = json.loads(r.stdout.decode() or "null")
                except ValueError:
                    res = None
                ok("вид %s: хук expand отдаёт outputs" % kname,
                   r.returncode == 0 and isinstance(res, dict) and isinstance(res.get("outputs"), dict),
                   "код %d: %s" % (r.returncode, err or r.stdout.decode(errors="replace")[:200]))
        hp = kind.get("helper")
        if not hp or kind.get("impl") == "core":
            continue
        cpath = os.path.join(tmp, "t1.json")
        with open(cpath, "w") as f:
            json.dump(conf_json(pid, "t1", kname, keys, tmp, [] if kind.get("expand") else None), f)
        argv = prog_argv(d, hp, {"out": "t1", "conf": cpath})
        if not hp.get("argv"):
            argv += [hp.get("cmd", kname), "t1", "--conf", cpath]
        rfd, wfd = os.pipe()
        e = dict(env, STEER_EVENT_FD=str(wfd), STEER_SUPD="1")
        p = subprocess.Popen(argv, env=e, pass_fds=(wfd,), stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
        os.close(wfd)
        line = b""
        end = time.time() + 5
        while time.time() < end and not line.endswith(b"\n"):
            rr, _, _ = select.select([rfd], [], [], max(0.0, end - time.time()))
            if not rr:
                break
            ch = os.read(rfd, 1)
            if not ch:
                break
            line += ch
        try:
            hello = json.loads(line.decode())
        except ValueError:
            hello = {}
        api = str(hello.get("api", ""))
        try:
            ma, mi = (int(x) for x in api.split(".")[:2])
        except ValueError:
            ma, mi = -1, -1
        ok("вид %s: помощник говорит hello первым (steer-api 2.1)" % kname, hello.get("ev") == "hello",
           "первая строка: %r" % line[:200])
        ok("  api %s — major %d, minor не новее %d" % (api, API[0], API[1]), ma == API[0] and 0 <= mi <= API[1])
        ok("  mod — id плагина (%s)" % pid, hello.get("mod") == pid, "mod: %r" % hello.get("mod"))
        if p.poll() is None:
            p.send_signal(signal.SIGTERM)
        try:
            p.wait(timeout=5)
            ok("  гаснет по SIGTERM", True)
        except subprocess.TimeoutExpired:
            p.kill()
            ok("  гаснет по SIGTERM", False, "не погас за 5 с")
        os.close(rfd)
    for sv in st.get("services", []):
        subst = {"plugin_dir": d, "plugin_conf": tmp, "plugin_state": tmp, "mark": "0x80000000", "mark0": "0x80000000",
                 "mark1": "0x40000000", "queue0": "4300", "queue1": "4301", "port0": "10480", "port1": "10481"}
        argv = prog_argv(d, sv, subst)
        if not sv.get("argv"):
            argv.append(sv["name"])
        rfd, wfd = os.pipe()
        e = env_for(pid, d, tmp)
        for item in (sv.get("env") or {}).items() if isinstance(sv.get("env"), dict) else [x.split("=", 1) for x in (sv.get("env") or [])]:
            e[item[0]] = item[1]
        e.update(STEER_EVENT_FD=str(wfd), STEER_SUPD="1")
        p = subprocess.Popen(argv, env=e, pass_fds=(wfd,), stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
        os.close(wfd)
        line = b""
        end = time.time() + sv.get("timeout_s", 10)
        while time.time() < end and not line.endswith(b"\n"):
            rr, _, _ = select.select([rfd], [], [], max(0.0, end - time.time()))
            if not rr:
                break
            ch = os.read(rfd, 1)
            if not ch:
                break
            line += ch
        try:
            hello = json.loads(line.decode())
        except ValueError:
            hello = {}
        api = str(hello.get("api", ""))
        try:
            ma, mi = (int(x) for x in api.split(".")[:2])
        except ValueError:
            ma, mi = -1, -1
        ok("служба %s: hello первым (steer-api 2.1)" % sv["name"], hello.get("ev") == "hello",
           "первая строка: %r, stderr: %s" % (line[:200], p.stderr.read(300).decode(errors="replace") if p.poll() is not None else ""))
        ok("  api %s — major %d, minor не новее %d" % (api, API[0], API[1]), ma == API[0] and 0 <= mi <= API[1])
        ok("  mod — id плагина (%s)" % pid, hello.get("mod") == pid, "mod: %r" % hello.get("mod"))
        time.sleep(0.3)
        ok("  служба живёт после hello (её не должно быть «выполнил и вышел»)", p.poll() is None,
           "код выхода %s" % p.poll())
        if p.poll() is None:
            p.send_signal(signal.SIGTERM)
        try:
            p.wait(timeout=5)
            ok("  гаснет по SIGTERM", True)
        except subprocess.TimeoutExpired:
            p.kill()
            ok("  гаснет по SIGTERM", False, "не погасла за 5 с")
        os.close(rfd)
    # Подкоманды с {conf}: программа принимает --conf (выход — из файла, steer-api 4.3).
    kinds = st.get("kinds", [])
    for c in st.get("commands", []):
        if not c.get("conf") or not kinds:
            continue
        kname = kinds[0]["name"]
        keys = keys_of(d, kinds[0], tmp)
        cpath = os.path.join(tmp, "c1.json")
        with open(cpath, "w") as f:
            json.dump(conf_json(pid, "c1", kname, keys, tmp), f)
        argv = [os.path.join(d, c["prog"])] + (c.get("argv") or [c["name"]]) + ["c1", "--conf", cpath]
        try:
            r = subprocess.run(argv, capture_output=True, env=env_for(pid, d, tmp), timeout=15)
            err = r.stderr.decode(errors="replace")
            good = r.returncode != 127 and "--conf" not in err
        except subprocess.TimeoutExpired:
            good, err = True, ""
        ok("подкоманда %s принимает --conf {conf}" % c["name"], good, err.strip()[:200])
    # steer.links (steer-api 1.3): менеджер ставит их после распаковки, до scripts.postinst.
    links = st.get("links", [])
    perms = [p[3:].rstrip("/") for p in m.get("permissions", []) if p.startswith("fs:")]
    root = os.path.join(tmp, "sysroot")
    seen = set()
    for lk in links:
        path, target, mode = lk.get("path", ""), lk.get("target", ""), lk.get("mode", "link")
        good = path.startswith("/") and ".." not in path and not path.endswith("/") and path not in seen
        seen.add(path)
        ok("ссылка %s: абсолютный путь файла, один раз" % path, good)
        core = path.startswith(("/usr/lib/steer/", "/etc/steer/", "/usr/sbin/steer")) or path == "/etc/init.d/steer"
        ok("  не путь ядра steer", not core)
        ok("  под правом fs:", any(path == b or path.startswith(b + "/") for b in perms), "права: %s" % perms)
        ok("  mode link или conf (%s)" % mode, mode in ("link", "conf"))
        src = os.path.join(d, target)
        ok("  цель %s — файл плагина" % target, ".." not in target and not target.startswith("/") and os.path.isfile(src))
        if not os.path.isfile(src) or mode != "link":
            continue
        full = root + path
        os.makedirs(os.path.dirname(full), exist_ok=True)
        os.symlink(src, full)
        ok("  ссылка ведёт в каталог плагина", os.path.realpath(full) == os.path.realpath(src))
        if path.startswith(("/usr/bin/", "/usr/sbin/", "/etc/init.d/", "/usr/libexec/")):
            ok("  программа по ссылке исполняемая", os.access(full, os.X_OK))
        if path.startswith(("/usr/bin/", "/usr/sbin/")) and os.access(full, os.X_OK):
            try:
                r = subprocess.run([full, "version"], capture_output=True, timeout=10)
                code = r.returncode
            except (OSError, subprocess.TimeoutExpired) as e:
                code = 127
            ok("  программа по ссылке запускается (%s version → код %s)" % (path, code), code not in (126, 127))
    print("steer-2.1: %d ok, %d fail" % (passes, len(fails)))
    return 1 if fails else 0


if __name__ == "__main__":
    sys.exit(main())
