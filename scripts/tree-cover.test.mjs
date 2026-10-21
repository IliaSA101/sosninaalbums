// Тесты разбора профиля покрытия и сравнения с эталоном: `node --test scripts/`.
import assert from 'node:assert/strict'
import test from 'node:test'
import { MAX_DROP, PRECHECK_SLACK, TREE_MAX_MISSED, TREE_MIN, compareTree, isTree, parseProfile } from './tree-cover.mjs'

const M = 'github.com/IliaSA101/sosninaalbums/internal/album'

test('parseProfile: блок покрыт, если покрыт в любом прогоне', () => {
  const text = [
    'mode: set',
    `${M}/grid/a.go:1.1,2.2 3 0`,
    `${M}/grid/a.go:1.1,2.2 3 1`, // тот же блок из другого пакета тестов
    `${M}/grid/a.go:5.1,6.2 2 0`,
    `${M}/grid/b.go:1.1,2.2 5 4`,
    '',
    'мусорная строка',
  ].join('\n')
  const r = parseProfile(text)
  assert.deepEqual(r, {
    stmts: 10,
    covered: 8,
    missed: 1,
    missedBlocks: [`${M}/grid/a.go:5.1,6.2`],
    cover: 80,
  })
})

test('parseProfile: пустой профиль — нули без деления на ноль', () => {
  assert.deepEqual(parseProfile('mode: set\n'), { stmts: 0, covered: 0, missed: 0, missedBlocks: [], cover: 0 })
})

test('isTree: корень и подпакеты, но не соседи по префиксу', () => {
  assert.ok(isTree('internal/album'))
  assert.ok(isTree('internal/album/grid'))
  assert.ok(!isTree('internal/albumx'))
  assert.ok(!isTree('internal/app'))
})

test('compareTree: эталон не ухудшился — ошибок нет', () => {
  assert.deepEqual(compareTree({ cover: 99, missed: 40 }, { cover: 99, missed: 40 }), [])
  assert.deepEqual(compareTree({ cover: 99, missed: 40 }, { cover: 99.2, missed: 30 }), [])
  // допуск падения ровно MAX_DROP
  assert.deepEqual(compareTree({ cover: 99, missed: 40 }, { cover: 99 - MAX_DROP, missed: 40 }), [])
})

test('compareTree: новый непокрытый блок, падение покрытия и уровень ниже порога', () => {
  assert.equal(compareTree({ cover: 99, missed: 40 }, { cover: 99, missed: 41 }).length, 1)
  assert.equal(compareTree({ cover: 99, missed: 40 }, { cover: 98.4, missed: 40 }).length, 1)
  assert.equal(compareTree({ cover: 99, missed: 40 }, { cover: TREE_MIN - 0.1, missed: 40 }).length, 2) // падение и порог
})

test('compareTree: эталон без missed — предел TREE_MAX_MISSED', () => {
  assert.deepEqual(compareTree({ cover: 99 }, { cover: 99, missed: TREE_MAX_MISSED }), [])
  assert.equal(compareTree({ cover: 99 }, { cover: 99, missed: TREE_MAX_MISSED + 1 }).length, 1)
})

test('compareTree: допуск precheck — небольшой спад только в допуске, сверх него ошибка', () => {
  const base = { cover: 99, missed: 40 }
  const small = { cover: 99 - MAX_DROP - 0.3, missed: 43 }
  assert.notDeepEqual(compareTree(base, small), [])
  assert.deepEqual(compareTree(base, small, PRECHECK_SLACK), [])
  const big = { cover: 99, missed: 40 + PRECHECK_SLACK.missed + 1 }
  assert.notDeepEqual(compareTree(base, big, PRECHECK_SLACK), [])
  const deep = { cover: 99 - MAX_DROP - PRECHECK_SLACK.drop - 0.3, missed: 40 }
  assert.notDeepEqual(compareTree(base, deep, PRECHECK_SLACK), [])
})
