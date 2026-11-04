import { describe, expect, it, vi } from 'vitest'
import { loupeAt, LOUPE, pointQueue, sheetPoint } from './eyedropat'

const box = { left: 100, top: 50, width: 300, height: 150 }

describe('sheetPoint: экран → мм листа', () => {
  it('масштаб 0,5 (лист 600 мм в 300 px): 2 мм на пиксель', () => {
    expect(sheetPoint(250, 100, box, 600)).toEqual({ x: 300, y: 100 })
    expect(sheetPoint(100, 50, box, 600)).toEqual({ x: 0, y: 0 })
  })

  it('масштаб 2 (лист 150 мм в 300 px): 0,5 мм на пиксель', () => {
    expect(sheetPoint(250, 100, box, 150)).toEqual({ x: 75, y: 25 })
  })

  it('вне слоя и пустой слой — null; правая и нижняя кромка не входят', () => {
    expect(sheetPoint(99, 100, box, 600)).toBeNull()
    expect(sheetPoint(250, 49, box, 600)).toBeNull()
    expect(sheetPoint(400, 100, box, 600)).toBeNull()
    expect(sheetPoint(250, 200, box, 600)).toBeNull()
    expect(sheetPoint(1, 1, { left: 0, top: 0, width: 0, height: 0 }, 600)).toBeNull()
  })
})

describe('loupeAt: где стоит лупа', () => {
  it('в середине экрана — справа-сверху от курсора', () => {
    expect(loupeAt(500, 400, 1280, 760)).toEqual({ left: 514, top: 340 })
  })

  it('у правого края — слева от курсора', () => {
    const r = loupeAt(1270, 400, 1280, 760)
    expect(r.left).toBe(1270 - 14 - LOUPE)
  })

  it('у верхнего края — под курсором, у нижнего — прижата к низу и целиком в окне', () => {
    expect(loupeAt(500, 20, 1280, 760).top).toBe(34)
    const b = loupeAt(500, 755, 1280, 760)
    expect(b.top + 70).toBeLessThanOrEqual(760)
    expect(b.top).toBeGreaterThanOrEqual(0)
  })

  it('у левого края не уходит за окно', () => {
    expect(loupeAt(0, 400, 1280, 760).left).toBeGreaterThanOrEqual(0)
  })
})

describe('pointQueue: очередь запросов лупы', () => {
  it('один в пути; пока он идёт, берётся только последняя точка', async () => {
    const waits: ((r: string) => void)[] = []
    const ask = vi.fn(() => new Promise<string>((r) => waits.push(r)))
    const done = vi.fn()
    const q = pointQueue(ask, done, vi.fn())
    q.push({ x: 1, y: 1 })
    q.push({ x: 2, y: 2 })
    q.push({ x: 3, y: 3 })
    expect(ask).toHaveBeenCalledTimes(1)
    waits[0]('a')
    await vi.waitFor(() => expect(ask).toHaveBeenCalledTimes(2))
    expect(ask).toHaveBeenLastCalledWith({ x: 3, y: 3 })
    expect(done).toHaveBeenCalledExactlyOnceWith({ x: 1, y: 1 }, 'a')
    waits[1]('b')
    await vi.waitFor(() => expect(done).toHaveBeenCalledTimes(2))
    expect(ask).toHaveBeenCalledTimes(2)
  })

  it('после ответа новая точка уходит сразу', async () => {
    const ask = vi.fn(async () => 'x')
    const q = pointQueue(ask, vi.fn(), vi.fn())
    q.push({ x: 1, y: 1 })
    await vi.waitFor(() => expect(ask).toHaveBeenCalledTimes(1))
    await Promise.resolve()
    q.push({ x: 2, y: 2 })
    expect(ask).toHaveBeenCalledTimes(2)
  })

  it('ошибка уходит в fail, очередь живёт дальше', async () => {
    const ask = vi.fn().mockRejectedValueOnce('плохо').mockResolvedValueOnce('ok')
    const fail = vi.fn()
    const done = vi.fn()
    const q = pointQueue(ask, done, fail)
    q.push({ x: 1, y: 1 })
    await vi.waitFor(() => expect(fail).toHaveBeenCalledExactlyOnceWith('плохо'))
    await Promise.resolve()
    q.push({ x: 2, y: 2 })
    await vi.waitFor(() => expect(done).toHaveBeenCalledExactlyOnceWith({ x: 2, y: 2 }, 'ok'))
  })

  it('stop: ответ в пути отбрасывается, ожидающая точка не уходит, новые не принимаются', async () => {
    const waits: ((r: string) => void)[] = []
    const ask = vi.fn(() => new Promise<string>((r) => waits.push(r)))
    const done = vi.fn()
    const q = pointQueue(ask, done, vi.fn())
    q.push({ x: 1, y: 1 })
    q.push({ x: 2, y: 2 })
    q.stop()
    waits[0]('a')
    await Promise.resolve()
    await Promise.resolve()
    q.push({ x: 3, y: 3 })
    expect(done).not.toHaveBeenCalled()
    expect(ask).toHaveBeenCalledTimes(1)
  })
})
