// Тесты goenv: `node --test scripts/`.
import assert from 'node:assert/strict'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { goCaches, mainRoot, withGoCaches } from './goenv.mjs'

const here = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')

test('mainRoot: корень основной копии, не worktree', () => {
  const m = mainRoot(here)
  assert.ok(path.isAbsolute(m))
  assert.ok(!m.includes(`${path.sep}.claude${path.sep}worktrees`), m)
})

test('mainRoot: не git — сама папка', () => {
  const root = path.parse(here).root
  assert.equal(mainRoot(root), root)
})

test('goCaches: пути в .gotmp основной копии', () => {
  const c = goCaches(here)
  assert.equal(c.GOCACHE, path.join(mainRoot(here), '.gotmp', 'cache-go'))
  assert.equal(c.GOLANGCI_LINT_CACHE, path.join(mainRoot(here), '.gotmp', 'cache-golangci'))
})

test('withGoCaches: перебивает заданный GOCACHE, остальное сохраняет', () => {
  const e = withGoCaches({ GOCACHE: 'X:/user', FOO: '1' }, here)
  assert.equal(e.GOCACHE, goCaches(here).GOCACHE)
  assert.equal(e.FOO, '1')
})
