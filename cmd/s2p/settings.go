package main

// The neutral schema is opt-in. Do not impose a new schema dialect on legacy
// splify2 plugins just because this SDK also supports another host.
func (l *linter) settings(p *pluginDoc, t *tree) {
	if p.Settings == nil {
		return
	}
	path := p.Settings.Schema
	if p.Splify2 != nil && p.Splify2.Settings != nil && p.Splify2.Settings.Schema != path {
		l.errf("settings.schema и splify2.settings.schema должны ссылаться на один файл: %q и %q", path, p.Splify2.Settings.Schema)
	}
	f, ok := t.files[path]
	if !ok {
		// Generated schemas are checked again when the built archive is linted.
		// Missing files are reported by files().
		return
	}
	doc, err := decodeJSON(f.data)
	if err != nil {
		l.errf("settings.schema %s: %v", path, err)
		return
	}
	obj, ok := doc.(map[string]any)
	if !ok || obj["type"] != "object" {
		l.errf("settings.schema %s: корень схемы должен иметь type: object", path)
		return
	}
	for _, e := range mustValidator("kind-keys.schema.json").check(doc) {
		l.errf("settings.schema %s%s (разрешённое подмножество — schema/kind-keys.schema.json)", path, e)
	}
	walkPatterns(doc, func(pat string) {
		if _, err := regexpPosix(pat); err != nil {
			l.errf("settings.schema %s: pattern %q не POSIX ERE: %v", path, pat, err)
		}
	})
}
