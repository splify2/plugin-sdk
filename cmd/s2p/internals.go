package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Граница API (docs/splify2-api.md, раздел 9): плагин пользуется платформой через манифест, хуки, задачи, события,
// схему настроек и службы хоста ($SPLIFY2_HOST) и не заглядывает в то, что каркас считает своим. Здесь — список
// запретного и поиск обращений к нему в программах плагина.

// internalPaths — пути каркаса и ядра. Совпадение — подстрока; `exempt` снимает разрешённые каталоги плагинов внутри
// запретного корня (`/etc/steer/plugins/conf/<id>` — это STEER_PLUGIN_CONF, хотя имя называть всё равно лучше через
// переменную).
var internalPaths = []string{
	"/usr/lib/splify2", "/usr/share/splify2", "/usr/libexec/rpcd/splify2", "/etc/splify2", "/var/run/splify2",
	"/var/lib/splify2", "/var/lock/splify2", "/etc/steer", "/etc/config/splify2", "/etc/init.d/steer", "/etc/rc.d",
}

var internalPathExempt = regexp.MustCompile(`/etc/steer/plugins(/|$)`)

// internalFiles — файлы каркаса, которые плагин не подключает. Плагин, у которого такой файл есть **свой** (в архиве),
// не считается обратившимся: имя тогда указывает на его собственный файл.
var internalFiles = []string{"common.sh", "env.sh", "fetch.sh", "spec.sh", "ext.sh", "plugin.sh", "fast.sh"}

// internalVars — переменные-швы каркаса, которыми стенды уводят его пути, и его внутреннее состояние.
var internalVars = regexp.MustCompile(`(?:\$\{?|^|[\s;&|(])(SPEC|SUB|SUBS_DIR|SUB_INFO|LISTS|APPLIED|MANIFEST|UCI_SPLIFY2|XSTEER_DIR|XS_RUN|` +
	`XS_STATE_DIR|GEO_DIR|STEER|RPCD_LIB|JSHN_SH|FETCH_SH|SPEC_SH|ENV_SH|PLUGIN_ROOT|PLUGIN_STATE|PLUGIN_CONF_DIR|` +
	`SPLIFY2_LIBDIR|EXT_[A-Z_]+|SPLIFY2_LOCK_[A-Z_]+)(?:\}|=|\b)`)

// internalFuncs — функции каркаса. Имя, которое плагин определил сам, обращением не считается. Ищется только слово на месте
// команды (начало строки, после `;`, `&`, `|`, `(`, `{`, `$(`, `if`, `then`, `do`, `!`): переменная с похожим именем
// (`sub_done`) вызовом не считается.
var internalFuncs = regexp.MustCompile(`^(call_lock|need_fetch|need_ext|spec_json|spec_put|spec_have|spec_rev|spec_is_yaml|` +
	`spec_migrate|uci_file|uci_commit|private_file|own_file|engine_check|sanitize_list|backup_build|download|` +
	`fetch_[a-z_]+|sub_[a-z_]+|plugin_[a-z_]+|ext_[a-z_]+)$`)
var reCmdWord = regexp.MustCompile(`(?:^|[;&|(]|\{\s|\$\()\s*(?:(?:if|then|do|else|elif|while|until|!)\s+)*([A-Za-z_][A-Za-z0-9_]*)(=?)`)

// reCaseLabel — строка с метками `case` (`lists|list_fetch|…) ;;`): имена методов плагина вызовами не считаются.
var reCaseLabel = regexp.MustCompile(`^\s*([A-Za-z_*][A-Za-z0-9_*-]*\|)+([A-Za-z_*][A-Za-z0-9_*-]*\)|\\?$)`)

// Определение функции: shell `имя() {` и ucode/JS `function имя(`.
var reFuncDef = regexp.MustCompile(`(?m)^\s*(?:function\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(|([A-Za-z_][A-Za-z0-9_]*)\s*\(\)\s*\{)`)
var reUciSplify2 = regexp.MustCompile(`\buci\b[^|;&\n]*\bsplify2\b`)

// scriptLike — файл, который стоит читать как программу: расширение .sh, строка `#!` или каталоги программ.
func scriptLike(p string, data []byte) bool {
	if len(data) == 0 || len(data) > 2<<20 || strings.IndexByte(string(data[:min(len(data), 512)]), 0) >= 0 {
		return false
	}
	if strings.HasSuffix(p, ".sh") || strings.HasPrefix(string(data), "#!") {
		return true
	}
	for _, d := range []string{"bin/", "hooks/", "rpc/", "lib/", "scripts/"} {
		if strings.HasPrefix(p, d) && !strings.HasSuffix(p, ".json") && !strings.HasSuffix(p, ".so") {
			return true
		}
	}
	return false
}

// internals ищет в программах плагина обращения к внутренностям каркаса. По умолчанию — предупреждения; с официальным
// режимом (`s2p lint --official`) — отказ: так проверяются плагины официального репозитория, которые служат образцом.
func (l *linter) internals(p *pluginDoc, t *tree, o lintOpts) {
	report := l.warnf
	if o.official {
		report = l.errf
	}
	own := map[string]bool{}  // файлы плагина по базовому имени
	defs := map[string]bool{} // функции, которые плагин определяет сам
	var names []string
	for path, f := range t.files {
		own[path[strings.LastIndex(path, "/")+1:]] = true
		if scriptLike(path, f.data) {
			names = append(names, path)
			for _, m := range reFuncDef.FindAllStringSubmatch(string(f.data), -1) {
				defs[m[1]+m[2]] = true
			}
		}
	}
	sort.Strings(names)
	for _, path := range names {
		for i, line := range strings.Split(string(t.files[path].data), "\n") {
			s := strings.TrimSpace(line)
			if s == "" || strings.HasPrefix(s, "#") || strings.HasPrefix(s, "//") {
				continue
			}
			at := func(what string) {
				report("граница API: %s:%d: %s — %s", path, i+1, what, "docs/splify2-api.md, раздел 9")
			}
			for _, ip := range internalPaths {
				if i := strings.Index(line, ip); i >= 0 {
					if ip == "/etc/steer" && internalPathExempt.MatchString(line[i:]) {
						continue
					}
					at(fmt.Sprintf("путь каркаса %s", ip))
				}
			}
			for _, f := range internalFiles {
				if own[f] {
					continue
				}
				if regexp.MustCompile(`(^|[^A-Za-z0-9_.-])` + regexp.QuoteMeta(f) + `\b`).MatchString(line) {
					at(fmt.Sprintf("файл каркаса %s", f))
				}
			}
			if m := internalVars.FindStringSubmatch(line); m != nil {
				at(fmt.Sprintf("переменная каркаса %s", m[1]))
			}
			for _, m := range reCmdWord.FindAllStringSubmatch(line, -1) {
				// `имя=значение` — присваивание, а метка `case` — не вызов.
				if m[2] == "=" || reCaseLabel.MatchString(line) || defs[m[1]] || !internalFuncs.MatchString(m[1]) {
					continue
				}
				at(fmt.Sprintf("функция каркаса %s", m[1]))
			}
			if reUciSplify2.MatchString(line) {
				at("настройка uci каркаса (splify2.*)")
			}
		}
	}
}
