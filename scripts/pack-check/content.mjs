// Содержимое предмета без декодирования: картинка по заголовку PNG/JPEG, шаблон и макет — по полям v и steps.
// Полный разбор (зажим, узоры, ссылки на библиотеку, окно рамочки) делает программа при установке.
import { LIMITS, ext } from './base.mjs'
import { Num, asInt, decodeObject, parseJson } from './manifest.mjs'

const PNG_SIG = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])

/** Вид и размеры растра по заголовку: PNG или JPEG; иное и усечённое — исключение. */
export function imageInfo(d) {
  if (d.length >= 8 && d.subarray(0, 8).equals(PNG_SIG)) {
    if (d.length < 24 || d.toString('latin1', 12, 16) !== 'IHDR') throw new Error('нет PNG-заголовка')
    return { type: 'png', w: d.readUInt32BE(16), h: d.readUInt32BE(20) }
  }
  if (d.length < 4 || d[0] !== 0xff || d[1] !== 0xd8) throw new Error('не PNG и не JPEG')
  let p = 2
  while (p + 4 <= d.length) {
    if (d[p] !== 0xff) throw new Error('JPEG: битая разметка')
    const m = d[p + 1]
    if (m === 0xff) {
      p++ // заполнитель
      continue
    }
    if (m === 0x01 || (m >= 0xd0 && m <= 0xd8)) {
      p += 2 // маркеры без длины
      continue
    }
    if (m === 0xd9 || m === 0xda) break
    const len = d.readUInt16BE(p + 2)
    if (len < 2) throw new Error('JPEG: битая длина сегмента')
    if (m >= 0xc0 && m <= 0xcf && m !== 0xc4 && m !== 0xc8 && m !== 0xcc) {
      if (len < 8 || p + 9 > d.length) throw new Error('JPEG: усечённый заголовок')
      return { type: 'jpeg', w: d.readUInt16BE(p + 7), h: d.readUInt16BE(p + 5) }
    }
    p += 2 + len
  }
  throw new Error('JPEG: нет размера в заголовке')
}

const asSteps = (v) => {
  if (!Array.isArray(v)) throw new TypeError('не массив')
  return v
}

/** JSON-объект предмета и его версия v (поле Go без учёта регистра); не объект или v не целое — null. */
function jsonHead(head) {
  try {
    const obj = parseJson(head.toString('utf8'))
    if (obj === null || typeof obj !== 'object' || Array.isArray(obj) || obj instanceof Num) return null
    return { obj, v: decodeObject(obj, { v: asInt }).v ?? 0 }
  } catch {
    return null
  }
}

/** Причина пропуска предмета по содержимому (как decor.stageImage, stageTpl, stageLook); "" — годен. */
export function contentSkip(it, head) {
  if (it.kind === 'print') return 'типографию ставит только набор из каталога библиотеки — из файла она не принимается'
  if (it.kind === 'spread') {
    const h = jsonHead(head)
    if (h && h.v > LIMITS.tplV) return 'шаблон из новой версии программы'
    return h && h.v === LIMITS.tplV ? '' : 'шаблон разворота не читается'
  }
  if (it.kind === 'album') {
    const h = jsonHead(head)
    if (!h) return 'макет альбома не читается'
    if (h.v > LIMITS.lookV) return 'макет из новой версии программы'
    let steps
    try {
      steps = decodeObject(h.obj, { steps: asSteps }).steps
    } catch {
      return 'макет альбома не читается'
    }
    if (steps && steps.length > LIMITS.maxLookSteps) return `в макете больше ${LIMITS.maxLookSteps} разворотов`
    return h.v === LIMITS.lookV && steps && steps.length >= 1 ? '' : 'макет альбома не читается'
  }
  let info
  try {
    info = imageInfo(head)
  } catch (e) {
    return `картинка не читается: ${e.message}`
  }
  const e = ext(it.file)
  if (info.type === 'png' ? e !== '.png' : e !== '.jpg' && e !== '.jpeg') return 'содержимое не совпадает с расширением файла'
  if (info.w < 1 || info.h < 1) return 'картинка нулевого размера'
  if (it.kind === 'bg' && info.w * info.h > LIMITS.bgMaxPixels) return `фон больше ${LIMITS.bgMaxPixels / 1e6} Мп`
  if (it.kind === 'sticker' && Math.max(info.w, info.h) > LIMITS.maxSide) return `стикер больше ${LIMITS.maxSide} px по стороне`
  if (it.kind === 'frame' && Math.max(info.w, info.h) > LIMITS.maxSide) return `рамочка больше ${LIMITS.maxSide} px по стороне`
  return ''
}
