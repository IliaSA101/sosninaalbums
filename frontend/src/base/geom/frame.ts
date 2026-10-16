// Кадр фото в ячейке (спека 006, design §6; поворот — 010 design-rotation §2.2). Зеркало Go
// (internal/geom/frame.go, geom.PlaceImage): геометрию считает Go, это только предпросмотр сдвига, масштаба и наклона,
// пока не отпущена кнопка. Паритет держат общие векторы testdata/parity/frames.json.
import type { Frame, Rect } from '../core/types'
import type { Outline } from './shape'

export const MIN_SCALE = 1
export const MAX_SCALE = 4
// Запас хода (мм): меньше — кадр «стоит», фото тянется между ячейками как обычно.
export const SLACK_MM = 0.5

export const DEFAULT_FRAME: Frame = { s: 1, x: 0, y: 0 }

const clamp = (v: number, lo: number, hi: number) => (Number.isFinite(v) ? Math.min(hi, Math.max(lo, v)) : 0)

// Поворот фото: нет или 0 — поля нет вовсе (кадры без поворота остаются как были).
const withR = (out: Frame, r: number | undefined): Frame => (r ? { ...out, r } : out)

// Приводит кадр к допустимому: s 1..4 (NaN — 1), x и y −1..1 (NaN — 0). Как зажим в Go.
// Угол r несётся как есть: зажим наклона и четвертей — в Go (clampR), UI на 90 не делит.
export function clampFrame(f: Frame): Frame {
  return withR(
    {
      s: Number.isFinite(f.s) ? clamp(f.s, MIN_SCALE, MAX_SCALE) : 1,
      x: clamp(f.x, -1, 1),
      y: clamp(f.y, -1, 1),
    },
    Number.isFinite(f.r) ? f.r : 0,
  )
}

// Синус и косинус угла в градусах; кратные 90° точные (как sinCosDeg в Go).
export function sinCosDeg(deg: number): [number, number] {
  if (deg % 90 === 0) {
    switch (((Math.round(deg / 90) % 4) + 4) % 4) {
      case 0:
        return [0, 1]
      case 1:
        return [1, 0]
      case 2:
        return [0, -1]
      default:
        return [-1, 0]
    }
  }
  const t = (deg * Math.PI) / 180
  return [Math.sin(t), Math.cos(t)]
}

// Габарит формы w×h в осях фото, повёрнутого на θ (c = |cos θ|, s = |sin θ|); зеркало Outline.bound в Go
// (044 design §2.1). Форма выпукла, поэтому внутри фото она ⇔ внутри её габарит. При θ = 0 у всех форм — w×h.
function boundOf(w: number, h: number, c: number, s: number, o?: Outline): { w: number; h: number } {
  if (o?.oval) {
    const a = w / 2
    const b = h / 2
    return { w: 2 * Math.sqrt(a * a * c * c + b * b * s * s), h: 2 * Math.sqrt(a * a * s * s + b * b * c * c) }
  }
  const rad = o?.radius && o.radius > 0 ? Math.min(o.radius, Math.min(w, h) / 2) : 0
  if (rad > 0) {
    const iw = w - 2 * rad
    const ih = h - 2 * rad
    return { w: iw * c + ih * s + 2 * rad, h: iw * s + ih * c + 2 * rad }
  }
  return { w: w * c + h * s, h: w * s + h * c }
}

// Рамка в осях повёрнутого на r фото: описанный прямоугольник (w, h) и знаковые sin, cos угла.
function boundIn(cell: Rect, r: number, o?: Outline): { w: number; h: number; sn: number; cs: number } {
  const [sn, cs] = sinCosDeg(r)
  const b = boundOf(cell.w, cell.h, Math.abs(cs), Math.abs(sn), o)
  return { ...b, sn, cs }
}

// Фото целиком (мм) для ячейки: cover × S, сдвиг — доля свободного хода. При r ≠ 0 фото повёрнуто
// вокруг своего центра и закрывает рамку (или её форму o — круг, скругление); результат — прямоугольник
// в осях самого фото. Без наклона форма не влияет: габарит любой формы — рамка.
export function placeImage(cell: Rect, aspect: number, f: Frame, o?: Outline): Rect {
  if (f.r) return placeImageRot(cell, aspect, f, f.r, o)
  // Без поворота — прежняя формула дословно: иной порядок сложений меняет последний бит (как и в Go).
  let w = cell.w
  let h = cell.w / aspect
  if (h < cell.h) {
    h = cell.h
    w = cell.h * aspect
  }
  w *= f.s
  h *= f.s
  const fx = w - cell.w
  const fy = h - cell.h
  return { x: cell.x - (fx * (1 + f.x)) / 2, y: cell.y - (fy * (1 + f.y)) / 2, w, h }
}

function placeImageRot(cell: Rect, aspect: number, f: Frame, r: number, o?: Outline): Rect {
  const b = boundIn(cell, r, o)
  let w = b.w
  let h = b.w / aspect
  if (h < b.h) {
    h = b.h
    w = b.h * aspect
  }
  w *= f.s
  h *= f.s
  const fx = w - b.w
  const fy = h - b.h
  // Сдвиг задан в осях фото; на экране он повёрнут на r.
  const ux = (-fx * f.x) / 2
  const uy = (-fy * f.y) / 2
  const ox = ux * b.cs - uy * b.sn
  const oy = ux * b.sn + uy * b.cs
  return { x: cell.x + cell.w / 2 + ox - w / 2, y: cell.y + cell.h / 2 + oy - h / 2, w, h }
}

// Эффективный dpi: px (ширина фото в пикселях) на ширину картинки в дюймах; 0 — размер неизвестен.
export function effectiveDPI(px: number, img: Rect): number {
  if (!(px > 0) || !(img.w > 0)) return 0
  return Math.round(px / (img.w / 25.4))
}

// Есть ли куда двигать кадр: тогда выделенное фото тянется как кадр, а переносится за ✥.
// img — фото в своих осях, r — его угол: ход считается от рамки в осях фото.
export function hasSlack(cell: Rect, img: Rect, r = 0, o?: Outline): boolean {
  const b = r ? boundIn(cell, r, o) : cell
  return img.w - b.w > SLACK_MM || img.h - b.h > SLACK_MM
}

// Сдвиг кадра рукой: dx, dy — смещение курсора в мм в осях рамки. Фото едет за курсором,
// поэтому доля хода уменьшается. При нулевом ходе по оси значение не меняется.
// У повёрнутого фото сдвиг переводится в его оси (−r), ход — рамка в осях фото.
export function panFrame(f: Frame, cell: Rect, img: Rect, dxMM: number, dyMM: number, o?: Outline): Frame {
  let dx = dxMM
  let dy = dyMM
  let bw = cell.w
  let bh = cell.h
  if (f.r) {
    const b = boundIn(cell, f.r, o)
    bw = b.w
    bh = b.h
    dx = dxMM * b.cs + dyMM * b.sn
    dy = -dxMM * b.sn + dyMM * b.cs
  }
  const fx = img.w - bw
  const fy = img.h - bh
  return withR(
    {
      s: f.s,
      x: fx > 1e-9 ? clamp(f.x - (2 * dx) / fx, -1, 1) : f.x,
      y: fy > 1e-9 ? clamp(f.y - (2 * dy) / fy, -1, 1) : f.y,
    },
    f.r,
  )
}

// Новый масштаб (зажат в 1..4), сдвиг — доля хода — и поворот переживают.
export function zoomFrame(f: Frame, s: number): Frame {
  return withR({ s: Number.isFinite(s) ? clamp(s, MIN_SCALE, MAX_SCALE) : f.s, x: f.x, y: f.y }, f.r)
}

// Пропорции фото, как они лежат в раскладке: при нечётном числе четвертей (turn) стороны меняются местами.
// fv.img — в осях самого фото, поэтому его w/h — пропорции файла.
export function actualAspect(fv: { img: Rect; turn?: number }): number {
  const a = fv.img.w / fv.img.h
  return (fv.turn ?? 0) % 2 === 1 ? 1 / a : a
}

// Тот же кадр с другим углом: предпросмотр ползунка наклона (в Go уйдёт SetFrameAngle).
export const rotateFrame = (f: Frame, r: number): Frame => withR({ s: f.s, x: f.x, y: f.y }, Number.isFinite(r) ? r : f.r)

// Кадр не отличается от исходного (s=1, x=y=0): «Сбросить» не нужен. Поворот не кадр — сброс его не трогает.
export const isDefaultFrame = (f: Frame) => f.s === 1 && f.x === 0 && f.y === 0

// Зеркало внутри img (CSS): геометрия кадра не меняется, отражается только содержимое.
export function flipCss(f?: { flipH?: boolean; flipV?: boolean }): string | undefined {
  if (!f?.flipH && !f?.flipV) return undefined
  return `scale(${f.flipH ? -1 : 1}, ${f.flipV ? -1 : 1})`
}

// transform картинки в рамке: зеркало в осях фото, затем поворот (так же рисует экспорт);
// вокруг центра самой картинки (transform-origin по умолчанию).
export function photoCss(f?: { r?: number; flipH?: boolean; flipV?: boolean }): string | undefined {
  const flip = flipCss(f)
  if (!f?.r) return flip
  return flip ? `rotate(${f.r}deg) ${flip}` : `rotate(${f.r}deg)`
}

// Положение повёрнутого фото на миниатюре ленты (в процентах ячейки): фото целиком (s=1, без сдвига).
// aspect — пропорции файла (fv.img.w / fv.img.h).
export function thumbBox(cell: Rect, aspect: number, r: number, o?: Outline): string {
  const img = placeImage(cell, aspect, { s: 1, x: 0, y: 0, r }, o)
  const pct = (v: number, of: number) => `${(v / of) * 100}%`
  return `left:${pct(img.x - cell.x, cell.w)};top:${pct(img.y - cell.y, cell.h)};width:${pct(img.w, cell.w)};height:${pct(img.h, cell.h)}`
}

// Во сколько раз сжать повёрнутое на tilt° фото (пропорции aspect), чтобы оно влезло в свой прежний
// прямоугольник — крупный просмотр. Без наклона — 1.
export function fitTilt(tilt: number, aspect: number): number {
  const [sn, cs] = sinCosDeg(tilt)
  return 1 / (Math.abs(cs) + Math.abs(sn) * Math.max(aspect, 1 / aspect))
}
