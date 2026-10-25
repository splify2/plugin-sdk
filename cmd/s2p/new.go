package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	sdk "github.com/splify2/plugin-sdk"
)

// Идентификатор каждого шаблона: им он собирается как есть, а `s2p new` заменяет его на ваш.
var templateIDs = map[string]string{
	"helper": "example-helper",
	"so":     "example-so",
	"ui":     "example-ui",
	"theme":  "example-theme",
	"lang":   "lang-de",
}

func cmdNew(args []string) {
	fs_ := flag.NewFlagSet("new", flag.ExitOnError)
	id := fs_.String("id", "", "id плагина (умолчание — имя каталога)")
	name := fs_.String("name", "", "имя для человека (умолчание — id)")
	pos := parseInterleaved(fs_, args)
	if len(pos) != 2 {
		var names []string
		for k := range templateIDs {
			names = append(names, k)
		}
		sort.Strings(names)
		fatal("new: s2p new <шаблон> <каталог>; шаблоны: %s", strings.Join(names, ", "))
	}
	tmpl, dir := pos[0], pos[1]
	old, ok := templateIDs[tmpl]
	if !ok {
		fatal("new: шаблона %q нет", tmpl)
	}
	nid := *id
	if nid == "" {
		nid = filepath.Base(dir)
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`).MatchString(nid) {
		fatal("new: id %q — нужно [a-z0-9][a-z0-9-]*", nid)
	}
	repl := []string{old, nid, strings.ReplaceAll(old, "-", "_"), strings.ReplaceAll(nid, "-", "_")}
	rename := map[string]string{}
	if tmpl == "lang" {
		code := strings.TrimPrefix(nid, "lang-")
		if code == nid || !regexp.MustCompile(`^[a-z]{2,3}(-[A-Z]{2})?$`).MatchString(code) {
			fatal("new: id языка — lang-<код>, например lang-fr")
		}
		hn := *name
		if hn == "" {
			hn = strings.ToUpper(code)
		}
		repl = append(repl, "Deutsch", hn, `"de"`, `"`+code+`"`, "/de.json", "/"+code+".json")
		rename["de.json"] = code + ".json"
	} else if *name != "" {
		repl = append(repl, "Пример", *name)
	}
	r := strings.NewReplacer(repl...)
	if _, err := os.Stat(dir); err == nil {
		fatal("new: %s уже есть", dir)
	}
	root := "templates/" + tmpl
	err := fs.WalkDir(sdk.FS, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, root), "/")
		if rel == "" {
			return os.MkdirAll(dir, 0755)
		}
		rel = r.Replace(rel)
		if rn, ok := rename[path.Base(rel)]; ok {
			rel = path.Join(path.Dir(rel), rn)
		}
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(dst, 0755)
		}
		b, err := sdk.FS.ReadFile(p)
		if err != nil {
			return err
		}
		mode := os.FileMode(0644)
		if strings.HasPrefix(rel, "scripts/") || strings.HasPrefix(rel, "rpc/") {
			mode = 0755
		}
		return os.WriteFile(dst, []byte(r.Replace(string(b))), mode)
	})
	if err != nil {
		fatal("%v", err)
	}
	fmt.Printf("%s: заготовка %q (id %s). Дальше: cd %s && cat README.md\n", dir, tmpl, nid, dir)
}
