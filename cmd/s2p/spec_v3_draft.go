package main

// Draft-only validation helpers. Deliberately not wired into any s2p command,
// stable manifest parser, compatibility declaration or runtime apply path.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// Limits belong to this draft entry point, not the stable schema validator.
// Check before schema traversal, whose uniqueItems checks can be quadratic.
const draftMaxBytes = 1 << 20
const draftMaxDepth = 64
const draftMaxCollection = 1024

func checkDraftJSON(b []byte) error {
	if len(b) > draftMaxBytes {
		return fmt.Errorf("draft document exceeds %d bytes", draftMaxBytes)
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > draftMaxDepth {
			return fmt.Errorf("draft nesting exceeds %d", draftMaxDepth)
		}
		tok, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		keys := map[string]bool{}
		count := 0
		for d.More() {
			count++
			if count > draftMaxCollection {
				return fmt.Errorf("draft collection exceeds %d entries", draftMaxCollection)
			}
			if delim == '{' {
				key, err := d.Token()
				if err != nil {
					return err
				}
				s, ok := key.(string)
				if !ok {
					return fmt.Errorf("invalid object key")
				}
				if keys[s] {
					return fmt.Errorf("duplicate JSON key %q", s)
				}
				keys[s] = true
			}
			if err := walk(depth + 1); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("draft must contain exactly one JSON value")
	}
	return nil
}

func draftDocument(schema string, b []byte) (map[string]any, []string) {
	if err := checkDraftJSON(b); err != nil {
		return nil, []string{err.Error()}
	}
	v, err := decodeJSON(b)
	if err != nil {
		return nil, []string{err.Error()}
	}
	if errs := mustValidator("draft/" + schema).check(v); len(errs) > 0 {
		return nil, errs
	}
	return v.(map[string]any), nil
}

func validateSpecV3Draft(b []byte) []string {
	d, errs := draftDocument("spec-v3.schema.json", b)
	if d == nil {
		return errs
	}
	remotes := d["plugin_remotes"].(map[string]any)
	plugins := d["plugins"].(map[string]any)
	ins := d["inbounds"].(map[string]any)
	outs := d["outbounds"].(map[string]any)
	add := func(f string, a ...any) { errs = append(errs, fmt.Sprintf(f, a...)) }
	for name, r := range remotes {
		u, e := url.Parse(r.(map[string]any)["url"].(string))
		if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
			add("plugin_remotes.%s: invalid HTTPS index URL", name)
		} else if strings.HasSuffix(u.Host, ":") {
			add("plugin_remotes.%s: empty HTTPS port", name)
		} else if port := u.Port(); port != "" {
			n, err := strconv.Atoi(port)
			if err != nil || n < 1 || n > 65535 {
				add("plugin_remotes.%s: HTTPS port outside 1..65535", name)
			}
		}
	}
	for alias, p := range plugins {
		m := p.(map[string]any)
		if _, ok := remotes[m["remote"].(string)]; !ok {
			add("plugins.%s: unknown remote", alias)
		}
		if _, e := parseRange(m["version"].(string)); e != nil {
			add("plugins.%s: invalid version constraint", alias)
		}
	}
	for section, items := range map[string]map[string]any{"inbounds": ins, "outbounds": outs} {
		for name, item := range items {
			if p, ok := item.(map[string]any)["plugin"].(string); ok {
				alias := strings.Split(p, ":")[1]
				if _, ok := plugins[alias]; !ok {
					add("%s.%s: unknown plugin alias %s", section, name, alias)
				}
			}
		}
	}
	routing := d["routing"].(map[string]any)
	checkOut := func(ref, where string) {
		if _, ok := outs[ref]; !ok {
			add("%s: unknown outbound %s", where, ref)
		}
	}
	checkOut(routing["default"].(string), "routing.default")
	names := map[string]bool{}
	for _, r := range routing["rules"].([]any) {
		m := r.(map[string]any)
		name := m["name"].(string)
		if names[name] {
			add("routing.rules: duplicate name %s", name)
		}
		names[name] = true
		checkOut(m["outbound"].(string), "routing.rules."+name)
		for _, ref := range m["match"].(map[string]any)["inbound"].([]any) {
			if _, ok := ins[ref.(string)]; !ok {
				add("routing.rules.%s: unknown inbound %s", name, ref)
			}
		}
	}
	edges := map[string][]string{}
	for name, o := range outs {
		m := o.(map[string]any)
		if m["type"] == "group" {
			for _, ref := range m["members"].([]any) {
				s := ref.(string)
				checkOut(s, "outbounds."+name)
				edges[name] = append(edges[name], s)
			}
		}
	}
	errs = append(errs, draftCycles(edges, "outbound group")...)
	sort.Strings(errs)
	return errs
}

func draftCycles(edges map[string][]string, label string) []string {
	state := map[string]int{}
	var errs []string
	var visit func(string)
	visit = func(n string) {
		if state[n] == 1 {
			errs = append(errs, label+" cycle at "+n)
			return
		}
		if state[n] == 2 {
			return
		}
		state[n] = 1
		for _, next := range edges[n] {
			visit(next)
		}
		state[n] = 2
	}
	keys := make([]string, 0, len(edges))
	for n := range edges {
		keys = append(keys, n)
	}
	sort.Strings(keys)
	for _, n := range keys {
		visit(n)
	}
	return errs
}

func validatePluginLockDraft(specJSON, lockJSON []byte) []string {
	if errs := validateSpecV3Draft(specJSON); len(errs) > 0 {
		return errs
	}
	spec, _ := draftDocument("spec-v3.schema.json", specJSON)
	lock, errs := draftDocument("plugin-lock.schema.json", lockJSON)
	if lock == nil {
		return errs
	}
	plugins := spec["plugins"].(map[string]any)
	remotes := spec["plugin_remotes"].(map[string]any)
	snapshots := lock["remotes"].(map[string]any)
	locked := lock["plugins"].(map[string]any)
	add := func(f string, a ...any) { errs = append(errs, fmt.Sprintf(f, a...)) }
	for name, snapshot := range snapshots {
		want, ok := remotes[name]
		if !ok {
			add("lock.remotes.%s: unknown remote", name)
			continue
		}
		m := snapshot.(map[string]any)
		w := want.(map[string]any)
		if m["url"] != w["url"] || m["trust"] != w["trust"] {
			add("lock.remotes.%s: remote URL or trust identity mismatch", name)
		}
	}
	edges := map[string][]string{}
	identities := map[string]string{}
	metadata := map[string]map[string]any{}
	origins := map[string]string{}
	for alias, p := range plugins {
		entry, ok := locked[alias]
		if !ok {
			add("lock: missing alias %s", alias)
			continue
		}
		m := entry.(map[string]any)
		want := p.(map[string]any)
		if snapshot, ok := snapshots[m["remote"].(string)]; !ok {
			add("lock.%s: missing remote snapshot", alias)
		} else {
			s := snapshot.(map[string]any)
			if m["index_sha256"] != s["index_sha256"] || m["signer"] != s["signer"] {
				add("lock.%s: index digest or signer differs from remote snapshot", alias)
			}
		}
		if m["remote"] != want["remote"] || m["package"] != want["package"] {
			add("lock.%s: package identity mismatch", alias)
		}
		if m["target"] != lock["target"] {
			add("lock.%s: target mismatch", alias)
		}
		v, e := parseVersion(m["version"].(string))
		r, _ := parseRange(want["version"].(string))
		if e != nil || !r.contains(v) {
			add("lock.%s: version outside constraint", alias)
		}
		key := m["package"].(string)
		origin := m["remote"].(string)
		if previous, ok := origins[key]; ok && previous != origin {
			add("lock.%s: ambiguous origin for package %s", alias, key)
		} else {
			origins[key] = origin
		}
		identity := m["version"].(string) + "/" + m["artifact_sha256"].(string)
		if old, ok := identities[key]; ok && old != identity {
			add("lock.%s: conflicting package versions or artifacts", alias)
		}
		identities[key] = identity
		if old, ok := metadata[key]; ok {
			if !reflect.DeepEqual(draftProviderSet(old["providers"]), draftProviderSet(m["providers"])) || !reflect.DeepEqual(old["api"], m["api"]) {
				add("lock.%s: inconsistent provider or API metadata for package", alias)
			}
		} else {
			metadata[key] = m
		}
		for axis, value := range m["api"].(map[string]any) {
			if _, e := parseRange(value.(string)); e != nil {
				add("lock.%s: invalid API range %s", alias, axis)
			}
		}
		for _, dep := range m["dependencies"].([]any) {
			s := dep.(string)
			if _, ok := locked[s]; !ok {
				add("lock.%s: unknown dependency %s", alias, s)
			}
			edges[alias] = append(edges[alias], s)
		}
	}
	// Minimum draft: dependency nodes must also be explicit aliases in the spec.
	for alias := range locked {
		if _, ok := plugins[alias]; !ok {
			add("lock: unexpected alias %s", alias)
		}
	}
	for section, items := range map[string]any{"inbounds": spec["inbounds"], "outbounds": spec["outbounds"]} {
		for name, item := range items.(map[string]any) {
			p, ok := item.(map[string]any)["plugin"].(string)
			if !ok {
				continue
			}
			parts := strings.Split(p, ":")
			if entry, ok := locked[parts[1]]; ok {
				found := false
				for _, provider := range entry.(map[string]any)["providers"].([]any) {
					if provider == parts[0] {
						found = true
					}
				}
				if !found {
					add("lock: %s.%s provider not declared", section, name)
				}
			}
		}
	}
	errs = append(errs, draftCycles(edges, "dependency")...)
	sort.Strings(errs)
	return errs
}

func draftProviderSet(v any) []string {
	var result []string
	for _, p := range v.([]any) {
		result = append(result, p.(string))
	}
	sort.Strings(result)
	return result
}
