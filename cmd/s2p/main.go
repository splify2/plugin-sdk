// s2p — инструмент SDK плагинов splify2 и steer: проверка, сборка и подпись архивов .s2p, индекс
// репозитория, сборка C-плагинов под 10 архитектур и заготовки проектов.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const usage = `s2p — SDK плагинов splify2 и steer

  s2p lint <каталог|.s2p>        манифест по схеме, файлы, права, совместимость, машинный код
  s2p build [каталог]            собрать .s2p (по архитектуре на архив): сборка, MANIFEST, подпись
  s2p cc                         скомпилировать C-плагин или помощника через zig, без SDK OpenWrt
  s2p sign <файл.s2p>            подписать (usign/signify Ed25519), несколько ключей — ротация
  s2p verify <файл.s2p|index.json>  проверить структуру и подпись
  s2p index <каталог .s2p>       собрать и подписать индекс репозитория
  s2p keygen -o <имя>            пара ключей в формате usign
  s2p new <шаблон> <каталог>     заготовка проекта (helper, so, ui, theme, lang)
  s2p arches                     архитектуры и цели zig
  s2p version

Подробности: s2p <команда> -h, docs/guide.md.`

var toolVersion = "1.5.0" // версия SDK: s2p выходит вместе с ним (последняя строка tests/compat/matrix.json)

func fatal(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "s2p: "+f+"\n", a...)
	os.Exit(1)
}

// multi — флаг, который можно повторять.
type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(s string) error { *m = append(*m, s); return nil }

// parseInterleaved разбирает флаги и позиционные слова в любом порядке (flag из stdlib
// останавливается на первом слове).
func parseInterleaved(fs *flag.FlagSet, args []string) []string {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			os.Exit(2)
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return pos
		}
		pos = append(pos, rest[0])
		args = rest[1:]
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "lint":
		cmdLint(args)
	case "build":
		cmdBuild(args)
	case "cc":
		cmdCC(args)
	case "sign":
		cmdSign(args)
	case "verify":
		cmdVerify(args)
	case "index":
		cmdIndex(args)
	case "keygen":
		cmdKeygen(args)
	case "new":
		cmdNew(args)
	case "arches":
		for _, a := range arches {
			fmt.Printf("%-26s %-22s %s\n", a.id, a.triple, a.mcpu)
		}
	case "version", "--version":
		fmt.Println("s2p", toolVersion)
	case "help", "-h", "--help":
		fmt.Println(usage)
	default:
		fmt.Fprintf(os.Stderr, "s2p: неизвестная команда %q\n\n%s\n", cmd, usage)
		os.Exit(2)
	}
}
