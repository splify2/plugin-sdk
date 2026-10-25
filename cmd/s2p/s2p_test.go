package main

import (
	"bytes"
	"crypto/ed25519"
	"strings"
	"testing"
)

func TestSemverRanges(t *testing.T) {
	cases := []struct {
		r, v string
		want bool
	}{
		{"^1.0", "1.0.0", true}, {"^1.0", "1.9.3", true}, {"^1.0", "2.0.0", false},
		{"^1.4", "1.3.0", false}, {"^0.2.3", "0.2.9", true}, {"^0.2.3", "0.3.0", false},
		{"~1.2", "1.2.9", true}, {"~1.2", "1.3.0", false}, {">=2.1.0", "2.1.0", true},
		{">=2.1.0", "2.0.9", false}, {">1.2.3", "1.2.3", false}, {"<1.5", "1.4.9", true},
		{"^1.0 || ^3.0", "3.1.0", true}, {"^1.0 || ^3.0", "2.0.0", false},
		{">=1.2 <1.5", "1.4.0", true}, {">=1.2 <1.5", "1.5.0", false}, {"*", "9.9.9", true},
		{"1.2.x", "1.2.7", true}, {"=1.2.3", "1.2.4", false}, {">= 1.2", "1.3.0", true},
	}
	for _, c := range cases {
		r, err := parseRange(c.r)
		if err != nil {
			t.Fatalf("%s: %v", c.r, err)
		}
		v, _ := parseVersion(c.v)
		if got := r.contains(v); got != c.want {
			t.Errorf("%s содержит %s: %v, ждали %v", c.r, c.v, got, c.want)
		}
	}
}

func TestPathRules(t *testing.T) {
	bad := []string{"../x", "a/../b", "/abs", "./a", "a//b", "a/./b", "", "a\x00b"}
	for _, p := range bad {
		if checkPath(p) == nil {
			t.Errorf("путь %q должен быть отвергнут", p)
		}
	}
	for _, p := range []string{"a", "bin/x", "i18n/host/de.json", "a b/c"} {
		if err := checkPath(p); err != nil {
			t.Errorf("путь %q: %v", p, err)
		}
	}
}

func TestLookaheadPatterns(t *testing.T) {
	v := &validator{cache: map[string]*lookPattern{}}
	rel, _ := v.pattern(`^(?!/)(?!.*(^|/)\.\.(/|$))[^\x00]+$`)
	for p, want := range map[string]bool{"bin/x": true, "/bin": false, "a/../b": false, "..": false, "a..b": true, "a/..": false} {
		if rel.match(p) != want {
			t.Errorf("relpath %q: ждали %v", p, want)
		}
	}
	abs, _ := v.pattern(`^/(?!.*(^|/)\.\.(/|$))[^\x00]*$`)
	for p, want := range map[string]bool{"/etc/x": true, "etc": false, "/a/../b": false, "/": true} {
		if abs.match(p) != want {
			t.Errorf("abspath %q: ждали %v", p, want)
		}
	}
}

func TestSignRoundtrip(t *testing.T) {
	k, err := genKey()
	if err != nil {
		t.Fatal(err)
	}
	k2, err := parseSec(formatSec(k, "x"))
	if err != nil || k2.num != k.num || !bytes.Equal(k2.key, k.key) {
		t.Fatalf("закрытый ключ не прошёл круг: %v", err)
	}
	pub, err := parsePub(formatPub(k.pub(), "x"))
	if err != nil || pub.num != k.num {
		t.Fatalf("открытый ключ: %v", err)
	}
	msg := []byte("MANIFEST")
	sig := formatSigs([]sigBlock{k.sign(msg)})
	tr := newTrust()
	tr.keys[pub.num] = pub
	if err := tr.verify("m", msg, sig); err != nil {
		t.Fatal(err)
	}
	if err := tr.verify("m", []byte("other"), sig); err == nil || !strings.Contains(err.Error(), "не сходится") {
		t.Fatalf("чужое сообщение: %v", err)
	}
	tr.revoked[pub.num] = true
	if err := tr.verify("m", msg, sig); err == nil || !strings.Contains(err.Error(), "отозван") {
		t.Fatalf("отозванный ключ: %v", err)
	}
	if len(ed25519.PublicKey(pub.key)) != 32 {
		t.Fatal("длина ключа")
	}
}

func TestArchiveDeterministicAndStrict(t *testing.T) {
	files := map[string]fileSpec{
		"plugin.json":   {0644, []byte(`{"schema":1}`)},
		"bin/x":         {0755, []byte("\x7fELF")},
		"i18n/ru.json":  {0644, []byte(`{}`)},
		"kinds/a/b.txt": {0644, []byte("b")},
	}
	m := manifestText("p", "1.0.0", "all", files)
	a1, err := writeArchive(orderedEntries(files, nil, m))
	if err != nil {
		t.Fatal(err)
	}
	a2, _ := writeArchive(orderedEntries(files, nil, m))
	if !bytes.Equal(a1, a2) {
		t.Fatal("одинаковый вход дал разные байты")
	}
	ar, err := openArchive(a1)
	if err != nil {
		t.Fatal(err)
	}
	if ar.manifest.installed() != int64(12+4+2+1) {
		t.Fatalf("installed = %d", ar.manifest.installed())
	}
	if _, err := openArchive(append(append([]byte(nil), a1...), 'x')); err == nil {
		t.Fatal("мусор после члена gzip должен отвергаться")
	}
	if _, err := openArchive(append(append([]byte(nil), a1...), a1...)); err == nil {
		t.Fatal("два члена gzip должны отвергаться")
	}
}
