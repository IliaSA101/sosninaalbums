// Фильтр winqa-push.mjs на временном репозитории, без сети: коммит-сирота без путей publish-exclude.txt.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
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
