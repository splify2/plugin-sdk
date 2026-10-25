import { defineConfig } from 'vite'
import { splify2Plugin } from '@splify2/plugin-api/vite'

// Один ES-модуль и style.css в каталог S2P_UI_OUT (его задаёт `s2p build`); preact — общий с панелью.
export default defineConfig({ plugins: [splify2Plugin()] })
