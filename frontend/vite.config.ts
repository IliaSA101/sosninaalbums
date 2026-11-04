import { defineConfig, type Plugin } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import { readdirSync, rmSync, statSync, writeFileSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'
import { releaseCsp } from './src/shell/boot/csp'

// Удаляет в outDir файлы, которых нет в новой сборке (прежние хэшированные бандлы) и которые старше её начала.
function pruneOld(dir: string, root: string, keep: Set<string>, since: number) {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    if (statSync(full).isDirectory()) {
      pruneOld(full, root, keep, since)
      if (readdirSync(full).length === 0) rmSync(full, { recursive: true })
    } else if (!keep.has(relative(root, full).split('\\').join('/')) && statSync(full).mtimeMs < since) {
      rmSync(full)
    }
  }
}

// Пока идёт сборка, в dist всегда лежит хотя бы gitkeep: go:embed в параллельном go test не должен видеть пустую папку.
// Поэтому Vite не очищает dist (emptyOutDir: false), а старые файлы убираются после записи нового бандла.
function keepDist(): Plugin {
  let outDir = ''
  let started = 0
  const keep = new Set<string>(['gitkeep'])
  return {
    name: 'keep-dist',
    apply: 'build',
    configResolved: (c) => {
      outDir = resolve(c.root, c.build.outDir)
    },
    buildStart: () => {
      started = Date.now()
    },
    writeBundle: (_o, bundle) => {
      for (const k of Object.keys(bundle)) keep.add(k)
    },
    closeBundle: () => {
      // gitkeep нужен go:embed в чистом клоне, где dist ещё нет.
      writeFileSync(join(outDir, 'gitkeep'), '')
      pruneOld(outDir, outDir, keep, started)
    },
  }
}

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
    keepDist(),
  ],
  // Один бандл грузится из exe (go:embed), не по сети: порог 500 КБ для сайтов здесь ни о чём,
  // а предупреждение роняет check.ps1. Поднято с 0.10 (503 КБ после 034), 1500 — с 0.21 (1004 КБ после 081),
  // 2000 — с 0.34 (1517 КБ после 150/151). emptyOutDir: false — см. keepDist.
  build: { chunkSizeWarningLimit: 2000, emptyOutDir: false },
}))
