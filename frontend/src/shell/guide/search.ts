// Поиск по справочнику (спека 033, design §3). Чистая логика: регистр, «ё/е», окончания и английская
// раскладка не важны. Индекс строится при первом поиске и заново, когда клавиши реестра сменились (074).
import { keymap } from '../../base/keys/keymap.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { fromLatin, norm } from '../../base/words/kbdlayout'
import { articleText } from '../guidetext/articletext'
import { keyRows, menus } from './show'
import { ARTICLES, placesOf, type Article } from '../guidetext/articles'
import { ACTS } from '../../editor/photomenu/photoacts'
import type { MenuItem } from '../menu/menu'
import { sectionsOf } from './guidelist'
import { orderRank } from '../guidetext/order'

// Нижний регистр, «ё → е» и раскладка — общие с поиском фото (base/words/kbdlayout.ts).
export { fromLatin, norm }

// Плюс остаётся в слове: «ctrl+z».
const TOKEN = /[^a-zа-я0-9+]+/
export const tokens = (s: string): string[] => norm(s).split(TOKEN).filter(Boolean)

// Служебные слова запроса («как сделать виньетку») не ищем, если после них что-то осталось (0.29-136-5).
// «Найти» и «свои» сюда не входят: это слова названий.
const STOP = new Set(
  'как где что чтобы сделать можно мне я в во на по с со и или для из к у это ли не еще а же бы хочу нужно надо есть'.split(' '),
)

// Слово с цифрой или «+» («ctrl+z», «f1», «p3») ищем целиком и точно: основа тут ломает клавишу.
export const isExact = (w: string): boolean => /[0-9+]/.test(w)

// Основа слова запроса: «виньетку» → «винье», «имена» → «име», «белое/белые» → «бел». Без словаря:
// чем длиннее слово, тем больше окончание отбрасываем.
export function stem(w: string): string {
  if (isExact(w)) return w
  if (w.length >= 7) return w.slice(0, Math.max(5, w.length - 3))
  if (w.length >= 5) return w.slice(0, Math.max(3, w.length - 2))
  if (w.length === 4) return w.slice(0, 3)
  return w
}

// Запрос длиннее этого режется: поиск не должен зависеть от длины вставленного текста.
export const MAX_QUERY = 200

export interface Hit {
  a: Article
  score: number
}

export interface Result {
  hits: Hit[]
  // Запрос, который реально искали (после смены раскладки); null — искали как набрано.
  from: string | null
  // Точных совпадений нет: показаны статьи, где нашлась часть слов (над списком — «Точных совпадений нет — похожие»).
  loose: boolean
}

// Вес поля: название 5, синонимы 3, клавиши 2, остальной текст 1.
const W = { title: 5, words: 3, keys: 2, sub: 2, text: 1 } as const
interface Indexed {
  a: Article
  title: string
  fields: { w: number; t: string[] }[]
}

// Подписи подменю всех уровней у верхних пунктов статьи (a.menu и spot menu:*) и подписи её действий фото (acts):
// «Выровнять ▸ По левому краю», «Одинаковый размер» (design §3, п. 8).
const subLabels = (items: MenuItem[]): string[] => items.flatMap((i) => (i.sub ? [...i.sub.map((x) => x.label), ...subLabels(i.sub)] : []))
function menuWords(a: Article): string {
  const ids = new Set(a.menu ?? [])
  for (const p of placesOf(a)) if (p.spot.startsWith('menu:')) ids.add(p.spot.slice(5))
  const tops = menus().flatMap((m) => m.items.filter((i) => ids.has(i.id)))
  const labels = [...subLabels(tops), ...(a.acts ?? []).map((id) => ACTS[id].label)]
  return labels.join(' ')
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
      { w: W.sub, t: tokens(menuWords(a)) },
      { w: W.text, t: tokens(articleText(`${a.what} ${a.where ?? ''} ${a.how ?? ''}`)) },
    ],
  }))

// Клавиши и уровень меняет пользователь (куски {can:…} зависят от набора функций): индекс привязан к виду реестра
// и к набору функций и пересобирается при замене любого.
let built: { view: unknown; enabled: unknown; index: Indexed[] } | null = null
function index(): Indexed[] {
  if (!built || built.view !== keymap.view || built.enabled !== prefs.enabled)
    built = { view: keymap.view, enabled: prefs.enabled, index: build() }
  return built.index
}

// Лучший вес слова среди полей статьи; 0 — слова нет. Основа — по началу токена, «точное» слово — только целиком.
function weight(it: Indexed, w: Stemmed): number {
  let best = 0
  for (const f of it.fields) {
    if (f.w <= best) continue
    if (f.t.some((t) => (w.exact ? t === w.s : t.startsWith(w.s)))) best = f.w
  }
  return best
}

interface Stemmed {
  s: string
  exact: boolean
}
interface Scored extends Hit {
  // Сколько слов запроса нашлось в статье.
  found: number
}

function run(q: string): { strict: Hit[]; loose: Hit[] } {
  let words = tokens(q)
  const kept = words.filter((w) => !STOP.has(w))
  if (kept.length > 0) words = kept
  const st: Stemmed[] = words.map((w) => ({ s: stem(w), exact: isExact(w) }))
  const whole = norm(q).trim()
  const all: Scored[] = index().map((it) => {
    let score = 0
    let found = 0
    for (const w of st) {
      const x = weight(it, w)
      if (x) {
        found++
        score += x
      }
    }
    if (found > 0 && it.title.startsWith(whole)) score += 3
    return { a: it.a, score, found }
  })
  // Порядок: вес по убыванию, при равенстве — как в окне (GUIDE_ORDER: раздел, затем место в разделе).
  const byRank = (x: Hit, y: Hit): number => orderRank(x.a.id) - orderRank(y.a.id)
  const strict = all.filter((h) => h.found === st.length).sort((x, y) => y.score - x.score || byRank(x, y))
  // Запасной проход: больше совпавших слов — выше, потом вес, потом порядок окна.
  const loose = all.filter((h) => h.found > 0).sort((x, y) => y.found - x.found || y.score - x.score || byRank(x, y))
  return { strict, loose }
}

const plain = (hits: Hit[]): Hit[] => hits.map((h) => ({ a: h.a, score: h.score }))

// Пустой запрос — все статьи в порядке списка. Сначала все слова; нет — русский вариант латиницы; нет —
// «похожие» по части слов (loose). Запрос не превращается в RegExp: только startsWith и ===.
export function search(raw: string): Result {
  const query = raw.slice(0, MAX_QUERY)
  if (tokens(query).length === 0) return { hits: ARTICLES.map((a) => ({ a, score: 0 })), from: null, loose: false }
  const first = run(query)
  if (first.strict.length > 0) return { hits: plain(first.strict), from: null, loose: false }
  if (/[a-z;',.[\]`]/i.test(query)) {
    const alt = fromLatin(query)
    if (alt !== query) {
      const again = run(alt)
      if (again.strict.length > 0) return { hits: plain(again.strict), from: alt.trim(), loose: false }
    }
  }
  if (first.loose.length > 0) return { hits: plain(first.loose), from: null, loose: true }
  return { hits: [], from: null, loose: false }
}

export interface Section {
  id: string
  title: string
  items: Article[]
}

// Список без запроса: смысловые разделы в порядке GUIDE_GROUPS, статьи — по GUIDE_ORDER, пустые не показываем.
export const grouped = (articles: readonly Article[] = ARTICLES): Section[] => sectionsOf(articles)
