package main

import (
	"fmt"
	"strconv"
	"strings"
)

// version — major.minor.patch и признак предвыпуска (для сортировки; в диапазонах не участвует).
type version struct {
	n   [3]int
	pre string
}

func parseVersion(s string) (version, error) {
	var v version
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexByte(s, '-'); i >= 0 {
		v.pre = s[i+1:]
		s = s[:i]
	}
	p := strings.Split(s, ".")
	if len(p) != 3 {
		return v, fmt.Errorf("версия %q: нужно major.minor.patch", s)
	}
	for i := range p {
		x, err := strconv.Atoi(p[i])
		if err != nil || x < 0 || (len(p[i]) > 1 && p[i][0] == '0') {
			return v, fmt.Errorf("версия %q: %q не число semver", s, p[i])
		}
		v.n[i] = x
	}
	return v, nil
}

func cmpVersion(a, b version) int {
	for i := 0; i < 3; i++ {
		if a.n[i] != b.n[i] {
			if a.n[i] < b.n[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case a.pre == b.pre:
		return 0
	case a.pre == "":
		return 1
	case b.pre == "":
		return -1
	case a.pre < b.pre:
		return -1
	}
	return 1
}

// partial — версия с необязательными minor и patch ("1", "1.2", "1.2.x").
type partial struct {
	n    [3]int
	have int // сколько чисел задано, 0 — «любая»
}

func parsePartial(s string) (partial, error) {
	var p partial
	if s == "" || s == "*" || s == "x" || s == "X" {
		return p, nil
	}
	if i := strings.IndexAny(s, "+-"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) > 3 {
		return p, fmt.Errorf("версия %q", s)
	}
	for i, x := range parts {
		if x == "x" || x == "X" || x == "*" {
			break
		}
		n, err := strconv.Atoi(x)
		if err != nil || n < 0 {
			return p, fmt.Errorf("версия %q", s)
		}
		p.n[i] = n
		p.have = i + 1
	}
	return p, nil
}

type bound struct {
	op string // ">=" или "<"
	v  [3]int
}

// desugar разворачивает один оператор npm в границы >= и <.
func desugar(op string, p partial) ([]bound, error) {
	lo := p.n
	var hi [3]int
	switch op {
	case "^":
		switch {
		case p.have == 0:
			return nil, nil
		case p.n[0] > 0 || p.have == 1:
			hi = [3]int{p.n[0] + 1, 0, 0}
		case p.n[1] > 0 || p.have == 2:
			hi = [3]int{0, p.n[1] + 1, 0}
		default:
			hi = [3]int{0, 0, p.n[2] + 1}
		}
	case "~":
		switch {
		case p.have == 0:
			return nil, nil
		case p.have == 1:
			hi = [3]int{p.n[0] + 1, 0, 0}
		default:
			hi = [3]int{p.n[0], p.n[1] + 1, 0}
		}
	case "", "=":
		switch p.have {
		case 0:
			return nil, nil
		case 1:
			hi = [3]int{p.n[0] + 1, 0, 0}
		case 2:
			hi = [3]int{p.n[0], p.n[1] + 1, 0}
		default:
			hi = [3]int{p.n[0], p.n[1], p.n[2] + 1}
		}
	case ">=":
		return []bound{{">=", lo}}, nil
	case ">":
		switch p.have {
		case 0:
			return []bound{{"<", [3]int{0, 0, 0}}}, nil
		case 1:
			return []bound{{">=", [3]int{p.n[0] + 1, 0, 0}}}, nil
		case 2:
			return []bound{{">=", [3]int{p.n[0], p.n[1] + 1, 0}}}, nil
		}
		return []bound{{">=", [3]int{p.n[0], p.n[1], p.n[2] + 1}}}, nil
	case "<":
		return []bound{{"<", lo}}, nil
	case "<=":
		switch p.have {
		case 0:
			return nil, nil
		case 1:
			return []bound{{"<", [3]int{p.n[0] + 1, 0, 0}}}, nil
		case 2:
			return []bound{{"<", [3]int{p.n[0], p.n[1] + 1, 0}}}, nil
		}
		return []bound{{"<", [3]int{p.n[0], p.n[1], p.n[2] + 1}}}, nil
	default:
		return nil, fmt.Errorf("оператор %q", op)
	}
	return []bound{{">=", lo}, {"<", hi}}, nil
}

type semRange [][]bound // ИЛИ из И

// parseRange разбирает диапазон в синтаксисе npm: = ^ ~ >= > <= <, пробел — «и», || — «или».
func parseRange(s string) (semRange, error) {
	var out semRange
	for _, alt := range strings.Split(s, "||") {
		toks := strings.Fields(alt)
		var cur []bound
		for i := 0; i < len(toks); i++ {
			t := toks[i]
			op := ""
			for _, o := range []string{">=", "<=", ">", "<", "=", "^", "~"} {
				if strings.HasPrefix(t, o) {
					op, t = o, t[len(o):]
					break
				}
			}
			if t == "" && i+1 < len(toks) { // «>= 1.2»
				i++
				t = toks[i]
			}
			p, err := parsePartial(t)
			if err != nil {
				return nil, fmt.Errorf("диапазон %q: %v", s, err)
			}
			bs, err := desugar(op, p)
			if err != nil {
				return nil, fmt.Errorf("диапазон %q: %v", s, err)
			}
			cur = append(cur, bs...)
		}
		out = append(out, cur)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("диапазон пуст")
	}
	return out, nil
}

func (r semRange) contains(v version) bool {
	for _, and := range r {
		ok := true
		for _, b := range and {
			c := cmpVersion(version{n: v.n}, version{n: b.v})
			if (b.op == ">=" && c < 0) || (b.op == "<" && c >= 0) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}
