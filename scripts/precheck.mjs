// Быстрая предпроверка для исполнителей перед отчётом: `node scripts/precheck.mjs [--base <rev>]`.
// Ловит то, что иначе падает только на полном check.ps1 у оркестратора: gofmt, vet по всему модулю, golangci-lint
// (только новое), go test -short изменённых пакетов, покрытие дерева album (при правке internal/album/**) против эталона,
// prettier, eslint, svelte-check, vitest related + стражи (тесты со сканом дерева), язык комментариев, размер файлов, лимит файлов в папке. Полный go test и test-baseline
// остаются шагом оркестратора (check.ps1 без -short: эталоны и большие наборы там).
// Пределы размера и папок — scripts/limits.mjs (один источник с check.ps1, pre-commit и хуком file-size).
import { spawnSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { TREE_KEY, compareTree, isTree, treeCover } from './tree-cover.mjs'
import { withGoCaches } from './goenv.mjs'
import { checkFolders, checkSizes } from './limits.mjs'
import { collect as collectAdded, violations as langViolations } from './comment-lang.mjs'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
process.chdir(root)
const gotmp = path.join(root, '.gotmp') // антивирус: exe go только сюда (как в check.ps1)
fs.mkdirSync(gotmp, { recursive: true })
const env = withGoCaches({ ...process.env, GOTMPDIR: gotmp }, root)

// Пакеты модуля: не ./... — тот заходит в frontend/node_modules (flatted/golang). Один шаблон для всех скриптов и CI.
const MODULE_PKGS = ['.', './internal/...']

const argv = process.argv.slice(2)
const bi = argv.indexOf('--base')

const run = (cmd, args, opts = {}) => {
  const r = spawnSync(cmd, args, { cwd: root, env, encoding: 'utf8', maxBuffer: 1 << 28, ...opts })
  return { code: r.status ?? 1, out: `${r.stdout || ''}${r.stderr || ''}` }
}
const git = (...a) => run('git', a)
const q = (s) => (/[\s"&|<>^()]/.test(s) ? `"${s.replace(/"/g, '\\"')}"` : s)
// npx — .cmd: нужен shell, аргументы кладём в командную строку по частям (предел cmd.exe 8 тыс. символов)
const chunks = (files, size = 40) => {
  const res = []
  for (let i = 0; i < files.length; i += size) res.push(files.slice(i, i + size))
  return res
}

let base = bi >= 0 ? argv[bi + 1] : ''
if (!base) {
  // Интеграционная ветка текущей версии — int-* с наибольшей версией (номер версии не зашиваем: в 0.13 база int-0.12
  // заставляла агентов гонять линтер по всему приросту версии, ~4,5 мин).
  const ints = git('for-each-ref', '--sort=-version:refname', '--format=%(refname:short)', 'refs/heads/int-*')
  const latest =
    ints.code === 0
      ? ints.out
          .split('\n')
          .map((l) => l.trim())
          .filter(Boolean)
      : []
  const head = git('rev-parse', 'HEAD').out.trim()
  for (const b of [...latest.slice(0, 1), 'dev']) {
    if (git('rev-parse', '--verify', '--quiet', b).code !== 0) continue
    const mb = git('merge-base', 'HEAD', b)
    if (mb.code === 0) {
      base = mb.out.trim()
      // Запуск на самой int-* ветке: merge-base == HEAD, дифф пуст — берём следующую базу (dev)
      if (base === head) continue
      break
    }
  }
  if (!base && head) {
    base = head
    console.warn('precheck: дифф пуст: база = HEAD')
  }
}
if (!base) {
  console.error('precheck: не найдена база (ни int-*, ни dev); укажи --base <rev>')
  process.exit(2)
}

const changedSet = new Set(
  git('diff', '--name-only', base)
    .out.split('\n')
    .map((s) => s.trim())
    .filter(Boolean),
)
for (const l of git('status', '--porcelain', '--untracked-files=all').out.split('\n')) {
  if (!l.trim()) continue
  let p = l.slice(3).trim()
  if (p.includes(' -> ')) p = p.split(' -> ')[1]
  changedSet.add(p.replace(/^"|"$/g, ''))
}
const changed = [...changedSet].filter((f) => fs.existsSync(path.join(root, f)))
if (changed.length === 0) {
  console.log('нечего проверять')
  process.exit(0)
}

const failed = []
const total = Date.now()
const step = (name, fn) => {
  const t = Date.now()
  const r = fn()
  const time = `${((Date.now() - t) / 1000).toFixed(1)}s`
  if (r.ok) console.log(`OK   ${name} (${time})`)
  else {
    console.log(`FAIL ${name} (${time})`)
    r.out
      .split('\n')
      .filter((l) => l.trim())
      .slice(-30)
      .forEach((l) => console.log(`     ${l}`))
    failed.push(name)
  }
}
const res = (r, emptyOut = false) => ({ ok: r.code === 0 && !(emptyOut && r.out.trim()), out: r.out })

// ---- Go ----
const goFiles = changed.filter((f) => f.endsWith('.go'))
if (goFiles.length) {
  // Разовые скрипты с //go:build ignore (scripts/*.go) — не пакет: go test и golangci-lint на их папке падают
  const ignored = (f) => /^\/\/go:build ignore\b/m.test(fs.readFileSync(path.join(root, f), 'utf8').slice(0, 400))
  const built = goFiles.filter((f) => !ignored(f))
  const dirs = [...new Set(built.map((f) => (f.includes('/') ? f.slice(0, f.lastIndexOf('/')) : '.')))]
  const pkgs = dirs.map((d) => (d === '.' ? '.' : `./${d}`))
  step('gofmt', () => res(run('gofmt', ['-l', ...goFiles]), true))
  // vet по всему модулю (2–4 с): ловит чужие вызовы и тесты после смены сигнатуры, а не только изменённые пакеты
  step('go vet', () => res(run('go', ['vet', ...MODULE_PKGS])))
  step('go test -short', () => {
    if (!pkgs.length) return res({ code: 0, out: '' })
    // Больше трёх пакетов — через очередь: go test занимает все ядра
    const goTest = ['test', '-short', '-count=1', ...pkgs]
    return res(pkgs.length > 3 ? run('node', ['scripts/heavy.mjs', 'go', ...goTest]) : run('go', goTest))
  })
  if (goFiles.some((f) => isTree(path.posix.dirname(f)))) {
    step('покрытие дерева album', () => {
      const baseJson = path.join(root, 'testdata', 'test-baseline.json')
      const entry = fs.existsSync(baseJson) ? JSON.parse(fs.readFileSync(baseJson, 'utf8')).go?.[TREE_KEY] : null
      if (!entry) return { ok: false, out: `в testdata/test-baseline.json нет записи ${TREE_KEY}` }
      let cur
      try {
        cur = treeCover({ root, env, heavy: true })
      } catch (e) {
        return { ok: false, out: String(e.message) }
      }
      const errors = compareTree(entry, cur)
      if (errors.length === 0) return { ok: true, out: '' }
      // Где не хватает теста: непокрытые блоки изменённых файлов (остальные — наследие эталона)
      const mine = cur.missedBlocks
        .map((b) => b.replace(/^[^/]+\/[^/]+\/[^/]+\//, '')) // модуль github.com/<owner>/<repo>/
        .filter((b) => goFiles.includes(b.slice(0, b.lastIndexOf(':'))) && !b.includes('_test.go'))
      const lines = mine.length ? [`непокрытые блоки в изменённых файлах (${mine.length}):`, ...mine.slice(0, 12)] : []
      return { ok: false, out: [...lines, ...errors].join('\n') }
    })
  }
  step('golangci-lint', () => {
    if (!pkgs.length) return res({ code: 0, out: '' })
    const gcl = spawnSync(process.platform === 'win32' ? 'where' : 'which', ['golangci-lint'], { encoding: 'utf8' })
    let exe = gcl.status === 0 ? gcl.stdout.split('\n')[0].trim() : ''
    if (!exe) {
      const gp = run('go', ['env', 'GOPATH']).out.trim()
      exe = path.join(gp, 'bin', process.platform === 'win32' ? 'golangci-lint.exe' : 'golangci-lint')
    }
    return res(run('node', ['scripts/heavy.mjs', exe, 'run', '--allow-parallel-runners', `--new-from-rev=${base}`, ...pkgs]))
  })
}

// ---- Фронт ----
// scanGuards — тесты src, читающие всё дерево через import.meta.glob (пути от frontend/, как у vitest)
function scanGuards(dir) {
  const res = []
  const walk = (d) => {
    for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      const p = path.join(d, e.name)
      if (e.isDirectory()) walk(p)
      else if (/\.test\.ts$/.test(e.name) && fs.readFileSync(p, 'utf8').includes('import.meta.glob')) {
        res.push(path.relative(path.join(root, 'frontend'), p).replaceAll('\\', '/'))
      }
    }
  }
  walk(dir)
  return res.sort()
}
const fe = changed
  .filter((f) => /^frontend\//.test(f) && !/^frontend\/(wailsjs|dist)\//.test(f) && !/package(-lock)?\.json$/.test(f))
  .filter((f) => /\.(ts|js|svelte|css|html|json)$/.test(f))
  .map((f) => f.slice('frontend/'.length))
if (fe.length) {
  const npx = (parts) => {
    let out = ''
    let code = 0
    for (const part of parts) {
      const r = run(`npx ${part.map(q).join(' ')}`, [], { cwd: path.join(root, 'frontend'), shell: true })
      out += r.out
      if (r.code !== 0) code = r.code
    }
    return { code, out }
  }
  const fePart = (pre, files) => chunks(files).map((c) => [...pre, ...c])
  step('prettier', () => res(npx(fePart(['prettier', '--check', '--log-level', 'warn'], fe))))
  const lint = fe.filter((f) => /\.(ts|js|svelte)$/.test(f))
  if (lint.length) step('eslint', () => res(npx(fePart(['eslint', '--max-warnings', '0'], lint))))
  if (fe.some((f) => /\.(ts|svelte)$/.test(f))) {
    step('svelte-check', () => res(npx([['svelte-check', '--output', 'human', '--fail-on-warnings']])))
  }
  const src = fe.filter((f) => /^src\/.*\.(ts|js|svelte)$/.test(f))
  if (src.length) step('vitest related', () => res(npx(fePart(['vitest', 'related', '--run', '--passWithNoTests'], src))))
  // Стражи: тесты-сканеры всего дерева фронта (import.meta.glob) — vitest related их не выбирает, а ломаются они от
  // любой правки src. Список вычисляем по содержимому, не храним.
  if (src.length) {
    const guards = scanGuards(path.join(root, 'frontend', 'src'))
    if (guards.length) step(`стражи фронта (${guards.length})`, () => res(npx([['vitest', 'run', ...guards]])))
  }
}

// ---- Python: e2e-скрипты (089) ----
// Юнит-тесты чистых частей scripts/e2e; нет каталога или Python — шаг пропускается молча.
if (changed.some((f) => /^scripts\/e2e\/.*\.py$/.test(f)) && fs.existsSync(path.join(root, 'scripts', 'e2e'))) {
  const py = spawnSync('python', ['--version'], { encoding: 'utf8' })
  if (py.status === 0) {
    step('python unittest (scripts/e2e)', () => res(run('python', ['-m', 'unittest', 'discover', '-s', 'scripts/e2e'])))
  }
}

// ---- Всегда ----
const tracked = git('ls-files', '--cached', '--others', '--exclude-standard')
  .out.split('\n')
  .map((s) => s.trim())
  .filter((f) => f && fs.existsSync(path.join(root, f)))

step('язык комментариев', () => {
  // Только добавленные строки (дифф к базе + рабочее дерево); исключение строки — маркер lang-ok
  const seen = new Set()
  const bad = langViolations(collectAdded({ root, base }))
    .filter((v) => !seen.has(`${v.file}:${v.line}`) && seen.add(`${v.file}:${v.line}`))
    .map((v) => `${v.file}:${v.line}: ${v.text}`)
  return { ok: bad.length === 0, out: bad.length ? ['английские комментарии — переписать по-русски (или lang-ok):', ...bad].join('\n') : '' }
})

step('размер файлов', () => {
  // Только изменённые файлы; пределы по языку — limits.mjs
  const bad = checkSizes(changed, root)
  return { ok: bad.length === 0, out: bad.join('\n') }
})

step('folder limit', () => {
  // По всему дереву, как в check.ps1; исключения — scripts/folder-exceptions.txt
  const out = checkFolders(tracked, root)
  return { ok: out.length === 0, out: out.join('\n') }
})

const time = `${((Date.now() - total) / 1000).toFixed(1)}s`
if (failed.length) {
  console.log(`ИТОГО: FAIL - ${failed.join(', ')} (${time})`)
  process.exit(1)
}
console.log(`ИТОГО: OK (${time})`)
