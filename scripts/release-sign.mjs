#!/usr/bin/env node
// Подпись выпуска для автообновления (спека 111, design §5): между «черновик собран» и «опубликовать»
// (docs/RELEASE.md) делает ассет SosninaAlbums-update.json — конверт {"sig", "body"} как у каталога библиотеки
// (fetch.Open): sig — base64 ed25519 по байтам body, body — base64 JSON. Только node:* и gh/git, без зависимостей.
//
//   node scripts/release-sign.mjs vX.Y.Z [--key docs/private/release-key.pem] [--dry] [--min-auto X.Y.Z] [--bmp-mtime <unix-сек>]
//       (перед подписью пересобирает установщик из метки на этой машине, ~2–4 мин)
//   node scripts/release-sign.mjs --new-key <путь.pem>    создаёт пару, печатает открытый ключ (base64, 32 байта)
//   node scripts/release-sign.mjs --test-key [путь.pem]   одноразовый ключ в .gotmp/ для тестов и QA (design §9)
//   QA без выпуска на GitHub: --from <папка с setup.exe и SHA256SUMS.txt> [--whatsnew файл.ts] [--date ГГГГ-ММ-ДД]
//
// Порядок (design §5.4): 1) gh release download установщика и SHA256SUMS.txt черновика; 2) sha256 установщика
// обязан равняться строке SHA256SUMS.txt (её сделал CI по цепочке хешей заданий) — иначе отказ; 3) установщик
// пересобирается из метки здесь же (release-rebuild.mjs) и обязан совпасть с черновиком побайтно — иначе отказ, ничего
// не подписано (аккаунт GitHub и CI не единственное доверенное звено); 4) «Что нового» — WHATS_NEW из whatsnew.ts метки,
// WHATS_NEW_VERSION обязан быть X.Y.Z; 5) body, подпись, самопроверка открытым ключом, проверка, что ключ есть в
// internal/update/keys.go (иначе программы файл отвергнут), gh release upload.
// --dry — всё, кроме загрузки; --from и --whatsnew тоже ничего не загружают. --skip-rebuild (без пересборки) принимается
// только вместе с --from: это QA на заготовленных файлах; настоящая подпись без пересборки невозможна.
//
// --bmp-mtime <unix-сек> (или SA_INSTALLER_BMP_MTIME) — для выпусков до 0.25.1: их установщик хранит mtime wizard.bmp и
// header.bmp (момент checkout в pack CI); в копии метки время выставляется перед сборкой, остальная сверка — побайтно.
// Закрытый ключ в вывод не печатается и не копируется; --new-key не перезаписывает существующий файл.
import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { publicRaw, signBytes, verifyBytes } from './library-pack.mjs'
import { rebuildSetup } from './release-rebuild.mjs'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
export const ASSET = 'SosninaAlbums-update.json'
export const APP = 'sosninaalbums'
export const FILE_KEY = 'windows-x64-setup'
export const MIN_SIZE = 1 << 20 // как internal/update.Parse: 1…80 МБ
export const MAX_SIZE = 80 << 20
export const MAX_NOTES = 8
export const MAX_NOTE_LEN = 300
export const MAX_ENVELOPE = 16 << 10 // конверт ≤ 16 КБ (fetch.Client.Signed)
export const DEFAULT_MIN_AUTO = '0.25.0' // первая версия с автообновлением
const VER_RE = /^\d{1,4}\.\d{1,4}\.\d{1,4}$/
const TAG_RE = /^v(\d{1,4}\.\d{1,4}\.\d{1,4})$/
const SETUP_RE = /^SosninaAlbums-(\d{1,4}\.\d{1,4}\.\d{1,4})-windows-x64-setup\.exe$/

/** Отказ с понятным текстом (в CLI — без стека); code 2 — неверный вызов, 1 — отказ проверки. */
export class SignError extends Error {
  constructor(message, code = 1) {
    super(message)
    this.code = code
  }
}

export const setupName = (ver) => `SosninaAlbums-${ver}-windows-x64-setup.exe`
const sha256 = (b) => crypto.createHash('sha256').update(b).digest('hex')

// ---- сверка с пересборкой ----

/** Первый разошедшийся байт двух буферов (при разной длине — конец короткого); -1 — равны. */
const firstDiff = (a, b) => {
  const n = Math.min(a.length, b.length)
  for (let i = 0; i < n; i++) if (a[i] !== b[i]) return i
  return a.length === b.length ? -1 : n
}

/** Установщик черновика обязан побайтно совпасть с пересобранным из метки; иначе отказ с понятным текстом. */
export function compareRebuilt(tag, name, draft, rebuilt) {
  const at = firstDiff(draft, rebuilt)
  if (at < 0) return
  throw new SignError(
    `установщик ${name} из черновика не совпал с пересобранным из метки ${tag} на этой машине ` +
      `(черновик ${draft.length} байт, sha256 ${sha256(draft).slice(0, 16)}…; пересборка ${rebuilt.length} байт, sha256 ${sha256(rebuilt).slice(0, 16)}…; ` +
      `первое расхождение — байт ${at}). Ничего не подписано. Либо файл в черновике собран не из этой метки или подменён, ` +
      `либо сборка не воспроизводится: сверьте тулчейны с release.yml (Go, Wails, Node, CGO_ENABLED=0, версия NSIS — makensis -VERSION). ` +
      `Подписывать «похожий» установщик нельзя`,
  )
}

// ---- «Что нового» из whatsnew.ts ----

const ESC = { n: '\n', t: '\t', r: '\r', b: '\b', f: '\f', v: '\v', 0: '\0' }

/** Строковые литералы ('…', "…", `…`) из текста массива TS по порядку до закрывающей «]»; прочее (запятые, комментарии) пропускается. */
function stringLiterals(text) {
  const out = []
  let i = 0
  let closed = false
  while (i < text.length) {
    const c = text[i]
    if (c === ']') {
      closed = true
      break
    }
    if (c === '/' && text[i + 1] === '/') {
      while (i < text.length && text[i] !== '\n') i++
    } else if (c === '/' && text[i + 1] === '*') {
      const end = text.indexOf('*/', i + 2)
      i = end < 0 ? text.length : end + 2
    } else if (c === "'" || c === '"' || c === '`') {
      let s = ''
      i++
      for (;;) {
        if (i >= text.length) throw new SignError('whatsnew.ts: строка не закрыта')
        const d = text[i]
        if (d === c) break
        if (d === '\\') {
          const e = text[i + 1]
          if (e === 'u') {
            const hex = /^\{([0-9a-fA-F]+)\}|^([0-9a-fA-F]{4})/.exec(text.slice(i + 2))
            if (!hex) throw new SignError('whatsnew.ts: неверная \\u-последовательность')
            s += String.fromCodePoint(parseInt(hex[1] ?? hex[2], 16))
            i += 2 + hex[0].length
          } else {
            s += e in ESC ? ESC[e] : e
            i += 2
          }
        } else {
          s += d
          i++
        }
      }
      i++
      out.push(s)
    } else i++
  }
  if (!closed) throw new SignError('whatsnew.ts: массив WHATS_NEW не закрыт')
  return out
}

/** whatsnew.ts -> {version, notes}: WHATS_NEW_VERSION и строки массива WHATS_NEW. */
export function parseWhatsNew(ts) {
  const ver = /WHATS_NEW_VERSION\s*=\s*'([^']+)'/.exec(ts)?.[1]
  if (!ver) throw new SignError('whatsnew.ts: нет WHATS_NEW_VERSION')
  const m = /export const WHATS_NEW\b[^=]*=\s*\[/.exec(ts)
  if (!m) throw new SignError('whatsnew.ts: нет массива WHATS_NEW')
  return { version: ver, notes: stringLiterals(ts.slice(m.index + m[0].length)) }
}

/** Строка «Что нового»: без управляющих знаков, пробелы схлопнуты; пустая — отбрасывается вызывающим. */
export const cleanNote = (s) =>
  s
    .replace(/[\u0000-\u001f\u007f-\u009f\u2028\u2029]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()

// ---- SHA256SUMS.txt ----

/** Строки «<hex64>  <имя>» (формат sha256sum, имя может начинаться с «*») -> Map имя -> hex. Мусорные строки — отказ. */
export function parseSums(text) {
  const out = new Map()
  for (const line of text.split(/\r?\n/)) {
    if (line.trim() === '') continue
    const m = /^([0-9a-fA-F]{64}) [ *](.+)$/.exec(line)
    if (!m) throw new SignError(`SHA256SUMS.txt: строка не по формату sha256sum: «${line.slice(0, 80)}»`)
    const name = path.posix.basename(m[2].trim())
    const hex = m[1].toLowerCase()
    if (out.has(name) && out.get(name) !== hex) throw new SignError(`SHA256SUMS.txt: у ${name} два разных хеша`)
    out.set(name, hex)
  }
  return out
}

// ---- body и конверт ----

/**
 * Внутренний JSON выпуска (ключи по порядку design §5.1). Проверяет то же, что internal/update.Parse, чтобы не
 * подписать файл, который программы отвергнут: версия, minAuto, ≤ 8 строк по ≤ 300 знаков, имя и размер установщика.
 */
export function buildBody({ version, date, minAuto, notes, setup }) {
  if (!VER_RE.test(version ?? '')) throw new SignError(`версия «${version}» — ждали X.Y.Z`)
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date ?? '')) throw new SignError(`дата «${date}» — ждали ГГГГ-ММ-ДД`)
  if (minAuto !== '' && !VER_RE.test(minAuto ?? '')) throw new SignError(`minAuto «${minAuto}» — пусто или X.Y.Z`)
  const lines = notes.map(cleanNote).filter((n) => n !== '')
  if (lines.length < 1) throw new SignError('«Что нового» пусто: в WHATS_NEW нет строк')
  if (lines.length > MAX_NOTES) throw new SignError(`в «Что нового» ${lines.length} строк, больше ${MAX_NOTES}: программа их не примет`)
  const long = lines.find((n) => [...n].length > MAX_NOTE_LEN)
  if (long) throw new SignError(`строка «Что нового» длиннее ${MAX_NOTE_LEN} знаков: «${long.slice(0, 40)}…»`)
  if (setup.name !== setupName(version)) throw new SignError(`имя установщика ${setup.name}, ждали ${setupName(version)}`)
  if (!Number.isInteger(setup.size) || setup.size < MIN_SIZE || setup.size > MAX_SIZE) {
    throw new SignError(`размер установщика ${setup.size} байт вне 1…80 МБ: программа его не примет`)
  }
  if (!/^[0-9a-f]{64}$/.test(setup.sha256)) throw new SignError('sha256 установщика — 64 hex нижним регистром')
  return {
    format: 1,
    app: APP,
    version,
    date,
    minAuto,
    notes: lines,
    files: { [FILE_KEY]: { name: setup.name, size: setup.size, sha256: setup.sha256 } },
  }
}

/** Конверт: подпись по точным байтам body (до base64), как sealCatalog библиотеки. Возвращает {sig, body}. */
export function sealEnvelope(body, privateKey) {
  const bytes = Buffer.from(JSON.stringify(body))
  return { sig: signBytes(bytes, privateKey), body: bytes.toString('base64') }
}

/** Проверка конверта открытым ключом (base64 32 байта) -> внутренний JSON; порча или чужой ключ — исключение. */
export function openEnvelope(text, pubRaw) {
  let env
  try {
    env = JSON.parse(text)
  } catch {
    throw new SignError('конверт — не JSON')
  }
  if (typeof env?.sig !== 'string' || typeof env?.body !== 'string') throw new SignError('в конверте нет sig и body')
  const bytes = Buffer.from(env.body, 'base64')
  if (!verifyBytes(bytes, env.sig, pubRaw)) throw new SignError('подпись конверта не сошлась')
  return JSON.parse(bytes.toString('utf8'))
}

// ---- ключи ----

/** Закрытый ключ из pem; не ed25519 — отказ. Содержимое файла нигде не печатается. */
export function loadKey(file) {
  if (!fs.existsSync(file)) throw new SignError(`нет ключа подписи: ${file} (создать: --new-key; для QA — --test-key)`)
  let key
  try {
    key = crypto.createPrivateKey(fs.readFileSync(file))
  } catch {
    throw new SignError(`файл ключа не читается как закрытый ключ: ${file}`)
  }
  if (key.asymmetricKeyType !== 'ed25519') throw new SignError('ключ подписи должен быть ed25519')
  return key
}

/** Новая пара: pem (pkcs8, права 0600) создаётся только если файла нет; возвращает открытый ключ base64. */
export function newKey(file) {
  const { privateKey } = crypto.generateKeyPairSync('ed25519')
  fs.mkdirSync(path.dirname(file), { recursive: true })
  try {
    fs.writeFileSync(file, privateKey.export({ type: 'pkcs8', format: 'pem' }), { flag: 'wx', mode: 0o600 })
  } catch (e) {
    if (e.code === 'EEXIST') throw new SignError(`${file} уже есть: существующий ключ не перезаписывается`)
    throw e
  }
  return publicRaw(privateKey)
}

/** Ключ из 32-байтного seed (для воспроизводимой фикстуры теста, не для выпуска). */
export function keyFromSeed(seed) {
  if (seed.length !== 32) throw new Error('seed — 32 байта')
  return crypto.createPrivateKey({ key: Buffer.concat([Buffer.from('302e020100300506032b657004220420', 'hex'), seed]), format: 'der', type: 'pkcs8' })
}

/** Открытые ключи, зашитые в программу: base64-строки по 32 байта из internal/update/keys.go (null — файла ещё нет). */
export function programKeys(file) {
  if (!fs.existsSync(file)) return null
  const text = fs.readFileSync(file, 'utf8')
  return [...text.matchAll(/"([A-Za-z0-9+/]{43}=)"/g)].map((m) => m[1]).filter((k) => Buffer.from(k, 'base64').length === 32)
}

// ---- время BMP установщика ----

/**
 * Разбирает --bmp-mtime / SA_INSTALLER_BMP_MTIME: целые unix-секунды. Пусто — undefined (поведение прежнее).
 * Нужен для выпусков до 0.25.1: их установщик хранит mtime wizard.bmp и header.bmp (= момент checkout в pack CI).
 */
export function parseBmpMtime(v) {
  if (v === undefined || v === '') return undefined
  if (!/^\d{1,12}$/.test(String(v))) throw new SignError(`--bmp-mtime: ждали unix-секунды (целое число), получено «${v}»`, 2)
  return Number(v)
}

// ---- выпуск ----

/**
 * Подписывает выпуск tag. deps: gh(args) и git(args) -> строка/Buffer, rebuild({tag, version, work, bmpMtime}) -> путь к
 * установщику, пересобранному из метки (в тестах подменяются).
 * opts: {tag, keyPath, dry, minAuto, from, whatsnew, date, keysFile, work, skipRebuild, bmpMtime}. Возвращает {file, version, size, sha256, uploaded}.
 */
export function signRelease(opts, deps) {
  const m = TAG_RE.exec(opts.tag ?? '')
  if (!m) throw new SignError(`метка «${opts.tag}» — ждали vX.Y.Z`)
  const version = m[1]
  // без пересборки подписывает только QA на готовых файлах (--from); настоящая подпись без неё не бывает
  if (opts.skipRebuild && !opts.from) throw new SignError('--skip-rebuild допустим только вместе с --from (QA-режим): настоящий выпуск подписывается только после пересборки установщика', 2)
  // ключ — первым: без него не тратим время на скачивание
  const key = loadKey(opts.keyPath)
  const pub = publicRaw(key)

  const work = path.join(opts.work, opts.tag)
  fs.rmSync(work, { recursive: true, force: true })
  fs.mkdirSync(work, { recursive: true })
  let dir = work
  if (opts.from) {
    dir = path.resolve(opts.from)
  } else {
    deps.gh(['release', 'download', opts.tag, '-p', '*-windows-x64-setup.exe', '-p', 'SHA256SUMS.txt', '-D', work])
  }

  const sumsFile = path.join(dir, 'SHA256SUMS.txt')
  if (!fs.existsSync(sumsFile)) throw new SignError('нет SHA256SUMS.txt в черновике: его делает CI; без него подписывать нельзя')
  const sums = parseSums(fs.readFileSync(sumsFile, 'utf8'))
  const setups = fs.readdirSync(dir).filter((f) => SETUP_RE.test(f))
  if (setups.length !== 1) throw new SignError(`установщиков в ${opts.from ? 'папке' : 'черновике'} ${setups.length}, ждали один (*-windows-x64-setup.exe)`)
  const name = setups[0]
  if (name !== setupName(version)) throw new SignError(`установщик ${name} не от версии ${version}`)
  const data = fs.readFileSync(path.join(dir, name))
  const hash = sha256(data)
  if (!sums.has(name)) throw new SignError(`в SHA256SUMS.txt нет строки для ${name}`)
  if (sums.get(name) !== hash) throw new SignError(`sha256 ${name} не равен строке SHA256SUMS.txt: файл подменён или повреждён`)

  if (!opts.skipRebuild) {
    // независимая сверка: установщик собран здесь из метки, а не взят на веру у CI (design §5.4, §10)
    const rebuilt = deps.rebuild({ tag: opts.tag, version, work, bmpMtime: opts.bmpMtime })
    if (path.basename(rebuilt) !== name) throw new SignError(`пересборка дала ${path.basename(rebuilt)}, ждали ${name}`)
    compareRebuilt(opts.tag, name, data, fs.readFileSync(rebuilt))
  }

  const ts = opts.whatsnew ? fs.readFileSync(opts.whatsnew, 'utf8') : deps.git(['show', `${opts.tag}:frontend/src/shell/dialogs/whatsnew.ts`]).toString()
  const wn = parseWhatsNew(ts)
  if (wn.version !== version) throw new SignError(`WHATS_NEW_VERSION ${wn.version} в метке ${opts.tag}, ждали ${version}: «Что нового» от другого выпуска`)
  const date = opts.date ?? deps.git(['show', '-s', '--format=%cs', `${opts.tag}^{commit}`]).toString().trim()

  const body = buildBody({ version, date, minAuto: opts.minAuto ?? DEFAULT_MIN_AUTO, notes: wn.notes, setup: { name, size: data.length, sha256: hash } })
  const env = sealEnvelope(body, key)
  const text = JSON.stringify(env) + '\n'
  if (Buffer.byteLength(text) > MAX_ENVELOPE) throw new SignError(`конверт ${Buffer.byteLength(text)} байт, больше ${MAX_ENVELOPE}: программа его не примет`)
  // самопроверка: что записали, то открывается ключом, а тот — зашит в программу
  if (JSON.stringify(openEnvelope(text, pub)) !== JSON.stringify(body)) throw new SignError('самопроверка конверта не сошлась')
  const real = !opts.from && !opts.whatsnew
  const known = programKeys(opts.keysFile)
  if (known === null) console.error(`внимание: ${path.relative(root, opts.keysFile)} нет — не проверено, что ключ зашит в программу`)
  else if (!known.includes(pub) && real) throw new SignError('открытого ключа подписи нет в internal/update/keys.go: программы отвергнут файл. Проверьте --key')

  const file = path.join(work, ASSET)
  fs.writeFileSync(file, text)
  const uploaded = !opts.dry && real
  if (uploaded) deps.gh(['release', 'upload', opts.tag, file, '--clobber']) // --clobber: повторная подпись черновика заменяет прежний файл
  return { file, version, size: data.length, sha256: hash, uploaded, notes: body.notes.length }
}

// ---- CLI ----

/**
 * Куда писать одноразовый тестовый ключ: только внутрь <корень>/.gotmp/ (по умолчанию .gotmp/release-test-key.pem).
 * Прежний файл по этому пути удаляется, поэтому любой другой путь (docs/private с настоящим ключом, чужие
 * файлы) — отказ; ссылка, ведущая из .gotmp наружу, тоже.
 */
export function testKeyPath(next, rootDir = root) {
  const gotmp = path.join(rootDir, '.gotmp')
  const file = path.resolve(next && !next.startsWith('--') ? next : path.join(gotmp, 'release-test-key.pem'))
  const inside = (base, f) => {
    const rel = path.relative(base, f)
    return rel !== '' && !rel.startsWith('..') && !path.isAbsolute(rel)
  }
  // реальный путь ближайшего существующего предка: ссылка в .gotmp не должна вести наружу
  let real = file
  const tail = []
  while (!fs.existsSync(real) && path.dirname(real) !== real) {
    tail.unshift(path.basename(real))
    real = path.dirname(real)
  }
  const base = fs.existsSync(gotmp) ? fs.realpathSync(gotmp) : gotmp
  if (!inside(gotmp, file) || !inside(base, path.join(fs.realpathSync(real), ...tail))) {
    throw new SignError(`--test-key пишет и удаляет файл только внутри .gotmp/: ${path.relative(rootDir, file) || file} не годится`, 2)
  }
  return file
}

const arg = (argv, name, def) => {
  const i = argv.indexOf(name)
  return i >= 0 ? argv[i + 1] : def
}

// Сбой внешней команды — отказ с её именем, без стека; вывод команды (причина) идёт в stderr как есть
const external = (cmd, args) => {
  try {
    return execFileSync(cmd, args, { cwd: root, stdio: ['ignore', 'pipe', 'inherit'], maxBuffer: 1 << 28 })
  } catch (e) {
    throw new SignError(`${cmd} ${args.slice(0, 2).join(' ')}: не удалось${e.code === 'ENOENT' ? ` (${cmd} не найден)` : ''}`)
  }
}
const realDeps = {
  gh: (args) => external('gh', args),
  git: (args) => external('git', args),
  rebuild: ({ tag, version, work, bmpMtime }) => {
    if (bmpMtime !== undefined) console.error(`release-sign: mtime wizard.bmp и header.bmp в копии метки выставлен: ${bmpMtime} (${new Date(bmpMtime * 1000).toISOString()})`)
    return rebuildSetup({ tag, version, work, repo: root, bmpMtime, fail: (m) => new SignError(m) })
  },
}

function main(argv) {
  const newKeyPath = arg(argv, '--new-key', undefined)
  if (argv.includes('--new-key')) {
    if (!newKeyPath || newKeyPath.startsWith('--')) throw new SignError('использование: --new-key <путь.pem>', 2)
    const pub = newKey(path.resolve(newKeyPath))
    console.log(`закрытый ключ записан: ${newKeyPath} (хранить вне git, копию — вне машины)`)
    console.log(`открытый ключ (base64, 32 байта): ${pub}`)
    return
  }
  if (argv.includes('--test-key')) {
    const next = arg(argv, '--test-key', undefined)
    const file = testKeyPath(next)
    fs.rmSync(file, { force: true }) // одноразовый: прежний тестовый ключ заменяется
    console.log(`тестовый ключ: ${file}`)
    console.log(`открытый ключ (base64, 32 байта): ${newKey(file)}`)
    return
  }
  const tag = argv.find((a, i) => /^v\d/.test(a) && (i === 0 || !argv[i - 1].startsWith('--')))
  if (!tag) throw new SignError('использование: node scripts/release-sign.mjs vX.Y.Z [--key pem] [--dry] [--min-auto X.Y.Z] [--bmp-mtime сек] [--from папка --skip-rebuild] | --new-key <путь> | --test-key', 2)
  // флаг без значения (последний или перед другим флагом) — отказ, а не молчаливое «не задано»
  const bi = argv.indexOf('--bmp-mtime')
  if (bi >= 0 && (bi + 1 >= argv.length || argv[bi + 1].startsWith('--'))) throw new SignError('--bmp-mtime: не указано значение (unix-секунды)', 2)
  const bmpMtime = parseBmpMtime(arg(argv, '--bmp-mtime', process.env.SA_INSTALLER_BMP_MTIME))
  const r = signRelease(
    {
      tag,
      bmpMtime,
      keyPath: path.resolve(arg(argv, '--key', path.join(root, 'docs', 'private', 'release-key.pem'))),
      dry: argv.includes('--dry'),
      skipRebuild: argv.includes('--skip-rebuild'),
      minAuto: arg(argv, '--min-auto', undefined),
      from: arg(argv, '--from', undefined),
      whatsnew: arg(argv, '--whatsnew', undefined),
      date: arg(argv, '--date', undefined),
      keysFile: path.resolve(arg(argv, '--keys', path.join(root, 'internal', 'update', 'keys.go'))),
      work: path.join(root, '.gotmp', 'release-sign'),
    },
    realDeps,
  )
  console.log(`${ASSET}: версия ${r.version}, установщик ${r.size} байт, sha256 ${r.sha256.slice(0, 12)}…, строк «Что нового» ${r.notes}`)
  console.log(r.uploaded ? `загружен в ${tag}` : `не загружен (${argv.includes('--dry') ? '--dry' : 'QA-режим'}): ${r.file}`)
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    main(process.argv.slice(2))
  } catch (e) {
    if (!(e instanceof SignError)) throw e
    console.error(`release-sign: ${e.message}`)
    process.exit(e.code)
  }
}
