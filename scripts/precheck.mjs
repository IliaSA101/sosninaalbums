// Быстрая предпроверка для исполнителей перед отчётом: `node scripts/precheck.mjs [--base <rev>]`.
// Ловит то, что иначе падает только на полном check.ps1 у оркестратора: gofmt, vet, golangci-lint (только новое),
// prettier, eslint, svelte-check, vitest related, размер файлов, лимит файлов в папке. Тесты Go не гоняет —
// исполнитель гоняет свои; test-baseline тоже не проверяется (нужен полный прогон всех тестов — шаг оркестратора).
// Пороги: размер — как CheckSizes в check.ps1; папки — читаются из scripts/folder-exceptions.txt.
import { spawnSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
process.chdir(root)
const gotmp = path.join(root, '.gotmp') // антивирус: exe go только сюда (как в check.ps1)
fs.mkdirSync(gotmp, { recursive: true })
const env = { ...process.env, GOTMPDIR: gotmp }

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
  const latest = ints.code === 0 ? ints.out.split('\n').map((l) => l.trim()).filter(Boolean) : []
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
  const dirs = [...new Set(goFiles.map((f) => (f.includes('/') ? f.slice(0, f.lastIndexOf('/')) : '.')))]
  const pkgs = dirs.map((d) => (d === '.' ? '.' : `./${d}`))
  step('gofmt', () => res(run('gofmt', ['-l', ...goFiles]), true))
  step('go vet', () => res(run('go', ['vet', ...pkgs])))
  step('golangci-lint', () => {
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
}

// ---- Всегда ----
const tracked = git('ls-files', '--cached', '--others', '--exclude-standard')
  .out.split('\n')
  .map((s) => s.trim())
  .filter((f) => f && fs.existsSync(path.join(root, f)))

step('размер файлов', () => {
  // Как CheckSizes в check.ps1, но только по изменённым файлам
  const skip = /(^|\/)(wailsjs|build|docs\/private|docs\/refs)\/|\.d\.ts$/
  const bad = []
  for (const f of changed) {
    if (!/\.(go|ts|js|svelte|css|ps1|py)$/.test(f) || skip.test(f)) continue
    const buf = fs.readFileSync(path.join(root, f))
    const text = buf.toString('utf8')
    // Get-Content считает строки без учёта хвостового перевода строки
    const lines = text.length === 0 ? 0 : text.replace(/\r?\n$/, '').split(/\r?\n/).length
    if (lines > 600 || buf.length > 30 * 1024) bad.push(`${f} (${lines} строк, ${(buf.length / 1024).toFixed(1)} КБ)`)
  }
  return { ok: bad.length === 0, out: bad.join('\n') }
})

step('folder limit', () => {
  // Как CheckFolders в check.ps1: считаются по всему дереву, исключения — scripts/folder-exceptions.txt
  const skip = /(^|\/)(wailsjs|testdata|node_modules)\/|\.d\.ts$/
  const maxFiles = {}
  const maxDirs = {}
  const exc = path.join(root, 'scripts', 'folder-exceptions.txt')
  if (fs.existsSync(exc)) {
    for (const line of fs.readFileSync(exc, 'utf8').split(/\r?\n/)) {
      let m = line.match(/^\s*([^\s#]+)\/\s+(\d+)/)
      if (m) maxDirs[m[1]] = Number(m[2])
      else if ((m = line.match(/^\s*([^\s#]+)\s+(\d+)/))) maxFiles[m[1]] = Number(m[2])
    }
  }
  const out = []
  const code = tracked.filter((f) => /\.(go|ts|svelte)$/.test(f) && !skip.test(f))
  const cnt = {}
  for (const f of code) {
    const d = f.includes('/') ? f.slice(0, f.lastIndexOf('/')) : '.'
    cnt[d] = (cnt[d] || 0) + 1
  }
  for (const [d, n] of Object.entries(cnt)) {
    const lim = maxFiles[d] ?? 20
    if (n > lim) out.push(`${d}: ${n} files (max ${lim})`)
  }
  const kids = {}
  for (const f of tracked) {
    const parts = f.split('/')
    let parent = '.'
    for (let i = 0; i < parts.length - 1; i++) {
      ;(kids[parent] ||= new Set()).add(parts[i])
      parent = i === 0 ? parts[0] : `${parent}/${parts[i]}`
    }
  }
  for (const [p, s] of Object.entries(kids)) {
    const lim = maxDirs[p] ?? 20
    if (s.size > lim) out.push(`${p}: ${s.size} folders (max ${lim})`)
  }
  for (const f of code) {
    const n = f.split('/').length
    if (f.startsWith('frontend/src/') && n !== 3 && n !== 5) {
      out.push(`${f.slice(0, f.lastIndexOf('/'))}: code outside frontend/src/<group>/<area>`)
    }
  }
  return { ok: out.length === 0, out: [...new Set(out)].join('\n') }
})

const time = `${((Date.now() - total) / 1000).toFixed(1)}s`
if (failed.length) {
  console.log(`ИТОГО: FAIL - ${failed.join(', ')} (${time})`)
  process.exit(1)
}
console.log(`ИТОГО: OK (${time})`)
