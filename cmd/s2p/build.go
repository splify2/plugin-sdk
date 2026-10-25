package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func archiveName(p *pluginDoc, arch string) string {
	return p.ID + "-" + p.Version + "-" + p.assetKey(arch) + ".s2p"
}

// runUI запускает сборку интерфейса один раз и кладёт результат в t.files под ui.out.
func runUI(dir string, cfg *projCfg, t *tree) error {
	u := cfg.UI
	stage := abs(filepath.Join(dir, ".s2p-stage", "ui"))
	os.RemoveAll(stage)
	if err := os.MkdirAll(stage, 0755); err != nil {
		return err
	}
	c := exec.Command(u.Cmd[0], u.Cmd[1:]...)
	c.Dir = filepath.Join(dir, u.Dir)
	c.Env = append(os.Environ(), "S2P_UI_OUT="+stage)
	c.Stdout, c.Stderr = os.Stderr, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("сборка интерфейса (%s): %v", strings.Join(u.Cmd, " "), err)
	}
	n := 0
	err := filepath.WalkDir(stage, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(stage, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		t.files[strings.TrimSuffix(u.Out, "/")+"/"+filepath.ToSlash(rel)] = fileSpec{0644, b}
		n++
		return nil
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("сборка интерфейса ничего не положила в S2P_UI_OUT (%s)", stage)
	}
	return nil
}

func cmdBuild(args []string) {
	fl := flag.NewFlagSet("build", flag.ExitOnError)
	var archFlags, keys multi
	fl.Var(&archFlags, "arch", "архитектура; можно повторять, all — все из манифеста (умолчание)")
	fl.Var(&keys, "key", "закрытый ключ usign; можно повторять — несколько блоков подписи")
	outDir := fl.String("o", "dist", "каталог для .s2p")
	unsigned := fl.Bool("unsigned", false, "собрать без подписи (MANIFEST.sig пустой) — только для разработки")
	noUI := fl.Bool("no-ui", false, "не собирать интерфейс: взять уже собранный")
	pos := parseInterleaved(fl, args)
	dir := "."
	if len(pos) > 1 {
		fatal("build: один каталог проекта")
	} else if len(pos) == 1 {
		dir = pos[0]
	}
	if len(keys) == 0 && !*unsigned {
		fatal("build: нужен --key <ключ> (или --unsigned для разработки)")
	}
	cfg, err := loadCfg(dir)
	if err != nil {
		fatal("%v", err)
	}
	t, err := collectTree(dir, cfg)
	if err != nil {
		fatal("%v", err)
	}
	// схема и права — до сборки: ошибку в манифесте не нужно ждать минуты компиляции
	pre := lintTree(t, lintOpts{})
	printDiags(pre, dir)
	if pre.errors() > 0 {
		fatal("%s: манифест не прошёл проверку (ошибок %d)", dir, pre.errors())
	}
	p, _ := parsePlugin(t.files[fPlugin].data)
	list, all, _ := p.archs()
	want := []string(archFlags)
	if len(want) == 0 || (len(want) == 1 && want[0] == "all") {
		want = list
	}
	if all {
		want = []string{"all"}
	}
	for _, a := range want {
		ok := all && a == "all"
		for _, x := range list {
			if x == a {
				ok = true
			}
		}
		if !ok {
			fatal("архитектура %s не названа в plugin.json", a)
		}
	}
	if len(cfg.CC) > 0 && all {
		fatal("в s2p.json есть cc, а plugin.json: arch all — машинный код требует список архитектур")
	}
	var secs []secKey
	if len(keys) > 0 {
		secs = loadSecs(keys)
	}
	if cfg.UI != nil && !*noUI {
		if err := runUI(dir, cfg, t); err != nil {
			fatal("%v", err)
		}
	} else if cfg.UI != nil {
		// уже собранный результат — из .s2p-stage/ui
		stage := filepath.Join(dir, ".s2p-stage", "ui")
		if st, err := os.Stat(stage); err != nil || !st.IsDir() {
			fatal("--no-ui: нет %s, сначала соберите интерфейс", stage)
		}
		filepath.WalkDir(stage, func(pth string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(stage, pth)
			b, _ := os.ReadFile(pth)
			t.files[strings.TrimSuffix(cfg.UI.Out, "/")+"/"+filepath.ToSlash(rel)] = fileSpec{0644, b}
			return nil
		})
	}
	var tc *toolchain
	if len(cfg.CC) > 0 {
		if tc, err = findToolchain(); err != nil {
			fatal("%v", err)
		}
	}
	if err := os.MkdirAll(*outDir, 0755); err != nil {
		fatal("%v", err)
	}
	for _, arch := range want {
		files := map[string]fileSpec{}
		for k, v := range t.files {
			files[k] = v
		}
		for i, s := range cfg.CC {
			tmp := filepath.Join(dir, ".s2p-stage", arch, fmt.Sprintf("cc%d", i))
			os.RemoveAll(tmp)
			var link []string
			for _, l := range s.Link {
				link = append(link, filepath.Join(dir, l))
			}
			var inc []string
			for _, x := range s.Include {
				inc = append(inc, filepath.Join(dir, x))
			}
			var srcs []string
			for _, x := range s.Src {
				srcs = append(srcs, filepath.Join(dir, x))
			}
			var defs []string
			for _, d := range s.Defs {
				defs = append(defs, strings.NewReplacer("{id}", p.ID, "{version}", p.Version).Replace(d))
			}
			spec := ccSpec{arch: arch, kind: s.Kind, out: tmp, srcs: srcs, include: inc, defs: defs,
				link: link, requires: s.Requires, evline: s.Evline, std: s.Std, opt: s.Opt, flags: s.Flags, soname: filepath.Base(s.Out)}
			fmt.Fprintf(os.Stderr, "[%s] %s → %s\n", arch, strings.Join(s.Src, " "), s.Out)
			if err := compile(tc, spec); err != nil {
				fatal("%v", err)
			}
			b, err := os.ReadFile(tmp)
			if err != nil {
				fatal("%v", err)
			}
			files[s.Out] = fileSpec{0755, b}
		}
		final := &tree{files: files, generated: map[string]bool{}}
		l := lintTree(final, lintOpts{arch: arch})
		printDiags(l, archiveName(p, arch))
		if l.errors() > 0 {
			fatal("%s: не прошёл проверку (ошибок %d)", archiveName(p, arch), l.errors())
		}
		mtext := manifestText(p.ID, p.Version, arch, files)
		var sig []byte
		if len(secs) > 0 {
			sig = signAll(secs, mtext)
		} else {
			fmt.Fprintf(os.Stderr, "%s: предупреждение: архив без подписи (MANIFEST.sig пустой); ставится только --unsigned\n", archiveName(p, arch))
		}
		data, err := writeArchive(orderedEntries(files, sig, mtext))
		if err != nil {
			fatal("%v", err)
		}
		if _, err := openArchive(data); err != nil {
			fatal("внутренняя ошибка: собранный архив не читается: %v", err)
		}
		dst := filepath.Join(*outDir, archiveName(p, arch))
		if err := os.WriteFile(dst, data, 0644); err != nil {
			fatal("%v", err)
		}
		fmt.Printf("%s  %d байт  sha256 %s\n", dst, len(data), sha256hex(data))
	}
}
