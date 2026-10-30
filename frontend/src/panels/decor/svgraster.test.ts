import { afterEach, describe, expect, it, vi } from 'vitest'
import { domEnv, mergeImport, rasterSvgs, svgSrc, type RasterEnv } from './svgraster'
import type { app } from '../../../wailsjs/go/models'
import type { DecorItem } from './decor'

const job = (n: number, over: Partial<app.DecorSvg> = {}): app.DecorSvg => ({
  token: `t${n}`,
  name: `s${n}`,
  data: `ZGF0YQ${n}`,
  w: 400,
  h: 300,
  ...over,
})
const item = (id: string): DecorItem => ({ id, theme: 'mine', name: id, url: '', own: true, aspect: 1, w: 1, h: 1 })

// Холст и картинка подменены: в node их нет. Здесь важно, что рисуется заказанный размер и уходит PNG.
const okEnv = (): RasterEnv & { calls: string[] } => {
  const calls: string[] = []
  return {
    calls,
    load: vi.fn(async (src: string) => {
      calls.push(src)
      return {} as CanvasImageSource
    }),
    png: vi.fn(async (_i: CanvasImageSource, w: number, h: number) => `PNG${w}x${h}`),
  }
}
const send = vi.fn(async (token: string) => item(`my:${token}`))

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  send.mockClear()
})

describe('rasterSvgs', () => {
  it('SVG → data:image/svg+xml;base64 → PNG заказанного размера → Go по токену', async () => {
    const env = okEnv()
    const r = await rasterSvgs([job(1)], send, { env })
    expect(env.calls).toEqual([svgSrc('ZGF0YQ1')])
    expect(env.calls[0].startsWith('data:image/svg+xml;base64,')).toBe(true)
    expect(env.png).toHaveBeenCalledWith(expect.anything(), 400, 300)
    expect(send).toHaveBeenCalledWith('t1', 'PNG400x300')
    expect(r.added.map((i) => i.id)).toEqual(['my:t1'])
    expect(r.skipped).toEqual([])
  })

  it('пустой список и null — ничего не зовёт', async () => {
    const env = okEnv()
    expect(await rasterSvgs([], send, { env })).toEqual({ added: [], skipped: [] })
    expect(await rasterSvgs(null, send, { env })).toEqual({ added: [], skipped: [] })
    expect(await rasterSvgs(undefined, send, { env })).toEqual({ added: [], skipped: [] })
    expect(env.load).not.toHaveBeenCalled()
    expect(send).not.toHaveBeenCalled()
  })

  it('по очереди: второе не начинается, пока не закончено первое', async () => {
    const log: string[] = []
    let release!: () => void
    const env: RasterEnv = {
      load: async (src) => {
        log.push(`load ${src.slice(-1)}`)
        if (src.endsWith('1')) await new Promise<void>((r) => (release = r))
        return {} as CanvasImageSource
      },
      png: async () => 'P',
    }
    const p = rasterSvgs([job(1), job(2)], async (t) => (log.push(`send ${t}`), item(t)), { env })
    await Promise.resolve()
    await Promise.resolve()
    expect(log).toEqual(['load 1'])
    release()
    await p
    expect(log).toEqual(['load 1', 'send t1', 'load 2', 'send t2'])
  })

  it('ошибка одного из трёх не роняет остальные', async () => {
    const env = okEnv()
    env.load = vi.fn(async (src: string) => {
      if (src.endsWith('2')) throw new Error('битый')
      return {} as CanvasImageSource
    })
    const r = await rasterSvgs([job(1), job(2), job(3)], send, { env })
    expect(r.added.map((i) => i.id)).toEqual(['my:t1', 'my:t3'])
    expect(r.skipped).toEqual([{ name: 's2', why: 'не удалось прочитать рисунок' }])
  })

  it('отказ Go по токену — пропуск с его текстом, дальше идёт', async () => {
    const env = okEnv()
    const bad = vi.fn(async (t: string) => {
      if (t === 't1') throw 'Error: токен устарел'
      return item(t)
    })
    const r = await rasterSvgs([job(1), job(2)], bad, { env })
    expect(r.skipped).toEqual([{ name: 's1', why: 'токен устарел' }])
    expect(r.added.map((i) => i.id)).toEqual(['t2'])
  })

  it('холст «грязный» (SecurityError) — совет сохранить как PNG', async () => {
    const env = okEnv()
    env.png = vi.fn(async () => {
      throw Object.assign(new Error('tainted'), { name: 'SecurityError' })
    })
    const r = await rasterSvgs([job(1)], send, { env })
    expect(r.skipped[0].why).toContain('сохраните как PNG')
    expect(send).not.toHaveBeenCalled()
  })

  it('пустой PNG и сбой рисования — «не удалось нарисовать»', async () => {
    const env = okEnv()
    env.png = vi.fn(async () => '')
    const r = await rasterSvgs([job(1)], send, { env })
    expect(r.skipped).toEqual([{ name: 's1', why: 'не удалось нарисовать' }])
  })

  it('неверный размер (0, дробный, больше 4096) — пропуск без загрузки', async () => {
    const env = okEnv()
    const r = await rasterSvgs([job(1, { w: 0 }), job(2, { h: 10.5 }), job(3, { w: 4097 }), job(4, { w: 4096, h: 1 })], send, { env })
    expect(r.skipped.map((s) => s.name)).toEqual(['s1', 's2', 's3'])
    expect(r.skipped.every((s) => s.why === 'неверный размер рисунка')).toBe(true)
    expect(r.added).toHaveLength(1) // 4096×1 — предел включительно
    expect(env.load).toHaveBeenCalledTimes(1)
  })

  it('таймаут: зависшее задание пропускается, следующее рисуется', async () => {
    vi.useFakeTimers()
    const env = okEnv()
    env.load = vi.fn(async (src: string) => {
      if (src.endsWith('1')) return new Promise<CanvasImageSource>(() => {}) // не откроется никогда
      return {} as CanvasImageSource
    })
    const p = rasterSvgs([job(1), job(2)], send, { env, timeoutMs: 1000 })
    await vi.advanceTimersByTimeAsync(1000)
    const r = await p
    expect(r.skipped).toEqual([{ name: 's1', why: 'рисунок не открылся за отведённое время' }])
    expect(r.added.map((i) => i.id)).toEqual(['my:t2'])
    expect(vi.getTimerCount()).toBe(0) // таймеры сняты
  })

  it('быстрое задание таймер не оставляет', async () => {
    vi.useFakeTimers()
    const r = await rasterSvgs([job(1)], send, { env: okEnv(), timeoutMs: 1000 })
    expect(r.added).toHaveLength(1)
    expect(vi.getTimerCount()).toBe(0)
  })
})

describe('domEnv: только Image и холст', () => {
  it('load: Image с data-адресом и decode()', async () => {
    const made: { src?: string; decoded: boolean }[] = []
    class FakeImage {
      src = ''
      decoded = false
      constructor() {
        made.push(this)
      }
      async decode() {
        this.decoded = true
      }
    }
    vi.stubGlobal('Image', FakeImage)
    const img = await domEnv.load(svgSrc('QQ=='))
    expect(made).toHaveLength(1)
    expect(made[0].src).toBe('data:image/svg+xml;base64,QQ==')
    expect(made[0].decoded).toBe(true)
    expect(img).toBe(made[0])
  })

  it('load: decode отклонён — отказ', async () => {
    class BadImage {
      src = ''
      decode() {
        return Promise.reject(new Error('bad'))
      }
    }
    vi.stubGlobal('Image', BadImage)
    await expect(domEnv.load('data:image/svg+xml;base64,')).rejects.toThrow()
  })

  it('png: холст W×H, drawImage на весь холст, base64 без префикса', async () => {
    const draw = vi.fn()
    const cv = { width: 0, height: 0, getContext: () => ({ drawImage: draw }), toDataURL: () => 'data:image/png;base64,QUJD' }
    vi.stubGlobal('document', { createElement: (t: string) => (t === 'canvas' ? cv : null) })
    const img = {} as CanvasImageSource
    expect(await domEnv.png(img, 123, 45)).toBe('QUJD')
    expect([cv.width, cv.height]).toEqual([123, 45])
    expect(draw).toHaveBeenCalledWith(img, 0, 0, 123, 45)
  })

  it('png: нет 2d-контекста — ошибка', async () => {
    vi.stubGlobal('document', { createElement: () => ({ getContext: () => null }) })
    await expect(domEnv.png({} as CanvasImageSource, 1, 1)).rejects.toThrow()
  })
})

describe('mergeImport', () => {
  it('складывает добавленное и пропущенное', () => {
    const m = mergeImport(
      { added: [item('a')], skipped: [{ name: 'x', why: 'y' }] },
      { added: [item('b')], skipped: [{ name: 'z', why: 'w' }] },
    )
    expect(m.added.map((i) => i.id)).toEqual(['a', 'b'])
    expect(m.skipped.map((s) => s.name)).toEqual(['x', 'z'])
  })
})
