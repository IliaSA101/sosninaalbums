// Чтение zip набора (pack/eocd.go, archive.go) так же, как Go archive/zip: каталог по концу файла со сдвигом на данные
// перед архивом, записи до первого битого заголовка, обычный файл по создателю, имена как байты Go. Только чтение.
import crypto from 'node:crypto'
import fs from 'node:fs'
import { Readable } from 'node:stream'
import zlib from 'node:zlib'
import { LIMITS, PackError, clean, fail } from './base.mjs'
import { validName } from './manifest.mjs'

const LOC64 = 20 // локатор ZIP64
const END64 = 56 // конец каталога ZIP64
const FATAL = Symbol('fatal') // io.EOF на границе заголовка: archive/zip отказывает, а не останавливает чтение

/** Байты [pos, pos+len) файла: за пределами файла и при нечитаемой позиции — меньше или пусто. */
export function readAt(fd, pos, len) {
  if (!(pos >= 0 && pos <= Number.MAX_SAFE_INTEGER && len > 0)) return Buffer.alloc(0)
  const b = Buffer.alloc(len)
  return b.subarray(0, fs.readSync(fd, b, 0, len, pos))
}

/** Последняя подпись конца каталога в хвосте с комментарием, не вылезающим за конец; нет — −1 (eocdAt). */
function eocdAt(b) {
  for (let i = b.length - 22; i >= 0; i--) {
    if (b.readUInt32LE(i) === 0x06054b50) return b.readUInt16LE(i + 20) + 22 + i > b.length ? -1 : i
  }
  return -1
}

const num = (v) => (v > BigInt(Number.MAX_SAFE_INTEGER) ? Infinity : Number(v))
const badZip = () => fail('damaged', 'не читается как zip')

/** Конец каталога и его пределы до чтения записей (checkDir + readDirectoryEnd): записи, размер, смещение, где он кончается. */
function locateDir(fd, size) {
  const n = Math.min(size, LIMITS.tailLen)
  const buf = readAt(fd, size - n, n)
  const p = eocdAt(buf)
  if (p < 0) throw badZip()
  let end = size - n + p
  let records = BigInt(buf.readUInt16LE(p + 10))
  let dirSize = BigInt(buf.readUInt32LE(p + 12))
  let dirOff = BigInt(buf.readUInt32LE(p + 16))
  if (records === 0xffffn || dirSize === 0xffffn || dirOff === 0xffffffffn) {
    // условие archive/zip: локатор есть — запись ZIP64 обязана читаться, иначе отказ
    const loc = end >= LOC64 ? readAt(fd, end - LOC64, LOC64) : Buffer.alloc(0)
    if (loc.length === LOC64 && loc.readUInt32LE(0) === 0x07064b50 && loc.readUInt32LE(4) === 0 && loc.readUInt32LE(16) === 1) {
      const pb = loc.readBigUInt64LE(8)
      if (pb < 1n << 63n) { // int64 < 0 archive/zip за ZIP64 не считает
        const rec = readAt(fd, num(pb), END64)
        if (rec.length !== END64 || rec.readUInt32LE(0) !== 0x06064b50) throw badZip()
        end = num(pb)
        records = rec.readBigUInt64LE(32)
        dirSize = rec.readBigUInt64LE(40)
        dirOff = rec.readBigUInt64LE(48)
      }
    }
  }
  if (records > BigInt(LIMITS.maxEntries)) throw new PackError('other', `в архиве больше ${LIMITS.maxEntries} записей`)
  if (dirSize > BigInt(end)) throw badZip() // каталог до начала файла
  // archive/zip читает с end − dirSize, а если там нет заголовка и dirOff раньше — с dirOff: берём раннее из двух
  const start = Math.min(end - Number(dirSize), num(dirOff))
  if (dirSize > BigInt(LIMITS.maxDir) || size - start > LIMITS.maxDir + LIMITS.tailLen + LOC64 + END64) {
    throw fail('damaged', `каталог архива больше ${LIMITS.maxDir >> 20} МБ`)
  }
  if (!Number.isFinite(num(dirOff))) throw badZip()
  return { records: Number(records), dirSize: Number(dirSize), dirOff: num(dirOff), end, start }
}

/**
 * Запись центрального каталога по смещению p буфера b (readDirectoryHeader). Нет записи (подпись не та, обрыв, zip64 без
 * поля) — null: чтение каталога останавливается; ровно конец буфера на границе — FATAL.
 */
function readHeader(b, p) {
  const rem = b.length - p
  if (rem <= 0) return FATAL
  if (rem < 46 || b.readUInt32LE(p) !== 0x02014b50) return null
  const nl = b.readUInt16LE(p + 28)
  const el = b.readUInt16LE(p + 30)
  const cl = b.readUInt16LE(p + 32)
  const dl = nl + el + cl
  if (dl > 0 && rem === 46) return FATAL
  if (rem - 46 < dl) return null
  const e = {
    madeBy: b.readUInt16LE(p + 4),
    flags: b.readUInt16LE(p + 8),
    method: b.readUInt16LE(p + 10),
    crc: b.readUInt32LE(p + 16),
    csize: BigInt(b.readUInt32LE(p + 20)),
    usize: BigInt(b.readUInt32LE(p + 24)),
    ext: b.readUInt32LE(p + 38),
    lho: BigInt(b.readUInt32LE(p + 42)),
    name: goDecode(b.subarray(p + 46, p + 46 + nl)),
    next: p + 46 + dl,
  }
  const need = { usize: e.usize === 0xffffffffn, csize: e.csize === 0xffffffffn, lho: e.lho === 0xffffffffn }
  // поле ZIP64: значения берутся по порядку, только для «заполненных» полей; чужие поля и обрывок после — пропуск
  let q = p + 46 + nl
  const stop = q + el
  while (stop - q >= 4) {
    const tag = b.readUInt16LE(q)
    const fsize = b.readUInt16LE(q + 2)
    q += 4
    if (stop - q < fsize) break
    if (tag === 1) {
      let r = q
      for (const k of ['usize', 'csize', 'lho']) {
        if (!need[k]) continue
        if (q + fsize - r < 8) return null
        e[k] = b.readBigUInt64LE(r)
        r += 8
        need[k] = false
      }
    }
    q += fsize
  }
  if (need.csize || need.lho) return null
  e.csize = num(e.csize)
  e.usize = num(e.usize)
  e.lho = num(e.lho)
  return e
}

/** Записи каталога так, как их прочёл бы archive/zip: сдвиг base, чтение до первого битого заголовка, число по модулю 65536. */
export function readEntries(fd, size) {
  const d = locateDir(fd, size)
  const buf = readAt(fd, d.start, size - d.start)
  let base = d.end - d.dirSize - d.dirOff
  // данные перед архивом сдвигают смещения; но годный заголовок по dirOff — значит сдвига нет
  if (base > 0) {
    const h = readHeader(buf, d.dirOff - d.start)
    if (h !== null && h !== FATAL) base = 0
  }
  const list = []
  for (let p = base + d.dirOff - d.start; ; ) {
    const e = readHeader(buf, p)
    if (e === FATAL) throw badZip()
    if (e === null) break
    e.lho += base
    list.push(e)
    p = e.next
  }
  if ((list.length & 0xffff) !== (d.records & 0xffff)) throw badZip()
  if (list.length > LIMITS.maxEntries) throw new PackError('other', `в архиве больше ${LIMITS.maxEntries} записей`)
  return list
}

/** Имя записи как строка Go: каждый негодный байт UTF-8 — отдельный U+FFFD (DecodeRune), не «максимальная часть» WHATWG. */
export function goDecode(b) {
  let out = ''
  for (let i = 0; i < b.length; ) {
    const c = b[i]
    if (c < 0x80) {
      out += String.fromCharCode(c)
      i++
      continue
    }
    const n = c >= 0xc2 && c <= 0xdf ? 2 : c >= 0xe0 && c <= 0xef ? 3 : c >= 0xf0 && c <= 0xf4 ? 4 : 0
    let cp = n === 2 ? c & 0x1f : n === 3 ? c & 0x0f : c & 0x07
    let ok = n > 0 && i + n <= b.length
    for (let k = 1; ok && k < n; k++) {
      ok = (b[i + k] & 0xc0) === 0x80
      cp = (cp << 6) | (b[i + k] & 0x3f)
    }
    ok = ok && cp >= [0, 0, 0x80, 0x800, 0x10000][n] && cp <= 0x10ffff && !(cp >= 0xd800 && cp <= 0xdfff)
    out += ok ? String.fromCodePoint(cp) : '�'
    i += ok ? n : 1
  }
  return out
}

/** Нижний регистр как strings.ToLower: по знакам без контекста (unicode.ToLower), «İ» → «i», конечная сигма — «σ». */
export const goLower = (s) => [...s].map((ch) => (ch === 'İ' ? 'i' : ch.toLowerCase())).join('')

/** Опасное как путь имя (абсолютное, диск, «\», «..», управляющие знаки): архив собран не для нас — отказ целиком. */
function unsafeName(name) {
  if (name === '' || name.startsWith('/') || /[\\:]/.test(name) || /[\x00-\x1f\x7f]/.test(name)) return true
  return name.split('/').some((p) => p === '' || p === '.' || p === '..')
}

/** Обычный файл (не папка, не ссылка, не устройство) по создателю записи, как zip.File.Mode().IsRegular(). */
function isRegular(e) {
  if (e.name.endsWith('/')) return false
  switch (e.madeBy >> 8) {
    case 3: // Unix
    case 19: // macOS
      // тип в старших битах атрибутов: блочное, символьное, папка, канал, ссылка, сокет — не файл; прочие значения Go файлом считает
      return ![0o060000, 0o020000, 0o040000, 0o010000, 0o120000, 0o140000].includes((e.ext >>> 16) & 0o170000)
    case 0: // FAT
    case 11: // NTFS
    case 14: // VFAT
      return (e.ext & 0x10) === 0
    default:
      return true // иной создатель: Go режима не знает — обычный файл
  }
}

/** Проверки всех записей (pack.entries): имя, повтор, шифрование, метод, сжатие; итог — обычные файлы с годным именем. */
export function checkEntries(list, size) {
  const seen = new Set()
  const files = new Map()
  for (const e of list) {
    const name = e.name.replace(/\/$/, '')
    if (unsafeName(name)) throw new PackError('other', 'недопустимое имя в архиве набора')
    const low = goLower(name)
    if (seen.has(low)) throw fail('damaged', `имя «${clean(name, 64)}» повторяется`)
    seen.add(low)
    if (e.flags & 1) throw new PackError('other', 'зашифрованный архив не принимается')
    if (e.method !== 0 && e.method !== 8) throw fail('damaged', 'незнакомый метод сжатия')
    if (e.csize > size || e.usize > LIMITS.maxRatio * e.csize + LIMITS.slack) throw fail('damaged', 'запись сжата слишком сильно (похоже на zip-бомбу)')
    if (isRegular(e) && (validName(name) || name === 'manifest.json')) files.set(name, e)
  }
  return files
}

/** Дескриптор данных после записи (флаг 0x8): подпись необязательна, CRC в нём обязан совпасть с каталогом (readDataDescriptor). */
const descriptorOk = (dd, crc) => {
  if (dd.length < 4) return false
  const sig = dd.readUInt32LE(0) === 0x08074b50
  return dd.length >= (sig ? 16 : 12) && dd.readUInt32LE(sig ? 4 : 0) === crc
}

/** Байты записи потоком: не больше max (лишний байт — over), sha256, CRC и начало head; ошибка потока и чтения — damaged. */
export async function streamEntry(fd, e, size, max, keep) {
  const lh = readAt(fd, e.lho, 30)
  if (lh.length < 30 || lh.readUInt32LE(0) !== 0x04034b50) throw fail('damaged', `«${e.name}» не читается`)
  const start = e.lho + 30 + lh.readUInt16LE(26) + lh.readUInt16LE(28)
  const stop = Math.min(start + e.csize, size) // обрыв данных ловят размер и конец потока, как у Go
  async function* raw() {
    for (let p = start; p < stop; p += 65536) yield readAt(fd, p, Math.min(65536, stop - p))
  }
  const src = Readable.from(raw(), { objectMode: false })
  const out = e.method === 8 ? src.pipe(zlib.createInflateRaw()) : src
  if (out !== src) src.on('error', (err) => out.destroy(err)) // pipe ошибку источника не передаёт
  const h = crypto.createHash('sha256')
  const heads = []
  let n = 0
  let crc = 0
  let over = false
  let kept = 0
  try {
    for await (const chunk of out) {
      if (n + chunk.length > max) {
        over = true
        break
      }
      n += chunk.length
      h.update(chunk)
      crc = zlib.crc32(chunk, crc)
      if (kept < keep) {
        heads.push(chunk)
        kept += chunk.length
      }
    }
  } catch {
    throw fail('damaged', `«${e.name}» не читается`)
  } finally {
    out.destroy()
    src.destroy()
  }
  const crcBad = e.flags & 8 ? !descriptorOk(readAt(fd, start + e.csize, 16), e.crc) || crc !== e.crc : e.crc !== 0 && crc !== e.crc
  return { n, over, sha: h.digest('hex'), crcBad, head: Buffer.concat(heads).subarray(0, keep) }
}
