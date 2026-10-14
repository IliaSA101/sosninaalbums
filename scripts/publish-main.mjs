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
// `--only <путь>…` — служебный коммит поверх main: из <ref> берутся только указанные файлы/папки,
// остальное дерево — как в main (docs/specs/0.10/029-install-guide/design.md §2).
const onlyAt = args.indexOf('--only')
const only = []
if (onlyAt >= 0) {
  for (const a of args.slice(onlyAt + 1)) {
    if (a.startsWith('-')) break
    only.push(a)
  }
}
const ref = args.find((a, i) => !a.startsWith('-') && args[i - 1] !== '-m' && (onlyAt < 0 || i < onlyAt))
const msg = args.includes('-m') ? args[args.indexOf('-m') + 1] : null
if (!ref || (!msg && !flag('--dry')) || (onlyAt >= 0 && (!only.length || flag('--fresh')))) {
  console.error('usage: node scripts/publish-main.mjs <ref> -m "Sosnina Albums X.Y.Z" [--fresh] [--dry]')
  console.error('       node scripts/publish-main.mjs <ref> --only <путь>… -m "<сообщение>" [--dry]')
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

// Служебный коммит: main + только указанные пути из ref.
function publishOnly() {
  // Сначала свежий origin/main: без fetch сравнение шло бы со старой копией, и коммит мог затереть
  // правку, сделанную на GitHub (находка второго варианта замера exec:19).
  if (git(['remote']).split('\n').includes('origin')) git(['fetch', '--quiet', 'origin', 'main'])
  const hasOrigin = git(['branch', '-r', '--list', 'origin/main']) !== ''
  if (hasOrigin && git(['rev-parse', 'main']) !== git(['rev-parse', 'origin/main'])) {
    throw new Error('локальный main расходится с origin/main: сначала git fetch и выровнять')
  }
  gi(['read-tree', 'main'])
  const norm = only.map((p) => p.replace(/\\/g, '/').replace(/^\.\//, '').replace(/\/+$/, ''))
  const under = (f, p) => f === p || f.startsWith(p + '/')
  // Путь попадает в исключённое: сам исключён или лежит внутри исключённой папки.
  const bad = norm.filter((p) => excluded(p) || exclude.some((e) => e.endsWith('/') && (p + '/').startsWith(e)))
  if (bad.length) throw new Error('путь в publish-exclude.txt: ' + bad.join(', '))
  const refFiles = git(['ls-tree', '-r', ref])
    .split('\n')
    .filter(Boolean)
    .map((l) => {
      const [meta, name] = l.split('\t')
      const [mode, , sha] = meta.split(' ')
      return { mode, sha, name }
    })
  const mainFiles = gi(['ls-files']).split('\n').filter(Boolean)
  for (const p of norm) {
    const src = refFiles.filter((f) => under(f.name, p))
    if (!src.length) throw new Error(`в ${ref} нет файлов под путём ${p}`)
    const keep = new Set()
    for (const f of src) {
      if (excluded(f.name)) throw new Error('путь в publish-exclude.txt: ' + f.name)
      gi(['update-index', '--add', '--cacheinfo', `${f.mode},${f.sha},${f.name}`])
      keep.add(f.name)
    }
    const gone = mainFiles.filter((f) => under(f, p) && !keep.has(f))
    for (let i = 0; i < gone.length; i += 200) gi(['update-index', '--force-remove', '--', ...gone.slice(i, i + 200)])
  }
  const tree = gi(['write-tree'])
  if (tree === git(['rev-parse', 'main^{tree}'])) throw new Error('нечего публиковать: дерево совпало с main')
  const diff = git(['diff-tree', '-r', '--name-status', 'main', tree]).split('\n').filter(Boolean)
  console.log(diff.map((l) => l.replace('\t', ' ')).join('\n'))
  console.log(git(['diff-tree', '-r', '--shortstat', 'main', tree]))
  // Ссылки проверяем только в добавленных/изменённых .md.
  const have = new Set(gi(['ls-files']).split('\n').filter(Boolean))
  const broken = []
  for (const l of diff.filter((l) => /^[AM]\t.*\.md$/.test(l))) {
    const f = l.split('\t')[1]
    for (const m of git(['show', `${tree}:${f}`]).matchAll(/\]\(([^)\s]+)\)/g)) {
      if (/^[a-z]+:|^#|^mailto:/i.test(m[1])) continue
      const t = path.posix.normalize(path.posix.join(path.posix.dirname(f), decodeURI(m[1].split('#')[0])))
      if (!have.has(t) && ![...have].some((h) => h.startsWith(t.replace(/\/$/, '') + '/'))) broken.push(`${f}: ${m[1]}`)
    }
  }
  if (broken.length) {
    throw new Error(`битые ссылки в публикуемом (${broken.length}):\n  ` + broken.join('\n  '))
  }
  console.log('ссылки в .md целы')
  if (flag('--dry')) return
  const commit = git(['commit-tree', tree, '-p', 'main', '-m', msg])
  git(['update-ref', 'refs/heads/main', commit])
  console.log(`main = ${commit.slice(0, 7)} (${msg})`)
}

try {
  if (onlyAt >= 0) {
    publishOnly()
    fs.rmSync(index, { force: true })
    process.exit(0)
  }
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
