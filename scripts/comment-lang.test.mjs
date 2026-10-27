// Тесты проверки языка комментариев: `node --test scripts/`.
// Английские образцы собираются из частей (SL, HT), чтобы сам тест не ловился проверкой.
import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { collect, englishComment, isCodeFile, parseDiff, violations } from './comment-lang.mjs'

const SL = '/' + '/ '
const HT = '<' + '!-- '
const EN = 'this function returns the current value'

test('русский комментарий — ок', () => {
  assert.equal(englishComment(`${SL}возвращает текущее значение из хранилища`, 'a.go'), null)
  assert.equal(englishComment(`${SL}returns the значение from store`, 'a.ts'), null)
})

test('английская фраза — провал в Go, TS, Svelte (HTML), CSS, ps1', () => {
  assert.equal(englishComment(`\tx := 1 ${SL}${EN}`, 'a.go'), EN)
  assert.equal(englishComment(`${SL}${EN}`, 'a.ts'), EN)
  assert.equal(englishComment(`${HT}${EN} -->`, 'a.svelte'), EN)
  assert.equal(englishComment(` * ${EN}`, 'a.css'), EN)
  assert.equal(englishComment(`# ${EN}`, 'a.ps1'), EN)
  assert.equal(englishComment(`# ${EN}`, 'a.sh'), EN)
  assert.equal(englishComment(`# ${EN}`, 'a.go'), null) // # вне ps1/py/sh — не комментарий
})

test('директивы, URL, короткие, коды — ок', () => {
  for (const c of [
    `${SL}go:embed all:frontend/dist`,
    `${SL}nolint:gosec this is intentionally unchecked here`,
    `${SL}eslint-disable-next-line no-console and some more words`,
    `${SL}@ts-expect-error this is expected to fail here`,
    `${SL}prettier-ignore`,
    `${SL}export Foo with some more words here`,
    `${SL}see https://example.com/some/long/path/with/words/inside`,
    `${SL}short two words`,
    `${SL}fn func(*Project) error → View`,
    `${SL}back | front | spine | cover`,
    `${SL}x = a + b + c + d + e`,
  ]) {
    assert.equal(englishComment(c, 'a.go'), null, c)
  }
  assert.equal(englishComment(`const u = 'http://example.com' + path + other + words`, 'a.ts'), null)
})

test('.sh: русский комментарий, shebang, директива shellcheck и lang-ok — ок; файл .sh проверяется', () => {
  assert.equal(englishComment('# запускает сборку для текущей системы', 'a.sh'), null)
  assert.equal(englishComment('#!/usr/bin/env bash', 'a.sh'), null)
  assert.equal(englishComment('# shellcheck disable SC2086 because word splitting is wanted', 'a.sh'), null)
  assert.equal(englishComment(`# ${EN} lang-ok`, 'a.sh'), null)
  assert.equal(englishComment(`# ${EN}`, 'scripts/mac/shots.sh'), EN)
  assert.equal(violations([{ file: 'scripts/b.sh', line: 3, text: `# ${EN}` }]).length, 1)
  assert.equal(violations([{ file: 'scripts/b.sh', line: 3, text: 'x=1' }]).length, 0)
})

test('маркер lang-ok снимает строку', () => {
  assert.equal(englishComment(`${SL}${EN} lang-ok`, 'a.go'), null)
})

test('isCodeFile: wailsjs, node_modules, dist, bin, .gotmp, testdata — мимо', () => {
  for (const f of ['frontend/wailsjs/go/app/App.js', 'frontend/node_modules/x/a.js', 'frontend/dist/a.js', 'build/bin/a.ps1', '.gotmp/a.go', 'testdata/a.go']) {
    assert.equal(isCodeFile(f), false, f)
  }
  for (const f of ['a.go', 'frontend/src/a.svelte', 'scripts/a.mjs', 'x/a.css', 'a.py', 'a.ps1', 'a.ts', 'a.js', 'scripts/a.sh']) {
    assert.equal(isCodeFile(f), true, f)
  }
  assert.equal(isCodeFile('README.md'), false)
})

test('violations: wailsjs мимо, номера строк из диффа', () => {
  const diff = [
    'diff --git a/frontend/wailsjs/x.js b/frontend/wailsjs/x.js',
    '--- a/frontend/wailsjs/x.js',
    '+++ b/frontend/wailsjs/x.js',
    '@@ -1,0 +1,1 @@',
    `+${SL}${EN}`,
    'diff --git a/src/a.go b/src/a.go',
    '--- a/src/a.go',
    '+++ b/src/a.go',
    '@@ -3,0 +10,2 @@',
    '+var x = 1',
    `+${SL}${EN}`,
    '@@ -20 +30 @@',
    `-${SL}${EN}`, // удалённая строка не считается
    `+${SL}добавили по-русски`,
  ].join('\n')
  assert.deepEqual(parseDiff(diff).map((l) => [l.file, l.line]), [
    ['frontend/wailsjs/x.js', 1],
    ['src/a.go', 10],
    ['src/a.go', 11],
    ['src/a.go', 30],
  ])
  assert.deepEqual(violations(parseDiff(diff)), [{ file: 'src/a.go', line: 11, text: EN }])
})

// Интеграция с git: старое английское не трогаем, новое (закоммиченное, правка, новый файл) — ловим.
test('collect: режим по диффу не трогает старое, ловит новое', () => {
  const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
  const tmp = path.join(root, '.gotmp')
  fs.mkdirSync(tmp, { recursive: true })
  const repo = fs.mkdtempSync(path.join(tmp, 'comment-lang-'))
  const env = { ...process.env, GIT_AUTHOR_NAME: 't', GIT_AUTHOR_EMAIL: 't@t', GIT_COMMITTER_NAME: 't', GIT_COMMITTER_EMAIL: 't@t' }
  const git = (...a) => {
    const r = spawnSync('git', a, { cwd: repo, env, encoding: 'utf8' })
    assert.equal(r.status, 0, r.stderr)
    return r.stdout
  }
  try {
    git('init', '-q', '-b', 'dev')
    git('config', 'core.autocrlf', 'false') // не зависеть от CRLF-настроек машины
    fs.writeFileSync(path.join(repo, 'old.go'), `package a\n${SL}${EN}\n`)
    git('add', '.')
    git('commit', '-q', '-m', 'base')
    git('checkout', '-q', '-b', 'work')
    fs.writeFileSync(path.join(repo, 'new.go'), `package b\n${SL}${EN}\n`)
    git('add', '.')
    git('commit', '-q', '-m', 'new')
    fs.appendFileSync(path.join(repo, 'old.go'), `var x = 1 ${SL}${EN} again\n`) // незакоммиченная правка
    fs.writeFileSync(path.join(repo, 'fresh.ts'), `${SL}${EN} fresh\n`) // неотслеживаемый файл
    const got = violations(collect({ root: repo, base: '', staged: false, all: false }))
      .map((v) => `${v.file}:${v.line}`)
      .sort()
    assert.deepEqual(got, ['fresh.ts:1', 'new.go:2', 'old.go:3'])
    // --all видит и старое
    const all = violations(collect({ root: repo, base: '', staged: false, all: true })).map((v) => `${v.file}:${v.line}`)
    assert.ok(all.includes('old.go:2'))
    // --staged: только индекс
    git('add', 'old.go')
    const st = violations(collect({ root: repo, base: '', staged: true, all: false })).map((v) => `${v.file}:${v.line}`)
    assert.deepEqual(st, ['old.go:3'])
    // чистый дифф — пусто
    git('reset', '-q', '--hard')
    fs.rmSync(path.join(repo, 'fresh.ts'))
    git('checkout', '-q', 'dev')
    assert.deepEqual(violations(collect({ root: repo, base: '', staged: false, all: false })), [])
  } finally {
    fs.rmSync(repo, { recursive: true, force: true })
  }
})
