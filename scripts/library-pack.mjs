#!/usr/bin/env node
// Выпуск набора библиотеки (спека 067, design §4–§5, §12): папка набора -> zip с manifest.json,
// строка в catalog.json и подпись catalog.json.sig (ed25519). Только node:*, без зависимостей.
//
// Папка набора: manifest.src.json (id, version, minApp, title, about, author, license, source и
// items[{file, kind, theme, name}] — sha256 и size считает скрипт) + файлы предметов + LICENSE.txt.
//
//   node scripts/library-pack.mjs <папка набора> [--out dist] [--catalog catalog.json] [--key pem]
//   node scripts/library-pack.mjs --test-key     одноразовый ключ в .gotmp/ + открытый ключ (base64, 32 байта)
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
const LIMITS = { bg: 60 << 20, sticker: 30 << 20, spread: 256 << 10 }
const KINDS = Object.keys(LIMITS)

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
    const useDeflate = (f.deflate ?? f.name.endsWith('.json')) && deflated.length < f.data.length
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
    const raw = buf.subarray(start, start + csize)
    const data = method === 8 ? zlib.inflateRawSync(raw) : raw
    if (data.length !== usize || zlib.crc32(data) !== crc) throw new Error(`zip: ${name} не сошёлся`)
    out.set(name, Buffer.from(data))
    p += 46 + nl
  }
  return out
}

// ---- набор ----

const sha256 = (b) => crypto.createHash('sha256').update(b).digest('hex')

/** Читает папку набора -> {manifest, files:[{name,data}]}; нарушение предела — исключение. */
export function buildPack(dir) {
  const src = JSON.parse(fs.readFileSync(path.join(dir, 'manifest.src.json'), 'utf8'))
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
    const data = fs.readFileSync(path.join(dir, ...it.file.split('/')))
    if (data.length < 1 || data.length > LIMITS[it.kind]) throw new Error(`${it.file}: размер вне предела вида`)
    files.push({ name: it.file, data })
    return { file: it.file, kind: it.kind, theme: it.theme ?? 'basic', name: it.name, sha256: sha256(data), size: data.length }
  })
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
  const extra = fs.existsSync(lic) ? [{ name: 'LICENSE.txt', data: fs.readFileSync(lic) }] : []
  return { manifest, files: [{ name: 'manifest.json', data: mdata }, ...files, ...extra] }
}

/** Строка каталога по собранному архиву. */
export function catalogRow(manifest, zip, tag) {
  const counts = { bg: 0, sticker: 0, spread: 0 }
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
  }
}

/** Добавляет/заменяет строку по id; возвращает новый каталог. Старее стоящей версии — отказ. */
export function upsertCatalog(cat, row) {
  const packs = [...(cat?.packs ?? [])]
  const i = packs.findIndex((p) => p.id === row.id)
  if (i >= 0 && packs[i].version > row.version) throw new Error(`в каталоге версия ${packs[i].version} новее ${row.version}`)
  if (i >= 0) packs[i] = row
  else packs.push(row)
  packs.sort((a, b) => a.id.localeCompare(b.id))
  if (packs.length > 500) throw new Error('в каталоге больше 500 наборов')
  return { format: 1, packs }
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

function arg(argv, name, def) {
  const i = argv.indexOf(name)
  return i >= 0 ? argv[i + 1] : def
}

function main(argv) {
  if (argv.includes('--test-key')) {
    const { privateKey } = crypto.generateKeyPairSync('ed25519')
    const file = path.join(root, '.gotmp', 'library-test-key.pem')
    fs.mkdirSync(path.dirname(file), { recursive: true })
    fs.writeFileSync(file, privateKey.export({ type: 'pkcs8', format: 'pem' }))
    console.log(`тестовый ключ: ${path.relative(root, file)}`)
    console.log(`открытый ключ (base64, 32 байта): ${publicRaw(privateKey)}`)
    return
  }
  const dir = argv.find((a, i) => !a.startsWith('--') && (i === 0 || !argv[i - 1].startsWith('--')))
  if (!dir) {
    console.error('использование: node scripts/library-pack.mjs <папка набора> [--out dist] [--catalog catalog.json] [--key pem] | --test-key')
    process.exit(2)
  }
  const keyPath = path.resolve(arg(argv, '--key', path.join(root, 'docs', 'private', 'library-key.pem')))
  if (!fs.existsSync(keyPath)) {
    console.error(`нет ключа подписи: ${keyPath} (для QA — --test-key и --key)`)
    process.exit(2)
  }
  const privateKey = crypto.createPrivateKey(fs.readFileSync(keyPath))
  const outDir = path.resolve(arg(argv, '--out', 'dist'))
  const catPath = path.resolve(arg(argv, '--catalog', path.join(outDir, 'catalog.json')))
  const { manifest, files } = buildPack(path.resolve(dir))
  const zip = makeZip(files)
  const tag = `${manifest.id}-${manifest.version}.zip`
  fs.mkdirSync(outDir, { recursive: true })
  fs.writeFileSync(path.join(outDir, tag), zip)
  const old = fs.existsSync(catPath) ? JSON.parse(fs.readFileSync(catPath, 'utf8')) : null
  const cat = upsertCatalog(old, catalogRow(manifest, zip, tag))
  const catData = Buffer.from(JSON.stringify(cat, null, 2) + '\n')
  fs.writeFileSync(catPath, catData)
  fs.writeFileSync(catPath + '.sig', signBytes(catData, privateKey) + '\n')
  console.log(`${tag}: ${zip.length} байт, предметов ${manifest.items.length}; каталог и подпись обновлены`)
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) main(process.argv.slice(2))
