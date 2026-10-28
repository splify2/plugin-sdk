package main

import (
	"encoding/json"
	"fmt"
	"sort"
)

// plugin — разобранный plugin.json (поля, которые читает SDK). Разбирать после проверки по схеме.
type prog struct {
	Prog     string   `json:"prog"`
	Argv     []string `json:"argv"`
	TimeoutS int      `json:"timeout_s"`
	Cmd      string   `json:"cmd"`
	Name     string   `json:"name"`
	Events   []string `json:"events"`
}

type kindDoc struct {
	Name   string   `json:"name"`
	Impl   string   `json:"impl"`
	Keys   string   `json:"keys"`
	Caps   []string `json:"caps"`
	Helper *prog    `json:"helper"`
	Check  *prog    `json:"check"`
	Expand *prog    `json:"expand"`
	Nft    []string `json:"nft"`
	Paths  *bool    `json:"paths"` // steer-api 1.4: помощник отставляет пути (paths_down в status всегда)
	Res    struct {
		Ports  int `json:"ports"`
		Queues int `json:"queues"`
	} `json:"resources"`
}

// linkDoc — путь в системе, который ведёт менеджер, пока плагин стоит (steer.links, steer-api 1.3).
type linkDoc struct {
	Path   string `json:"path"`
	Target string `json:"target"`
	Mode   string `json:"mode"`
}

type pluginDoc struct {
	Schema      int               `json:"schema"`
	ID          string            `json:"id"`
	Version     string            `json:"version"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	License     string            `json:"license"`
	Variant     string            `json:"variant"`
	Compat      map[string]string `json:"compat"`
	Arch        json.RawMessage   `json:"arch"`
	Requires    struct {
		Plugins map[string]string `json:"plugins"`
		System  []string          `json:"system"`
		Kernel  []string          `json:"kernel"`
	} `json:"requires"`
	Conflicts   []string `json:"conflicts"`
	Permissions []string `json:"permissions"`
	Resources   struct {
		MarkBits int `json:"mark_bits"`
		Queues   int `json:"queues"`
		Ports    int `json:"ports"`
		Tables   int `json:"tables"`
	} `json:"resources"`
	Steer struct {
		Commands []struct {
			Name string   `json:"name"`
			Prog string   `json:"prog"`
			Argv []string `json:"argv"`
			Help string   `json:"help"`
		} `json:"commands"`
		Kinds    []kindDoc `json:"kinds"`
		Services []prog    `json:"services"`
		Hooks    struct {
			PreApply *prog `json:"pre_apply"`
			Event    *prog `json:"event"`
		} `json:"hooks"`
		Nft      []string  `json:"nft"`
		Links    []linkDoc `json:"links"`
		Settings string    `json:"settings"` // steer-api 1.4: "restart" — settings.json в подписи перезапуска
		So       *struct {
			Path string `json:"path"`
		} `json:"so"`
	} `json:"steer"`
	RPC *struct {
		Prog    string `json:"prog"`
		Methods map[string]struct {
			Access string `json:"access"`
		} `json:"methods"`
	} `json:"rpc"`
	Splify2 *struct {
		Priority int `json:"priority"`
		Hooks    map[string]struct {
			Prog string `json:"prog"`
			Mode string `json:"mode"`
		} `json:"hooks"`
		Tasks []struct {
			Name  string `json:"name"`
			Every string `json:"every"`
			Cron  string `json:"cron"`
			Prog  string `json:"prog"`
		} `json:"tasks"`
		Events *struct {
			Prog      string   `json:"prog"`
			Subscribe []string `json:"subscribe"`
		} `json:"events"`
		Methods struct {
			Alias    map[string]string `json:"alias"`
			Override map[string]struct {
				Prog string `json:"prog"`
				Mode string `json:"mode"`
			} `json:"override"`
		} `json:"methods"`
		Settings *struct {
			Schema string `json:"schema"`
		} `json:"settings"`
		Hide []string `json:"hide"`
		Deny []string `json:"deny"`
		// luci.interface (splify2-api 1.1): вид LuCI, которым плагин заменяет главный экран каркаса.
		Luci *struct {
			Interface string `json:"interface"`
		} `json:"luci"`
	} `json:"splify2"`
	UI *struct {
		Entry string   `json:"entry"`
		CSS   []string `json:"css"`
	} `json:"ui"`
	I18n *struct {
		Dir       string   `json:"dir"`
		Languages []string `json:"languages"`
		Host      map[string]struct {
			File string `json:"file"`
			Name string `json:"name"`
			Host string `json:"host"`
		} `json:"host"`
	} `json:"i18n"`
	Themes []struct {
		ID   string `json:"id"`
		File string `json:"file"`
	} `json:"themes"`
	Libs []struct {
		Soname string `json:"soname"`
		File   string `json:"file"`
		Abi    string `json:"abi"`
	} `json:"libs"`
	Keep    []string `json:"keep"`
	Scripts struct {
		Postinst string `json:"postinst"`
		Prerm    string `json:"prerm"`
		Migrate  string `json:"migrate"`
	} `json:"scripts"`
	Standalone *struct {
		Prog string `json:"prog"`
		Init string `json:"init"`
	} `json:"standalone"`
}

func parsePlugin(b []byte) (*pluginDoc, error) {
	var p pluginDoc
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// archs — список архитектур манифеста; all — «нет машинного кода».
func (p *pluginDoc) archs() (list []string, all bool, err error) {
	var s string
	if json.Unmarshal(p.Arch, &s) == nil {
		if s == "all" {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("arch: строка может быть только \"all\"")
	}
	if err := json.Unmarshal(p.Arch, &list); err != nil {
		return nil, false, err
	}
	return list, false, nil
}

func (p *pluginDoc) variant() string {
	if p.Variant == "" {
		return "full"
	}
	return p.Variant
}

func (p *pluginDoc) hasPerm(want string) bool {
	for _, x := range p.Permissions {
		if x == want {
			return true
		}
	}
	return false
}

// assetKey — ключ assets в индексе: арх или all, для варианта не full — арх@вариант.
func (p *pluginDoc) assetKey(arch string) string {
	if v := p.variant(); v != "full" {
		return arch + "@" + v
	}
	return arch
}

// provides — что версия регистрирует в ядре (docs/index.md, «Сборка индекса»).
func (p *pluginDoc) provides() map[string]any {
	out := map[string]any{}
	var kinds, cmds, langs, libs []string
	for _, k := range p.Steer.Kinds {
		kinds = append(kinds, k.Name)
	}
	for _, c := range p.Steer.Commands {
		cmds = append(cmds, c.Name)
	}
	if p.I18n != nil {
		for l := range p.I18n.Host {
			langs = append(langs, l)
		}
	}
	for _, l := range p.Libs {
		libs = append(libs, l.Soname)
	}
	sort.Strings(kinds)
	sort.Strings(cmds)
	sort.Strings(langs)
	sort.Strings(libs)
	if len(kinds) > 0 {
		out["kinds"] = kinds
	}
	if len(cmds) > 0 {
		out["commands"] = cmds
	}
	if len(langs) > 0 {
		out["languages"] = langs
	}
	if len(libs) > 0 {
		out["libs"] = libs
	}
	return out
}
