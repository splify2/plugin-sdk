// Сборка UI плагина splify2 на vite. Подключение — vite.config.ts плагина:
//
//   import { defineConfig } from 'vite'
//   import { splify2Plugin } from '@splify2/plugin-api/vite'
//   export default defineConfig({ plugins: [splify2Plugin()] })
//
// Что делает: собирает ES-модуль без чанков, а preact, preact/hooks, preact/jsx-runtime, react,
// react/jsx-runtime и react-dom подменяет прокладкой, которая берёт всё из
// globalThis.__splify2_host.preact — панель ставит его до первого import() плагина. Свой экземпляр
// preact сломал бы хуки (docs/host-api.md, «Модуль плагина»).
import { existsSync } from 'node:fs'

const HOOKS = [
  'useState', 'useReducer', 'useEffect', 'useLayoutEffect', 'useRef', 'useImperativeHandle', 'useMemo',
  'useCallback', 'useContext', 'useDebugValue', 'useErrorBoundary', 'useId',
]

const host = `const p = globalThis.__splify2_host && globalThis.__splify2_host.preact
if (!p) throw new Error('splify2: плагин загружен вне панели — нет globalThis.__splify2_host')
`

const hooksExport = HOOKS.map(n => `export const ${n} = (...a) => p.hooks.${n}(...a)`).join('\n')

const jsxRuntime = `${host}
const h = p.h
export const Fragment = p.Fragment
export function jsx(type, props, key) {
  const { children, ...rest } = props
  if (key !== undefined) rest.key = key
  return children === undefined ? h(type, rest) : h(type, rest, children)
}
export const jsxs = jsx
export const jsxDEV = jsx
`

const preactShim = `${host}
export const h = p.h
export const createElement = p.h
export const Fragment = p.Fragment
export const render = p.render
`

const hooksShim = `${host}
${hooksExport}
`

const reactShim = `${host}
${hooksExport}
export const createElement = p.h
export const Fragment = p.Fragment
export default { createElement: p.h, Fragment: p.Fragment, ${HOOKS.map(n => `${n}: (...a) => p.hooks.${n}(...a)`).join(', ')} }
`

const reactDomShim = `${host}
export const render = p.render
export default { render: p.render }
`

export const shims = {
  'preact': preactShim,
  'preact/hooks': hooksShim,
  'preact/jsx-runtime': jsxRuntime,
  'preact/jsx-dev-runtime': jsxRuntime,
  'react': reactShim,
  'react/jsx-runtime': jsxRuntime,
  'react/jsx-dev-runtime': jsxRuntime,
  'react-dom': reactDomShim,
}

const PREFIX = '\0splify2-shim:'

export function splify2Plugin(opts = {}) {
  const entry = opts.entry ?? ['ui-src/index.tsx', 'ui-src/index.ts', 'src/index.tsx', 'src/index.ts'].find(existsSync) ?? 'ui-src/index.tsx'
  return [
    {
      name: 'splify2:shim',
      enforce: 'pre',
      resolveId(id) {
        return id in shims ? PREFIX + id : null
      },
      load(id) {
        return id.startsWith(PREFIX) ? shims[id.slice(PREFIX.length)] : null
      },
    },
    {
      name: 'splify2:build',
      config() {
        return {
          esbuild: { jsx: 'automatic', jsxImportSource: 'preact' },
          build: {
            target: 'es2022',
            outDir: opts.outDir ?? process.env.S2P_UI_OUT ?? 'dist-ui',
            emptyOutDir: true,
            cssCodeSplit: false,
            sourcemap: false,
            lib: { entry, formats: ['es'], fileName: () => 'index.js', cssFileName: 'style' },
            rollupOptions: { output: { inlineDynamicImports: true } },
          },
        }
      },
    },
  ]
}
