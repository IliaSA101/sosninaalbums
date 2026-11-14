import { afterEach, describe, expect, it, vi } from 'vitest'
import { EDGE_DELAY, EDGE_VMAX, EdgeDwell, edgeScroll, edgeSpeed, edgeStep, edgeZone, edges, FRAME_MS } from './edgescroll'

// Лента разворотов: 1000×80 px с (100, 600).
const strip = { left: 100, top: 600, width: 1000, height: 80 }
const y = 640

describe('edgescroll: чистая часть', () => {
  it('зона — min(48, 15 %)', () => {
    expect(edgeZone(1000)).toBe(48)
    expect(edgeZone(200)).toBe(30)
    expect(edgeZone(0)).toBe(0)
  })

  it('скорость квадратично до 18 px/кадр', () => {
    // у самого края — полная, на середине зоны — четверть, на границе зоны — ноль
    expect(edgeSpeed(strip, 'x', 1100, y)).toBe(EDGE_VMAX)
    expect(edgeSpeed(strip, 'x', 100, y)).toBe(-EDGE_VMAX)
    expect(edgeSpeed(strip, 'x', 1100 - 24, y)).toBeCloseTo(EDGE_VMAX / 4, 9)
    expect(edgeSpeed(strip, 'x', 100 + 24, y)).toBeCloseTo(-EDGE_VMAX / 4, 9)
    expect(edgeSpeed(strip, 'x', 1100 - 48, y)).toBe(0)
    expect(edgeSpeed(strip, 'x', 600, y)).toBe(0)
  })

  it('за краем — полная, дальше зоны — ноль', () => {
    expect(edgeSpeed(strip, 'x', 1100 + 10, y)).toBe(EDGE_VMAX)
    expect(edgeSpeed(strip, 'x', 100 - 47, y)).toBe(-EDGE_VMAX)
    expect(edgeSpeed(strip, 'x', 1100 + 49, y)).toBe(0)
    expect(edgeSpeed(strip, 'x', 100 - 49, y)).toBe(0)
  })

  it('вне поперечной полосы — ноль', () => {
    expect(edgeSpeed(strip, 'x', 1095, 599)).toBe(0)
    expect(edgeSpeed(strip, 'x', 1095, 681)).toBe(0)
    expect(edgeSpeed(strip, 'x', 1095, 600)).toBeGreaterThan(0)
    // по вертикали (панель деталей) поперёк — x
    const panel = { left: 800, top: 40, width: 400, height: 600 }
    expect(edgeSpeed(panel, 'y', 1000, 40)).toBe(-EDGE_VMAX)
    expect(edgeSpeed(panel, 'y', 799, 40)).toBe(0)
    expect(edgeSpeed(panel, 'y', 1000, 640)).toBe(EDGE_VMAX)
  })

  it('шаг нормирован на время кадра', () => {
    expect(edgeStep(18, FRAME_MS, 100, 1000)).toBeCloseTo(118, 9)
    expect(edgeStep(18, FRAME_MS / 2, 100, 1000)).toBeCloseTo(109, 9)
    expect(edgeStep(-18, 2 * FRAME_MS, 100, 1000)).toBeCloseTo(64, 9)
    // пауза вкладки: не больше 50 мс за шаг
    expect(edgeStep(18, 5000, 100, 1000)).toBeCloseTo(100 + (18 * 50) / FRAME_MS, 9)
  })

  it('у конца прокрутки — стоп', () => {
    expect(edgeStep(18, FRAME_MS, 995, 1000)).toBe(1000)
    expect(edgeStep(-18, FRAME_MS, 5, 1000)).toBe(0)
  })

  it('вход через 150 мс, выход сбрасывает', () => {
    const d = new EdgeDwell()
    expect(d.feed(true, 1000)).toBe(false)
    expect(d.feed(true, 1000 + EDGE_DELAY - 1)).toBe(false)
    expect(d.feed(true, 1000 + EDGE_DELAY)).toBe(true)
    expect(d.feed(false, 1200)).toBe(false)
    // вернулся в зону — отсчёт заново
    expect(d.feed(true, 1300)).toBe(false)
    expect(d.feed(true, 1300 + EDGE_DELAY)).toBe(true)
    d.reset()
    expect(d.feed(true, 2000)).toBe(false)
  })
})

// Скроллер-заглушка: прямоугольник, прокрутка и слушатель scroll.
function fakeNode(box = strip, width = 5000) {
  const ls: Record<string, () => void> = {}
  return {
    scrollLeft: 0,
    scrollTop: 0,
    scrollWidth: width,
    clientWidth: box.width,
    scrollHeight: box.height,
    clientHeight: box.height,
    getBoundingClientRect: () => box,
    closest: () => null,
    addEventListener: (t: string, f: () => void) => (ls[t] = f),
    removeEventListener: (t: string) => delete ls[t],
    fire: (t: string) => ls[t]?.(),
  }
}

describe('edgescroll: цикл кадров', () => {
  let now = 0
  let queued: (() => void) | null = null
  vi.stubGlobal('performance', { now: () => now })
  vi.stubGlobal('requestAnimationFrame', (f: () => void) => {
    queued = f
    return 1
  })
  vi.stubGlobal('cancelAnimationFrame', () => (queued = null))
  const tickTo = (t: number) => {
    now = t
    const f = queued
    queued = null
    f?.()
  }

  afterEach(() => {
    edges.stop()
    queued = null
    now = 0
  })

  it('курсор стоит у края: через 150 мс лента едет, ушёл — стоп', () => {
    const n = fakeNode()
    const act = edgeScroll(n as never, { axis: 'x' })
    edges.feed(1099, y)
    expect(edges.running).toBe(true)
    tickTo(100)
    expect(n.scrollLeft).toBe(0) // задержка
    tickTo(160)
    tickTo(160 + FRAME_MS)
    expect(n.scrollLeft).toBeGreaterThan(0)
    edges.feed(600, y) // ушёл из зоны
    tickTo(200)
    expect(edges.running).toBe(false)
    act.destroy()
  })

  it('быстрый проход через зону ленту не трогает', () => {
    const n = fakeNode()
    const act = edgeScroll(n as never, { axis: 'x' })
    edges.feed(1099, y)
    tickTo(50)
    edges.feed(600, y)
    tickTo(100)
    expect(n.scrollLeft).toBe(0)
    expect(edges.running).toBe(false)
    act.destroy()
  })

  it('у конца прокрутки цикл останавливается', () => {
    const n = fakeNode(strip, 1000) // прокручивать нечего
    const act = edgeScroll(n as never, { axis: 'x' })
    edges.feed(1099, y)
    expect(edges.running).toBe(false)
    act.destroy()
  })

  it('stop гасит цикл; прокрутка скроллера после stop цель не пересчитывает', () => {
    const n = fakeNode()
    const act = edgeScroll(n as never, { axis: 'x' })
    const on = vi.fn()
    edges.onScroll = on
    edges.feed(1099, y)
    n.fire('scroll')
    expect(on).toHaveBeenCalledTimes(1)
    edges.stop()
    expect(edges.running).toBe(false)
    n.fire('scroll')
    expect(on).toHaveBeenCalledTimes(1)
    edges.onScroll = () => {}
    act.destroy()
  })
})
