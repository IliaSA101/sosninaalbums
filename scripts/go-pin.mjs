#!/usr/bin/env node
// Версия Go в репозитории (спека 171, design §8): одна, во всех закреплениях, и порог macOS у этой версии Go.
//
//   node scripts/go-pin.mjs [--check]       все закрепления равны toolchain из go.mod; плисты и MAC_MIN_OS не ниже порога Go
//   node scripts/go-pin.mjs --latest        свежий патч текущего минора и последний минор (go.dev/dl, только печатает)
//   node scripts/go-pin.mjs --write go1.26.N правит все закрепления, затем --check; плисты нового минора — вручную
//
// Закрепления: go.mod и api/albums/go.mod (toolchain), все `go-version:` в .github/workflows (15 строк на 0.40),
// производные места (THIRD_PARTY_NOTICES.txt — версия стандартной библиотеки, docs/qa/e2e-regress.md — «setup-go X»).
// Вне репозитория (`.claude/settings.local.json` GOTOOLCHAIN, `.claude/LOCAL.md`) скрипт не трогает: он напоминает.
// Порог macOS у Go — таблица MAC_FLOOR; минора нет в ней — отказ с просьбой вписать (go.dev/doc/go1.N, Ports).
// Только node:*, без зависимостей; release-sign.mjs берёт отсюда порог и разбор плиста.
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')

/** Минор Go -> минимальная macOS, на которой работают программы этой версии (go.dev/doc/go1.N, Ports). */
export const MAC_FLOOR = { '1.23': '11.0', '1.24': '11.0', '1.25': '12.0', '1.26': '12.0', '1.27': '13.0' }

/** Версия системы, как в манифесте обновления (internal/update.ParseOS): 1–3 числа по 1–6 цифр. */
export const OS_RE = /^\d{1,6}(\.\d{1,6}){0,2}$/

/** Отказ с понятным текстом (в CLI — без стека). */
export class PinError extends Error {}

const parts = (s) => [0, 1, 2].map((i) => Number(s.split('.')[i] ?? 0))

/** Сравнение версий системы по числам, недостающие части = 0: <0, 0, >0. */
export function cmpOS(a, b) {
  const [x, y] = [parts(a), parts(b)]
  for (let i = 0; i < 3; i++) if (x[i] !== y[i]) return x[i] - y[i]
  return 0
}

/** Большая из двух версий системы (строка остаётся как была). */
export const maxOS = (a, b) => (cmpOS(a, b) >= 0 ? a : b)

/** «go1.26.11» / «1.26.11» / «1.26» -> {minor: '1.26', patch: 11}; не версия Go — null. */
export function parseGo(s) {
  const m = /^(?:go)?(\d+\.\d+)(?:\.(\d+))?$/.exec(String(s ?? ''))
  return m ? { minor: m[1], patch: Number(m[2] ?? 0) } : null
}

/** Порог macOS для версии Go (по минору); минор не из таблицы — отказ. */
export function macFloor(ver) {
  const g = parseGo(ver)
  if (!g) throw new PinError(`«${ver}» — не версия Go (ждали 1.N.P)`)
  const f = MAC_FLOOR[g.minor]
  if (!f) throw new PinError(`нет порога macOS для go${g.minor}: впишите его в MAC_FLOOR в scripts/go-pin.mjs (go.dev/doc/go${g.minor}, Ports)`)
  return f
}

/** Версия Go из текста go.mod: строка toolchain, а без неё директива go. null — ни той, ни другой. */
export function goVersionOf(goMod) {
  return /^toolchain go(\d+\.\d+\.\d+)\s*$/m.exec(goMod)?.[1] ?? /^go (\d+\.\d+(?:\.\d+)?)\s*$/m.exec(goMod)?.[1] ?? null
}

/** LSMinimumSystemVersion из текста Info.plist; null — ключа нет. */
export function plistMin(plist) {
  return /<key>LSMinimumSystemVersion<\/key>\s*<string>([^<]*)<\/string>/.exec(plist)?.[1].trim() ?? null
}

// ---- места закрепления ----

const read = (root, rel) => fs.readFileSync(path.join(root, rel), 'utf8')
const exists = (root, rel) => fs.existsSync(path.join(root, rel))

const GO_MODS = ['go.mod', 'api/albums/go.mod']
const TOOLCHAIN_RE = /^(toolchain go)(\d+\.\d+\.\d+)(\s*)$/gm
const WORKFLOW_RE = /^(\s*(?:-\s*)?go-version:\s*)(["']?)(\d+\.\d+\.\d+)\2(\s*)$/gm
const GO_VERSION_ANY = /^\s*(?:-\s*)?go-version:/gm
// Производные места: (файл, шаблон с версией в группе 2, min — сколько вхождений обязано быть, если файл есть). Версию они не
// задают, а повторяют. Файла нет — место пропускается: docs/qa/ не входит в публичное дерево (main, winqa/*, CI).
const DERIVED = [
  { file: 'THIRD_PARTY_NOTICES.txt', re: /(Стандартная библиотека Go go)(\d+\.\d+\.\d+)/g, min: 1 },
  { file: 'docs/qa/e2e-regress.md', re: /(`setup-go` )(\d+\.\d+\.\d+)/g, min: 1 },
]
export const PLISTS = ['build/darwin/Info.plist', 'build/darwin/Info.dev.plist']
export const SMOKE = 'scripts/mac/smoke.sh'
const SMOKE_RE = /(MIN_OS="\$\{MAC_MIN_OS:-)([\d.]+)(\}")/

const workflows = (root) =>
  fs.existsSync(path.join(root, '.github', 'workflows'))
    ? fs
        .readdirSync(path.join(root, '.github', 'workflows'))
        .filter((f) => /\.ya?ml$/.test(f))
        .sort()
        .map((f) => `.github/workflows/${f}`)
    : []

/** Все закрепления версии: [{file, line, version}]. line — номер строки (для вывода), 0 — не строчное. */
export function collectPins(root) {
  const pins = []
  const each = (rel, re, group = 2) => {
    const text = read(root, rel)
    let n = 0
    for (const m of text.matchAll(re)) {
      pins.push({ file: rel, line: text.slice(0, m.index).split('\n').length, version: m[group] })
      n++
    }
    return { text, n }
  }
  for (const rel of GO_MODS) {
    if (!exists(root, rel)) pins.push({ file: rel, line: 0, version: null })
    else if (each(rel, TOOLCHAIN_RE).n === 0) pins.push({ file: rel, line: 0, version: null })
  }
  for (const rel of workflows(root)) {
    const { text, n } = each(rel, WORKFLOW_RE, 3)
    // строка go-version не по шаблону (без патча, «stable») не должна проскочить мимо проверки
    const all = [...text.matchAll(GO_VERSION_ANY)].length
    for (let i = n; i < all; i++) pins.push({ file: rel, line: 0, version: null })
  }
  for (const d of DERIVED) {
    if (!exists(root, d.file)) continue
    const { n } = each(d.file, d.re)
    if (n < d.min) pins.push({ file: d.file, line: 0, version: null })
  }
  return pins
}

/**
 * Проверка: все закрепления равны toolchain корневого go.mod, плисты и MAC_MIN_OS в smoke.sh не ниже порога macOS у этой Go.
 * Возвращает {version, floor, problems: [строки]}; problems пуст — всё согласовано.
 */
export function checkPins(root) {
  const problems = []
  const want = /^toolchain go(\d+\.\d+\.\d+)\s*$/m.exec(read(root, 'go.mod'))?.[1]
  if (!want) return { version: null, floor: null, problems: ['go.mod: нет строки «toolchain goX.Y.Z»'] }
  for (const p of collectPins(root)) {
    if (p.version === null) problems.push(`${p.file}: версия Go не найдена или записана не как X.Y.Z`)
    else if (p.version !== want) problems.push(`${p.file}${p.line ? `:${p.line}` : ''}: ${p.version}, в go.mod ${want}`)
  }
  let floor = null
  try {
    floor = macFloor(want)
  } catch (e) {
    problems.push(e.message)
    return { version: want, floor, problems }
  }
  for (const rel of PLISTS) {
    if (!exists(root, rel)) {
      problems.push(`${rel}: файла нет`)
      continue
    }
    const v = plistMin(read(root, rel))
    if (v === null || !OS_RE.test(v)) problems.push(`${rel}: LSMinimumSystemVersion ${v === null ? 'не найден' : `«${v}» — не версия системы`}`)
    else if (cmpOS(v, floor) < 0) problems.push(`${rel}: LSMinimumSystemVersion ${v} ниже порога macOS ${floor} у go${want}: поднимите до ${floor}`)
  }
  const smoke = exists(root, SMOKE) ? SMOKE_RE.exec(read(root, SMOKE)) : null
  if (!smoke) problems.push(`${SMOKE}: нет строки MIN_OS="\${MAC_MIN_OS:-X.Y}"`)
  else if (cmpOS(smoke[2], floor) < 0) problems.push(`${SMOKE}: MAC_MIN_OS по умолчанию ${smoke[2]} ниже порога macOS ${floor} у go${want}: поднимите до ${floor}`)
  return { version: want, floor, problems }
}

/** Правит все закрепления на версию ver («go1.26.12» или «1.26.12»); возвращает {version, changed: [файлы]}. Порог macOS у минора обязан быть в таблице. */
export function writePins(root, ver) {
  const m = /^(?:go)?(\d+\.\d+\.\d+)$/.exec(String(ver ?? ''))
  if (!m) throw new PinError(`«${ver}» — ждали go1.N.P`)
  const version = m[1]
  macFloor(version) // неизвестный минор — отказ до первой правки
  const changed = []
  const edit = (rel, re, build) => {
    const text = read(root, rel)
    const next = text.replace(re, build)
    if (next !== text) {
      fs.writeFileSync(path.join(root, rel), next)
      changed.push(rel)
    }
  }
  for (const rel of GO_MODS) if (exists(root, rel)) edit(rel, TOOLCHAIN_RE, (_, a, _v, c) => `${a}${version}${c}`)
  for (const rel of workflows(root)) edit(rel, WORKFLOW_RE, (_, a, q, _v, d) => `${a}${q}${version}${q}${d}`)
  for (const d of DERIVED) if (exists(root, d.file)) edit(d.file, d.re, (_, a) => `${a}${version}`)
  return { version, changed }
}

// ---- свежий патч ----

/**
 * Из списка выпусков go.dev/dl (JSON, include=all) -> {patch, minor}: последний стабильный патч минора текущей версии и
 * последний стабильный минор (оба — «go1.N.P»). current — «1.26.11». Список без стабильных выпусков — отказ.
 */
export function latestFrom(list, current) {
  if (!Array.isArray(list)) throw new PinError('go.dev/dl: ждали список выпусков')
  const cur = parseGo(current)
  if (!cur) throw new PinError(`«${current}» — не версия Go`)
  const stable = list.filter((r) => r && r.stable === true && typeof r.version === 'string').map((r) => parseGo(r.version)).filter(Boolean)
  const newer = (a, b) => (a.minor === b.minor ? a.patch - b.patch : cmpOS(a.minor, b.minor))
  const best = (xs) => xs.sort(newer).at(-1)
  const same = best(stable.filter((g) => g.minor === cur.minor))
  const top = best(stable)
  if (!top) throw new PinError('go.dev/dl: нет стабильных выпусков')
  const name = (g) => `go${g.minor}.${g.patch}`
  return { patch: same ? name(same) : null, minor: name(top) }
}

// ---- CLI ----

function printCheck(root) {
  const r = checkPins(root)
  if (r.problems.length === 0) {
    console.log(`go-pin: go${r.version} во всех закреплениях, порог macOS ${r.floor} (плисты и MAC_MIN_OS не ниже)`)
    return 0
  }
  console.error(`go-pin: расхождения (${r.problems.length}):`)
  for (const p of r.problems) console.error(`  - ${p}`)
  return 1
}

async function main(argv) {
  if (argv.includes('--latest')) {
    const cur = goVersionOf(read(ROOT, 'go.mod'))
    const res = await fetch('https://go.dev/dl/?mode=json&include=all', { signal: AbortSignal.timeout(30000) })
    if (!res.ok) throw new PinError(`go.dev/dl: ответ ${res.status}`)
    const l = latestFrom(await res.json(), cur)
    console.log(`в go.mod: go${cur}`)
    console.log(`свежий патч этого минора: ${l.patch ?? 'нет'}${l.patch && l.patch !== `go${cur}` ? `  ->  node scripts/go-pin.mjs --write ${l.patch}` : '  (стоит последний)'}`)
    console.log(`последний минор: ${l.minor}${parseGo(l.minor).minor !== parseGo(cur).minor ? ` (порог macOS ${MAC_FLOOR[parseGo(l.minor).minor] ?? 'не в таблице MAC_FLOOR'}; переход — шаг 2, design §9)` : ''}`)
    return 0
  }
  const wi = argv.indexOf('--write')
  if (wi >= 0) {
    const w = writePins(ROOT, argv[wi + 1])
    console.log(`go-pin: go${w.version}; изменены: ${w.changed.join(', ') || 'ничего (уже стоит)'}`)
    const code = printCheck(ROOT)
    console.log('Вручную (вне репозитория): `.claude/settings.local.json` GOTOOLCHAIN и `.claude/LOCAL.md` — на обеих машинах;')
    console.log('далее `node scripts/third-party.mjs` (если менялась не только запись о версии) и сверка sha256 exe двух машин (CGO_ENABLED=0).')
    return code
  }
  const extra = argv.filter((a) => a !== '--check')
  if (extra.length) throw new PinError('использование: node scripts/go-pin.mjs [--check] | --latest | --write go1.N.P')
  return printCheck(ROOT)
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main(process.argv.slice(2)).then(
    (code) => process.exit(code),
    (e) => {
      if (!(e instanceof PinError)) throw e
      console.error(`go-pin: ${e.message}`)
      process.exit(2)
    },
  )
}
