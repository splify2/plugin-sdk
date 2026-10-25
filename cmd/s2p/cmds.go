package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func readFile(p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		fatal("%v", err)
	}
	return b
}

// checkHeader сверяет заголовок MANIFEST с plugin.json (docs/s2p.md, «MANIFEST»).
func checkHeader(a *archive) (*pluginDoc, error) {
	p, err := parsePlugin(a.files[fPlugin].data)
	if err != nil {
		return nil, fmt.Errorf("plugin.json: %v", err)
	}
	m := a.manifest
	if p.ID != m.id || p.Version != m.ver {
		return p, fmt.Errorf("заголовок MANIFEST (%s %s) не совпадает с plugin.json (%s %s)", m.id, m.ver, p.ID, p.Version)
	}
	list, all, err := p.archs()
	if err != nil {
		return p, err
	}
	if all {
		if m.arch != "all" {
			return p, fmt.Errorf("plugin.json: arch all, а в MANIFEST %s", m.arch)
		}
	} else {
		ok := false
		for _, x := range list {
			if x == m.arch {
				ok = true
			}
		}
		if !ok {
			return p, fmt.Errorf("архитектура %s из MANIFEST не названа в plugin.json (%s)", m.arch, strings.Join(list, ", "))
		}
	}
	return p, nil
}

func cmdLint(args []string) {
	fs := flag.NewFlagSet("lint", flag.ExitOnError)
	arch := fs.String("arch", "", "сверять машинный код с этой архитектурой (для каталога)")
	schemaOnly := fs.Bool("schema-only", false, "только схема манифеста")
	strict := fs.Bool("strict", false, "предупреждения — тоже отказ")
	official := fs.Bool("official", false, "плагин официального репозитория: обращение к внутренностям каркаса — отказ (splify2-api, раздел 9)")
	pos := parseInterleaved(fs, args)
	if len(pos) != 1 {
		fatal("lint: нужен один путь: каталог проекта, .s2p или plugin.json")
	}
	target := pos[0]
	st, err := os.Stat(target)
	if err != nil {
		fatal("%v", err)
	}
	var t *tree
	o := lintOpts{arch: *arch, schemaOnly: *schemaOnly, official: *official}
	switch {
	case st.IsDir():
		cfg, err := loadCfg(target)
		if err != nil {
			fatal("%v", err)
		}
		t, err = collectTree(target, cfg)
		if err != nil {
			fatal("%v", err)
		}
	case strings.HasSuffix(target, ".s2p"):
		a, err := openArchive(readFile(target))
		if err != nil {
			fatal("%s: %v", target, err)
		}
		if _, err := checkHeader(a); err != nil {
			fatal("%s: %v", target, err)
		}
		t = treeFromArchive(a)
		if a.manifest.arch != "all" {
			o.arch = a.manifest.arch
		}
	default:
		t = &tree{files: map[string]fileSpec{fPlugin: {0644, readFile(target)}}}
		o.schemaOnly = true
	}
	l := lintTree(t, o)
	printDiags(l, target)
	warns := len(l.ds) - l.errors()
	if l.errors() > 0 || (*strict && warns > 0) {
		fmt.Fprintf(os.Stderr, "%s: ошибок %d, предупреждений %d\n", target, l.errors(), warns)
		os.Exit(1)
	}
	fmt.Printf("%s: ok (предупреждений %d)\n", target, warns)
}

func loadSecs(paths []string) []secKey {
	var ks []secKey
	for _, p := range paths {
		k, err := parseSec(readFile(p))
		if err != nil {
			fatal("%s: %v", p, err)
		}
		ks = append(ks, k)
	}
	return ks
}

func signAll(ks []secKey, msg []byte) []byte {
	var bs []sigBlock
	for _, k := range ks {
		bs = append(bs, k.sign(msg))
	}
	return formatSigs(bs)
}

func cmdKeygen(args []string) {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	out := fs.String("o", "", "имя: пишутся <имя>.sec и <имя>.pub")
	comment := fs.String("c", "s2p key", "комментарий в файлах")
	parseInterleaved(fs, args)
	if *out == "" {
		fatal("keygen: нужен -o <имя>")
	}
	k, err := genKey()
	if err != nil {
		fatal("%v", err)
	}
	if err := os.WriteFile(*out+".sec", formatSec(k, *comment+" (secret)"), 0600); err != nil {
		fatal("%v", err)
	}
	if err := os.WriteFile(*out+".pub", formatPub(k.pub(), *comment+" (public)"), 0644); err != nil {
		fatal("%v", err)
	}
	fmt.Printf("ключ %s: %s.sec (секрет, 0600), %s.pub\n", k.num, *out, *out)
}

func addTrust(t *trust, pubs, revoked []string, keysDir, role string) {
	for _, p := range pubs {
		if err := t.addFile(p); err != nil {
			fatal("%v", err)
		}
	}
	if keysDir != "" {
		if role == "" {
			fatal("--keys-dir требует --role (index, protocols или plugins)")
		}
		if err := t.addRole(keysDir, role); err != nil {
			fatal("%v", err)
		}
	}
	for _, r := range revoked {
		if err := t.addRevoked(r); err != nil {
			fatal("%v", err)
		}
	}
}

func cmdSign(args []string) {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	var keys multi
	fs.Var(&keys, "key", "закрытый ключ usign без пароля; можно повторять — несколько блоков для ротации")
	add := fs.Bool("add", false, "оставить прежние блоки и добавить новые")
	out := fs.String("o", "", "куда писать (по умолчанию — на месте)")
	pos := parseInterleaved(fs, args)
	if len(pos) != 1 || len(keys) == 0 {
		fatal("sign: нужны <файл.s2p> и хотя бы один --key")
	}
	data := readFile(pos[0])
	a, err := openArchive(data)
	if err != nil {
		fatal("%s: %v", pos[0], err)
	}
	sig := signAll(loadSecs(keys), a.mtext)
	if *add {
		sig = append(append([]byte(nil), a.sigText...), sig...)
	}
	es := append([]entry(nil), a.entries...)
	es[2] = entry{name: fSig, mode: 0644, data: sig}
	nb, err := writeArchive(es)
	if err != nil {
		fatal("%v", err)
	}
	dst := *out
	if dst == "" {
		dst = pos[0]
	}
	if err := os.WriteFile(dst, nb, 0644); err != nil {
		fatal("%v", err)
	}
	fmt.Printf("%s: подписан (%d блок(ов))\n", dst, strings.Count(string(sig), commentPrefix))
}

func cmdVerify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	var pubs, revoked multi
	fs.Var(&pubs, "pub", "доверенный открытый ключ; можно повторять")
	fs.Var(&revoked, "revoked", "файл с номерами отозванных ключей")
	keysDir := fs.String("keys-dir", "", "каталог ключей ролей (<dir>/<роль>/*.pub, как /usr/share/steer/keys)")
	role := fs.String("role", "", "роль ключей из --keys-dir: index, protocols, plugins")
	unsigned := fs.Bool("unsigned", false, "принять архив без подписи (только для разработки)")
	base := fs.String("base", "", "для index.json: каталог, где лежат архивы — сверить sha256 и размер")
	now := fs.String("now", "", "для index.json: время проверки expires (RFC 3339), по умолчанию сейчас")
	pos := parseInterleaved(fs, args)
	if len(pos) != 1 {
		fatal("verify: нужен один файл: .s2p или index.json")
	}
	tr := newTrust()
	addTrust(tr, pubs, revoked, *keysDir, *role)
	if strings.HasSuffix(pos[0], ".s2p") {
		verifyArchiveCmd(pos[0], tr, *unsigned)
		return
	}
	verifyIndexCmd(pos[0], tr, *base, *now)
}

func verifyArchiveCmd(path string, tr *trust, unsigned bool) {
	a, err := openArchive(readFile(path))
	if err != nil {
		fatal("%s: %v", path, err)
	}
	p, err := checkHeader(a)
	if err != nil {
		fatal("%s: %v", path, err)
	}
	signedBy := "без подписи"
	if len(strings.TrimSpace(string(a.sigText))) == 0 {
		if !unsigned {
			fatal("%s: MANIFEST %s не подписан (для разработки: --unsigned)", path, p.ID)
		}
	} else {
		if len(tr.keys) == 0 && !unsigned {
			fatal("%s: нет доверенных ключей (--pub или --keys-dir)", path)
		}
		if err := tr.verify("MANIFEST "+p.ID, a.mtext, a.sigText); err != nil {
			if !unsigned {
				fatal("%s: %v", path, err)
			}
			fmt.Fprintf(os.Stderr, "%s: предупреждение: %v\n", path, err)
		} else {
			bs, _ := parseSigs(a.sigText)
			signedBy = "подпись ключа " + bs[0].num.String()
			for _, b := range bs {
				if _, ok := tr.keys[b.num]; ok && !tr.revoked[b.num] {
					signedBy = "подпись ключа " + b.num.String()
					break
				}
			}
		}
	}
	o := lintOpts{}
	if a.manifest.arch != "all" {
		o.arch = a.manifest.arch
	}
	l := lintTree(treeFromArchive(a), o)
	printDiags(l, path)
	if l.errors() > 0 {
		fatal("%s: манифест и файлы не прошли проверку (ошибок %d)", path, l.errors())
	}
	fmt.Printf("%s: ok — %s %s %s, %s, %d файлов, %d байт после распаковки\n", path, p.ID, p.Version, a.manifest.arch, signedBy, len(a.manifest.lines), a.manifest.installed())
}

func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		var n int
		if _, err := fmt.Sscanf(strings.TrimSuffix(s, "d"), "%d", &n); err != nil {
			return 0, fmt.Errorf("срок %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func nowUTC(flagVal string) time.Time {
	if flagVal != "" {
		t, err := time.Parse(time.RFC3339, flagVal)
		if err != nil {
			fatal("--now: %v", err)
		}
		return t.UTC()
	}
	if e := os.Getenv("SOURCE_DATE_EPOCH"); e != "" {
		var n int64
		if _, err := fmt.Sscanf(e, "%d", &n); err == nil {
			return time.Unix(n, 0).UTC()
		}
	}
	return time.Now().UTC().Truncate(time.Second)
}

var _ = filepath.Join
