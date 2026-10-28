// Рамочка места на холсте (079 U2): чистые решения для ячейки и пустого места. Рисунок рамочки, окно и
// маску считает Go (View, /media); здесь только перевод мм в проценты, размер запроса и загрузка маски.
import type { FrameView, Rect } from '../../base/core/types'
import { insetBox } from '../../base/geom/inset'
import { outlineOf } from '../../base/geom/shape'
import { layerPx, patternSrc } from '../pattern/pattern'

// Ширина PNG рамочки: ширина места на экране × dpr, шаг 256, ≤ 4096 — то же правило, что у узора и стикера
// (зум просит новую картинку не на каждый пиксель). Пока тянут ручки места, старая картинка растягивается.
export const borderPx = layerPx

// Адрес накладки и маски: url от Go всегда с «?», px дописывает интерфейс.
export const borderSrc = patternSrc

const pct = (v: number, of: number) => (of > 0 ? `${(v / of) * 100}%` : '0%')
const SAME = 1e-9

// Окно ячейки на экране: фото, форма и значки лежат в нём. Без рамочки окно = место.
export interface CellWindow {
  win: Rect // окно, мм разворота
  shape: string | undefined // форма окна ('oval' — овал)
  radius: number // скругление окна, мм
  windowed: boolean // окно меньше места: .clip ставится позиционно
}

// Окно места r по кадру fv. liveRadius — скругление места с ползунка (ShapeToggle), равное fv.radius, когда
// ползунок не тянут; ins — отступы рамочки места (BorderView.ins). Окно и его скругление присылает Go; пока
// тянут ползунок скругления, скругление окна считается зеркалом InsetBox (паритет inset.json).
// В галерее (preview) кадров из View нет, у «Авто» без рамочки — тоже: окно = место, прежний рисунок.
// insLive — отступы ins идут с ползунка «Толщина»/«Размер» (borderlive): окно от Go устарело, считаем зеркалом InsetBox.
export function windowOf(
  r: Rect,
  fv: FrameView | undefined,
  liveRadius: number,
  ins: readonly number[] | undefined,
  preview = false,
  insLive = false,
): CellWindow {
  if (preview || !fv?.win) return { win: r, shape: preview ? undefined : fv?.shape, radius: liveRadius, windowed: false }
  if (insLive) return liveWindow(r, fv.shape, liveRadius, ins)
  const win = fv.win
  const windowed = !sameRect(win, r)
  const shape = fv.winShape
  if (shape === 'oval') return { win, shape, radius: 0, windowed }
  return { win, shape, radius: winRadius(r, fv, liveRadius, ins), windowed }
}

// Скругление окна: от Go, а пока тянут ползунок скругления места — по зеркалу InsetBox.
function winRadius(r: Rect, fv: FrameView, liveRadius: number, ins: readonly number[] | undefined): number {
  if (liveRadius === (fv.radius ?? 0)) return fv.winRadius ?? 0
  return insetBox(r, outlineOf(fv.shape, liveRadius), ins ?? [0, 0, 0, 0]).outline?.radius ?? 0
}

// Окно целиком по зеркалу InsetBox: отступы ins с ползунка, ответа Go ещё нет.
function liveWindow(r: Rect, shape: string | undefined, radius: number, ins: readonly number[] | undefined): CellWindow {
  const box = insetBox(r, outlineOf(shape, radius), ins ?? [0, 0, 0, 0])
  const oval = !!box.outline?.oval
  return { win: box.win, shape: oval ? 'oval' : undefined, radius: oval ? 0 : (box.outline?.radius ?? 0), windowed: !sameRect(box.win, r) }
}

const sameRect = (a: Rect, b: Rect): boolean =>
  Math.abs(a.x - b.x) < SAME && Math.abs(a.y - b.y) < SAME && Math.abs(a.w - b.w) < SAME && Math.abs(a.h - b.h) < SAME

// Позиция окна внутри ячейки r, проценты. Нет окна — undefined (стили не ставятся, .clip занимает ячейку).
export function winStyle(r: Rect, w: CellWindow): { left: string; top: string; width: string; height: string } | undefined {
  if (!w.windowed) return undefined
  return { left: pct(w.win.x - r.x, r.w), top: pct(w.win.y - r.y, r.h), width: pct(w.win.w, r.w), height: pct(w.win.h, r.h) }
}

// Значение для CSS-переменной маски: url("…") с экранированием кавычек (адрес строит Go, но он попадает в стиль).
export function maskUrlCss(src: string): string | undefined {
  if (!src) return undefined
  return `url("${src.replace(/["\\\n\r]/g, encodeURIComponent)}")`
}

export type Loader = (src: string) => Promise<boolean>

export const loadImage: Loader = (src) =>
  new Promise((done) => {
    const img = new Image()
    img.onload = () => done(true)
    img.onerror = () => done(false)
    img.src = src
  })

// Маска ставится только загруженной: mask-image с адресом, который не открылся (404, диск ушёл), по правилам CSS —
// прозрачная маска, то есть фото исчезло бы. Пока грузится новый размер, держится прежняя (растянута по окну);
// не открылся — set('') и окно прямоугольное. Возвращает отмену для $effect.
export function watchMask(src: string, set: (ready: string) => void, load: Loader = loadImage): () => void {
  if (!src) {
    set('')
    return () => {}
  }
  let live = true
  load(src).then((ok) => {
    if (live) set(ok ? src : '')
  })
  return () => {
    live = false
  }
}
