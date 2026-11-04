import { describe, expect, it } from 'vitest'
import { borderPx, borderSrc, maskUrlCss, watchMask } from './border'

describe('borderPx / borderSrc', () => {
  it('ширина места на экране × dpr, шаг 256', () => {
    // место 100 из 400 мм листа на экране 1600 px → 400 px, dpr 1 → 512
    expect(borderPx(100, 400, 1600, 1)).toBe(512)
    expect(borderPx(100, 400, 1600, 2)).toBe(1024)
  })
  it('зум крупнее — запрос крупнее; предел 4096', () => {
    expect(borderPx(400, 400, 6000, 2)).toBe(4096)
  })
  it('нет ширины листа — минимальный шаг', () => {
    expect(borderPx(100, 0, 1600, 1)).toBe(256)
    expect(borderPx(100, 400, 0, 1)).toBe(256)
  })
  it('px дописывается к адресу Go: через & после ? и через ? без него', () => {
    expect(borderSrc('/media/f/p1?w=100&h=150', 512)).toBe('/media/f/p1?w=100&h=150&px=512')
    expect(borderSrc('/media/gen/frame', 256)).toBe('/media/gen/frame?px=256')
  })
})

describe('maskUrlCss', () => {
  it('нет адреса — нет значения', () => expect(maskUrlCss('')).toBeUndefined())
  it('адрес в url("…")', () => expect(maskUrlCss('/media/f/p?m=1&px=512')).toBe('url("/media/f/p?m=1&px=512")'))
  it('кавычка, обратная косая и перевод строки не выходят из url()', () => {
    const css = maskUrlCss('/a"b\\c\nd') ?? ''
    expect(css).toBe('url("/a%22b%5Cc%0Ad")')
  })
})

describe('watchMask: маска ставится только открывшейся', () => {
  const defer = () => {
    let done!: (ok: boolean) => void
    const p = new Promise<boolean>((r) => (done = r))
    return { p, done }
  }
  const flush = () => new Promise((r) => setTimeout(r, 0))

  it('пустой адрес — сразу снимает маску', () => {
    const got: string[] = []
    watchMask(
      '',
      (u) => got.push(u),
      () => Promise.reject(new Error('не должна грузить')),
    )
    expect(got).toEqual([''])
  })
  it('открылась — маска ставится', async () => {
    const got: string[] = []
    watchMask(
      '/m?px=256',
      (u) => got.push(u),
      () => Promise.resolve(true),
    )
    expect(got).toEqual([]) // пока грузится, прежняя маска остаётся
    await flush()
    expect(got).toEqual(['/m?px=256'])
  })
  it('404 — маски нет (иначе фото исчезло бы)', async () => {
    const got: string[] = []
    watchMask(
      '/m?px=256',
      (u) => got.push(u),
      () => Promise.resolve(false),
    )
    await flush()
    expect(got).toEqual([''])
  })
  it('отмена (смена адреса или размонтирование) — запоздавший ответ молчит', async () => {
    const d = defer()
    const got: string[] = []
    const stop = watchMask(
      '/m?px=256',
      (u) => got.push(u),
      () => d.p,
    )
    stop()
    d.done(true)
    await flush()
    expect(got).toEqual([])
  })
})
