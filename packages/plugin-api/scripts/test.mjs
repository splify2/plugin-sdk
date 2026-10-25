// Проверка после сборки: константа версии равна версии пакета, прокладки разбираются и работают
// на подставном хосте, а vite-плагин отдаёт конфиг для библиотеки ES.
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const pkg = JSON.parse(readFileSync(join(root, 'package.json'), 'utf8'))
const dist = f => pathToFileURL(join(root, 'dist', f)).href

const { HOST_API_VERSION } = await import(dist('index.js'))
assert.equal(HOST_API_VERSION, pkg.version, 'HOST_API_VERSION должна быть равна версии пакета')

const { shims, splify2Plugin } = await import(dist('vite.js'))
const calls = []
globalThis.__splify2_host = {
  preact: {
    h: (...a) => ({ h: a }),
    Fragment: Symbol('Fragment'),
    render: () => {},
    hooks: Object.fromEntries(['useState', 'useEffect', 'useRef', 'useMemo', 'useCallback', 'useReducer', 'useContext',
      'useLayoutEffect', 'useImperativeHandle', 'useDebugValue', 'useErrorBoundary', 'useId'].map(n => [n, (...a) => (calls.push(n), a)])),
  },
}
for (const [name, src] of Object.entries(shims)) {
  const m = await import('data:text/javascript;base64,' + Buffer.from(src).toString('base64'))
  assert.ok(Object.keys(m).length > 0, `прокладка ${name} ничего не экспортирует`)
  if (name === 'preact/hooks') { m.useState(1); assert.deepEqual(calls, ['useState']) }
  if (name === 'preact/jsx-runtime') assert.deepEqual(m.jsx('div', { id: 'a', children: 'x' }, 'k'), { h: ['div', { id: 'a', key: 'k' }, 'x'] })
}
delete globalThis.__splify2_host
const bare = await import('data:text/javascript;base64,' + Buffer.from(shims.preact + '// ' + Math.random()).toString('base64')).then(() => 'loaded', e => e.message)
assert.match(bare, /вне панели/, 'без хоста прокладка должна отказать понятным сообщением')

const plugins = splify2Plugin({ entry: 'x.tsx', outDir: 'out' })
const cfg = plugins.find(p => p.name === 'splify2:build').config()
assert.deepEqual(cfg.build.lib.formats, ['es'])
assert.equal(cfg.build.outDir, 'out')
assert.equal(plugins[0].resolveId('preact/hooks').startsWith('\0splify2-shim:'), true)
assert.equal(plugins[0].resolveId('lodash'), null)
console.log('plugin-api: ok')
