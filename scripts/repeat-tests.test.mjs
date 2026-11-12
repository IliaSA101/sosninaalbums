// Тесты отбора и расчёта повтора новых Go-тестов: `node --test scripts/repeat-tests.test.mjs`.
import assert from 'node:assert/strict'
import test from 'node:test'
import { CAP_SEC, MAX_COUNT, hasRisk, parseDurations, planRepeats, repeatLine, runPattern, selectRiskyTests, testBodies } from './repeat-tests.mjs'

const goTest = (name, body) => `func ${name}(t *testing.T) {\n${body}\n}\n`
const none = () => new Set()

test('тест с go func берётся, чистый — нет', () => {
  const src = `package x\n\n${goTest('TestRace', '\tgo func() {}()')}\n${goTest('TestPure', '\tif 1 != 1 {\n\t\tt.Fatal()\n\t}')}`
  const r = selectRiskyTests(['internal/x/a_test.go'], (f) => (f === 'internal/x/a_test.go' ? src : null), none)
  assert.deepEqual(r, { 'internal/x': ['TestRace'] })
})

test('признаки: sync., time., t.Parallel; комментарий не считается', () => {
  assert.ok(hasRisk('var mu sync.Mutex'))
  assert.ok(hasRisk('time.Sleep(1)'))
  assert.ok(hasRisk('t.Parallel()'))
  assert.ok(hasRisk('go func() {}()'))
  assert.ok(!hasRisk('// go func и time. в комментарии\nx := 1'))
  assert.ok(!hasRisk('runtime.GC()\nxsync := 1'))
})

test('признак в тестируемом файле x.go делает тестом риска и чистый тест x_test.go', () => {
  const files = { 'p/x_test.go': goTest('TestPure', '\t_ = 1'), 'p/x.go': 'package p\nvar once sync.Once\n' }
  assert.deepEqual(selectRiskyTests(['p/x_test.go'], (f) => files[f] ?? null, none), { p: ['TestPure'] })
  files['p/x.go'] = 'package p\nvar a int\n'
  assert.deepEqual(selectRiskyTests(['p/x_test.go'], (f) => files[f] ?? null, none), {})
})

test('тест, уже бывший в базе, не берётся; новый рядом — берётся', () => {
  const src = goTest('TestOld', '\ttime.Sleep(1)') + goTest('TestNew', '\ttime.Sleep(1)')
  const r = selectRiskyTests(['p/a_test.go'], () => src, () => new Set(['TestOld']))
  assert.deepEqual(r, { p: ['TestNew'] })
})

test('несколько файлов и папок; корневой пакет — «.»; не-тестовые файлы пропущены', () => {
  const files = {
    'a_test.go': goTest('TestA', '\tgo func() {}()'),
    'p/b_test.go': goTest('TestB', '\tt.Parallel()') + goTest('TestC', '\ttime.Now()'),
    'p/b.go': 'package p',
  }
  const r = selectRiskyTests(['a_test.go', 'p/b_test.go', 'p/b.go', 'gone_test.go'], (f) => files[f] ?? null, none)
  assert.deepEqual(r, { '.': ['TestA'], p: ['TestB', 'TestC'] })
})

test('testBodies: границы тел, пустой файл', () => {
  const b = testBodies(`package x\n\nfunc helper() {\n\tgo func() {}()\n}\n\n${goTest('TestA', '\t_ = 1')}${goTest('TestB', '\t_ = 2')}`)
  assert.deepEqual(Object.keys(b), ['TestA', 'TestB'])
  assert.ok(!hasRisk(b.TestA))
  assert.deepEqual(testBodies(''), {})
})

test('planRepeats: ≈ 3 с на тест, не меньше 2 и не больше MAX_COUNT', () => {
  assert.deepEqual(planRepeats({ p: ['TestA'] }, { p: 0.3 }), { p: 10 })
  assert.deepEqual(planRepeats({ p: ['TestA'] }, { p: 100 }), { p: 2 })
  assert.deepEqual(planRepeats({ p: ['TestA'] }, { p: 0 }), { p: MAX_COUNT })
})

test('planRepeats: потолок 60 с делится по числу тестов', () => {
  const names = Array.from({ length: 40 }, (_, i) => `T${i}`)
  const groups = { a: names.slice(0, 30), b: names.slice(30) }
  const seconds = { a: 3, b: 1 }
  const r = planRepeats(groups, seconds)
  // бюджет min(60, 120) = 60: a получает 45 с на 3 с за прогон, b — 15 с на 1 с
  assert.deepEqual(r, { a: 15, b: 15 })
  const spent = Object.keys(r).reduce((s, d) => s + r[d] * seconds[d], 0)
  assert.ok(spent <= CAP_SEC, `${spent}`)
})

test('parseDurations: только верхний уровень, PASS и FAIL', () => {
  const out = '=== RUN   TestA\n--- PASS: TestA (0.12s)\n    --- PASS: TestA/sub (0.30s)\n--- FAIL: TestB (1.50s)\n--- SKIP: TestC (0.00s)\nok  p 1s'
  assert.deepEqual(parseDurations(out), { TestA: 0.12, TestB: 1.5 })
  assert.deepEqual(parseDurations(''), {})
})

test('runPattern и repeatLine', () => {
  assert.equal(runPattern(['TestA', 'TestB']), '^(TestA|TestB)$')
  assert.equal(repeatLine(2, { a: 10 }, 6.04), 'повтор новых тестов: 2 тестов, 10 раз, 6.0s')
  assert.equal(repeatLine(3, { a: 10, b: 4 }, 1), 'повтор новых тестов: 3 тестов, 4–10 раз, 1.0s')
})
