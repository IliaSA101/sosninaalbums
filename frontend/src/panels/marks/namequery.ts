// Поиск фото по имени файла в ленте (0.26-124, design §8.2–8.3). Чистая логика без Svelte: разбор запроса,
// индекс имён и совпадение. Регулярок из ввода нет (только split/includes) — запрос пользователя не ломает поиск.
// Правило номера кадра — D с правками 0.26-124-21: последнее число из 3+ цифр, «IMG 0798» и «#798» ищутся по номеру.
import { norm, toLatin } from '../../base/words/kbdlayout'

export interface Tok {
  raw: string // в нижнем регистре, без расширения и «ё»
  lat: string // для кириллицы — то же в латинской раскладке («шьп» → «img»), иначе пусто
  num: boolean // из одних цифр: номер кадра
}

const DIGIT = /\d/
const ONLY_DIGITS = /^\d+$/
const CYRILLIC = /[а-я]/
const LETTER = /\p{L}/gu
// Расширение из 2–4 знаков с хотя бы одной буквой («.cr2», «.jpg»); «.2024» расширением не считается.
const EXT = /\.([a-z0-9]{2,4})$/
// Второй проход по числу (0.26-124-21 а): в токене не больше этого числа букв («img», «фото»), иначе это фраза.
const MAX_WORD_LETTERS = 4
// Окончание номера ищется с этой длины токена (0.26-124-8).
const TAIL = 4

const stripExt = (t: string): string => {
  const m = EXT.exec(t)
  return m && /[a-z]/.test(m[1]) ? t.slice(0, -m[0].length) : t
}

// Без ведущих нулей (последний ноль остаётся: «000» → «0»).
const strip0 = (s: string): string => {
  let i = 0
  while (i < s.length - 1 && s.charCodeAt(i) === 48) i++
  return i === 0 ? s : s.slice(i)
}

function toTok(raw: string): Tok {
  const lat = CYRILLIC.test(raw) ? toLatin(raw) : ''
  return { raw, lat, num: ONLY_DIGITS.test(raw) }
}

// Запрос → токены; пусто → []. Список: «,» «;» перевод строки, а части через пробел — если в каждой есть цифра
// («798 801 IMG_0805»); иначе часть — фраза целиком («Свадьба Ивановых», «Свадьба 2024»).
export function parseQuery(q: string): Tok[] {
  const text = norm(q.trim())
  if (!text) return []
  const out: Tok[] = []
  for (const part of text.split(/[,;\n]+/)) {
    const p = part.trim()
    if (!p) continue
    const words = p.split(/\s+/)
    for (const w of words.length > 1 && words.every((x) => DIGIT.test(x)) ? words : [p]) {
      const t = stripExt(w)
      if (t) out.push(toTok(t))
    }
  }
  return out
}

// Счётчик кадра в имени: последнее число из 3+ цифр, иначе последнее число. Если последнее число — 1–2 цифры,
// а раньше есть число из 3+ («IMG_1234-2», «Ivanovy2024-12»), счётчиков два (0.26-124-21 в).
export function counters(base: string): string[] {
  // Проход по символам: имён десятки тысяч, регулярка с массивом на каждое заметно медленнее (замер 20 000 имён).
  let last = ''
  let main = ''
  let from = -1
  for (let i = 0; i <= base.length; i++) {
    const c = i < base.length ? base.charCodeAt(i) : 0
    if (c >= 48 && c <= 57) {
      if (from < 0) from = i
    } else if (from >= 0) {
      last = base.slice(from, i)
      if (last.length >= 3) main = last
      from = -1
    }
  }
  if (!main) return last ? [last] : []
  return last.length <= 2 ? [main, last] : [main]
}

interface Parsed {
  base: string
  cnt: string[]
}

// Разбор имени кэшируется по строке: при пересборке индекса (меняется библиотека) имена в основном те же.
const parsed = new Map<string, Parsed>()
const PARSED_MAX = 200_000

function parseName(name: string): Parsed {
  let p = parsed.get(name)
  if (!p) {
    const file = name.slice(Math.max(name.lastIndexOf('/'), name.lastIndexOf('\\')) + 1)
    const dot = file.lastIndexOf('.')
    const base = norm(dot > 0 ? file.slice(0, dot) : file)
    p = { base, cnt: counters(base) }
    if (parsed.size >= PARSED_MAX) parsed.clear()
    parsed.set(name, p)
  }
  return p
}

export interface NameIndex {
  ids: string[]
  base: string[] // имя без пути и расширения, нижний регистр
  cnt: string[][] // счётчики кадра
  byVal: Map<string, number[]> // значение без ведущих нулей → номера фото
  byTail: Map<string, number[]> // последние 4 цифры счётчика из 4+ цифр → номера фото
}

export function buildIndex(photos: readonly { id: string; name: string }[]): NameIndex {
  const idx: NameIndex = { ids: [], base: [], cnt: [], byVal: new Map(), byTail: new Map() }
  const push = (m: Map<string, number[]>, k: string, i: number) => {
    const l = m.get(k)
    if (!l) m.set(k, [i])
    else if (l[l.length - 1] !== i) l.push(i)
  }
  photos.forEach((ph, i) => {
    const p = parseName(ph.name)
    idx.ids.push(ph.id)
    idx.base.push(p.base)
    idx.cnt.push(p.cnt)
    for (const c of p.cnt) {
      push(idx.byVal, strip0(c), i)
      if (c.length >= TAIL) push(idx.byTail, c.slice(-TAIL), i)
    }
  })
  return idx
}

// Фото, чей счётчик равен числу (без ведущих нулей) или, при 4+ цифрах в запросе, оканчивается на него.
function byNumber(idx: NameIndex, q: string, hit: (i: number) => void) {
  for (const i of idx.byVal.get(strip0(q)) ?? []) hit(i)
  if (q.length >= TAIL) for (const i of idx.byTail.get(q.slice(-TAIL)) ?? []) if (idx.cnt[i].some((c) => c.endsWith(q))) hit(i)
}

// Токен и фото, которые он нашёл: по ним ищется «найдено, но скрыто фильтром» (запись 0.26-124-35).
export interface TokHits {
  raw: string
  ids: string[]
}

export function matchIds(idx: NameIndex, toks: readonly Tok[]): { ids: Set<string>; miss: string[]; hits: TokHits[] } {
  const found = new Set<number>()
  const miss: string[] = []
  const hits: TokHits[] = []
  for (const t of toks) {
    let any = false
    const own: number[] = []
    const hit = (i: number) => {
      found.add(i)
      own.push(i)
      any = true
    }
    if (t.num) byNumber(idx, t.raw, hit)
    else {
      for (let i = 0; i < idx.base.length; i++) if (idx.base[i].includes(t.raw) || (t.lat && idx.base[i].includes(t.lat))) hit(i)
      // Подстроки нет: «img 0798», «img0798», «#798», «фото 798» — по последнему числу токена как номеру.
      const nums = any ? null : t.raw.match(/\d+/g)
      if (nums && (t.raw.match(LETTER)?.length ?? 0) <= MAX_WORD_LETTERS) byNumber(idx, nums[nums.length - 1], hit)
    }
    if (any) hits.push({ raw: t.raw, ids: own.map((i) => idx.ids[i]) })
    else miss.push(t.raw)
  }
  return { ids: new Set([...found].map((i) => idx.ids[i])), miss, hits }
}

// Токены, найденные в библиотеке, но ни одного кадра нет в ленте (скрыты папкой, подборкой, фильтром отметок).
export function hiddenToks(hits: readonly TokHits[], inFeed: ReadonlySet<string>): string[] {
  return hits.filter((h) => !h.ids.some((id) => inFeed.has(id))).map((h) => h.raw)
}

// Не найденные токены: до трёх — списком «· нет: 7811, 7815», больше — «· нет 5», список — в подсказке.
const MISS_LIST = 3
function listLabel(items: readonly string[], word: string, title: string): { text: string; title: string } {
  if (items.length === 0) return { text: '', title: '' }
  const list = items.join(', ')
  return { text: items.length <= MISS_LIST ? `· ${word}: ${list}` : `· ${word} ${items.length}`, title: `${title}: ${list}` }
}
export const missLabel = (miss: readonly string[]) => listLabel(miss, 'нет', 'Не найдено')
// Найдены в библиотеке, но скрыты фильтрами ленты (0.26-124-35): «· скрыто: 805», больше трёх — «· скрыто 4».
export const hiddenLabel = (hidden: readonly string[]) => listLabel(hidden, 'скрыто', 'Есть в библиотеке, но скрыто фильтрами ленты')

// Запрос в тексте пустой ленты: длинный не раздувает строку.
const SHOWN_MAX = 60
export const shortQuery = (q: string): string => {
  const t = q.trim()
  return t.length > SHOWN_MAX ? t.slice(0, SHOWN_MAX) + '…' : t
}
