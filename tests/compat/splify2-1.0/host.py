#!/usr/bin/env python3
"""Заглушка хоста splify2, splify2-api 1.0 (docs/splify2-api.md): подключится ли плагин.

Не копия каркаса, а стенд, который ведёт себя как он по спецификации: таблица methods.tsv версии, права,
разбор блока `splify2`, запуск хуков, задач и событий по фикстурам контекста, службы хоста (`host`, `lib.sh`) поверх
каталога-песочницы. Плагин в песочнице видит только окружение раздела 7 спецификации — ни путей, ни швов каркаса.
Печатает по строке на проверку (`ok  …` или `FAIL …`), код возврата — число отказов (0 — плагин подключится).

    host.py <каталог плагина>
"""
import json, os, re, shutil, subprocess, sys, tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, ".."))
from semver import range_has  # noqa: E402
HOST_API = (1, 0, 0)  # host-api панели (docs/host-api.md)
API = "1.0.0"
HOOKS = ["pre_spec_set", "post_spec_set", "pre_apply", "post_apply", "post_apply_fail", "heal",
         "backup_sections", "support_report", "purge", "plugin_changed", "sub_fields"]
OFFICIAL_ONLY = {}  # moved-методы: плагин назван в methods.tsv

fails = 0


def ok(msg):
    print("  ok    " + msg)


def fail(msg):
    global fails
    fails += 1
    print("  FAIL  " + msg)


def check(cond, msg):
    (ok if cond else fail)(msg)


def methods():
    kinds = {}
    for line in open(os.path.join(HERE, "methods.tsv"), encoding="utf-8"):
        if line.startswith("#") or not line.strip():
            continue
        f = line.rstrip("\n").split("\t")
        kinds[f[0]] = (f[1], f[2] if len(f) > 2 else "")
    return kinds


def range_ok(r):
    """Диапазон плагина включает версию хоста (minor добавляет: ^1.2 на хосте 1.1 не подключится)."""
    return range_has(r, tuple(int(x) for x in API.split(".")))


def fixture(name, pid, spec):
    s = open(os.path.join(HERE, "fixtures", name + ".json"), encoding="utf-8").read()
    return s.replace("@ID@", pid).replace("@SPEC@", spec)


def relpath_ok(p):
    return isinstance(p, str) and p != "" and not p.startswith("/") and ".." not in p.split("/")


def sandbox_env(pid, pdir, work):
    """Окружение программы плагина — ровно раздел 7 спецификации. Переменные разработчика (SPEC, LISTS, …) не просачиваются."""
    env = {
        "PATH": os.path.join(HERE, "tools") + ":" + os.environ.get("PATH", "/usr/bin:/bin"),
        "HOME": work, "LANG": "C.UTF-8", "TMPDIR": work,
        "STEER_PLUGIN_ID": pid, "STEER_PLUGIN_DIR": pdir,
        "STEER_PLUGIN_CONF": os.path.join(work, "conf"), "STEER_PLUGIN_STATE": os.path.join(work, "state"),
        "SPLIFY2_API": "1", "SPLIFY2_LIB": os.path.join(HERE, "lib.sh"), "SPLIFY2_HOST": os.path.join(HERE, "host"),
        "S2_SANDBOX": work,
    }
    return env


def run_prog(pdir, pid, prog, arg, stdin_text, env_extra, timeout, work):
    path = os.path.join(pdir, prog)
    env = sandbox_env(pid, pdir, work)
    env.update(env_extra)
    cmd = [path] + ([arg] if arg else [])
    try:
        r = subprocess.run(cmd, input=stdin_text, capture_output=True, text=True, timeout=timeout, cwd=pdir, env=env)
        return r.returncode, r.stdout, r.stderr
    except subprocess.TimeoutExpired:
        return 124, "", ""


def json_object(text):
    try:
        d = json.loads(text)
    except Exception:
        return None
    return d if isinstance(d, dict) else None


def backup_roundtrip(pdir, pid, prog, to, work, spec):
    """export → check → restore → export: данные плагина в архиве переживают круг и ничего не теряют и не плодят."""
    env = {"SANDBOX": work, "SPLIFY2_HOOK": "backup_sections", "SPLIFY2_SPEC_TMP": spec}

    def call(ctx):
        body = json.dumps({"api": 1, "hook": "backup_sections", "plugin": pid, "ctx": ctx}, ensure_ascii=False) + "\n"
        rc, out, err = run_prog(pdir, pid, prog, None, body, env, to, work)
        return rc, json_object(out)

    rc, d1 = call({"op": "export"})
    secs = (d1 or {}).get("sections", {})
    if not isinstance(secs, dict):
        return
    rc, dc = call({"op": "check", "sections": secs})
    check(rc == 0 and dc is not None and dc.get("ok") is True, "хук backup_sections: check принимает то, что сам же отдал")
    if dc is not None and dc.get("ok") is False:
        check(isinstance(dc.get("error", ""), str), "хук backup_sections: отказ check называет причину")
    rc, dr = call({"op": "restore", "sections": secs})
    check(rc == 0 and dr is not None and dr.get("ok") is True, "хук backup_sections: restore принимает то, что сам же отдал")
    if dr is not None and "restored" in dr:
        check(isinstance(dr["restored"], list) and all(isinstance(x, dict) for x in dr["restored"]),
              "хук backup_sections: restored — массив объектов")
    rc, d2 = call({"op": "export"})
    check(d2 is not None and d2.get("sections", {}) == secs, "хук backup_sections: после restore export отдаёт то же самое")
    # Раздел, которого плагин не знает, не роняет хук: либо отказ с причиной, либо молчаливое согласие.
    rc, du = call({"op": "check", "sections": {"zz_unknown_section": "x"}})
    check(rc == 0 and du is not None and isinstance(du.get("ok"), bool), "хук backup_sections: незнакомый раздел — ответ, а не падение")


def main(pdir):
    pm = os.path.join(pdir, "plugin.json")
    m = json.load(open(pm, encoding="utf-8"))
    pid = m["id"]
    sp = m.get("splify2")
    compat = m.get("compat", {})
    print("== %s (splify2-api %s)" % (pid, API))
    if "host-api" in compat:
        check(range_has(compat["host-api"], HOST_API), "compat.host-api %r включает %s" % (compat["host-api"], ".".join(map(str, HOST_API))))
    if sp is None:
        ok("блока splify2 нет — плагину нечего проверять")
        return
    check("splify2-api" in compat and range_ok(compat["splify2-api"]),
          "compat.splify2-api %r включает %s" % (compat.get("splify2-api"), API))
    perms = set(m.get("permissions", []))
    kinds = methods()
    work = tempfile.mkdtemp(prefix="s2compat.")
    try:
        spec = os.path.join(work, "spec.json")
        shutil.copy(os.path.join(HERE, "fixtures", "spec.json"), spec)
        os.makedirs(os.path.join(work, "marks"), exist_ok=True)
        # Песочница служб хоста: каталог списков, «сеть», состояние каркаса, каталоги плагина (conf, state).
        for sub in ("lists", "web", "core-state", "subs"):
            shutil.copytree(os.path.join(HERE, "fixtures", sub), os.path.join(work, sub))
        os.makedirs(os.path.join(work, "conf"), exist_ok=True)
        os.makedirs(os.path.join(work, "state"), mode=0o700, exist_ok=True)
        env0 = {"SANDBOX": work}

        # --- хуки
        hooks = sp.get("hooks", {})
        if hooks:
            check("splify2.hooks" in perms, "хуки заявлены с правом splify2.hooks")
        for h, hd in hooks.items():
            check(h in HOOKS, "хук %s известен" % h)
            if h not in HOOKS:
                continue
            prog, mode = hd.get("prog"), hd.get("mode", "observe")
            to = hd.get("timeout_s", 10)
            check(relpath_ok(prog) and os.access(os.path.join(pdir, prog), os.X_OK), "хук %s: программа %s исполняема" % (h, prog))
            if not (relpath_ok(prog) and os.access(os.path.join(pdir, prog), os.X_OK)):
                continue
            if mode == "transform":
                check(h in ("pre_spec_set", "heal"), "хук %s: режим transform допустим" % h)
            if hd.get("critical"):
                ok("хук %s: critical — карточка плагина покажет это как право" % h)
            if h == "backup_sections":
                cases = ["backup_sections_export", "backup_sections_check", "backup_sections_restore"]
            elif h == "purge":
                cases = ["purge_dry", "purge"]
            else:
                cases = [h]
            for c in cases:
                shutil.copy(os.path.join(HERE, "fixtures", "spec.json"), spec)
                rc, out, err = run_prog(pdir, pid, prog, None, fixture(c, pid, spec),
                                        dict(env0, SPLIFY2_HOOK=h, SPLIFY2_SPEC_TMP=spec), to, work)
                d = json_object(out)
                check(rc == 0 and d is not None and isinstance(d.get("ok"), bool),
                      "хук %s (%s): код 0 и объект с булевым ok за %d с (код %s)" % (h, c, to, rc))
                if d is None:
                    continue
                check("warn" not in d or isinstance(d["warn"], list), "хук %s (%s): warn — массив" % (h, c))
                if h in ("pre_spec_set", "heal"):
                    try:
                        json.load(open(spec, encoding="utf-8")); good = True
                    except Exception:
                        good = False
                    check(good, "хук %s: после transform спека — годный JSON" % h)
                if c == "backup_sections_export":
                    secs = d.get("sections", {})
                    check(isinstance(secs, dict) and all(re.fullmatch(r"[A-Za-z0-9_]{1,64}", k) and isinstance(v, str)
                                                         for k, v in secs.items()),
                          "хук backup_sections: имена разделов [A-Za-z0-9_]+, значения — строки")
                if h == "support_report":
                    s = d.get("section")
                    check(s is None or (isinstance(s, dict) and isinstance(s.get("text", ""), str)), "хук support_report: section.text — строка")
                if h == "sub_fields":
                    fl = d.get("fields", {})
                    check(isinstance(fl, dict) and all(isinstance(v, dict) and all(isinstance(x, (str, int, bool)) for x in v.values())
                                                       for v in fl.values()),
                          "хук sub_fields: fields — «имя подписки → поля», значения — строка, число или флаг")
                if h == "purge":
                    check(isinstance(d.get("plan", []), list), "хук purge (%s): plan — массив строк" % c)
            if h == "backup_sections":
                backup_roundtrip(pdir, pid, prog, to, work, spec)

        # --- задачи
        seen = set()
        for t in sp.get("tasks", []):
            n = t.get("name")
            check(n not in seen, "задача %s: имя уникально" % n)
            seen.add(n)
            check(bool(t.get("every")) != bool(t.get("cron")), "задача %s: ровно одно из every и cron" % n)
            if t.get("every"):
                check(re.fullmatch(r"[1-9][0-9]*[mhd]", t["every"]) is not None, "задача %s: every %r разбирается" % (n, t["every"]))
            if t.get("cron"):
                f = t["cron"].split()
                check(len(f) == 5 and ("M" not in "".join(f[1:])) and re.fullmatch(r"[0-9M*/,\-]+", "".join(f)) is not None,
                      "задача %s: cron из пяти полей, M только в минутах" % n)
            check(relpath_ok(t.get("prog")) and os.access(os.path.join(pdir, t.get("prog", "")), os.X_OK), "задача %s: программа исполняема" % n)

        # --- события
        ev = sp.get("events")
        if ev:
            check(relpath_ok(ev.get("prog")) and os.access(os.path.join(pdir, ev.get("prog", "")), os.X_OK), "события: программа исполняема")
            check(all(re.fullmatch(r"[a-z0-9-]+|\*", s) for s in ev.get("subscribe", ["*"])), "события: имена в subscribe годны")
            if relpath_ok(ev.get("prog")) and os.access(os.path.join(pdir, ev["prog"]), os.X_OK):
                rc, out, err = run_prog(pdir, pid, ev["prog"], None, fixture("event", pid, spec), dict(env0, SPLIFY2_EVENT="applied"), ev.get("timeout_s", 10), work)
                check(rc == 0, "события: программа принимает событие и выходит с кодом 0 (код %s)" % rc)

        # --- методы
        rpc = m.get("rpc", {}).get("methods", {})
        mt = sp.get("methods", {})
        if mt.get("override"):
            check("splify2.override" in perms, "переопределения заявлены с правом splify2.override")
        replaced = 0
        for name, target in mt.get("alias", {}).items():
            k, who = kinds.get(name, ("", ""))
            check(target in rpc, "псевдоним %s → %s: у плагина есть такой rpc-метод" % (name, target))
            if k in ("final", "fast", "frame"):
                fail("псевдоним %s: имя занято (%s)" % (name, k))
            elif k == "moved":
                check(who == pid, "псевдоним %s: вынесенный метод берёт плагин %s, а это %s" % (name, who, pid))
            else:
                ok("псевдоним %s: новое имя на объекте splify2" % name)
        for name, od in mt.get("override", {}).items():
            k, _ = kinds.get(name, ("", ""))
            check(k == "frame", "переопределение %s: метод каркаса (вид %r)" % (name, k or "нет такого"))
            check(od.get("mode") in ("replace", "wrap"), "переопределение %s: режим replace или wrap" % name)
            check(relpath_ok(od.get("prog")) and os.access(os.path.join(pdir, od.get("prog", "")), os.X_OK), "переопределение %s: программа исполняема" % name)
            if od.get("mode") == "replace":
                replaced += 1
        for name, od in mt.get("override", {}).items():
            pass
        for name in mt.get("override", {}):
            if mt["override"][name].get("mode") == "wrap" and relpath_ok(mt["override"][name].get("prog")) \
                    and os.access(os.path.join(pdir, mt["override"][name]["prog"]), os.X_OK):
                nxt = '{"ok":true}'
                rc, out, err = run_prog(pdir, pid, mt["override"][name]["prog"], name, "{}\n",
                                        dict(env0, SPLIFY2_NEXT=nxt), mt["override"][name].get("timeout_s", 10), work)
                check(rc == 0 and json_object(out) is not None, "обёртка %s: ответ — объект JSON (код %s)" % (name, rc))

        # --- настройки
        st = sp.get("settings", {}).get("schema")
        if st:
            sch = None
            try:
                sch = json.load(open(os.path.join(pdir, st), encoding="utf-8")); good = True
            except Exception:
                good = False
            check(good, "settings.schema: %s — JSON" % st)
            if sch is not None:
                props = sch.get("properties", {})
                check(isinstance(props, dict), "settings.schema: properties — объект")
                for k, pr in props.items():
                    # Умолчание схемы доезжает до программы плагина службой хоста settings get.
                    r = subprocess.run([os.path.join(HERE, "host"), "settings", "get", k], capture_output=True, text=True,
                                       env=sandbox_env(pid, pdir, work))
                    has_default = "default" in pr
                    check((r.returncode == 0) == has_default,
                          "настройка %s: службой settings get %s" % (k, "умолчание схемы доезжает" if has_default else "без значения и без умолчания — пусто"))
                    check(bool(pr.get("title")) and isinstance(pr.get("title"), str), "настройка %s: у схемы есть title" % k)

        # --- граница API: тот же поиск запретного, что у `s2p lint --official`
        s2p = os.environ.get("S2P") or os.path.join(HERE, "..", "..", "..", "bin", "s2p")
        if os.access(s2p, os.X_OK):
            r = subprocess.run([s2p, "lint", "--official", pdir], capture_output=True, text=True)
            check(r.returncode == 0, "граница API: s2p lint --official%s" % ("" if r.returncode == 0 else ": " + (r.stderr or r.stdout).strip().splitlines()[0]))
        else:
            ok("граница API: пропущено — нет bin/s2p (sh build.sh)")
    finally:
        shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print(__doc__); sys.exit(2)
    main(os.path.abspath(sys.argv[1]))
    sys.exit(1 if fails else 0)
