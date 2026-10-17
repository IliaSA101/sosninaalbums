// Шкала наклона на фото (спека 059): чистая логика без Svelte. Пиксели линейки → градусы, прилипание,
// клавиши, подпись и деления; решение «слать правку или нет» принимает DialDrag, сессию отмены (057) ведёт DialControl.
// Кадр считает Go (SetFrameAngle).
import type { Frame } from '../../base/core/types'

export const DIAL_PPD = 10 // px на градус: 1 px = 0,1°
export const TILT_MAX = 44.9 // как min/max строки «Наклон, °»
export const SNAP = 0.3 // около 0° значение прилипает к нулю
export const DRAG_MIN = 3 // px: меньше — это щелчок, а не протяжка (как у ✥)

export const sameTilt = (a: number, b: number): boolean => Math.abs(a - b) < 0.05

// До десятых и без «−0»: так угол запишет Go (ClampFrame), шкала после ответа не прыгнет.
const tenth = (v: number): number => Math.round(v * 10) / 10 || 0
const clamp = (v: number): number => Math.min(TILT_MAX, Math.max(-TILT_MAX, v))

// Сдвиг курсора вдоль оси шкалы: шкала повёрнута вместе с местом (cellRot, градусы), протяжка идёт вдоль неё.
export function axisDelta(dx: number, dy: number, cellRot: number): number {
  const a = (cellRot * Math.PI) / 180
  return dx * Math.cos(a) + dy * Math.sin(a)
}

// Угол при протяжке: t0 — угол в начале, dpx — сдвиг по оси шкалы. Вправо — к минусу (линейка едет под стрелкой).
// Shift — целые градусы; иначе около нуля прилипание (Alt его снимает); предел и округление до 0,1°.
export function dialAngle(t0: number, dpx: number, m: { shift: boolean; alt: boolean }): number {
  let t = t0 - dpx / DIAL_PPD
  if (m.shift) t = Math.round(t)
  else if (!m.alt && Math.abs(t) < SNAP) t = 0
  return tenth(clamp(t))
}

// ←/→ на шкале в фокусе: ±0,1°, с Shift ±1°. Прочие клавиши — null.
export function keyAngle(t: number, key: string, shift: boolean): number | null {
  if (key !== 'ArrowLeft' && key !== 'ArrowRight') return null
  const step = (shift ? 1 : 0.1) * (key === 'ArrowRight' ? 1 : -1)
  return tenth(clamp(t + step))
}

// «+3,5°», «−12,0°», «0,0°»: минус U+2212, десятичная запятая.
export function fmtTilt(t: number): string {
  const v = tenth(t)
  if (v === 0) return '0,0°'
  return `${v > 0 ? '+' : '−'}${Math.abs(v).toFixed(1).replace('.', ',')}°`
}

export interface Tick {
  x: number // px от левого края шкалы
  big: boolean // кратно 5
  label?: string // кратно 10
}

// Деления в окне шириной width: на каждый градус; стрелка стоит в середине, значение t — под ней.
// Линейка едет вместе с t: больший угол — левее.
export function dialTicks(t: number, width: number): Tick[] {
  const half = width / 2 / DIAL_PPD
  const out: Tick[] = []
  for (let n = Math.ceil(t - half); n <= Math.floor(t + half); n++) {
    if (Math.abs(n) > Math.floor(TILT_MAX)) continue
    const tick: Tick = { x: width / 2 + (n - t) * DIAL_PPD, big: n % 5 === 0 }
    if (n % 10 === 0) tick.label = n === 0 ? '0' : n < 0 ? `−${-n}` : String(n)
    out.push(tick)
  }
  return out
}

// Контроллер одной протяжки: Svelte передаёт события, решение о правке — здесь.
export class DialDrag {
  private moved = false
  private cancelled = false
  private last: number

  constructor(private readonly t0: number) {
    this.last = t0
  }

  // Живой угол; null — сдвиг меньше DRAG_MIN, протяжки ещё нет (или она отменена).
  move(dpx: number, m: { shift: boolean; alt: boolean }): number | null {
    if (this.cancelled) return null
    if (!this.moved && Math.abs(dpx) < DRAG_MIN) return null
    this.moved = true
    this.last = dialAngle(this.t0, dpx, m)
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

// Сессия жеста наклона (tiltlive.ts: TiltSession) — ровно то, чем пользуется шкала.
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
  send: (angle: number, session: string) => Promise<unknown> // SetFrameAngle; session '' — без merge
}

// Шаг стрелки: base — кадр начала сессии (основа предпросмотра), commit шлёт правку. base null — на пределе, шага нет.
export interface KeyStep {
  angle: number
  base: Frame | null
  commit: () => Promise<void>
}

// Управление сессией отмены шкалы: что и когда уходит в Go. Svelte передаёт события, рисует предпросмотр и фокус.
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

  // Нажатие на линейку: новая сессия; false — протяжка уже идёт. Возвращает кадр начала.
  down(x: number, y: number): Frame | null {
    if (this.dd) return null
    this.d.session.end() // протяжка — свой шаг отмены, не продолжение серии стрелок
    this.s = this.d.session.keep(this.d.key(), this.d.frame(), this.d.rev())
    this.dd = new DialDrag(this.d.tilt())
    this.x = x
    this.y = y
    return this.s.base
  }

  // Живой угол или null (до порога, отмена, протяжки нет).
  move(x: number, y: number, cellRot: number, m: { shift: boolean; alt: boolean }): number | null {
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

  // ←/→ на шкале в фокусе; null — клавиша не наша.
  key(t: number, key: string, shift: boolean): KeyStep | null {
    const next = keyAngle(t, key, shift)
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

  // Фокус ушёл со шкалы: серия стрелок кончилась.
  blur(): void {
    this.d.session.end()
  }

  // Шкала исчезла: протяжка бросается без записи. true — она шла.
  dispose(): boolean {
    const was = this.dd !== null
    this.dd = null
    this.d.session.end()
    return was
  }
}
