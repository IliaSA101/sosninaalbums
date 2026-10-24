// Построение команд очистки winqa-ветки без сети.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { buildCleanup, parseArtifacts } from './winqa-cleanup.mjs'

const runs = [
  { databaseId: 11, createdAt: '2026-10-23T10:00:00Z' },
  { databaseId: 12, createdAt: '2026-10-23T12:00:00Z' },
]
const artifacts = [
  { id: 1, run: 11 },
  { id: 2, run: 12 },
]

test('all runs and artifacts, branch last', () => {
  const c = buildCleanup('winqa/t', runs, artifacts, false).map((x) => x.join(' '))
  assert.deepEqual(c, [
    'gh api -X DELETE repos/{owner}/{repo}/actions/artifacts/1',
    'gh api -X DELETE repos/{owner}/{repo}/actions/artifacts/2',
    'gh run delete 12',
    'gh run delete 11',
    'git push origin --delete winqa/t',
  ])
})

test('keep-last keeps the newest run with its artifacts', () => {
  const c = buildCleanup('winqa/t', runs, artifacts, true).map((x) => x.join(' '))
  assert.deepEqual(c, ['gh api -X DELETE repos/{owner}/{repo}/actions/artifacts/1', 'gh run delete 11', 'git push origin --delete winqa/t'])
})

test('no runs: only the branch; artifacts filtered by branch', () => {
  assert.deepEqual(buildCleanup('winqa/t', [], [], true), [['git', 'push', 'origin', '--delete', 'winqa/t']])
  const text = '{"id":1,"run":5,"branch":"winqa/t"}\n{"id":2,"run":6,"branch":"main"}\n'
  assert.deepEqual(parseArtifacts(text, 'winqa/t'), [{ id: 1, run: 5, branch: 'winqa/t' }])
})
