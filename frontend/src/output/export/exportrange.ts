import { plural } from '../../base/words/plural'

// Диапазон экспорта (спека 009, design §6.1.1): «3-5, 8, 7п». Паритет с Go parseRange —
// общие примеры testdata/parity/ranges.json. layouts/ranges.ts (копирование раскладки) — отдельно: страниц там нет.
export type ExportUnit = [spread: number, page: 0 | 1 | 2] // page: 0 — разворот целиком, 1 — левая, 2 — правая

export interface ExportRange {
  units: ExportUnit[] // по возрастанию, без повторов; пусто при ошибке
  error: string | null // текст под полем; null — ввод верный
}

export const MAX_RANGE_LEN = 1000

// Как Go export.ErrSheetPages (без заглавной: errText тут не нужен).
const SHEET_PAGES = 'В листе нет страниц: укажите номера листов без «л» и «п»'

const fail = (error: string): ExportRange => ({ units: [], error })

// Один разобранный элемент («3-5», «8», «7п») в набор seen; ошибка — текстом, успех — null.
function addToken(seen: Set<number>, m: RegExpExecArray, raw: string, n: number, sheet: boolean): string | null {
  const a = +m[1]
  const b = m[2] === undefined ? a : +m[2]
  if (a < 1 || b < 1) return 'Номера начинаются с 1'
  if (a > b) return `Диапазон «${raw}» задан наоборот`
  if (b > n) return `${sheet ? 'Листа' : 'Разворота'} ${b} нет (всего ${n})`
  if (sheet && m[3] !== undefined) return SHEET_PAGES
  if (m[3] !== undefined) {
    seen.add(a * 4 + (m[3] === 'л' || m[3] === 'l' ? 1 : 2))
  } else {
    for (let k = a; k <= b; k++) seen.add(k * 4)
  }
  return null
}

// sheet — проект-лист (031): слова «лист», страниц нет. Альбом (по умолчанию) говорит «разворот».
export function parseExportRange(text: string, n: number, sheet = false): ExportRange {
  const empty = sheet ? 'Укажите лист' : 'Укажите разворот или страницу'
  if (!(n >= 1)) return fail(sheet ? 'Нет листов' : 'Нет разворотов')
  if (text.length > MAX_RANGE_LEN) return fail('Слишком длинная строка')
  const src = text.replace(/\s*[-–—]\s*/g, '-').trim()
  if (src === '') return fail(empty)
  const seen = new Set<number>() // spread*4 + page
  for (const raw of src.split(/[,;\s]+/)) {
    if (raw === '') continue // хвостовая запятая
    const m = /^(\d{1,6})(?:-(\d{1,6})|([лпlr]))?$/.exec(raw.toLowerCase())
    if (!m) return fail(sheet && /[лпlr]/i.test(raw) ? SHEET_PAGES : badToken(raw, src))
    const err = addToken(seen, m, raw, n, sheet)
    if (err) return fail(err)
  }
  if (seen.size === 0) return fail(empty) // одни запятые
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

// Сводка под полем диапазона (073): «Выгрузится 3 разворота и 1 страница», у листа 031 — «4 листа»; с обложкой — «… и обложка».
export function rangeSummary(units: readonly ExportUnit[], sheet: boolean, cover: boolean): string {
  const whole = units.filter(([, p]) => p === 0).length
  const pages = units.length - whole
  const parts: string[] = []
  if (whole > 0) {
    const forms: [string, string, string] = sheet ? ['лист', 'листа', 'листов'] : ['разворот', 'разворота', 'разворотов']
    parts.push(`${whole} ${plural(whole, forms)}`)
  }
  if (pages > 0) parts.push(`${pages} ${plural(pages, ['страница', 'страницы', 'страниц'])}`)
  if (cover) parts.push('обложка')
  if (parts.length < 2) return `Выгрузится ${parts[0] ?? '0 разворотов'}`
  return `Выгрузится ${parts.slice(0, -1).join(', ')} и ${parts[parts.length - 1]}`
}
