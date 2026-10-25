package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Проект плагина — каталог с plugin.json и (необязательно) s2p.json: сборочные шаги SDK, которые в
// архив не попадают. Всё остальное, кроме исходников и служебного, ложится в архив как есть.

type ccStep struct {
	Kind     string   `json:"kind"` // helper | so
	Out      string   `json:"out"`
	Src      []string `json:"src"`
	Evline   bool     `json:"evline"`
	Requires []string `json:"requires"` // плагины-библиотеки: путь поиска $ORIGIN/../../<id>/lib
	Link     []string `json:"link"`     // файлы .so/.a для связи (в проекте)
	Defs     []string `json:"defs"`
	Include  []string `json:"include"`
	Flags    []string `json:"flags"`
	Std      string   `json:"std"`
	Opt      string   `json:"opt"`
}

type uiStep struct {
	Dir string   `json:"dir"` // где запускать, от корня проекта
	Cmd []string `json:"cmd"`
	Out string   `json:"out"` // куда в архиве ложится результат; команда получает S2P_UI_OUT
}

type projCfg struct {
	CC      []ccStep `json:"cc"`
	UI      *uiStep  `json:"ui"`
	Exclude []string `json:"exclude"`
}

var defaultExclude = []string{
	"s2p.json", "package.json", "package-lock.json", "tsconfig.json", "vite.config.ts", "vite.config.js",
	"README.md", "README", "src", "ui-src", "build", "dist", "node_modules", ".git", ".s2p-stage", ".gitignore",
}

func loadCfg(dir string) (*projCfg, error) {
	var c projCfg
	b, err := os.ReadFile(filepath.Join(dir, "s2p.json"))
	if os.IsNotExist(err) {
		return &c, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("s2p.json: %v", err)
	}
	for i, s := range c.CC {
		if s.Kind != "helper" && s.Kind != "so" {
			return nil, fmt.Errorf("s2p.json: cc[%d].kind %q — нужен helper или so", i, s.Kind)
		}
		if s.Out == "" || len(s.Src) == 0 {
			return nil, fmt.Errorf("s2p.json: cc[%d] без out или src", i)
		}
		if err := checkPath(s.Out); err != nil {
			return nil, fmt.Errorf("s2p.json: cc[%d].out: %v", i, err)
		}
	}
	if c.UI != nil {
		if len(c.UI.Cmd) == 0 || c.UI.Out == "" {
			return nil, fmt.Errorf("s2p.json: ui требует cmd и out")
		}
		if err := checkPath(c.UI.Out); err != nil {
			return nil, fmt.Errorf("s2p.json: ui.out: %v", err)
		}
	}
	return &c, nil
}

func (c *projCfg) excluded(rel string) bool {
	top := strings.SplitN(rel, "/", 2)[0]
	for _, e := range defaultExclude {
		if top == e {
			return true
		}
	}
	for _, e := range c.Exclude {
		if rel == e || strings.HasPrefix(rel, strings.TrimSuffix(e, "/")+"/") {
			return true
		}
		if ok, _ := path.Match(e, rel); ok {
			return true
		}
		if ok, _ := path.Match(e, path.Base(rel)); ok {
			return true
		}
	}
	return false
}

// tree — то, что ляжет в архив (или лежит в нём).
type tree struct {
	files     map[string]fileSpec
	generated map[string]bool // пути, которые создаст сборка (при проверке проекта до сборки)
	genDirs   []string        // каталоги, которые заполнит сборка (результат ui)
	fromDir   string
}

func (t *tree) has(p string) bool {
	if _, ok := t.files[p]; ok {
		return true
	}
	if t.generated[p] {
		return true
	}
	for _, d := range t.genDirs {
		if strings.HasPrefix(p, d+"/") {
			return true
		}
	}
	return false
}

func (t *tree) generatedOnly(p string) bool {
	if _, ok := t.files[p]; ok {
		return false
	}
	return t.has(p)
}

func (t *tree) mode(p string) int64 {
	if f, ok := t.files[p]; ok {
		return f.mode
	}
	return 0755
}

func (t *tree) names() []string {
	var n []string
	for k := range t.files {
		n = append(n, k)
	}
	sort.Strings(n)
	return n
}

// collectTree читает проект по правилам исключений.
func collectTree(dir string, cfg *projCfg) (*tree, error) {
	t := &tree{files: map[string]fileSpec{}, generated: map[string]bool{}, fromDir: dir}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if cfg.excluded(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			return fmt.Errorf("%s: ссылка — в архив попадают только файлы и каталоги", rel)
		case d.IsDir():
			return nil
		case !info.Mode().IsRegular():
			return fmt.Errorf("%s: не обычный файл", rel)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		mode := int64(0644)
		if info.Mode().Perm()&0o111 != 0 {
			mode = 0755
		}
		t.files[rel] = fileSpec{mode: mode, data: data}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, s := range cfg.CC {
		t.generated[s.Out] = true
	}
	if cfg.UI != nil {
		t.genDirs = append(t.genDirs, strings.TrimSuffix(cfg.UI.Out, "/"))
	}
	return t, nil
}

func treeFromArchive(a *archive) *tree {
	return &tree{files: a.files, generated: map[string]bool{}}
}
