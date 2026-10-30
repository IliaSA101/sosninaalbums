// Быстрая предпроверка для исполнителей перед отчётом: `node scripts/precheck.mjs [--base <rev>]`.
// Ловит то, что иначе падает только на полном check.ps1 у оркестратора: gofmt, vet по всему модулю, golangci-lint
// (только новое), go test -short изменённых пакетов, покрытие дерева album (при правке internal/album/**) против эталона,
// prettier, eslint, svelte-check, vitest related + стражи (тесты со сканом дерева), язык комментариев, размер файлов, лимит файлов в папке. Полный go test и test-baseline
// остаются шагом оркестратора (check.ps1 без -short: эталоны и большие наборы там).
// go test -short — с кэшем go (теги как в check.sh); Go и фронт идут параллельно (фронт — дочерний процесс --part fe).
// Пределы размера и папок — scripts/limits.mjs (один источник с check.ps1, pre-commit и хуком file-size).
import { spawn, spawnSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { GO_BASELINE, GO_KEY, PRECHECK_SLACK, TREE_KEY, compareTree, isTree, treeCover } from './tree-cover.mjs'
import { withGoCaches } from './goenv.mjs'
import { checkFolders, checkSizes } from './limits.mjs'
import { collect as collectAdded, violations as langViolations } from './comment-lang.mjs'
import { NO_DESKTOP, affectedScriptTests } from './script-tests.mjs'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
process.chdir(root)
const gotmp = path.join(root, '.gotmp') // антивирус: exe go только сюда (как в check.ps1)
fs.mkdirSync(gotmp, { recursive: true })
const env = withGoCaches({ ...process.env, GOTMPDIR: gotmp, SA_TEST_SHORT: '1' }, root) // SA_TEST_SHORT: цепочки состояний по 50 шагов
// Теги сборки, как в check.sh (scripts/env.sh): иначе ключ кэша go test другой и результаты precheck и check.sh не делятся.
// Для internal/... тег ничего не меняет, но ключ у go test включает его.
if (process.platform === 'linux' && !(env.GOFLAGS || '').includes('webkit2_41')) {
  env.GOFLAGS = `${env.GOFLAGS ? `${env.GOFLAGS} ` : ''}-tags=webkit2_41`
}

// Пакеты модуля: не ./... — тот заходит в frontend/node_modules (flatted/golang). Один шаблон для всех скриптов и CI.
const MODULE_PKGS = ['.', './internal/...']

// Пакеты, чьи тесты читают файлы вне модуля (.testphotos/stock и docs/refs/fsfera-psd ищутся вверх от папки теста,
// в worktree они в основном дереве): go кладёт в ключ кэша только файлы модуля, поэтому всегда -count=1.
const NO_CACHE = ['./internal/library/imgsig', './internal/library/psd']

const argv = process.argv.slice(2)
const bi = argv.indexOf('--base')
// --part fe — только фронт: так precheck запускает сам себя дочерним процессом, чтобы Go и фронт шли параллельно
const part = argv[argv.indexOf('--part') + 1] === 'fe' ? 'fe' : ''

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

// Файлы фронта (пути от frontend/)
const fe = changed
  .filter((f) => /^frontend\//.test(f) && !/^frontend\/(wailsjs|dist)\//.test(f) && !/package(-lock)?\.json$/.test(f))
  .filter((f) => /\.(ts|js|svelte|css|html|json)$/.test(f))
  .map((f) => f.slice('frontend/'.length))
// Эталоны Go, которые импортируют тесты фронта (каталог клавиш и пр.): правка только Go-стороны ломала стражи фронта
// незаметно (138 Г1: команда каталога без обработчика), поэтому такие файлы гонят vitest related и стражи. Пути от frontend/.
const feShared = changed.filter((f) => /^testdata\/parity\/.+\.json$/.test(f)).map((f) => `../${f}`)

const failed = []
const total = Date.now()
const step = (name, fn) => {
  const t = Date.now()
  const r = fn()
  const time = `${((Date.now() - t) / 1000).toFixed(1)}s`
  if (r.ok && r.warn) {
    console.log(`WARN ${name} (${time})`)
    r.warn
      .split('\n')
      .filter((l) => l.trim())
      .slice(-30)
      .forEach((l) => console.log(`     ${l}`))
  } else if (r.ok) console.log(`OK   ${name} (${time})`)
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
// 146 (да владельца 2026-10-29): модуль контракта api/albums — свой go.mod. Его пакеты корневой go test
// не находит («main module does not contain package»), поэтому они идут отдельными шагами из папки модуля.
const API_MOD = 'api/albums'
const allGo = changed.filter((f) => f.endsWith('.go'))
const apiGoFiles = allGo.filter((f) => f.startsWith(`${API_MOD}/`))
const goFiles = allGo.filter((f) => !f.startsWith(`${API_MOD}/`))
// golangci-lint ставится через go install — может не быть в PATH
const gclExe = () => {
  const gcl = spawnSync(process.platform === 'win32' ? 'where' : 'which', ['golangci-lint'], { encoding: 'utf8' })
  if (gcl.status === 0) return gcl.stdout.split('\n')[0].trim()
  const gp = run('go', ['env', 'GOPATH']).out.trim()
  return path.join(gp, 'bin', process.platform === 'win32' ? 'golangci-lint.exe' : 'golangci-lint')
}
// Go и фронт не зависят друг от друга: фронт идёт дочерним процессом, пока Go занят (вывод — в файл, а не в pipe)
let feJob = null
if (part === '' && goFiles.length && (fe.length || feShared.length)) {
  const log = path.join(gotmp, `precheck-fe-${process.pid}.log`)
  const fd = fs.openSync(log, 'w')
  const child = spawn(process.execPath, [fileURLToPath(import.meta.url), '--part', 'fe', '--base', base], {
    cwd: root,
    env: process.env,
    stdio: ['ignore', fd, fd],
  })
  fs.closeSync(fd)
  feJob = { log, done: new Promise((resolve) => child.on('close', (code) => resolve(code ?? 1))) }
}
if (part !== 'fe' && goFiles.length) {
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
    // Без -count=1: нетронутые пакеты берутся из кэша go test (повторный precheck не гоняет их заново).
    // Исключение — NO_CACHE: их тесты читают файлы вне модуля, кэш вернул бы устаревший результат.
    const fresh = pkgs.filter((p) => NO_CACHE.includes(p))
    const cached = pkgs.filter((p) => !NO_CACHE.includes(p))
    // Больше трёх пакетов — через очередь: go test занимает все ядра
    const exec = (args, n) => (n > 3 ? run('node', ['scripts/heavy.mjs', 'go', ...args]) : run('go', args))
    const parts = []
    if (cached.length) parts.push(exec(['test', '-short', ...cached], cached.length))
    if (fresh.length) parts.push(exec(['test', '-short', '-count=1', ...fresh], fresh.length))
    return res({ code: parts.find((r) => r.code !== 0)?.code ?? 0, out: parts.map((r) => r.out).join('\n') })
  })
  // Стражи Go: тесты internal/testkit, читающие всё дерево (t.TempDir, своя ссылка на бинарь), — go test -short
  // изменённых пакетов их не берёт, а ломаются они от любого *_test.go (25.10: три t.TempDir прошли precheck
  // трёх агентов и упали только в check --go; «да» владельца 25.10)
  if (built.some((f) => f.endsWith('_test.go'))) {
    step('стражи Go', () => res(run('go', ['test', '-count=1', '-run', '^TestNo', './internal/testkit'])))
  }
  // Стражи album, сканирующие всё internal/ (TestPageWReaders, TestNoExportedGlobals): -short берёт album,
  // только если правили album (29.10: чтение PageW в app/agents прошло precheck и упало в CI)
  if (goFiles.some((f) => f.startsWith('internal/') && !f.startsWith('internal/album/'))) {
    step('стражи album', () =>
      res(run('go', ['test', '-count=1', '-run', '^(TestPageWReaders|TestNoExportedGlobals)$', './internal/album'])),
    )
  }
  if (goFiles.some((f) => isTree(path.posix.dirname(f)))) {
    step('покрытие дерева album', () => {
      const baseJson = path.join(root, 'testdata', GO_BASELINE)
      const entry = fs.existsSync(baseJson) ? JSON.parse(fs.readFileSync(baseJson, 'utf8'))[GO_KEY]?.[TREE_KEY] : null
      if (!entry) return { ok: false, out: `в testdata/${GO_BASELINE} нет записи ${GO_KEY}.${TREE_KEY}` }
      let cur
      try {
        cur = treeCover({ root, env, heavy: true })
      } catch (e) {
        return { ok: false, out: String(e.message) }
      }
      const errors = compareTree(entry, cur)
      if (errors.length === 0) return { ok: true, out: '' }
      // Небольшой спад — допуск быстрого уровня (владелец 2026-10-21): предупреждение, строго проверяют check.ps1 и CI
      if (compareTree(entry, cur, PRECHECK_SLACK).length === 0) {
        const hint = 'в допуске precheck; check.ps1 и CI проверяют строго — допиши тест к своему коду'
        return { ok: true, out: '', warn: [...errors, hint].join('\n') }
      }
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
    const exe = gclExe()
    const heavy = (...args) => run('node', ['scripts/heavy.mjs', exe, 'run', '--allow-parallel-runners', ...args, ...pkgs])
    const fresh = heavy(`--new-from-rev=${base}`)
    // Линтеры «на функцию» (funlen, gocognit) ставят замечание на заголовок: выросло тело старой функции —
    // --new-from-rev его отфильтрует, поэтому второй прогон без фильтра по тем же пакетам
    const perFunc = heavy('--enable-only', 'gocognit,funlen')
    // Да владельца 29.10 (очередь п. 6): файлы //go:build windows на Linux и macOS не линтятся — промах (unused
    // у метода, звавшегося только из файла linux) доходил до CI на Windows; второй прогон — под GOOS=windows
    const win =
      process.platform === 'win32'
        ? { code: 0, out: '' }
        : run('node', ['scripts/heavy.mjs', exe, 'run', '--allow-parallel-runners', `--new-from-rev=${base}`, ...pkgs], {
            env: { ...env, GOOS: 'windows' },
          })
    const outs = [fresh.out, perFunc.out, win.code ? `GOOS=windows:\n${win.out}` : '']
    return res({ code: fresh.code || perFunc.code || win.code, out: outs.filter((o) => o.trim()).join('\n') })
  })
}
// Модуль api/albums целиком (он мал): конфиг линтера — корневой, golangci-lint ищет его вверх от папки
if (part !== 'fe' && apiGoFiles.length && fs.existsSync(path.join(root, API_MOD, 'go.mod'))) {
  const cwd = path.join(root, API_MOD)
  const heavy = path.join(root, 'scripts', 'heavy.mjs')
  step('gofmt api/albums', () => res(run('gofmt', ['-l', ...apiGoFiles]), true))
  step('go vet api/albums', () => res(run('go', ['vet', './...'], { cwd })))
  step('go test -short api/albums', () => res(run('go', ['test', '-short', './...'], { cwd })))
  step('golangci-lint api/albums', () =>
    res(run('node', [heavy, gclExe(), 'run', '--allow-parallel-runners', './...'], { cwd })),
  )
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
if ((fe.length || feShared.length) && (part === 'fe' || !feJob)) {
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
  if (fe.length) step('prettier', () => res(npx(fePart(['prettier', '--check', '--log-level', 'warn'], fe))))
  const lint = fe.filter((f) => /\.(ts|js|svelte)$/.test(f))
  if (lint.length) step('eslint', () => res(npx(fePart(['eslint', '--max-warnings', '0'], lint))))
  if (fe.some((f) => /\.(ts|svelte)$/.test(f))) {
    step('svelte-check', () => res(npx([['svelte-check', '--output', 'human', '--fail-on-warnings']])))
  }
  const src = [...fe.filter((f) => /^src\/.*\.(ts|js|svelte)$/.test(f)), ...feShared]
  if (src.length) step('vitest related', () => res(npx(fePart(['vitest', 'related', '--run', '--passWithNoTests'], src))))
  // Стражи: тесты-сканеры всего дерева фронта (import.meta.glob) — vitest related их не выбирает, а ломаются они от
  // любой правки src. Список вычисляем по содержимому, не храним.
  if (src.length) {
    const guards = scanGuards(path.join(root, 'frontend', 'src'))
    if (guards.length) step(`стражи фронта (${guards.length})`, () => res(npx([['vitest', 'run', ...guards]])))
  }
}

// Дочерний процесс фронта (--part fe): остальное делает родитель, итог читает он же
if (part === 'fe') {
  console.log(failed.length ? `ИТОГО: FAIL - ${failed.join(', ')}` : 'ИТОГО: OK')
  process.exit(failed.length ? 1 : 0)
}

// ---- Python: скрипты с юнит-тестами (e2e 089, win 039, demo 102) ----
// Юнит-тесты чистых частей каталогов, где правился .py. Интерпретатор — python3 (Ubuntu, macOS), затем python (Windows);
// нет ни того ни другого — WARN, а не молчаливый пропуск (на Ubuntu `python` нет, тесты годами не запускались).
const PY_DIRS = ['scripts/e2e', 'scripts/win', 'scripts/demo']
// Тесты скриптов по затронутым файлам: scripts/win — и при правке любого файла папки или workflow (0.25.0: правка одного yml ломала тест)
const scriptTests = affectedScriptTests(changed, (f) => fs.existsSync(path.join(root, f)))
const pyTouched = PY_DIRS.filter(
  (d) =>
    fs.existsSync(path.join(root, d)) &&
    ((d === 'scripts/win' && scriptTests.win) || changed.some((f) => f.startsWith(`${d}/`) && f.endsWith('.py'))),
)
if (pyTouched.length) {
  const py = ['python3', 'python'].find((c) => spawnSync(c, ['--version'], { encoding: 'utf8' }).status === 0)
  if (py) {
    for (const d of pyTouched) step(`python unittest (${d})`, () => res(run(py, ['-I', '-m', 'unittest', 'discover', '-s', d])))
  } else {
    step('python unittest', () => ({ ok: true, warn: `нет python3 и python в PATH: тесты ${pyTouched.join(', ')} не запущены` }))
  }
}

// ---- Скрипты: node --test и test_deb.sh по затронутым файлам ----
if (scriptTests.node.length) step('node --test скриптов', () => res(run('node', ['--test', ...scriptTests.node])))
if (scriptTests.deb) {
  const hasDpkg = process.platform !== 'win32' && spawnSync('dpkg-deb', ['--version'], { encoding: 'utf8' }).status === 0
  if (hasDpkg) {
    // Без рабочего стола, как на раннере CI: xdg-mime при DISPLAY смотрит не туда (0.25.0)
    const clean = { ...env }
    for (const k of NO_DESKTOP) delete clean[k]
    step('test_deb.sh', () => res(run('bash', ['scripts/linux/test_deb.sh'], { env: clean })))
  } else step('test_deb.sh', () => ({ ok: true, warn: 'нет dpkg-deb (или Windows): scripts/linux/test_deb.sh не запущен' }))
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

// Фронт, шедший параллельно с Go: его вывод — блоком после шагов Go
if (feJob) {
  const code = await feJob.done
  const text = fs.readFileSync(feJob.log, 'utf8')
  fs.rmSync(feJob.log, { force: true })
  console.log('--- фронт (шёл параллельно с Go) ---')
  for (const l of text.split('\n')) if (l.trim() && !l.startsWith('ИТОГО')) console.log(l)
  if (code !== 0) {
    const m = /^ИТОГО: FAIL - (.*)$/m.exec(text)
    failed.push(...(m ? m[1].split(', ') : ['фронт']))
  }
}

const time = `${((Date.now() - total) / 1000).toFixed(1)}s`
if (failed.length) {
  console.log(`ИТОГО: FAIL - ${failed.join(', ')} (${time})`)
  process.exit(1)
}
console.log(`ИТОГО: OK (${time})`)
