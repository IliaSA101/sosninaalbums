// Поиск по справочнику (спека 033, design §3). Чистая логика: регистр, «ё/е», окончания и английская
// раскладка не важны. Индекс строится при первом поиске и заново, когда клавиши реестра сменились (074).
import { keymap } from '../../base/keys/keymap.svelte'
import { keyRows } from './show'
import { ARTICLES, GUIDE_GROUPS, type Article } from './articles'

// Нижний регистр и «ё → е».
export const norm = (s: string): string => s.toLowerCase().replace(/ё/g, 'е')

// Плюс остаётся в слове: «ctrl+z».
const TOKEN = /[^a-zа-я0-9+]+/
export const tokens = (s: string): string[] => norm(s).split(TOKEN).filter(Boolean)

// Основа слова запроса: «виньетку» → «виньет», «имена» → «имен», «фото» → «фот».
export function stem(w: string): string {
  if (w.length >= 6) return w.slice(0, -2)
  if (w.length >= 4) return w.slice(0, -1)
  return w
}

// Клавиши ЙЦУКЕН по латинской раскладке: «j,kj;rf» набрано вместо «обложка».
const LATIN = "qwertyuiop[]asdfghjkl;'zxcvbnm,.`"
const CYR = 'йцукенгшщзхъфывапролджэячсмитьбюё'
export function fromLatin(q: string): string {
  let out = ''
  for (const ch of q.toLowerCase()) {
    const i = LATIN.indexOf(ch)
    out += i >= 0 ? CYR[i] : ch
  }
  return out
}

export interface Hit {
  a: Article
  score: number
}

export interface Result {
  hits: Hit[]
  // Запрос, который реально искали (после смены раскладки); null — искали как набрано.
  from: string | null
}

// Вес поля: название 5, синонимы 3, клавиши 2, остальной текст 1.
const W = { title: 5, words: 3, keys: 2, text: 1 } as const
interface Indexed {
  a: Article
  title: string
  fields: { w: number; t: string[] }[]
}

const build = (): Indexed[] =>
  ARTICLES.map((a) => ({
    a,
    title: norm(a.title),
    fields: [
      { w: W.title, t: tokens(a.title) },
      { w: W.words, t: tokens(a.words ?? '') },
      {
        w: W.keys,
        t: tokens(
          keyRows(a)
            .map((r) => r.show)
            .join(' '),
        ),
      },
      { w: W.text, t: tokens(keymap.text(`${a.what} ${a.where ?? ''} ${a.how ?? ''}`)) },
    ],
  }))

// Клавиши меняет пользователь: индекс привязан к виду реестра и пересобирается при его замене.
let built: { view: unknown; index: Indexed[] } | null = null
function index(): Indexed[] {
  if (!built || built.view !== keymap.view) built = { view: keymap.view, index: build() }
  return built.index
}

// Лучший вес слова среди полей статьи; 0 — слова нет.
function weight(it: Indexed, stemmed: string): number {
  let best = 0
  for (const f of it.fields) if (f.w > best && f.t.some((t) => t.startsWith(stemmed))) best = f.w
  return best
}

function run(q: string): Hit[] {
  const words = tokens(q).map(stem)
  const whole = norm(q).trim()
  const hits: Hit[] = []
  for (const it of index()) {
    let score = 0
    let ok = true
    for (const w of words) {
      const x = weight(it, w)
      if (!x) {
        ok = false
        break
      }
      score += x
    }
    if (!ok) continue
    if (it.title.startsWith(whole)) score += 3
    hits.push({ a: it.a, score })
  }
  // Порядок: вес по убыванию, при равенстве — порядок ARTICLES (сортировка устойчивая).
  return hits.sort((x, y) => y.score - x.score)
}

// Пустой запрос — все статьи в порядке списка. Ничего не нашлось и в запросе латиница — ищем русский вариант.
export function search(query: string): Result {
  if (tokens(query).length === 0) return { hits: ARTICLES.map((a) => ({ a, score: 0 })), from: null }
  const hits = run(query)
  if (hits.length > 0 || !/[a-z;',.[\]`]/i.test(query)) return { hits, from: null }
  const alt = fromLatin(query)
  const again = alt === query ? [] : run(alt)
  return again.length > 0 ? { hits: again, from: alt.trim() } : { hits, from: null }
}

export interface Section {
  id: string
  title: string
  items: Article[]
}

// Список без запроса: смысловые разделы в порядке GUIDE_GROUPS, пустые не показываем.
export function grouped(articles: readonly Article[] = ARTICLES): Section[] {
  const out: Section[] = []
  for (const g of GUIDE_GROUPS) {
    const items = articles.filter((a) => a.group === g.id)
    if (items.length) out.push({ id: g.id, title: g.title, items })
  }
  return out
}
