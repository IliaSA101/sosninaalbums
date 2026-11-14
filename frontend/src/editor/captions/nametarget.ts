// Куда попал двойной щелчок на подписи (спека 037): подпись по шаблону или заглушка с полями имени — цель
// ввода имени; заглушка пустого места (071) — цель ввода таблички места. Чистая логика в мм разворота, без DOM; рамки и
// `who` приходят из Go (View), поворот — вокруг левого верхнего угла рамки, как у подписи.
import { aabb, rotate, type Pt } from '../../base/geom/textbox'
import type { CaptionView, Rect, SpreadView, StubView } from '../../base/core/types'

export type Who = 'last' | 'first' | 'middle' | 'subject'

// photo === '' — заглушка пустого места: имя вводится табличкой места (place — ID места). Иначе place === ''.
export interface NameTarget {
  photo: string
  place: string
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

// Подпись или заглушка под точкой (x, y — мм разворота), верхняя по z. Своя подпись, подпись без полей
// имени и пустое поле — null: там работают прежние жесты («вписать», правка текста). Заглушка пустого места
// с who (Go отдаёт их по шаблону, 071) — цель с place.
export function nameTargetAt(s: SpreadView, x: number, y: number): NameTarget | null {
  const p = { x, y }
  let best: Item | null = null
  for (const it of [...s.captions, ...(s.stubs ?? [])]) {
    if (whoOfItem(it).length === 0) continue
    if (!inside(it, p)) continue
    if (!best || it.z > best.z) best = it
  }
  if (!best) return null
  const place = 'place' in best && !best.photo ? best.place : ''
  return { photo: best.photo, place, who: whoOfItem(best), box: aabb(best) }
}

// Ключ цели: фото или (у пустого места) место; по нему сравнивают цели и ключуют подсказку.
export const targetKey = (t: Pick<NameTarget, 'photo' | 'place'>): string => t.place || t.photo

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

// Поля имени заглушки пустого места (071); пусто — у места нет заглушки с полями (занято фото, шаблона нет).
export function whoOfPlace(s: SpreadView | undefined, place: string): Who[] {
  if (!s || !place) return []
  const it = (s.stubs ?? []).find((t) => t.photo === '' && t.place === place)
  return it ? whoOfItem(it) : []
}

// Подписи и заглушки выделенных мест (мм): подпись фото и заглушка по фото, заглушка пустого места — по месту.
function captionItems(s: SpreadView, sel: { id: string; photo: string }[]): Item[] {
  return sel.flatMap((b) =>
    b.photo
      ? [...s.captions.filter((c) => c.photo === b.photo), ...(s.stubs ?? []).filter((t) => t.photo === b.photo)]
      : (s.stubs ?? []).filter((t) => t.photo === '' && t.place === b.id),
  )
}

// Низ подписи или заглушки выделенных мест (мм, описанный прямоугольник); 0 — подписей нет.
// Мини-панель встаёт ниже этой полосы, иначе щелчки «по подписи» попадают в кнопки.
export function captionBottom(s: SpreadView | undefined, sel: { id: string; photo: string }[]): number {
  if (!s) return 0
  let bottom = 0
  for (const it of captionItems(s, sel)) {
    const a = aabb(it)
    bottom = Math.max(bottom, a.y + a.h)
  }
  return bottom
}

// Боксы подписей и заглушек выделенных мест (мм, описанные прямоугольники). Внутри у низа фото мини-панель сама выбирает те,
// что пересекают её полосу, и встаёт над ними (063).
export function captionBoxes(s: SpreadView | undefined, sel: { id: string; photo: string }[]): Rect[] {
  return s ? captionItems(s, sel).map(aabb) : []
}
