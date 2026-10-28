package main

import "debug/elf"

// arch — пакетная архитектура OpenWrt и всё, что нужно для сборки под неё. Список — копия
// steer/build/arches.sh (ISAS, interp_of); источник истины — он, при расхождении правится эта таблица.
type archInfo struct {
	id, triple, mcpu, interp string
	machine                  elf.Machine
	class                    elf.Class
	order                    elf.Data
}

var arches = []archInfo{
	{"mipsel_24kc", "mipsel-linux-musl", "mips32r2+soft_float", "/lib/ld-musl-mipsel-sf.so.1", elf.EM_MIPS, elf.ELFCLASS32, elf.ELFDATA2LSB},
	{"mips_24kc", "mips-linux-musl", "mips32r2+soft_float", "/lib/ld-musl-mips-sf.so.1", elf.EM_MIPS, elf.ELFCLASS32, elf.ELFDATA2MSB},
	{"aarch64_cortex-a53", "aarch64-linux-musl", "cortex_a53", "/lib/ld-musl-aarch64.so.1", elf.EM_AARCH64, elf.ELFCLASS64, elf.ELFDATA2LSB},
	{"aarch64_cortex-a72", "aarch64-linux-musl", "cortex_a72", "/lib/ld-musl-aarch64.so.1", elf.EM_AARCH64, elf.ELFCLASS64, elf.ELFDATA2LSB},
	{"aarch64_generic", "aarch64-linux-musl", "baseline", "/lib/ld-musl-aarch64.so.1", elf.EM_AARCH64, elf.ELFCLASS64, elf.ELFDATA2LSB},
	{"arm_cortex-a7_neon-vfpv4", "arm-linux-musleabihf", "cortex_a7", "/lib/ld-musl-armhf.so.1", elf.EM_ARM, elf.ELFCLASS32, elf.ELFDATA2LSB},
	{"arm_cortex-a9", "arm-linux-musleabi", "cortex_a9", "/lib/ld-musl-arm.so.1", elf.EM_ARM, elf.ELFCLASS32, elf.ELFDATA2LSB},
	{"arm_cortex-a9_neon", "arm-linux-musleabihf", "cortex_a9+neon", "/lib/ld-musl-armhf.so.1", elf.EM_ARM, elf.ELFCLASS32, elf.ELFDATA2LSB},
	{"arm_cortex-a9_vfpv3-d16", "arm-linux-musleabihf", "cortex_a9+vfp3d16", "/lib/ld-musl-armhf.so.1", elf.EM_ARM, elf.ELFCLASS32, elf.ELFDATA2LSB},
	{"x86_64", "x86_64-linux-musl", "baseline", "/lib/ld-musl-x86_64.so.1", elf.EM_X86_64, elf.ELFCLASS64, elf.ELFDATA2LSB},
}

func findArch(id string) *archInfo {
	for i := range arches {
		if arches[i].id == id {
			return &arches[i]
		}
	}
	return nil
}

func archIDs() []string {
	var r []string
	for _, a := range arches {
		r = append(r, a.id)
	}
	return r
}

// Версии осей совместимости, под которые собран этот SDK (docs/manifest.md, «compat»).
const (
	sdkSteerAPI = "1.2.0"
	sdkSteerABI = "1.0.0"
	sdkHostAPI  = "1.0.0"
	// splify2-api: хуки, задачи, события, псевдонимы и переопределения методов бэкенда splify2; с 1.1 — интерфейс плагина
	// (`splify2.luci.interface`) и метод `paths`.
	sdkSplify2API = "1.1.0"
)
