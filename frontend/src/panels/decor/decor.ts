// Окно «Фоны» (048): чистые решения — темы, плитки, «Куда», итог загрузки, тексты. Каталог и правки — у Go;
// здесь ничего не считается, кроме отбора плиток по теме и шага размера картинки.
import type { app } from '../../../wailsjs/go/models'
import type { BackdropScope, CoverView, PatternSpec, PatternView, SpreadView, View } from '../../base/core/types'
import { hasSeam, isCover } from '../../base/geom/sheet'
import { backdropOf } from '../../editor/backdrop/backdrop'
import { patternOf } from '../../editor/pattern/pattern'
import { stickerById } from '../../editor/sticker/sticker'
import { coverLabel, coverScopes } from '../../editor/cover/cover'
import type { SegItem } from '../../base/tpl/Seg.svelte'
import type { SideItem } from '../../base/tpl/SideList.svelte'

export type DecorKind = 'bg' | 'sticker' | 'spread' | 'frame'

// Типы ответа Go (app.DecorCatalog и др.): интерфейс читает их как есть.
export interface DecorItem {
  id: string // sa:<пресет> | my:<16 hex>
  theme: string
  name: string
  url: string // без px
  own: boolean
  aspect: number // ширина/высота своей картинки; узор — 0
  w: number
  h: number
  color?: string // цвет страниц пресета: подложка плитки
  dark?: boolean // светлый стикер (116): плитке — тёмная шашечка
  pages?: number // 067, только вид spread: 1 — страница, 2 — весь разворот
  places?: number // 067: сколько мест для фото в шаблоне
  pack?: string // 067: название набора, из которого плитка
  packId?: string // 133: 'sa:builtin' | id набора; нет — своё
  peek?: string // 133: крупная картинка без px (/media/lib/p/…) у своих и наборов; нет — url + px
  form?: string // 133, только стикеры: 'corner' | 'divider'; нет — обычный стикер (в биндинге просто string)
  style?: string // 133, только стикер без form: стиль из имени «Значки: Колокольчик» → «Значки»
}
export interface DecorTheme {
  id: string
  name: string
  count: number
}
export interface DecorCatalog {
  kind: DecorKind
  themes: DecorTheme[]
  items: DecorItem[]
  recent: string[]
  new: string[] // 133: ID плиток строки «Новые» в порядке каталога; пусто — []
  readOnly: boolean
}
export interface DecorSkip {
  name: string
  why: string
}
export interface DecorImport {
  added: DecorItem[]
  skipped: DecorSkip[]
  // Задания app.DecorSvg (SVG прошёл проверку Go, окно рисует его в PNG — svgraster); только у стикеров, нет или пусто — рисовать нечего.
  svg?: app.DecorSvg[]
}

// Итог ApplySpreadTpl (app.TplResult): вид, сколько фото ушло в ленту, сколько слоёв библиотеки не нашлось на этом компьютере.
export interface TplResult {
  view: View
  left: number
  missing: number
}

export const DECOR_TITLE: Record<DecorKind, string> = { bg: 'Фоны', sticker: 'Стикеры', spread: 'Развороты', frame: 'Рамочки' }
// Что лежит на плитке — для подписей ✎ и ×.
export const NOUN: Record<DecorKind, string> = { bg: 'фон', sticker: 'стикер', spread: 'разворот', frame: 'рамочку' }
export const NAME_MAX = 60
export const RECENT = 'recent'
export const NEW = 'new' // 133: «Новые» — выборка, как «Недавние»
export const ALL = 'all'
export const MINE = 'mine'

// Выборка — не тема предметов, а отбор: «Все», «Недавние», «Новые» (themeItems ведёт их особо).
export const isSelection = (id: string): boolean => id === ALL || id === RECENT || id === NEW
// Куда своё не кладётся: выборки и «Фирменные» (зеркало Go isTheme); импорт и сохранение уходят в «Мои».
export const noOwn = (id: string): boolean => isSelection(id) || id === 'brand'

type SourceCat = Pick<DecorCatalog, 'themes' | 'items'> & { new?: string[] }

// Куда прыгнуть из-за свежего набора (fresh — названия наборов сеанса): «Новые», если там плитка такого набора (133),
// иначе тема первой его плитки (073-1); '' — свежего в этом каталоге нет. Так обновление набора при чужих свежих
// не уводит на «Новые» с чужим содержимым.
export function freshSource(cat: SourceCat | null, fresh?: ReadonlySet<string>): string {
  if (!cat || !fresh?.size) return ''
  const has = (id: string) => cat.themes.some((t) => t.id === id)
  const items = cat.items ?? []
  if (has(NEW)) {
    const news = new Set(cat.new ?? [])
    if (items.some((i) => news.has(i.id) && i.pack && fresh.has(i.pack))) return NEW
  }
  return items.find((i) => i.pack && fresh.has(i.pack) && has(i.theme))?.theme ?? ''
}

// Источник при открытии: свежий набор (freshSource), иначе прежний выбор, если он ещё есть; иначе «Недавние» (если не пусты), иначе «Все».
export function firstSource(cat: SourceCat | null, prev = '', fresh?: ReadonlySet<string>): string {
  if (!cat) return ALL
  const has = (id: string) => cat.themes.some((t) => t.id === id)
  const hot = freshSource(cat, fresh)
  if (hot) return hot
  if (prev && has(prev)) return prev
  return cat.themes.find((t) => t.id === RECENT && t.count > 0) ? RECENT : ALL
}

// Прежнее имя: окно макетов (shell/looks, 131) зовёт его, «Новых» у макетов нет — Go их не отдаёт.
export const firstTheme = firstSource

// Подсказки строк-выборок слева (Т5)
export const SIDE_TIPS: Readonly<Record<string, string>> = {
  [NEW]: 'Из наборов, поставленных за последние 30 дней (кроме первой установки библиотеки)',
  [RECENT]: 'Что вы недавно ставили',
}

export const sideItems = (cat: Pick<DecorCatalog, 'themes'> | null): SideItem<string>[] =>
  (cat?.themes ?? []).map((t) => ({ id: t.id, label: t.name, count: t.count, title: SIDE_TIPS[t.id] }))

// Плитки темы. «Недавние» — в порядке поставленных (новые первыми), «Новые» — в порядке каталога (новый набор выше),
// «Все» — каталог целиком.
export function themeItems<T extends DecorItem>(cat: { items: T[]; recent: string[]; new?: string[] } | null, theme: string): T[] {
  if (!cat) return []
  if (theme === ALL) return cat.items
  if (theme === RECENT || theme === NEW) {
    const by = new Map(cat.items.map((it) => [it.id, it]))
    return (theme === RECENT ? cat.recent : (cat.new ?? [])).flatMap((id) => by.get(id) ?? [])
  }
  return cat.items.filter((it) => it.theme === theme)
}

// Своё из выборок и «Фирменных» кладётся в «Мои» (в выборках лежит не тема, а отбор; Go кладёт туда же).
export const importTheme = (theme: string): string => (noOwn(theme) ? MINE : theme)

// Кнопка своего в окнах плиток (100-1): одна на три окна, крайняя справа, вид primary.
export const ADD_OWN: Record<'bg' | 'sticker' | 'frame', string> = {
  bg: 'Добавить свой фон…',
  sticker: 'Добавить свой стикер…',
  frame: 'Добавить свою рамочку…',
}
// Логотип и надписи: что принимает «Добавить свой стикер…» (подсказка кнопки и строка над плитками «Мои»).
export const OWN_TIP_STICKER = 'Логотип и надписи — PNG или SVG с прозрачным фоном; JPEG ляжет с белым прямоугольником'
// Вид, у которого есть кнопка своего (у «Развороты» — «Сохранить разворот…»).
export const ownKind = (kind: DecorKind): kind is 'bg' | 'sticker' | 'frame' => kind !== 'spread'

// Пустая тема: «Мои» зовут к загрузке (с именем кнопки), остальные — просто пусто.
export function emptyText(theme: string, readOnly: boolean, kind: DecorKind = 'bg'): string {
  if (theme === MINE && kind === 'spread')
    return readOnly ? 'Здесь появятся ваши развороты' : 'Сохраните свой разворот кнопкой «Сохранить разворот…»'
  if (theme === MINE && ownKind(kind)) {
    const what = kind === 'sticker' ? 'стикеры' : kind === 'frame' ? 'рамочки' : 'фоны'
    return readOnly ? `Здесь появятся ваши ${what}` : `Здесь появятся ваши ${what}: нажмите «${ADD_OWN[kind]}»`
  }
  return 'В этой теме пока пусто'
}

// Подсказка кнопки своего: стикеры принимают и SVG, о логотипе — отдельной фразой.
export const uploadHint = (kind: DecorKind): string =>
  `Свои ${kind === 'sticker' ? 'JPEG, PNG и SVG' : kind === 'frame' ? 'PNG с прозрачностью и SVG' : 'JPEG и PNG'}: копия остаётся в библиотеке программы` +
  (kind === 'sticker' ? `. ${OWN_TIP_STICKER}` : '')

export const READ_ONLY_HINT = 'Библиотеку изменила более новая версия программы: свои файлы пока только смотреть'

// Ширина PNG плитки в px: ширина × dpr, шаг 64, ≤ 4096. Адрес Go отдаёт без px.
const TILE_STEP = 64
export function tilePx(cssWidth: number, dpr: number): number {
  const w = cssWidth * (dpr > 0 ? dpr : 1)
  if (!Number.isFinite(w) || w <= 0) return TILE_STEP
  return Math.min(4096, Math.max(TILE_STEP, Math.ceil(w / TILE_STEP) * TILE_STEP))
}
export const tileSrc = (url: string, px: number): string => `${url}${url.includes('?') ? '&' : '?'}px=${px}`

// Области обложки для цвета и узора: от Go по раскладке, но без корешка — его принимает только своё фото (061).
const patternScopes = (cv: Pick<CoverView, 'scopes'> | null | undefined) => coverScopes(cv).filter((s) => s !== 'spine')

// «Куда» (сц. 1): обложка — сторонки по раскладке (без корешка); лист без шва (виньетка-лист) — только весь лист;
// разворот — весь, левая, правая. Один пункт — переключатель прячется.
export function whereItems(
  sp: Pick<SpreadView, 'mid' | 'id'> | undefined,
  cover: Pick<CoverView, 'scopes'> | null | undefined,
  all: string,
): SegItem<BackdropScope>[] {
  if (sp && isCover(sp)) return patternScopes(cover).map((s) => ({ id: s, label: coverLabel(cover, s) }))
  if (!sp || !hasSeam(sp)) return [{ id: 'spread', label: all }]
  return [
    { id: 'spread', label: all },
    { id: 'left', label: 'Левая' },
    { id: 'right', label: 'Правая' },
  ]
}

// Область по умолчанию для входа из меню: у обложки — первая из доступных, иначе весь лист.
export function defaultScope(
  sp: Pick<SpreadView, 'mid' | 'id'> | undefined,
  cover: Pick<CoverView, 'scopes'> | null | undefined,
): BackdropScope {
  return sp && isCover(sp) ? (patternScopes(cover)[0] ?? 'right') : 'spread'
}

// Выбранная область могла пропасть из списка (смена раскладки обложки): тогда первая доступная.
export const scopeIn = (items: readonly SegItem<BackdropScope>[], scope: BackdropScope): BackdropScope =>
  items.some((i) => i.id === scope) ? scope : (items[0]?.id ?? 'spread')

// Итог «Загрузить своё…» (сц. 5) общий со шрифтами (082): живёт в base/words.
export { importSummary } from '../../base/words/importsummary'

export const deleteText = (name: string, kind: DecorKind = 'bg'): string =>
  kind === 'spread'
    ? `«${name}» будет убран из библиотеки. Уже поставленные развороты не изменятся.`
    : kind === 'frame'
      ? `«${name}» будет убрана из библиотеки. Где она уже стоит, не изменится.`
      : `«${name}» будет убран из библиотеки. Где он уже стоит, не изменится.`

// Имя в поле ✎: пробелы по краям лишние; пустое Go отклонит само.
export const cleanName = (s: string): string => s.trim()

// Встроенная плитка узора, из которой, по всему виду, поставлен этот узор (048): в альбоме лежат параметры генератора,
// а не id плитки, поэтому ищем в каталоге по генератору (k в адресе плитки) и цветам. Цвета плитки — только заданные
// ею (i), остальные до комплекта генератор подставляет сам, поэтому плитка подходит, если её цвета — начало цветов
// узора; из подходящих берётся с наибольшим числом заданных цветов («Голубые полосы» раньше «Тонких»). Масштаб,
// «Перемешать» и прозрачность не в счёт: это правка того же узора. Нет такой — undefined (имя — у генератора).
export function presetOf(pat: Pick<PatternSpec, 'kind' | 'inks'>, items: readonly DecorItem[]): DecorItem | undefined {
  const have = (pat.inks ?? []).map((c) => c.toLowerCase())
  let best: DecorItem | undefined
  let bestN = -1
  for (const it of items) {
    if (it.own) continue
    const q = new URLSearchParams(it.url.split('?')[1] ?? '')
    if (q.get('k') !== pat.kind) continue
    const inks = (q.get('i') ?? '')
      .split(',')
      .filter(Boolean)
      .map((h) => `#${h.toLowerCase()}`)
    if (inks.length > have.length || inks.some((c, i) => c !== have[i])) continue
    if (inks.length > bestN) {
      best = it
      bestN = inks.length
    }
  }
  return best
}

// Плитка каталога, стоящая в области (№201): Go отдаёт PatternView.preset — берём её по id; нет поля (узор правили
// руками или альбом старше поля) — запасной путь presetOf. Id нет в каталоге (плитку скрыли) — undefined, имя у Go.
export function presetOfView(p: Pick<PatternView, 'pat' | 'preset'>, items: readonly DecorItem[]): DecorItem | undefined {
  if (p.preset) return items.find((it) => it.id === p.preset)
  return presetOf(p.pat, items)
}

// Плитка, из которой поставлен заменяемый стикер («Заменить…»): своё — по ключу, процедурный — как presetOf.
// Окно «Стикеры» без замены (добавление) ничего не отмечает: '' .
export function standingSticker(sp: SpreadView | undefined, id: string | undefined, items: readonly DecorItem[]): string {
  const st = id ? stickerById(sp, id) : undefined
  if (!st) return ''
  if (st.lib) return items.some((it) => it.id === st.lib) ? st.lib : ''
  return st.known && st.art ? (presetOf(st.art, items)?.id ?? '') : ''
}

// Плитка, которая сейчас стоит в области листа (отметка в окне «Фоны», как текущая раскладка в галерее ⊞):
// своё фото из библиотеки — по ключу, иначе узор — по presetOf. Нет ничего или плитки нет в каталоге — ''.
export function standingId(sp: SpreadView | undefined, scope: BackdropScope, items: readonly DecorItem[]): string {
  const lib = backdropOf(sp, scope)?.lib
  if (lib) return items.some((it) => it.id === lib) ? lib : ''
  const p = patternOf(sp, scope)
  return p?.known ? (presetOfView(p, items)?.id ?? '') : ''
}
