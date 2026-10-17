// Дуга наклона на фото (спеки 059, 063): чистая логика без Svelte. Геометрия дуги, перевод протяжки в угол,
// прилипание, клавиши, подпись; решение «слать правку или нет» принимает DialDrag, сессию отмены (057) ведёт
// DialControl. Кадр считает Go (SetFrameAngle).
import type { Frame } from '../../base/core/types'
import type { ArcRoom } from '../places/minipanel'

export const TILT_MAX = 44.9 // как min/max строки «Наклон, °»
export const TILT_ARC = 10 // ° на дуге (владелец 2026-10-16); больше — карточка
export const ARC_PHI = 25 // ° половины дуги на окружности: дуга — 50° окружности
export const ARC_TOP = 22 // px от верха видимой части фото до вершины (центр кружка при 0°)
export const ARC_K = 0.42 // хорда / ширина видимой части (макет владельца: 227 / 548)
export const ARC_CMIN = 140
export const ARC_CMAX = 260
export const ARC_OUT_GAP = 18 // px от верха рамки до концов дуги, когда она снаружи
export const KNOB = 9 // радиус кружка (⌀ 18, как у ползунка масштаба)
export const SNAP = 0.3 // около 0° значение прилипает к нулю
export const DRAG_MIN = 3 // px: меньше — это щелчок, а не протяжка (как у ✥)

export const sameTilt = (a: number, b: number): boolean => Math.abs(a - b) < 0.05

// До десятых и без «−0»: так угол запишет Go (ClampFrame), дуга после ответа не прыгнет.
const tenth = (v: number): number => Math.round(v * 10) / 10 || 0

// Сдвиг курсора вдоль оси места: дуга повёрнута вместе с местом (cellRot, градусы), протяжка идёт вдоль её оси.
export function axisDelta(dx: number, dy: number, cellRot: number): number {
  const a = (cellRot * Math.PI) / 180
  return dx * Math.cos(a) + dy * Math.sin(a)
}

export interface Mods {
  shift: boolean
  alt: boolean
}

const rad = (deg: number): number => (deg * Math.PI) / 180
const clampTo = (v: number, lim: number): number => Math.min(lim, Math.max(-lim, v))

export interface ArcGeom {
  c: number // хорда, px
  R: number // радиус окружности
  s: number // высота дуги (стрела)
  ax: number // вершина (центр кружка при 0°) в осях рамки
  ay: number
}

// Хорда дуги от ширины видимой части фото.
export const arcChord = (vw: number): number => Math.min(ARC_CMAX, Math.max(ARC_CMIN, ARC_K * vw))

// Дуга в осях места: w × h — рамка, shaped — круг или скругление (нижний угол срезан, дуга ниже на 8 % высоты, как у ползунка).
export function arcGeom(room: ArcRoom, w: number, h: number, shaped = false): ArcGeom {
  const out = room.mode === 'out'
  const c = out ? ARC_CMIN : arcChord(room.vw)
  const R = c / (2 * Math.sin(rad(ARC_PHI)))
  const s = R * (1 - Math.cos(rad(ARC_PHI)))
  if (out) return { c, R, s, ax: w / 2, ay: -ARC_OUT_GAP - s }
  return { c, R, s, ax: room.vx, ay: room.vy + ARC_TOP + (shaped ? 0.08 * h : 0) }
}

// Кружок на дуге; угол за ±10° (задан в карточке) — в конце дуги.
export function knobAt(t: number, g: ArcGeom): { x: number; y: number } {
  const a = (clampTo(t, TILT_ARC) / TILT_ARC) * rad(ARC_PHI)
  return { x: g.ax + g.R * Math.sin(a), y: g.ay + g.R * (1 - Math.cos(a)) }
}

// Угол по x кружка в осях места: кружок держится под курсором по горизонтали, сам стоит на дуге. Вправо — плюс.
// Shift — целые градусы; иначе около нуля прилипание (Alt его снимает); предел дуги и округление до 0,1°.
export function arcAngle(x: number, g: ArcGeom, m: Mods): number {
  const lim = Math.sin(rad(ARC_PHI))
  const a = Math.asin(clampTo((x - g.ax) / g.R, lim))
  let t = (TILT_ARC * a) / rad(ARC_PHI)
  if (m.shift) t = Math.round(t)
  else if (!m.alt && Math.abs(t) < SNAP) t = 0
  return tenth(clampTo(t, TILT_ARC))
}

// ←/→ на кружке в фокусе: ±0,1°, с Shift ±1°; предел lim (на дуге — max(10, |t|): из-за предела только назад).
export function keyAngle(t: number, key: string, shift: boolean, lim = TILT_MAX): number | null {
  if (key !== 'ArrowLeft' && key !== 'ArrowRight') return null
  const step = (shift ? 1 : 0.1) * (key === 'ArrowRight' ? 1 : -1)
  return tenth(clampTo(t + step, lim))
}

// «+3,5°», «−12,0°», «0,0°»: минус U+2212, десятичная запятая.
export function fmtTilt(t: number): string {
  const v = tenth(t)
  if (v === 0) return '0,0°'
  return `${v > 0 ? '+' : '−'}${Math.abs(v).toFixed(1).replace('.', ',')}°`
}

// Перевод сдвига протяжки вдоль оси места в угол: t0 — угол в начале, dpx — сдвиг, px.
export type ToAngle = (t0: number, dpx: number, m: Mods) => number

// Контроллер одной протяжки: Svelte передаёт события, решение о правке — здесь.
export class DialDrag {
  private moved = false
  private cancelled = false
  private last: number

  // jump — сдвиг от кружка до точки нажатия на полосе: протяжка сразу «с ходом» (как щелчок по полосе ползунка).
  constructor(
    private readonly t0: number,
    private readonly conv: ToAngle,
    private readonly jump?: number,
    m: Mods = { shift: false, alt: false },
  ) {
    this.last = t0
    if (jump !== undefined) {
      this.moved = true
      this.last = conv(t0, jump, m)
    }
  }

  // Угол, с которым началась протяжка по полосе; null — нажатие было на кружок.
  get start(): number | null {
    return this.jump === undefined ? null : this.last
  }

  // Живой угол; null — сдвиг меньше DRAG_MIN, протяжки ещё нет (или она отменена).
  move(dpx: number, m: Mods): number | null {
    if (this.cancelled) return null
    if (!this.moved && Math.abs(dpx) < DRAG_MIN) return null
    this.moved = true
    this.last = this.conv(this.t0, dpx + (this.jump ?? 0), m)
    return this.last
  }

  // Угол для правки; null — хода не было, отмена или угол тот же.
  up(): number | null {
    if (!this.moved || this.cancelled || sameTilt(this.last, this.t0)) return null
    return this.last
  }

  cancel(): void {
    this.cancelled = true
  }
}

// Сессия жеста наклона (tiltlive.ts: TiltSession) — ровно то, чем пользуется дуга.
export interface TiltSessionLike {
  readonly id: string
  keep(key: string, frame: Frame, rev: number): { id: string; base: Frame }
  settle(rev: number): void
  end(): void
}

export interface DialDeps {
  session: TiltSessionLike
  key: () => string // объект сессии: фото на развороте
  frame: () => Frame // кадр из Go сейчас
  tilt: () => number // наклон из View, °
  rev: () => number
  toAngle: ToAngle // px → угол по геометрии дуги
  send: (angle: number, session: string) => Promise<unknown> // SetFrameAngle; session '' — без merge
}

// Шаг стрелки: base — кадр начала сессии (основа предпросмотра), commit шлёт правку. base null — на пределе, шага нет.
export interface KeyStep {
  angle: number
  base: Frame | null
  commit: () => Promise<void>
}

// Управление сессией отмены дуги: что и когда уходит в Go. Svelte передаёт события, рисует предпросмотр и фокус.
// Протяжка — свой шаг отмены (вход обрывает серию стрелок); серия стрелок до ухода фокуса — один шаг;
// двойной щелчок и отмена — без сессии (отмена не шлёт ничего).
export class DialControl {
  private dd: DialDrag | null = null
  private s: { id: string; base: Frame } | null = null
  private x = 0
  private y = 0

  constructor(private readonly d: DialDeps) {}

  get dragging(): boolean {
    return this.dd !== null
  }

  // Нажатие на кружок или (jump — сдвиг до точки нажатия) на полосу дуги: новая сессия; null — протяжка уже идёт.
  // Возвращает кадр начала.
  down(x: number, y: number, jump?: number, m?: Mods): Frame | null {
    if (this.dd) return null
    this.d.session.end() // протяжка — свой шаг отмены, не продолжение серии стрелок
    this.s = this.d.session.keep(this.d.key(), this.d.frame(), this.d.rev())
    this.dd = new DialDrag(this.d.tilt(), this.d.toAngle, jump, m)
    this.x = x
    this.y = y
    return this.s.base
  }

  // Угол сразу после нажатия на полосу; null — нажатие было на кружок.
  get startAngle(): number | null {
    return this.dd?.start ?? null
  }

  // Живой угол или null (до порога, отмена, протяжки нет).
  move(x: number, y: number, cellRot: number, m: Mods): number | null {
    if (!this.dd) return null
    return this.dd.move(axisDelta(x - this.x, y - this.y, cellRot), m)
  }

  // Esc и сбой жеста (pointercancel, потеря захвата): записи не будет, сессия закроется на release.
  cancel(): void {
    this.dd?.cancel()
  }

  // Отпускание: угол для правки или null (щелчок, отмена, тот же угол) — тогда сессия закрыта и ничего не уходит.
  release(): number | null {
    const dd = this.dd
    if (!dd) return null
    this.dd = null
    const a = dd.up()
    if (a === null) this.d.session.end()
    return a
  }

  // Правка по результату release: один вызов с id сессии; потом сессия закрывается, если за время ответа её не сменила
  // новая протяжка.
  async commit(angle: number): Promise<void> {
    const s = this.s
    if (!s) return
    await this.d.send(angle, s.id)
    if (this.d.session.id === s.id) this.d.session.end()
  }

  // ←/→ на кружке в фокусе; null — клавиша не наша. На дуге предел ±10°, а угол из карточки больше — только назад.
  key(t: number, key: string, shift: boolean): KeyStep | null {
    const next = keyAngle(t, key, shift, Math.max(TILT_ARC, Math.abs(t)))
    if (next === null) return null
    if (sameTilt(next, t)) return { angle: next, base: null, commit: async () => {} }
    const s = this.d.session.keep(this.d.key(), this.d.frame(), this.d.rev())
    return {
      angle: next,
      base: s.base,
      commit: async () => {
        await this.d.send(next, s.id)
        this.d.session.settle(this.d.rev())
      },
    }
  }

  // Двойной щелчок: 0° одной правкой без сессии; уже 0 — вызова нет.
  async reset(): Promise<boolean> {
    if (sameTilt(this.d.tilt(), 0)) return false
    this.d.session.end()
    await this.d.send(0, '')
    return true
  }

  // Фокус ушёл с кружка: серия стрелок кончилась.
  blur(): void {
    this.d.session.end()
  }

  // Дуга исчезла: протяжка бросается без записи. true — она шла.
  dispose(): boolean {
    const was = this.dd !== null
    this.dd = null
    this.d.session.end()
    return was
  }
}
