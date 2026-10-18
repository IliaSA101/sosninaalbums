// Живой кадр фото на холсте (спека 057): пока тянут наклон или масштаб, ячейка рисует кадр из framelive,
// а не из View. Чистая логика под тест; Cell только подставляет результат в стиль.
import type { Frame, Rect, SpreadView } from '../../base/core/types'
import { containImage, photoCss, placeImage, rotateFrame, tiltFrame } from '../../base/geom/frame'
import { outlineOf, type Outline } from '../../base/geom/shape'

// Фото в осях самого фото (мм разворота). «Целиком» (contain) вписано в форму и от масштаба и сдвига
// не зависит — иначе при протяжке оно росло бы как «С обрезкой»; остальные закрывают место (cover × S).
export function liveImage(cell: Rect, aspect: number, cur: Frame, contain: boolean, o?: Outline): Rect {
  return contain ? containImage(cell, aspect, cur.r ?? 0, o) : placeImage(cell, aspect, cur, o)
}

// transform картинки: поворот берётся из живого кадра (`cur.r`), зеркало — из View (протяжка его не меняет).
export const liveTransform = (cur: Frame, fv?: { flipH?: boolean; flipV?: boolean }): string | undefined =>
  photoCss({ r: cur.r, flipH: fv?.flipH, flipV: fv?.flipV })

// До тысячных и без «−0»: так Go округлит кадр при записи (ClampFrame), живой просмотр не прыгнет на отпускании.
const round3 = (v: number) => Math.round(v * 1000) / 1000 || 0

// Живой кадр наклона (057): base — кадр начала сессии, tilt — наклон в градусах (четверти берутся из turn).
// Место, форма и пропорции файла — из View, как в Cell. «Целиком» меняет только R; остальные — правило
// «наклон держит приближение» (tiltFrame, зеркало Go TiltCrop). Нет фото на листе — null.
export function liveTilt(sp: SpreadView, photoId: string, base: Frame, tilt: number): Frame | null {
  const i = sp.photos.indexOf(photoId)
  const fv = sp.frames?.[i]
  const cell = sp.rects[i]
  if (i < 0 || !fv || !cell) return null
  const r = (fv.turn ?? 0) * 90 + tilt
  if (sp.grid && sp.places?.find((p) => p.photo === photoId)?.fit === 'contain') return rotateFrame(base, r)
  const aspect = fv.img.h > 0 ? fv.img.w / fv.img.h : cell.w / cell.h
  const f = tiltFrame(cell, aspect, base, r, outlineOf(fv.shape, fv.radius))
  return { ...f, s: round3(f.s), x: round3(f.x), y: round3(f.y) }
}

// Живой кадр фона: рамка box — прямоугольник w×h, наклон — весь R.
export function liveTiltBox(box: { w: number; h: number }, aspect: number, base: Frame, tilt: number): Frame {
  const f = tiltFrame({ x: 0, y: 0, w: box.w, h: box.h }, aspect, base, tilt)
  return { ...f, s: round3(f.s), x: round3(f.x), y: round3(f.y) }
}

// Сессия жеста наклона (решение 11): пока фокус в строке «Наклон» одного фото/фона, все правки — один шаг отмены
// и расчёт от кадра начала сессии (0 → 3 → 0 возвращает кадр точно). id уходит в Go как session (ключ merge).
// Кончается при уходе фокуса (end), смене объекта (key) и при чужой правке (rev не тот, что после нашей).
let seq = 0
export class TiltSession {
  id = ''
  key = ''
  base: Frame | null = null
  private rev: number | null = null

  // Сессия для key: продолжается, пока объект тот же и проект не менял никто, кроме нас; иначе — новая от frame.
  keep(key: string, frame: Frame, rev: number): { id: string; base: Frame } {
    if (!this.base || this.key !== key || (this.rev !== null && this.rev !== rev)) {
      this.key = key
      this.base = frame
      this.rev = null
      this.id = `tl-${(Date.now() * 1000 + (seq++ % 1000)).toString(36)}`
    }
    return { id: this.id, base: this.base }
  }

  // После нашей правки: rev, который считается «своим».
  settle(rev: number) {
    if (this.base) this.rev = rev
  }

  end() {
    this.base = null
    this.rev = null
    this.id = ''
    this.key = ''
  }
}

// По одному на карточку фото и на панель фона: живут в модуле, пересоздание компонента сессию не рвёт.
export const photoTilt = new TiltSession()
export const backdropTilt = new TiltSession()

// Дуга на фото (059, 063): своя сессия — протяжка дуги и правка в карточке дают разные шаги отмены.
export const canvasTilt = new TiltSession()
