package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Архив .s2p по docs/s2p.md: ustar в одном потоке gzip; первые три записи plugin.json, MANIFEST,
// MANIFEST.sig; режимы 0644/0755; время, владелец, группа — нули. Одинаковый вход даёт одинаковые байты.

const (
	fPlugin   = "plugin.json"
	fManifest = "MANIFEST"
	fSig      = "MANIFEST.sig"
)

type entry struct {
	name string // без «/» в конце
	mode int64
	dir  bool
	data []byte
}

type fileSpec struct {
	mode int64 // 0644 или 0755
	data []byte
}

func sha256hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// manifestText строит MANIFEST: заголовок и по строке на файл (кроме MANIFEST и MANIFEST.sig),
// сортировка по пути побайтово.
func manifestText(id, ver, arch string, files map[string]fileSpec) []byte {
	var names []string
	for n := range files {
		if n != fManifest && n != fSig {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	var b bytes.Buffer
	fmt.Fprintf(&b, "s2p 1 %s %s %s\n", id, ver, arch)
	for _, n := range names {
		f := files[n]
		fmt.Fprintf(&b, "%s %d %04o %s\n", sha256hex(f.data), len(f.data), f.mode, n)
	}
	return b.Bytes()
}

type manifestLine struct {
	sha  string
	size int
	mode int64
	path string
}

type manifest struct {
	id, ver, arch string
	lines         []manifestLine
}

func parseManifest(b []byte) (*manifest, error) {
	if len(b) == 0 || b[len(b)-1] != '\n' {
		return nil, fmt.Errorf("MANIFEST: нет перевода строки в конце")
	}
	ls := strings.Split(string(b[:len(b)-1]), "\n")
	h := strings.Split(ls[0], " ")
	if len(h) != 5 || h[0] != "s2p" {
		return nil, fmt.Errorf("MANIFEST: заголовок должен быть «s2p 1 <id> <версия> <арх>»")
	}
	if h[1] != "1" {
		return nil, fmt.Errorf("MANIFEST: формат %s, этот s2p знает 1", h[1])
	}
	m := &manifest{id: h[2], ver: h[3], arch: h[4]}
	prev := ""
	for i, l := range ls[1:] {
		p := strings.SplitN(l, " ", 4)
		if len(p) != 4 {
			return nil, fmt.Errorf("MANIFEST, строка %d: нужно «sha256 размер режим путь»", i+2)
		}
		if len(p[0]) != 64 || strings.Trim(p[0], "0123456789abcdef") != "" {
			return nil, fmt.Errorf("MANIFEST, строка %d: sha256 не 64 знака hex", i+2)
		}
		size, err := strconv.Atoi(p[1])
		if err != nil || size < 0 {
			return nil, fmt.Errorf("MANIFEST, строка %d: размер %q", i+2, p[1])
		}
		if p[2] != "0644" && p[2] != "0755" {
			return nil, fmt.Errorf("MANIFEST, строка %d: режим %s (разрешены 0644 и 0755)", i+2, p[2])
		}
		mode, _ := strconv.ParseInt(p[2], 8, 32)
		if err := checkPath(p[3]); err != nil {
			return nil, fmt.Errorf("MANIFEST, строка %d: %v", i+2, err)
		}
		if p[3] == fManifest || p[3] == fSig {
			return nil, fmt.Errorf("MANIFEST, строка %d: %s не описывает сам себя", i+2, p[3])
		}
		if i > 0 && p[3] <= prev {
			return nil, fmt.Errorf("MANIFEST, строка %d: пути не по порядку или повторены (%s)", i+2, p[3])
		}
		prev = p[3]
		m.lines = append(m.lines, manifestLine{p[0], size, mode, p[3]})
	}
	return m, nil
}

func (m *manifest) installed() int64 {
	var n int64
	for _, l := range m.lines {
		n += int64(l.size)
	}
	return n
}

// checkPath — правила пути записи (docs/s2p.md, «Архив»).
func checkPath(p string) error {
	switch {
	case p == "":
		return fmt.Errorf("пустой путь")
	case strings.ContainsRune(p, 0):
		return fmt.Errorf("путь %q: нулевой байт", p)
	case strings.HasPrefix(p, "/"):
		return fmt.Errorf("путь %q: абсолютный", p)
	case strings.HasPrefix(p, "./"):
		return fmt.Errorf("путь %q: начинается с ./", p)
	case strings.Contains(p, "//"):
		return fmt.Errorf("путь %q: двойной /", p)
	}
	for _, c := range strings.Split(strings.TrimSuffix(p, "/"), "/") {
		if c == ".." {
			return fmt.Errorf("путь %q: содержит ..", p)
		}
		if c == "." || c == "" {
			return fmt.Errorf("путь %q: лишний компонент", p)
		}
	}
	return nil
}

// orderedEntries — записи архива в порядке записи: три первые, затем всё остальное по пути
// (каталоги перед содержимым).
func orderedEntries(files map[string]fileSpec, sig []byte, manifest []byte) []entry {
	out := []entry{
		{name: fPlugin, mode: files[fPlugin].mode, data: files[fPlugin].data},
		{name: fManifest, mode: 0644, data: manifest},
		{name: fSig, mode: 0644, data: sig},
	}
	rest := map[string]entry{}
	for n, f := range files {
		if n == fPlugin {
			continue
		}
		rest[n] = entry{name: n, mode: f.mode, data: f.data}
		parts := strings.Split(n, "/")
		for i := 1; i < len(parts); i++ {
			d := strings.Join(parts[:i], "/")
			rest[d] = entry{name: d, mode: 0755, dir: true}
		}
	}
	var names []string
	for n := range rest {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		out = append(out, rest[n])
	}
	return out
}

func writeArchive(es []entry) ([]byte, error) {
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	epoch := time.Unix(0, 0)
	for _, e := range es {
		h := &tar.Header{Name: e.name, Mode: e.mode, ModTime: epoch, Format: tar.FormatUSTAR}
		if e.dir {
			h.Typeflag = tar.TypeDir
			h.Name += "/"
		} else {
			h.Typeflag = tar.TypeReg
			h.Size = int64(len(e.data))
		}
		if err := tw.WriteHeader(h); err != nil {
			return nil, fmt.Errorf("запись %s: %v (предел ustar: имя 100 + префикс 155 байт)", e.name, err)
		}
		if !e.dir {
			if _, err := tw.Write(e.data); err != nil {
				return nil, err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	zw, err := gzip.NewWriterLevel(&out, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	// заголовок gzip без имени и времени — иначе байты зависели бы от даты сборки
	zw.Header = gzip.Header{OS: 255}
	if _, err := zw.Write(raw.Bytes()); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// readArchive читает архив строго: один член gzip, только ustar, только обычные файлы и каталоги,
// пути и режимы по правилам, мусор после конца tar — отказ. Порядок записей сохраняется.
func readArchive(data []byte) ([]entry, error) {
	br := bytes.NewReader(data)
	zr, err := gzip.NewReader(br)
	if err != nil {
		return nil, fmt.Errorf("не gzip: %v", err)
	}
	zr.Multistream(false)
	tr := tar.NewReader(zr)
	var es []entry
	seen := map[string]bool{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %v", err)
		}
		name := strings.TrimSuffix(h.Name, "/")
		switch h.Typeflag {
		case tar.TypeReg, tar.TypeDir:
		case tar.TypeSymlink, tar.TypeLink:
			return nil, fmt.Errorf("запись %s: ссылка (разрешены только файлы и каталоги)", h.Name)
		default:
			return nil, fmt.Errorf("запись %s: тип %q (разрешены только файлы и каталоги)", h.Name, string(h.Typeflag))
		}
		if h.Format != tar.FormatUSTAR {
			return nil, fmt.Errorf("запись %s: не ustar (формат %v)", h.Name, h.Format)
		}
		if err := checkPath(h.Name); err != nil {
			return nil, fmt.Errorf("запись %s: %v", h.Name, err)
		}
		if seen[name] {
			return nil, fmt.Errorf("запись %s повторена", name)
		}
		seen[name] = true
		isDir := h.Typeflag == tar.TypeDir
		mode := h.Mode
		if mode&^0o777 != 0 {
			return nil, fmt.Errorf("запись %s: режим %04o (setuid, setgid, sticky запрещены)", h.Name, mode)
		}
		if isDir && mode != 0o755 {
			return nil, fmt.Errorf("запись %s: режим каталога %04o, нужен 0755", h.Name, mode)
		}
		if !isDir && mode != 0o644 && mode != 0o755 {
			return nil, fmt.Errorf("запись %s: режим файла %04o, нужен 0644 или 0755", h.Name, mode)
		}
		e := entry{name: name, mode: mode, dir: isDir}
		if !isDir {
			e.data, err = io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("запись %s: %v", h.Name, err)
			}
		}
		es = append(es, e)
	}
	// всё после конца tar — только нули (добивка блоками), и ровно один член gzip
	rest, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("gzip: %v", err)
	}
	if len(bytes.Trim(rest, "\x00")) != 0 {
		return nil, fmt.Errorf("мусор после конца tar")
	}
	if br.Len() != 0 {
		return nil, fmt.Errorf("мусор после члена gzip (%d байт): член должен быть один", br.Len())
	}
	return es, nil
}

// archive — разобранный и сверенный с MANIFEST архив.
type archive struct {
	entries  []entry
	files    map[string]fileSpec
	manifest *manifest
	mtext    []byte
	sigText  []byte
}

// openArchive читает и сверяет структуру с MANIFEST; подпись не проверяет.
func openArchive(data []byte) (*archive, error) {
	es, err := readArchive(data)
	if err != nil {
		return nil, err
	}
	if len(es) < 3 || es[0].name != fPlugin || es[1].name != fManifest || es[2].name != fSig {
		return nil, fmt.Errorf("первые три записи должны быть plugin.json, MANIFEST, MANIFEST.sig")
	}
	for i := 0; i < 3; i++ {
		if es[i].dir {
			return nil, fmt.Errorf("запись %s: каталог вместо файла", es[i].name)
		}
	}
	a := &archive{entries: es, files: map[string]fileSpec{}, mtext: es[1].data, sigText: es[2].data}
	a.manifest, err = parseManifest(a.mtext)
	if err != nil {
		return nil, err
	}
	for _, e := range es {
		if e.dir || e.name == fManifest || e.name == fSig {
			continue
		}
		a.files[e.name] = fileSpec{mode: e.mode, data: e.data}
	}
	listed := map[string]bool{}
	for _, l := range a.manifest.lines {
		listed[l.path] = true
		f, ok := a.files[l.path]
		if !ok {
			return nil, fmt.Errorf("файл %s есть в MANIFEST, но нет в архиве", l.path)
		}
		if len(f.data) != l.size {
			return nil, fmt.Errorf("файл %s: размер %d, в MANIFEST %d", l.path, len(f.data), l.size)
		}
		if f.mode != l.mode {
			return nil, fmt.Errorf("файл %s: режим %04o, в MANIFEST %04o", l.path, f.mode, l.mode)
		}
		if sha256hex(f.data) != l.sha {
			return nil, fmt.Errorf("файл %s: sha256 не совпадает с MANIFEST", l.path)
		}
	}
	for n := range a.files {
		if !listed[n] {
			return nil, fmt.Errorf("файл %s есть в архиве, но не описан в MANIFEST", n)
		}
	}
	// каталоги должны следовать из путей: пустой или лишний каталог — подмена
	need := map[string]bool{}
	for n := range a.files {
		parts := strings.Split(n, "/")
		for i := 1; i < len(parts); i++ {
			need[strings.Join(parts[:i], "/")] = true
		}
	}
	for _, e := range es {
		if e.dir && !need[e.name] {
			return nil, fmt.Errorf("каталог %s не нужен ни одному файлу из MANIFEST", e.name)
		}
	}
	return a, nil
}
