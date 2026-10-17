import { describe, expect, it, vi } from 'vitest'
import { axisDelta, DialControl, DialDrag, dialAngle, dialTicks, fmtTilt, keyAngle, sameTilt, TILT_MAX } from './tiltdial'
import { TiltSession } from './tiltlive'
import type { Frame } from '../../base/core/types'

const no = { shift: false, alt: false }

describe('tiltdial', () => {
  it('px to degrees and direction', () => {
    // 1 px = 0,1°; вправо — к минусу
    expect(dialAngle(0, 50, no)).toBe(-5)
    expect(dialAngle(0, -50, no)).toBe(5)
    expect(dialAngle(3, 12, no)).toBe(1.8)
    expect(dialAngle(3, 0, no)).toBe(3)
  })

  it('snap to zero and alt', () => {
    expect(dialAngle(0, -2, no)).toBe(0) // 0,2° — прилипло
    expect(dialAngle(0, 2, no)).toBe(0)
    expect(Object.is(dialAngle(0, 2, no), -0)).toBe(false) // не «−0»
    expect(dialAngle(0, -3, no)).toBe(0.3) // граница 0,3 — уже не прилипает
    expect(dialAngle(0, -2, { shift: false, alt: true })).toBe(0.2) // Alt снимает прилипание
    expect(dialAngle(0, 2, { shift: false, alt: true })).toBe(-0.2)
  })

  it('shift whole degrees', () => {
    expect(dialAngle(0, -34, { shift: true, alt: false })).toBe(3)
    expect(dialAngle(0, -36, { shift: true, alt: false })).toBe(4)
    expect(dialAngle(0.4, 0, { shift: true, alt: false })).toBe(0)
    expect(dialAngle(2.3, 0, { shift: true, alt: false })).toBe(2)
  })

  it('clamp and round', () => {
    expect(dialAngle(0, -1000, no)).toBe(TILT_MAX)
    expect(dialAngle(0, 1000, no)).toBe(-TILT_MAX)
    expect(dialAngle(0, -1000, { shift: true, alt: false })).toBe(TILT_MAX) // 100 → предел
    expect(dialAngle(0, -12.34, no)).toBe(1.2)
    expect(dialAngle(0, -12.36, no)).toBe(1.2)
    expect(dialAngle(10.04, 0, no)).toBe(10)
  })

  it('axis of rotated place', () => {
    expect(axisDelta(10, 4, 0)).toBeCloseTo(10)
    expect(axisDelta(10, 4, 90)).toBeCloseTo(4)
    expect(axisDelta(10, 4, 180)).toBeCloseTo(-10)
    expect(axisDelta(10, 0, 30)).toBeCloseTo(10 * Math.cos(Math.PI / 6))
    expect(axisDelta(0, 10, 30)).toBeCloseTo(5)
  })

  it('arrow keys', () => {
    expect(keyAngle(1, 'ArrowRight', false)).toBe(1.1)
    expect(keyAngle(1, 'ArrowLeft', false)).toBe(0.9)
    expect(keyAngle(1, 'ArrowRight', true)).toBe(2)
    expect(keyAngle(1, 'ArrowLeft', true)).toBe(0)
    expect(keyAngle(0.3, 'ArrowLeft', false)).toBe(0.2) // без накопления ошибки float
    expect(keyAngle(44.9, 'ArrowRight', false)).toBe(44.9)
    expect(keyAngle(-44.5, 'ArrowLeft', true)).toBe(-44.9)
    expect(keyAngle(0, 'ArrowUp', false)).toBeNull()
    expect(keyAngle(0, 'Tab', false)).toBeNull()
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

  it('ticks in window', () => {
    const t = dialTicks(0, 140)
    const xs = t.map((k) => k.x)
    expect(xs).toContain(70) // нулевое деление под стрелкой
    expect(t.filter((k) => k.label).map((k) => k.label)).toEqual(['0'])
    expect(t.find((k) => k.x === 70)?.big).toBe(true)
    expect(t.every((k) => k.x >= 0 && k.x <= 140)).toBe(true)
    // линейка едет вместе с углом: больший угол — деление «0» левее
    const a = dialTicks(3, 140).find((k) => k.label === '0')!
    expect(a.x).toBe(70 - 30)
    // кратные 10 подписаны, минус — U+2212; за пределом диапазона делений нет
    expect(
      dialTicks(-10, 400)
        .filter((k) => k.label)
        .map((k) => k.label),
    ).toEqual(['−30', '−20', '−10', '0', '10'])
    expect(dialTicks(44, 400).some((k) => k.label === '50')).toBe(false)
    expect(Math.max(...dialTicks(44, 400).map((k) => k.x))).toBeLessThan(400)
    // крупные деления — кратные 5: −5, 0, 5
    expect(
      dialTicks(0, 100)
        .filter((k) => k.big)
        .map((k) => k.x),
    ).toEqual([0, 50, 100])
  })

  it('drag: one up — one commit', () => {
    const d = new DialDrag(0)
    expect(d.move(1, no)).toBeNull() // меньше 3 px — ещё не протяжка
    expect(d.move(-20, no)).toBe(2)
    expect(d.move(-45, no)).toBe(4.5)
    expect(d.up()).toBe(4.5)
  })

  it('drag: no move or same angle — none', () => {
    const click = new DialDrag(2)
    expect(click.move(2, no)).toBeNull()
    expect(click.up()).toBeNull() // щелчок без хода
    const back = new DialDrag(2)
    expect(back.move(-20, no)).toBe(4)
    expect(back.move(0, no)).toBe(2)
    expect(back.up()).toBeNull() // вернулись к исходному — правки нет
    expect(new DialDrag(0).up()).toBeNull()
  })

  it('drag: cancel — none', () => {
    const d = new DialDrag(0)
    d.move(-30, no)
    d.cancel()
    expect(d.move(-60, no)).toBeNull()
    expect(d.up()).toBeNull()
  })
})

// Сессия отмены шкалы (057/059): какие вызовы SetFrameAngle и с каким id уходят.
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
    const ctl = new DialControl({ session, key: () => 'p', frame: () => frame, tilt: () => state.tilt, rev: () => state.rev, send })
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

  it('две протяжки подряд — два разных шага (разные id)', async () => {
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

  it('двойной щелчок — одна правка без сессии; на нуле — без вызова', async () => {
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
