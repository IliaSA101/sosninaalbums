// Выбор нижнего объекта Alt+щелчком (спека 015, design §3). Чистая логика под тесты: стек объектов под
// курсором по View (рамки, углы и z приходят из Go) и решение на нажатие. Без DOM и состояния.
import { placeBox } from '../../base/geom/rotbox'
import { corners, rotate, type Pt, type TextBox } from '../../base/geom/textbox'
import type { SpreadView } from '../../base/core/types'

export type PickKind = 'photo' | 'place' | 'caption' | 'text'

// box — угол до поворота и rot вокруг угла (как у текста); место приведено к такому виду через placeBox.
export interface PickItem {
  kind: PickKind
  ref: string // фото и подпись — ID фото, место — ID места, текст — ID блока
  z: number
  box: TextBox
}

export const sameItem = (a: PickItem | null, b: PickItem | null): boolean => !!a && !!b && a.kind === b.kind && a.ref === b.ref

// Допуск на ошибки округления при повороте (мм).
const EPS = 1e-6

function inBox(t: TextBox, p: Pt): boolean {
  const l = rotate(-t.rot, p.x - t.x, p.y - t.y)
  return l.x >= -EPS && l.x <= t.w + EPS && l.y >= -EPS && l.y <= t.h + EPS
}

// Объекты разворота, рамка которых содержит p (мм разворота), сверху вниз по z. can отсекает то, что уровень
// не даёт выделить. Точка вне разворота — пусто: вылет повёрнутого места обрезан.
export function stackAt(s: SpreadView, p: Pt, sw: number, ph: number, can: (k: PickKind) => boolean): PickItem[] {
  if (p.x < 0 || p.x > sw || p.y < 0 || p.y > ph) return []
  const all: PickItem[] = []
  if (s.grid) {
    for (const pl of s.places ?? []) {
      const kind: PickKind = pl.photo ? 'photo' : 'place'
      all.push({ kind, ref: pl.photo || pl.id, z: pl.z, box: placeBox(pl) })
    }
  } else {
    s.photos.forEach((id, i) => {
      const r = s.rects[i]
      const f = s.frames?.[i]
      if (r && f) all.push({ kind: 'photo', ref: id, z: f.z, box: placeBox({ ...r, rot: f.cellRot }) })
    })
  }
  for (const c of s.captions) all.push({ kind: 'caption', ref: c.photo, z: c.z, box: c })
  for (const t of s.texts) all.push({ kind: 'text', ref: t.id, z: t.z, box: t })
  return all.filter((it) => can(it.kind) && inBox(it.box, p)).sort((a, b) => b.z - a.z)
}

// Следующий под текущим, по кругу. Меньше двух — null. Текущего нет в стеке — второй сверху.
export function nextBelow(stack: PickItem[], cur: PickItem | null): PickItem | null {
  if (stack.length < 2) return null
  const i = stack.findIndex((it) => sameItem(it, cur))
  return i < 0 ? stack[1] : stack[(i + 1) % stack.length]
}

// Есть ли объект, целиком закрытый одним вышележащим (все 4 угла внутри его рамки; рамки выпуклые).
// Сумму нескольких верхних не считаем: дёшево и без ложных подсказок.
export function coveredBelow(stack: PickItem[]): boolean {
  for (let j = 1; j < stack.length; j++) {
    const cs = corners(stack[j].box)
    for (let i = 0; i < j; i++) if (cs.every((c) => inBox(stack[i].box, c))) return true
  }
  return false
}

export interface PressCtx {
  button: number
  alt: boolean
  additive: boolean // Shift/Ctrl/Cmd — группа
  stack: PickItem[]
  cur: PickItem | null // одиночное выделение текущего разворота до нажатия
  picked: PickItem | null // выбранное Alt+щелчком, пока выделение не менялось (вопрос 2)
}

export interface PressPlan {
  press: 'pass' | 'move' // move — перенос cur берём на себя, обработчик верхнего глушим
  move?: PickItem
  click: 'pass' | 'select' // select — щелчок без сдвига выделяет select
  select?: PickItem
  hint: boolean // обычный щелчок над скрытым объектом: подсказать про Alt
}

const PASS: PressPlan = { press: 'pass', click: 'pass', hint: false }

// Решение на pointerdown, таблица design §3 (строки 1–6).
export function pressPlan(c: PressCtx): PressPlan {
  if (c.button !== 0 || c.additive) return PASS
  const { stack, cur } = c
  const top = stack[0] ?? null
  // Текущий лежит под курсором, но закрыт другим.
  const covered = !!cur && stack.some((it) => sameItem(it, cur)) && !sameItem(cur, top)
  if (c.alt) {
    if (stack.length < 2) return PASS
    const select = nextBelow(stack, cur) ?? undefined
    return covered
      ? { press: 'move', move: cur!, click: 'select', select, hint: false }
      : { press: 'pass', click: 'select', select, hint: false }
  }
  if (covered && sameItem(c.picked, cur)) return { press: 'move', move: cur!, click: 'select', select: top!, hint: false }
  return { press: 'pass', click: 'pass', hint: stack.length >= 2 && coveredBelow(stack) }
}
