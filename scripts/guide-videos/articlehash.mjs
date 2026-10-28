#!/usr/bin/env node
// Хеш текста статьи F1 для GUIDE_VIDEOS (спека 140, design §6): FNV-1a 32 по сырым `title\nwhat\nwhere\nhow`,
// без подстановки клавиш; пустое поле — пустая строка; печатается 8 знаков hex.
//
//   node scripts/guide-videos/articlehash.mjs <id> [<id>…]   → по строке «id хеш»; нет статьи — код 1
//
// Статьи — TypeScript фронта: собираются esbuild'ом в память (esbuild берётся из frontend/node_modules; в копии
// агента её нет — ищется в основном дереве по `.git`-файлу worktree). Файлы не пишутся.
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const root = path.resolve(here, '..', '..')

// Корень основного дерева: у worktree `.git` — файл `gitdir: <основное>/.git/worktrees/<имя>`.
export function mainRoot(r = root) {
  const g = path.join(r, '.git')
  try {
    if (fs.statSync(g).isFile()) {
      const m = /^gitdir:\s*(.+?)[/\\]\.git[/\\]worktrees[/\\]/.exec(fs.readFileSync(g, 'utf8').trim())
      if (m) return m[1]
    }
  } catch {
    // нет .git — считаем корнем сам r
  }
  return r
}

// FNV-1a 32 по байтам UTF-8; Math.imul держит умножение в 32 битах.
export function fnv1a32(s) {
  let h = 0x811c9dc5
  for (const b of Buffer.from(s, 'utf8')) h = Math.imul(h ^ b, 0x01000193) >>> 0
  return h
}

export function articleHash(a) {
  return fnv1a32([a.title, a.what ?? '', a.where ?? '', a.how ?? ''].join('\n')).toString(16).padStart(8, '0')
}

async function loadArticles() {
  const src = path.join(root, 'frontend', 'src', 'shell', 'guidetext', 'articles.ts')
  const nm = [root, mainRoot()].map((r) => path.join(r, 'frontend', 'node_modules')).find((d) => fs.existsSync(path.join(d, 'esbuild')))
  if (!nm) throw new Error('нет frontend/node_modules/esbuild (npm ci во frontend основного дерева)')
  const esbuild = createRequire(path.join(nm, 'x.js'))('esbuild')
  const r = await esbuild.build({ entryPoints: [src], bundle: true, write: false, format: 'esm', platform: 'node', logLevel: 'silent' })
  const url = 'data:text/javascript;base64,' + Buffer.from(r.outputFiles[0].text).toString('base64')
  return (await import(url)).ARTICLES
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const ids = process.argv.slice(2)
  if (ids.length === 0) {
    console.error('articlehash: нужен id статьи')
    process.exit(2)
  }
  const list = await loadArticles()
  let bad = 0
  for (const id of ids) {
    const a = list.find((x) => x.id === id)
    if (!a) {
      console.error(`articlehash: нет статьи ${id}`)
      bad = 1
      continue
    }
    console.log(`${id} ${articleHash(a)}`)
  }
  process.exit(bad)
}
