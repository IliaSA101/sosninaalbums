// Приём ветки агента: временный репозиторий в .gotmp (правило проекта: временные файлы — не в ОС-темпе), precheck подменён.
import { test, before, after } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { Stop, accept, ensureNodeModules, mergeBothSides } from './accept.mjs'
import { appendEntry, journalFile, readJournal } from './accept-log.mjs'

const here = path.dirname(fileURLToPath(import.meta.url))
const gotmp = path.join(here, '..', '.gotmp')
let tmp

// Конфигурация git из окружения (GIT_CONFIG_*) сильнее конфигурации репозитория: подменяем явно, чтобы
// тест проверял логику, а не автоматический перевод строк машины (core.autocrlf=true на раннере Windows).
Object.assign(process.env, { GIT_CONFIG_COUNT: '2', GIT_CONFIG_KEY_0: 'core.autocrlf', GIT_CONFIG_VALUE_0: 'false', GIT_CONFIG_KEY_1: 'core.eol', GIT_CONFIG_VALUE_1: 'lf' })

before(() => {
  fs.mkdirSync(gotmp, { recursive: true })
  tmp = fs.mkdtempSync(path.join(gotmp, 'accept-test-'))
})
after(() => fs.rmSync(tmp, { recursive: true, force: true }))

const git = (cwd, ...a) => execFileSync('git', a, { cwd, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim()
const write = (root, f, s) => {
  fs.mkdirSync(path.dirname(path.join(root, f)), { recursive: true })
  fs.writeFileSync(path.join(root, f), s)
}
const commit = (root, f, s, msg) => {
  write(root, f, s)
  git(root, 'add', '-A')
  git(root, 'commit', '-q', '-m', msg)
}

const DEC = 'docs/specs/0.27/decisions.md'

/** Репозиторий с dev, заглушкой precheck и ссылкой frontend/node_modules; wt(имя) заводит копию-ветку. */
let seq = 0
function setup() {
  const dir = path.join(tmp, `r${++seq}`)
  const root = path.join(dir, 'main')
  fs.mkdirSync(root, { recursive: true })
  git(root, 'init', '-q', '-b', 'dev')
  // не зависеть от настроек машины: на Windows-раннере core.autocrlf=true выписывает CRLF
  git(root, 'config', 'core.autocrlf', 'false')
  git(root, 'config', 'core.eol', 'lf')
  git(root, 'config', 'user.name', 't')
  git(root, 'config', 'user.email', 't@example.com')
  git(root, 'config', 'commit.gpgsign', 'false')
  write(root, '.gitignore', 'node_modules\n.gotmp/\n.claude/\n')
  commit(root, DEC, '# решения\n\nстарое\n', 'init')
  commit(root, 'a.txt', 'a\n', 'a')
  // заглушка precheck: пишет аргументы и cwd; провал — по ACCEPT_TEST_FAIL
  const stub = path.join(dir, 'precheck-stub.mjs')
  fs.writeFileSync(
    stub,
    `import fs from 'node:fs'
fs.writeFileSync(${JSON.stringify(path.join(dir, 'precheck.log'))}, JSON.stringify({ args: process.argv.slice(2), cwd: process.cwd() }))
if (process.env.ACCEPT_TEST_FAIL) { console.log('FAIL: заглушка'); process.exit(1) }
`,
  )
  const nm = path.join(dir, 'real-node-modules')
  fs.mkdirSync(nm)
  fs.writeFileSync(path.join(nm, 'keep.txt'), 'x')
  const env = { ...process.env, ACCEPT_PRECHECK: stub }
  delete env.ACCEPT_TEST_FAIL
  const wt = (name, link = true) => {
    const p = path.join(root, '.claude', 'worktrees', name)
    git(root, 'worktree', 'add', '-q', '-b', name, p, 'dev')
    fs.mkdirSync(path.join(p, 'frontend'), { recursive: true })
    if (link) fs.symlinkSync(nm, path.join(p, 'frontend', 'node_modules'), 'junction')
    return p
  }
  // основная копия: frontend/node_modules — цель ссылки, которую accept ставит сам
  fs.mkdirSync(path.join(root, 'frontend'), { recursive: true })
  fs.symlinkSync(nm, path.join(root, 'frontend', 'node_modules'), 'junction')
  return { root, dir, nm, env, wt, log: () => JSON.parse(fs.readFileSync(path.join(dir, 'precheck.log'), 'utf8')) }
}

const stopMsg = (fn) => {
  try {
    fn()
  } catch (e) {
    assert.ok(e instanceof Stop, `не Stop: ${e.stack}`)
    return e.message
  }
  assert.fail('ожидался Stop')
}

test('mergeBothSides: dev, пустая строка, ветка; без маркеров — null', () => {
  const src = 'a\n<<<<<<< HEAD\ndev1\ndev2\n=======\nbr1\n>>>>>>> x\nb\n<<<<<<< HEAD\nd\n||||||| base\nb\n=======\nB\n>>>>>>> y\n'
  assert.equal(mergeBothSides(src), 'a\ndev1\ndev2\n\nbr1\nb\nd\n\nB\n')
  assert.equal(mergeBothSides('просто\nтекст\n'), null)
  assert.equal(mergeBothSides('<<<<<<< HEAD\nd\n=======\nb\n'), null)
  assert.equal(mergeBothSides('<<<<<<< HEAD\r\nd\r\n=======\r\nb\r\n>>>>>>> x\r\n'), 'd\r\n\r\nb\r\n')
  // сторона пуста: пустая строка не добавляется
  assert.equal(mergeBothSides('<<<<<<< HEAD\n=======\nb\n>>>>>>> x\n'), 'b\n')
})

test('чистый ff: rebase на ушедший вперёд dev, precheck по базе dev, worktree и ветка удалены, цель ссылки цела', () => {
  const s = setup()
  const w = s.wt('b1')
  commit(w, 'b1.txt', '1\n', 'b1 первый')
  commit(w, 'b2.txt', '2\n', 'b1 второй')
  commit(s.root, 'dev.txt', 'd\n', 'dev ушёл вперёд')
  const devBefore = git(s.root, 'rev-parse', 'dev')
  const wReal = fs.realpathSync(w)
  const line = accept('b1', 'b1', { cwd: s.root, env: s.env })
  assert.match(line, /^принято b1: 2 коммита, [0-9a-f]+ b1 второй$/)
  assert.equal(git(s.root, 'log', '--format=%s', '-3'), 'b1 второй\nb1 первый\ndev ушёл вперёд')
  assert.equal(git(s.root, 'rev-parse', 'HEAD~2'), devBefore)
  assert.ok(!fs.existsSync(w), 'worktree удалён')
  assert.equal(git(s.root, 'branch', '--list', 'b1'), '')
  assert.ok(fs.existsSync(path.join(s.nm, 'keep.txt')), 'цель ссылки node_modules цела')
  const log = s.log()
  assert.deepEqual(log.args, ['--base', devBefore])
  assert.equal(log.cwd, wReal)
})

test('нет frontend/node_modules в копии → ссылка на основную копию создана до precheck и снята при приёме', () => {
  const s = setup()
  const w = s.wt('b10', false)
  commit(w, 'b.txt', 'b\n', 'b10')
  // precheck-заглушка фиксирует, видна ли ссылка на момент запуска
  const seen = path.join(s.dir, 'seen.txt')
  const stub = path.join(s.dir, 'precheck-seen.mjs')
  fs.writeFileSync(
    stub,
    `import fs from 'node:fs'
const l = fs.lstatSync('frontend/node_modules', { throwIfNoEntry: false })
fs.writeFileSync(${JSON.stringify(seen)}, String(!!l?.isSymbolicLink()) + ' ' + fs.realpathSync('frontend/node_modules'))
`,
  )
  const line = accept('b10', 'b10', { cwd: s.root, env: { ...s.env, ACCEPT_PRECHECK: stub } })
  assert.match(line, /^принято b10:/)
  assert.equal(fs.readFileSync(seen, 'utf8'), `true ${fs.realpathSync(s.nm)}`)
  assert.ok(!fs.existsSync(w), 'worktree удалён вместе со ссылкой')
  assert.ok(fs.existsSync(path.join(s.nm, 'keep.txt')), 'цель ссылки цела')
})

test('ensureNodeModules: ссылка есть — не трогает; нет в основной копии или нет frontend/ — пропуск', () => {
  const s = setup()
  const w = s.wt('b11')
  assert.equal(ensureNodeModules(s.root, w), false, 'ссылка уже есть')
  const w2 = s.wt('b12', false)
  fs.rmSync(path.join(w2, 'frontend'), { recursive: true })
  assert.equal(ensureNodeModules(s.root, w2), false, 'нет frontend/')
  fs.mkdirSync(path.join(w2, 'frontend'))
  fs.rmSync(path.join(s.root, 'frontend', 'node_modules'), { recursive: true, force: true }) // ссылка: цель s.nm не трогаем
  assert.equal(ensureNodeModules(s.root, w2), false, 'нет цели в основной копии')
  assert.ok(!fs.existsSync(path.join(w2, 'frontend', 'node_modules')))
})

test('worktree можно задать полным путём; 1 коммит — единственное число', () => {
  const s = setup()
  const w = s.wt('b1')
  commit(w, 'b1.txt', '1\n', 'один')
  assert.match(accept(w, 'b1', { cwd: s.root, env: s.env }), /^принято b1: 1 коммит, /)
})

test('конфликт только в decisions.md сводится: сначала dev, затем ветка', () => {
  const s = setup()
  const w = s.wt('b2')
  commit(w, DEC, '# решения\n\nветка\n', 'решение ветки')
  commit(w, 'b.txt', 'b\n', 'после')
  commit(s.root, DEC, '# решения\n\ndev\n', 'решение dev')
  const line = accept('b2', 'b2', { cwd: s.root, env: s.env })
  assert.match(line, /^принято b2: 2 коммита, /)
  const text = fs.readFileSync(path.join(s.root, DEC), 'utf8')
  assert.equal(text, '# решения\n\ndev\n\nветка\n')
  assert.ok(!/[<>=]{7}/.test(text))
  assert.equal(git(s.root, 'status', '--porcelain', '--untracked-files=no'), '')
})

test('конфликт в другом файле — стоп, rebase отменён, копия и dev целы', () => {
  const s = setup()
  const w = s.wt('b3')
  commit(w, 'a.txt', 'ветка\n', 'правка a')
  const head = git(w, 'rev-parse', 'HEAD')
  commit(s.root, 'a.txt', 'dev\n', 'правка a в dev')
  commit(s.root, DEC, '# решения\n\ndev\n', 'dev решение')
  const devHead = git(s.root, 'rev-parse', 'dev')
  const msg = stopMsg(() => accept('b3', 'b3', { cwd: s.root, env: s.env }))
  assert.match(msg, /a\.txt/)
  assert.ok(fs.existsSync(w))
  assert.equal(git(w, 'rev-parse', 'HEAD'), head)
  assert.equal(git(w, 'rev-parse', '--abbrev-ref', 'HEAD'), 'b3')
  assert.equal(git(w, 'status', '--porcelain'), '')
  assert.ok(!fs.existsSync(path.join(s.root, '.git', 'worktrees', 'b3', 'rebase-merge')), 'rebase не остался висеть')
  assert.equal(git(s.root, 'rev-parse', 'dev'), devHead)
  assert.ok(fs.existsSync(path.join(s.nm, 'keep.txt')))
})

test('конфликт и в decisions.md, и в другом файле — стоп с перечнем только чужих', () => {
  const s = setup()
  const w = s.wt('b4')
  commit(w, 'a.txt', 'ветка\n', 'a')
  commit(w, DEC, '# решения\n\nветка\n', 'dec')
  commit(s.root, 'a.txt', 'dev\n', 'a dev')
  const msg = stopMsg(() => accept('b4', 'b4', { cwd: s.root, env: s.env }))
  assert.match(msg, /a\.txt/)
  assert.ok(!msg.includes('decisions.md'))
  assert.equal(git(w, 'status', '--porcelain'), '')
})

test('грязная копия (изменённый и новый файл) — стоп до rebase', () => {
  const s = setup()
  const w = s.wt('b5')
  commit(w, 'b.txt', 'b\n', 'b')
  write(w, 'b.txt', 'правка не закоммичена\n')
  assert.match(stopMsg(() => accept('b5', 'b5', { cwd: s.root, env: s.env })), /копия: есть незакоммиченное/)
  git(w, 'checkout', '--', 'b.txt')
  write(w, 'новый.txt', 'x\n')
  assert.match(stopMsg(() => accept('b5', 'b5', { cwd: s.root, env: s.env })), /копия: есть незакоммиченное/)
  assert.ok(fs.existsSync(w))
  assert.ok(!fs.existsSync(path.join(s.dir, 'precheck.log')), 'precheck не запускался')
})

test('основная копия: не на dev или с правкой отслеживаемого файла — стоп; неотслеживаемое не мешает', () => {
  const s = setup()
  const w = s.wt('b6')
  commit(w, 'b.txt', 'b\n', 'b')
  write(s.root, 'a.txt', 'грязно\n')
  assert.match(stopMsg(() => accept('b6', 'b6', { cwd: s.root, env: s.env })), /основная копия: есть незакоммиченное/)
  git(s.root, 'checkout', '--', 'a.txt')
  git(s.root, 'switch', '-q', '-c', 'other')
  assert.match(stopMsg(() => accept('b6', 'b6', { cwd: s.root, env: s.env })), /не на dev/)
  git(s.root, 'switch', '-q', 'dev')
  write(s.root, 'untracked.txt', 'x\n')
  assert.match(accept('b6', 'b6', { cwd: s.root, env: s.env }), /^принято b6: 1 коммит, /)
})

test('precheck провален — ветка не влита, копия и ссылка целы, в сообщении хвост вывода', () => {
  const s = setup()
  const w = s.wt('b7')
  commit(w, 'b.txt', 'b\n', 'b')
  const devHead = git(s.root, 'rev-parse', 'dev')
  const msg = stopMsg(() => accept('b7', 'b7', { cwd: s.root, env: { ...s.env, ACCEPT_TEST_FAIL: '1' } }))
  assert.match(msg, /precheck провален/)
  assert.match(msg, /FAIL: заглушка/)
  assert.equal(git(s.root, 'rev-parse', 'dev'), devHead)
  assert.ok(fs.existsSync(w))
  assert.ok(fs.lstatSync(path.join(w, 'frontend', 'node_modules')).isSymbolicLink())
  assert.equal(git(s.root, 'branch', '--list', 'b7').includes('b7'), true)
})

test('неверные аргументы: нет ветки, нет worktree, ветка не совпадает, dev', () => {
  const s = setup()
  const w = s.wt('b8')
  assert.match(stopMsg(() => accept('', '', { cwd: s.root, env: s.env })), /использование/)
  assert.match(stopMsg(() => accept('нет-такого', 'x', { cwd: s.root, env: s.env })), /не найден/)
  assert.match(stopMsg(() => accept('b8', 'другая', { cwd: s.root, env: s.env })), /ветка «b8», а не «другая»/)
  assert.match(stopMsg(() => accept('b8', 'dev', { cwd: s.root, env: s.env })), /dev не принимается/)
  assert.ok(fs.existsSync(w))
})

test('CLI: итог одной строкой, код 1 при СТОП', () => {
  const s = setup()
  const w = s.wt('b9')
  commit(w, 'b.txt', 'b\n', 'тема')
  const script = path.join(here, 'accept.mjs')
  const out = execFileSync(process.execPath, [script, 'b9', 'b9'], { cwd: s.root, env: s.env, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] })
  assert.match(out, /^принято b9: 1 коммит, [0-9a-f]+ тема\n$/)
  let code = 0
  let stdout = ''
  try {
    execFileSync(process.execPath, [script, 'b9', 'b9'], { cwd: s.root, env: s.env, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] })
  } catch (e) {
    code = e.status
    stdout = e.stdout
  }
  assert.equal(code, 1)
  assert.match(stdout, /^СТОП: worktree не найден/)
})

// Журнал приёмов (С1, 0.35): <git-common-dir>/sa-accept.jsonl, счёт «приёмов с Go после последнего winqa».
test('журнал: приём с Go и без — записи, пакеты, счёт растёт; winqa сбрасывает, rc-winqa нет; вне дерева', () => {
  const s = setup()
  const file = journalFile(s.root)
  assert.equal(path.resolve(file), path.resolve(s.root, '.git', 'sa-accept.jsonl'))
  const msgs = []
  const take = (name, files) => {
    const w = s.wt(name)
    for (const [f, t] of files) commit(w, f, t, name)
    msgs.length = 0
    accept(name, name, { cwd: s.root, env: s.env, log: (m) => msgs.push(m) })
    return msgs.find((m) => m.startsWith('приёмов с Go')) ?? msgs.join('|')
  }
  assert.equal(take('g1', [['internal/layout/x.go', 'package layout\n'], ['internal/app/edit/y.go', 'package edit\n'], ['doc.md', 'd\n']]), 'приёмов с Go после последнего winqa: 1 (winqa в журнале нет)')
  assert.equal(take('d1', [['docs/only.md', 'd\n']]), 'приёмов с Go после последнего winqa: 1 (winqa в журнале нет)')
  assert.equal(take('g2', [['go.mod', 'module x\n']]), 'приёмов с Go после последнего winqa: 2 (winqa в журнале нет)')
  const e = readJournal(file)
  assert.equal(e.length, 3)
  assert.deepEqual(e.map((x) => [x.branch, x.go]), [['g1', true], ['d1', false], ['g2', true]])
  assert.deepEqual(e[0].packages, ['internal/app/edit', 'internal/layout'])
  assert.deepEqual(e[1].packages, [])
  assert.deepEqual(e[2].packages, ['go.mod'])
  assert.match(e[0].from, /^[0-9a-f]{12}$/)
  assert.equal(e[0].to, e[1].from, 'диапазоны приёмов смыкаются')
  assert.ok(fs.existsSync(file) && !fs.existsSync(path.join(s.root, 'sa-accept.jsonl')), 'журнал в .git, а не в дереве')
  // не-rc winqa обнуляет счёт; rc — нет
  appendEntry(file, { winqa: '143-g1', dev: 'abc' })
  assert.equal(take('g3', [['internal/a/z.go', 'package a\n']]), 'приёмов с Go после последнего winqa: 1 (winqa/143-g1)')
  appendEntry(file, { winqa: 'rc-1', dev: 'abc' })
  assert.equal(take('g4', [['internal/a/w.go', 'package a\n']]), 'приёмов с Go после последнего winqa: 2 (winqa/143-g1)')
})

test('журнал: повреждённый файл не срывает приём, на месте журнала каталог — предупреждение, приём влит', () => {
  const s = setup()
  fs.writeFileSync(journalFile(s.root), '{"branch":\n\u0000\u0000мусор\n[1,2]\n')
  const w = s.wt('j1')
  commit(w, 'internal/a/a.go', 'package a\n', 'j1')
  const msgs = []
  assert.match(accept('j1', 'j1', { cwd: s.root, env: s.env, log: (m) => msgs.push(m) }), /^принято j1/)
  assert.ok(msgs.includes('приёмов с Go после последнего winqa: 1 (winqa в журнале нет)'), msgs.join('|'))
  const s2 = setup()
  fs.mkdirSync(journalFile(s2.root))
  const w2 = s2.wt('j2')
  commit(w2, 'a.go', 'package a\n', 'j2')
  const m2 = []
  assert.match(accept('j2', 'j2', { cwd: s2.root, env: s2.env, log: (m) => m2.push(m) }), /^принято j2/)
  assert.ok(m2.some((m) => m.startsWith('журнал приёмов не записан')), m2.join('|'))
})
