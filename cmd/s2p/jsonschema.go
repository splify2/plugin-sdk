package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Проверка по JSON Schema: подмножество, которое нужно схемам SDK (type, enum, const, required,
// properties, additionalProperties, items, min/max, pattern, propertyNames, oneOf/anyOf/allOf, $ref
// внутри документа, format date-time и uri). Без внешних библиотек: s2p собирается одним go build.

type validator struct {
	root  any
	cache map[string]*lookPattern
}

func newValidator(schemaJSON []byte) (*validator, error) {
	var root any
	if err := json.Unmarshal(schemaJSON, &root); err != nil {
		return nil, err
	}
	return &validator{root: root, cache: map[string]*lookPattern{}}, nil
}

func decodeJSON(b []byte) (any, error) {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// lookPattern — регулярное выражение с начальными отрицательными «заглядываниями» (?!…), которых
// нет в RE2, но есть в схемах (relpath, abspath): они вырезаются и проверяются отдельно.
type lookPattern struct {
	re   *regexp.Regexp
	off  int // длина литерального начала до заглядываний (^/(?!…) — 1)
	deny []*regexp.Regexp
}

func compileLook(p string) (*lookPattern, error) {
	lp := &lookPattern{}
	rest := p
	prefix := ""
	if strings.HasPrefix(rest, "^") {
		prefix, rest = "^", rest[1:]
	}
	lit := 0
	for lit < len(rest) && !strings.ContainsRune(`\.+*?()|[]{}^$`, rune(rest[lit])) {
		lit++
	}
	if strings.HasPrefix(rest[lit:], "(?!") {
		lp.off = lit
		prefix += rest[:lit]
		rest = rest[lit:]
	}
	for strings.HasPrefix(rest, "(?!") {
		depth, i := 0, 0
		for ; i < len(rest); i++ {
			if rest[i] == '\\' {
				i++
				continue
			}
			if rest[i] == '(' {
				depth++
			} else if rest[i] == ')' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
		if i >= len(rest) {
			return nil, fmt.Errorf("шаблон %q: скобки", p)
		}
		d, err := regexp.Compile("^(?:" + rest[3:i] + ")")
		if err != nil {
			return nil, err
		}
		lp.deny = append(lp.deny, d)
		rest = rest[i+1:]
	}
	re, err := regexp.Compile(prefix + rest)
	if err != nil {
		return nil, err
	}
	lp.re = re
	return lp, nil
}

func (l *lookPattern) match(s string) bool {
	for _, d := range l.deny {
		if l.off <= len(s) && d.MatchString(s[l.off:]) {
			return false
		}
	}
	return l.re.MatchString(s)
}

func (v *validator) pattern(p string) (*lookPattern, error) {
	if c, ok := v.cache[p]; ok {
		return c, nil
	}
	c, err := compileLook(p)
	if err != nil {
		return nil, err
	}
	v.cache[p] = c
	return c, nil
}

func (v *validator) resolve(ref string) (any, error) {
	if !strings.HasPrefix(ref, "#") {
		return nil, fmt.Errorf("$ref %q: только внутри документа", ref)
	}
	cur := v.root
	for _, seg := range strings.Split(strings.TrimPrefix(ref, "#"), "/") {
		if seg == "" {
			continue
		}
		seg = strings.ReplaceAll(strings.ReplaceAll(seg, "~1", "/"), "~0", "~")
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("$ref %q не найден", ref)
		}
		cur, ok = m[seg]
		if !ok {
			return nil, fmt.Errorf("$ref %q не найден", ref)
		}
	}
	return cur, nil
}

func jtype(x any) string {
	switch t := x.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		if t == math.Trunc(t) {
			return "integer"
		}
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "?"
}

func typeOK(want string, x any) bool {
	got := jtype(x)
	return got == want || (want == "number" && got == "integer")
}

func equalJSON(a, b any) bool { return reflect.DeepEqual(a, b) }

// check возвращает список ошибок «путь: что не так»; пустой — документ годится.
func (v *validator) check(inst any) []string {
	var errs []string
	v.walk(v.root, inst, "", &errs)
	return errs
}

func (v *validator) subOK(sch, inst any, path string) bool {
	var e []string
	v.walk(sch, inst, path, &e)
	return len(e) == 0
}

func (v *validator) walk(sch, inst any, path string, errs *[]string) {
	add := func(f string, a ...any) {
		p := path
		if p == "" {
			p = "/"
		}
		*errs = append(*errs, p+": "+fmt.Sprintf(f, a...))
	}
	switch s := sch.(type) {
	case bool:
		if !s {
			add("не допускается")
		}
		return
	case map[string]any:
		v.walkObj(s, inst, path, errs, add)
	}
}

func (v *validator) walkObj(s map[string]any, inst any, path string, errs *[]string, add func(string, ...any)) {
	if ref, ok := s["$ref"].(string); ok {
		t, err := v.resolve(ref)
		if err != nil {
			add("%v", err)
		} else {
			v.walk(t, inst, path, errs)
		}
	}
	if t, ok := s["type"]; ok {
		var want []string
		switch tt := t.(type) {
		case string:
			want = []string{tt}
		case []any:
			for _, x := range tt {
				want = append(want, x.(string))
			}
		}
		okAny := false
		for _, w := range want {
			if typeOK(w, inst) {
				okAny = true
			}
		}
		if !okAny {
			add("ожидался %s, а не %s", strings.Join(want, "|"), jtype(inst))
			return
		}
	}
	if e, ok := s["enum"].([]any); ok {
		found := false
		for _, x := range e {
			if equalJSON(x, inst) {
				found = true
			}
		}
		if !found {
			var names []string
			for _, x := range e {
				b, _ := json.Marshal(x)
				names = append(names, string(b))
			}
			b, _ := json.Marshal(inst)
			add("значение %s не из списка %s", b, strings.Join(names, ", "))
		}
	}
	if c, ok := s["const"]; ok && !equalJSON(c, inst) {
		b, _ := json.Marshal(c)
		add("должно быть %s", b)
	}
	if alts, ok := s["allOf"].([]any); ok {
		for _, a := range alts {
			v.walk(a, inst, path, errs)
		}
	}
	if alts, ok := s["anyOf"].([]any); ok {
		okAny := false
		for _, a := range alts {
			if v.subOK(a, inst, path) {
				okAny = true
			}
		}
		if !okAny {
			add("не подходит ни один из вариантов anyOf")
		}
	}
	if alts, ok := s["oneOf"].([]any); ok {
		n := 0
		for _, a := range alts {
			if v.subOK(a, inst, path) {
				n++
			}
		}
		if n != 1 {
			add("должен подходить ровно один вариант oneOf, подходит %d", n)
		}
	}
	switch x := inst.(type) {
	case string:
		n := utf8.RuneCountInString(x)
		if m, ok := s["minLength"].(float64); ok && float64(n) < m {
			add("короче %d знаков", int(m))
		}
		if m, ok := s["maxLength"].(float64); ok && float64(n) > m {
			add("длиннее %d знаков", int(m))
		}
		if p, ok := s["pattern"].(string); ok {
			lp, err := v.pattern(p)
			if err != nil {
				add("шаблон %q: %v", p, err)
			} else if !lp.match(x) {
				add("%q не подходит под шаблон %s", x, p)
			}
		}
		if f, ok := s["format"].(string); ok {
			switch f {
			case "date-time":
				if _, err := time.Parse(time.RFC3339, x); err != nil {
					add("%q не дата-время RFC 3339", x)
				}
			case "uri":
				if u, err := url.Parse(x); err != nil || u.Scheme == "" {
					add("%q не адрес", x)
				}
			}
		}
	case float64:
		if m, ok := s["minimum"].(float64); ok && x < m {
			add("%v меньше %v", x, m)
		}
		if m, ok := s["maximum"].(float64); ok && x > m {
			add("%v больше %v", x, m)
		}
	case []any:
		if m, ok := s["minItems"].(float64); ok && float64(len(x)) < m {
			add("элементов меньше %d", int(m))
		}
		if m, ok := s["maxItems"].(float64); ok && float64(len(x)) > m {
			add("элементов больше %d", int(m))
		}
		if u, ok := s["uniqueItems"].(bool); ok && u {
			for i := range x {
				for j := i + 1; j < len(x); j++ {
					if equalJSON(x[i], x[j]) {
						add("элементы %d и %d одинаковы", i, j)
					}
				}
			}
		}
		if it, ok := s["items"]; ok {
			for i, e := range x {
				v.walk(it, e, fmt.Sprintf("%s/%d", path, i), errs)
			}
		}
	case map[string]any:
		if m, ok := s["minProperties"].(float64); ok && float64(len(x)) < m {
			add("ключей меньше %d", int(m))
		}
		if req, ok := s["required"].([]any); ok {
			for _, r := range req {
				if _, has := x[r.(string)]; !has {
					add("нет обязательного поля %q", r)
				}
			}
		}
		props, _ := s["properties"].(map[string]any)
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if ps, ok := props[k]; ok {
				v.walk(ps, x[k], path+"/"+k, errs)
			} else if ap, ok := s["additionalProperties"]; ok {
				if b, isB := ap.(bool); isB {
					if !b {
						add("неизвестное поле %q", k)
					}
				} else {
					v.walk(ap, x[k], path+"/"+k, errs)
				}
			}
			if pn, ok := s["propertyNames"]; ok {
				v.walk(pn, k, path+"/"+k+"(имя)", errs)
			}
		}
	}
}
