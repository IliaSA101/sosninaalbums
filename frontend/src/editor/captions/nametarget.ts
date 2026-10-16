// Куда попал двойной щелчок на подписи (спека 037): подпись по шаблону или заглушка с полями имени — цель
// ввода имени; заглушка пустого места — цель подсказки. Чистая логика в мм разворота, без DOM; рамки и
// `who` приходят из Go (View), поворот — вокруг левого верхнего угла рамки, как у подписи.
import { aabb, rotate, type Pt } from '../../base/geom/textbox'
import { PERSON_FIELDS } from './fields'
import type { CaptionView, Rect, SpreadView, StubView } from '../../base/core/types'

export type Who = 'last' | 'first' | 'middle' | 'subject'

// photo === '' — заглушка пустого места: имени у неё нет (имя — свойство фото).
export interface NameTarget {
  photo: string
  who: Who[]
  box: Rect // описанный прямоугольник, мм: от него окошко ставится под подпись
}

type Item = CaptionView | StubView

const EPS = 1e-6

function inside(t: Item, p: Pt): boolean {
  const l = rotate(-t.rot, p.x - t.x, p.y - t.y)
  return l.x >= -EPS && l.x <= t.w + EPS && l.y >= -EPS && l.y <= t.h + EPS
}

const WHO: readonly string[] = ['last', 'first', 'middle', 'subject']
const whoOfItem = (t: Item): Who[] => (t.who ?? []).filter((w): w is Who => WHO.includes(w))

// Шаблон подписи ждёт имя: в нём есть {Фамилия}, {Имя}, {Отчество} или {Предмет} (регистр не важен).
// «Предмет» не в PERSON_FIELDS: оттуда берётся меню свадебного листа (решение 173).
const hasPersonField = (t: string | undefined): boolean =>
  !!t && [...PERSON_FIELDS, 'Предмет'].some((f) => t.toLowerCase().includes(`{${f.toLowerCase()}}`))

// В листе есть поля имени: у какой-либо подписи или заглушки есть who, либо шаблон листа (подписи к каждому
// фото, подпись учителя виньетки) содержит поле имени — тогда пустое место тоже ждёт имя, тост уместен.
// Заглушки пустых мест who не несут, поэтому шаблон смотрим отдельно.
export function hasNameFields(s: SpreadView): boolean {
  if (hasPersonField(s.caps?.text) || hasPersonField(s.vig?.ttext)) return true
  return [...s.captions, ...(s.stubs ?? [])].some((t) => whoOfItem(t).length > 0)
}

// Подпись или заглушка под точкой (x, y — мм разворота), верхняя по z. Своя подпись, подпись без полей
// имени и пустое поле — null: там работают прежние жесты («вписать», правка текста).
export function nameTargetAt(s: SpreadView, x: number, y: number): NameTarget | null {
  const p = { x, y }
  const fields = hasNameFields(s)
  const items: [Item, boolean][] = [
    ...s.captions.map((c): [Item, boolean] => [c, false]),
    ...(s.stubs ?? []).map((t): [Item, boolean] => [t, t.photo === '' && fields]),
  ]
  let best: Item | null = null
  for (const [it, emptyStub] of items) {
    if (!emptyStub && whoOfItem(it).length === 0) continue
    if (!inside(it, p)) continue
    if (!best || it.z > best.z) best = it
  }
  if (!best) return null
  return { photo: best.photo, who: whoOfItem(best), box: aabb(best) }
}

// Признаки цели жеста (двойной щелчок, наведение): брать ли разворот под цель ввода имени. Цель по точке
// не знает, что лежит выше по z (свободный текст, фото) и открыт ли редактор, — это решают признаки:
// открытый редактор текста — нет; иначе только пустое поле или подпись с полями имени (`.hit.caption.named`).
export interface PointCtx {
  can: boolean // уровень «подписи» включён
  bgEdit: boolean // режим фона
  editing: boolean // открыт редактор текста
  empty: boolean // цель — пустое поле (emptyField)
  onNamed: boolean // цель — подпись с полями имени
}

export const takesNameTarget = (c: PointCtx): boolean => c.can && !c.bgEdit && !c.editing && (c.empty || c.onNamed)

export const NAME_W = 220 // минимальная ширина окошка имени, px (шире — по подписи)
const NAME_GAP = 4

// Где стоит окошко имени (px от угла разворота): под подписью по её центру, не выходя за разворот по бокам;
// снизу нет места — над подписью. h — высота окошка, px.
export function namePlace(box: Rect, ppm: number, width: number, height: number, h: number, w = NAME_W): { left: number; top: number } {
  const half = w / 2
  const left = Math.max(half, Math.min((box.x + box.w / 2) * ppm, width - half))
  const below = (box.y + box.h) * ppm + NAME_GAP
  if (below + h <= height) return { left, top: below }
  const above = box.y * ppm - NAME_GAP - h
  return { left, top: above >= 0 ? above : Math.max(0, height - h) }
}

// Поля имени подписи фото (подпись по шаблону или заглушка фото); пусто — шаблона с полями нет.
export function whoOf(s: SpreadView | undefined, photo: string): Who[] {
  if (!s || !photo) return []
  const it = s.captions.find((c) => c.photo === photo) ?? (s.stubs ?? []).find((t) => t.photo === photo)
  return it ? whoOfItem(it) : []
}

// Низ подписи или заглушки выделенных мест (мм, описанный прямоугольник); 0 — подписей нет.
// Мини-панель встаёт ниже этой полосы, иначе щелчки «по подписи» попадают в кнопки.
export function captionBottom(s: SpreadView | undefined, sel: { id: string; photo: string }[]): number {
  if (!s) return 0
  let bottom = 0
  for (const b of sel) {
    const items: Item[] = b.photo
      ? [...s.captions.filter((c) => c.photo === b.photo), ...(s.stubs ?? []).filter((t) => t.photo === b.photo)]
      : (s.stubs ?? []).filter((t) => t.photo === '' && t.place === b.id)
    for (const it of items) {
      const a = aabb(it)
      bottom = Math.max(bottom, a.y + a.h)
    }
  }
  return bottom
}
