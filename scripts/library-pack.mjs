#!/usr/bin/env node
// Выпуск набора библиотеки (спека 067, design §4–§5, §12): папка набора -> zip с manifest.json,
// строка в catalog.json — конверт с подписью ed25519 (0.17 decisions №2). Только node:*, без зависимостей.
//
// Папка набора: manifest.src.json (id, version, minApp, title, about, author, license, source и
// items[{file, kind, theme, name}] — sha256 и size считает скрипт) + файлы предметов + LICENSE.txt.
//
//   node scripts/library-pack.mjs <папка набора> [--out dist] [--catalog catalog.json] [--key pem] [--seq N] [--new]
//                                    [--prev прежний.zip [--drop файл]...]
//   node scripts/library-pack.mjs --test-key     одноразовый ключ в .gotmp/ + открытый ключ (base64, 32 байта)
//   node scripts/library-pack.mjs --help
//
// Скрипт всегда подписывает. Локальная сборка без боевого ключа: `--test-key`, затем
// `<папка набора> --new --key .gotmp/library-test-key.pem --out .gotmp/lib` (первый набор нового
// каталога; следующие — без --new). Программа примет такой каталог только со своим открытым ключом
// (SA_LIBRARY_KEY в сборке dev, docs/qa/library.md); публикуется набор, собранный боевым ключом.
//
// Все проверки — до записи: при отказе папка вывода и каталог не меняются.

// catalog.json = {"sig": base64 ed25519 по байтам body, "body": base64 внутреннего JSON {format:1, seq, packs}}.
// seq > 0: --seq N (обязательно больше seq прежнего каталога: клиент не принимает каталог с меньшим seq,
// и одна такая публикация ломает проверку у всех, кто принял больший), иначе seq прежнего + 1.
// Нет файла каталога (опечатка в --catalog и т. п.) — отказ; новый каталог с seq 1 — только с явным --new.
// Файла catalog.json.sig больше нет.
// --prev <zip>: опубликованный zip прежней версии; каждый ключ (sha256 картинки bg/sticker/frame) обязан
// остаться в новой, иначе отказ — кроме файлов, названных явным --drop (имя в прежнем манифесте).
// Если набор с этим id уже есть в каталоге, --prev обязателен, его id совпадает с набором, а sha256 — со
// строкой каталога (истина — опубликованный zip, а не папка на одной машине).
// Порядок строк каталога не сортируется: прежние на месте, новая в конец (порядок установки = порядок выпуска).
// zip больше 70 МБ — отказ. manifest.src.json `needs: "<id донора>"` -> поле `needs` строки каталога (minApp от 0.27.0);
// донор обязан быть в каталоге и стоять в нём раньше (порядок строк = порядок установки).
// Стикер — сторона до 4096 px, фон — до 100 Мп (по заголовку PNG/JPEG, без декодирования).
//
// Закрытый ключ (по умолчанию docs/private/library-key.pem, вне git) в вывод не печатается.
import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import zlib from 'node:zlib'
import { fileURLToPath, pathToFileURL } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
export const URL_PREFIX = 'https://github.com/IliaSA101/sosninaalbums-library/releases/download/'
const NAME_RE = /^[a-z0-9][a-z0-9._-]{0,63}(\/[a-z0-9][a-z0-9._-]{0,63}){0,2}$/
const ID_RE = /^[a-z0-9][a-z0-9-]{2,39}$/
const LIMITS = { bg: 60 << 20, sticker: 30 << 20, spread: 256 << 10, frame: 30 << 20, print: 64 << 10 }
const KINDS = Object.keys(LIMITS)
const FRAME_MIN_APP = [0, 20, 0] // рамочки-картинки умеют только версии с 0.20.0 (079)
const FRAME_MAX_SIDE = 4096 // как decor.MaxFrameSide: больше приложение не рисует
const PRINT_MIN_APP = [0, 26, 0] // профили типографий (.saprint) умеют только версии с 0.26.0 (123)
const NEEDS_MIN_APP = [0, 27, 0] // 0.26 поле needs не знает и дополнение без донора поставит (0.27-129-12)
export const MAX_ZIP = 70 << 20 // бюджет набора (0.27-129-13): 70 МБ за 15 мин — от 0,62 Мбит/с
const STICKER_MAX_SIDE = 4096 // как packMaxSide в decor: больше стикер не ставится
const BG_MAX_PIXELS = 100_000_000 // как packMaxBgPixels
const MAX_SRC = 1 << 20 // manifest.src.json: 1000 предметов — десятки КБ
const MAX_CATALOG = 8 << 20 // файл каталога: 500 строк — сотни КБ
const KEYED_KINDS = ['bg', 'sticker', 'frame'] // ключи, которые нельзя терять между версиями
const MAX_ENTRY = 64 << 20 // запись zip при чтении --prev: предел вида (60 МБ) с запасом

/** x.y.z -> число для сравнения версий. */
const verNum = (v) => v.split('.').reduce((a, n) => a * 1e6 + Number(n), 0)

const PNG_SIG = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])

/** Размеры PNG по IHDR (первые 24 байта); не PNG — исключение. */
export function pngSize(data) {
  if (data.length < 24 || !data.subarray(0, 8).equals(PNG_SIG) || data.toString('latin1', 12, 16) !== 'IHDR') throw new Error('нет PNG-заголовка')
  return { w: data.readUInt32BE(16), h: data.readUInt32BE(20) }
}

/** Размеры JPEG по первому SOF (без декодирования); усечённый или без SOF — исключение. */
export function jpegSize(data) {
  if (data.length < 4 || data[0] !== 0xff || data[1] !== 0xd8) throw new Error('нет JPEG-заголовка')
  let p = 2
  while (p + 4 <= data.length) {
    if (data[p] !== 0xff) throw new Error('JPEG: битая разметка')
    const m = data[p + 1]
    if (m === 0xff) { // заполнитель между сегментами
      p++
      continue
    }
    if (m === 0x01 || (m >= 0xd0 && m <= 0xd8)) { // маркеры без длины
      p += 2
      continue
    }
    if (m === 0xd9 || m === 0xda) break // конец или сами данные: SOF не встретился
    const len = data.readUInt16BE(p + 2)
    if (len < 2) throw new Error('JPEG: битая длина сегмента')
    if (m >= 0xc0 && m <= 0xcf && m !== 0xc4 && m !== 0xc8 && m !== 0xcc) {
      if (len < 8 || p + 9 > data.length) throw new Error('JPEG: усечённый SOF')
      return { w: data.readUInt16BE(p + 7), h: data.readUInt16BE(p + 5) }
    }
    p += 2 + len
  }
  throw new Error('JPEG: нет SOF')
}

/** Вид и размеры растра по заголовку: PNG или JPEG; иное — исключение. */
export function imageInfo(data) {
  if (data.length >= 8 && data.subarray(0, 8).equals(PNG_SIG)) return { type: 'png', ...pngSize(data) }
  if (data.length >= 2 && data[0] === 0xff && data[1] === 0xd8) return { type: 'jpeg', ...jpegSize(data) }
  throw new Error('не PNG и не JPEG')
}

// ---- zip (Store/Deflate, без зависимостей) ----

const DOS_TIME = 0 // фиксированное время: один и тот же набор даёт тот же архив
const DOS_DATE = (1 << 5) | 1 // 1980-01-01

/** files: [{name, data}] -> Buffer zip. Картинки уже сжаты — для них Store, для json — Deflate. */
export function makeZip(files) {
  const parts = []
  const central = []
  let offset = 0
  for (const f of files) {
    const name = Buffer.from(f.name, 'utf8')
    const crc = zlib.crc32(f.data)
    const deflated = zlib.deflateRawSync(f.data, { level: 9 })
    const useDeflate = (f.deflate ?? (f.name.endsWith('.json') || f.name.endsWith('.saprint'))) && deflated.length < f.data.length
    const body = useDeflate ? deflated : f.data
    const method = useDeflate ? 8 : 0
    const h = Buffer.alloc(30)
    h.writeUInt32LE(0x04034b50, 0)
    h.writeUInt16LE(20, 4)
    h.writeUInt16LE(0x0800, 6) // имена UTF-8
    h.writeUInt16LE(method, 8)
    h.writeUInt16LE(DOS_TIME, 10)
    h.writeUInt16LE(DOS_DATE, 12)
    h.writeUInt32LE(crc, 14)
    h.writeUInt32LE(body.length, 18)
    h.writeUInt32LE(f.data.length, 22)
    h.writeUInt16LE(name.length, 26)
    parts.push(h, name, body)
    const c = Buffer.alloc(46)
    c.writeUInt32LE(0x02014b50, 0)
    c.writeUInt16LE(20, 4)
    c.writeUInt16LE(20, 6)
    c.writeUInt16LE(0x0800, 8)
    c.writeUInt16LE(method, 10)
    c.writeUInt16LE(DOS_TIME, 12)
    c.writeUInt16LE(DOS_DATE, 14)
    c.writeUInt32LE(crc, 16)
    c.writeUInt32LE(body.length, 20)
    c.writeUInt32LE(f.data.length, 24)
    c.writeUInt16LE(name.length, 28)
    c.writeUInt32LE(offset, 42)
    central.push(c, name)
    offset += 30 + name.length + body.length
  }
  const cd = Buffer.concat(central)
  const end = Buffer.alloc(22)
  end.writeUInt32LE(0x06054b50, 0)
  end.writeUInt16LE(files.length, 8)
  end.writeUInt16LE(files.length, 10)
  end.writeUInt32LE(cd.length, 12)
  end.writeUInt32LE(offset, 16)
  return Buffer.concat([...parts, cd, end])
}

/** Читает zip (для теста и самопроверки): Map имя -> Buffer. */
export function readZip(buf) {
  const eocd = buf.lastIndexOf(Buffer.from([0x50, 0x4b, 0x05, 0x06]))
  if (eocd < 0) throw new Error('zip: нет конца каталога')
  const n = buf.readUInt16LE(eocd + 10)
  let p = buf.readUInt32LE(eocd + 16)
  const out = new Map()
  for (let i = 0; i < n; i++) {
    if (buf.readUInt32LE(p) !== 0x02014b50) throw new Error('zip: битый каталог')
    const method = buf.readUInt16LE(p + 10)
    const crc = buf.readUInt32LE(p + 16)
    const csize = buf.readUInt32LE(p + 20)
    const usize = buf.readUInt32LE(p + 24)
    const nl = buf.readUInt16LE(p + 28)
    const lho = buf.readUInt32LE(p + 42)
    const name = buf.toString('utf8', p + 46, p + 46 + nl)
    const start = lho + 30 + buf.readUInt16LE(lho + 26) + buf.readUInt16LE(lho + 28)
    if (usize > MAX_ENTRY) throw new Error(`zip: ${name} больше ${MAX_ENTRY >> 20} МБ`)
    const raw = buf.subarray(start, start + csize)
    const data = method === 8 ? zlib.inflateRawSync(raw, { maxOutputLength: usize }) : raw
    if (data.length !== usize || zlib.crc32(data) !== crc) throw new Error(`zip: ${name} не сошёлся`)
    out.set(name, Buffer.from(data))
    p += 46 + nl
  }
  return out
}

// ---- набор ----

const sha256 = (b) => crypto.createHash('sha256').update(b).digest('hex')

/** Читает файл, сперва сверив размер с пределом (огромный файл в память не читается). */
function readLimited(file, max, what) {
  const st = fs.statSync(file)
  if (!st.isFile()) throw new Error(`${what}: не обычный файл`)
  if (st.size > max) throw new Error(`${what}: ${st.size} байт больше предела ${max}`)
  return fs.readFileSync(file)
}

/** Файл профиля типографии: JSON-объект с именем и id (полный разбор строго — в программе, printprof.Parse). */
export function checkPrintFile(name, data) {
  let v
  try {
    v = JSON.parse(data.toString('utf8'))
  } catch {
    throw new Error(`${name}: профиль типографии — не JSON`)
  }
  if (v === null || typeof v !== 'object' || Array.isArray(v)) throw new Error(`${name}: профиль типографии — не JSON-объект`)
  if (v.format !== 1) throw new Error(`${name}: format профиля — 1`)
  if (typeof v.id !== 'string' || !ID_RE.test(v.id)) throw new Error(`${name}: id профиля не по грамматике`)
  if (typeof v.name !== 'string' || v.name.trim() === '') throw new Error(`${name}: у профиля нет name`)
}

/** Фон и стикер: PNG/JPEG по заголовку, расширение сходится с содержимым, предел фона (Мп) и стикера (сторона). */
function checkRaster(it, data) {
  let info
  try {
    info = imageInfo(data)
  } catch (e) {
    throw new Error(`${it.file}: ${e.message}`)
  }
  const ext = path.extname(it.file).toLowerCase()
  if ((info.type === 'png') !== (ext === '.png') || (info.type === 'jpeg') !== (ext === '.jpg' || ext === '.jpeg')) throw new Error(`${it.file}: содержимое (${info.type}) не совпадает с расширением`)
  if (info.w < 1 || info.h < 1) throw new Error(`${it.file}: размер картинки ${info.w}×${info.h}`)
  if (it.kind === 'bg' && info.w * info.h > BG_MAX_PIXELS) throw new Error(`${it.file}: фон ${info.w}×${info.h} больше ${BG_MAX_PIXELS / 1e6} Мп`)
  if (it.kind === 'sticker' && Math.max(info.w, info.h) > STICKER_MAX_SIDE) throw new Error(`${it.file}: сторона стикера ${info.w}×${info.h} больше ${STICKER_MAX_SIDE}`)
}

/** Читает папку набора -> {manifest, files:[{name,data}]}; нарушение предела — исключение. */
export function buildPack(dir) {
  const src = JSON.parse(readLimited(path.join(dir, 'manifest.src.json'), MAX_SRC, 'manifest.src.json').toString('utf8'))
  if (!ID_RE.test(src.id ?? '')) throw new Error('id набора не по грамматике')
  if (!Number.isInteger(src.version) || src.version < 1 || src.version > 999999) throw new Error('version 1…999999')
  if (!/^\d+\.\d+\.\d+$/.test(src.minApp ?? '')) throw new Error('minApp — x.y.z')
  if (!src.title) throw new Error('нет title')
  if (!Array.isArray(src.items) || src.items.length < 1 || src.items.length > 1000) throw new Error('items 1…1000')
  const seen = new Set()
  const files = []
  const items = src.items.map((it) => {
    if (!NAME_RE.test(it.file) || it.file.includes('..')) throw new Error(`имя файла не по грамматике: ${it.file}`)
    if (seen.has(it.file.toLowerCase())) throw new Error(`повтор файла: ${it.file}`)
    seen.add(it.file.toLowerCase())
    if (!KINDS.includes(it.kind)) throw new Error(`kind ${it.kind}`)
    if (it.kind === 'frame' && !it.file.endsWith('.png')) throw new Error(`${it.file}: рамочка в наборе — только PNG`)
    if (it.kind === 'print' && !it.file.endsWith('.saprint')) throw new Error(`${it.file}: типография в наборе — только .saprint`)
    const file = path.join(dir, ...it.file.split('/'))
    const size = fs.statSync(file).size
    if (size < 1 || size > LIMITS[it.kind]) throw new Error(`${it.file}: размер вне предела вида`)
    const data = fs.readFileSync(file)
    if (it.kind === 'bg' || it.kind === 'sticker') checkRaster(it, data)
    if (it.kind === 'frame') {
      if (verNum(src.minApp) < verNum(FRAME_MIN_APP.join('.'))) throw new Error(`${it.file}: набор с рамочками требует minApp не ниже ${FRAME_MIN_APP.join('.')}`)
      const { w, h } = pngSize(data)
      if (w < 1 || h < 1 || Math.max(w, h) > FRAME_MAX_SIDE) throw new Error(`${it.file}: сторона PNG ${w}×${h} вне 1…${FRAME_MAX_SIDE}`)
    }
    if (it.kind === 'print') {
      if (verNum(src.minApp) < verNum(PRINT_MIN_APP.join('.'))) throw new Error(`${it.file}: набор с типографиями требует minApp не ниже ${PRINT_MIN_APP.join('.')}`)
      checkPrintFile(it.file, data)
    }
    files.push({ name: it.file, data })
    return { file: it.file, kind: it.kind, theme: it.theme ?? 'basic', name: it.name, sha256: sha256(data), size: data.length }
  })
  let needs
  if (src.needs !== undefined) {
    if (typeof src.needs !== 'string' || !ID_RE.test(src.needs)) throw new Error('needs — id набора-донора по грамматике')
    if (src.needs === src.id) throw new Error('needs указывает на сам набор')
    if (verNum(src.minApp) < verNum(NEEDS_MIN_APP.join('.'))) throw new Error(`набор с needs требует minApp не ниже ${NEEDS_MIN_APP.join('.')}`)
    needs = src.needs
  }
  const manifest = {
    format: 1,
    id: src.id,
    version: src.version,
    minApp: src.minApp,
    title: src.title,
    about: src.about ?? '',
    author: src.author ?? '',
    license: src.license ?? '',
    source: src.source ?? '',
    items,
  }
  const mdata = Buffer.from(JSON.stringify(manifest, null, 2) + '\n')
  if (mdata.length > 256 << 10) throw new Error('манифест больше 256 КБ')
  const lic = path.join(dir, 'LICENSE.txt')
  const extra = fs.existsSync(lic) ? [{ name: 'LICENSE.txt', data: readLimited(lic, 1 << 20, 'LICENSE.txt') }] : []
  // needs — не в манифесте (формат 1 не меняем), а только в строке каталога
  return { manifest, needs, files: [{ name: 'manifest.json', data: mdata }, ...files, ...extra] }
}

/** Отказ, если zip больше бюджета набора. */
export function assertZipSize(zip) {
  if (zip.length > MAX_ZIP) throw new Error(`zip ${(zip.length / (1 << 20)).toFixed(1)} МБ больше предела ${MAX_ZIP >> 20} МБ: уберите предметы из набора`)
}

/** Ключи прежней версии (sha256 bg/sticker/frame) должны остаться в новой; drops — имена файлов прежнего манифеста. */
export function checkPrev(prevManifest, manifest, drops = []) {
  const have = new Set(manifest.items.map((it) => it.sha256))
  const prevFiles = new Set(prevManifest.items.map((it) => it.file))
  for (const d of drops) if (!prevFiles.has(d)) throw new Error(`--drop ${d}: такого файла нет в прежней версии`)
  const lost = prevManifest.items.filter((it) => KEYED_KINDS.includes(it.kind) && !have.has(it.sha256) && !drops.includes(it.file))
  if (lost.length > 0) throw new Error(`из новой версии пропали ключи прежней (${lost.length}): ${lost.slice(0, 5).map((it) => it.file).join(', ')}${lost.length > 5 ? ', …' : ''}; нужное — вернуть, намеренное — явный --drop <файл>`)
}

/** Прежний опубликованный архив: Buffer zip -> {manifest, sha256}. */
export function readPrev(zipBuf) {
  const mf = readZip(zipBuf).get('manifest.json')
  if (!mf) throw new Error('--prev: в архиве нет manifest.json')
  return { manifest: JSON.parse(mf.toString('utf8')), sha256: sha256(zipBuf) }
}

/**
 * Проверки выпуска против прежнего каталога (до записи любых файлов). Набор уже в каталоге — --prev обязателен
 * (иначе заморозка ключей по желанию), его id совпадает, а sha256 — со строкой каталога. Донор `needs` есть в
 * каталоге и стоит раньше этой строки (новая встанет в конец, прежняя остаётся на месте).
 */
export function checkPublish(base, manifest, needs, prev, drops = []) {
  const packs = base?.packs ?? []
  const at = packs.findIndex((p) => p.id === manifest.id)
  const row = at >= 0 ? packs[at] : null
  if (row && !prev) throw new Error(`в каталоге уже есть ${manifest.id} v${row.version}: нужен --prev <опубликованный zip> (заморозка ключей)`)
  if (prev) {
    if (prev.manifest.id !== manifest.id) throw new Error(`--prev: архив набора ${prev.manifest.id}, а собирается ${manifest.id}`)
    if (row && prev.sha256 !== row.sha256) throw new Error(`--prev: sha256 архива не совпадает со строкой каталога ${row.id} v${row.version} — нужен именно опубликованный zip`)
    checkPrev(prev.manifest, manifest, drops)
  }
  if (needs) {
    const donor = packs.findIndex((p) => p.id === needs)
    if (donor < 0) throw new Error(`needs: донора ${needs} нет в каталоге: сначала выпустите его`)
    if (donor > (at >= 0 ? at : packs.length)) throw new Error(`needs: донор ${needs} стоит в каталоге после ${manifest.id}: порядок строк — порядок установки`)
  }
}

/**
 * Предупреждения о порядке каталога: дополнение (`needs`) раньше своего донора. Клиент 0.27 ставит донор → дополнение
 * независимо от порядка строк, но 0.26 и старше порядок строк — порядок установки; отказывать нельзя (прежний каталог
 * уже подписан, строки не переставить), поэтому — текст, а новая строка в таком порядке отсекается в checkPublish.
 */
export function orderWarnings(cat) {
  const packs = cat?.packs ?? []
  const out = []
  packs.forEach((p, i) => {
    if (!p.needs) return
    const donor = packs.findIndex((d) => d.id === p.needs)
    if (donor > i) out.push(`дополнение ${p.id} стоит в каталоге раньше донора ${p.needs}`)
  })
  return out
}

/** Строка каталога по собранному архиву. */
export function catalogRow(manifest, zip, tag, needs) {
  const counts = { bg: 0, sticker: 0, spread: 0, frame: 0, print: 0 }
  for (const it of manifest.items) counts[it.kind]++
  const url = `${URL_PREFIX}${manifest.id}-v${manifest.version}/${tag}`
  return {
    id: manifest.id,
    version: manifest.version,
    minApp: manifest.minApp,
    title: manifest.title,
    about: manifest.about,
    license: manifest.license,
    size: zip.length,
    sha256: sha256(zip),
    url,
    counts,
    ...(needs ? { needs } : {}),
  }
}

/** Добавляет/заменяет строку по id; возвращает новый каталог. Старее стоящей версии — отказ. Порядок не сортируется: прежняя строка остаётся на месте, новая — в конец. */
export function upsertCatalog(cat, row, seq) {
  const packs = [...(cat?.packs ?? [])]
  const i = packs.findIndex((p) => p.id === row.id)
  if (i >= 0 && packs[i].version > row.version) throw new Error(`в каталоге версия ${packs[i].version} новее ${row.version}`)
  if (i >= 0) packs[i] = row
  else packs.push(row)
  if (packs.length > 500) throw new Error('в каталоге больше 500 наборов')
  const prev = Number.isInteger(cat?.seq) && cat.seq > 0 ? cat.seq : 0
  seq ??= prev + 1
  if (!Number.isInteger(seq) || seq <= 0) throw new Error('seq должен быть целым > 0')
  if (cat && seq <= prev) throw new Error(`seq ${seq} должен быть больше seq прежнего каталога (${prev})`)
  return { format: 1, seq, packs }
}

/** Прежний каталог обязателен: без файла — отказ, если не задан явный --new (тогда каталог начинается с seq 1). */
export function requireBase(cat, isNew) {
  if (!cat && !isNew) throw new Error('нет прежнего каталога: проверьте --catalog или передайте --new (новый каталог, seq 1)')
  if (cat && isNew) throw new Error('--new, но каталог уже есть: уберите --new или укажите другой --catalog')
  return cat
}

/** Конверт каталога: подпись — по точным байтам внутреннего JSON (до base64). */
export function sealCatalog(cat, privateKey) {
  const body = Buffer.from(JSON.stringify(cat))
  return { sig: signBytes(body, privateKey), body: body.toString('base64') }
}

/** Внутренний каталог из файла-конверта (без проверки подписи); null, если файла нет. */
export function readCatalogFile(file) {
  if (!fs.existsSync(file)) return null
  const env = JSON.parse(readLimited(file, MAX_CATALOG, 'каталог').toString('utf8'))
  return JSON.parse(Buffer.from(env.body, 'base64').toString('utf8'))
}

// ---- подпись ----

/** Открытый ключ — сырые 32 байта в base64 (как хранит программа). */
export function publicRaw(privateKey) {
  const der = crypto.createPublicKey(privateKey).export({ type: 'spki', format: 'der' })
  return der.subarray(der.length - 32).toString('base64')
}

export const signBytes = (data, privateKey) => crypto.sign(null, data, privateKey).toString('base64')

/** pubRaw — base64 сырых 32 байт; sig — base64. */
export function verifyBytes(data, sigB64, pubRaw) {
  const spki = Buffer.concat([Buffer.from('302a300506032b6570032100', 'hex'), Buffer.from(pubRaw, 'base64')])
  const key = crypto.createPublicKey({ key: spki, format: 'der', type: 'spki' })
  return crypto.verify(null, data, key, Buffer.from(sigB64, 'base64'))
}

// ---- CLI ----

const VALUE_FLAGS = ['--out', '--catalog', '--key', '--seq', '--prev', '--drop']
const BOOL_FLAGS = ['--new', '--test-key', '--help']
const USAGE = `использование: node scripts/library-pack.mjs <папка набора> [--out dist] [--catalog catalog.json] [--key pem] [--seq N] [--new] [--prev zip [--drop файл]...]
              node scripts/library-pack.mjs --test-key   (одноразовый ключ .gotmp/library-test-key.pem и открытый ключ)
Скрипт всегда подписывает. Локально: --test-key, затем <папка> --new --key .gotmp/library-test-key.pem --out .gotmp/lib
(следующие наборы — без --new). Набор уже в каталоге — нужен --prev <опубликованный zip>. Подробности — шапка файла.`

/** Значение ключа: следующий аргумент; нет значения — исключение. */
function arg(argv, name, def) {
  const i = argv.indexOf(name)
  if (i < 0) return def
  const v = argv[i + 1]
  if (v === undefined || v.startsWith('--')) throw new Error(`${name}: нужно значение`)
  return v
}

/** Отказ с текстом и кодом; до записи любых файлов. */
function die(msg, code = 1) {
  console.error(msg)
  process.exit(code)
}

function main(argv) {
  if (argv.includes('--help')) {
    console.log(USAGE)
    return
  }
  if (argv.includes('--test-key')) {
    const { privateKey } = crypto.generateKeyPairSync('ed25519')
    const file = path.join(root, '.gotmp', 'library-test-key.pem')
    fs.mkdirSync(path.dirname(file), { recursive: true })
    fs.writeFileSync(file, privateKey.export({ type: 'pkcs8', format: 'pem' }))
    console.log(`тестовый ключ: ${path.relative(root, file)}`)
    console.log(`открытый ключ (base64, 32 байта): ${publicRaw(privateKey)}`)
    return
  }
  const unknown = argv.find((a) => a.startsWith('--') && !VALUE_FLAGS.includes(a) && !BOOL_FLAGS.includes(a))
  if (unknown) die(`неизвестный ключ ${unknown}\n${USAGE}`, 2)
  const dir = argv.find((a, i) => !a.startsWith('--') && (i === 0 || !VALUE_FLAGS.includes(argv[i - 1])))
  if (!dir) die(USAGE, 2)
  let keyPath, outDir, catPath, prevPath, seqArg
  try {
    keyPath = path.resolve(arg(argv, '--key', path.join(root, 'docs', 'private', 'library-key.pem')))
    outDir = path.resolve(arg(argv, '--out', 'dist'))
    catPath = path.resolve(arg(argv, '--catalog', path.join(outDir, 'catalog.json')))
    prevPath = arg(argv, '--prev', undefined)
    seqArg = arg(argv, '--seq', undefined)
  } catch (e) {
    die(e.message, 2)
  }
  if (!fs.existsSync(keyPath)) die(`нет ключа подписи: ${keyPath} (для QA — --test-key и --key)`, 2)
  const privateKey = crypto.createPrivateKey(fs.readFileSync(keyPath))
  let base
  try {
    base = requireBase(readCatalogFile(catPath), argv.includes('--new'))
  } catch (e) {
    die(e.message, 2)
  }
  // Всё, что может отказать, — до первой записи: при отказе папка вывода и каталог не тронуты.
  let manifest, needs, zip, cat
  try {
    ;({ manifest, needs, files: zip } = buildPack(path.resolve(dir)))
    zip = makeZip(zip)
    assertZipSize(zip)
    let prev
    if (prevPath) prev = readPrev(readLimited(path.resolve(prevPath), MAX_ZIP, '--prev'))
    const drops = argv.flatMap((a, i) => (a === '--drop' ? [argv[i + 1]] : []))
    checkPublish(base, manifest, needs, prev, drops)
    const tag = `${manifest.id}-${manifest.version}.zip`
    cat = upsertCatalog(base, catalogRow(manifest, zip, tag, needs), seqArg === undefined ? undefined : Number(seqArg))
  } catch (e) {
    die(e.message)
  }
  for (const w of orderWarnings(cat)) console.warn(`предупреждение: ${w}`)
  const tag = `${manifest.id}-${manifest.version}.zip`
  fs.mkdirSync(outDir, { recursive: true })
  fs.writeFileSync(path.join(outDir, tag), zip)
  fs.mkdirSync(path.dirname(catPath), { recursive: true })
  const tmp = `${catPath}.tmp`
  fs.writeFileSync(tmp, JSON.stringify(sealCatalog(cat, privateKey)) + '\n')
  fs.renameSync(tmp, catPath) // каталог меняется целиком или не меняется
  fs.rmSync(catPath + '.sig', { force: true }) // отдельная подпись формата 0.16 больше не нужна
  console.log(`${tag}: ${zip.length} байт, предметов ${manifest.items.length}; каталог (seq ${cat.seq}) подписан`)
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) main(process.argv.slice(2))
