// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Фон альбома в окне «Фоны» (098 §3, 0.23-q5): пункт «Весь альбом» в строке «На» и окно «Фон альбома» из настроек. Go подменён.
const api = vi.hoisted(() => ({
  DecorCatalog: vi.fn(),
  PlanBackgroundTo: vi.fn(),
  ApplyBackground: vi.fn(),
  ApplyBackgroundTo: vi.fn(),
  SetAlbumBg: vi.fn(),
  SetAlbumBgToAll: vi.fn(),
  LibraryPacks: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))

const { default: DecorDialog } = await import('../decor/DecorDialog.svelte')
const { decor } = await import('../decor/decor.svelte')
const { store } = await import('../../base/core/store.svelte')

const cat = {
  kind: 'bg',
  themes: [{ id: 'all', name: 'Все', count: 2 }],
  items: [
    { id: 'sa:x', theme: 'basic', name: 'Икс', url: '/media/gen/art?k=x&v=1', own: false, aspect: 0, w: 0, h: 0 },
    { id: 'sa:y', theme: 'basic', name: 'Игрек', url: '/media/gen/art?k=y&v=1', own: false, aspect: 0, w: 0, h: 0 },
  ],
  recent: [],
  readOnly: false,
}
const sheet = (id: string, mid = 300) => ({
  id,
  photos: [],
  w: 600,
  h: 300,
  mid,
  frames: [],
  texts: [],
  captions: [],
  bg: ['#fff', '#fff'],
  bgSet: ['', ''],
  backdrops: null,
  n: 0,
})
const setView = (o: Record<string, unknown> = {}, settings: Record<string, unknown> = {}) => {
  store.view = {
    rev: 1,
    folders: [],
    settings: { pageW: 300, pageH: 300, ...settings },
    spreads: [sheet('s1'), sheet('s2'), sheet('s3')],
    ...o,
  } as never
  store.currentId = 's1'
}
const btn = (text: string) => Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.trim() === text)
const segs = (label: string) => Array.from(document.querySelectorAll(`[aria-label="${label}"] button`), (b) => b.textContent?.trim())
const openBg = async () => {
  render(DecorDialog)
  decor.show('bg', 's1', 'spread')
  await vi.waitFor(() => expect(decor.cat).not.toBeNull())
  flushSync()
}
const done = (rev: number) => ({ ...store.view, rev })

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.DecorCatalog.mockResolvedValue(cat)
  api.LibraryPacks.mockResolvedValue({ packs: [], net: {}, busy: false, readOnly: false })
  api.SetAlbumBg.mockImplementation(async () => done(store.view.rev + 1))
  api.SetAlbumBgToAll.mockImplementation(async () => done(store.view.rev + 1))
  api.ApplyBackground.mockImplementation(async () => done(store.view.rev + 1))
  decor.close()
  decor.cat = null
  decor.theme = ''
  setView()
})
afterEach(() => cleanup())

describe('«Весь альбом» в строке «На»', () => {
  it('стоит после «Все развороты», «Куда» переключается на «Весь разворот · Каждая страница»', async () => {
    await openBg()
    expect(segs('На какие развороты').slice(0, 3)).toEqual(['Этот разворот', 'Все развороты (3)', 'Весь альбом'])
    expect(segs('Куда поставить фон')).toEqual(['Весь разворот', 'Левая', 'Правая'])
    await fireEvent.click(btn('Весь альбом')!)
    expect(segs('Куда поставить фон альбома')).toEqual(['Весь разворот', 'Каждая страница'])
    expect(document.querySelector('.targets .hint')?.textContent).toBe('Не виден на листах со своим фоном')
  })

  it('щелчок по плитке ставит фон альбома одним вызовом, окно закрывается, листы не трогаются', async () => {
    await openBg()
    await fireEvent.click(btn('Весь альбом')!)
    await fireEvent.click(document.querySelector('.tiles .tile')!)
    await vi.waitFor(() => expect(api.SetAlbumBg).toHaveBeenCalledTimes(1))
    expect(api.SetAlbumBg).toHaveBeenCalledWith('sa:x', false)
    expect(api.ApplyBackground).not.toHaveBeenCalled()
    expect(api.PlanBackgroundTo).not.toHaveBeenCalled()
    expect(api.ApplyBackgroundTo).not.toHaveBeenCalled()
    expect(decor.cur).toBeNull()
  })

  it('«Каждая страница» — pages = true', async () => {
    await openBg()
    await fireEvent.click(btn('Весь альбом')!)
    await fireEvent.click(btn('Каждая страница')!)
    await fireEvent.click(document.querySelector('.tiles .tile')!)
    await vi.waitFor(() => expect(api.SetAlbumBg).toHaveBeenCalledWith('sa:x', true))
  })

  it('вернулись на «Этот разворот» — снова фон листа, альбом не трогаем', async () => {
    await openBg()
    await fireEvent.click(btn('Весь альбом')!)
    await fireEvent.click(btn('Этот разворот')!)
    expect(segs('Куда поставить фон')).toEqual(['Весь разворот', 'Левая', 'Правая'])
    await fireEvent.click(document.querySelector('.tiles .tile')!)
    await vi.waitFor(() => expect(api.ApplyBackground).toHaveBeenCalledWith('s1', 'spread', 'sa:x'))
    expect(api.SetAlbumBg).not.toHaveBeenCalled()
  })

  it('стоящий фон альбома отмечен в списке; повторный щелчок по нему уходит в Go (повтор там холостой)', async () => {
    setView({ albumBg: { name: 'Икс', fade: 0, pages: false, preset: 'sa:y' } })
    await openBg()
    await fireEvent.click(btn('Весь альбом')!)
    const cur = document.querySelectorAll('.tiles .tile.current')
    expect(cur).toHaveLength(1)
    expect(cur[0]).toBe(document.querySelectorAll('.tiles .tile')[1]) // sa:y — вторая плитка
    await fireEvent.click(cur[0])
    await vi.waitFor(() => expect(api.SetAlbumBg).toHaveBeenCalledWith('sa:y', false))
  })

  it('строки «На» нет на обложке и в альбоме из одного разворота — пункта «Весь альбом» там нет', async () => {
    setView({ spreads: [sheet('s1')] })
    await openBg()
    expect(document.querySelector('.targets')).toBeNull()
  })
})

describe('окно «Фон альбома» (из настроек и карточки листа)', () => {
  const openAlbum = async () => {
    render(DecorDialog)
    decor.showAlbum()
    await vi.waitFor(() => expect(decor.cat).not.toBeNull())
    flushSync()
  }

  it('заголовок «Фон альбома», строки «На» нет, «Куда» — разворот или страницы', async () => {
    await openAlbum()
    expect(document.querySelector('[role="dialog"]')?.textContent).toContain('Фон альбома')
    expect(document.querySelector('.targets')).toBeNull()
    expect(segs('Куда поставить фон альбома')).toEqual(['Весь разворот', 'Каждая страница'])
  })

  it('щелчок по плитке — SetAlbumBg и закрытие', async () => {
    await openAlbum()
    await fireEvent.click(document.querySelector('.tiles .tile')!)
    await vi.waitFor(() => expect(api.SetAlbumBg).toHaveBeenCalledWith('sa:x', false))
    expect(decor.cur).toBeNull()
  })

  it('альбом уже «по страницам» — окно открывается с «Каждая страница»', async () => {
    setView({ albumBg: { name: 'Икс', fade: 0, pages: true, preset: 'sa:x' } })
    await openAlbum()
    await fireEvent.click(document.querySelector('.tiles .tile')!)
    await vi.waitFor(() => expect(api.SetAlbumBg).toHaveBeenCalledWith('sa:x', true))
  })

  it('лист виньетки: страниц нет — переключателя «Куда» нет, pages всегда false', async () => {
    setView({ albumBg: { name: 'Икс', fade: 0, pages: true, preset: 'sa:x' }, spreads: [sheet('s1', 0), sheet('s2', 0)] }, { sheet: true })
    await openAlbum()
    expect(segs('Куда поставить фон альбома')).toEqual([])
    await fireEvent.click(document.querySelector('.tiles .tile')!)
    await vi.waitFor(() => expect(api.SetAlbumBg).toHaveBeenCalledWith('sa:x', false))
  })

  it('открыто с обложки — плитки рисуются по первому развороту, а не по обложке', async () => {
    store.currentId = 'cover'
    await openAlbum()
    expect(decor.cur?.spread).toBe('s1')
  })

  it('ошибка Go — красный тост, не молчание', async () => {
    api.SetAlbumBg.mockRejectedValue(new Error('нет такой плитки'))
    await openAlbum()
    await fireEvent.click(document.querySelector('.tiles .tile')!)
    await vi.waitFor(() => expect(store.toast?.kind).toBe('error'))
  })
})

// 115 Б: у разворотов свой фон — фон альбома на них не виден; до постановки вопрос в том же окне.
describe('вопрос про развороты со своим фоном (115)', () => {
  const photo = (o: Record<string, unknown> = {}) => ({ scope: 'spread', photo: 'p', ...o })
  const withOwn = () =>
    setView({
      spreads: [
        sheet('s1'),
        { ...sheet('s2'), backdrops: [photo(), photo({ scope: 'left' })] },
        { ...sheet('s3'), patterns: [{ scope: 'spread' }] },
        { ...sheet('s4'), backdrops: [photo({ album: true })] },
        { ...sheet('s5'), noAlbumBg: true },
      ],
    })
  const ask = () => screen.queryByRole('dialog', { name: 'Развороты со своим фоном' })
  const pickTile = async () => {
    await fireEvent.click(btn('Весь альбом')!)
    await fireEvent.click(document.querySelector('.tiles .tile')!)
    flushSync()
  }

  it('свои фоны есть — вопрос с номерами и фото, SetAlbumBg не зван, фокус на «Оставить свой»', async () => {
    withOwn()
    await openBg()
    await pickTile()
    expect(ask()?.textContent).toContain('У 2 разворотов свой фон: 2, 3. Фон альбома на них не виден. Сделать их как в альбоме?')
    expect(ask()?.textContent).toContain('Фото фона с них уйдут: 2')
    await vi.waitFor(() => expect(document.activeElement?.textContent?.trim()).toBe('Оставить свой'))
    expect(api.SetAlbumBg).not.toHaveBeenCalled()
    expect(api.SetAlbumBgToAll).not.toHaveBeenCalled()
    expect(decor.cur).not.toBeNull()
  })

  it('«Оставить свой» — окно закрыто, SetAlbumBg(id, pages), До-всех не зван', async () => {
    withOwn()
    await openBg()
    await pickTile()
    await fireEvent.click(btn('Оставить свой')!)
    await vi.waitFor(() => expect(api.SetAlbumBg).toHaveBeenCalledExactlyOnceWith('sa:x', false))
    expect(api.SetAlbumBgToAll).not.toHaveBeenCalled()
    expect(decor.cur).toBeNull()
    expect(ask()).toBeNull()
  })

  it('«Каждая страница» доходит до обоих ответов как pages = true', async () => {
    withOwn()
    await openBg()
    await fireEvent.click(btn('Весь альбом')!)
    await fireEvent.click(btn('Каждая страница')!)
    await fireEvent.click(document.querySelector('.tiles .tile')!)
    await fireEvent.click(btn('Как в альбоме')!)
    await vi.waitFor(() => expect(api.SetAlbumBgToAll).toHaveBeenCalledExactlyOnceWith('sa:x', true))
  })

  it('«Как в альбоме» — SetAlbumBgToAll один раз, окно закрыто, тост с числом', async () => {
    withOwn()
    await openBg()
    await pickTile()
    await fireEvent.click(btn('Как в альбоме')!)
    await vi.waitFor(() => expect(api.SetAlbumBgToAll).toHaveBeenCalledExactlyOnceWith('sa:x', false))
    expect(api.SetAlbumBg).not.toHaveBeenCalled()
    await vi.waitFor(() => expect(store.toast?.text).toBe('Фон альбома — теперь и на 2 разворотах'))
    expect(decor.cur).toBeNull()
  })

  it('× и Esc — назад к плиткам: окно «Фоны» на месте, вызовов нет', async () => {
    withOwn()
    await openBg()
    await pickTile()
    await fireEvent.click(within(ask()!).getByRole('button', { name: 'Закрыть' }))
    flushSync()
    expect(ask()).toBeNull()
    expect(decor.cur).not.toBeNull()
    expect(document.querySelectorAll('.tiles .tile').length).toBeGreaterThan(0)
    await fireEvent.click(document.querySelector('.tiles .tile')!)
    expect(ask()).not.toBeNull()
    await fireEvent.keyDown(window, { key: 'Escape' })
    flushSync()
    expect(ask()).toBeNull()
    expect(decor.cur).not.toBeNull()
    expect(api.SetAlbumBg).not.toHaveBeenCalled()
    expect(api.SetAlbumBgToAll).not.toHaveBeenCalled()
  })

  it('тот же фон альбома ещё раз — вопрос тоже есть', async () => {
    setView({
      albumBg: { name: 'Икс', fade: 0, pages: false, preset: 'sa:x' },
      spreads: [sheet('s1'), { ...sheet('s2'), patterns: [{ scope: 'spread' }] }],
    })
    await openBg()
    await pickTile()
    expect(ask()?.textContent).toContain('У 1 разворота свой фон: 2.')
    expect(api.SetAlbumBg).not.toHaveBeenCalled()
  })

  it('своих нет (фон альбома, «скрыт» без своего) — сразу SetAlbumBg, без вопроса', async () => {
    setView({ spreads: [sheet('s1'), { ...sheet('s2'), backdrops: [photo({ album: true })] }, { ...sheet('s3'), noAlbumBg: true }] })
    await openBg()
    await pickTile()
    await vi.waitFor(() => expect(api.SetAlbumBg).toHaveBeenCalledExactlyOnceWith('sa:x', false))
    expect(ask()).toBeNull()
    expect(api.SetAlbumBgToAll).not.toHaveBeenCalled()
  })

  it('ошибка Go на «Как в альбоме» — красный тост', async () => {
    api.SetAlbumBgToAll.mockRejectedValue(new Error('нет такой плитки'))
    withOwn()
    await openBg()
    await pickTile()
    await fireEvent.click(btn('Как в альбоме')!)
    await vi.waitFor(() => expect(store.toast?.kind).toBe('error'))
  })

  it('окно «Фон альбома» (из настроек) тоже спрашивает', async () => {
    withOwn()
    render(DecorDialog)
    decor.showAlbum()
    await vi.waitFor(() => expect(decor.cat).not.toBeNull())
    flushSync()
    await fireEvent.click(document.querySelector('.tiles .tile')!)
    expect(ask()).not.toBeNull()
  })
})
