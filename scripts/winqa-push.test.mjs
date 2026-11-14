// Фильтр winqa-push.mjs на временном репозитории, без сети: коммит-сирота без путей publish-exclude.txt.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync, spawnSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'

const script = path.join(path.dirname(new URL(import.meta.url).pathname.replace(/^\/(\w:)/, '$1')), 'winqa-push.mjs')

function makeRepo() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'winqa-push-'))
  const g = (...a) => execFileSync('git', ['-c', 'user.name=T', '-c', 'user.email=t@example.invalid', ...a], { cwd: dir, encoding: 'utf8' })
  g('init', '-q')
  g('config', 'core.autocrlf', 'false') // не зависеть от CRLF-настроек машины
  const put = (p, t) => {
    fs.mkdirSync(path.dirname(path.join(dir, p)), { recursive: true })
    fs.writeFileSync(path.join(dir, p), t)
  }
  put('scripts/publish-exclude.txt', '# x\ndocs/qa/\ndocs/BACKLOG.md\n')
  put('docs/qa/update.md', 'x')
  put('docs/BACKLOG.md', 'x')
  put('docs/keep.md', 'x')
  put('scripts/win/a.ps1', 'x')
  g('add', '-A')
  g('commit', '-q', '-m', 'one')
  put('second.txt', 'x')
  g('add', '-A')
  g('commit', '-q', '-m', 'two')
  return { dir, g }
}
// Пользователь git для commit-tree берётся из окружения: в тесте задаём явно
const env = { ...process.env, GIT_AUTHOR_NAME: 'T', GIT_AUTHOR_EMAIL: 't@example.invalid', GIT_COMMITTER_NAME: 'T', GIT_COMMITTER_EMAIL: 't@example.invalid' }
const run = (dir, ...a) => execFileSync('node', [script, '--repo', dir, ...a], { encoding: 'utf8', env })

test('orphan commit without excluded paths', () => {
  const { dir, g } = makeRepo()
  try {
    const out = run(dir, 'smoke')
    const sha = out.match(/origin ([0-9a-f]{40}):refs\/heads\/winqa\/smoke/)[1]
    const files = g('ls-tree', '-r', '--name-only', sha).split('\n').filter(Boolean)
    assert.deepEqual(files.sort(), ['docs/keep.md', 'scripts/publish-exclude.txt', 'scripts/win/a.ps1', 'second.txt'])
    assert.equal(g('rev-list', '--parents', '-n', '1', sha).trim(), sha, 'родителей нет')
    assert.equal(g('log', '-1', '--format=%an', sha).trim(), 'T')
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('ref other than HEAD, delete only prints, bad name refused', () => {
  const { dir } = makeRepo()
  try {
    assert.match(run(dir, 'old', 'HEAD~1'), /refs\/heads\/winqa\/old/)
    assert.match(run(dir, '--delete', 'old'), /--delete winqa\/old/)
    assert.throws(() => run(dir, 'Bad Name'))
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

// Журнал и проверка rc (С1, 0.35): gh подменён скриптом WINQA_GH, журнал — в .git временного репозитория.
const exec = (dir, args, ghOut, ghFail) => {
  const stub = path.join(dir, 'gh-stub.mjs')
  fs.writeFileSync(
    stub,
    `import fs from 'node:fs'
fs.appendFileSync(${JSON.stringify(path.join(dir, 'gh.log'))}, JSON.stringify(process.argv.slice(2)) + '\\n')
if (${JSON.stringify(!!ghFail)}) { console.error('gh: нет сети'); process.exit(1) }
console.log(${JSON.stringify(ghOut ?? '[{"status":"completed","conclusion":"success"}]')})
`,
  )
  const r = spawnSync(process.execPath, [script, '--repo', dir, ...args], { encoding: 'utf8', env: { ...env, WINQA_GH: stub } })
  return { code: r.status, out: r.stdout, err: r.stderr }
}
const journalOf = (dir) => path.join(dir, '.git', 'sa-accept.jsonl')
const writeJournal = (dir, entries) => fs.writeFileSync(journalOf(dir), entries.map((e) => JSON.stringify(e)).join('\n') + '\n')
const goAccepts = (n) => Array.from({ length: n }, (_, i) => ({ branch: `b${i}`, go: true }))
const ghCalls = (dir) => (fs.existsSync(path.join(dir, 'gh.log')) ? fs.readFileSync(path.join(dir, 'gh.log'), 'utf8').split('\n').filter(Boolean).map((l) => JSON.parse(l)) : [])
const readLines = (dir) => fs.readFileSync(journalOf(dir), 'utf8').trim().split('\n').map((l) => JSON.parse(l))

test('--push пишет в журнал {winqa, dev}; без --push журнал не трогается', () => {
  const { dir, g } = makeRepo()
  const origin = fs.mkdtempSync(path.join(os.tmpdir(), 'winqa-origin-'))
  try {
    g('init', '-q', '--bare', origin)
    g('remote', 'add', 'origin', origin)
    g('branch', '-M', 'dev')
    assert.equal(exec(dir, ['n1']).code, 0)
    assert.ok(!fs.existsSync(journalOf(dir)), 'без --push записи нет')
    assert.equal(exec(dir, ['n1', '--push']).code, 0)
    const e = readLines(dir)
    assert.equal(e.length, 1)
    assert.equal(e[0].winqa, 'n1')
    assert.equal(e[0].dev, g('rev-parse', 'dev').trim())
    assert.ok(!('bypass' in e[0]))
    assert.match(g('ls-remote', origin, 'refs/heads/winqa/n1'), /refs\/heads\/winqa\/n1/)
    assert.deepEqual(ghCalls(dir), [], 'не-rc имя gh не спрашивает')
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
    fs.rmSync(origin, { recursive: true, force: true })
  }
})

test('rc-*: 5 приёмов с Go после зелёного winqa — проходит; 6 — отказ с подсказкой; не-Go приёмы не считаются', () => {
  const { dir } = makeRepo()
  try {
    writeJournal(dir, [{ winqa: 'a1', dev: 'x' }, ...goAccepts(5), { branch: 'doc', go: false }, { branch: 'doc2', go: false }])
    const ok = exec(dir, ['rc-1'])
    assert.equal(ok.code, 0, ok.err)
    assert.equal(ghCalls(dir)[0][ghCalls(dir)[0].indexOf('--branch') + 1], 'winqa/a1')
    writeJournal(dir, [{ winqa: 'a1', dev: 'x' }, ...goAccepts(6)])
    const bad = exec(dir, ['rc-1'])
    assert.equal(bad.code, 1)
    assert.match(bad.err, /rc остановлен: .*6 приёмов с Go/)
    assert.match(bad.err, /winqa-push\.mjs <имя> --push/)
    assert.match(bad.err, /--no-winqa "<причина>"/)
    assert.doesNotMatch(bad.out, /refs\/heads/, 'команда отправки не напечатана')
    // после нового winqa счёт с нуля
    fs.appendFileSync(journalOf(dir), JSON.stringify({ winqa: 'a2', dev: 'y' }) + '\n')
    assert.equal(exec(dir, ['rc-1']).code, 0)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('rc-*: красный последний winqa — отказ; зелёный, идёт или нет прогонов — проход (с предупреждением)', () => {
  const { dir } = makeRepo()
  try {
    writeJournal(dir, [{ winqa: 'a1', dev: 'x' }, ...goAccepts(1)])
    const red = exec(dir, ['rc-1'], '[{"status":"completed","conclusion":"failure"}]')
    assert.equal(red.code, 1)
    assert.match(red.err, /winqa\/a1 красный/)
    assert.equal(exec(dir, ['rc-1'], '[{"status":"completed","conclusion":"success"}]').code, 0)
    const run = exec(dir, ['rc-1'], '[{"status":"in_progress","conclusion":""}]')
    assert.equal(run.code, 0)
    assert.match(run.err, /предупреждение: .*не завершён/)
    assert.equal(exec(dir, ['rc-1'], '[]').code, 0)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('rc-*: gh недоступен — предупреждение, не отказ; счёт приёмов при этом всё равно действует', () => {
  const { dir } = makeRepo()
  try {
    writeJournal(dir, [{ winqa: 'a1', dev: 'x' }, ...goAccepts(2)])
    const r = exec(dir, ['rc-1'], null, true)
    assert.equal(r.code, 0)
    assert.match(r.err, /предупреждение: gh не ответил \(gh: нет сети\)/)
    writeJournal(dir, [{ winqa: 'a1', dev: 'x' }, ...goAccepts(7)])
    assert.equal(exec(dir, ['rc-1'], null, true).code, 1)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('rc-*: нет журнала или он пуст/повреждён — проход с предупреждением; битые строки не мешают счёту', () => {
  const { dir } = makeRepo()
  try {
    const none = exec(dir, ['rc-1'])
    assert.equal(none.code, 0)
    assert.match(none.err, /нет ни одного не-rc winqa/)
    fs.writeFileSync(journalOf(dir), '')
    assert.equal(exec(dir, ['rc-1']).code, 0)
    fs.writeFileSync(journalOf(dir), '{"winqa":"a1"}\nмусор\n' + goAccepts(6).map((e) => JSON.stringify(e)).join('\n') + '\n{"branch":')
    assert.equal(exec(dir, ['rc-1']).code, 1)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('--no-winqa "<причина>": rc проходит, причина печатается и пишется в журнал, gh не вызывается; без причины — отказ', () => {
  const { dir, g } = makeRepo()
  const origin = fs.mkdtempSync(path.join(os.tmpdir(), 'winqa-origin-'))
  try {
    g('init', '-q', '--bare', origin)
    g('remote', 'add', 'origin', origin)
    writeJournal(dir, [{ winqa: 'a1', dev: 'x' }, ...goAccepts(8)])
    const r = exec(dir, ['rc-1', '--no-winqa', 'горит срок, Windows без правок', '--push'])
    assert.equal(r.code, 0, r.err)
    assert.match(r.out, /проверка winqa пропущена \(--no-winqa\): горит срок, Windows без правок/)
    assert.deepEqual(ghCalls(dir), [])
    const last = readLines(dir).at(-1)
    assert.equal(last.winqa, 'rc-1')
    assert.equal(last.bypass, 'горит срок, Windows без правок')
    // без причины: в конце, пустая, или следом другой флаг; журнал и ветка не меняются
    for (const a of [['rc-2', '--no-winqa'], ['rc-2', '--no-winqa', '  '], ['rc-2', '--no-winqa', '--push']]) {
      const bad = exec(dir, a)
      assert.equal(bad.code, 2, a.join(' '))
      assert.match(bad.err, /--no-winqa требует причину/)
    }
    assert.equal(g('ls-remote', origin, 'refs/heads/winqa/rc-2').trim(), '')
    assert.equal(readLines(dir).length, 10)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
    fs.rmSync(origin, { recursive: true, force: true })
  }
})

test('удаление rc-ветки проверку не включает', () => {
  const { dir } = makeRepo()
  try {
    writeJournal(dir, [{ winqa: 'a1', dev: 'x' }, ...goAccepts(9)])
    const r = exec(dir, ['--delete', 'rc-1'])
    assert.equal(r.code, 0, r.err)
    assert.match(r.out, /--delete winqa\/rc-1/)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})
