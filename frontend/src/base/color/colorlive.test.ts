import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { INK_DELAY, colorLive, pageBg, withInk } from './colorlive.svelte'

beforeEach(() => vi.useFakeTimers())
afterEach(() => {
  colorLive.clear()
  vi.useRealTimers()
})

const ZERO: [string, string] = ['', '']

describe('colorLive', () => {
  it('свой ключ — живой цвет, чужой — из View', () => {
    expect(colorLive.color('text:t1', '#000000')).toBe('#000000')
    colorLive.set(['text:t1'], '#D8A9A1')
    expect(colorLive.color('text:t1', '#000000')).toBe('#d8a9a1')
    expect(colorLive.color('text:t2', '#000000')).toBe('#000000')
    expect(colorLive.style('text:t1', { color: '#000000', size: 12 })).toEqual({ color: '#d8a9a1', size: 12 })
    const st = { color: '#000000' }
    expect(colorLive.style('text:t2', st)).toBe(st)
    colorLive.clear()
    expect(colorLive.color('text:t1', '#000000')).toBe('#000000')
  })

  it('мусор вместо цвета и пустые ключи показ не включают', () => {
    colorLive.set(['k'], 'мусор')
    colorLive.set([], '#111111')
    expect(colorLive.color('k', 'x')).toBe('x')
  })

  it('inkUrl меняет только i-ю краску', () => {
    const url = '/media/gen/pat?k=bands&i=c8201e,203040&s=1.5&r=7&w=100&h=80&v=3'
    expect(withInk(url, 1, '#AABBCC')).toBe('/media/gen/pat?k=bands&i=c8201e,aabbcc&s=1.5&r=7&w=100&h=80&v=3')
    expect(withInk(url, 0, '#aabbcc')).toBe('/media/gen/pat?k=bands&i=aabbcc,203040&s=1.5&r=7&w=100&h=80&v=3')
    // i= последним параметром
    expect(withInk('/media/gen/art?k=heart&i=c8201e', 0, '#000000')).toBe('/media/gen/art?k=heart&i=000000')
    // i= первым
    expect(withInk('/m?i=aaaaaa&k=x', 0, '#bbbbbb')).toBe('/m?i=bbbbbb&k=x')
  })

  it('inkUrl без i= и с номером вне списка — адрес как был', () => {
    const noInk = '/media/gen/pat?k=bands&w=100&h=80&v=3'
    expect(withInk(noInk, 0, '#aabbcc')).toBe(noInk)
    const one = '/media/gen/art?k=heart&i=c8201e&v=3'
    expect(withInk(one, 1, '#aabbcc')).toBe(one)
    expect(withInk(one, -1, '#aabbcc')).toBe(one)
    expect(withInk(one, 0, 'мусор')).toBe(one)
    expect(withInk('/m?i=&k=x', 0, '#aabbcc')).toBe('/m?i=&k=x')
    expect(withInk('/m', 0, '#aabbcc')).toBe('/m')
    // «i=» внутри значения другого параметра не считается
    expect(withInk('/m?xi=aaaaaa&k=x', 0, '#bbbbbb')).toBe('/m?xi=aaaaaa&k=x')
  })

  it('краска — через 120 мс, CSS-цвет — сразу; clear снимает отложенный', () => {
    const url = '/media/gen/pat?k=bands&i=c8201e&v=3'
    colorLive.set(['pat:s1:left'], '#112233', 0)
    expect(colorLive.color('pat:s1:left', '#fff')).toBe('#112233')
    expect(colorLive.inkUrl('pat:s1:left', url)).toBe(url)
    vi.advanceTimersByTime(INK_DELAY - 1)
    expect(colorLive.inkUrl('pat:s1:left', url)).toBe(url)
    vi.advanceTimersByTime(1)
    expect(colorLive.inkUrl('pat:s1:left', url)).toBe('/media/gen/pat?k=bands&i=112233&v=3')
    expect(colorLive.inkUrl('pat:s1:right', url)).toBe(url)
    // новое наведение: пауза заново, прежняя краска держится до неё
    colorLive.set(['pat:s1:left'], '#445566', 0)
    vi.advanceTimersByTime(INK_DELAY - 1)
    expect(colorLive.inkUrl('pat:s1:left', url)).toBe('/media/gen/pat?k=bands&i=112233&v=3')
    vi.advanceTimersByTime(1)
    expect(colorLive.inkUrl('pat:s1:left', url)).toBe('/media/gen/pat?k=bands&i=445566&v=3')
    // clear до паузы — картинка не подменяется вовсе
    colorLive.clear()
    colorLive.set(['pat:s1:left'], '#778899', 0)
    colorLive.clear()
    vi.advanceTimersByTime(INK_DELAY * 2)
    expect(colorLive.inkUrl('pat:s1:left', url)).toBe(url)
  })

  it('commit: показ снимается после ответа Go', async () => {
    colorLive.set(['spine'], '#aabbcc')
    let done!: (v: unknown) => void
    const p = colorLive.commit(() => new Promise((r) => (done = r)))
    await Promise.resolve() // run стартует в микрозадаче очереди
    colorLive.clear() // уход мыши с образца, пока правка в пути
    expect(colorLive.color('spine', '')).toBe('#aabbcc')
    done(true)
    await p
    expect(colorLive.color('spine', '')).toBe('')
  })

  it('commit: ошибка Go тоже снимает показ, очередь не рвётся', async () => {
    colorLive.set(['spine'], '#aabbcc')
    const order: number[] = []
    const a = colorLive.commit(async () => {
      order.push(1)
      throw new Error('нет')
    })
    const b = colorLive.commit(async () => order.push(2))
    await Promise.allSettled([a, b])
    expect(order).toEqual([1, 2])
    expect(colorLive.color('spine', '')).toBe('')
  })
})

describe('pageBg', () => {
  const bg: [string, string] = ['#ffffff', '#eeeeee']

  it('нет показа — цвета из View', () => {
    expect(pageBg('s1', bg, ZERO)).toEqual(bg)
  })

  it('фон разворота — обе страницы, левая и правая — своя', () => {
    colorLive.set(['bg:s1:spread'], '#112233')
    expect(pageBg('s1', bg, ZERO)).toEqual(['#112233', '#112233'])
    expect(pageBg('s2', bg, ZERO)).toEqual(bg)
    colorLive.set(['bg:s1:right'], '#445566')
    expect(pageBg('s1', bg, ZERO)).toEqual(['#ffffff', '#445566'])
    colorLive.set(['bg:s1:left'], '#778899')
    expect(pageBg('s1', bg, ZERO)).toEqual(['#778899', '#eeeeee'])
  })

  it('album: только у страниц без своего цвета', () => {
    colorLive.set(['album'], '#123456')
    expect(pageBg('s1', bg, ZERO)).toEqual(['#123456', '#123456'])
    expect(pageBg('s1', bg, ['#ffffff', ''])).toEqual(['#ffffff', '#123456'])
    expect(pageBg('s1', bg, ['#ffffff', '#eeeeee'])).toEqual(bg)
  })
})
