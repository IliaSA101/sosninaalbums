// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// «Фон» карточки разворота и окно цвета (109): наведение показывает цвет на нужной странице, не правя альбом.
const setBackground = vi.hoisted(() => vi.fn(async () => undefined))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    MyColors: vi.fn(async () => []),
    SetBackground: setBackground,
  }
})

const { default: InspectorSpread } = await import('./InspectorSpread.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { colorLive, pageBg } = await import('../../base/color/colorlive.svelte')
const { mycolors } = await import('../../base/color/mycolorlist.svelte')

afterEach(() => {
  cleanup()
  colorLive.clear()
  setBackground.mockClear()
  vi.restoreAllMocks()
})

function show(bg: [string, string], bgSet: [string, string], albumBg = '') {
  store.view = {
    ...store.view,
    rev: 5,
    settings: { pageW: 300, pageH: 300, bg: albumBg },
    spreads: [{ id: 's1', grid: false, photos: [], places: [], frames: [], rects: [], bg, bgSet, backdrops: [], patterns: [] }],
  } as never
  vi.spyOn(prefs, 'can').mockImplementation((f) => f === 'bgColor')
  render(InspectorSpread)
}

async function hover(label: string, hex: string) {
  await fireEvent.click(screen.getByRole('button', { name: label }))
  flushSync()
  await fireEvent.pointerEnter(screen.getAllByRole('button', { name: new RegExp(hex) })[0])
}

describe('InspectorSpread: показ цвета фона', () => {
  it('наведение на фон страницы меняет только её, rev и Go не затронуты', async () => {
    show(['#ffffff', '#eeeeee'], ['', ''])
    await fireEvent.click(screen.getByRole('button', { name: /По страницам/ }))
    await hover('Цвет фона: правая страница', '#4a4643')
    const bg: [string, string] = ['#ffffff', '#eeeeee']
    expect(pageBg('s1', bg, ['', ''])).toEqual(['#ffffff', '#4a4643'])
    expect(store.view.rev).toBe(5)
    expect(setBackground).not.toHaveBeenCalled()
  })

  it('наведение на фон всего разворота — обе страницы; уход — прежние', async () => {
    show(['#ffffff', '#ffffff'], ['', ''])
    await hover('Цвет фона разворота', '#4a4643')
    const bg: [string, string] = ['#ffffff', '#ffffff']
    expect(pageBg('s1', bg, ['', ''])).toEqual(['#4a4643', '#4a4643'])
    await fireEvent.pointerLeave(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    expect(pageBg('s1', bg, ['', ''])).toEqual(bg)
  })

  it('щелчок — одна правка SetBackground, показ держится до ответа Go', async () => {
    show(['#ffffff', '#ffffff'], ['', ''])
    await hover('Цвет фона разворота', '#4a4643')
    await fireEvent.click(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    expect(setBackground).toHaveBeenCalledExactlyOnceWith('s1', 'spread', '#4a4643')
    expect(colorLive.color('bg:s1:spread', '')).toBe('#4a4643')
    await new Promise((r) => setTimeout(r, 0))
    expect(colorLive.color('bg:s1:spread', '')).toBe('')
  })
})

// 115 В и Г1: строка «Как в альбоме» в окне цвета и подпись «свой / как в альбоме» у строки карточки.
describe('InspectorSpread: свой цвет или как в альбоме (115)', () => {
  // Go подменён: RememberColor прежних тестов вернул undefined в «Свои» — окну нужен список
  beforeEach(() => (mycolors.list = []))
  const label = (t: string) => Array.from(document.querySelectorAll('.row .lbl')).find((e) => e.textContent?.startsWith(t))?.textContent
  const inheritRow = () => within(screen.getByRole('dialog')).getByRole('button', { name: /^Как в альбоме/ })
  const openWin = async (name: string) => {
    await fireEvent.click(screen.getByRole('button', { name }))
    flushSync()
  }

  it('подписи: «Весь разворот · как в альбоме» / «· свой» (достаточно одной страницы)', () => {
    show(['#ffffff', '#ffffff'], ['', ''])
    expect(label('Весь разворот')).toBe('Весь разворот · как в альбоме')
    cleanup()
    show(['#4f7a5a', '#4f7a5a'], ['#4f7a5a', '#4f7a5a'])
    expect(label('Весь разворот')).toBe('Весь разворот · свой')
  })

  it('по страницам: подпись каждой — по её цвету', async () => {
    show(['#4f7a5a', '#ffffff'], ['#4f7a5a', ''])
    await fireEvent.click(screen.getByRole('button', { name: /По страницам/ }))
    expect(label('Левая')).toBe('Левая · свой')
    expect(label('Правая')).toBe('Правая · как в альбоме')
  })

  it('своё: строка в окне не отмечена и показывает цвет альбома; щелчок — SetBackground(…, "") и окно закрыто', async () => {
    show(['#4f7a5a', '#4f7a5a'], ['#4f7a5a', '#4f7a5a'], '#e1ebd8')
    await openWin('Цвет фона разворота')
    expect(inheritRow().getAttribute('aria-pressed')).toBe('false')
    expect(inheritRow().textContent).toContain('#e1ebd8')
    await fireEvent.click(inheritRow())
    expect(setBackground).toHaveBeenCalledExactlyOnceWith('s1', 'spread', '')
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('как в альбоме: строка отмечена, шапка «Как в альбоме», щелчок — без правки', async () => {
    show(['#e1ebd8', '#e1ebd8'], ['', ''], '#e1ebd8')
    await openWin('Цвет фона разворота')
    expect(inheritRow().getAttribute('aria-pressed')).toBe('true')
    expect(screen.getByRole('dialog').querySelector('.name b')?.textContent).toBe('Как в альбоме')
    await fireEvent.click(inheritRow())
    expect(setBackground).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('«весь разворот» отмечен, только если своего нет на обеих страницах (цвета страниц совпали случайно)', async () => {
    show(['#4f7a5a', '#4f7a5a'], ['', '#4f7a5a'], '#4f7a5a')
    await openWin('Цвет фона разворота')
    expect(inheritRow().getAttribute('aria-pressed')).toBe('false')
  })

  it('по страницам: строка у левой и правой отмечена по отдельности, щелчок снимает только свою', async () => {
    show(['#4f7a5a', '#ffffff'], ['#4f7a5a', ''])
    await fireEvent.click(screen.getByRole('button', { name: /По страницам/ }))
    await openWin('Цвет фона: правая страница')
    expect(inheritRow().getAttribute('aria-pressed')).toBe('true')
    await fireEvent.click(inheritRow())
    expect(setBackground).not.toHaveBeenCalled()
    await openWin('Цвет фона: левая страница')
    expect(inheritRow().getAttribute('aria-pressed')).toBe('false')
    await fireEvent.click(inheritRow())
    expect(setBackground).toHaveBeenCalledExactlyOnceWith('s1', 'left', '')
  })

  it('наведение на строку показывает цвет альбома на развороте, уход — прежний', async () => {
    show(['#4f7a5a', '#4f7a5a'], ['#4f7a5a', '#4f7a5a'], '#e1ebd8')
    await openWin('Цвет фона разворота')
    await fireEvent.pointerEnter(inheritRow())
    expect(pageBg('s1', ['#4f7a5a', '#4f7a5a'], ['#4f7a5a', '#4f7a5a'])).toEqual(['#e1ebd8', '#e1ebd8'])
    await fireEvent.pointerLeave(inheritRow())
    expect(pageBg('s1', ['#4f7a5a', '#4f7a5a'], ['#4f7a5a', '#4f7a5a'])).toEqual(['#4f7a5a', '#4f7a5a'])
    expect(setBackground).not.toHaveBeenCalled()
  })

  it('белый альбом (bg пуст): строка называет #ffffff', async () => {
    show(['#4f7a5a', '#4f7a5a'], ['#4f7a5a', '#4f7a5a'])
    await openWin('Цвет фона разворота')
    expect(inheritRow().textContent).toContain('#ffffff')
  })

  it('обложка: строка «Как в альбоме» есть и в её окне цвета; без своего цвета сторонки идут за альбомом', async () => {
    const coverView = (sheet: unknown) => ({
      sheet,
      spec: { preset: 'custom', wrap: 20, steps: [] },
      preset: 'custom',
      spine: 10,
      n: 5,
      over: false,
      layout: 'both',
      spineBg: '',
      spineText: '',
      zones: [],
      drop: [],
      scopes: [],
      safe: [],
      markup: null,
      pxW: 100,
      pxH: 100,
      fix: '',
    })
    const cover = {
      id: 'cover',
      grid: false,
      photos: [],
      places: [],
      frames: [],
      rects: [],
      bg: ['#e1ebd8', '#e1ebd8'],
      bgSet: ['', ''],
      backdrops: [],
      patterns: [],
    }
    store.view = {
      ...store.view,
      rev: 5,
      settings: { pageW: 300, pageH: 300, bg: '#e1ebd8' },
      spreads: [],
      cover: coverView(cover),
    } as never
    store.goCover()
    vi.spyOn(prefs, 'can').mockImplementation((f) => f === 'bgColor')
    render(InspectorSpread)
    expect(document.querySelector('[aria-label="Фон"] .row .lbl')?.textContent).toContain('· как в альбоме')
    await fireEvent.click(document.querySelector<HTMLElement>('[aria-label="Фон"] .row button[aria-haspopup="dialog"]')!)
    flushSync()
    expect(inheritRow().getAttribute('aria-pressed')).toBe('true')
  })
})
