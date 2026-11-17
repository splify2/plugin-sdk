package main

import (
	"encoding/json"
	"strings"
	"testing"
)

const draftSpecFixture = `{"version":3,"plugin_remotes":{"official":{"url":"https://example.invalid/index.json","trust":"official"}},"plugins":{"main":{"remote":"official","package":"steer-main","version":"^1.0"}},"inbounds":{"local":{"plugin":"socks:main","listen":{"address":"127.0.0.1","port":1080},"options":{}}},"outbounds":{"direct":{"type":"direct"},"vpn":{"plugin":"vless:main","options":{}},"pool":{"type":"group","strategy":"failover","members":["vpn","direct"]}},"routing":{"rules":[{"name":"local-vpn","match":{"inbound":["local"]},"outbound":"pool"}],"default":"direct"}}`

func draftFixture(t *testing.T) map[string]any {
	t.Helper()
	var d map[string]any
	if e := json.Unmarshal([]byte(draftSpecFixture), &d); e != nil {
		t.Fatal(e)
	}
	return d
}
func draftBytes(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func draftMap(d map[string]any, k string) map[string]any { return d[k].(map[string]any) }
func draftLockFixture() map[string]any {
	return map[string]any{"version": 1, "target": "linux-x86_64", "remotes": map[string]any{"official": map[string]any{"url": "https://example.invalid/index.json", "trust": "official", "index_sha256": strings.Repeat("b", 64), "signer": "external-trust-key-id"}}, "plugins": map[string]any{"main": map[string]any{"remote": "official", "package": "steer-main", "version": "1.2.0", "target": "linux-x86_64", "artifact_sha256": strings.Repeat("a", 64), "index_sha256": strings.Repeat("b", 64), "signer": "external-trust-key-id", "providers": []any{"socks", "vless"}, "api": map[string]any{"steer-provider-api": "^1.0"}, "dependencies": []any{}}}}
}

func TestSpecV3Draft(t *testing.T) {
	if errs := validateSpecV3Draft([]byte(draftSpecFixture)); len(errs) != 0 {
		t.Fatal(errs)
	}
	cases := map[string]func(map[string]any){
		"unsupported field": func(d map[string]any) { d["subscriptions"] = map[string]any{} },
		"legacy version":    func(d map[string]any) { d["version"] = 2 },
		"unknown remote":    func(d map[string]any) { draftMap(draftMap(d, "plugins"), "main")["remote"] = "missing" },
		"unknown alias":     func(d map[string]any) { draftMap(draftMap(d, "inbounds"), "local")["plugin"] = "socks:missing" },
		"multiple colons":   func(d map[string]any) { draftMap(draftMap(d, "inbounds"), "local")["plugin"] = "socks:main:extra" },
		"http remote": func(d map[string]any) {
			draftMap(draftMap(d, "plugin_remotes"), "official")["url"] = "http://example.invalid/index.json"
		},
		"url credentials": func(d map[string]any) {
			draftMap(draftMap(d, "plugin_remotes"), "official")["url"] = "https://user@example.invalid/index.json"
		},
		"bad constraint":  func(d map[string]any) { draftMap(draftMap(d, "plugins"), "main")["version"] = "garbage" },
		"unknown default": func(d map[string]any) { draftMap(d, "routing")["default"] = "missing" },
		"unknown inbound": func(d map[string]any) {
			draftMap(d, "routing")["rules"].([]any)[0].(map[string]any)["match"] = map[string]any{"inbound": []any{"missing"}}
		},
		"unknown rule outbound": func(d map[string]any) {
			draftMap(d, "routing")["rules"].([]any)[0].(map[string]any)["outbound"] = "missing"
		},
		"duplicate rules": func(d map[string]any) {
			r := draftMap(d, "routing")
			r["rules"] = append(r["rules"].([]any), r["rules"].([]any)[0])
		},
		"duplicate members": func(d map[string]any) { draftMap(draftMap(d, "outbounds"), "pool")["members"] = []any{"vpn", "vpn"} },
		"unknown member":    func(d map[string]any) { draftMap(draftMap(d, "outbounds"), "pool")["members"] = []any{"missing"} },
		"group self cycle":  func(d map[string]any) { draftMap(draftMap(d, "outbounds"), "pool")["members"] = []any{"pool"} },
		"group indirect cycle": func(d map[string]any) {
			o := draftMap(d, "outbounds")
			draftMap(o, "pool")["members"] = []any{"other"}
			o["other"] = map[string]any{"type": "group", "strategy": "order", "members": []any{"pool"}}
		},
		"plugin type xor": func(d map[string]any) { draftMap(draftMap(d, "outbounds"), "vpn")["type"] = "direct" },
		"bad listen port": func(d map[string]any) { draftMap(draftMap(draftMap(d, "inbounds"), "local"), "listen")["port"] = 0 },
		"unknown match": func(d map[string]any) {
			draftMap(d, "routing")["rules"].([]any)[0].(map[string]any)["match"].(map[string]any)["user"] = "alice"
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			d := draftFixture(t)
			mutate(d)
			if errs := validateSpecV3Draft(draftBytes(t, d)); len(errs) == 0 {
				t.Fatal("invalid draft accepted")
			}
		})
	}
	for _, strategy := range []string{"order", "latency", "manual", "balance", "failover"} {
		d := draftFixture(t)
		draftMap(draftMap(d, "outbounds"), "pool")["strategy"] = strategy
		if e := validateSpecV3Draft(draftBytes(t, d)); len(e) > 0 {
			t.Fatal(strategy, e)
		}
	}
}

func TestPluginLockDraft(t *testing.T) {
	if errs := validatePluginLockDraft([]byte(draftSpecFixture), draftBytes(t, draftLockFixture())); len(errs) > 0 {
		t.Fatal(errs)
	}
	cases := map[string]func(map[string]any){
		"missing alias":      func(d map[string]any) { delete(draftMap(d, "plugins"), "main") },
		"unknown field":      func(d map[string]any) { d["resolver"] = "shell" },
		"wrong target":       func(d map[string]any) { draftMap(draftMap(d, "plugins"), "main")["target"] = "other" },
		"wrong package":      func(d map[string]any) { draftMap(draftMap(d, "plugins"), "main")["package"] = "other" },
		"version constraint": func(d map[string]any) { draftMap(draftMap(d, "plugins"), "main")["version"] = "2.0.0" },
		"version not exact":  func(d map[string]any) { draftMap(draftMap(d, "plugins"), "main")["version"] = "^1.0" },
		"digest":             func(d map[string]any) { draftMap(draftMap(d, "plugins"), "main")["artifact_sha256"] = "abc" },
		"missing signer":     func(d map[string]any) { delete(draftMap(draftMap(d, "plugins"), "main"), "signer") },
		"provider":           func(d map[string]any) { draftMap(draftMap(d, "plugins"), "main")["providers"] = []any{"socks"} },
		"dependency":         func(d map[string]any) { draftMap(draftMap(d, "plugins"), "main")["dependencies"] = []any{"missing"} },
		"dependency cycle":   func(d map[string]any) { draftMap(draftMap(d, "plugins"), "main")["dependencies"] = []any{"main"} },
		"api range": func(d map[string]any) {
			draftMap(draftMap(d, "plugins"), "main")["api"] = map[string]any{"steer-provider-api": "bad"}
		},
		"extra alias": func(d map[string]any) { draftMap(d, "plugins")["extra"] = draftMap(d, "plugins")["main"] },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			d := draftLockFixture()
			mutate(d)
			if e := validatePluginLockDraft([]byte(draftSpecFixture), draftBytes(t, d)); len(e) == 0 {
				t.Fatal("invalid lock accepted")
			}
		})
	}
}

func TestPluginLockDraftConflictingAliases(t *testing.T) {
	spec := draftFixture(t)
	draftMap(spec, "plugins")["second"] = map[string]any{"remote": "official", "package": "steer-main", "version": "^1.0"}
	lock := draftLockFixture()
	original := draftMap(draftMap(lock, "plugins"), "main")
	second := map[string]any{}
	for k, v := range original {
		second[k] = v
	}
	draftMap(lock, "plugins")["second"] = second
	if e := validatePluginLockDraft(draftBytes(t, spec), draftBytes(t, lock)); len(e) > 0 {
		t.Fatal("compatible aliases rejected", e)
	}
	second["version"] = "1.3.0"
	if e := validatePluginLockDraft(draftBytes(t, spec), draftBytes(t, lock)); len(e) == 0 {
		t.Fatal("conflicting versions accepted")
	}
	second["version"] = "1.2.0"
	second["artifact_sha256"] = strings.Repeat("c", 64)
	if e := validatePluginLockDraft(draftBytes(t, spec), draftBytes(t, lock)); len(e) == 0 {
		t.Fatal("conflicting artifacts accepted")
	}
}

func TestDraftMalformedJSON(t *testing.T) {
	if e := validateSpecV3Draft([]byte(`{"version":`)); len(e) == 0 {
		t.Fatal("malformed spec accepted")
	}
	if e := validatePluginLockDraft([]byte(draftSpecFixture), []byte(`null`)); len(e) == 0 {
		t.Fatal("null lock accepted")
	}
}

func TestDraftAdversarialJSON(t *testing.T) {
	for _, b := range []string{
		`{"version":2,"version":3}`, `{"x":{"a":1,"\u0061":2}}`,
		`{"a":[{"x":1,"x":2}]}`, `{} {}`, `{"x":`,
		strings.Repeat("[", draftMaxDepth+2) + strings.Repeat("]", draftMaxDepth+2),
		`[` + strings.Repeat(`0,`, draftMaxCollection) + `0]`,
		`"` + strings.Repeat("x", draftMaxBytes) + `"`,
	} {
		if err := checkDraftJSON([]byte(b)); err == nil {
			t.Fatal("ambiguous or oversized JSON accepted")
		}
	}
	for _, b := range []string{`{"a":1,"b":{"a":2}}`, `[null,true,"x",-1,{}]`} {
		if err := checkDraftJSON([]byte(b)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDraftRemoteSnapshot(t *testing.T) {
	cases := map[string]func(map[string]any){
		"missing snapshots":     func(d map[string]any) { delete(d, "remotes") },
		"missing used snapshot": func(d map[string]any) { delete(draftMap(d, "remotes"), "official") },
		"changed URL": func(d map[string]any) {
			draftMap(draftMap(d, "remotes"), "official")["url"] = "https://elsewhere.invalid/index.json"
		},
		"changed trust": func(d map[string]any) { draftMap(draftMap(d, "remotes"), "official")["trust"] = "other" },
		"changed index": func(d map[string]any) {
			draftMap(draftMap(d, "plugins"), "main")["index_sha256"] = strings.Repeat("c", 64)
		},
		"changed signer":   func(d map[string]any) { draftMap(draftMap(d, "plugins"), "main")["signer"] = "other" },
		"unknown snapshot": func(d map[string]any) { draftMap(d, "remotes")["extra"] = draftMap(d, "remotes")["official"] },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			d := draftLockFixture()
			mutate(d)
			if e := validatePluginLockDraft([]byte(draftSpecFixture), draftBytes(t, d)); len(e) == 0 {
				t.Fatal("snapshot mismatch accepted")
			}
		})
	}
	spec := draftFixture(t)
	draftMap(spec, "plugin_remotes")["unused"] = draftMap(spec, "plugin_remotes")["official"]
	lock := draftLockFixture()
	if e := validatePluginLockDraft(draftBytes(t, spec), draftBytes(t, lock)); len(e) > 0 {
		t.Fatal("unused remote without snapshot rejected", e)
	}
	draftMap(lock, "remotes")["unused"] = draftMap(lock, "remotes")["official"]
	if e := validatePluginLockDraft(draftBytes(t, spec), draftBytes(t, lock)); len(e) > 0 {
		t.Fatal("unused matching snapshot rejected", e)
	}
}

func TestDraftRemotePortAndSemver(t *testing.T) {
	for _, u := range []string{"https://example.invalid:99999/index.json", "https://example.invalid:0/index.json", "https://example.invalid:/index.json"} {
		d := draftFixture(t)
		draftMap(draftMap(d, "plugin_remotes"), "official")["url"] = u
		if e := validateSpecV3Draft(draftBytes(t, d)); len(e) == 0 {
			t.Fatal("bad port accepted", u)
		}
	}
	for _, version := range []string{"1.2.0-a..b", "1.2.0-01", "1.2.0+build..1", "1.2.0-.", "1.2.0+.", "01.2.0"} {
		d := draftLockFixture()
		draftMap(draftMap(d, "plugins"), "main")["version"] = version
		if e := validatePluginLockDraft([]byte(draftSpecFixture), draftBytes(t, d)); len(e) == 0 {
			t.Fatal("bad semver accepted", version)
		}
	}
	for _, version := range []string{"1.2.0-alpha.1", "1.2.0+build.01", "1.2.0-0"} {
		d := draftLockFixture()
		draftMap(draftMap(d, "plugins"), "main")["version"] = version
		if e := validatePluginLockDraft([]byte(draftSpecFixture), draftBytes(t, d)); len(e) > 0 {
			t.Fatal("valid semver rejected", version, e)
		}
	}
}

func TestDraftAliasMetadata(t *testing.T) {
	spec := draftFixture(t)
	draftMap(spec, "plugins")["second"] = draftMap(spec, "plugins")["main"]
	for _, field := range []string{"providers", "api"} {
		lock := draftLockFixture()
		second := map[string]any{}
		for k, v := range draftMap(draftMap(lock, "plugins"), "main") {
			second[k] = v
		}
		draftMap(lock, "plugins")["second"] = second
		second["providers"] = []any{"vless", "socks"}
		if e := validatePluginLockDraft(draftBytes(t, spec), draftBytes(t, lock)); len(e) > 0 {
			t.Fatal("reordered provider set rejected", e)
		}
		if field == "providers" {
			second[field] = []any{"socks", "vless", "invented"}
		} else {
			second[field] = map[string]any{"steer-provider-api": "^2.0"}
		}
		if e := validatePluginLockDraft(draftBytes(t, spec), draftBytes(t, lock)); len(e) == 0 {
			t.Fatal("inconsistent metadata accepted", field)
		}
	}
}

func TestDraftRejectsWrongTypesBeforeSemanticTraversal(t *testing.T) {
	paths := [][]string{{"plugin_remotes"}, {"plugin_remotes", "official"}, {"plugins"}, {"plugins", "main"}, {"inbounds"}, {"inbounds", "local"}, {"outbounds"}, {"outbounds", "pool"}, {"routing"}, {"routing", "rules"}, {"routing", "default"}}
	bad := []any{nil, true, float64(42), []any{}, "wrong"}
	for _, path := range paths {
		for _, value := range bad {
			if strings.Join(path, ".") == "routing.rules" {
				if _, ok := value.([]any); ok {
					continue
				}
			}
			d := draftFixture(t)
			node := d
			for _, key := range path[:len(path)-1] {
				node = draftMap(node, key)
			}
			node[path[len(path)-1]] = value
			if errs := validateSpecV3Draft(draftBytes(t, d)); len(errs) == 0 {
				t.Fatalf("invalid type accepted at %v: %#v", path, value)
			}
		}
	}
	for _, field := range []string{"plugins", "remotes"} {
		for _, value := range bad {
			d := draftLockFixture()
			d[field] = value
			if errs := validatePluginLockDraft([]byte(draftSpecFixture), draftBytes(t, d)); len(errs) == 0 {
				t.Fatalf("invalid lock type accepted: %s %#v", field, value)
			}
		}
	}
}

func TestDraftGlobalPackageOrigin(t *testing.T) {
	for _, sameURL := range []bool{false, true} {
		spec := draftFixture(t)
		remote := map[string]any{"url": "https://other.invalid/index.json", "trust": "official"}
		if sameURL {
			remote["url"] = "https://example.invalid/index.json"
		}
		draftMap(spec, "plugin_remotes")["second"] = remote
		draftMap(spec, "plugins")["other"] = map[string]any{"remote": "second", "package": "steer-main", "version": "^1.0"}
		lock := draftLockFixture()
		snapshot := map[string]any{"url": remote["url"], "trust": "official", "index_sha256": strings.Repeat("b", 64), "signer": "external-trust-key-id"}
		draftMap(lock, "remotes")["second"] = snapshot
		entry := map[string]any{}
		for k, v := range draftMap(draftMap(lock, "plugins"), "main") {
			entry[k] = v
		}
		entry["remote"] = "second"
		draftMap(lock, "plugins")["other"] = entry
		for _, version := range []string{"1.2.0", "1.3.0"} {
			entry["version"] = version
			errs := validatePluginLockDraft(draftBytes(t, spec), draftBytes(t, lock))
			if !strings.Contains(strings.Join(errs, "\n"), "ambiguous origin") {
				t.Fatalf("sameURL=%v version=%s: missing ambiguous origin error: %v", sameURL, version, errs)
			}
		}
		// Different globally unique package IDs may use separate remotes.
		draftMap(draftMap(spec, "plugins"), "other")["package"] = "other-package"
		entry["package"] = "other-package"
		if errs := validatePluginLockDraft(draftBytes(t, spec), draftBytes(t, lock)); len(errs) > 0 {
			t.Fatal("distinct packages rejected", errs)
		}
	}
}
