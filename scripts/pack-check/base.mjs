// Общее для pack-check: пределы набора, отказы с кодом, чистка текста. Часть pack-check.mjs (скачивают вместе с папкой).

/**
 * Пределы набора и архива. Копия констант Go (pack/manifest.go, eocd.go, archive.go; decor packMax*; model TplV, LookV,
 * MaxLookSteps): сверка с файлом testdata/pack-manifest-cases.json (блок limits) — в TestLimits (Go) и в pack-check.test.mjs.
 */
export const LIMITS = Object.freeze({
  format: 1,
  maxZip: 300 << 20,
  maxEntries: 2000,
  maxManifest: 256 << 10,
  maxItems: 1000,
  maxTotal: 1 << 30,
  kind: Object.freeze({ bg: 60 << 20, sticker: 30 << 20, spread: 256 << 10, frame: 30 << 20, print: 64 << 10, album: 512 << 10 }),
  maxDir: 1 << 20, // каталог zip (eocd.go)
  tailLen: 65 * 1024,
  maxRatio: 200, // сжатие сильнее — zip-бомба (archive.go)
  slack: 1 << 20,
  bgMaxPixels: 100_000_000, // packMaxBgPixels в decor
  maxSide: 4096, // packMaxSide
  tplV: 1,
  lookV: 1,
  maxLookSteps: 24,
})

export const E = {
  newer: 'набор из новой версии программы — нужна новая версия Sosnina Albums',
  manifest: 'манифест набора не читается',
  damaged: 'архив набора повреждён',
  empty: 'в наборе нечего ставить',
  appOld: 'нужна версия Sosnina Albums',
}

/** Отказ с кодом (newer, manifest, damaged, empty, appOld, other): code для сверки с Go, message — для человека. */
export class PackError extends Error {
  constructor(code, message) {
    super(message)
    this.code = code
  }
}
export const fail = (code, detail) => new PackError(code, detail ? `${E[code]}: ${detail}` : E[code])

/** Текст для показа: без управляющих и невидимых знаков, пробелы схлопнуты, не длиннее max знаков (pack.Clean). */
export function clean(s, max) {
  let out = ''
  let n = 0
  let space = false
  for (const r of s.toWellFormed()) {
    if (/\p{White_Space}/u.test(r)) {
      space = out.length > 0
      continue
    }
    if (r === '�' || /[\p{Cc}\p{Cf}]/u.test(r)) continue
    if (space) {
      if (n + 1 >= max) break
      out += ' '
      n++
      space = false
    }
    if (n === max) break
    out += r
    n++
  }
  return out
}

/** Расширение последней части имени (path.Ext): с точкой или "". */
export const ext = (file) => {
  const base = file.slice(file.lastIndexOf('/') + 1)
  const i = base.lastIndexOf('.')
  return i < 0 ? '' : base.slice(i)
}
