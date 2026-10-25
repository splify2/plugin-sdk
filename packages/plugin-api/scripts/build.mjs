// Сборка пакета: проверка типов и раскладка dist/. Типы написаны руками (src/index.d.ts) —
// tsc их не порождает, а проверяет (strict, без skipLibCheck) и прокладка dist копируется как есть.
import { spawnSync } from 'node:child_process'
import { cpSync, mkdirSync, rmSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const tsc = join(root, 'node_modules', 'typescript', 'bin', 'tsc')

const r = spawnSync(process.execPath, [tsc, '-p', join(root, 'tsconfig.json')], { stdio: 'inherit', cwd: root })
if (r.status !== 0) process.exit(r.status ?? 1)

rmSync(join(root, 'dist'), { recursive: true, force: true })
mkdirSync(join(root, 'dist'), { recursive: true })
for (const f of ['index.d.ts', 'index.js', 'vite.js', 'vite.d.ts']) cpSync(join(root, 'src', f), join(root, 'dist', f))
console.log('dist/: index.d.ts index.js vite.js vite.d.ts')
