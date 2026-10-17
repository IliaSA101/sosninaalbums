import { describe, expect, it, vi } from 'vitest'
import {
  ARC_CMAX,
  ARC_CMIN,
  ARC_OUT_GAP,
  ARC_PHI,
  ARC_TOP,
  arcAngle,
  arcChord,
  arcGeom,
  axisDelta,
  DialControl,
  DialDrag,
  fmtTilt,
  keyAngle,
  knobAt,
  sameTilt,
  TILT_ARC,
  TILT_MAX,
} from './tiltdial'
import { TiltSession } from './tiltlive'
import type { Frame } from '../../base/core/types'
import type { ArcRoom } from '../places/minipanel'

const no = { shift: false, alt: false }
const room: ArcRoom = { mode: 'in', vx: 200, vy: 0, vw: 400 }
const g = arcGeom(room, 400, 300)
// Простой перевод для тестов DialDrag / DialControl: 1 px = 0,1°, влево — плюс.
const lin = (t0: number, dpx: number) => Math.round((t0 - dpx / 10) * 10) / 10

describe('tiltdial', () => {
  it('arc: chord, radius, sagitta from width', () => {
    expect(arcChord(100)).toBe(ARC_CMIN)
    expect(arcChord(1000)).toBe(ARC_CMAX)
    expect(arcChord(548)).toBeCloseTo(230.2, 1) // макет владельца: 0,42 ширины
    expect(g.c).toBeCloseTo(168)
    expect(g.R).toBeCloseTo(168 / (2 * Math.sin((ARC_PHI * Math.PI) / 180)))
    expect(g.s / g.c).toBeCloseTo(0.111, 3) // высота ≈ 0,11 хорды
    expect(g.ax).toBe(200)
    expect(g.ay).toBe(ARC_TOP)
  })

  it('arc: out — chord 140 above the frame, ends 18 px over it', () => {
    const o = arcGeom({ mode: 'out', vx: 0, vy: 0, vw: 0 }, 90, 60)
    expect(o.c).toBe(ARC_CMIN)
    expect(o.ax).toBe(45)
    expect(o.ay + o.s).toBeCloseTo(-ARC_OUT_GAP) // концы дуги выше рамки на 18 px
    expect(knobAt(TILT_ARC, o).y).toBeCloseTo(-ARC_OUT_GAP)
  })

  it('arc: shaped shifts the arc down by 8% of height', () => {
    expect(arcGeom(room, 400, 300, true).ay).toBeCloseTo(ARC_TOP + 24)
  })

  it('arc: circle — all knob points on one radius (владелец 2026-10-16)', () => {
    const cx = g.ax
    const cy = g.ay + g.R // центр окружности под вершиной
    for (let t = -10; t <= 10; t += 2.5) {
      const k = knobAt(t, g)
      expect(Math.hypot(k.x - cx, k.y - cy)).toBeCloseTo(g.R, 6)
    }
  })

  it('arc: knob in the middle at 0, ends at ±10 (владелец 2026-10-16)', () => {
    expect(knobAt(0, g)).toEqual({ x: g.ax, y: g.ay })
    const r = knobAt(TILT_ARC, g)
    const l = knobAt(-TILT_ARC, g)
    expect(r.x - g.ax).toBeCloseTo(g.c / 2) // конец дуги — на полухорде
    expect(g.ax - l.x).toBeCloseTo(g.c / 2)
    expect(r.y).toBeCloseTo(g.ay + g.s)
    expect(l.y).toBeCloseTo(r.y)
    expect(r.x).toBeGreaterThan(knobAt(5, g).x) // вправо — плюс
  })

  it('arc: beyond 10 — knob at the end, label exact (владелец 2026-10-16)', () => {
    expect(knobAt(30, g)).toEqual(knobAt(TILT_ARC, g))
    expect(knobAt(-44.9, g)).toEqual(knobAt(-TILT_ARC, g))
    expect(fmtTilt(30)).toBe('+30,0°')
    expect(fmtTilt(-44.9)).toBe('−44,9°')
  })

  it('arc: drag right — plus, knob under pointer', () => {
    const t0 = 2
    const x = knobAt(t0, g).x + 20
    const a = arcAngle(x, g, no)
    expect(a).toBeGreaterThan(t0)
    expect(Math.abs(knobAt(a, g).x - x)).toBeLessThan(1) // с округлением до 0,1°
    expect(arcAngle(knobAt(t0, g).x - 20, g, no)).toBeLessThan(t0)
  })

  it('arc: limit ±10 on the arc and round (владелец 2026-10-16)', () => {
    expect(arcAngle(g.ax + 1000, g, no)).toBe(TILT_ARC)
    expect(arcAngle(g.ax - 1000, g, no)).toBe(-TILT_ARC)
    expect(arcAngle(g.ax + g.c / 2, g, no)).toBe(TILT_ARC)
    expect(arcAngle(knobAt(3.33, g).x, g, no)).toBe(3.3)
    expect(arcAngle(knobAt(-6.78, g).x, g, no)).toBe(-6.8)
  })

  it('arc: from an angle beyond 10 — the angle comes from the arc', () => {
    // из 25° (карточка): кружок в конце, первый же ход даёт угол с дуги, не больше 10
    expect(arcAngle(knobAt(25, g).x + 5, g, no)).toBe(TILT_ARC)
    expect(arcAngle(knobAt(25, g).x - 60, g, no)).toBeLessThan(TILT_ARC)
  })

  it('snap to zero and alt', () => {
    const near = (t: number) => knobAt(t, g).x
    expect(arcAngle(near(0.2), g, no)).toBe(0) // прилипло
    expect(arcAngle(near(-0.2), g, no)).toBe(0)
    expect(Object.is(arcAngle(near(0.2), g, no), -0)).toBe(false) // не «−0»
    expect(arcAngle(near(0.4), g, no)).toBe(0.4) // дальше порога не прилипает
    expect(arcAngle(near(0.2), g, { shift: false, alt: true })).toBe(0.2) // Alt снимает прилипание
    expect(arcAngle(near(-0.2), g, { shift: false, alt: true })).toBe(-0.2)
  })

  it('shift whole degrees', () => {
    const sh = { shift: true, alt: false }
    expect(arcAngle(knobAt(3.4, g).x, g, sh)).toBe(3)
    expect(arcAngle(knobAt(3.6, g).x, g, sh)).toBe(4)
    expect(arcAngle(knobAt(0.4, g).x, g, sh)).toBe(0)
    expect(arcAngle(g.ax + 1000, g, sh)).toBe(TILT_ARC)
  })

  it('axis of rotated place', () => {
    expect(axisDelta(10, 4, 0)).toBeCloseTo(10)
    expect(axisDelta(10, 4, 90)).toBeCloseTo(4)
    expect(axisDelta(10, 4, 180)).toBeCloseTo(-10)
    expect(axisDelta(10, 0, 30)).toBeCloseTo(10 * Math.cos(Math.PI / 6))
    expect(axisDelta(0, 10, 30)).toBeCloseTo(5)
  })

  it('arrow keys (владелец 2026-10-16)', () => {
    expect(keyAngle(1, 'ArrowRight', false)).toBe(1.1)
    expect(keyAngle(1, 'ArrowLeft', false)).toBe(0.9)
    expect(keyAngle(1, 'ArrowRight', true)).toBe(2)
    expect(keyAngle(1, 'ArrowLeft', true)).toBe(0)
    expect(keyAngle(0.3, 'ArrowLeft', false)).toBe(0.2) // без накопления ошибки float
    expect(keyAngle(44.9, 'ArrowRight', false)).toBe(44.9)
    expect(keyAngle(-44.5, 'ArrowLeft', true)).toBe(-44.9)
    expect(keyAngle(0, 'ArrowUp', false)).toBeNull()
    expect(keyAngle(0, 'Tab', false)).toBeNull()
    // предел дуги: на ±10° дальше не уйти
    expect(keyAngle(9.95, 'ArrowRight', true, TILT_ARC)).toBe(TILT_ARC)
    expect(keyAngle(-9.5, 'ArrowLeft', true, TILT_ARC)).toBe(-TILT_ARC)
  })

  it('format', () => {
    expect(fmtTilt(-3.5)).toBe('−3,5°')
    expect(fmtTilt(0)).toBe('0,0°')
    expect(fmtTilt(-0.04)).toBe('0,0°')
    expect(fmtTilt(12)).toBe('+12,0°')
    expect(fmtTilt(44.9)).toBe('+44,9°')
    expect(sameTilt(1, 1.04)).toBe(true)
    expect(sameTilt(1, 1.1)).toBe(false)
  })

  it('drag: one up — one commit', () => {
    const d = new DialDrag(0, lin)
    expect(d.move(1, no)).toBeNull() // меньше 3 px — ещё не протяжка
    expect(d.move(-20, no)).toBe(2)
    expect(d.move(-45, no)).toBe(4.5)
    expect(d.up()).toBe(4.5)
  })

  it('drag: no move or same angle — none', () => {
    const click = new DialDrag(2, lin)
    expect(click.move(2, no)).toBeNull()
    expect(click.up()).toBeNull() // щелчок без хода
    const back = new DialDrag(2, lin)
    expect(back.move(-20, no)).toBe(4)
    expect(back.move(0, no)).toBe(2)
    expect(back.up()).toBeNull() // вернулись к исходному — правки нет
    expect(new DialDrag(0, lin).up()).toBeNull()
  })

  it('drag: cancel — none', () => {
    const d = new DialDrag(0, lin)
    d.move(-30, no)
    d.cancel()
    expect(d.move(-60, no)).toBeNull()
    expect(d.up()).toBeNull()
  })
})

// Сессия отмены дуги (057/063): какие вызовы SetFrameAngle и с каким id уходят.
describe('DialControl: сессия отмены', () => {
  const frame = { s: 1, x: 0, y: 0 } as Frame
  const make = (tilt = 0) => {
    const state = { tilt, rev: 1 }
    const calls: [number, string][] = []
    const send = vi.fn(async (a: number, id: string) => {
      calls.push([a, id])
      state.tilt = a
      state.rev++ // Go ответил новой ревизией
    })
    const session = new TiltSession()
    const ctl = new DialControl({
      session,
      key: () => 'p',
      frame: () => frame,
      tilt: () => state.tilt,
      rev: () => state.rev,
      toAngle: lin,
      send,
    })
    // протяжка на dx px влево (+dx/10 градусов), одним жестом
    const drag = async (dx: number) => {
      ctl.down(100, 0)
      ctl.move(100 - dx, 0, 0, no)
      const a = ctl.release()
      if (a !== null) await ctl.commit(a)
      return a
    }
    return { ctl, calls, send, session, state, drag }
  }

  it('две протяжки подряд — два разных шага (разные id) (владелец 2026-10-16)', async () => {
    const { calls, drag } = make()
    expect(await drag(20)).toBe(2)
    expect(await drag(30)).toBe(5)
    expect(calls.map((c) => c[0])).toEqual([2, 5])
    expect(calls[0][1]).not.toBe('')
    expect(calls[1][1]).not.toBe(calls[0][1])
  })

  it('серия стрелок — один шаг (один id), пока фокус на шкале', async () => {
    const { ctl, calls, state } = make(1)
    for (let i = 0; i < 3; i++) {
      const k = ctl.key(state.tilt, 'ArrowRight', false)!
      expect(k.base).toEqual(frame)
      await k.commit()
    }
    expect(calls.map((c) => c[0])).toEqual([1.1, 1.2, 1.3])
    expect(new Set(calls.map((c) => c[1])).size).toBe(1)
    expect(calls[0][1]).not.toBe('')
    ctl.blur() // фокус ушёл — серия кончилась
    const next = ctl.key(state.tilt, 'ArrowRight', false)!
    await next.commit()
    expect(calls[3][1]).not.toBe(calls[0][1])
  })

  it('чужая правка между стрелками рвёт серию', async () => {
    const { ctl, calls, state } = make(1)
    await ctl.key(state.tilt, 'ArrowRight', false)!.commit()
    state.rev++ // Ctrl+Z или другая правка
    await ctl.key(state.tilt, 'ArrowRight', false)!.commit()
    expect(calls[1][1]).not.toBe(calls[0][1])
  })

  it('протяжка после стрелок — новый шаг', async () => {
    const { ctl, calls, state, drag } = make(1)
    await ctl.key(state.tilt, 'ArrowRight', false)!.commit()
    await drag(20)
    expect(calls[1][1]).not.toBe(calls[0][1])
  })

  it('стрелка на пределе и чужая клавиша — вызова нет', async () => {
    const { ctl, send, state } = make(TILT_MAX)
    const k = ctl.key(state.tilt, 'ArrowRight', false)!
    expect(k.base).toBeNull()
    await k.commit()
    expect(ctl.key(0, 'ArrowUp', false)).toBeNull()
    expect(send).not.toHaveBeenCalled()
  })

  it('двойной щелчок — одна правка без сессии; на нуле — без вызова (владелец 2026-10-16)', async () => {
    const { ctl, calls } = make(3)
    expect(await ctl.reset()).toBe(true)
    expect(calls).toEqual([[0, '']])
    expect(await ctl.reset()).toBe(false)
    expect(calls).toHaveLength(1)
  })

  it('cancel — ни одного вызова, сессия закрыта; следующая протяжка работает', async () => {
    const { ctl, calls, session, drag } = make()
    ctl.down(100, 0)
    ctl.move(60, 0, 0, no)
    ctl.cancel()
    expect(ctl.move(0, 0, 0, no)).toBeNull()
    expect(ctl.release()).toBeNull()
    expect(calls).toHaveLength(0)
    expect(session.base).toBeNull()
    await drag(20)
    expect(calls).toHaveLength(1)
  })

  it('щелчок без хода и возврат к исходному — без вызова', async () => {
    const { ctl, send } = make(2)
    ctl.down(0, 0)
    expect(ctl.release()).toBeNull()
    ctl.down(100, 0)
    ctl.move(80, 0, 0, no)
    ctl.move(100, 0, 0, no)
    expect(ctl.release()).toBeNull()
    expect(send).not.toHaveBeenCalled()
  })

  it('вторая протяжка не стартует поверх первой; поздний ответ не закрывает новую сессию', async () => {
    const { ctl, session, send } = make()
    expect(ctl.down(0, 0)).toEqual(frame)
    expect(ctl.down(0, 0)).toBeNull()
    ctl.move(-30, 0, 0, no)
    const a = ctl.release()!
    let ok!: () => void
    send.mockImplementationOnce(() => new Promise<void>((r) => (ok = r)))
    const slow = ctl.commit(a)
    ctl.down(0, 0) // новая протяжка, пока ответ на прежнюю не пришёл
    const fresh = session.id
    ok()
    await slow
    expect(session.id).toBe(fresh)
  })

  it('нажатие на полосу — кружок туда, одна правка без хода', async () => {
    const { ctl, calls } = make()
    ctl.down(0, 0, -30) // точка нажатия на 30 px правее кружка в «lin»-осях → +3°
    expect(ctl.startAngle).toBe(3)
    const a = ctl.release() // отпустили без движения — всё равно правка
    expect(a).toBe(3)
    await ctl.commit(a!)
    expect(calls).toHaveLength(1)
    expect(calls[0][1]).not.toBe('')
  })

  it('нажатие на полосу, а дальше протяжка — одна правка по итоговой точке', async () => {
    const { ctl, calls } = make()
    ctl.down(100, 0, -30)
    ctl.move(90, 0, 0, no) // ещё +1°
    const a = ctl.release()
    expect(a).toBe(4)
    await ctl.commit(a!)
    expect(calls.map((c) => c[0])).toEqual([4])
  })

  it('нажатие на полосу в точке кружка — правки нет; на кружок — startAngle пуст', () => {
    const { ctl, send } = make(2)
    ctl.down(0, 0, 0)
    expect(ctl.release()).toBeNull()
    ctl.down(0, 0)
    expect(ctl.startAngle).toBeNull()
    ctl.release()
    expect(send).not.toHaveBeenCalled()
  })

  it('стрелки на дуге: из угла больше 10° — только назад', () => {
    const { ctl } = make(12)
    expect(ctl.key(12, 'ArrowRight', false)!.base).toBeNull() // дальше от дуги — нет
    const back = ctl.key(12, 'ArrowLeft', false)!
    expect(back.angle).toBe(11.9)
    expect(back.base).not.toBeNull()
  })

  it('dispose: протяжка брошена без записи', () => {
    const { ctl, send } = make()
    ctl.down(0, 0)
    ctl.move(-30, 0, 0, no)
    expect(ctl.dispose()).toBe(true)
    expect(ctl.dragging).toBe(false)
    expect(ctl.dispose()).toBe(false)
    expect(send).not.toHaveBeenCalled()
  })
})
