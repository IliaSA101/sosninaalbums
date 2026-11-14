// Журнал приёмов (accept-log.mjs): разбор, счёт, проверка rc. Журнал — во временной папке, gh подменён функцией.
import { test, before, after } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { MAX_BYTES, appendEntry, checkRc, goPackages, readJournal, runState, sinceWinqa } from './accept-log.mjs'

const gotmp = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', '.gotmp')
let tmp
before(() => {
  fs.mkdirSync(gotmp, { recursive: true })
  tmp = fs.mkdtempSync(path.join(gotmp, 'accept-log-'))
})
after(() => fs.rmSync(tmp, { recursive: true, force: true }))

const acc = (go) => ({ branch: 'b', go })
const green = () => JSON.stringify([{ status: 'completed', conclusion: 'success' }])

test('goPackages: каталоги .go, go.mod/go.sum одним пакетом, прочее и пустой список — пусто', () => {
  assert.deepEqual(goPackages([]), [])
  assert.deepEqual(goPackages(['docs/a.md', 'frontend/src/a.ts', 'wails.json']), [])
  assert.deepEqual(goPackages(['internal/a/x.go', 'internal/a/y_test.go', 'main.go', 'go.mod', 'go.sum']), ['go.mod', 'internal/a', '.'].sort())
})

test('readJournal: нет файла, пустой, мусор и оборванная строка, каталог — без падения', () => {
  assert.deepEqual(readJournal(path.join(tmp, 'нет')), [])
  const f = path.join(tmp, 'j.jsonl')
  fs.writeFileSync(f, '')
  assert.deepEqual(readJournal(f), [])
  fs.writeFileSync(f, '\n\n{"a":1}\n[1]\n"s"\nне json\n{"b":2}\n{"c":')
  assert.deepEqual(readJournal(f), [{ a: 1 }, { b: 2 }])
  assert.deepEqual(readJournal(tmp), [], 'каталог на месте файла')
})

test('readJournal: файл крупнее предела — читается хвост, обрезанная первая строка отброшена', () => {
  const f = path.join(tmp, 'big.jsonl')
  const line = `${JSON.stringify({ pad: 'x'.repeat(990), n: 0 })}\n`
  const total = Math.ceil((MAX_BYTES * 2) / line.length)
  const fd = fs.openSync(f, 'w')
  for (let i = 0; i < total; i++) fs.writeSync(fd, line.replace('"n":0', `"n":${i}`))
  fs.closeSync(fd)
  const e = readJournal(f)
  assert.ok(e.length > 1000 && e.length < total, `хвост, а не весь файл: ${e.length} из ${total}`)
  assert.equal(e.at(-1).n, total - 1)
})

test('appendEntry: строка с временем, записи копятся', () => {
  const f = path.join(tmp, 'a.jsonl')
  appendEntry(f, { branch: 'x', go: true })
  appendEntry(f, { winqa: 'n', dev: 'h' })
  const e = readJournal(f)
  assert.equal(e.length, 2)
  assert.ok(!Number.isNaN(Date.parse(e[0].t)))
  assert.equal(e[1].winqa, 'n')
})

test('sinceWinqa: пусто; без winqa — все приёмы с Go; не-rc winqa сбрасывает; rc нет; не-Go не считаются', () => {
  assert.deepEqual(sinceWinqa([]), { count: 0, winqa: null })
  assert.deepEqual(sinceWinqa([acc(true), acc(false), acc(true)]), { count: 2, winqa: null })
  assert.deepEqual(sinceWinqa([acc(true), { winqa: 'a' }, acc(true), { winqa: 'rc-1' }, acc(true), acc(false)]), { count: 2, winqa: 'a' })
  assert.deepEqual(sinceWinqa([acc(true), { winqa: 'a' }, { winqa: 'b' }]), { count: 0, winqa: 'b' })
})

test('runState: success, failure/cancelled, идёт, пусто, мусор', () => {
  assert.equal(runState(green()), 'green')
  assert.equal(runState('[{"status":"completed","conclusion":"failure"}]'), 'red')
  assert.equal(runState('[{"status":"completed","conclusion":"cancelled"}]'), 'red')
  assert.equal(runState('[{"status":"in_progress","conclusion":""}]'), 'unknown')
  assert.equal(runState('[]'), 'unknown')
  assert.equal(runState(''), 'unknown')
  assert.equal(runState('{не json'), 'unknown')
  assert.equal(runState('{"status":"completed"}'), 'unknown')
})

test('checkRc: 5 приёмов и зелёный — можно; 6 — отказ; красный — отказ; оба — два отказа', () => {
  const many = (n) => [{ winqa: 'a' }, ...Array.from({ length: n }, () => acc(true))]
  assert.deepEqual(checkRc(many(5), green), { problems: [], warnings: [] })
  const six = checkRc(many(6), green)
  assert.equal(six.problems.length, 1)
  assert.match(six.problems[0], /6 приёмов с Go.*winqa-push/)
  const red = checkRc(many(0), () => '[{"status":"completed","conclusion":"failure"}]')
  assert.match(red.problems[0], /winqa\/a красный/)
  assert.equal(checkRc(many(9), () => '[{"status":"completed","conclusion":"failure"}]').problems.length, 2)
})

test('checkRc: gh спрашивает ветку последнего не-rc winqa; сбой gh, нет прогонов, нет winqa — предупреждение, не отказ', () => {
  let seen
  checkRc([{ winqa: 'old' }, { winqa: 'new' }, { winqa: 'rc-3' }], (a) => ((seen = a), green()))
  assert.equal(seen[seen.indexOf('--branch') + 1], 'winqa/new')
  const down = checkRc([{ winqa: 'a' }], () => {
    throw new Error('gh: нет сети\nвторая строка')
  })
  assert.deepEqual(down.problems, [])
  assert.match(down.warnings[0], /gh не ответил \(gh: нет сети\)/)
  assert.match(checkRc([{ winqa: 'a' }], () => '[]').warnings[0], /нет или он не завершён/)
  const none = checkRc([acc(true)], () => assert.fail('gh не нужен'))
  assert.deepEqual(none.problems, [])
  assert.match(none.warnings[0], /нет ни одного не-rc winqa/)
})
