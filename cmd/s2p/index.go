package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

type indexMeta struct {
	Plugins map[string]struct {
		Default   *bool             `json:"default"`
		KeyRole   string            `json:"key_role"`
		Tags      []string          `json:"tags"`
		Homepage  string            `json:"homepage"`
		Changelog map[string]string `json:"changelog"`
	} `json:"plugins"`
}

func jsonRoundtrip(v any) any {
	b, _ := json.Marshal(v)
	var out any
	json.Unmarshal(b, &out)
	return out
}

func cmdIndex(args []string) {
	fl := flag.NewFlagSet("index", flag.ExitOnError)
	var pubs, mirrors, keys, drops, yanks multi
	fl.Var(&pubs, "pub", "доверенный ключ, которым подписаны архивы; можно повторять")
	fl.Var(&mirrors, "mirror", "базовый адрес HTTPS репозитория; можно повторять")
	fl.Var(&keys, "key", "закрытый ключ роли index (или ключ репозитория); можно повторять — ротация")
	fl.Var(&drops, "drop", "убрать версию из индекса: <id>@<версия>")
	fl.Var(&yanks, "yank", "отозвать версию: <id>@<версия>, причина — --why")
	repo := fl.String("repo", "", "id репозитория (у официального — splify2)")
	name := fl.String("name", "", "имя репозитория для человека")
	homepage := fl.String("homepage", "", "")
	desc := fl.String("description", "", "")
	expires := fl.String("expires", "30d", "срок действия индекса: 30d, 720h")
	out := fl.String("o", "", "каталог репозитория: сюда пишутся index.json, index.json.sig и <id>/<версия>/*.s2p")
	prev := fl.String("prev", "", "прежний индекс для слияния (умолчание — <o>/index.json, если он есть)")
	meta := fl.String("meta", "", "JSON с полями плагинов вне манифеста: default, key_role, tags, changelog")
	why := fl.String("why", "", "причина отзыва для --yank")
	security := fl.Bool("security", false, "отзыв по безопасности: автообновление меняет версию сразу")
	replace := fl.Bool("replace", false, "заменить архив, если у той же версии и архитектуры другая сумма")
	unsigned := fl.Bool("allow-unsigned", false, "принять архивы без подписи (только для проверки на стенде)")
	now := fl.String("now", "", "время сборки (RFC 3339); умолчание — SOURCE_DATE_EPOCH или сейчас")
	pos := parseInterleaved(fl, args)
	if len(pos) != 1 {
		fatal("index: нужен каталог с .s2p")
	}
	if *repo == "" || *out == "" || len(keys) == 0 {
		fatal("index: нужны --repo, -o и --key")
	}
	if len(yanks) > 0 && *why == "" {
		fatal("index: --yank требует --why")
	}
	tr := newTrust()
	addTrust(tr, pubs, nil, "", "")
	if len(tr.keys) == 0 && !*unsigned {
		fatal("index: нет доверенных ключей архивов (--pub): индекс не принимает неподписанное")
	}
	secs := loadSecs(keys)
	gen := nowUTC(*now)
	dur, err := parseDuration(*expires)
	if err != nil {
		fatal("--expires: %v", err)
	}

	// прежний индекс
	plugins := map[string]any{}
	prevPath := *prev
	if prevPath == "" {
		if _, err := os.Stat(filepath.Join(*out, "index.json")); err == nil {
			prevPath = filepath.Join(*out, "index.json")
		}
	}
	repoDoc := map[string]any{"id": *repo}
	if prevPath != "" {
		doc, err := decodeJSON(readFile(prevPath))
		if err != nil {
			fatal("%s: %v", prevPath, err)
		}
		if e := mustValidator("index.schema.json").check(doc); len(e) > 0 {
			fatal("%s не по схеме: %s", prevPath, strings.Join(e, "; "))
		}
		d := doc.(map[string]any)
		plugins = d["plugins"].(map[string]any)
		if r := d["repo"].(map[string]any); r["id"] != *repo {
			fatal("прежний индекс — репозитория %v, а не %s", r["id"], *repo)
		}
		if g, err := time.Parse(time.RFC3339, d["generated"].(string)); err == nil && gen.Before(g) {
			fatal("время сборки %s раньше прежнего индекса (%s): менеджер не принимает индекс старее виденного", gen.Format(time.RFC3339), g.Format(time.RFC3339))
		}
		if r, ok := d["repo"].(map[string]any); ok {
			for k, v := range r {
				repoDoc[k] = v
			}
		}
		if m, ok := d["mirrors"].([]any); ok && len(mirrors) == 0 {
			for _, x := range m {
				mirrors = append(mirrors, x.(string))
			}
		}
	}
	if *name != "" {
		repoDoc["name"] = *name
	}
	if repoDoc["name"] == nil {
		repoDoc["name"] = *repo
	}
	if *homepage != "" {
		repoDoc["homepage"] = *homepage
	}
	if *desc != "" {
		repoDoc["description"] = *desc
	}
	var im indexMeta
	if *meta != "" {
		if err := json.Unmarshal(readFile(*meta), &im); err != nil {
			fatal("%s: %v", *meta, err)
		}
	}

	// архивы
	var files []string
	filepath.WalkDir(pos[0], func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".s2p") {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	if len(files) == 0 && prevPath == "" {
		fatal("index: в %s нет .s2p", pos[0])
	}
	type copyJob struct{ src, rel string }
	var copies []copyJob
	newest := map[string]version{} // самая новая версия, пришедшая сейчас, по id
	for _, f := range files {
		data := readFile(f)
		a, err := openArchive(data)
		if err != nil {
			fatal("%s: %v", f, err)
		}
		p, err := checkHeader(a)
		if err != nil {
			fatal("%s: %v", f, err)
		}
		if len(strings.TrimSpace(string(a.sigText))) == 0 {
			if !*unsigned {
				fatal("%s: архив не подписан", f)
			}
		} else if err := tr.verify("MANIFEST "+p.ID, a.mtext, a.sigText); err != nil {
			if !*unsigned {
				fatal("%s: %v", f, err)
			}
		}
		o := lintOpts{}
		if a.manifest.arch != "all" {
			o.arch = a.manifest.arch
		}
		if l := lintTree(treeFromArchive(a), o); l.errors() > 0 {
			printDiags(l, f)
			fatal("%s: не прошёл проверку", f)
		}
		rel := p.ID + "/" + p.Version + "/" + filepath.Base(f)
		asset := map[string]any{"path": rel, "sha256": sha256hex(data), "size": len(data), "installed": a.manifest.installed()}
		if p.Standalone != nil {
			asset["standalone"] = true
		}
		ver := map[string]any{
			"version":  p.Version,
			"compat":   jsonRoundtrip(p.Compat),
			"provides": jsonRoundtrip(p.provides()),
		}
		if len(p.Requires.Plugins)+len(p.Requires.System)+len(p.Requires.Kernel) > 0 {
			r := map[string]any{}
			if len(p.Requires.Plugins) > 0 {
				r["plugins"] = jsonRoundtrip(p.Requires.Plugins)
			}
			if len(p.Requires.System) > 0 {
				r["system"] = jsonRoundtrip(p.Requires.System)
			}
			if len(p.Requires.Kernel) > 0 {
				r["kernel"] = jsonRoundtrip(p.Requires.Kernel)
			}
			ver["requires"] = r
		}
		if len(p.Conflicts) > 0 {
			ver["conflicts"] = jsonRoundtrip(p.Conflicts)
		}
		pl, _ := plugins[p.ID].(map[string]any)
		if pl == nil {
			pl = map[string]any{"versions": []any{}}
			plugins[p.ID] = pl
		}
		pv, _ := parseVersion(p.Version)
		if cur, ok := newest[p.ID]; !ok || cmpVersion(pv, cur) > 0 {
			newest[p.ID] = pv
			pl["name"] = p.Name
			pl["type"] = firstNonEmpty(p.Type, "plugin")
			setOrDel(pl, "description", p.Description)
			setOrDel(pl, "license", p.License)
		}
		var target map[string]any
		for _, x := range pl["versions"].([]any) {
			if x.(map[string]any)["version"] == p.Version {
				target = x.(map[string]any)
			}
		}
		if target == nil {
			ver["released"] = gen.Format(time.RFC3339)
			if m, ok := im.Plugins[p.ID]; ok {
				if c := m.Changelog[p.Version]; c != "" {
					ver["changelog"] = c
				}
			}
			ver["assets"] = map[string]any{}
			pl["versions"] = append(pl["versions"].([]any), ver)
			target = ver
		} else {
			// все архитектуры и варианты версии обязаны совпасть по тегам, зависимостям и видам
			for _, k := range []string{"compat", "requires", "conflicts", "provides"} {
				if !reflect.DeepEqual(target[k], ver[k]) {
					fatal("%s: поле %s у %s %s расходится с уже записанным (архивы одной версии обязаны совпадать, docs/manifest.md, «Варианты»)", f, k, p.ID, p.Version)
				}
			}
		}
		assets := target["assets"].(map[string]any)
		key := p.assetKey(a.manifest.arch)
		if old, ok := assets[key].(map[string]any); ok && old["sha256"] != asset["sha256"] && !*replace {
			fatal("%s: %s %s %s уже в индексе с другой суммой (%v); сборка воспроизводима — разберитесь, откуда расхождение, или --replace", f, p.ID, p.Version, key, old["sha256"])
		}
		assets[key] = asset
		if m, ok := im.Plugins[p.ID]; ok {
			if m.Default != nil {
				pl["default"] = *m.Default
			}
			setOrDel(pl, "key_role", m.KeyRole)
			if len(m.Tags) > 0 {
				pl["tags"] = jsonRoundtrip(m.Tags)
			}
			setOrDel(pl, "homepage", m.Homepage)
		}
		copies = append(copies, copyJob{f, rel})
	}
	// --drop и --yank
	find := func(spec string) (map[string]any, map[string]any, string) {
		i := strings.LastIndex(spec, "@")
		if i < 0 {
			fatal("%q: нужно <id>@<версия>", spec)
		}
		id, v := spec[:i], spec[i+1:]
		pl, ok := plugins[id].(map[string]any)
		if !ok {
			fatal("в индексе нет плагина %s", id)
		}
		for _, x := range pl["versions"].([]any) {
			if x.(map[string]any)["version"] == v {
				return pl, x.(map[string]any), id
			}
		}
		fatal("в индексе нет %s", spec)
		return nil, nil, ""
	}
	for _, d := range drops {
		pl, ver, id := find(d)
		var keep []any
		for _, x := range pl["versions"].([]any) {
			if x.(map[string]any)["version"] != ver["version"] {
				keep = append(keep, x)
			}
		}
		if len(keep) == 0 {
			delete(plugins, id)
		} else {
			pl["versions"] = keep
		}
	}
	for _, y := range yanks {
		_, ver, _ := find(y)
		ver["yanked"] = map[string]any{"why": *why, "security": *security}
	}
	// порядок версий в файле: новые первыми (менеджер всё равно сортирует сам)
	for _, pl := range plugins {
		vs := pl.(map[string]any)["versions"].([]any)
		sort.SliceStable(vs, func(i, j int) bool {
			a, _ := parseVersion(vs[i].(map[string]any)["version"].(string))
			b, _ := parseVersion(vs[j].(map[string]any)["version"].(string))
			return cmpVersion(a, b) > 0
		})
	}
	idx := map[string]any{
		"schema":    1,
		"repo":      repoDoc,
		"generated": gen.Format(time.RFC3339),
		"expires":   gen.Add(dur).Format(time.RFC3339),
		"plugins":   plugins,
	}
	if len(mirrors) > 0 {
		idx["mirrors"] = []string(mirrors)
	}
	doc := jsonRoundtrip(idx)
	if e := mustValidator("index.schema.json").check(doc); len(e) > 0 {
		fatal("собранный индекс не по схеме: %s", strings.Join(e, "; "))
	}
	body, _ := json.MarshalIndent(doc, "", "  ")
	body = append(body, '\n')
	if err := os.MkdirAll(*out, 0755); err != nil {
		fatal("%v", err)
	}
	for _, c := range copies {
		dst := filepath.Join(*out, filepath.FromSlash(c.rel))
		if sa, err := filepath.Abs(c.src); err == nil {
			if da, _ := filepath.Abs(dst); da == sa {
				continue
			}
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			fatal("%v", err)
		}
		if err := os.WriteFile(dst, readFile(c.src), 0644); err != nil {
			fatal("%v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(*out, "index.json"), body, 0644); err != nil {
		fatal("%v", err)
	}
	if err := os.WriteFile(filepath.Join(*out, "index.json.sig"), signAll(secs, body), 0644); err != nil {
		fatal("%v", err)
	}
	n := 0
	for _, pl := range plugins {
		n += len(pl.(map[string]any)["versions"].([]any))
	}
	fmt.Printf("%s/index.json: плагинов %d, версий %d, подписан %d ключом(ами), действует до %s\n", *out, len(plugins), n, len(secs), idx["expires"])
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func setOrDel(m map[string]any, k, v string) {
	if v == "" {
		delete(m, k)
	} else {
		m[k] = v
	}
}

func verifyIndexCmd(path string, tr *trust, base, nowFlag string) {
	body := readFile(path)
	sigPath := path + ".sig"
	sig, err := os.ReadFile(sigPath)
	if err != nil {
		fatal("%s: нет подписи рядом (%s)", path, sigPath)
	}
	if len(tr.keys) == 0 {
		fatal("нет доверенных ключей (--pub или --keys-dir --role index)")
	}
	// сначала подпись, потом всё остальное (docs/index.md, «Подпись индекса»)
	if err := tr.verify("index.json", body, sig); err != nil {
		fatal("%s: %v", path, err)
	}
	doc, err := decodeJSON(body)
	if err != nil {
		fatal("%s: %v", path, err)
	}
	if e := mustValidator("index.schema.json").check(doc); len(e) > 0 {
		fatal("%s не по схеме: %s", path, strings.Join(e, "; "))
	}
	d := doc.(map[string]any)
	exp, _ := time.Parse(time.RFC3339, d["expires"].(string))
	if nowUTC(nowFlag).After(exp) {
		fatal("%s: срок индекса вышел (%s)", path, d["expires"])
	}
	checked := 0
	if base != "" {
		for id, p := range d["plugins"].(map[string]any) {
			for _, v := range p.(map[string]any)["versions"].([]any) {
				for key, a := range v.(map[string]any)["assets"].(map[string]any) {
					am := a.(map[string]any)
					rel, ok := am["path"].(string)
					if !ok {
						continue
					}
					b, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(rel)))
					if err != nil {
						fatal("%s %v %s: %v", id, v.(map[string]any)["version"], key, err)
					}
					if sha256hex(b) != am["sha256"] || float64(len(b)) != am["size"].(float64) {
						fatal("%s: сумма или размер не совпадают с индексом", rel)
					}
					checked++
				}
			}
		}
	}
	fmt.Printf("%s: ok — репозиторий %v, сформирован %v, действует до %v", path, d["repo"].(map[string]any)["id"], d["generated"], d["expires"])
	if base != "" {
		fmt.Printf(", архивов сверено %d", checked)
	}
	fmt.Println()
}
