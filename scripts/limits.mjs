// Пределы размера файлов и папок — единственный источник (0.13.1, решение владельца 16.10: структура по
// обычаям языка). Читают: scripts/check.ps1 и scripts/git-hooks/pre-commit (CLI), scripts/precheck.mjs и
// .claude/hooks/file-size.js (import). Менять пределы — только здесь и с согласия владельца.
//
// Go: пакет — единица смысла и закрытости, счётчика файлов в папке нет; файл кода — предупреждение 600,
// провал 800 строк, без предела в КБ (кириллица в комментариях — 2 байта); тест — 1200 строк. Сигнал
// «посмотреть разрез пакета» (> 6–8 тыс. строк кода) — дело ревьюера, не провал.
// Фронт: область — не больше 20 файлов кода без тестов; .ts — 600 строк / 30 КБ; .svelte — предупреждение
// 400, провал 600; тест — 900. Подпапок у любой папки — не больше 20. Исключения — scripts/folder-exceptions.txt.
//
// CLI: node scripts/limits.mjs sizes|folders [--cached] — пусто и код 0, если чисто; иначе список и код 1.
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const KB = 1024
// internal/export/flipbook/vendor — чужая минифицированная библиотека (StPageFlip, MIT), правится только заменой версии (владелец 28.10, 0.30-139-4)
const SIZE_SKIP = /(^|\/)(wailsjs|build|docs\/private|docs\/refs|node_modules|dist|internal\/export\/flipbook\/vendor)\/|\.d\.ts$/
const FOLDER_SKIP = /(^|\/)(wailsjs|testdata|node_modules)\/|\.d\.ts$/
const TEST = /(_test\.go|\.test\.(ts|js|mjs))$/

// sizeRule — пределы файла: fail — провал проверки, warn — предупреждение хука при правке; null — не проверяется.
export function sizeRule(file) {
  const f = file.replace(/\\/g, '/')
  if (SIZE_SKIP.test(f) || !/\.(go|ts|js|svelte|css|ps1|py|sh)$/.test(f)) return null
  if (f.endsWith('.go')) return TEST.test(f) ? { fail: { lines: 1200 } } : { fail: { lines: 800 }, warn: { lines: 600 } }
  if (TEST.test(f)) return { fail: { lines: 900 } }
  if (f.endsWith('.svelte')) return { fail: { lines: 600, bytes: 30 * KB }, warn: { lines: 400 } }
  return { fail: { lines: 600, bytes: 30 * KB } }
}

// countLines — как Get-Content: хвостовой перевод строки не считается.
export function countLines(text) {
  return text.length === 0 ? 0 : text.replace(/\r?\n$/, '').split(/\r?\n/).length
}

// over — превышен ли предел lim ({lines, bytes}).
export function over(lim, lines, bytes) {
  return !!lim && ((!!lim.lines && lines > lim.lines) || (!!lim.bytes && bytes > lim.bytes))
}

export const limText = (lim) => `${lim.lines} строк` + (lim.bytes ? ` / ${lim.bytes / KB} КБ` : '')

// checkSizes — нарушения провального предела у файлов (пути от root).
export function checkSizes(files, root) {
  const bad = []
  for (const f of files) {
    const r = sizeRule(f)
    const p = path.join(root, f)
    if (!r || !fs.existsSync(p)) continue
    const buf = fs.readFileSync(p)
    const lines = countLines(buf.toString('utf8'))
    if (over(r.fail, lines, buf.length)) bad.push(`${f} (${lines} строк, ${(buf.length / KB).toFixed(1)} КБ; предел ${limText(r.fail)})`)
  }
  return bad
}

// counted — файл идёт в счёт «не больше 20 в папке»: только код фронта без тестов (Go — пакет по смыслу).
export function counted(file) {
  const f = file.replace(/\\/g, '/')
  return f.startsWith('frontend/') && /\.(ts|svelte)$/.test(f) && !TEST.test(f) && !FOLDER_SKIP.test(f)
}

// exceptions — scripts/folder-exceptions.txt: «<папка> N» — файлы, «<папка>/ N» — подпапки.
export function exceptions(root) {
  const files = {}
  const dirs = {}
  const p = path.join(root, 'scripts', 'folder-exceptions.txt')
  if (!fs.existsSync(p)) return { files, dirs }
  for (const line of fs.readFileSync(p, 'utf8').split(/\r?\n/)) {
    let m = line.match(/^\s*([^\s#]+)\/\s+(\d+)/)
    if (m) dirs[m[1]] = Number(m[2])
    else if ((m = line.match(/^\s*([^\s#]+)\s+(\d+)/))) files[m[1]] = Number(m[2])
  }
  return { files, dirs }
}

// folderOf — папка файла («.» — корень).
export const folderOf = (f) => (f.includes('/') ? f.slice(0, f.lastIndexOf('/')) : '.')

// checkFolders — нарушения по всему дереву tracked (пути от корня, через «/»).
export function checkFolders(tracked, root) {
  const exc = exceptions(root)
  const out = []
  const cnt = {}
  for (const f of tracked.filter(counted)) cnt[folderOf(f)] = (cnt[folderOf(f)] || 0) + 1
  for (const [d, n] of Object.entries(cnt)) {
    const lim = exc.files[d] ?? 20
    if (n > lim) out.push(`${d}: ${n} файлов кода без тестов (макс. ${lim})`)
  }
  const kids = {}
  for (const f of tracked) {
    const parts = f.split('/')
    let parent = '.'
    for (let i = 0; i < parts.length - 1; i++) {
      ;(kids[parent] ||= new Set()).add(parts[i])
      parent = i === 0 ? parts[0] : `${parent}/${parts[i]}`
    }
  }
  for (const [p, s] of Object.entries(kids)) {
    const lim = exc.dirs[p] ?? 20
    if (s.size > lim) out.push(`${p}: ${s.size} подпапок (макс. ${lim})`)
  }
  for (const f of tracked) {
    if (!f.startsWith('frontend/src/') || !/\.(ts|svelte)$/.test(f) || FOLDER_SKIP.test(f)) continue
    const n = f.split('/').length
    if (n !== 3 && n !== 5) out.push(`${folderOf(f)}: код вне frontend/src/<группа>/<область>`)
  }
  return [...new Set(out)].sort()
}

// gitFiles — файлы репозитория: индекс (cached) или индекс + новые неигнорируемые (как check.ps1).
export function gitFiles(root, cachedOnly = false) {
  const args = cachedOnly ? ['ls-files', '--cached'] : ['ls-files', '--cached', '--others', '--exclude-standard']
  return execFileSync('git', args, { cwd: root, encoding: 'utf8', maxBuffer: 64 * KB * KB })
    .split('\n')
    .map((s) => s.trim())
    .filter((f) => f && fs.existsSync(path.join(root, f)))
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
  const [cmd, flag] = process.argv.slice(2)
  const files = gitFiles(root, flag === '--cached')
  const bad = cmd === 'sizes' ? checkSizes(files, root) : cmd === 'folders' ? checkFolders(files, root) : null
  if (!bad) {
    console.error('использование: node scripts/limits.mjs sizes|folders [--cached]')
    process.exit(2)
  }
  if (bad.length) {
    console.log(bad.join('\n'))
    process.exit(1)
  }
}
