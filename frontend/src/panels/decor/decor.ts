// Окно «Фоны» (048): чистые решения — темы, плитки, «Куда», итог загрузки, тексты. Каталог и правки — у Go;
// здесь ничего не считается, кроме отбора плиток по теме и шага размера картинки.
import type { app } from '../../../wailsjs/go/models'
import type { BackdropScope, PatternSpec, PatternView, SpreadView } from '../../base/core/types'
import { hasSeam, isCover } from '../../base/geom/sheet'
import { backdropOf } from '../../editor/backdrop/backdrop'
import { patternOf } from '../../editor/backdrop/pattern'
import { stickerById } from '../../editor/sticker/sticker'
import { COVER_SCOPE_LABEL, layoutScopes } from '../../editor/cover/cover'
import type { SegItem } from '../../base/tpl/Seg.svelte'
import type { SideItem } from '../../base/tpl/SideList.svelte'

export type DecorKind = 'bg' | 'sticker'

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

export const DECOR_TITLE: Record<DecorKind, string> = { bg: 'Фоны', sticker: 'Стикеры' }
export const NAME_MAX = 60
export const RECENT = 'recent'
export const ALL = 'all'
export const MINE = 'mine'

// Тема при открытии: прежний выбор, если он ещё есть; иначе «Недавние» (если не пусты), иначе «Все».
export function firstTheme(cat: Pick<DecorCatalog, 'themes'> | null, prev = ''): string {
  if (!cat) return ALL
  const has = (id: string) => cat.themes.some((t) => t.id === id)
  if (prev && has(prev)) return prev
  return cat.themes.find((t) => t.id === RECENT && t.count > 0) ? RECENT : ALL
}

export const sideItems = (cat: Pick<DecorCatalog, 'themes'> | null): SideItem<string>[] =>
  (cat?.themes ?? []).map((t) => ({ id: t.id, label: t.name, count: t.count }))

// Плитки темы. «Недавние» — в порядке поставленных (новые первыми), «Все» — каталог целиком.
export function themeItems(cat: DecorCatalog | null, theme: string): DecorItem[] {
  if (!cat) return []
  if (theme === ALL) return cat.items
  if (theme === RECENT) {
    const by = new Map(cat.items.map((it) => [it.id, it]))
    return cat.recent.flatMap((id) => by.get(id) ?? [])
  }
  return cat.items.filter((it) => it.theme === theme)
}

// Своё из «Все» и «Недавних» кладётся в «Мои» (в выборках лежит не тема, а отбор).
export const importTheme = (theme: string): string => (theme === ALL || theme === RECENT ? MINE : theme)

// Пустая тема: «Мои» зовут к загрузке, остальные — просто пусто.
export function emptyText(theme: string, readOnly: boolean): string {
  if (theme === MINE) return readOnly ? 'Здесь появятся ваши фоны' : 'Загрузите свои кнопкой выше'
  return 'В этой теме пока пусто'
}

// Подсказка кнопки «Загрузить своё…»: стикеры принимают и SVG.
export const uploadHint = (kind: DecorKind): string =>
  `Свои ${kind === 'sticker' ? 'JPEG, PNG и SVG' : 'JPEG и PNG'}: копия остаётся в библиотеке программы`

export const READ_ONLY_HINT = 'Библиотеку изменила более новая версия программы: свои файлы пока только смотреть'

// Ширина PNG плитки в px: ширина × dpr, шаг 64, ≤ 4096. Адрес Go отдаёт без px.
const TILE_STEP = 64
export function tilePx(cssWidth: number, dpr: number): number {
  const w = cssWidth * (dpr > 0 ? dpr : 1)
  if (!Number.isFinite(w) || w <= 0) return TILE_STEP
  return Math.min(4096, Math.max(TILE_STEP, Math.ceil(w / TILE_STEP) * TILE_STEP))
}
export const tileSrc = (url: string, px: number): string => `${url}${url.includes('?') ? '&' : '?'}px=${px}`

// «Куда» (сц. 1): обложка — сторонки по раскладке; лист без шва (виньетка-лист) — только весь лист;
// разворот — весь, левая, правая. Один пункт — переключатель прячется.
export function whereItems(
  sp: Pick<SpreadView, 'mid' | 'id'> | undefined,
  coverLayout: string | undefined,
  all: string,
): SegItem<BackdropScope>[] {
  if (sp && isCover(sp)) return layoutScopes(coverLayout).map((s) => ({ id: s, label: COVER_SCOPE_LABEL[s] }))
  if (!sp || !hasSeam(sp)) return [{ id: 'spread', label: all }]
  return [
    { id: 'spread', label: all },
    { id: 'left', label: 'Левая' },
    { id: 'right', label: 'Правая' },
  ]
}

// Область по умолчанию для входа из меню: у обложки — первая из доступных, иначе весь лист.
export function defaultScope(sp: Pick<SpreadView, 'mid' | 'id'> | undefined, coverLayout: string | undefined): BackdropScope {
  return sp && isCover(sp) ? layoutScopes(coverLayout)[0] : 'spread'
}

// Выбранная область могла пропасть из списка (смена раскладки обложки): тогда первая доступная.
export const scopeIn = (items: readonly SegItem<BackdropScope>[], scope: BackdropScope): BackdropScope =>
  items.some((i) => i.id === scope) ? scope : (items[0]?.id ?? 'spread')

const WHY_MAX = 2 // сколько причин вписывать в тост

// Итог «Загрузить своё…» (сц. 5): «Добавлено: 3, пропущено: 1 (имя — причина)». Пусто — диалог закрыли ничего не выбрав.
export function importSummary(r: DecorImport): { text: string; error: boolean } | null {
  const added = r.added.length
  const skipped = r.skipped.length
  if (added === 0 && skipped === 0) return null
  const head = skipped > 0 ? `Добавлено: ${added}, пропущено: ${skipped}` : `Добавлено: ${added}`
  if (skipped === 0) return { text: head, error: false }
  const why = r.skipped.slice(0, WHY_MAX).map((s) => `${s.name} — ${s.why}`)
  const rest = skipped - why.length
  return { text: `${head} (${why.join('; ')}${rest > 0 ? ` и ещё ${rest}` : ''})`, error: added === 0 }
}

export const deleteText = (name: string): string => `«${name}» будет убран из библиотеки. Где он уже стоит, не изменится.`

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
