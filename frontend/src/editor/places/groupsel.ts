// Модель выделения группы (спека 009, gestures.md, design §4.2). Чистая логика под тесты:
// фото и места лежат в placesel.refs, тексты группы — в placesel.texts, одиночный текст живёт в edit.sel.
// Группа — 2 и больше объекта. Фон в выделение не входит: его правят в режиме фона.
import type { SpreadView, Zone } from '../../base/core/types'

// Тип лежит в основе (store.zone); здесь — для остальных.
export type { Zone }

export interface GroupSel {
  refs: string[] // фото и пустые места (ref, как в placesel)
  texts: string[] // тексты группы
  text: string | null // одиночный текст (edit.sel), когда выделен только он
}

export const EMPTY: GroupSel = { refs: [], texts: [], text: null }

export const count = (s: GroupSel) => s.refs.length + s.texts.length + (s.text ? 1 : 0)
export const isGroup = (s: GroupSel) => count(s) > 1

// Единственный текст без фото — одиночный (правка, курсор); иначе все тексты лежат в группе.
export function normalize(s: GroupSel): GroupSel {
  const texts = s.text ? [s.text, ...s.texts] : s.texts
  if (s.refs.length === 0 && texts.length === 1) return { refs: [], texts: [], text: texts[0] }
  return { refs: s.refs, texts, text: null }
}

// Shift/Ctrl+щелчок по фото или месту: добавить или убрать. Одиночный текст при этом уходит в группу.
export function toggleRef(s: GroupSel, ref: string): GroupSel {
  const refs = s.refs.includes(ref) ? s.refs.filter((r) => r !== ref) : [...s.refs, ref]
  return normalize({ refs, texts: s.texts, text: s.text })
}

// Shift/Ctrl+щелчок по тексту: добавить или убрать; остаётся один текст — он снова одиночный.
export function toggleText(s: GroupSel, id: string): GroupSel {
  const all = s.text ? [s.text, ...s.texts] : s.texts
  const texts = all.includes(id) ? all.filter((t) => t !== id) : [...all, id]
  return normalize({ refs: s.refs, texts, text: null })
}

// Ctrl+A на холсте: все фото, места и тексты разворота. У «Ручной» занятое место — по фото
// (так выделяет щелчок по ячейке), пустое — по ID места; подписи не входят.
export function selectAll(s: SpreadView | undefined): GroupSel {
  if (!s) return EMPTY
  const refs = s.grid ? (s.places ?? []).map((p) => p.photo || p.id) : [...s.photos]
  return normalize({ refs, texts: s.texts.map((t) => t.id), text: null })
}

// После правки: тексты, которых нет на развороте, выпадают.
export function pruneTexts(texts: string[], s: SpreadView | undefined): string[] {
  const left = texts.filter((id) => s?.texts.some((t) => t.id === id))
  return left.length === texts.length ? texts : left
}

// Выделение после правки: пропавшие тексты выпадают, остаток приводится к норме (из группы из двух
// осталось одно — оно снова одиночное, а не «текст группы без группы»).
export function pruneSel(s: GroupSel, spread: SpreadView | undefined): GroupSel {
  return normalize({ refs: s.refs, texts: pruneTexts(s.texts, spread), text: s.text })
}

// Зона по элементу под курсором; null — нажатие вне библиотеки, холста и ленты (зона не меняется).
export function zoneOf(el: { closest(sel: string): unknown } | null | undefined): Zone | null {
  if (el?.closest('.library')) return 'library'
  if (el?.closest('.stage, .tl')) return 'canvas'
  return null
}
