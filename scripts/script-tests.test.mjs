// Список шагов script-tests.mjs: совпадение с workflow (страж расхождения), пропуски без инструментов, код первого провала.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { NO_DESKTOP, affectedScriptTests, buildSteps, runAll, runStep } from './script-tests.mjs'

const root = path.resolve(path.dirname(new URL(import.meta.url).pathname.replace(/^\/(\w:)/, '$1')), '..')
const workflow = (f) => fs.readFileSync(path.join(root, '.github', 'workflows', f), 'utf8')
const FULL = { platform: 'linux', python: 'python3', hasDocx: true, hasMakensis: true, hasDpkg: true }
const byName = (steps, part) => steps.find((s) => s.name.includes(part))

test('каждый шаг есть в workflow (ci.yml / release.yml) дословно', () => {
  for (const s of buildSteps(FULL)) {
    assert.ok(workflow(s.ci[0]).includes(s.ci[1]), `${s.name}: «${s.ci[1]}» не найдено в ${s.ci[0]}`)
  }
})

test('обратный страж: каждый python unittest и test_*.sh из ci.yml/release.yml есть в списке', () => {
  const have = buildSteps(FULL)
  const text = workflow('ci.yml') + workflow('release.yml')
  const dirs = new Set([...text.matchAll(/unittest discover -s (scripts\/[\w-]+)/g)].map((m) => m[1]))
  assert.ok(dirs.size >= 5, 'ожидали win, mac, guide, guide-videos, e2e')
  for (const d of dirs) assert.ok(have.some((s) => s.args.at(-1) === d), `в script-tests нет ${d}`)
  const shells = new Set([...text.matchAll(/bash (scripts\/\w+\/test_\w+\.sh)/g)].map((m) => m[1]))
  for (const sh of shells) assert.ok(have.some((s) => s.args.includes(sh)), `в script-tests нет ${sh}`)
  assert.ok(text.includes('node --test "scripts/*.test.mjs"') && have.some((s) => s.cmd === 'node' && s.args[0] === '--test'))
})

test('полный набор: нет пропусков и провалов без запуска, у win — CI=true, у test_deb — без рабочего стола', () => {
  const steps = buildSteps(FULL)
  assert.equal(steps.length, 9)
  for (const s of steps) assert.ok(!s.skip && !s.fail, s.name)
  assert.equal(byName(steps, 'scripts/win').env.CI, 'true')
  assert.deepEqual(byName(steps, 'test_deb').unset, ['DISPLAY', 'WAYLAND_DISPLAY', 'XDG_CURRENT_DESKTOP', 'DESKTOP_SESSION'])
})

test('нет python: пять python-шагов пропущены с предупреждением, остальные идут', () => {
  const steps = buildSteps({ ...FULL, python: '', hasDocx: false })
  const skipped = steps.filter((s) => s.skip).map((s) => s.name)
  assert.deepEqual(skipped, ['python scripts/win', 'python scripts/mac', 'python scripts/guide', 'python scripts/guide-videos', 'python scripts/e2e'])
  assert.ok(!byName(steps, 'scripts/win').fail, 'без python нечего проверять на makensis')
  assert.ok(!byName(steps, 'node --test').skip && !byName(steps, 'test_lib').skip)
})

test('нет python-docx — пропуск только guide; нет makensis — провал win (CI его требует); нет dpkg и Windows — пропуск test_deb', () => {
  assert.deepEqual(buildSteps({ ...FULL, hasDocx: false }).filter((s) => s.skip).map((s) => s.name), ['python scripts/guide'])
  const noNsis = buildSteps({ ...FULL, hasMakensis: false })
  assert.match(byName(noNsis, 'scripts/win').fail, /apt install nsis/)
  assert.equal(noNsis.filter((s) => s.fail).length, 1)
  assert.match(byName(buildSteps({ ...FULL, hasDpkg: false }), 'test_deb').skip, /dpkg-deb/)
  assert.match(byName(buildSteps({ ...FULL, platform: 'win32', hasDpkg: false }), 'test_deb').skip, /Windows/)
})

test('runStep: пропуск и провал без запуска не вызывают spawn; окружение шага накладывается, DISPLAY снят', () => {
  const never = () => assert.fail('spawn не должен вызываться')
  assert.equal(runStep({ name: 'x', skip: 'нет' }, { cwd: '.', env: {}, spawn: never }).status, 'warn')
  assert.deepEqual(runStep({ name: 'x', fail: 'плохо' }, { cwd: '.', env: {}, spawn: never }), { status: 'fail', code: 1, out: 'плохо' })
  let seen
  const spawn = (cmd, args, o) => ((seen = { cmd, args, o }), { status: 0, stdout: 'ok', stderr: '' })
  const r = runStep({ name: 'd', cmd: 'bash', args: ['a'], env: { CI: 'true' }, unset: ['DISPLAY'] }, { cwd: '/w', env: { DISPLAY: ':0', KEEP: '1' }, spawn })
  assert.equal(r.status, 'ok')
  assert.equal(seen.o.cwd, '/w')
  assert.equal(seen.o.env.CI, 'true')
  assert.equal(seen.o.env.KEEP, '1')
  assert.ok(!('DISPLAY' in seen.o.env))
})

test('runStep: код ≠ 0, сигнал (status null) и ошибка запуска — провал', () => {
  const run = (res) => runStep({ name: 'x', cmd: 'c', args: [] }, { cwd: '.', env: {}, spawn: () => res })
  assert.deepEqual(run({ status: 3, stdout: 'a', stderr: 'b' }), { status: 'fail', code: 3, out: 'ab' })
  assert.equal(run({ status: null, stdout: '', stderr: '' }).code, 1)
  assert.match(run({ error: new Error('ENOENT') }).out, /ENOENT/)
})

test('runAll: упавший шаг не останавливает остальные, код — первого провала; без провалов и пустой список — 0', () => {
  const results = { a: { status: 'ok', code: 0 }, b: { status: 'fail', code: 7 }, c: { status: 'fail', code: 9 } }
  const steps = ['a', 'b', 'c'].map((n) => ({ name: n, cmd: n, args: [] }))
  const spawn = (cmd) => ({ status: results[cmd].code, stdout: '', stderr: '' })
  const names = []
  assert.equal(runAll(steps, { cwd: '.', env: {}, spawn }, (s) => names.push(s.name)), 7)
  assert.deepEqual(names, ['a', 'b', 'c'])
  assert.equal(runAll(steps.slice(0, 1), { cwd: '.', env: {}, spawn }), 0)
  assert.equal(runAll([], { cwd: '.', env: {} }), 0)
  // предупреждение (пропуск) — не провал
  assert.equal(runAll([{ name: 'w', skip: 'нет' }], { cwd: '.', env: {} }), 0)
})

test('affectedScriptTests: правка скрипта берёт его тест; тест — себя; скрипт без теста и чужие файлы — ничего', () => {
  const has = new Set(['scripts/release-sign.test.mjs', 'scripts/winqa-push.test.mjs'])
  const ex = (f) => has.has(f)
  const r = affectedScriptTests(['scripts/release-sign.mjs', 'scripts/winqa-push.test.mjs', 'scripts/heavy.mjs', 'README.md', 'internal/a.go'], ex)
  assert.deepEqual(r, { node: ['scripts/release-sign.test.mjs', 'scripts/winqa-push.test.mjs'], win: false, deb: false })
  // скрипт и его тест одновременно — один раз
  assert.deepEqual(affectedScriptTests(['scripts/release-sign.mjs', 'scripts/release-sign.test.mjs'], ex).node, ['scripts/release-sign.test.mjs'])
  // вложенная папка и посторонний тип не считаются скриптами верхнего уровня
  assert.deepEqual(affectedScriptTests(['scripts/win/x.mjs', 'scripts/release-sign.py', 'frontend/scripts/release-sign.mjs'], ex), { node: [], win: true, deb: false })
  assert.deepEqual(affectedScriptTests([], ex), { node: [], win: false, deb: false })
})

test('affectedScriptTests: pack-check — паритет с Go (pack/*.go, файлы случаев) и модули папки запускают оба его теста', () => {
  const both = ['scripts/pack-check-zip.test.mjs', 'scripts/pack-check.test.mjs']
  const ex = (f) => both.includes(f)
  for (const f of ['internal/decor/pack/manifest.go', 'internal/decor/pack/eocd.go', 'testdata/pack-manifest-cases.json', 'testdata/pack-archive-cases.json', 'scripts/pack-check/zip.mjs', 'scripts/pack-check.mjs']) {
    assert.deepEqual(affectedScriptTests([f], ex).node, both, f)
  }
  assert.deepEqual(affectedScriptTests(['internal/decor/packs_install.go', 'internal/decor/pack_other/x.go', 'testdata/other.json'], ex).node, [])
  assert.deepEqual(affectedScriptTests(['internal/decor/pack/manifest.go'], () => false).node, []) // нет файла теста — нечего запускать
})

test('affectedScriptTests: workflow — все node-тесты и scripts/win; scripts/win/** — упаковка; scripts/linux/** — test_deb', () => {
  const none = () => false
  assert.deepEqual(affectedScriptTests(['.github/workflows/win-qa.yml'], none), { node: ['scripts/*.test.mjs'], win: true, deb: false })
  assert.deepEqual(affectedScriptTests(['scripts/win/pack.py'], none), { node: [], win: true, deb: false })
  assert.deepEqual(affectedScriptTests(['scripts/linux/deb.sh'], none), { node: [], win: false, deb: true })
  assert.deepEqual(affectedScriptTests(['scripts/linux/deb.sh', 'scripts/win/a.sh', '.github/workflows/ci.yml'], none), { node: ['scripts/*.test.mjs'], win: true, deb: true })
  assert.deepEqual(NO_DESKTOP, buildSteps(FULL).find((s) => s.unset).unset)
})
