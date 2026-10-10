// Сборка коммита версии для публичного main: `node scripts/publish-main.mjs <ref> -m "Sosnina Albums X.Y.Z" [--fresh] [--dry]`.
// Берёт дерево <ref> (коммит версии из dev), убирает пути из scripts/publish-exclude.txt и проверяет,
// что в публикуемых .md нет ссылок на убранное или несуществующее (иначе на GitHub — битые ссылки).
// Коммит — один на версию, родитель — прежний main; --fresh — без родителя (первая публикация),
// прежний main сохраняется веткой backup/main-<дата>. --dry — только проверка. Рабочую копию не трогает.
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'

const args = process.argv.slice(2)
const flag = (f) => args.includes(f)
const ref = args.find((a, i) => !a.startsWith('-') && args[i - 1] !== '-m')
const msg = args.includes('-m') ? args[args.indexOf('-m') + 1] : null
if (!ref || (!msg && !flag('--dry'))) {
  console.error('usage: node scripts/publish-main.mjs <ref> -m "Sosnina Albums X.Y.Z" [--fresh] [--dry]')
  process.exit(2)
}

const root = path.resolve(path.dirname(new URL(import.meta.url).pathname.replace(/^\/(\w:)/, '$1')), '..')
const index = path.join(os.tmpdir(), `publish-index-${process.pid}`)
const git = (a, env = {}) => execFileSync('git', a, { cwd: root, encoding: 'utf8', env: { ...process.env, ...env } }).trim()
const gi = (a) => git(a, { GIT_INDEX_FILE: index })

const exclude = fs
  .readFileSync(path.join(root, 'scripts', 'publish-exclude.txt'), 'utf8')
  .split('\n')
  .map((l) => l.trim())
  .filter((l) => l && !l.startsWith('#'))
const excluded = (p) => exclude.some((e) => (e.endsWith('/') ? p.startsWith(e) : p === e))

try {
  gi(['read-tree', ref])
  const all = gi(['ls-files']).split('\n').filter(Boolean)
  const drop = all.filter(excluded)
  for (let i = 0; i < drop.length; i += 200) gi(['update-index', '--force-remove', '--', ...drop.slice(i, i + 200)])
  const files = gi(['ls-files']).split('\n').filter(Boolean)
  const have = new Set(files)
  const dirs = new Set(
    files.flatMap((f) =>
      f
        .split('/')
        .slice(0, -1)
        .map((_, i, a) => a.slice(0, i + 1).join('/')),
    ),
  )

  // Ссылки Markdown на файлы репозитория: ](путь) без схемы и якоря-только.
  const broken = []
  for (const f of files.filter((f) => f.endsWith('.md'))) {
    const text = git(['show', `${ref}:${f}`])
    for (const m of text.matchAll(/\]\(([^)\s]+)\)/g)) {
      const link = m[1]
      if (/^[a-z]+:|^#|^mailto:/i.test(link)) continue
      const target = path.posix.normalize(path.posix.join(path.posix.dirname(f), decodeURI(link.split('#')[0])))
      if (!have.has(target) && !dirs.has(target.replace(/\/$/, ''))) broken.push(`${f}: ${link}`)
    }
  }
  if (files.some(excluded)) throw new Error('в дереве остались исключённые пути')
  if (broken.length) {
    console.error(`битые ссылки в публикуемом (${broken.length}):\n  ` + broken.join('\n  '))
    process.exit(1)
  }
  console.log(`публикуется файлов: ${files.length}, убрано: ${drop.length}, ссылки в .md целы`)
  if (flag('--dry')) process.exit(0)

  const tree = gi(['write-tree'])
  const hasMain = git(['branch', '--list', 'main']) !== ''
  const parent = hasMain && !flag('--fresh') ? ['-p', 'main'] : []
  const commit = git(['commit-tree', tree, ...parent, '-m', msg])
  if (hasMain && flag('--fresh')) {
    const backup = `backup/main-${new Date().toISOString().slice(0, 10)}`
    git(['branch', '-f', backup, 'main'])
    console.log(`прежний main сохранён: ${backup}`)
  }
  git(['update-ref', 'refs/heads/main', commit])
  console.log(`main = ${commit.slice(0, 7)} (${msg})`)
} finally {
  fs.rmSync(index, { force: true })
}
