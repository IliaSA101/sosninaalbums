import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import { writeFileSync } from 'node:fs'

export default defineConfig({
  plugins: [
    svelte(),
    // CSP: ws:// нужен только HMR dev-сервера, в выпуск он не попадает.
    {
      name: 'csp-no-ws',
      apply: 'build',
      transformIndexHtml: (html: string) => html.replace(' ws://localhost:* ws://127.0.0.1:*', ''),
    },
    // Vite очищает dist при сборке, а go:embed нужен хотя бы один файл в чистом клоне.
    { name: 'keep-dist', closeBundle: () => writeFileSync('dist/gitkeep', '') },
  ],
  // Один бандл грузится из exe (go:embed), не по сети: порог 500 КБ для сайтов здесь ни о чём,
  // а предупреждение роняет check.ps1. Поднято с 0.10 (503 КБ после 034).
  build: { chunkSizeWarningLimit: 1000 },
})
