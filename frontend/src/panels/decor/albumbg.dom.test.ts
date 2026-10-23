// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Строка «Фон альбома» в настройках и карточка листа (098 §3 п. 1, 3, 4). Go подменён.
const api = vi.hoisted(() => ({
  DecorCatalog: vi.fn(),
  ClearAlbumBg: vi.fn(),
  SetAlbumBgFade: vi.fn(),
  SetNoAlbumBg: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)

const { default: AlbumBgRow } = await import('../details/AlbumBgRow.svelte')
const { default: InspectorSpread } = await import('../../editor/inspector/InspectorSpread.svelte')
const { albumFadeLive } = await import('../../editor/backdrop/fade.svelte')
const { decor } = await import('./decor.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')

const AB = { name: 'Лён', fade: 0.3, pages: false, preset: 'sa:len' }
const rect = { x: 0, y: 0, w: 100, h: 100 }
const sheet = (id: string, o: Record<string, unknown> = {}) => ({
  id,
  name: '',
  photos: [],
  rects: [],
  w: 600,
  h: 300,
  mid: 300,
  frames: [],
  texts: [],
  captions: [],
  bg: ['#fff', '#fff'],
  bgSet: ['', ''],
  backdrops: null,
  n: 0,
  ...o,
})
const albumLayer = { scope: 'spread', photo: 'p1', area: rect, box: { ...rect, rot: 0 }, img: rect, album: true }
const setView = (o: Record<string, unknown> = {}) => {
  store.view = { rev: 1, folders: [], settings: { pageW: 300, pageH: 300 }, spreads: [sheet('s1'), sheet('s2')], ...o } as never
  store.currentId = 's1'
}
const btn = (text: string) => Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.trim() === text)
const next = () => ({ ...store.view, rev: store.view.rev + 1 })
const slider = () => document.querySelector<HTMLInputElement>('input[type="range"]')!

const can = vi.spyOn(prefs, 'can')
beforeEach(() => {
  can.mockReturnValue(true) // уровень «Фоны» включён
  for (const f of Object.values(api)) f.mockReset()
  api.DecorCatalog.mockResolvedValue({ kind: 'bg', themes: [], items: [], recent: [], readOnly: false })
  for (const f of [api.ClearAlbumBg, api.SetAlbumBgFade, api.SetNoAlbumBg]) f.mockImplementation(async () => next())
  decor.close()
  albumFadeLive.clear()
  setView()
})
afterEach(() => cleanup())

describe('строка «Фон альбома» в настройках', () => {
  it('фона нет: «Выбрать…» открывает окно «Фон альбома», «Убрать» и ползунка нет', () => {
    render(AlbumBgRow)
    expect(btn('Убрать')).toBeUndefined()
    expect(slider()).toBeNull()
    btn('Выбрать…')!.click()
    flushSync()
    expect(decor.cur?.album).toBe(true)
    expect(decor.cur?.kind).toBe('bg')
  })

  it('фон есть: имя, «Выбрать…» (сменить), «Убрать» — один вызов Go', async () => {
    setView({ albumBg: AB })
    render(AlbumBgRow)
    expect(document.querySelector('.name')?.textContent).toBe('Лён')
    expect(btn('Выбрать…')).toBeTruthy()
    await fireEvent.click(btn('Убрать')!)
    expect(api.ClearAlbumBg).toHaveBeenCalledTimes(1)
  })

  it('ползунок: oninput только просмотр, Go зовётся на change — один раз, с сессией merge', async () => {
    setView({ albumBg: AB })
    render(AlbumBgRow)
    const s = slider()
    expect(s.value).toBe('30')
    await fireEvent.focus(s)
    s.value = '45'
    await fireEvent.input(s)
    s.value = '60'
    await fireEvent.input(s)
    expect(api.SetAlbumBgFade).not.toHaveBeenCalled()
    expect(albumFadeLive.cur?.fade).toBe(0.6) // просмотр на холсте
    await fireEvent.change(s)
    await vi.waitFor(() => expect(api.SetAlbumBgFade).toHaveBeenCalledTimes(1))
    const [fade, merge] = api.SetAlbumBgFade.mock.calls[0]
    expect(fade).toBe(0.6)
    expect(merge).toMatch(/^bd-[a-z0-9]+$/)
  })

  it('холостое отпускание (значение то же) просмотр снимает и в Go не идёт', async () => {
    setView({ albumBg: AB })
    render(AlbumBgRow)
    const s = slider()
    await fireEvent.focus(s)
    s.value = '50'
    await fireEvent.input(s)
    s.value = '30'
    await fireEvent.input(s)
    await fireEvent.blur(s)
    expect(albumFadeLive.cur).toBeNull()
    expect(api.SetAlbumBgFade).not.toHaveBeenCalled()
  })

  it('фон убрали (Ctrl+Z, чужая правка) — живой просмотр снят, ползунок исчез', async () => {
    setView({ albumBg: AB })
    render(AlbumBgRow)
    albumFadeLive.set('album', 'album', 0.8)
    setView()
    flushSync()
    expect(albumFadeLive.cur).toBeNull()
    expect(slider()).toBeNull()
  })

  it('без уровня «Фоны» выбрать нельзя, а стоящий фон — можно убрать', () => {
    can.mockReturnValue(false)
    setView({ albumBg: AB })
    render(AlbumBgRow)
    expect(btn('Выбрать…')).toBeUndefined()
    expect(btn('Убрать')).toBeTruthy()
  })
})

describe('карточка листа', () => {
  const card = () => document.querySelector('.album')
  const open = (sp: Record<string, unknown>, album: unknown = AB) => {
    setView({ albumBg: album, spreads: [sheet('s1', sp), sheet('s2')] })
    render(InspectorSpread)
  }

  it('фон альбома виден: «Фон: фон альбома» — «Изменить…», «Не показывать здесь»', async () => {
    open({ backdrops: [albumLayer] })
    expect(card()?.querySelector('.lbl')?.textContent).toBe('Фон: фон альбома')
    await fireEvent.click(btn('Не показывать здесь')!)
    expect(api.SetNoAlbumBg).toHaveBeenCalledWith('s1', true)
    await fireEvent.click(btn('Изменить…')!)
    expect(decor.cur?.album).toBe(true)
  })

  it('скрытый: «Фон альбома скрыт · Показать» — вызов с off = false', async () => {
    open({ noAlbumBg: true })
    expect(card()?.querySelector('.lbl')?.textContent).toBe('Фон альбома скрыт')
    expect(btn('Не показывать здесь')).toBeUndefined()
    await fireEvent.click(btn('Показать')!)
    expect(api.SetNoAlbumBg).toHaveBeenCalledWith('s1', false)
  })

  it('у листа свой фон — карточки нет; фона альбома нет — карточки нет', () => {
    open({ backdrops: [{ ...albumLayer, album: undefined }] })
    expect(card()).toBeNull()
    cleanup()
    open({ backdrops: [albumLayer] }, null)
    expect(card()).toBeNull()
  })

  it('без уровня «Фоны» «Изменить…» не показана, «Не показывать здесь» остаётся', () => {
    can.mockImplementation((id: string) => id !== 'decor')
    open({ backdrops: [albumLayer] })
    expect(btn('Изменить…')).toBeUndefined()
    expect(btn('Не показывать здесь')).toBeDefined()
  })

  it('слой альбома не даёт зон «Фото фона»: «Править» и «Убрать» своего фона нет', () => {
    open({ backdrops: [albumLayer] })
    expect(btn('Править')).toBeUndefined()
    expect(document.querySelector('.have')).toBeNull()
  })
})
