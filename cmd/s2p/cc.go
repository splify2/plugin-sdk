package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	sdk "github.com/splify2/plugin-sdk"
)

// Сборка C под архитектуру OpenWrt через zig (cc, musl), без SDK OpenWrt — как recheck/build.sh и
// steer/build.sh. Нужен либо zig в PATH (S2P_ZIG — путь), либо docker с образом, в котором zig есть
// (S2P_BUILDER_IMAGE, по умолчанию steer-builder:wolfssl: zig 0.13.0).

type ccSpec struct {
	arch     string
	kind     string // helper | so | lib
	out      string
	srcs     []string
	include  []string
	defs     []string
	link     []string
	requires []string
	evline   bool
	std, opt string
	flags    []string
	soname   string
}

type toolchain struct {
	zig   string // путь к zig; пусто — docker
	image string
}

func findToolchain() (*toolchain, error) {
	if z := os.Getenv("S2P_ZIG"); z != "" {
		return &toolchain{zig: z}, nil
	}
	if z, err := exec.LookPath("zig"); err == nil {
		return &toolchain{zig: z}, nil
	}
	img := os.Getenv("S2P_BUILDER_IMAGE")
	if img == "" {
		img = "steer-builder:wolfssl"
	}
	if d, err := exec.LookPath("docker"); err == nil {
		if exec.Command(d, "image", "inspect", img).Run() == nil {
			return &toolchain{image: img}, nil
		}
		return nil, fmt.Errorf("zig не найден, а образа %s нет: поставьте zig 0.13 (S2P_ZIG=путь) или соберите образ steer-builder:wolfssl (steer/build/Dockerfile)", img)
	}
	return nil, fmt.Errorf("нужен zig 0.13 (в PATH или S2P_ZIG) либо docker с образом steer-builder:wolfssl")
}

func extractSDK() (string, error) {
	dir, err := os.MkdirTemp("", "s2p-sdk-")
	if err != nil {
		return "", err
	}
	for _, root := range []string{"include", "lib"} {
		err := fs.WalkDir(sdk.FS, root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			dst := filepath.Join(dir, p)
			if d.IsDir() {
				return os.MkdirAll(dst, 0755)
			}
			b, err := sdk.FS.ReadFile(p)
			if err != nil {
				return err
			}
			return os.WriteFile(dst, b, 0644)
		})
		if err != nil {
			return "", err
		}
	}
	return dir, nil
}

// run исполняет zig с аргументами args; пути в args — абсолютные. Docker: монтируются каталоги
// всех путей, процесс идёт от текущего пользователя (файлы не остаются у root).
func (tc *toolchain) run(args []string, paths []string) error {
	if tc.zig != "" {
		c := exec.Command(tc.zig, args...)
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		return c.Run()
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	cache = filepath.Join(cache, "s2p-zig")
	if err := os.MkdirAll(cache, 0755); err != nil {
		return err
	}
	da := []string{"run", "--rm", "--network=none", "--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"-e", "HOME=/tmp", "-e", "ZIG_GLOBAL_CACHE_DIR=/zig-cache", "-e", "ZIG_LOCAL_CACHE_DIR=/zig-cache/local",
		"-v", cache + ":/zig-cache"}
	var mounts []string
	for _, p := range paths {
		d := p
		if st, err := os.Stat(p); err != nil || !st.IsDir() {
			d = filepath.Dir(p)
		}
		covered := false
		for _, m := range mounts {
			if d == m || strings.HasPrefix(d, m+"/") {
				covered = true
			}
		}
		if !covered {
			mounts = append(mounts, d)
		}
	}
	for _, m := range mounts {
		da = append(da, "-v", m+":"+m)
	}
	wd, _ := os.Getwd()
	da = append(da, "-w", wd, "--entrypoint", "zig", tc.image)
	da = append(da, args...)
	c := exec.Command("docker", da...)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

func abs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		fatal("%v", err)
	}
	return a
}

func compile(tc *toolchain, s ccSpec) error {
	ai := findArch(s.arch)
	if ai == nil {
		return fmt.Errorf("архитектура %q неизвестна (s2p arches)", s.arch)
	}
	sdkDir, err := extractSDK()
	if err != nil {
		return err
	}
	defer os.RemoveAll(sdkDir)
	out := abs(s.out)
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	std, opt := s.std, s.opt
	if std == "" {
		std = "c11"
	}
	if opt == "" {
		opt = "s"
	}
	var srcs, paths []string
	for _, x := range s.srcs {
		srcs = append(srcs, abs(x))
	}
	if s.evline {
		srcs = append(srcs, filepath.Join(sdkDir, "lib/evline/evline.c"))
	}
	if len(srcs) == 0 {
		return fmt.Errorf("нет исходников")
	}
	paths = append(paths, srcs...)
	paths = append(paths, out, sdkDir)
	wd, _ := os.Getwd()
	paths = append(paths, wd)

	common := []string{"-target", ai.triple, "-mcpu=" + ai.mcpu, "-std=" + std, "-O" + opt,
		"-Wall", "-Wextra", "-D_GNU_SOURCE", "-fno-ident", "-ffunction-sections", "-fdata-sections",
		"-I" + filepath.Join(sdkDir, "include")}
	for _, i := range s.include {
		common = append(common, "-I"+abs(i))
		paths = append(paths, abs(i))
	}
	for _, d := range s.defs {
		common = append(common, "-D"+d)
	}
	common = append(common, s.flags...)

	switch s.kind {
	case "lib":
		tmp, err := os.MkdirTemp(filepath.Dir(out), ".s2p-obj-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		paths = append(paths, tmp)
		var objs []string
		for i, src := range srcs {
			o := filepath.Join(tmp, fmt.Sprintf("%d.o", i))
			a := append([]string{"cc", "-c", "-fPIC"}, common...)
			a = append(a, "-o", o, src)
			if err := tc.run(a, paths); err != nil {
				return fmt.Errorf("zig cc (%s): %v", ai.id, err)
			}
			objs = append(objs, o)
		}
		os.Remove(out)
		a := append([]string{"ar", "rcsD", out}, objs...) // D — без времени и владельца: байты повторяемы
		if err := tc.run(a, paths); err != nil {
			return fmt.Errorf("zig ar: %v", err)
		}
		return nil
	case "helper", "so":
	default:
		return fmt.Errorf("kind %q: helper, so или lib", s.kind)
	}

	a := append([]string{"cc"}, common...)
	a = append(a, "-Wl,--build-id=none", "-Wl,--gc-sections", "-s")
	if s.kind == "so" {
		soname := s.soname
		if soname == "" {
			soname = filepath.Base(out)
		}
		vs := filepath.Join(sdkDir, "exports.map")
		if err := os.WriteFile(vs, []byte("{ global: steer_plugin_v1_entry; local: *; };\n"), 0644); err != nil {
			return err
		}
		a = append(a, "-shared", "-fPIC", "-fvisibility=hidden", "-Wl,--version-script="+vs,
			"-Wl,-z,defs", "-Wl,-soname,"+soname)
	} else if len(s.requires) == 0 && len(s.link) == 0 {
		a = append(a, "-static")
	} else {
		a = append(a, "-Wl,--dynamic-linker="+ai.interp)
	}
	for _, r := range s.requires {
		// у зависимого плагина путь к библиотеке — относительно его файла (docs/manifest.md, «libs»)
		a = append(a, "-Wl,-rpath,$ORIGIN/../../"+r+"/lib")
	}
	for _, l := range s.link {
		a = append(a, abs(l))
		paths = append(paths, abs(l))
	}
	a = append(a, "-o", out)
	a = append(a, srcs...)
	if err := tc.run(a, paths); err != nil {
		return fmt.Errorf("zig cc (%s): %v", ai.id, err)
	}
	return nil
}

func cmdCC(args []string) {
	fs := flag.NewFlagSet("cc", flag.ExitOnError)
	arch := fs.String("arch", "", "архитектура (s2p arches) или all; при all в -o должно быть %a")
	kind := fs.String("kind", "helper", "helper — статическая программа, so — .so по steer-abi, lib — статическая библиотека .a")
	out := fs.String("o", "", "результат")
	evline := fs.Bool("evline", false, "добавить evline (include/s2p/evline.h, lib/evline/evline.c)")
	std := fs.String("std", "", "стандарт C (умолчание c11)")
	opt := fs.String("opt", "", "уровень оптимизации (умолчание s)")
	var inc, defs, req, link, flags multi
	fs.Var(&inc, "I", "каталог заголовков")
	fs.Var(&defs, "D", "макрос")
	fs.Var(&req, "requires", "плагин-библиотека, на которую ссылается программа (RUNPATH $ORIGIN/../../<id>/lib)")
	fs.Var(&link, "link", "файл .so или .a для связи")
	fs.Var(&flags, "flag", "ещё один флаг компилятора")
	srcs := parseInterleaved(fs, args)
	if *arch == "" || *out == "" {
		fatal("cc: нужны --arch и -o")
	}
	tc, err := findToolchain()
	if err != nil {
		fatal("%v", err)
	}
	list := []string{*arch}
	if *arch == "all" {
		if !strings.Contains(*out, "%a") {
			fatal("cc: при --arch all в -o нужно %%a (имя архитектуры)")
		}
		list = archIDs()
	}
	for _, a := range list {
		dst := strings.ReplaceAll(*out, "%a", a)
		err := compile(tc, ccSpec{arch: a, kind: *kind, out: dst, srcs: srcs, include: inc, defs: defs,
			link: link, requires: req, evline: *evline, std: *std, opt: *opt, flags: flags})
		if err != nil {
			fatal("%v", err)
		}
		fmt.Printf("%s: %s\n", a, dst)
	}
}
