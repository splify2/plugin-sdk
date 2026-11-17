package main

import (
	"encoding/json"
	"strings"
	"testing"
)

const settingsObject = `{"type":"object","properties":{"enabled":{"type":"boolean","default":true}},"additionalProperties":false}`

func settingsTree(t *testing.T, edit func(map[string]any), schema string) *tree {
	t.Helper()
	m := map[string]any{
		"schema": 1, "id": "settings-example", "version": "0.1.0",
		"name": "Настройки", "compat": map[string]any{"steer-api": "^1.5"},
		"arch": []string{"x86_64"}, "settings": map[string]any{"schema": "settings.schema.json"},
		"steer": map[string]any{"settings": "restart", "services": []any{map[string]any{"name": "settings-example", "prog": "bin/service"}}},
	}
	if edit != nil {
		edit(m)
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return &tree{
		files:     map[string]fileSpec{"plugin.json": {0644, b}, "settings.schema.json": {0644, []byte(schema)}},
		generated: map[string]bool{"bin/service": true},
	}
}

func settingsDiags(l *linter, errorsOnly bool) string {
	var text []string
	for _, d := range l.ds {
		if !errorsOnly || d.err {
			text = append(text, d.msg)
		}
	}
	return strings.Join(text, "\n")
}

func legacySettings(m map[string]any, path string) {
	m["splify2"] = map[string]any{"settings": map[string]any{"schema": path}}
	m["compat"].(map[string]any)["splify2-api"] = "^1.0"
}

func TestNeutralSettingsLint(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		tr := settingsTree(t, func(m map[string]any) {
			if legacy {
				legacySettings(m, "settings.schema.json")
			}
		}, settingsObject)
		l := lintTree(tr, lintOpts{official: true})
		if l.errors() != 0 || strings.Contains(settingsDiags(l, false), "без схемы настроек") {
			t.Fatalf("legacy=%v: %s", legacy, settingsDiags(l, false))
		}
		p, _ := parsePlugin(tr.files["plugin.json"].data)
		if p.settingsSchema() != "settings.schema.json" {
			t.Fatal("neutral schema was lost")
		}
	}
}

func TestNeutralSettingsRejectsInvalidContracts(t *testing.T) {
	cases := []struct {
		name   string
		edit   func(map[string]any)
		schema string
		want   string
	}{
		{"different aliases", func(m map[string]any) { legacySettings(m, "other.json") }, settingsObject, "должны ссылаться на один файл"},
		{"missing file", func(m map[string]any) { m["settings"] = map[string]any{"schema": "absent.json"} }, settingsObject, "файла absent.json нет"},
		{"escaping path", func(m map[string]any) { m["settings"] = map[string]any{"schema": "../settings.json"} }, settingsObject, "plugin.json"},
		{"missing schema", func(m map[string]any) { m["settings"] = map[string]any{} }, settingsObject, "schema"},
		{"invalid JSON", nil, `{`, "settings.schema settings.schema.json"},
		{"array root", nil, `{"type":"array"}`, "type: object"},
		{"non-schema root", nil, `[]`, "type: object"},
		{"unsupported keyword", nil, `{"type":"object","oneOf":[]}`, "oneOf"},
		{"unsupported regex", nil, `{"type":"object","properties":{"name":{"type":"string","pattern":"(?=a)"}}}`, "не POSIX ERE"},
		{"old runtime", func(m map[string]any) { m["compat"].(map[string]any)["steer-api"] = "^1.4" }, settingsObject, "только 1.5"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := lintTree(settingsTree(t, c.edit, c.schema), lintOpts{})
			if l.errors() == 0 || !strings.Contains(settingsDiags(l, true), c.want) {
				t.Fatalf("wanted refusal containing %q, got: %s", c.want, settingsDiags(l, false))
			}
		})
	}
}

func TestLegacySettingsDialectUnchanged(t *testing.T) {
	tr := settingsTree(t, func(m map[string]any) {
		delete(m, "settings")
		legacySettings(m, "settings.schema.json")
	}, `{"type":"object","oneOf":[]}`)
	l := lintTree(tr, lintOpts{})
	if l.errors() != 0 || strings.Contains(settingsDiags(l, false), "без схемы настроек") {
		t.Fatal(settingsDiags(l, false))
	}
	p, _ := parsePlugin(tr.files["plugin.json"].data)
	if p.settingsSchema() != "settings.schema.json" {
		t.Fatal("legacy schema was lost")
	}
}
