// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'

// Силуэт под лицо (130): ответ Go рисуется только при show; устаревший ответ отбрасывается; ошибка вызова — тихо без силуэта.
const m = vi.hoisted(() => ({ guide: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ FaceGuide: m.guide }))

const { default: FaceGuide } = await import('./FaceGuide.svelte')
const { store } = await import('../../base/core/store.svelte')
const { framelive } = await import('./framelive.svelte')

const answer = (o: object = {}) => ({
  on: true,
  w: 40,
  h: 60,
  eyeY: 19.2,
  path: [
    [20, 10],
    [30, 20],
    [50, 60],
    [-10, 60],
    [10, 20],
  ],
  ...o,
})
const svg = () => document.querySelector<SVGElement>('[data-testid="face-guide"]')
const settle = async () => {
  await Promise.resolve()
  await tick()
  flushSync()
}
const setRev = (rev: number) => {
  store.view = { ...store.view, rev } as never
}

const onhave = vi.fn()
beforeEach(() => {
  onhave.mockReset()
  setRev(1)
  m.guide.mockReset()
  m.guide.mockResolvedValue(answer())
})
afterEach(cleanup)

describe('FaceGuide', () => {
  it('при show рисует вуаль, контур и линию глаз из ответа Go', async () => {
    render(FaceGuide, { spreadId: 's1', photo: 'a', show: true, onhave })
    await settle()
    expect(m.guide).toHaveBeenCalledWith('s1', 'a')
    expect(svg()?.getAttribute('viewBox')).toBe('0 0 40 60')
    expect(svg()?.querySelector('.veil')?.getAttribute('d')).toContain('M0 0H40V60H0Z')
    expect(svg()?.querySelector('.veil')?.getAttribute('fill-rule')).toBe('evenodd')
    expect(svg()?.querySelector('.line')?.getAttribute('d')).toMatch(/^M-10 60L10 20/)
    expect(svg()?.querySelector('.eye')?.getAttribute('y1')).toBe('19.2')
  })

  it('без show — ничего, с show — силуэт', async () => {
    const r = render(FaceGuide, { spreadId: 's1', photo: 'a', show: false, onhave })
    await settle()
    expect(svg()).toBeNull()
    await r.rerender({ spreadId: 's1', photo: 'a', show: true, onhave })
    expect(svg()).not.toBeNull()
  })

  it('show включается и гаснет вслед за подгонкой кадра', async () => {
    const r = render(FaceGuide, { spreadId: 's1', photo: 'a', show: false, onhave })
    await settle()
    await r.rerender({ spreadId: 's1', photo: 'a', show: true, onhave })
    expect(svg()).not.toBeNull()
    await r.rerender({ spreadId: 's1', photo: 'a', show: false, onhave })
    expect(svg()).toBeNull()
  })

  it('on=false от Go (групповое, «Целиком») или мусор — силуэта нет, onhave(false)', async () => {
    m.guide.mockResolvedValue(answer({ on: false, path: [] }))
    render(FaceGuide, { spreadId: 's1', photo: 'a', show: true, onhave })
    await settle()
    expect(svg()).toBeNull()
    expect(onhave).toHaveBeenLastCalledWith(false)
  })

  it('onhave(true), когда силуэт есть (даже без show), и onhave(false) при размонтировании', async () => {
    const r = render(FaceGuide, { spreadId: 's1', photo: 'a', show: false, onhave })
    await settle()
    expect(onhave).toHaveBeenLastCalledWith(true)
    r.unmount()
    expect(onhave).toHaveBeenLastCalledWith(false)
  })

  it('ошибка вызова — ничего на экране и в консоли', async () => {
    const err = vi.spyOn(console, 'error').mockImplementation(() => {})
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    m.guide.mockRejectedValue(new Error('нет'))
    render(FaceGuide, { spreadId: 's1', photo: 'a', show: true, onhave })
    await settle()
    expect(svg()).toBeNull()
    expect(err).not.toHaveBeenCalled()
    expect(warn).not.toHaveBeenCalled()
    err.mockRestore()
    warn.mockRestore()
  })

  it('перезапрос при смене rev; устаревший ответ не рисуется', async () => {
    let first!: (v: unknown) => void
    m.guide.mockReturnValueOnce(new Promise((res) => (first = res)))
    m.guide.mockResolvedValueOnce(answer({ eyeY: 30 }))
    render(FaceGuide, { spreadId: 's1', photo: 'a', show: true, onhave })
    await settle()
    setRev(2)
    await settle()
    expect(m.guide).toHaveBeenCalledTimes(2)
    expect(svg()?.querySelector('.eye')?.getAttribute('y1')).toBe('30')
    first(answer({ eyeY: 5 })) // ответ на первый запрос пришёл позже второго
    await settle()
    expect(svg()?.querySelector('.eye')?.getAttribute('y1')).toBe('30')
  })

  it('смена нормы листа без смены rev тоже перезапрашивает', async () => {
    render(FaceGuide, { spreadId: 's1', photo: 'a', show: true, onhave })
    await settle()
    m.guide.mockResolvedValue(answer({ eyeY: 25 }))
    framelive.bumpNorm()
    await settle()
    expect(svg()?.querySelector('.eye')?.getAttribute('y1')).toBe('25')
  })

  it('другое фото: силуэт прежнего места не остаётся, пока идёт новый запрос', async () => {
    const r = render(FaceGuide, { spreadId: 's1', photo: 'a', show: true, onhave })
    await settle()
    expect(svg()).not.toBeNull()
    m.guide.mockReturnValue(new Promise(() => {}))
    await r.rerender({ spreadId: 's1', photo: 'b', show: true, onhave })
    await settle()
    expect(svg()).toBeNull()
  })
})
