// Построение команд очистки winqa-ветки без сети.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { WORKFLOWS, buildCleanup, gatherRuns, parseArtifacts } from './winqa-cleanup.mjs'

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

const wruns = [
  { databaseId: 1, createdAt: '2026-10-23T10:00:00Z', workflow: 'win-qa.yml' },
  { databaseId: 2, createdAt: '2026-10-23T11:00:00Z', workflow: 'win-qa.yml' },
  { databaseId: 3, createdAt: '2026-10-23T09:00:00Z', workflow: 'ci.yml' },
  { databaseId: 4, createdAt: '2026-10-23T08:00:00Z', workflow: 'release.yml' },
  { databaseId: 5, createdAt: '2026-10-23T07:00:00Z', workflow: 'release.yml' },
]
const wart = [1, 2, 3, 4, 5].map((run) => ({ id: 100 + run, run }))

test('keep-last по workflow: по новому прогону каждого остаётся, старые и их артефакты удаляются', () => {
  const c = buildCleanup('winqa/t', wruns, wart, true).map((x) => x.join(' '))
  assert.deepEqual(c, [
    'gh api -X DELETE repos/{owner}/{repo}/actions/artifacts/101',
    'gh api -X DELETE repos/{owner}/{repo}/actions/artifacts/105',
    'gh run delete 1',
    'gh run delete 5',
    'git push origin --delete winqa/t',
  ])
})

test('без keep-last с несколькими workflow — удаляется всё; один workflow с одним прогоном и keep-last — ничего, кроме ветки', () => {
  const all = buildCleanup('winqa/t', wruns, wart, false)
  assert.equal(all.filter((c) => c[1] === 'run').length, 5)
  assert.equal(all.filter((c) => c[1] === 'api').length, 5)
  assert.deepEqual(buildCleanup('winqa/t', [wruns[2]], [wart[2]], true), [['git', 'push', 'origin', '--delete', 'winqa/t']])
})

test('gatherRuns: по одному gh run list на workflow ветки, прогонам проставлен workflow; пустой ответ — пусто', () => {
  const calls = []
  const gh = (a) => {
    calls.push(a)
    const w = a[a.indexOf('--workflow') + 1]
    if (w === 'ci.yml') return ''
    return JSON.stringify([{ databaseId: w === 'win-qa.yml' ? 7 : 8, createdAt: '2026-10-23T10:00:00Z' }])
  }
  const runs = gatherRuns(gh, 'winqa/t')
  assert.deepEqual(WORKFLOWS, ['win-qa.yml', 'ci.yml', 'release.yml'])
  assert.equal(calls.length, 3)
  assert.ok(calls.every((a) => a[a.indexOf('--branch') + 1] === 'winqa/t'))
  assert.deepEqual(runs.map((r) => [r.databaseId, r.workflow]), [[7, 'win-qa.yml'], [8, 'release.yml']])
  assert.deepEqual(gatherRuns(() => '[]', 'winqa/t'), [])
  assert.throws(() => gatherRuns(() => 'не json', 'winqa/t'))
})

// Полный путь CLI с подставным gh в PATH: печатает команды (без --yes ничего не выполняет)
test('winqa-push --cleanup --keep-last: команды по трём workflow, ничего не выполняется без --yes', { skip: process.platform === 'win32' && 'подставной gh — bash-скрипт' }, () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'winqa-gh-'))
  try {
    const log = path.join(dir, 'log')
    fs.writeFileSync(
      path.join(dir, 'gh'),
      `#!/bin/bash
echo "$*" >> "${log}"
if [ "$1" = run ]; then
  case "$*" in
    *win-qa.yml*) echo '[{"databaseId":1,"createdAt":"2026-10-23T10:00:00Z"},{"databaseId":2,"createdAt":"2026-10-23T11:00:00Z"}]' ;;
    *ci.yml*) echo '[{"databaseId":3,"createdAt":"2026-10-23T09:00:00Z"}]' ;;
    *) echo '[]' ;;
  esac
else
  echo '{"id":201,"run":1,"branch":"winqa/t"}'; echo '{"id":203,"run":3,"branch":"winqa/t"}'
fi
`,
      { mode: 0o755 },
    )
    const script = path.join(path.dirname(new URL(import.meta.url).pathname), 'winqa-push.mjs')
    const out = execFileSync('node', [script, '--cleanup', 't', '--keep-last'], { encoding: 'utf8', env: { ...process.env, PATH: `${dir}${path.delimiter}${process.env.PATH}` } })
    assert.match(out, /gh api -X DELETE repos\/\{owner\}\/\{repo\}\/actions\/artifacts\/201/)
    assert.ok(!out.includes('artifacts/203'), 'артефакт последнего прогона ci остаётся')
    assert.match(out, /gh run delete 1\n/)
    assert.ok(!/gh run delete [23]\n/.test(out))
    assert.match(out, /git push origin --delete winqa\/t/)
    assert.match(out, /печать команд/)
    assert.equal(fs.readFileSync(log, 'utf8').split('\n').filter((l) => l.startsWith('run list')).length, 3)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})
