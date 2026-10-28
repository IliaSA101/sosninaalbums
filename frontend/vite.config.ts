import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import { writeFileSync } from 'node:fs'
import { releaseCsp } from './src/shell/boot/csp'

export default defineConfig(({ mode }) => ({
  // Условие browser — только для тестов реактивности ($effect.root + flushSync, спека 045).
  ...(mode === 'test' ? { ssr: { resolve: { conditions: ['browser'] } }, resolve: { conditions: ['browser'] } } : {}),
  plugins: [
    // Тесты идут на клиентской сборке Svelte (компиляция + условие browser выше): на серверной $effect и flushSync пустые.
    svelte(mode === 'test' ? { dynamicCompileOptions: () => ({ generate: 'client' }) } : {}),
    // CSP: ws:// и worker-src blob: нужны только HMR dev-сервера, в выпуск не попадают.
    {
      name: 'csp-no-ws',
      apply: 'build',
      transformIndexHtml: releaseCsp,
    },
    // Vite очищает dist при сборке, а go:embed нужен хотя бы один файл в чистом клоне.
    { name: 'keep-dist', closeBundle: () => writeFileSync('dist/gitkeep', '') },
  ],
  // Один бандл грузится из exe (go:embed), не по сети: порог 500 КБ для сайтов здесь ни о чём,
  // а предупреждение роняет check.ps1. Поднято с 0.10 (503 КБ после 034), 1500 — с 0.21 (1004 КБ после 081).
  build: { chunkSizeWarningLimit: 1500 },
}))
