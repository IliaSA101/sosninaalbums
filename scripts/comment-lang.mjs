// Английские комментарии — стоп (владелец, правило 0.16): `node scripts/comment-lang.mjs [--base <rev>] [--staged] [--all]`.
// Берёт ДОБАВЛЕННЫЕ строки диффа (база...HEAD + незакоммиченное + новые файлы) и падает (exit 1) со списком файл:строка,
// если в них есть комментарий-фраза латиницей без кириллицы (от 4 слов). Режим `--all` — те же правила по всему дереву
// (отчёт о наследии; в работе не используется). Исключение строки — маркер `lang-ok` в комментарии.
// Распознавание скопировано из .claude/hooks/russian-guard.mjs (хук вне git — не импортируем): директивы Go/ESLint/TS,
// URL, коды и перечни значений не считаются фразами.
import { spawnSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const MIN_WORDS = 4 // комментарий из стольких латинских слов без кириллицы — английская фраза
export const MARKER = 'lang-ok'
const CODE_EXT = /\.(go|svelte|m?js|cjs|ts|css|ps1|py)$/i
const SKIP_PATH = /(^|[\\/])(wailsjs|node_modules|dist|bin|\.git|\.gotmp|testdata)[\\/]/i

export const isCodeFile = (f) => CODE_EXT.test(f) && !SKIP_PATH.test(f)

// Текст без кода, ссылок и путей — то, что считается словами.
function prose(text) {
  return String(text || '')
    .replace(/`[^`\n]*`/g, ' ')
    .replace(/https?:\/\/\S+/g, ' ')
    .replace(/(?:[A-Za-z]:)?[\\/]?(?:[\w.-]+[\\/])+[\w.-]*/g, ' ')
}

// englishComment — текст комментария одной строки, если это английская фраза; иначе null.
export function englishComment(raw, file = '') {
  const line = raw.trim()
  if (line.includes(MARKER)) return null
  let c = null
  const sl = line.match(/(?:^|\s)\/\/\s?(.*)$/)
  if (sl && !/https?:$/.test(line.slice(0, line.indexOf('//')))) c = sl[1]
  else if (/^\/\*+/.test(line)) c = line.replace(/^\/\*+/, '').replace(/\*+\/$/, '')
  else if (/^\*(\s|$)/.test(line)) c = line.replace(/^\*\s?/, '').replace(/\*+\/$/, '')
  else if (/\.(py|sh|ps1)$/i.test(file) && /^#(?![[!])/.test(line)) c = line.replace(/^#+\s?/, '')
  else if (/^<!--|<!--.*-->/.test(line)) c = line.replace(/^.*?<!--/, '').replace(/-->.*$/, '')
  if (c === null) return null
  c = c.trim()
  if (!c || /^[@#]|^(eslint|prettier|svelte-ignore|go:|nolint|export\s|line\s|noinspection|@ts-|istanbul|language=|region|endregion)/i.test(c)) return null
  if (/[А-Яа-яЁё]/.test(c)) return null
  // Продолжение описания типа в docblock: `sum: string, url: string}` — не фраза
  if (/[{}]/.test(c) || /\b\w+\??:\s*[\w\\|<>[\]?]+\s*[,}]/.test(c)) return null
  // Подпись типов со стрелкой (`fn func(*P) error → View`) — не фраза
  if (/→|=>|->/.test(c)) return null
  // Перечень значений, пример команды или присваивания — не фраза
  if (/\w\s*\|\s*\w[^|]*\|/.test(c) || /[=;]/.test(c) || /^(powershell|pwsh|node|go|git|npm|npx|wails|python)\b/i.test(c)) return null
  const words = prose(c).match(/\b[A-Za-z]{3,}\b/g) || []
  return words.length >= MIN_WORDS ? c.slice(0, 120) : null
}

// parseDiff — добавленные строки unified-диффа (-U0): [{file, line, text}].
export function parseDiff(diff) {
  const out = []
  let file = ''
  let line = 0
  for (const l of String(diff).split('\n')) {
    if (l.startsWith('+++ ')) {
      const m = l.match(/^\+\+\+ (?:b\/)?(.*)$/)
      file = m && m[1] !== '/dev/null' ? m[1].replace(/^"|"$/g, '') : ''
    } else if (l.startsWith('@@')) {
      const m = l.match(/\+(\d+)/)
      line = m ? Number(m[1]) : 0
    } else if (l.startsWith('+') && file) out.push({ file, line: line++, text: l.slice(1) })
  }
  return out
}

// violations — [{file, line, text}] английских комментариев среди строк.
export function violations(lines) {
  const res = []
  for (const { file, line, text } of lines) {
    if (!isCodeFile(file)) continue
    const c = englishComment(text, file)
    if (c) res.push({ file, line, text: c })
  }
  return res
}

const wholeFile = (file, root) =>
  fs
    .readFileSync(path.join(root, file), 'utf8')
    .split('\n')
    .map((text, i) => ({ file, line: i + 1, text: text.replace(/\r$/, '') }))

// collect — добавленные строки по режиму; git вызывается из корня репозитория.
export function collect({ root, base, staged, all }) {
  const git = (...a) => {
    const r = spawnSync('git', a, { cwd: root, encoding: 'utf8', maxBuffer: 1 << 28 })
    return { code: r.status ?? 1, out: r.stdout || '' }
  }
  const files = git('ls-files', '--cached', '--others', '--exclude-standard')
    .out.split('\n')
    .map((s) => s.trim())
  if (all) return files.filter((f) => f && isCodeFile(f) && fs.existsSync(path.join(root, f))).flatMap((f) => wholeFile(f, root))
  if (staged) return parseDiff(git('diff', '--cached', '-U0', '--no-color').out)
  let b = base
  if (!b) {
    for (const cand of ['dev', 'main']) {
      const mb = git('merge-base', 'HEAD', cand)
      if (mb.code === 0) {
        b = mb.out.trim()
        break
      }
    }
  }
  const diffs = []
  if (b) diffs.push(git('diff', '-U0', '--no-color', `${b}...HEAD`).out)
  diffs.push(git('diff', '-U0', '--no-color', 'HEAD').out) // индекс + рабочее дерево
  const lines = diffs.flatMap(parseDiff)
  // Новые неотслеживаемые файлы целиком
  const tracked = new Set(
    git('ls-files')
      .out.split('\n')
      .map((s) => s.trim()),
  )
  for (const f of files) {
    if (f && !tracked.has(f) && isCodeFile(f) && fs.existsSync(path.join(root, f))) lines.push(...wholeFile(f, root))
  }
  return lines
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const argv = process.argv.slice(2)
  const bi = argv.indexOf('--base')
  const all = argv.includes('--all')
  const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
  const bad = violations(collect({ root, base: bi >= 0 ? argv[bi + 1] : '', staged: argv.includes('--staged'), all }))
  // Дубли: строка могла попасть и в коммитный дифф, и в рабочий
  const seen = new Set()
  const uniq = bad.filter((v) => !seen.has(`${v.file}:${v.line}`) && seen.add(`${v.file}:${v.line}`))
  if (uniq.length === 0) {
    console.log('комментарии: английских фраз нет')
    process.exit(0)
  }
  console.log(`английские комментарии (${uniq.length}) — переписать по-русски или пометить lang-ok:`)
  for (const v of uniq.slice(0, all ? 40 : 200)) console.log(`${v.file}:${v.line}: ${v.text}`)
  process.exit(1)
}
