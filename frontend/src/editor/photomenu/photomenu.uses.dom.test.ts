// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// «Файл ▸ Где ещё стоит ▸» в меню фото на холсте (спека 108 U3): вхождения от WhereUsed, текущее место не показано,
// «…» пока ждём, поздний ответ отбрасывается, щелчок ведёт через goUse. Go и переход подменены.
const m = vi.hoisted(() => ({ whereUsed: vi.fn(), goUse: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ WhereUsed: m.whereUsed, ShowPhotoInExplorer: vi.fn() }))
vi.mock('../photoprops/gouse.svelte', () => ({ goUse: m.goUse }))

const { default: PhotoMenu } = await import('./PhotoMenu.svelte')
const { photoui } = await import('../photo/photoui.svelte')
const { placesel } = await import('../places/placesel.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')

const use = (spread: string, page: string, what: string, pick: string, kind = 'place') => ({
  sheet: 0,
  label: `${page} — ${what}`,
  kind,
  spread,
  page,
  what,
  pick,
})
// Фото a стоит здесь (s1, место «a»), на развороте 2 местом, на его фоне и в стикере.
const here = use('s1', 'Разворот 1', 'место', 'a')
const others = [use('s2', 'Разворот 2', 'место', 'a'), use('s2', 'Разворот 2', 'фон', 'left', 'backdrop')]

const spreadView = (id: string, photo: string) => ({
  id,
  w: 600,
  h: 300,
  mid: 300,
  grid: true,
  photos: [photo],
  rects: [{ x: 20, y: 20, w: 100, h: 100 }],
  frames: [],
  texts: [],
  captions: [],
  backdrops: [],
  places: [{ id: 'p1', photo, fit: '', spine: false, z: 0, x: 20, y: 20, w: 100, h: 100, rot: 0, shape: '', round: 0, radius: 0 }],
})

const open = async () => {
  render(PhotoMenu)
  placesel.select(['a'])
  photoui.menu = { x: 40, y: 30, kind: 'photo' }
  flushSync()
  await vi.waitFor(() => expect(document.body.querySelector('.drop')).not.toBeNull())
}
const file = () => fireEvent.pointerEnter(screen.getByRole('menuitem', { name: /Файл/ }))

beforeEach(() => {
  vi.spyOn(prefs, 'can').mockReturnValue(true)
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(200)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(200)
  window.innerWidth = 1000
  window.innerHeight = 700
  store.view = { ...store.view, rev: 1, spreads: [spreadView('s1', 'a')], folders: [], missing: [] } as never
  store.go(0)
  store.photos = [{ id: 'a', name: 'a.jpg', w: 3, h: 2 }] as never
  m.whereUsed.mockResolvedValue([{ id: 'a', uses: [here, ...others], places: 2, move: '' }])
})
afterEach(() => {
  cleanup()
  photoui.close()
  placesel.select([])
  vi.restoreAllMocks()
  vi.clearAllMocks()
  store.toast = null
})

describe('PhotoMenu: Файл ▸ Где ещё стоит ▸', () => {
  it('вход: «Файл ▸» одного фото — подпись «Где ещё стоит» и строки; запрос к Go по ID фото один раз', async () => {
    await open()
    expect(m.whereUsed).toHaveBeenCalledExactlyOnceWith(['a'])
    await file()
    // «Файл ▸» — второй уровень, глубже MenuList не рисует: вхождения идут строками под подписью.
    await vi.waitFor(() => expect(screen.getByText('Где ещё стоит')).toBeTruthy())
    expect(screen.queryByRole('menuitem', { name: /Где ещё стоит/ })).toBeNull()
  })

  it('строки — как на ленте: «Разворот N — что», текущее место не показано', async () => {
    await open()
    await file()
    await vi.waitFor(() => expect(screen.getByText('Разворот 2 — фон')).toBeTruthy())
    expect(screen.getByText('Разворот 2 — место')).toBeTruthy()
    expect(screen.queryByText('Разворот 1 — место')).toBeNull()
  })

  it('переход: щелчок по строке закрывает меню и зовёт goUse с вхождением', async () => {
    await open()
    await file()
    await vi.waitFor(() => expect(screen.getByText('Разворот 2 — фон')).toBeTruthy())
    await fireEvent.click(screen.getByText('Разворот 2 — фон'))
    expect(m.goUse).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ kind: 'backdrop', spread: 's2', pick: 'left' }))
    expect(photoui.menu).toBeNull()
  })

  it('стоит только здесь — серая строка «Больше нигде не стоит»', async () => {
    m.whereUsed.mockResolvedValue([{ id: 'a', uses: [here], places: 1, move: '' }])
    await open()
    await file()
    await vi.waitFor(() => expect(screen.getByText('Больше нигде не стоит')).toBeTruthy())
  })

  it('пока ответа нет — «…»; поздний ответ после закрытия отбрасывается', async () => {
    let done: (v: unknown) => void = () => {}
    m.whereUsed.mockReturnValueOnce(new Promise((r) => (done = r)))
    await open()
    await file()
    expect(screen.getByText('…')).toBeTruthy()
    photoui.close()
    flushSync()
    done([{ id: 'a', uses: others, places: 1, move: '' }])
    await Promise.resolve()
    await Promise.resolve()
    expect(document.body.querySelector('.drop')).toBeNull()
    expect(screen.queryByText('Разворот 2 — место')).toBeNull()
  })

  it('ошибка WhereUsed — меню закрыто, тост', async () => {
    m.whereUsed.mockRejectedValueOnce(new Error('сбой'))
    await open()
    await vi.waitFor(() => expect(store.toast?.kind).toBe('error'))
    expect(photoui.menu).toBeNull()
  })

  it('пустое место и несколько фото: Go за вхождениями не зовём', async () => {
    store.view = {
      ...store.view,
      spreads: [{ ...spreadView('s1', ''), places: [{ ...spreadView('s1', 'a').places[0], photo: '' }] }],
    } as never
    store.go(0)
    render(PhotoMenu)
    placesel.select(['p1'])
    photoui.menu = { x: 40, y: 30, kind: 'photo' }
    flushSync()
    await vi.waitFor(() => expect(document.body.querySelector('.drop')).not.toBeNull())
    expect(m.whereUsed).not.toHaveBeenCalled()
    expect(screen.queryByRole('menuitem', { name: /Где ещё стоит/ })).toBeNull()
  })
})
