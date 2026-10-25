package main

import (
	"bytes"
	"debug/elf"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	sdk "github.com/splify2/plugin-sdk"
)

type diag struct {
	err bool
	msg string
}

type linter struct {
	ds []diag
}

func (l *linter) errf(f string, a ...any) { l.ds = append(l.ds, diag{true, fmt.Sprintf(f, a...)}) }
func (l *linter) warnf(f string, a ...any) {
	l.ds = append(l.ds, diag{false, fmt.Sprintf(f, a...)})
}
func (l *linter) errors() int {
	n := 0
	for _, d := range l.ds {
		if d.err {
			n++
		}
	}
	return n
}

type lintOpts struct {
	arch       string // под какую архитектуру собрано; пусто — не сверять машинный код
	schemaOnly bool
}

var (
	reSubst      = regexp.MustCompile(`\{([a-z][a-z0-9_]*)\}`)
	reSubstKnown = regexp.MustCompile(`^(out|conf|plugin_dir|plugin_conf|plugin_state|mark|mark[0-9]+|queue[0-9]+|port[0-9]+|out_mark|out_table|out_port[0-9]+|steer_mask|tcp_port)$`)
	reTech       = regexp.MustCompile(`(?i)\b(json|stderr|stdout|errno|socket|nftables?|ubus|rpcd?|sigsegv|sigkill|netlink|iptables|fwmark|syscall|exit code|0x[0-9a-f]{2,})\b`)
	reBareImport = regexp.MustCompile(`(?:from|import)\s*\(?\s*["'](preact|react|react-dom)(/[^"']*)?["']`)
	reActivate   = regexp.MustCompile(`as default|export\s+default|activate`)
	coreCommands = map[string]bool{
		"apply": true, "failover": true, "select": true, "supervise": true, "down": true, "dnsd": true,
		"daemon": true, "ctl-serve": true, "reload": true, "subscribe": true, "ctl": true, "spec": true,
		"status": true, "diag": true, "explain": true, "outputs": true, "dnsd-sig": true, "dnsd-table": true,
		"conns": true, "dns-probe": true, "dns-log": true, "needs-dnsd": true, "dev-id": true, "fit": true,
		"plugin": true, "help": true, "version": true, "check": true,
	}
	gplCompat = map[string]bool{
		"GPL-3.0-only": true, "GPL-3.0-or-later": true, "GPL-3.0": true, "AGPL-3.0-only": true,
		"AGPL-3.0-or-later": true, "LGPL-3.0-only": true, "LGPL-3.0-or-later": true, "LGPL-2.1-or-later": true,
		"MIT": true, "BSD-2-Clause": true, "BSD-3-Clause": true, "ISC": true, "Apache-2.0": true,
		"0BSD": true, "Unlicense": true,
	}
	reNftTop   = regexp.MustCompile(`^\s*(set|map|chain)\s+([A-Za-z0-9_-]+)\s*\{`)
	reRelBound = regexp.MustCompile(`^(.*[^0-9])([0-9]+)$`)
)

func mustValidator(name string) *validator {
	b, err := sdk.FS.ReadFile("schema/" + name)
	if err != nil {
		panic(err)
	}
	v, err := newValidator(b)
	if err != nil {
		panic(err)
	}
	return v
}

// lintTree проверяет манифест по схеме, файлы, права, совместимость и машинный код.
func lintTree(t *tree, o lintOpts) *linter {
	l := &linter{}
	raw, ok := t.files[fPlugin]
	if !ok {
		l.errf("нет plugin.json в корне")
		return l
	}
	doc, err := decodeJSON(raw.data)
	if err != nil {
		l.errf("plugin.json: %v", err)
		return l
	}
	errs := mustValidator("plugin.schema.json").check(doc)
	for _, e := range errs {
		l.errf("plugin.json%s", e)
	}
	if len(errs) > 0 || o.schemaOnly {
		return l
	}
	p, err := parsePlugin(raw.data)
	if err != nil {
		l.errf("plugin.json: %v", err)
		return l
	}
	l.arches(p, o)
	l.compat(p)
	l.permissions(p, t)
	l.files(p, t)
	l.names(p)
	l.kindsKeys(p, t)
	l.substitutions(p, t)
	l.nft(p, t)
	l.types(p, t)
	l.i18n(p, t)
	l.themes(p, t)
	l.ui(p, t)
	l.elfs(p, t, o)
	l.license(p)
	return l
}

func (l *linter) arches(p *pluginDoc, o lintOpts) {
	list, all, err := p.archs()
	if err != nil {
		l.errf("arch: %v", err)
		return
	}
	for _, a := range list {
		if findArch(a) == nil {
			l.warnf("arch: %s не среди архитектур steer (%s)", a, strings.Join(archIDs(), ", "))
		}
	}
	if o.arch != "" && !all {
		found := false
		for _, a := range list {
			if a == o.arch {
				found = true
			}
		}
		if !found {
			l.errf("arch: архитектуры %s нет в списке манифеста (%s)", o.arch, strings.Join(list, ", "))
		}
	}
	if o.arch != "" && all && o.arch != "all" {
		l.errf("arch: плагин без машинного кода собирается как all, а не %s", o.arch)
	}
}

func (l *linter) checkRange(axis, r string, sdkV string) {
	rg, err := parseRange(r)
	if err != nil {
		l.errf("compat.%s: %v", axis, err)
		return
	}
	v, _ := parseVersion(sdkV)
	if !rg.contains(v) {
		l.warnf("compat.%s %q не включает %s, под которую собран этот SDK: плагин не проверить со стендом SDK", axis, r, sdkV)
	}
}

func (l *linter) compat(p *pluginDoc) {
	for axis, r := range p.Compat {
		switch axis {
		case "steer-api":
			l.checkRange(axis, r, sdkSteerAPI)
		case "steer-abi":
			l.checkRange(axis, r, sdkSteerABI)
		case "host-api":
			l.checkRange(axis, r, sdkHostAPI)
		case "steer-core":
		default:
			if _, err := parseRange(r); err != nil {
				l.errf("compat.%s: %v", axis, err)
			}
		}
	}
	for n, r := range p.Requires.Plugins {
		if _, err := parseRange(r); err != nil {
			l.errf("requires.plugins.%s: %v", n, err)
		}
	}
	has := func(a string) bool { _, ok := p.Compat[a]; return ok }
	// steer-api нужна всему, что исполняется как программа или подстановка: помощникам, проверкам,
	// разворотам, подкомандам, службам, хукам и файлам nft. Плагин, у которого только .so и виды без
	// этого, обходится осью steer-abi (docs/manifest.md, «compat»).
	usesAPI := len(p.Steer.Commands)+len(p.Steer.Services)+len(p.Steer.Nft) > 0 ||
		p.Steer.Hooks.PreApply != nil || p.Steer.Hooks.Event != nil
	for _, k := range p.Steer.Kinds {
		if k.Helper != nil || k.Check != nil || k.Expand != nil || len(k.Nft) > 0 {
			usesAPI = true
		}
	}
	if usesAPI && !has("steer-api") {
		l.errf("compat: есть программы, подкоманды, службы, хуки или nft, но нет оси steer-api")
	}
	if p.Steer.So != nil && !has("steer-abi") {
		l.errf("compat: есть steer.so, но нет оси steer-abi")
	}
	if p.Steer.So == nil && has("steer-abi") {
		l.warnf("compat.steer-abi объявлена, а steer.so нет")
	}
	if (p.UI != nil || len(p.Themes) > 0 || p.Type == "language") && !has("host-api") {
		l.errf("compat: у ui, тем и плагина-языка должна быть ось host-api")
	}
}

func (l *linter) permissions(p *pluginDoc, t *tree) {
	need := func(perm, why string) {
		if !p.hasPerm(perm) {
			l.errf("права: %s требует права %q — добавьте его в permissions", why, perm)
		}
	}
	if len(p.Steer.Nft) > 0 {
		need("nft", "steer.nft")
	}
	for _, k := range p.Steer.Kinds {
		if len(k.Nft) > 0 {
			need("nft", "kinds["+k.Name+"].nft")
		}
		if k.Expand != nil {
			need("spec.transform", "kinds["+k.Name+"].expand")
		}
	}
	if p.Steer.Hooks.PreApply != nil {
		need("spec.transform", "hooks.pre_apply")
	}
	if p.Steer.So != nil {
		need("so", "steer.so")
	}
	if len(p.Requires.System) > 0 {
		need("system-packages", "requires.system")
	}
	for _, k := range p.Keep {
		ok := false
		for _, perm := range p.Permissions {
			if strings.HasPrefix(perm, "fs:") {
				base := strings.TrimSuffix(perm[3:], "/")
				if k == base || strings.HasPrefix(k, base+"/") {
					ok = true
				}
			}
		}
		if !ok {
			l.errf("права: путь keep %s не разрешён правом fs:%s", k, k)
		}
	}
}

type fref struct {
	where, file string
	exec        bool
}

func (p *pluginDoc) refs() []fref {
	var r []fref
	add := func(w, f string, x bool) {
		if f != "" {
			r = append(r, fref{w, f, x})
		}
	}
	for _, k := range p.Steer.Kinds {
		w := "steer.kinds[" + k.Name + "]"
		add(w+".keys", k.Keys, false)
		if k.Helper != nil {
			add(w+".helper.prog", k.Helper.Prog, true)
		}
		if k.Check != nil {
			add(w+".check.prog", k.Check.Prog, true)
		}
		if k.Expand != nil {
			add(w+".expand.prog", k.Expand.Prog, true)
		}
		for _, n := range k.Nft {
			add(w+".nft", n, false)
		}
	}
	for _, c := range p.Steer.Commands {
		add("steer.commands["+c.Name+"].prog", c.Prog, true)
	}
	for _, s := range p.Steer.Services {
		add("steer.services["+s.Name+"].prog", s.Prog, true)
	}
	if h := p.Steer.Hooks.PreApply; h != nil {
		add("steer.hooks.pre_apply.prog", h.Prog, true)
	}
	if h := p.Steer.Hooks.Event; h != nil {
		add("steer.hooks.event.prog", h.Prog, true)
	}
	for _, n := range p.Steer.Nft {
		add("steer.nft", n, false)
	}
	if p.Steer.So != nil {
		add("steer.so.path", p.Steer.So.Path, false)
	}
	if p.RPC != nil {
		add("rpc.prog", p.RPC.Prog, true)
	}
	if p.UI != nil {
		add("ui.entry", p.UI.Entry, false)
		for _, c := range p.UI.CSS {
			add("ui.css", c, false)
		}
	}
	if p.I18n != nil {
		dir := p.I18n.Dir
		if dir == "" {
			dir = "i18n"
		}
		for _, lg := range p.I18n.Languages {
			add("i18n.languages["+lg+"]", dir+"/"+lg+".json", false)
		}
		for lg, h := range p.I18n.Host {
			add("i18n.host."+lg+".file", h.File, false)
		}
	}
	for _, th := range p.Themes {
		add("themes["+th.ID+"].file", th.File, false)
	}
	for _, lb := range p.Libs {
		add("libs["+lb.Soname+"].file", lb.File, false)
	}
	add("scripts.postinst", p.Scripts.Postinst, true)
	add("scripts.prerm", p.Scripts.Prerm, true)
	add("scripts.migrate", p.Scripts.Migrate, true)
	if p.Standalone != nil {
		add("standalone.prog", p.Standalone.Prog, true)
		add("standalone.init", p.Standalone.Init, true)
	}
	return r
}

func (l *linter) files(p *pluginDoc, t *tree) {
	for _, r := range p.refs() {
		if !t.has(r.file) {
			l.errf("%s: файла %s нет в плагине", r.where, r.file)
			continue
		}
		if r.exec && !t.generatedOnly(r.file) && t.mode(r.file) != 0755 {
			l.errf("%s: %s не исполняемый (chmod +x)", r.where, r.file)
		}
	}
	// файлы вне 0644/0755 в архив не попадут: режим берётся по биту x владельца/группы/прочих
	for _, n := range t.names() {
		if n == fManifest || n == fSig {
			l.errf("%s: имя занято под служебный файл архива", n)
		}
		if len(n) > 255 {
			l.errf("%s: путь длиннее 255 байт (предел ustar: имя 100 + префикс 155)", n)
		}
	}
}

func (l *linter) names(p *pluginDoc) {
	seen := map[string]string{}
	dup := func(kind, name string) {
		if _, ok := seen[kind+name]; ok {
			l.errf("имя %s %q повторено в манифесте", kind, name)
		}
		seen[kind+name] = "1"
	}
	for _, k := range p.Steer.Kinds {
		dup("вида", k.Name)
	}
	for _, c := range p.Steer.Commands {
		dup("подкоманды", c.Name)
		if coreCommands[c.Name] {
			l.errf("подкоманда %q принадлежит ядру steer — выберите другое имя", c.Name)
		}
	}
	for _, s := range p.Steer.Services {
		dup("службы", s.Name)
	}
	for _, c := range p.Conflicts {
		if c == p.ID {
			l.errf("conflicts: плагин конфликтует сам с собой")
		}
	}
	if _, ok := p.Requires.Plugins[p.ID]; ok {
		l.errf("requires.plugins: плагин зависит сам от себя")
	}
	for _, k := range p.Steer.Kinds {
		if k.Impl == "core" {
			l.warnf("kinds[%s].impl=core принимается только из официального репозитория", k.Name)
		}
		hasProg := k.Helper != nil || k.Expand != nil || len(k.Nft) > 0
		if !hasProg && k.Impl != "core" && p.Steer.So == nil {
			l.warnf("kinds[%s]: нет ни helper, ни expand, ни nft — вид ничего не делает", k.Name)
		}
	}
}

func (l *linter) kindsKeys(p *pluginDoc, t *tree) {
	v := mustValidator("kind-keys.schema.json")
	for _, k := range p.Steer.Kinds {
		if k.Keys == "" {
			continue
		}
		f, ok := t.files[k.Keys]
		if !ok {
			continue
		}
		doc, err := decodeJSON(f.data)
		if err != nil {
			l.errf("%s: %v", k.Keys, err)
			continue
		}
		for _, e := range v.check(doc) {
			l.errf("%s%s (разрешённое подмножество — schema/kind-keys.schema.json)", k.Keys, e)
		}
		walkPatterns(doc, func(pat string) {
			if _, err := regexpPosix(pat); err != nil {
				l.errf("%s: pattern %q не POSIX ERE: %v", k.Keys, pat, err)
			}
		})
	}
}

func walkPatterns(x any, f func(string)) {
	switch m := x.(type) {
	case map[string]any:
		if s, ok := m["pattern"].(string); ok {
			f(s)
		}
		for _, v := range m {
			walkPatterns(v, f)
		}
	case []any:
		for _, v := range m {
			walkPatterns(v, f)
		}
	}
}

func regexpPosix(s string) (*regexp.Regexp, error) { return regexp.CompilePOSIX(s) }

func (l *linter) checkSubst(where, text string, p *pluginDoc, kind *kindDoc, strict bool) {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "{{", ""), "}}", "")
	for _, m := range reSubst.FindAllStringSubmatch(text, -1) {
		name := m[1]
		if !reSubstKnown.MatchString(name) {
			if strict {
				l.errf("%s: неизвестная подстановка {%s}", where, name)
			} else {
				l.warnf("%s: {%s} похоже на подстановку, но такой нет (docs/steer-api.md, 1.3)", where, name)
			}
			continue
		}
		var idx int
		var base string
		if mm := reRelBound.FindStringSubmatch(name); mm != nil {
			base = mm[1]
			fmt.Sscanf(mm[2], "%d", &idx)
		}
		switch {
		case base == "queue" && idx >= p.Resources.Queues:
			l.errf("%s: {%s}, а resources.queues = %d", where, name, p.Resources.Queues)
		case base == "port" && idx >= p.Resources.Ports:
			l.errf("%s: {%s}, а resources.ports = %d", where, name, p.Resources.Ports)
		case base == "mark" && p.Resources.MarkBits == 0:
			l.errf("%s: {%s}, а resources.mark_bits не задан", where, name)
		case name == "mark" && p.Resources.MarkBits == 0:
			l.errf("%s: {mark}, а resources.mark_bits не задан", where)
		case base == "out_port" && (kind == nil || idx >= kind.Res.Ports):
			l.errf("%s: {%s}, а у вида resources.ports меньше", where, name)
		case (name == "out" || name == "conf" || name == "out_mark" || name == "out_table" || name == "tcp_port") && kind == nil && !strings.Contains(where, "kinds["):
			l.errf("%s: {%s} есть только у видов", where, name)
		}
	}
}

func (l *linter) substitutions(p *pluginDoc, t *tree) {
	argv := func(where string, a []string, k *kindDoc) {
		for _, w := range a {
			l.checkSubst(where, w, p, k, true)
		}
	}
	for i := range p.Steer.Kinds {
		k := &p.Steer.Kinds[i]
		w := "steer.kinds[" + k.Name + "]"
		if k.Helper != nil {
			argv(w+".helper.argv", k.Helper.Argv, k)
		}
		if k.Check != nil {
			argv(w+".check.argv", k.Check.Argv, k)
		}
		if k.Expand != nil {
			argv(w+".expand.argv", k.Expand.Argv, k)
		}
		for _, n := range k.Nft {
			if f, ok := t.files[n]; ok {
				l.checkSubst(n, string(f.data), p, k, false)
			}
		}
	}
	for _, c := range p.Steer.Commands {
		argv("steer.commands["+c.Name+"].argv", c.Argv, nil)
	}
	for _, s := range p.Steer.Services {
		argv("steer.services["+s.Name+"].argv", s.Argv, nil)
	}
	if h := p.Steer.Hooks.PreApply; h != nil {
		argv("steer.hooks.pre_apply.argv", h.Argv, nil)
	}
	if h := p.Steer.Hooks.Event; h != nil {
		argv("steer.hooks.event.argv", h.Argv, nil)
	}
	for _, n := range p.Steer.Nft {
		if f, ok := t.files[n]; ok {
			l.checkSubst(n, string(f.data), p, nil, false)
		}
	}
}

// nft: на верхнем уровне только set, map и chain <слот> (docs/steer-api.md, 5.3).
func (l *linter) nft(p *pluginDoc, t *tree) {
	slots := map[string]bool{}
	if v, err := sdk.FS.ReadFile("schema/plugin.schema.json"); err == nil {
		var s struct {
			Defs struct {
				Slot struct {
					Enum []string `json:"enum"`
				} `json:"slot"`
			} `json:"$defs"`
		}
		if json.Unmarshal(v, &s) == nil {
			for _, x := range s.Defs.Slot.Enum {
				slots[x] = true
			}
		}
	}
	files := append([]string(nil), p.Steer.Nft...)
	for _, k := range p.Steer.Kinds {
		files = append(files, k.Nft...)
	}
	for _, n := range files {
		f, ok := t.files[n]
		if !ok {
			continue
		}
		depth := 0
		for i, line := range strings.Split(string(f.data), "\n") {
			code := line
			if j := strings.Index(code, "#"); j >= 0 {
				code = code[:j]
			}
			if depth == 0 && strings.TrimSpace(code) != "" {
				m := reNftTop.FindStringSubmatch(code)
				switch {
				case m == nil:
					l.errf("%s:%d: на верхнем уровне допустимы только set, map и chain <слот>: %q", n, i+1, strings.TrimSpace(code))
				case m[1] == "chain" && !slots[m[2]]:
					l.errf("%s:%d: %q — не слот (слоты: docs/steer-api.md, 5.2)", n, i+1, m[2])
				}
			}
			depth += strings.Count(code, "{") - strings.Count(code, "}")
			if strings.Count(code, "{{")+strings.Count(code, "}}") > 0 {
				// {{ и }} — буквальные скобки, не меняют глубину
				depth -= strings.Count(code, "{{") * 2
				depth += strings.Count(code, "}}") * 2
			}
			if depth < 0 {
				l.errf("%s:%d: лишняя }", n, i+1)
				depth = 0
			}
		}
		if depth != 0 {
			l.errf("%s: не закрыта {", n)
		}
	}
}

func (l *linter) types(p *pluginDoc, t *tree) {
	switch p.Type {
	case "library":
		if len(p.Libs) == 0 {
			l.errf("type: library требует раздел libs")
		}
		m := reRelBound.FindStringSubmatch(p.ID)
		pv, _ := parseVersion(p.Version)
		if m == nil {
			l.errf("id библиотеки %q обязан нести major: net1, net2 …", p.ID)
		} else if fmt.Sprint(pv.n[0]) != m[2] {
			l.errf("id %q несёт major %s, а major версии %s", p.ID, m[2], p.Version)
		}
		for _, lb := range p.Libs {
			sm := regexp.MustCompile(`\.so\.([0-9]+)$`).FindStringSubmatch(lb.Soname)
			if sm != nil && lb.Abi != "" && strings.SplitN(lb.Abi, ".", 2)[0] != sm[1] {
				l.errf("libs[%s]: major abi %s не равен номеру в soname", lb.Soname, lb.Abi)
			}
			if !strings.HasPrefix(lb.Soname, "libs2p-") {
				l.warnf("libs[%s]: имя библиотеки плагина принято начинать с libs2p-", lb.Soname)
			}
			if !strings.HasPrefix(lb.File, "lib/") {
				l.warnf("libs[%s]: файл %s лежит не в lib/ — RUNPATH зависимых смотрит в ../../<id>/lib", lb.Soname, lb.File)
			}
		}
	default:
		if len(p.Libs) > 0 {
			l.errf("libs бывает только у type: library")
		}
	}
	if p.Type == "language" {
		if p.I18n == nil || len(p.I18n.Host) == 0 {
			l.errf("type: language требует i18n.host")
		}
	} else if p.I18n != nil && len(p.I18n.Host) > 0 {
		l.errf("i18n.host бывает только у type: language")
	}
	if p.Type == "theme" {
		if len(p.Themes) == 0 {
			l.errf("type: theme требует themes")
		}
		if p.UI != nil || p.RPC != nil || len(p.Steer.Kinds)+len(p.Steer.Commands)+len(p.Steer.Services) > 0 || p.Steer.So != nil {
			l.errf("type: theme — только темы: ui, rpc и steer недопустимы")
		}
	}
	if p.Variant != "" && p.Variant != "full" && len(p.Requires.Plugins) > 0 && p.Variant == "lite" {
		l.errf("variant lite: без requires.plugins (статическая программа)")
	}
	if p.Standalone != nil && p.Variant != "lite" {
		l.warnf("standalone есть у варианта %s: мини-ядро обновляет только вариант lite", p.variant())
	}
}

func (l *linter) dict(where string, data []byte, tech bool) {
	doc, err := decodeJSON(data)
	if err != nil {
		l.errf("%s: %v", where, err)
		return
	}
	m, ok := doc.(map[string]any)
	if !ok {
		l.errf("%s: словарь — плоский объект «ключ → строка»", where)
		return
	}
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s, ok := m[k].(string)
		if !ok {
			l.errf("%s: значение %q — не строка", where, k)
			continue
		}
		if tech {
			if w := reTech.FindString(s); w != "" {
				l.warnf("%s: %q — техническое слово %q в тексте для человека (правило хаба: состояние и действие)", where, k, w)
			}
		}
	}
}

func (l *linter) i18n(p *pluginDoc, t *tree) {
	if p.I18n == nil {
		return
	}
	dir := p.I18n.Dir
	if dir == "" {
		dir = "i18n"
	}
	for _, lg := range p.I18n.Languages {
		if f, ok := t.files[dir+"/"+lg+".json"]; ok {
			l.dict(dir+"/"+lg+".json", f.data, true)
		}
	}
	for lg, h := range p.I18n.Host {
		if f, ok := t.files[h.File]; ok {
			l.dict(h.File, f.data, false)
			_ = lg
		}
		if h.Host != "" {
			if _, err := parseRange(h.Host); err != nil {
				l.errf("i18n.host.%s.host: %v", lg, err)
			}
		}
	}
}

var reVar = regexp.MustCompile(`^--(sp|an)-[a-z0-9-]+$`)

func (l *linter) themes(p *pluginDoc, t *tree) {
	for _, th := range p.Themes {
		f, ok := t.files[th.File]
		if !ok {
			continue
		}
		doc, err := decodeJSON(f.data)
		if err != nil {
			l.errf("%s: %v", th.File, err)
			continue
		}
		m, ok := doc.(map[string]any)
		if !ok {
			l.errf("%s: тема — объект {name, light, dark?, css?}", th.File)
			continue
		}
		if _, ok := m["name"].(string); !ok {
			l.errf("%s: нет name", th.File)
		}
		if _, ok := m["light"].(map[string]any); !ok {
			l.errf("%s: нет light — объекта переменных", th.File)
		}
		for _, mode := range []string{"light", "dark"} {
			vars, _ := m[mode].(map[string]any)
			var keys []string
			for k := range vars {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				switch {
				case !reVar.MatchString(k):
					l.errf("%s: %s.%s — не переменная панели (--sp-* или --an-*)", th.File, mode, k)
				case k == "--sp-bleed" || k == "--sp-inset-l" || k == "--sp-inset-r":
					l.errf("%s: %s.%s ставит сама панель", th.File, mode, k)
				}
				if _, ok := vars[k].(string); !ok {
					l.errf("%s: %s.%s — значение должно быть строкой", th.File, mode, k)
				}
			}
		}
		if css, ok := m["css"].(string); ok {
			low := strings.ToLower(css)
			if strings.Contains(low, "@import") || strings.Contains(low, "javascript:") || regexp.MustCompile(`url\(\s*["']?(https?:)?//`).MatchString(low) {
				l.errf("%s: css тянет внешние ресурсы — тема только из переменных и локального CSS", th.File)
			}
		}
	}
}

func (l *linter) ui(p *pluginDoc, t *tree) {
	if p.UI == nil {
		return
	}
	f, ok := t.files[p.UI.Entry]
	if !ok {
		return
	}
	if reBareImport.Match(f.data) {
		l.errf("%s: модуль подключает preact или react отдельным import — панель даёт один экземпляр через прокладку; соберите через @splify2/plugin-api/vite", p.UI.Entry)
	}
	if !reActivate.Match(f.data) {
		l.warnf("%s: не видно export default или activate — панель вызывает activate(host)", p.UI.Entry)
	}
}

func (l *linter) license(p *pluginDoc) {
	if p.Steer.So == nil {
		return
	}
	if p.License == "" {
		l.errf("license: плагин с .so в steerd — производная работа steer (GPL-3.0); укажите совместимую лицензию")
		return
	}
	ok := false
	for _, alt := range regexp.MustCompile(`\s+OR\s+`).Split(p.License, -1) {
		if gplCompat[strings.Trim(alt, "() ")] {
			ok = true
		}
	}
	if !ok {
		l.errf("license: %q несовместима с GPL-3.0 steer, а плагин с .so — производная работа", p.License)
	}
}

func (l *linter) elfs(p *pluginDoc, t *tree, o lintOpts) {
	_, all, _ := p.archs()
	soPath := ""
	if p.Steer.So != nil {
		soPath = p.Steer.So.Path
	}
	var ai *archInfo
	if o.arch != "" {
		ai = findArch(o.arch)
	}
	for _, n := range t.names() {
		f := t.files[n]
		if !bytes.HasPrefix(f.data, []byte("\x7fELF")) {
			if n == soPath {
				l.errf("%s: steer.so.path не ELF", n)
			}
			continue
		}
		ef, err := elf.NewFile(bytes.NewReader(f.data))
		if err != nil {
			l.errf("%s: ELF не читается: %v", n, err)
			continue
		}
		if all {
			l.errf("%s: машинный код в плагине с arch: all", n)
			continue
		}
		if ai != nil {
			if ef.Machine != ai.machine || ef.Class != ai.class || ef.Data != ai.order {
				l.errf("%s: собран не под %s (машина %v, %v, %v)", n, ai.id, ef.Machine, ef.Class, ef.Data)
			}
		}
		needed, _ := ef.ImportedLibraries()
		runp, _ := ef.DynString(elf.DT_RUNPATH)
		rpath, _ := ef.DynString(elf.DT_RPATH)
		rp := strings.Join(append(runp, rpath...), ":")
		for _, nd := range needed {
			switch {
			case strings.HasPrefix(nd, "libsteer"):
				l.errf("%s: DT_NEEDED %s — плагин не связывается с libsteer (имя файла ядра меняется с каждым выпуском; всё нужное приходит таблицей в init, docs/steer-abi.md)", n, nd)
			case strings.HasPrefix(nd, "libs2p-"):
				m := regexp.MustCompile(`^libs2p-(.+)\.so\.([0-9]+)$`).FindStringSubmatch(nd)
				if m == nil {
					l.errf("%s: DT_NEEDED %s — не имя библиотеки плагина (libs2p-<имя>.so.<major>)", n, nd)
					continue
				}
				dep := m[1] + m[2]
				if _, ok := p.Requires.Plugins[dep]; !ok {
					l.errf("%s: DT_NEEDED %s, а в requires.plugins нет плагина-библиотеки %q", n, nd, dep)
				}
				if !strings.Contains(rp, "$ORIGIN/../../"+dep+"/lib") {
					l.errf("%s: у DT_NEEDED %s нет RUNPATH $ORIGIN/../../%s/lib (s2p cc --requires %s)", n, nd, dep, dep)
				}
			case strings.HasPrefix(nd, "libc.") || strings.HasPrefix(nd, "ld-musl"):
			default:
				l.warnf("%s: DT_NEEDED %s — такой библиотеки может не быть на роутере (musl держит libc одной .so)", n, nd)
			}
		}
		if ai != nil {
			for _, ph := range ef.Progs {
				if ph.Type == elf.PT_INTERP {
					b := make([]byte, ph.Filesz)
					ph.ReadAt(b, 0)
					in := strings.TrimRight(string(b), "\x00")
					if in != ai.interp {
						l.errf("%s: загрузчик %s, у %s на роутере %s", n, in, ai.id, ai.interp)
					}
				}
			}
		}
		if n == soPath {
			l.soExport(n, ef)
		}
		if strings.HasPrefix(n, "bin/") && f.mode != 0755 {
			l.errf("%s: программа без права исполнения", n)
		}
	}
}

// soExport: у .so плагина ровно один экспортируемый символ — steer_plugin_v1_entry (docs/steer-abi.md).
func (l *linter) soExport(n string, ef *elf.File) {
	if ef.Type != elf.ET_DYN {
		l.errf("%s: .so плагина должна быть разделяемой библиотекой (ET_DYN)", n)
		return
	}
	syms, err := ef.DynamicSymbols()
	if err != nil {
		l.errf("%s: нет таблицы динамических символов: %v", n, err)
		return
	}
	var exp []string
	for _, s := range syms {
		if s.Section == elf.SHN_UNDEF {
			continue
		}
		b := elf.ST_BIND(s.Info)
		if b == elf.STB_GLOBAL || b == elf.STB_WEAK {
			exp = append(exp, s.Name)
		}
	}
	sort.Strings(exp)
	if len(exp) != 1 || exp[0] != "steer_plugin_v1_entry" {
		l.errf("%s: экспорт должен быть ровно steer_plugin_v1_entry, а сейчас [%s] (-fvisibility=hidden и файл версий, docs/steer-abi.md)", n, strings.Join(exp, " "))
	}
	// неразрешённые символы — только libc: свои вызовы ядра идут таблицей host_v1
	for _, s := range syms {
		if s.Section == elf.SHN_UNDEF && strings.HasPrefix(s.Name, "steer_") {
			l.errf("%s: неразрешённый символ %s — ядро даёт функции таблицей host_v1, а не символами", n, s.Name)
		}
	}
}

func printDiags(l *linter, name string) {
	for _, d := range l.ds {
		lvl := "предупреждение"
		if d.err {
			lvl = "ошибка"
		}
		fmt.Fprintf(os.Stderr, "%s: %s: %s\n", name, lvl, d.msg)
	}
}

var _ = path.Join
