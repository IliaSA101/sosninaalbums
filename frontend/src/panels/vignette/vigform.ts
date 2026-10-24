// Форма мест на плитках виньетки (056): плитка = шаблон + форма. Чистая логика под тесты: фильтр формы,
// набор плиток окна и ▦, что ставит щелчок, листание Пробелом. Форму мест выбирает только щелчок по плитке:
// фильтр лишь отбирает плитки и сам не меняется. Места и портрет считает Go.
import type { VigChoice } from '../../base/core/types'
import { shapeKey, type ShapeKey } from '../../base/geom/shape'
import type { SegItem } from '../../base/tpl/Seg.svelte'
import { visibleTpls, type VigFamilyFilter } from '../../base/tpl/tplfilter'
import { OWN_TITLE, pickedRatio } from './vigown'
import { stepTpl, type Ratio, type Shape, type VigForm } from './vignette'

export type FormFilter = 'all' | ShapeKey

// «Скруглённые» — 10 % меньшей стороны (044 №35, 056 №6): у всех каталожных плиток, чужое скругление листа не рисуется.
export const VIG_ROUND = 10

// Порядок групп плиток и сегментов.
export const SHAPE_ORDER: readonly ShapeKey[] = ['rect', 'round', 'oval']

export const FORMS: Record<ShapeKey, { group: string; name: string; glyph: string; hint: string }> = {
  rect: { group: 'Прямоугольные', name: 'прямоугольные', glyph: '▭', hint: 'Места прямоугольные' },
  round: { group: 'Скруглённые', name: 'скруглённые', glyph: '▢', hint: `Углы мест скруглены (${VIG_ROUND} %)` },
  oval: { group: 'Круги', name: 'круги', glyph: '◯', hint: 'Круглые места; при портрете не 1:1 — овалы' },
}

const ALL: SegItem<FormFilter> = { id: 'all', label: 'Все' }

// Сегменты фильтра формы: в окне — со словами, в ▦ (узкая шапка) — значками, слова в подсказке.
export const FORM_FILTERS: readonly SegItem<FormFilter>[] = [
  ALL,
  ...SHAPE_ORDER.map((k) => ({ id: k, label: `${FORMS[k].glyph} ${FORMS[k].group}`, title: FORMS[k].hint })),
]
export const FORM_FILTERS_SHORT: readonly SegItem<FormFilter>[] = [
  ALL,
  ...SHAPE_ORDER.map((k) => ({ id: k, label: FORMS[k].glyph, title: `${FORMS[k].group}. ${FORMS[k].hint}` })),
]

type FormPair = { shape: Shape; round: number }

// Что встанет на лист для формы плитки: прямоугольник '' / 0, скруглённые '' / 10, круг 'oval' / 0.
export const tileShape = (k: ShapeKey): FormPair => ({ shape: k === 'oval' ? 'oval' : '', round: k === 'round' ? VIG_ROUND : 0 })

// Форма своего шаблона — как сохранена (у круга скругления нет); полей нет — прямоугольник.
function ownShape(c: Pick<VigChoice, 'shape' | 'round'>): FormPair {
  return c.shape === 'oval' ? { shape: 'oval', round: 0 } : { shape: '', round: c.round ?? 0 }
}

// Плитка: шаблон в одной форме. key уникален в наборе (id шаблона один, форм три). sep — заголовок группы перед плиткой.
export interface Tile {
  c: VigChoice
  k: ShapeKey
  key: string
  sep?: string
}

const tile = (c: VigChoice, k: ShapeKey): Tile => ({ c, k, key: `${c.id}|${k}` })

// Форма, в которой рисуется и ставится плитка: свой шаблон — своя, каталожный — по группе.
export const tileFormOf = (t: Pick<Tile, 'c' | 'k'>): FormPair => (t.c.own ? ownShape(t.c) : tileShape(t.k))

type Cur = Pick<VigForm, 'tpl' | 'shape' | 'round'>

// Плитка стоит на листе (в окне — в форме): тот же шаблон и вид формы. Свой шаблон — одна плитка, ищем по id:
// лист мог получить его форму раньше, чем она записана в шаблоне.
export const isCurrent = (t: Pick<Tile, 'c' | 'k'>, cur: Cur): boolean => t.c.id === cur.tpl && (!!t.c.own || t.k === shapeKey(cur))

// Каталожные плитки: группы rect → round → oval, внутри — порядок Go. Заголовок группы — на первой плитке, когда групп
// несколько (одна — и так видна по фильтру).
function catalogTiles(catalog: readonly VigChoice[], fam: VigFamilyFilter, form: FormFilter, cur: Cur): Tile[] {
  const byFam = new Set(visibleTpls(catalog, fam))
  const out: Tile[] = []
  const heads: Tile[] = []
  for (const k of SHAPE_ORDER) {
    const before = out.length
    for (const c of catalog) {
      const t = tile(c, k)
      if ((byFam.has(c) && (form === 'all' || form === k)) || isCurrent(t, cur)) out.push(t)
    }
    if (out.length > before) {
      out[before].sep = FORMS[k].group
      heads.push(out[before])
    }
  }
  if (heads.length < 2) for (const t of heads) delete t.sep
  return out
}

// Свои плитки: по одной в своей форме; форму отбирает фильтр формы, вид их не касается. Первая — под «Мои шаблоны».
function ownTiles(own: readonly VigChoice[], form: FormFilter, cur: Cur): Tile[] {
  const out = own.map((c) => tile(c, shapeKey(ownShape(c)))).filter((t) => form === 'all' || t.k === form || isCurrent(t, cur))
  if (out.length > 0) out[0].sep = OWN_TITLE
  return out
}

// Все плитки набора по порядку показа. items — ответ Go (каталог, потом свои). Текущая плитка видна всегда (иначе
// листание «теряет» выбор), даже вне фильтров.
export const vigTiles = (items: readonly VigChoice[], fam: VigFamilyFilter, form: FormFilter, cur: Cur): Tile[] => [
  ...catalogTiles(
    items.filter((c) => !c.own),
    fam,
    form,
    cur,
  ),
  ...ownTiles(
    items.filter((c) => c.own),
    form,
    cur,
  ),
]

// Ключ плитки, что стоит сейчас; нет такой (шаблон не пришёл) — ''.
export const curKey = (tiles: readonly Tile[], cur: Cur): string => tiles.find((t) => isCurrent(t, cur))?.key ?? ''

// Щелчок по плитке: шаблон и форма плитки; портрет каталожная плитка не трогает (круг при 3:4 — овал),
// своя ставит свой. Остальное (числа, фильтры, другие плитки) не меняется.
export function pickTile(t: Tile, curRatio: string): Pick<VigForm, 'tpl' | 'shape' | 'round'> & { ratio?: Ratio } {
  const r = pickedRatio([t.c], t.c.id, curRatio)
  return { tpl: t.c.id, ...tileFormOf(t), ...(r ? { ratio: r } : {}) }
}

// Пробел и стрелки: следующая (delta > 0) или предыдущая видимая плитка по кругу, «не поместится» пропускается.
// Нечего выбирать (видна одна — текущая) — null.
export function stepTile(tiles: readonly Tile[], cur: Cur, delta: number): Tile | null {
  const key = stepTpl(
    tiles.map((t) => ({ id: t.key, over: t.c.over })),
    curKey(tiles, cur),
    delta,
  )
  return tiles.find((t) => t.key === key) ?? null
}
