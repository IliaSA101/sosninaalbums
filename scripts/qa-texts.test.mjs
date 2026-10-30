// Сверка scripts/win/qa/texts.psd1 с исходниками: текст, который ищет UIA, должен быть в продукте (спека 118).
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'

const root = path.resolve(path.dirname(new URL(import.meta.url).pathname.replace(/^\/(\w:)/, '$1')), '..')

// Строки 'ключ' = 'режим: источник' из psd1, комментарии пропускаем
function entries() {
  const text = fs.readFileSync(path.join(root, 'scripts', 'win', 'qa', 'texts.psd1'), 'utf8')
  return [...text.matchAll(/^\s*'([^']+)'\s*=\s*'(точный|префикс)?[^']*'/gm)].map((m) => ({ key: m[1], mode: m[2] }))
}

function sourceBlob() {
  const files = execFileSync('git', ['ls-files', 'frontend/src', 'internal', 'scripts/win/installer.nsi'], { cwd: root, encoding: 'utf8' })
    .split('\n')
    .filter((f) => f && !/\.test\.|\.dom\./.test(f))
  return files.map((f) => fs.readFileSync(path.join(root, f), 'utf8')).join('\n')
}

const esc = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

test('every UIA text exists in sources', () => {
  const list = entries().map((e) => e.key)
  assert.ok(list.length > 0)
  const blob = sourceBlob()
  for (const t of list) assert.ok(blob.includes(t), `нет в исходниках: ${t}`)
})

test('every text has a mode', () => {
  for (const e of entries()) assert.ok(e.mode, `у текста нет режима (точный/префикс): ${e.key}`)
})

// Chromium берёт UIA Name кнопки из содержимого: «точный» текст обязан быть всем содержимым элемента
test('exact UIA names equal whole element text', () => {
  const blob = sourceBlob()
  for (const e of entries().filter((x) => x.mode === 'точный')) {
    const t = esc(e.key)
    const re = new RegExp(`>\\s*${t}\\s*<|'${t}'|"${t}"`)
    assert.ok(re.test(blob), `точный текст не совпадает со всем текстом элемента: ${e.key}`)
  }
})
