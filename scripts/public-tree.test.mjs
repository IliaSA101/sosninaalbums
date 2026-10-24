// Публичное дерево на временном репозитории, без сети: исключённые пути убраны, остальные на месте, --out только в .gotmp.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { checkOut, execPaths, exportPublic, filterIndex, isExcluded, parseExclude, publicTree } from './public-tree.mjs'

const script = path.join(path.dirname(new URL(import.meta.url).pathname.replace(/^\/(\w:)/, '$1')), 'public-tree.mjs')
const EXCLUDE = '# x\ndocs/qa/\ndocs/BACKLOG.md\n\n'

function makeRepo() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'public-tree-'))
  const g = (...a) => execFileSync('git', ['-c', 'user.name=T', '-c', 'user.email=t@example.invalid', ...a], { cwd: dir, encoding: 'utf8' }).trim()
  g('init', '-q')
  const put = (p, t) => {
    fs.mkdirSync(path.dirname(path.join(dir, p)), { recursive: true })
    fs.writeFileSync(path.join(dir, p), t)
  }
  put('scripts/publish-exclude.txt', EXCLUDE)
  put('docs/qa/update.md', 'x')
  put('docs/qa/deep/a.md', 'x')
  put('docs/BACKLOG.md', 'x')
  put('docs/keep.md', 'x')
  put('docs/qa-notes.md', 'имя похоже на docs/qa, но не внутри папки')
  put('run.sh', '#!/bin/sh\n')
  put('.gitignore', '.gotmp/\nignored.txt\n')
  fs.chmodSync(path.join(dir, 'run.sh'), 0o755)
  g('add', '-A')
  g('update-index', '--chmod=+x', 'run.sh')
  g('commit', '-q', '-m', 'one')
  return { dir, g, put }
}
const rm = (dir) => fs.rmSync(dir, { recursive: true, force: true })
const files = (g, tree) => g('ls-tree', '-r', '--name-only', tree).split('\n').filter(Boolean).sort()
const gitIn = (dir) => (a, env = {}) =>
  execFileSync('git', a, { cwd: dir, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'], env: { ...process.env, ...env } }).trim()
const KEPT = ['.gitignore', 'docs/keep.md', 'docs/qa-notes.md', 'run.sh', 'scripts/publish-exclude.txt']

test('parseExclude и isExcluded: комментарии, папка и файл, похожее имя не исключается', () => {
  const ex = parseExclude(EXCLUDE)
  assert.deepEqual(ex, ['docs/qa/', 'docs/BACKLOG.md'])
  assert.equal(isExcluded(ex, 'docs/qa/a/b.md'), true)
  assert.equal(isExcluded(ex, 'docs/BACKLOG.md'), true)
  assert.equal(isExcluded(ex, 'docs/qa-notes.md'), false)
  assert.equal(isExcluded(ex, 'docs/BACKLOG.md.bak'), false)
  assert.deepEqual(parseExclude(''), [])
  assert.equal(isExcluded([], 'any'), false)
})

test('publicTree: исключённые убраны, остальные на месте, число убранного', () => {
  const { dir, g } = makeRepo()
  try {
    const { tree, dropped } = publicTree(gitIn(dir), 'HEAD', EXCLUDE)
    assert.deepEqual(files(g, tree), KEPT)
    assert.equal(dropped, 3)
    // пустой список исключений — дерево коммита как есть
    const all = publicTree(gitIn(dir), 'HEAD', '# ничего\n')
    assert.equal(all.tree, g('rev-parse', 'HEAD^{tree}'))
    assert.equal(all.dropped, 0)
  } finally {
    rm(dir)
  }
})

test('filterIndex: возвращает остаток и убранное; ошибка git (нет ref) не глотается', () => {
  const { dir } = makeRepo()
  try {
    const idx = path.join(dir, '.git', 'tmp-index')
    const gi = (a) => gitIn(dir)(a, { GIT_INDEX_FILE: idx })
    const r = filterIndex(gi, 'HEAD', parseExclude(EXCLUDE))
    assert.deepEqual(r.files.sort(), KEPT)
    assert.deepEqual(r.drop.sort(), ['docs/BACKLOG.md', 'docs/qa/deep/a.md', 'docs/qa/update.md'])
    assert.throws(() => filterIndex(gi, 'no-such-ref', []))
  } finally {
    rm(dir)
  }
})

test('checkOut: только внутри .gotmp; пустое значение, сам .gotmp и выход вверх — отказ', () => {
  const repo = path.resolve('/r')
  assert.equal(checkOut(repo, '.gotmp/public-tree'), path.join(repo, '.gotmp', 'public-tree'))
  for (const bad of ['', '   ', undefined, '.gotmp', '.', '..', '.gotmp/../x', '/tmp/x', 'x']) {
    assert.throws(() => checkOut(repo, bad), /--out/, String(bad))
  }
})

test('exportPublic по ref: дерево публичное, режим +x сохранён, есть свой git-индекс; старая папка пересоздана', () => {
  const { dir } = makeRepo()
  try {
    const out = path.join(dir, '.gotmp', 'pt')
    fs.mkdirSync(out, { recursive: true })
    fs.writeFileSync(path.join(out, 'stale.txt'), 'x')
    const n = exportPublic(dir, '.gotmp/pt', 'HEAD')
    assert.equal(n, KEPT.length)
    assert.equal(fs.existsSync(path.join(out, 'stale.txt')), false)
    assert.equal(fs.existsSync(path.join(out, 'docs', 'qa')), false)
    assert.equal(gitIn(out)(['ls-files', '-s', 'run.sh']).slice(0, 6), '100755')
  } finally {
    rm(dir)
  }
})

// Имитация Windows: git init даёт core.fileMode=false (переменные окружения git действуют и на репозиторий в выходной папке).
function withoutFileMode(fn) {
  const keys = { GIT_CONFIG_COUNT: '1', GIT_CONFIG_KEY_0: 'core.fileMode', GIT_CONFIG_VALUE_0: 'false' }
  const old = Object.fromEntries(Object.keys(keys).map((k) => [k, process.env[k]]))
  Object.assign(process.env, keys)
  try {
    return fn()
  } finally {
    for (const [k, v] of Object.entries(old)) if (v === undefined) delete process.env[k]
    else process.env[k] = v
  }
}

test('exportPublic по ref при core.fileMode=false: режим 100755 берётся из дерева, остальные 100644', () => {
  const { dir, put, g } = makeRepo()
  try {
    put('bin/a.sh', '#!/bin/sh\n')
    put('bin/b.sh', '#!/bin/sh\n')
    put('bin/c.txt', 'x')
    g('add', '-A')
    g('update-index', '--chmod=+x', 'bin/a.sh', 'bin/b.sh')
    g('commit', '-q', '-m', 'two')
    const out = path.join(dir, '.gotmp', 'pt')
    withoutFileMode(() => exportPublic(dir, '.gotmp/pt', 'HEAD'))
    const modes = Object.fromEntries(
      gitIn(out)(['ls-files', '-s']).split('\n').map((l) => [l.split('\t')[1], l.slice(0, 6)]),
    )
    assert.equal(modes['run.sh'], '100755')
    assert.equal(modes['bin/a.sh'], '100755')
    assert.equal(modes['bin/b.sh'], '100755')
    assert.equal(modes['bin/c.txt'], '100644')
    assert.equal(modes['docs/keep.md'], '100644')
  } finally {
    rm(dir)
  }
})

test('exportPublic рабочей копии при core.fileMode=false: режим tracked-файла из индекса, новый исполняемый тоже', () => {
  const { dir, put } = makeRepo()
  try {
    put('new.sh', '#!/bin/sh\n')
    fs.chmodSync(path.join(dir, 'new.sh'), 0o755)
    // у tracked run.sh бит на диске сброшен (как на Windows) — режим всё равно из индекса
    fs.chmodSync(path.join(dir, 'run.sh'), 0o644)
    const out = path.join(dir, '.gotmp', 'wc')
    withoutFileMode(() => exportPublic(dir, '.gotmp/wc'))
    const ls = gitIn(out)(['ls-files', '-s']).split('\n')
    const mode = (f) => ls.find((l) => l.endsWith('\t' + f))?.slice(0, 6)
    assert.equal(mode('run.sh'), '100755')
    assert.equal(mode('docs/keep.md'), '100644')
    if (process.platform !== 'win32') assert.equal(mode('new.sh'), '100755')
  } finally {
    rm(dir)
  }
})

test('execPaths: только 100755, стадия 0, путь с пробелом и табом после первого разделителя; пусто и мусор — пусто', () => {
  const lsTree = '100755 blob aaa\ta b.sh\x00100644 blob bbb\tc.txt\x00120000 blob ccc\tlink\x00'
  assert.deepEqual(execPaths(lsTree), ['a b.sh'])
  const lsFiles = '100755 aaa 0\tx.sh\x00100755 bbb 2\tconf.sh\x00100644 ccc 0\ty\x00'
  assert.deepEqual(execPaths(lsFiles, '\t', true), ['x.sh'])
  assert.deepEqual(execPaths(''), [])
  assert.deepEqual(execPaths('мусор без разделителя\0'), [])
})

test('exportPublic рабочей копии: новые файлы есть, игнорируемые и удалённые — нет, исключённые убраны', () => {
  const { dir, put } = makeRepo()
  try {
    put('new.txt', 'x') // не в индексе
    put('ignored.txt', 'x')
    put('docs/qa/fresh.md', 'x')
    fs.rmSync(path.join(dir, 'docs', 'keep.md')) // в индексе есть, на диске нет
    const n = exportPublic(dir, '.gotmp/wc')
    const got = gitIn(path.join(dir, '.gotmp', 'wc'))(['ls-files']).split('\n').sort()
    assert.deepEqual(got, ['.gitignore', 'docs/qa-notes.md', 'new.txt', 'run.sh', 'scripts/publish-exclude.txt'])
    assert.equal(n, 5)
  } finally {
    rm(dir)
  }
})

test('CLI: --out вне .gotmp и пустой — код 2, без --out — код 2; корректный запуск печатает число файлов', () => {
  const { dir } = makeRepo()
  try {
    const run = (...a) => {
      try {
        return { code: 0, out: execFileSync('node', [script, '--repo', dir, ...a], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }) }
      } catch (e) {
        return { code: e.status, out: String(e.stderr) }
      }
    }
    assert.equal(run('--out', os.tmpdir()).code, 2)
    assert.equal(run('--out', '').code, 2)
    assert.equal(run().code, 2)
    const ok = run('--ref', 'HEAD', '--out', '.gotmp/cli')
    assert.equal(ok.code, 0)
    assert.match(ok.out, /5 файлов/)
  } finally {
    rm(dir)
  }
})

test('дерево совпадает с деревом коммита winqa-push для того же ref', () => {
  const { dir, g } = makeRepo()
  try {
    const env = { ...process.env, GIT_AUTHOR_NAME: 'T', GIT_AUTHOR_EMAIL: 't@example.invalid', GIT_COMMITTER_NAME: 'T', GIT_COMMITTER_EMAIL: 't@example.invalid' }
    const push = path.join(path.dirname(script), 'winqa-push.mjs')
    const out = execFileSync('node', [push, '--repo', dir, 'same'], { encoding: 'utf8', env })
    const sha = out.match(/origin ([0-9a-f]{40}):/)[1]
    assert.equal(g('rev-parse', `${sha}^{tree}`), publicTree(gitIn(dir), 'HEAD', EXCLUDE).tree)
  } finally {
    rm(dir)
  }
})
