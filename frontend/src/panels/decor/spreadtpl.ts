// Окно «Развороты» (067 Ф1): чистые решения — какие плитки видны на «Куда», размер и адрес картинки плитки,
// слова вопроса и тостов. Места и оформление ставит Go; здесь ничего не считается, кроме отбора и пропорций.
import type { BackdropScope, GridPlan, SpreadView } from '../../base/core/types'
import { hasSeam } from '../../base/geom/sheet'
import { plural } from '../../base/words/plural'
import { isVignette } from '../vignette/vignette'
import type { SegItem } from '../../base/tpl/Seg.svelte'
import type { SideItem } from '../../base/tpl/SideList.svelte'
import { sideCounts, type BrowseSel, NO_SEL } from '../decorbrowse/browse'
import { noOwn, tilePx, type DecorCatalog, type DecorItem } from './decor'
import { isGetItem, type GetKind } from '../decorbrowse/decorget'

type SheetFor = Pick<SpreadView, 'w' | 'h' | 'mid'>

// Плитка встанет на область? Шаблон разворота (pages 2) — только на «Весь разворот» листа из двух страниц;
// страница (pages 1) — на любую область. Так «Левая» и «Правая» показывают только плитки-страницы.
export const tplFits = (it: Pick<DecorItem, 'pages'>, scope: BackdropScope, sp: Pick<SpreadView, 'mid'>): boolean =>
  it.pages === 1 || (scope === 'spread' && hasSeam(sp))

// Плитки скачивания (144) «Куда» не отбирает: страница это или разворот, видно по скачанному предмету.
export const tplVisible = (items: readonly DecorItem[], scope: BackdropScope, sp: Pick<SpreadView, 'mid'>): DecorItem[] =>
  items.filter((it) => isGetItem(it) || tplFits(it, scope, sp))

// Счётчики тем считаются по тому же отбору «Куда» (и «Набор», 133), что и плитки (Go считает по всем): иначе «Недавние 3» при двух плитках.
export function tplSideItems(
  cat: DecorCatalog | null,
  scope: BackdropScope,
  sp: Pick<SpreadView, 'mid'>,
  sel: BrowseSel = NO_SEL,
  keep = '',
): SideItem<string>[] {
  return sideCounts(cat, sel, { extra: (list) => tplVisible(list, scope, sp), keep })
}

// Пустая тема после отбора по «Куда», хотя в ней что-то есть: подсказка вместо немой пустоты (owner-ux §2.7).
export const FILTERED_EMPTY = 'Здесь только развороты на две страницы — выберите «Весь разворот»'

// Ширина/высота страницы и листа целиком. Лист без шва (031, виньетка) — одна «страница» во весь лист.
export const pageRatio = (sp: SheetFor): number => (hasSeam(sp) ? sp.mid / sp.h : sp.w / sp.h)
export const sheetRatio = (sp: SheetFor): number => sp.w / sp.h

// Пропорция плитки «Можно скачать» (144) — как у стоящей плитки вида: фон — лист, развороты — лист или страница по «Куда»
// (tileLook.box); стикеры и рамочки — квадрат, его знает getRatio.
export const getBoxFor = (kind: GetKind, sp: SheetFor, scope: BackdropScope): number | undefined =>
  kind === 'bg' ? sheetRatio(sp) : kind === 'spread' ? (scope === 'spread' ? sheetRatio(sp) : pageRatio(sp)) : undefined

// Пределы Go для запроса плитки: px 64…1024, r 0,2…5 (иначе 404).
const PX_MAX = 1024
const R_MIN = 0.2
const R_MAX = 5

export interface TileLook {
  box: number // пропорция кнопки-плитки (ширина/высота)
  r: number // пропорция картинки: страница или лист
  px: number // ширина PNG
  page: boolean // страница в окне «Весь разворот»: картинка уже кнопки, по центру
}

// Кнопка плитки одной пропорции на всё окно: «Весь разворот» — лист, «Левая/Правая» — страница. Плитка-страница в
// широкой кнопке стоит по центру на своей пропорции, и PNG для неё заказывается уже (меньше px).
export function tileLook(it: Pick<DecorItem, 'pages'>, sp: SheetFor, scope: BackdropScope, tileW: number, dpr: number): TileLook {
  const box = scope === 'spread' ? sheetRatio(sp) : pageRatio(sp)
  const r = it.pages === 1 ? pageRatio(sp) : sheetRatio(sp)
  const w = r < box ? tileW * (r / box) : tileW
  const px = Math.min(PX_MAX, tilePx(w, dpr))
  return { box, r: Math.min(R_MAX, Math.max(R_MIN, r)), px, page: r < box }
}

// Адрес плитки: Go отдаёт /media/tpl/<id>?v=1, px и r дописывает окно.
export const tplSrc = (url: string, px: number, r: number): string =>
  `${url}${url.includes('?') ? '&' : '?'}px=${px}&r=${Math.round(r * 10000) / 10000}`

// Подсказка плитки для всех трёх видов: имя, «страница» (у разворотов), набор.
export const tplTitle = (it: Pick<DecorItem, 'name' | 'pack' | 'pages'>): string =>
  [it.name, tplNote(it), it.pack ? `набор «${it.pack}»` : ''].filter(Boolean).join(' · ')
export const tplNote = (it: Pick<DecorItem, 'pages'>): string | undefined => (it.pages === 1 ? 'страница' : undefined)

// Вопрос перед постановкой (сц. 3): уходят фото в ленту или заменяются стикеры и надписи. null — ставить сразу.
// «Авто» → «Сетка» вопроса не требует. Места виньетки и листа 031 шаблон не меняет — только если они в шаблоне есть.
export function putQuestion(plan: GridPlan, sp: Pick<SpreadView, 'mid' | 'kind' | 'vig'> | undefined, places = 0): string | null {
  const { left, texts, stickers } = plan
  if (!(left > 0 || texts > 0 || stickers > 0)) return null
  const parts: string[] = []
  if (left > 0) parts.push(`${left} ${plural(left, ['фото вернётся', 'фото вернутся', 'фото вернутся'])} в ленту`)
  if (texts > 0) parts.push(`${texts} ${plural(texts, ['блок текста заменится', 'блока текста заменятся', 'блоков текста заменятся'])}`)
  if (stickers > 0) parts.push(`${stickers} ${plural(stickers, ['стикер заменится', 'стикера заменятся', 'стикеров заменятся'])}`)
  const fixed = sp && places > 0 ? noPlaces(sp) : ''
  return `${parts.join(', ')}. ${fixed ? `${fixed} ` : ''}Поставить?`
}

// Где места не ставятся: виньетка и лист 031 (без шва).
function noPlaces(sp: Pick<SpreadView, 'mid' | 'kind' | 'vig'>): string {
  if (isVignette(sp)) return 'Места виньетки не меняются.'
  return hasSeam(sp) ? '' : 'Места листа не меняются.'
}

export interface PutToast {
  text: string
  undo: boolean // правка была: тост с «Вернуть»
}

// Итог постановки. Версия не выросла — Go ничего не менял (повтор), шага отмены нет. cover — «Поставить на обложку» (131).
export function putToast(changed: boolean, left: number, missing: number, cover = false): PutToast {
  const lack = missing > 0 ? `Нет в библиотеке: ${missing}` : ''
  if (!changed)
    return { text: [cover ? 'Здесь уже такая обложка' : 'Здесь уже такой разворот', lack].filter(Boolean).join(' · '), undo: false }
  const back = left > 0 ? `${left} ${plural(left, ['фото вернулось', 'фото вернулись', 'фото вернулись'])} в ленту` : ''
  return { text: [cover ? 'Обложка поставлена' : 'Разворот поставлен', back, lack].filter(Boolean).join(' · '), undo: true }
}

// ---------- «Сохранить разворот…» ----------

export const SAVE_TITLE = 'Сохранить разворот в библиотеку'
export const SAVE_HINT =
  'Фото не сохраняются — только места. Фото фона из съёмки не входит: сохраняются цвет, узор, фоны и стикеры библиотеки.'
export const SAVED = 'Разворот сохранён в библиотеку'

// Темы для сохранения: настоящие (не выборки «Все», «Недавние», «Новые» и не «Фирменные» — noOwn).
export const saveThemes = (cat: Pick<DecorCatalog, 'themes'> | null): { id: string; name: string }[] =>
  (cat?.themes ?? []).filter((t) => !noOwn(t.id)).map((t) => ({ id: t.id, name: t.name }))

// Тема по умолчанию: открытая в окне, если она настоящая; из выборок и «Фирменных» — «Мои».
export const saveTheme = (theme: string): string => (noOwn(theme) || theme === '' ? 'mine' : theme)

// Области «Что» для сохранения: те же, что «Куда», но без обложки (её сторонки сюда не попадают).
export const saveScopes = (items: readonly SegItem<BackdropScope>[]): SegItem<BackdropScope>[] =>
  items.filter((i) => i.id === 'spread' || i.id === 'left' || i.id === 'right')
