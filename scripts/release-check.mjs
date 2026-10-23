// Проверка перед меткой выпуска: `node scripts/release-check.mjs [--version X.Y.Z] [--root <папка>]`.
// Ловит механику выпуска, которую до сих пор находило только ревью: версия в четырёх местах (wails.json,
// README «Status», «Что нового», CHANGELOG), `needs` публикации в release.yml (0.11), эталоны для arm64,
// LICENSE = GPL v3 и условие 7(b) в NOTICE.md (0.23).
// Без --version берётся productVersion из wails.json (тогда проверяется согласованность остальных мест).
// Выход ≠ 0 и список ошибок.
import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const read = (root, rel) => {
  const f = path.join(root, rel)
  return fs.existsSync(f) ? fs.readFileSync(f, 'utf8').replace(/\r\n/g, '\n') : null
}

// ---- версия в четырёх местах ----

export function checkVersions(root, version) {
  const errors = []
  const need = (what, got) => {
    if (got !== version) errors.push(`${what}: ${got ?? 'не найдено'}, ждали ${version}`)
  }
  const wails = read(root, 'wails.json')
  if (wails === null) errors.push('wails.json: нет файла')
  else {
    try {
      need('wails.json → info.productVersion', JSON.parse(wails)?.info?.productVersion)
    } catch (e) {
      errors.push(`wails.json: не разобран (${e.message})`)
    }
  }
  const readme = read(root, 'README.md')
  need('README.md «- Status: version …»', readme?.match(/^- Status: version (\S+?),/m)?.[1])
  const wn = read(root, 'frontend/src/shell/dialogs/whatsnew.ts')
  need('whatsnew.ts → WHATS_NEW_VERSION', wn?.match(/WHATS_NEW_VERSION\s*=\s*'([^']+)'/)?.[1])
  if (wn !== null && !/export const WHATS_NEW[^=]*=\s*\[\s*['"`]/.test(wn)) errors.push('whatsnew.ts: список WHATS_NEW пуст')
  return errors
}

// ---- CHANGELOG ----

// Тело раздела «## X.Y.Z …» до следующего «## » (null — раздела нет).
export function changelogSection(text, version) {
  const lines = text.split('\n')
  const esc = version.replace(/\./g, '\\.')
  const start = lines.findIndex((l) => new RegExp(`^## ${esc}(\\s|$)`).test(l))
  if (start < 0) return null
  let end = lines.findIndex((l, i) => i > start && /^## /.test(l))
  if (end < 0) end = lines.length
  return lines.slice(start + 1, end).join('\n')
}

export function checkChangelog(root, version) {
  const text = read(root, 'CHANGELOG.md')
  if (text === null) return ['CHANGELOG.md: нет файла']
  const body = changelogSection(text, version)
  if (body === null) return [`CHANGELOG.md: нет раздела «## ${version}»`]
  const errors = []
  if (!/^- \S/m.test(body)) errors.push(`CHANGELOG.md: раздел ${version} без пунктов «- …»`)
  const top = text.match(/^## (\d+\.\d+\.\d+)/m)?.[1]
  if (top && top !== version) errors.push(`CHANGELOG.md: первым идёт раздел ${top}, а не ${version}`)
  errors.push(...specErrors(root, version, body))
  return errors
}

// Спеки версии (docs/specs/<версия>/NNN-*.md): каждая есть в таблице docs/specs/README.md, а если CHANGELOG
// уже ссылается на номера спек этой версии, то на все. CHANGELOG пишется языком пользователя и обычно номеров
// не содержит — тогда сверка с ним молчит (иначе вечное предупреждение; ноль warnings — правило проекта).
// Папка — по полной версии (0.13.1), для X.Y.0 — «X.Y»; у патча без своей папки спеки не проверяются.
export function specErrors(root, version, body) {
  const [maj, min, pat] = version.split('.')
  const dir = [version, pat === '0' ? `${maj}.${min}` : null]
    .filter(Boolean)
    .find((d) => fs.existsSync(path.join(root, 'docs/specs', d)))
  if (!dir) return []
  const nums = fs
    .readdirSync(path.join(root, 'docs/specs', dir), { withFileTypes: true })
    .filter((e) => e.isFile() && /^\d{3}-.+\.md$/.test(e.name))
    .map((e) => e.name.slice(0, 3))
  const table = read(root, 'docs/specs/README.md') ?? ''
  const cited = nums.filter((n) => new RegExp(`\\b${n}\\b`).test(body))
  const errors = []
  for (const n of nums) {
    if (!new RegExp(`^\\| ${n} \\|`, 'm').test(table)) errors.push(`спека ${n} (docs/specs/${dir}) не в таблице docs/specs/README.md`)
    if (cited.length > 0 && !cited.includes(n)) errors.push(`спека ${n} (docs/specs/${dir}) не упомянута в CHANGELOG ${version}`)
  }
  return errors
}

// ---- лицензия ----

// sha256 официального текста GPL v3 (https://www.gnu.org/licenses/gpl-3.0.txt, концы строк — LF). Любая правка
// LICENSE меняет сумму: GitHub (licensee) узнаёт GPL-3.0 только по чистому тексту, без строчки сверху и снизу.
export const GPL3_SHA256 = '3972dc9744f6499f0f9b2dbf76696f2ae7ad8af9b23dde66d6af86c9dfb36986'

// Строки условия п. 7(b) в NOTICE.md: без любой из них изменённые версии не обязаны называть автора.
export const NOTICE_MUST = [
  ['GPL-3.0-only', 'GPL-3.0-only'],
  ['п. 7(b)', '7(b)'],
  ['строка «Основано на Sosnina Albums, автор Илья Соснин»', 'Основано на Sosnina Albums, автор Илья Соснин'],
  ['адрес репозитория', 'https://github.com/IliaSA101/sosninaalbums'],
]

export function checkLicense(root) {
  const errors = []
  const lic = read(root, 'LICENSE')
  if (lic === null) errors.push('LICENSE: нет файла')
  else if (crypto.createHash('sha256').update(lic).digest('hex') !== GPL3_SHA256) {
    errors.push('LICENSE: не точный текст GNU GPL v3 (сверьте с https://www.gnu.org/licenses/gpl-3.0.txt, правок быть не должно)')
  }
  const notice = read(root, 'NOTICE.md')
  if (notice === null) errors.push('NOTICE.md: нет файла')
  else for (const [what, needle] of NOTICE_MUST) if (!notice.includes(needle)) errors.push(`NOTICE.md: нет условия 7(b): ${what}`)
  return errors
}

// ---- release.yml ----

// Задания и их needs без YAML-библиотеки: ключи заданий — два пробела под «jobs:», needs — строкой или списком.
export function parseJobs(yml) {
  const jobs = {}
  let cur = null
  let inJobs = false
  let inNeedsBlock = false
  for (const line of yml.split('\n')) {
    if (/^jobs:\s*$/.test(line)) {
      inJobs = true
      continue
    }
    if (!inJobs) continue
    if (/^\S/.test(line) && !/^#/.test(line)) break // следующий верхний ключ
    const job = /^ {2}([A-Za-z0-9_-]+):\s*$/.exec(line)
    if (job) {
      cur = job[1]
      jobs[cur] = []
      inNeedsBlock = false
      continue
    }
    if (!cur) continue
    const inline = /^ {4}needs:\s*(.*?)\s*(#.*)?$/.exec(line)
    if (inline) {
      const v = inline[1]
      inNeedsBlock = v === ''
      if (v) jobs[cur].push(...v.replace(/^\[|\]$/g, '').split(',').map((s) => s.trim()).filter(Boolean))
      continue
    }
    if (inNeedsBlock) {
      const item = /^ {6}- (\S+)/.exec(line)
      if (item) jobs[cur].push(item[1])
      else if (line.trim() && !/^\s*#/.test(line)) inNeedsBlock = false
    }
  }
  return jobs
}

// Публикация (задание `release`) ждёт все остальные задания: напрямую или через цепочку needs.
export function checkNeeds(yml, publish = 'release') {
  const jobs = parseJobs(yml)
  if (!(publish in jobs)) return [`release.yml: нет задания «${publish}»`]
  const seen = new Set()
  const walk = (j) => {
    for (const n of jobs[j] ?? []) if (!seen.has(n)) (seen.add(n), walk(n))
  }
  walk(publish)
  const missing = Object.keys(jobs).filter((j) => j !== publish && !seen.has(j))
  const unknown = [...seen].filter((n) => !(n in jobs))
  return [
    ...missing.map((j) => `release.yml: «${publish}» не ждёт задание «${j}» (needs, в том числе через цепочку)`),
    ...unknown.map((n) => `release.yml: needs ссылается на несуществующее задание «${n}»`),
  ]
}

// Эталоны для arm64: проверки из mac-check (`go test … -run '…'`) указывают на существующие тесты, а папки
// эталонов упомянутых пакетов содержат непустые *.sha.
export function checkArm64(root, yml) {
  const errors = []
  const tests = new Set()
  const pkgDirs = new Set()
  const walk = (dir) => {
    for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
      const p = path.join(dir, e.name)
      if (e.isDirectory()) walk(p)
      else if (e.name.endsWith('_test.go')) {
        for (const m of fs.readFileSync(p, 'utf8').matchAll(/^func (Test\w+)\(/gm)) tests.add(m[1])
      }
    }
  }
  if (fs.existsSync(path.join(root, 'internal/layout'))) walk(path.join(root, 'internal/layout'))
  // команды вида «go test <пакеты> -run '<re>'» (строки могут переноситься только после «||»)
  const cmds = [...yml.matchAll(/go test ([^\n]*?) -run '([^']+)'/g)]
  if (cmds.length === 0) errors.push('release.yml: нет проверки эталонов раскладок на arm64 (go test … -run …)')
  for (const [, pkgs, re] of cmds) {
    for (const t of re.match(/Test\w+/g) ?? []) if (!tests.has(t)) errors.push(`release.yml: тест «${t}» из проверки arm64 не найден в internal/layout`)
    for (const p of pkgs.split(/\s+/).filter((s) => s.startsWith('./'))) pkgDirs.add(p.slice(2))
  }
  for (const d of pkgDirs) {
    const g = path.join(root, d, 'testdata/golden')
    const files = fs.existsSync(g) ? fs.readdirSync(g).filter((f) => f.endsWith('.sha')) : []
    if (files.length === 0) errors.push(`${d}/testdata/golden: нет эталонов *.sha для проверки на arm64`)
    for (const f of files) if (fs.statSync(path.join(g, f)).size === 0) errors.push(`${d}/testdata/golden/${f}: пустой эталон`)
  }
  return errors
}

// ---- ролики и снимок README ----

export const WEBP_MAX = 2621440 // 2,5 МБ
export const JPG_MAX = 409600 // 400 КБ

// Заставка, снимок и адреса роликов A/B: заглушка TODO-README-MEDIA не должна доехать до выпуска.
export function checkReadmeMedia(root) {
  const errors = []
  const readme = read(root, 'README.md')
  if (readme === null) return ['README.md: нет файла']
  if (readme.includes('TODO-README-MEDIA')) errors.push('README.md: остался TODO-README-MEDIA (адреса роликов не подставлены)')
  const counts = new Map()
  for (const m of readme.matchAll(/https:\/\/github\.com\/user-attachments\/assets\/([0-9a-f-]{36})(?![0-9a-f-])/g)) counts.set(m[1], (counts.get(m[1]) ?? 0) + 1)
  if (counts.size !== 2) errors.push(`README.md: адресов роликов user-attachments ${counts.size}, ждали 2 разных`)
  for (const [id, n] of counts) if (n !== 2) errors.push(`README.md: адрес ролика ${id} встречается ${n} раз, ждали 2 (английская и русская часть)`)
  if (readme.includes('docs/screenshot.png')) errors.push('README.md: ссылка на docs/screenshot.png — снимок теперь docs/media/screenshot.jpg')
  for (const [rel, max] of [['docs/media/demo.webp', WEBP_MAX], ['docs/media/screenshot.jpg', JPG_MAX], ['docs/media/CREDITS.md', 0]]) {
    const f = path.join(root, rel)
    if (!fs.existsSync(f)) errors.push(`${rel}: нет файла`)
    else if (max && fs.statSync(f).size > max) errors.push(`${rel}: ${fs.statSync(f).size} байт, больше предела ${max}`)
  }
  return errors
}

// ---- всё вместе ----

export function checkAll(root, version) {
  const errors = []
  const wails = read(root, 'wails.json')
  if (!version) {
    try {
      version = JSON.parse(wails).info.productVersion
    } catch {
      return { version: '', errors: ['wails.json: не прочитать productVersion; укажите --version'] }
    }
  }
  if (!/^\d+\.\d+\.\d+$/.test(version)) return { version, errors: [`версия «${version}» — ждали X.Y.Z`] }
  errors.push(...checkVersions(root, version))
  errors.push(...checkChangelog(root, version))
  errors.push(...checkLicense(root))
  errors.push(...checkReadmeMedia(root))
  const yml = read(root, '.github/workflows/release.yml')
  if (yml === null) errors.push('.github/workflows/release.yml: нет файла')
  else errors.push(...checkNeeds(yml), ...checkArm64(root, yml))
  return { version, errors }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const argv = process.argv.slice(2)
  const opt = (n) => (argv.includes(n) ? argv[argv.indexOf(n) + 1] : '')
  const root = path.resolve(opt('--root') || path.join(path.dirname(fileURLToPath(import.meta.url)), '..'))
  const { version, errors } = checkAll(root, opt('--version'))
  if (errors.length) {
    console.error(`release-check ${version}: ${errors.length} ошибок`)
    for (const e of errors) console.error(`  - ${e}`)
    process.exit(1)
  }
  console.log(`release-check ${version}: OK`)
}
