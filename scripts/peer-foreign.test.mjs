import assert from 'node:assert/strict'
import test from 'node:test'
import { analyze } from './peer-foreign.mjs'

const pass = '--- PASS: TestForeignClient (0.52s)\n--- PASS: TestForeignServer (0.40s)\nPASS\nok  \tpeercred\t1.0s\n'
const both = ['TestForeignClient', 'TestForeignServer']

test('оба теста прошли — причин нет', () => {
  assert.deepEqual(analyze(pass, 0, both), [])
})

test('пропуск — провал, с --allow-skip — нет', () => {
  const why = analyze('--- SKIP: TestForeignClient (0.00s)\n--- PASS: TestForeignServer (0.1s)\n', 0, both)
  assert.equal(why.length, 2) // пропуск и нет PASS
  assert.match(why[0], /пропущен TestForeignClient/)
  assert.deepEqual(analyze('--- SKIP: TestElevatedServerWindows (0.00s)\n', 0, [], true), [])
})

test('no tests to run при коде 0 — провал: нет PASS названных тестов', () => {
  const why = analyze('testing: warning: no tests to run\nPASS\nok  \tpeercred\t0.01s [no tests to run]\n', 0, both)
  assert.equal(why.length, 2)
})

test('провал теста и ненулевой код собираются вместе', () => {
  const why = analyze('--- FAIL: TestForeignServer (1.0s)\n--- PASS: TestForeignClient (0.5s)\nFAIL\n', 1, both)
  assert.equal(why.length, 3) // код, FAIL, нет PASS
})

test('подтест не заменяет PASS самого теста; имя-префикс не считается', () => {
  assert.equal(analyze('    --- PASS: TestForeignClient/sub (0.1s)\n', 0, ['TestForeignClient']).length, 1)
  assert.equal(analyze('--- PASS: TestForeignClientX (0.1s)\n', 0, ['TestForeignClient']).length, 1)
})
