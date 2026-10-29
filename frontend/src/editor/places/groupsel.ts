// Модель выделения группы (спека 009, gestures.md, design §4.2). Чистая логика под тесты:
// фото и места лежат в placesel.refs, тексты группы — в placesel.texts, одиночный текст живёт в edit.sel,
// стикеры (048) — в placesel.stickers (один стикер тоже там, отдельного «одиночного» нет).
// Группа — 2 и больше объекта. Фон в выделение не входит: его правят в режиме фона.
import type { SpreadView, Zone } from '../../base/core/types'

// Тип лежит в основе (store.zone); здесь — для остальных.
export type { Zone }

export interface GroupSel {
  refs: string[] // фото и пустые места (ref, как в placesel)
  texts: string[] // тексты группы
  text: string | null // одиночный текст (edit.sel), когда выделен только он
  stickers?: string[] // стикеры (048); нет поля — пусто
}

export const EMPTY: GroupSel = { refs: [], texts: [], text: null }

const stk = (s: GroupSel): string[] => s.stickers ?? []
// Поле stickers есть, только когда в нём что-то лежит: выделение без стикеров остаётся прежнего вида.
const withStickers = (g: GroupSel, stickers: string[]): GroupSel => (stickers.length > 0 ? { ...g, stickers } : g)

export const count = (s: GroupSel) => s.refs.length + s.texts.length + stk(s).length + (s.text ? 1 : 0)
export const isGroup = (s: GroupSel) => count(s) > 1

// Единственный текст без фото и стикеров — одиночный (правка, курсор); иначе все тексты лежат в группе.
export function normalize(s: GroupSel): GroupSel {
  const texts = s.text ? [s.text, ...s.texts] : s.texts
  const stickers = stk(s)
  if (s.refs.length === 0 && stickers.length === 0 && texts.length === 1) return { refs: [], texts: [], text: texts[0] }
  return withStickers({ refs: s.refs, texts, text: null }, stickers)
}

// Shift/Ctrl+щелчок по фото или месту: добавить или убрать. Одиночный текст при этом уходит в группу.
export function toggleRef(s: GroupSel, ref: string): GroupSel {
  const refs = s.refs.includes(ref) ? s.refs.filter((r) => r !== ref) : [...s.refs, ref]
  return normalize(withStickers({ refs, texts: s.texts, text: s.text }, stk(s)))
}

// Shift/Ctrl+щелчок по тексту: добавить или убрать; остаётся один текст — он снова одиночный.
export function toggleText(s: GroupSel, id: string): GroupSel {
  const all = s.text ? [s.text, ...s.texts] : s.texts
  const texts = all.includes(id) ? all.filter((t) => t !== id) : [...all, id]
  return normalize(withStickers({ refs: s.refs, texts, text: null }, stk(s)))
}

// Shift/Ctrl+щелчок по стикеру: добавить или убрать; одиночный текст уходит в группу.
export function toggleSticker(s: GroupSel, id: string): GroupSel {
  const cur = stk(s)
  const stickers = cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id]
  return normalize(withStickers({ refs: s.refs, texts: s.text ? [s.text, ...s.texts] : s.texts, text: null }, stickers))
}

// Ctrl+A на холсте: все фото, места, тексты и стикеры разворота. У «Ручной» занятое место — по фото
// (так выделяет щелчок по ячейке), пустое — по ID места; подписи не входят.
export function selectAll(s: SpreadView | undefined): GroupSel {
  if (!s) return EMPTY
  const refs = s.grid ? (s.places ?? []).map((p) => p.photo || p.id) : [...s.photos]
  const stickers = (s.stickers ?? []).map((t) => t.id)
  return normalize(withStickers({ refs, texts: s.texts.map((t) => t.id), text: null }, stickers))
}

// После правки: тексты, которых нет на развороте, выпадают.
export function pruneTexts(texts: string[], s: SpreadView | undefined): string[] {
  const left = texts.filter((id) => s?.texts.some((t) => t.id === id))
  return left.length === texts.length ? texts : left
}

// То же для стикеров.
export function pruneStickers(ids: string[], s: SpreadView | undefined): string[] {
  const left = ids.filter((id) => s?.stickers?.some((t) => t.id === id))
  return left.length === ids.length ? ids : left
}

// Выделение после правки: пропавшие тексты и стикеры выпадают, остаток приводится к норме (из группы из двух
// осталось одно — оно снова одиночное, а не «текст группы без группы»).
export function pruneSel(s: GroupSel, spread: SpreadView | undefined): GroupSel {
  return normalize(withStickers({ refs: s.refs, texts: pruneTexts(s.texts, spread), text: s.text }, pruneStickers(stk(s), spread)))
}

// Зона по элементу под курсором; null — нажатие вне библиотеки, холста и ленты (зона не меняется).
// Модальное окно (просмотр, диалоги) зону не меняет, даже если по DOM лежит внутри холста.
export function zoneOf(el: { closest(sel: string): unknown } | null | undefined): Zone | null {
  if (el?.closest('[aria-modal="true"]')) return null
  if (el?.closest('.library')) return 'library'
  if (el?.closest('.stage, .tl')) return 'canvas'
  return null
}
