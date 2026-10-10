// Диапазон экспорта (спека 009, design §6.1.1): «3-5, 8, 7п». Паритет с Go parseRange —
// общие примеры testdata/parity/ranges.json. layouts/ranges.ts (копирование раскладки) — отдельно: страниц там нет.
export type ExportUnit = [spread: number, page: 0 | 1 | 2] // page: 0 — разворот целиком, 1 — левая, 2 — правая

export interface ExportRange {
  units: ExportUnit[] // по возрастанию, без повторов; пусто при ошибке
  error: string | null // текст под полем; null — ввод верный
}

export const MAX_RANGE_LEN = 1000

const fail = (error: string): ExportRange => ({ units: [], error })

// Один разобранный элемент («3-5», «8», «7п») в набор seen; ошибка — текстом, успех — null.
function addToken(seen: Set<number>, m: RegExpExecArray, raw: string, n: number): string | null {
  const a = +m[1]
  const b = m[2] === undefined ? a : +m[2]
  if (a < 1 || b < 1) return 'Номера начинаются с 1'
  if (a > b) return `Диапазон «${raw}» задан наоборот`
  if (b > n) return `Разворота ${b} нет (всего ${n})`
  if (m[3] !== undefined) {
    seen.add(a * 4 + (m[3] === 'л' || m[3] === 'l' ? 1 : 2))
  } else {
    for (let k = a; k <= b; k++) seen.add(k * 4)
  }
  return null
}

export function parseExportRange(text: string, n: number): ExportRange {
  if (!(n >= 1)) return fail('Нет разворотов')
  if (text.length > MAX_RANGE_LEN) return fail('Слишком длинная строка')
  const src = text.replace(/\s*[-–—]\s*/g, '-').trim()
  if (src === '') return fail('Укажите разворот или страницу')
  const seen = new Set<number>() // spread*4 + page
  for (const raw of src.split(/[,;\s]+/)) {
    if (raw === '') continue // хвостовая запятая
    const m = /^(\d{1,6})(?:-(\d{1,6})|([лпlr]))?$/.exec(raw.toLowerCase())
    if (!m) return fail(badToken(raw, src))
    const err = addToken(seen, m, raw, n)
    if (err) return fail(err)
  }
  if (seen.size === 0) return fail('Укажите разворот или страницу') // одни запятые
  const units = [...seen].sort((x, y) => x - y).map((v): ExportUnit => [v >> 2, (v & 3) as 0 | 1 | 2])
  return { units, error: null }
}

// Понятное объяснение типичных промахов со страницами; остальное — «Не понимаю».
function badToken(tok: string, src: string): string {
  const t = tok.toLowerCase()
  // Пример — из введённого: «3-5п» → «5п», «7п-9» → «7п», «6 п» → «6п».
  const m = /^\d+-(\d+)([лпlr])$/.exec(t) ?? /^(\d+)([лпlr])-\d+$/.exec(t)
  if (m) return `Страницу можно указать только у одного разворота: ${m[1]}${m[2]}`
  if (/^[лпlr]$/.test(t)) {
    const g = /(\d+)\s+([лпlr])(?![\p{L}\d])/iu.exec(src)
    if (g) return `Номер страницы пишется слитно: ${g[1]}${g[2].toLowerCase()}`
  }
  return `Не понимаю «${tok}». Пример: 3-5, 8, 7п`
}
