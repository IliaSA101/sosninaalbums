// Режим фона (спека 009, design-backdrop §6): чистые решения — что под курсором, что делает щелчок,
// жив ли режим. Всё в мм разворота; рамка повёрнута по часовой вокруг своего левого верхнего угла.
import { placeImage } from '../../base/geom/frame'
import { rotate, type Pt, type TextBox } from '../../base/geom/textbox'
import type { BackdropScope, BackdropView, Frame, Rect, SpreadView } from '../../base/core/types'

export interface BgCur {
  spread: string
  scope: BackdropScope
}

// Предпросмотр жеста: рамка и кадр, пока кнопка не отпущена (Go ещё не спрошен).
export interface BgLive {
  box: TextBox
  frame: Frame
}

const EPS = 1e-6

// Мелкая рамка (< 48 px на экране; у корешка 6 мм при «вписать» — 5–13 px): ползунок масштаба не помещается и лёг бы
// поверх ручек (⟳, стороны), поэтому его нет — масштаб есть в карточке «Фото фона». Порог тот же, что у CellOverlay.
export const MIN_SLIDER_PX = 48
export function roomForSlider(box: { w: number; h: number }, ppm: number): boolean {
  return Math.min(box.w, box.h) * ppm >= MIN_SLIDER_PX
}

const inRect = (r: Rect, p: Pt) => p.x >= r.x - EPS && p.x <= r.x + r.w + EPS && p.y >= r.y - EPS && p.y <= r.y + r.h + EPS

// Точка внутри повёрнутой рамки: переводим в оси рамки.
export function inBox(b: TextBox, p: Pt): boolean {
  const l = rotate(-b.rot, p.x - b.x, p.y - b.y)
  return l.x >= -EPS && l.x <= b.w + EPS && l.y >= -EPS && l.y <= b.h + EPS
}

// Фон, чьё видимое фото под точкой: внутри области (она обрезает) и внутри рамки.
export function backdropAt(bs: readonly BackdropView[] | null | undefined, p: Pt): BackdropScope | null {
  for (const b of bs ?? []) if (inRect(b.area, p) && inBox(b.box, p)) return b.scope
  return null
}

// Щелчок в режиме мимо ручек: по фото фона другой области — перейти к нему, по своему — ничего,
// иначе — выход (поле, приглушённые места и тексты, серое поле).
export function modeClick(at: BackdropScope | null, cur: BackdropScope): 'exit' | 'switch' | 'none' {
  if (at === null) return 'exit'
  return at === cur ? 'none' : 'switch'
}

// Режим жив, пока его разворот текущий, фон этой области есть и ничего другого не выделено
// (Undo постановки, «Убрать», чужая правка, переход на другой разворот и выделение его закрывают:
// иначе Del и стрелки достались бы и выделению, и фону).
export function keepMode(cur: BgCur | null, spread: SpreadView | null | undefined, selected = false): boolean {
  if (!cur || !spread || spread.id !== cur.spread || selected) return false
  return !!spread.backdrops?.some((b) => b.scope === cur.scope)
}

// Фото в осях рамки при предпросмотре: так же, как Go (placeImage по рамке и кадру); пропорции фото —
// из img последнего View.
export function liveImg(b: BackdropView, live: BgLive): Rect {
  const aspect = b.img.h > 0 ? b.img.w / b.img.h : 1
  return placeImage({ x: 0, y: 0, w: live.box.w, h: live.box.h }, aspect, live.frame)
}

// Сессия стрелок для merge в Go: [A-Za-z0-9-], ≤ 32 знаков.
export const bdSession = (t = Date.now()) => `bd-${t.toString(36)}`

// Подпись области на полосе режима.
export const SCOPE_BAR: Record<BackdropScope, string> = {
  spread: 'весь разворот',
  left: 'левая страница',
  right: 'правая страница',
  spine: 'корешок',
}
