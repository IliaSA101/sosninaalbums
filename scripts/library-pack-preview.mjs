// Лист образцов набора в каталоге (спека 144, design §2–§3, security Н5 и Н7): проверки листа JPEG и preview.json,
// поле `preview` строки и режим `--preview-only`. Подключается из library-pack.mjs; сам его не импортирует —
// обмен через объект lib, без цикла модулей (как library-pack-demo.mjs).
//
// Лист и preview.json делает `library-verify -sprite <набор.zip> -out <папка>` (scripts/library-verify-sprite.go):
// `<id>-<версия>-preview.jpg` и `<id>-<версия>-preview.json`. Обычный выпуск берёт их из `--sheets <папка>`.
//
//   node scripts/library-pack.mjs --preview-only <id>=<опубликованный.zip> [--preview-only ...] --sheets <папка>
//                                 --catalog <каталог> [--key <pem>] [--seq <N>]
//       Для вышедших наборов: в строке каталога меняется только `preview`, прочие поля и строки — байт в байт
//       (сверка до подписи); все пары — один прогон, одна подпись, seq+1. Архивы и их sha256 не меняются: zip не
//       пишется, sha256 переданного zip обязан совпасть со строкой каталога.
import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'

export const MAX_PREVIEW = 384 << 10 // как fetch.MaxPreview: больше клиент лист не берёт
const MAX_RAW = 8 << 10 // сырое поле preview строки: клиент отказывает до разбора
const MAX_META = 64 << 10 // preview.json: ряды — килобайты
const MAX_W = 4096
const MAX_H = 2048
const MAX_PIXELS = 4 << 20
const CELL_MIN = 32
const CELL_MAX = 480
const MAX_ROWS = 5
const MAX_ITEMS = 12
const ROW_KINDS = ['bg', 'sticker', 'frame', 'spread', 'album']
export const BODY_FAIL = 768 << 10 // тело каталога: ¾ MaxCatalog (1 МиБ) старых клиентов
export const BODY_WARN = 512 << 10
const MAX_MARKERS = 64 // сегментов до кадра, SOS считается (решение 0.31-144-33)
const JFIF_ID = Buffer.from('JFIF\0')
const ADOBE_ID = Buffer.from('Adobe')
const SUM_RE = /^[0-9a-f]{64}$/
const ITEM_RE = /^[0-9a-f]{16}$/

const usage = (msg) => Object.assign(new Error(msg), { code: 2 })
const isInt = (n, lo, hi) => Number.isInteger(n) && n >= lo && n <= hi

/**
 * Кадр JPEG по сегментам до первой развёртки (SOS): {marker, w, h, comps}. Только SOF0/SOF1 (baseline), точность 8,
 * 1 или 3 компонента; прогрессивный, CMYK, второй SOF, битая длина — исключение. Не больше 64 сегментов до кадра и
 * включая SOS. Три компонента, которые Go прочёл бы как RGB (`isRGB` в image/jpeg: нет JFIF и Adobe APP14 transform 0
 * либо id компонентов R, G, B), — тоже отказ: программа требует YCbCr или Gray (Н7), лист с RGB дал бы 404 на клетках.
 * Разбор APP0/APP14 повторяет Go: JFIF — последний APP0 с «JFIF\0»; кадр при уже виденном JFIF решает сразу.
 */
export function jpegFrame(data) {
  if (data.length < 4 || data[0] !== 0xff || data[1] !== 0xd8) throw new Error('лист: не JPEG')
  let p = 2
  let frame = null
  let ids = null
  let jfif = false
  let adobe = -1 // transform сегмента Adobe или -1, если его нет
  let sealed = false // JFIF был до кадра: Go дальше метки цвета не читает
  for (let n = 0; n < MAX_MARKERS; n++) {
    while (p < data.length && data[p] === 0xff && data[p + 1] === 0xff) p++ // заполнитель между сегментами
    if (p + 4 > data.length || data[p] !== 0xff) throw new Error('лист: JPEG битый или усечён')
    const m = data[p + 1]
    if (m === 0xda) {
      if (!frame) throw new Error('лист: развёртка раньше кадра')
      if (frame.comps === 3 && isRGB(jfif, adobe, ids)) throw new Error('лист: JPEG с RGB-компонентами (нет JFIF, Adobe transform 0 или id R,G,B): нужен YCbCr')
      return frame
    }
    // маркеры без длины и FF 00 (image/jpeg пропускает его как мусор, а здесь он сошёл бы за сегмент с длиной) — те же отказы, что в Go (0.31-144-33)
    if (m === 0x00 || m === 0x01 || m === 0xd9 || (m >= 0xd0 && m <= 0xd8)) throw new Error('лист: JPEG битый (маркер без длины до развёртки)')
    const len = data.readUInt16BE(p + 2)
    if (len < 2 || p + 2 + len > data.length) throw new Error('лист: JPEG битый (длина сегмента)')
    if (!sealed && m === 0xe0 && len - 2 >= 5) jfif = data.subarray(p + 4, p + 9).equals(JFIF_ID)
    if (!sealed && m === 0xee && len - 2 >= 12 && data.subarray(p + 4, p + 9).equals(ADOBE_ID)) adobe = data[p + 15]
    if (m >= 0xc0 && m <= 0xcf && m !== 0xc4 && m !== 0xc8 && m !== 0xcc) {
      if (frame) throw new Error('лист: в JPEG второй кадр (SOF)')
      if (m !== 0xc0 && m !== 0xc1) throw new Error('лист: JPEG не baseline (нужен SOF0/SOF1; прогрессивный и иные — отказ)')
      if (len < 8) throw new Error('лист: усечённый SOF')
      const comps = data[p + 9]
      if (data[p + 4] !== 8) throw new Error('лист: точность JPEG не 8 бит')
      if (comps !== 1 && comps !== 3) throw new Error(`лист: компонентов ${comps}, нужен 1 (серый) или 3 (YCbCr)`)
      if (len !== 8 + 3 * comps) throw new Error('лист: длина SOF не сходится с числом компонентов')
      frame = { marker: m, w: data.readUInt16BE(p + 7), h: data.readUInt16BE(p + 5), comps }
      ids = [data[p + 10], data[p + 13], data[p + 16]]
      sealed = jfif
    }
    p += 2 + len
  }
  throw new Error(`лист: больше ${MAX_MARKERS} сегментов до развёртки`)
}

/** Как `decoder.isRGB` в Go: нет JFIF, и Adobe transform 0 либо id компонентов R, G, B. */
const isRGB = (jfif, adobe, ids) => !jfif && (adobe === 0 || (ids[0] === 0x52 && ids[1] === 0x47 && ids[2] === 0x42))

/** preview.json -> {w, h, rows[{kind, w, h, items}]} по пределам design §2; чужое поле, тип или предел — исключение. */
export function parseMeta(text) {
  let v
  try {
    v = JSON.parse(text)
  } catch {
    throw new Error('preview.json: не JSON')
  }
  const obj = (x, what, keys) => {
    if (x === null || typeof x !== 'object' || Array.isArray(x)) throw new Error(`preview.json: ${what} — не объект`)
    const extra = Object.keys(x).find((k) => !keys.includes(k))
    if (extra) throw new Error(`preview.json: ${what}: неизвестное поле ${extra}`)
  }
  obj(v, 'корень', ['w', 'h', 'rows'])
  if (!isInt(v.w, 1, MAX_W) || !isInt(v.h, 1, MAX_H) || v.w * v.h > MAX_PIXELS) throw new Error(`preview.json: лист ${v.w}×${v.h} вне пределов (${MAX_W}×${MAX_H}, ${MAX_PIXELS >> 20} Мп)`)
  if (!Array.isArray(v.rows) || v.rows.length < 1 || v.rows.length > MAX_ROWS) throw new Error(`preview.json: рядов 1…${MAX_ROWS}`)
  const kinds = new Set()
  let sumH = 0
  let maxW = 0
  const rows = v.rows.map((r, i) => {
    obj(r, `ряд ${i + 1}`, ['kind', 'w', 'h', 'items'])
    if (!ROW_KINDS.includes(r.kind) || kinds.has(r.kind)) throw new Error(`preview.json: ряд ${i + 1}: вид ${r.kind} неизвестен или повторён`)
    kinds.add(r.kind)
    if (!isInt(r.w, CELL_MIN, CELL_MAX) || !isInt(r.h, CELL_MIN, CELL_MAX)) throw new Error(`preview.json: ряд ${i + 1}: клетка ${r.w}×${r.h} вне ${CELL_MIN}…${CELL_MAX}`)
    if (!Array.isArray(r.items) || r.items.length < 1 || r.items.length > MAX_ITEMS) throw new Error(`preview.json: ряд ${i + 1}: items 1…${MAX_ITEMS}`)
    if (!r.items.every((s) => typeof s === 'string' && ITEM_RE.test(s)) || new Set(r.items).size !== r.items.length) throw new Error(`preview.json: ряд ${i + 1}: items — 16 hex без повторов`)
    sumH += r.h
    maxW = Math.max(maxW, r.items.length * r.w)
    return { kind: r.kind, w: r.w, h: r.h, items: [...r.items] }
  })
  if (maxW > v.w || sumH > v.h) throw new Error(`preview.json: ряды (${maxW}×${sumH}) не помещаются в лист ${v.w}×${v.h}`)
  return { w: v.w, h: v.h, rows }
}

/** Каждый items — sha256[:16] предмета манифеста того же вида (Н5): иначе плитка поведёт на предмет, которого в zip нет. */
export function checkItems(meta, manifest) {
  for (const row of meta.rows) {
    const have = new Set(manifest.items.filter((it) => it.kind === row.kind).map((it) => String(it.sha256).slice(0, 16)))
    const miss = row.items.find((s) => !have.has(s))
    if (miss) throw new Error(`preview.json: ${row.kind} ${miss} нет в манифесте набора ${manifest.id}`)
  }
}

/** Поле `preview` строки каталога по листу и preview.json; sha256 и размер считает сам. Любое нарушение — исключение. */
export function previewField(jpeg, metaText, manifest, urlPrefix) {
  if (jpeg.length < 1 || jpeg.length > MAX_PREVIEW) throw new Error(`лист: ${jpeg.length} байт вне 1…${MAX_PREVIEW}`)
  const meta = parseMeta(metaText)
  const fr = jpegFrame(jpeg)
  if (fr.w !== meta.w || fr.h !== meta.h) throw new Error(`лист: кадр JPEG ${fr.w}×${fr.h}, а в preview.json ${meta.w}×${meta.h}: клиент отвергнет лист`)
  checkItems(meta, manifest)
  const field = {
    url: `${urlPrefix}${manifest.id}-v${manifest.version}/${manifest.id}-${manifest.version}-preview.jpg`,
    sha256: crypto.createHash('sha256').update(jpeg).digest('hex'),
    size: jpeg.length,
    w: meta.w,
    h: meta.h,
    rows: meta.rows,
  }
  if (Buffer.byteLength(JSON.stringify(field)) > MAX_RAW) throw new Error(`поле preview больше ${MAX_RAW >> 10} КиБ: клиент его не разберёт`)
  return field
}

/** Лист и preview.json набора из папки `-sprite -out`; предел размера — до чтения. */
export function readSheet(dir, manifest, lib) {
  const base = path.join(dir, `${manifest.id}-${manifest.version}-preview`)
  return {
    jpeg: lib.readLimited(`${base}.jpg`, MAX_PREVIEW, 'лист'),
    metaText: lib.readLimited(`${base}.json`, MAX_META, 'preview.json').toString('utf8'),
  }
}

/** Тело каталога не больше ¾ предела старых клиентов; возвращает предупреждение (от 512 КиБ) или ''. */
export function checkBody(cat) {
  const n = Buffer.byteLength(JSON.stringify(cat))
  if (n > BODY_FAIL) throw new Error(`тело каталога ${n} байт больше ${BODY_FAIL >> 10} КиБ (¾ предела 1 МиБ у клиентов 0.17+): уберите превью или наборы`)
  return n >= BODY_WARN ? `тело каталога ${n} байт: от ${BODY_WARN >> 10} КиБ предел ${BODY_FAIL >> 10} КиБ близко` : ''
}

export const isPreviewCmd = (argv) => argv.includes('--preview-only')

/** Значения повторяемого ключа. */
const all = (argv, name) => argv.flatMap((a, i) => (a === name ? [argv[i + 1]] : []))

const VALUE_FLAGS = ['--preview-only', '--sheets', '--catalog', '--key', '--seq']

/** Каталог после замены `preview` в строках: меняется только это поле, прочее сверяется до подписи. */
export function replacePreviews(base, fields, seq) {
  const packs = base.packs.map((p) => (fields.has(p.id) ? { ...p, preview: fields.get(p.id) } : p))
  const next = { ...base, format: 1, seq, packs }
  const strip = (p) => JSON.stringify(Object.fromEntries(Object.entries(p).filter(([k]) => k !== 'preview')))
  const same = packs.length === base.packs.length && packs.every((p, i) => p.id === base.packs[i].id && strip(p) === strip(base.packs[i]))
  const untouched = (c) => JSON.stringify(Object.fromEntries(Object.entries(c).filter(([k]) => k !== 'packs' && k !== 'seq')))
  if (!same || untouched(next) !== untouched({ ...base, format: 1 })) throw new Error('внутренняя ошибка: изменилось не только preview')
  for (const [id] of fields) if (!base.packs.some((p) => p.id === id)) throw new Error(`набора ${id} нет в каталоге`)
  return next
}

/**
 * Режим --preview-only. lib: readLimited, readPrev, readCatalogFile, sealCatalog, URL_PREFIX, MAX_ZIP_WAVE2.
 * root — корень репозитория (ключ по умолчанию). Ошибка — исключение (командной строки — code 2); каталог пишется
 * только после всех проверок, через временный файл.
 */
export function previewMain(argv, lib, root) {
  const unknown = argv.find((a) => a.startsWith('--') && !VALUE_FLAGS.includes(a))
  if (unknown) throw usage(`неизвестный ключ ${unknown} (режим --preview-only)`)
  for (const f of VALUE_FLAGS) {
    const at = argv.indexOf(f)
    for (let i = at; i >= 0; i = argv.indexOf(f, i + 1)) if (argv[i + 1] === undefined || argv[i + 1].startsWith('--')) throw usage(`${f}: нужно значение`)
  }
  if (argv.some((a, i) => !a.startsWith('--') && !VALUE_FLAGS.includes(argv[i - 1]))) throw usage('--preview-only не принимает папку набора')
  const one = (name) => {
    const v = all(argv, name)
    if (v.length > 1) throw usage(`${name} задан несколько раз`)
    return v[0]
  }
  const pairs = all(argv, '--preview-only').map((s) => {
    const at = s.indexOf('=')
    if (at < 1 || at === s.length - 1) throw usage(`--preview-only ${s}: нужно <id>=<zip>`)
    return { id: s.slice(0, at), zip: path.resolve(s.slice(at + 1)) }
  })
  const dup = pairs.find((p, i) => pairs.findIndex((q) => q.id === p.id) !== i)
  if (dup) throw usage(`--preview-only: набор ${dup.id} назван дважды`)
  const sheets = one('--sheets')
  const catArg = one('--catalog')
  if (!sheets || !catArg) throw usage('--preview-only: нужны --sheets <папка> и --catalog <каталог>')
  const catPath = path.resolve(catArg)
  const keyPath = path.resolve(one('--key') ?? path.join(root, 'docs', 'private', 'library-key.pem'))
  if (!fs.existsSync(keyPath)) throw usage(`нет ключа подписи: ${keyPath} (для QA — --test-key и --key)`)
  const base = lib.readCatalogFile(catPath)
  if (!base) throw usage(`нет каталога ${catPath}`)
  // Всё, что может отказать, — до записи. Провалы всех пар собираются вместе: один прогон — все причины.
  const fields = new Map()
  const fails = []
  for (const { id, zip } of pairs) {
    try {
      const row = base.packs.find((p) => p.id === id)
      if (!row) throw new Error(`набора ${id} нет в каталоге`)
      const prev = lib.readPrev(lib.readLimited(zip, lib.MAX_ZIP_WAVE2, 'zip'))
      if (prev.manifest.id !== id || prev.manifest.version !== row.version) throw new Error(`архив — набор ${prev.manifest.id} v${prev.manifest.version}, в каталоге ${id} v${row.version}`)
      if (prev.sha256 !== row.sha256) throw new Error('sha256 архива не совпадает со строкой каталога: нужен именно опубликованный zip')
      const { jpeg, metaText } = readSheet(path.resolve(sheets), prev.manifest, lib)
      fields.set(id, previewField(jpeg, metaText, prev.manifest, lib.URL_PREFIX))
    } catch (e) {
      fails.push(`${id}: ${e.message}`)
    }
  }
  if (pairs.length === 0) fails.push('нужна хотя бы одна пара --preview-only <id>=<zip>')
  if (fails.length > 0) throw new Error(`отказ (${fails.length}), ничего не записано:\n - ${fails.join('\n - ')}`)
  const prevSeq = Number.isInteger(base.seq) && base.seq > 0 ? base.seq : 0
  const seqArg = one('--seq')
  const seq = seqArg === undefined ? prevSeq + 1 : Number(seqArg)
  if (!Number.isInteger(seq) || seq <= prevSeq) throw new Error(`seq ${seqArg} должен быть целым и больше seq прежнего каталога (${prevSeq})`)
  const cat = replacePreviews(base, fields, seq)
  const warn = checkBody(cat)
  const env = lib.sealCatalog(cat, crypto.createPrivateKey(fs.readFileSync(keyPath)))
  if (warn) console.warn(`предупреждение: ${warn}`)
  const tmp = `${catPath}.tmp`
  fs.writeFileSync(tmp, JSON.stringify(env) + '\n')
  fs.renameSync(tmp, catPath) // каталог меняется целиком или не меняется
  console.log(`превью: ${[...fields.keys()].join(', ')}; каталог (seq ${seq}) подписан, архивы не менялись`)
}
