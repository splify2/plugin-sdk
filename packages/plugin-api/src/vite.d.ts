import type { Plugin } from 'vite'

export interface Splify2Options {
  /** Входной файл интерфейса. По умолчанию ui-src/index.tsx (или .ts). */
  entry?: string
  /** Куда класть результат. По умолчанию — переменная S2P_UI_OUT (её задаёт `s2p build`), иначе dist-ui. */
  outDir?: string
}

/**
 * Плагины vite для UI плагина splify2: один ES-модуль ui/index.js (+ ui/style.css), preact и react
 * подменены прокладкой на globalThis.__splify2_host.preact — экземпляр панели (docs/host-api.md).
 */
export function splify2Plugin(opts?: Splify2Options): Plugin[]

/** Исходники прокладок по имени модуля — для тестов и для сборщиков, не знающих vite. */
export const shims: Readonly<Record<string, string>>
