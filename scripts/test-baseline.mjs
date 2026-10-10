// Эталон силы тестов (решение владельца 2026-10-09: тесты не ослаблять).
// Эталон — testdata/test-baseline.json: на каждый Go-пакет «имя теста (с подтестами TestX/sub) -> сколько раз»
// и покрытие строк, для фронта — «полное имя теста vitest -> сколько раз». Источник — json-вывод самих прогонов
// (go test -cover -json, vitest --reporter=json), а не списки: go test -list и vitest list не видят
// подтесты и тесты, созданные в цикле (vitest list даёт шаблон заголовка вместо имён).
//
// Режимы (без флага — печать текущих значений):
//   --check      сравнить с эталоном (exit 1 при провале)
//   --write      переписать эталон (обновляется только измеренная часть)
//   --run-go     прогнать `go test -cover -json ./...` (+ --race) в файл --out, напечатать сводку (шаг go test в check.ps1);
//                запускаем из node, чтобы кодировка консоли PowerShell 5.1 не портила кириллицу в именах
// Опции: --go / --ui — только своя часть (по умолчанию обе);
//   --go-json <файл>  готовый вывод go test -json (check.ps1 берёт из шага go test), иначе прогоняем сами;
//   --ui-json <файл>  готовый вывод `vitest run --reporter=json` (из шага vitest), иначе прогоняем сами.
//
// Провал: сумма по имени уменьшилась (с учётом переноса теста в другой пакет/файл под тем же именем),
// либо покрытие пакета упало больше чем на 0,5 п. п. Рост и новые пакеты — ок. Эталон обновляется только
// через --write, с причиной в сообщении коммита и с согласия владельца, если что-то уменьшилось.
// Go: тест, запущенный и пропущенный по среде (t.Skip, симлинки без прав), считается — иначе эталон зависел бы от машины;
// vitest: пропущенный (it.skip) для эталона потерян.
// Покрытие фронта не меряем: @vitest/coverage-* не установлен, новых зависимостей не ставим.
// Сообщения — латиницей: вывод идёт через консоль PowerShell 5.1 шага check.ps1.
import fs from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const target = path.join(root, 'testdata', 'test-baseline.json')
const MAX_DROP = 0.5 // п. п. покрытия

// Дерево альбома (023, решение владельца 2026-10-10): при переносе тесты уезжают в пакеты областей, а код,
// который они проверяют, остаётся в соседних пакетах — поштучное покрытие проседало бы на бумаге. Поэтому
// пакеты дерева сравниваются одной записью: имена суммой, покрытие — один прогон -coverpkg по всему дереву.
const TREE_ROOTS = ['internal/album']
const TREE_KEY = 'tree:album'
const TREE_MIN = 96.9 // % — уровень до 023 (1b61383: 3127 из 3226)
const TREE_MAX_MISSED = 90 // непокрытых блоков, критерий спеки 023
const isTree = (p) => TREE_ROOTS.some((r) => p === r || p.startsWith(r + '/'))

const args = process.argv.slice(2)
const flag = (n) => args.includes(n)
const opt = (n) => (flag(n) ? args[args.indexOf(n) + 1] : '')
const mode = flag('--run-go') ? 'run-go' : flag('--write') ? 'write' : flag('--check') ? 'check' : 'print'
const doGo = flag('--go') || !flag('--ui')
const doUi = flag('--ui') || !flag('--go')

function fail(msg) {
  console.error(msg)
  process.exit(1)
}

function env() {
  const e = { ...process.env }
  // GOTMPDIR нужен из-за антивируса, см. README
  e.GOTMPDIR ||= path.join(root, '.gotmp')
  fs.mkdirSync(e.GOTMPDIR, { recursive: true })
  return e
}

// Запуск с выводом прямо в файл (без буфера и перекодировки).
function runToFile(cmd, cmdArgs, cwd, file) {
  fs.mkdirSync(path.dirname(file), { recursive: true })
  const fd = fs.openSync(file, 'w')
  const r = spawnSync(cmd, cmdArgs, {
    cwd,
    env: env(),
    stdio: ['ignore', fd, 'pipe'],
    encoding: 'utf8',
    maxBuffer: 64 << 20,
  })
  fs.closeSync(fd)
  if (r.error) throw r.error
  return r
}

// pid в имени: параллельные прогоны (агенты в worktree с общим .gotmp) иначе перезаписывают файлы друг друга
const defaultFile = (name) => path.join(env().GOTMPDIR, `${process.pid}-${name}`)

// ---------- Go ----------

const modulePath = fs.readFileSync(path.join(root, 'go.mod'), 'utf8').match(/^module\s+(\S+)/m)[1]
const pkgName = (full) => (full === modulePath ? '.' : full.replace(modulePath + '/', ''))

// Разбор go test -json: имена тестов (с подтестами), покрытие, упавшие.
function parseGo(file) {
  const text = fs.readFileSync(file, 'utf8').replace(/^﻿/, '')
  const pkgs = {} // пакет -> {cover, names:{имя:кол-во}}
  const result = new Map() // пакет\0тест -> pass|fail|skip
  const outputs = new Map() // пакет\0тест -> строки вывода
  const failedPkgs = new Map() // пакет -> строки вывода пакета
  const get = (p) => (pkgs[p] ??= { cover: 0, names: {} })
  let bad = 0
  for (const line of text.split('\n')) {
    if (!line.trim()) continue
    let ev
    try {
      ev = JSON.parse(line)
    } catch {
      bad++
      continue
    }
    if (!ev.Package) continue
    const p = pkgName(ev.Package)
    const key = p + '\0' + (ev.Test ?? '')
    if (ev.Action === 'output') {
      const m = !ev.Test && /coverage:\s*([\d.]+)% of statements/.exec(ev.Output)
      if (m) get(p).cover = parseFloat(m[1])
      if (!outputs.has(key)) outputs.set(key, [])
      outputs.get(key).push(ev.Output)
    } else if (ev.Test && (ev.Action === 'pass' || ev.Action === 'fail' || ev.Action === 'skip')) {
      result.set(key, ev.Action)
    } else if (!ev.Test && ev.Action === 'fail') {
      failedPkgs.set(p, outputs.get(key) ?? [])
    }
  }
  const failedTests = []
  for (const [key, act] of result) {
    const [p, name] = key.split('\0')
    const g = get(p)
    g.names[name] = (g.names[name] ?? 0) + 1
    if (act === 'fail') failedTests.push({ pkg: p, name, out: outputs.get(key) ?? [] })
  }
  for (const g of Object.values(pkgs)) g.names = sortObj(g.names)
  return { pkgs, failedTests, failedPkgs, bad }
}

function sortObj(o) {
  return Object.fromEntries(Object.entries(o).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)))
}
const total = (names) => Object.values(names).reduce((s, n) => s + n, 0)

// Сводка для консоли: упавшие пакеты и тесты с хвостом вывода.
function summary(parsed, stderr) {
  const lines = []
  for (const [p, out] of parsed.failedPkgs) {
    lines.push(`FAIL package ${p}`)
    if (!parsed.failedTests.some((t) => t.pkg === p)) lines.push(...out.map((s) => s.trimEnd()).slice(-15))
  }
  for (const t of parsed.failedTests) {
    lines.push(`--- FAIL ${t.pkg}: ${t.name}`)
    lines.push(
      ...t.out
        .map((s) => s.trimEnd())
        .filter((s) => s && !/^=== /.test(s))
        .slice(-12),
    )
  }
  if (stderr?.trim()) lines.push(...stderr.trim().split('\n').slice(-15))
  return lines
}

function runGo() {
  const out = opt('--out') || defaultFile('go-test.json')
  const goArgs = ['test', '-cover', '-json', ...(flag('--race') ? ['-race'] : []), './...']
  const r = runToFile('go', goArgs, root, out)
  const parsed = parseGo(out)
  const n = Object.values(parsed.pkgs).reduce((s, g) => s + total(g.names), 0)
  if (r.status !== 0) {
    console.error(summary(parsed, r.stderr).join('\n') || `go test exit ${r.status}`)
    process.exit(r.status || 1)
  }
  console.log(`go test ok: ${Object.keys(parsed.pkgs).length} packages, ${n} tests`)
}

function goMeasure() {
  let file = opt('--go-json')
  if (!file) {
    file = defaultFile('go-test.json')
    runToFile('go', ['test', '-cover', '-json', './...'], root, file)
  }
  const parsed = parseGo(file)
  if (parsed.failedPkgs.size || parsed.failedTests.length) {
    fail(`go test failed - baseline not compared:\n${summary(parsed).join('\n')}`)
  }
  const cur = {}
  const treeNames = {}
  for (const p of Object.keys(parsed.pkgs).sort()) {
    const g = parsed.pkgs[p]
    if (!Object.keys(g.names).length) continue // пакет без тестов (корень) в эталон не идёт
    if (isTree(p)) {
      for (const [n, k] of Object.entries(g.names)) treeNames[n] = (treeNames[n] ?? 0) + k
      continue
    }
    cur[p] = { cover: g.cover, tests: total(g.names), names: g.names }
  }
  if (Object.keys(treeNames).length) {
    const c = treeCover()
    cur[TREE_KEY] = {
      cover: c.cover,
      tests: total(treeNames),
      names: sortObj(treeNames),
      stmts: c.stmts,
      covered: c.covered,
      missed: c.missed,
    }
  }
  return sortObj(cur)
}

// Покрытие дерева: все тесты дерева против всего его кода. Профиль сливаем по блокам «файл:диапазон»
// (блок покрыт, если покрыт в любом прогоне), операторы считаем по уникальным блокам.
function treeCover() {
  // На Windows go list падает целиком, если папки шаблона нет: берём только существующие корни
  const roots = TREE_ROOTS.filter((r) => fs.existsSync(path.join(root, r)))
  const list = spawnSync('go', ['list', ...roots.map((r) => `./${r}/...`)], { cwd: root, env: env(), encoding: 'utf8' })
  if (list.error || list.status !== 0) fail(`go list of album tree failed:\n${list.error ?? list.stderr}`)
  const pkgs = list.stdout
    .split('\n')
    .map((s) => s.trim())
    .filter(Boolean)
  if (!pkgs.length) fail(`go list of album tree failed:\n${list.stderr}`)
  // edtest и inside — помощники тестов (до 023 жили в _test.go и в покрытие не входили): в знаменатель не берём,
  // иначе ветки отказа помощников (t.Fatal) выглядят непокрытым кодом альбома
  const measured = pkgs.filter((p) => !/\/internal\/(edtest|inside)$/.test(p))
  const prof = defaultFile('tree-cover.out')
  const r = spawnSync('go', ['test', '-count=1', `-coverpkg=${measured.join(',')}`, `-coverprofile=${prof}`, ...pkgs], {
    cwd: root,
    env: env(),
    encoding: 'utf8',
    maxBuffer: 64 << 20,
  })
  if (r.status !== 0)
    fail(`go test -coverpkg for album tree failed:\n${((r.stdout || '') + (r.stderr || '')).split('\n').slice(-30).join('\n')}`)
  const blocks = new Map() // файл:диапазон -> {n, hit}
  const profText = fs.readFileSync(prof, 'utf8')
  fs.rmSync(prof, { force: true })
  for (const line of profText.split('\n')) {
    const m = /^(.+:\d+\.\d+,\d+\.\d+) (\d+) (\d+)$/.exec(line.trim())
    if (!m) continue
    const b = blocks.get(m[1]) ?? { n: +m[2], hit: false }
    b.hit ||= +m[3] > 0
    blocks.set(m[1], b)
  }
  let stmts = 0
  let covered = 0
  let missed = 0
  for (const b of blocks.values()) {
    stmts += b.n
    if (b.hit) covered += b.n
    else missed++
  }
  return {
    stmts,
    covered,
    missed,
    cover: stmts ? Math.round((covered / stmts) * 1000) / 10 : 0,
  }
}

// ---------- vitest ----------

// vitest run --reporter=json: assertionResults[].fullName (набор + имя; путь файла не входит - перенос не мешает).
function uiMeasure() {
  let file = opt('--ui-json')
  if (!file) {
    file = defaultFile('vitest.json')
    const fe = path.join(root, 'frontend')
    const bin = path.join(fe, 'node_modules', 'vitest', 'vitest.mjs')
    if (!fs.existsSync(bin)) fail('frontend/node_modules/vitest not found (npm ci in frontend)')
    const r = spawnSync(process.execPath, [bin, 'run', '--reporter=json', `--outputFile=${file}`], {
      cwd: fe,
      env: env(),
      encoding: 'utf8',
    })
    if (!fs.existsSync(file)) fail(`vitest run failed:\n${((r.stdout || '') + (r.stderr || '')).split('\n').slice(-30).join('\n')}`)
  }
  const j = JSON.parse(fs.readFileSync(file, 'utf8').replace(/^﻿/, ''))
  const names = {}
  let failed = 0
  for (const f of j.testResults) {
    for (const a of f.assertionResults) {
      if (a.status === 'failed') failed++
      if (a.status !== 'passed') continue
      names[a.fullName] = (names[a.fullName] ?? 0) + 1
    }
  }
  if (failed) fail(`vitest: ${failed} tests failed - baseline not compared`)
  return { tests: total(names), names: sortObj(names) }
}

// ---------- сравнение ----------

// Пропавшие имена. groups: {ключ: {names:{имя:кол-во}}}. Сначала сверяем внутри ключа (пакета);
// недостающее покрываем «лишними» тестами с тем же именем из любых других групп (перенос).
// Возвращает [{key, name, k}] — реально потерянные.
function lost(base, cur) {
  const pool = new Map()
  const missing = []
  for (const [key, b] of Object.entries(base)) {
    for (const [n, k] of Object.entries(b.names)) {
      const c = cur[key]?.names[n] ?? 0
      if (c < k) missing.push({ key, name: n, k: k - c })
    }
  }
  for (const [key, c] of Object.entries(cur)) {
    for (const [n, k] of Object.entries(c.names)) {
      const extra = k - (base[key]?.names[n] ?? 0)
      if (extra > 0) pool.set(n, (pool.get(n) ?? 0) + extra)
    }
  }
  const res = []
  for (const m of missing) {
    const take = Math.min(m.k, pool.get(m.name) ?? 0)
    if (take) pool.set(m.name, pool.get(m.name) - take)
    if (m.k > take) res.push({ ...m, k: m.k - take })
  }
  return res
}

function check(base, cur) {
  const errors = []
  const notes = []
  if (cur.go) {
    if (!base.go) fail('baseline has no go section - run --write')
    for (const { key, name, k } of lost(base.go, cur.go)) errors.push(`go ${key}: test lost${k > 1 ? ` x${k}` : ''}: ${name}`)
    for (const [p, b] of Object.entries(base.go)) {
      const c = cur.go[p]
      if (!c) {
        notes.push(`go ${p}: package gone, its tests moved elsewhere`)
        continue
      }
      if (b.cover - c.cover > MAX_DROP + 1e-9) errors.push(`go ${p}: coverage fell ${b.cover}% -> ${c.cover}% (max drop ${MAX_DROP})`)
      if (p === TREE_KEY) {
        // Критерий 023 (решение владельца 10): дерево не ниже TREE_MIN без допуска MAX_DROP и непокрытых блоков
        // не больше эталона (до первого --write — TREE_MAX_MISSED)
        if (c.cover < TREE_MIN - 1e-9) errors.push(`go ${p}: coverage ${c.cover}% below ${TREE_MIN}% (023 criterion)`)
        const maxMissed = b.missed ?? TREE_MAX_MISSED
        if (c.missed > maxMissed) errors.push(`go ${p}: uncovered blocks ${c.missed} > ${maxMissed}`)
        notes.push(`go ${p}: ${c.covered}/${c.stmts} statements = ${c.cover}%, uncovered blocks ${c.missed} (baseline ${b.cover}%)`)
      }
      if (c.tests > b.tests) notes.push(`go ${p}: tests ${b.tests} -> ${c.tests}`)
      else if (c.cover > b.cover + 1e-9) notes.push(`go ${p}: coverage ${b.cover}% -> ${c.cover}%`)
    }
    for (const p of Object.keys(cur.go)) if (!base.go[p]) notes.push(`go ${p}: new package`)
  }
  if (cur.ui) {
    if (!base.ui) fail('baseline has no ui section - run --write')
    for (const { name, k } of lost({ ui: base.ui }, { ui: cur.ui })) errors.push(`ui test lost${k > 1 ? ` x${k}` : ''}: ${name}`)
    if (cur.ui.tests > base.ui.tests) notes.push(`ui: tests ${base.ui.tests} -> ${cur.ui.tests}`)
  }
  return { errors, notes }
}

if (mode === 'run-go') {
  runGo()
  process.exit(0)
}

const cur = {}
if (doGo) cur.go = goMeasure()
if (doUi) cur.ui = uiMeasure()
const brief = () =>
  [
    cur.go && `go ${Object.values(cur.go).reduce((s, p) => s + p.tests, 0)} tests in ${Object.keys(cur.go).length} packages`,
    cur.ui && `ui ${cur.ui.tests} tests`,
  ]
    .filter(Boolean)
    .join(', ')

if (mode === 'print') {
  console.log(JSON.stringify(cur, null, 1))
} else if (mode === 'write') {
  // Обновляем только измеренную часть, вторую оставляем как была.
  const old = fs.existsSync(target) ? JSON.parse(fs.readFileSync(target, 'utf8')) : {}
  fs.mkdirSync(path.dirname(target), { recursive: true })
  fs.writeFileSync(target, JSON.stringify({ ...old, ...cur }, null, 1) + '\n')
  console.log(`test-baseline written: ${brief()}`)
} else {
  if (!fs.existsSync(target)) fail('testdata/test-baseline.json not found - run: node scripts/test-baseline.mjs --write')
  const { errors, notes } = check(JSON.parse(fs.readFileSync(target, 'utf8')), cur)
  if (errors.length) {
    console.error('test baseline broken (tests must not weaken; update only via --write with owner approval):')
    for (const e of errors) console.error('  ' + e)
    process.exit(1)
  }
  console.log(`test baseline ok: ${brief()}`)
  for (const n of notes) console.log('  ' + n)
  if (notes.length) console.log('  baseline can be raised: node scripts/test-baseline.mjs --write')
}
