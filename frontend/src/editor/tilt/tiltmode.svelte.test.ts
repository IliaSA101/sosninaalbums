import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushSync } from 'svelte'
import { SvelteSet } from 'svelte/reactivity'
import type { SpreadView } from '../../base/core/types'

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { noHooks } = await import('../../base/core/hooks')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { tiltmode, tiltAlive, hidesRotate, tiltRow } = await import('./tiltmode.svelte')
const { miniSpot } = await import('../places/minispot.svelte')
const { NO_ROOM } = await import('../places/minipanel')
const { straighten } = await import('./straighten.svelte')
const { bgmode } = await import('../backdrop/bgmode.svelte')
const { placesel } = await import('../places/placesel.svelte')
const { drag } = await import('../../base/drag/drag.svelte')
const { modalStack } = await import('../../base/ui/modalstack.svelte')
const { gallery } = await import('../../panels/gallery/gallery.svelte')
const { photoui } = await import('../photo/photoui.svelte')
const { framelive } = await import('../photo/framelive.svelte')

const spread = (o: Partial<SpreadView> = {}): SpreadView =>
  ({
    id: 's1',
    photos: ['a', 'b'],
    rects: [
      { x: 10, y: 10, w: 100, h: 60 },
      { x: 200, y: 10, w: 100, h: 60 },
    ],
    frames: [{ cellRot: 0 }, { cellRot: 0 }],
    backdrops: [],
    ...o,
  }) as unknown as SpreadView

const sel = new SvelteSet<string>()
store.plug({
  ...noHooks,
  selectedRefs: () => [...sel],
  select: (ids) => {
    sel.clear()
    for (const id of ids) sel.add(id)
  },
})
const setView = (s: SpreadView) => {
  store.view = { ...store.view, spreads: [s] }
  store.go(0)
}

beforeEach(() => {
  prefs.level = 'full'
  tiltmode.exit()
  miniSpot.photo = ''
  miniSpot.arc = NO_ROOM
  straighten.exit()
  bgmode.exit()
  sel.clear()
  placesel.texts = []
  placesel.stickers = []
  drag.active = false
  store.missing = []
  setView(spread())
})

const alive = {
  refs: 1,
  texts: 0,
  stickers: 0,
  photo: 'a' as string | null,
  missing: false,
  canTilt: true,
  dragging: false,
  backdrop: false,
}

describe('toggle', () => {
  it('включает и выключает; повторное нажатие возвращает ползунок', () => {
    store.selectedCell = 'a'
    tiltmode.toggle()
    expect(tiltmode.on).toBe(true)
    tiltmode.toggle()
    expect(tiltmode.on).toBe(false)
  })
  it('нечего выпрямлять — не включается', () => {
    tiltmode.toggle() // пусто
    expect(tiltmode.on).toBe(false)
    sel.add('a')
    sel.add('b') // группа
    tiltmode.toggle()
    expect(tiltmode.on).toBe(false)
  })
})

describe('alive: group, empty, text, sticker, missing, level, drag, backdrop', () => {
  it('чистая проверка', () => {
    expect(tiltAlive(alive)).toBe(true)
    expect(tiltAlive({ ...alive, refs: 0, photo: null })).toBe(false) // пусто
    expect(tiltAlive({ ...alive, refs: 2 })).toBe(false) // группа
    expect(tiltAlive({ ...alive, photo: null })).toBe(false) // пустое место
    expect(tiltAlive({ ...alive, texts: 1 })).toBe(false)
    expect(tiltAlive({ ...alive, stickers: 1 })).toBe(false)
    expect(tiltAlive({ ...alive, missing: true })).toBe(false)
    expect(tiltAlive({ ...alive, canTilt: false })).toBe(false)
    expect(tiltAlive({ ...alive, dragging: true })).toBe(false)
    expect(tiltAlive({ ...alive, backdrop: true })).toBe(false)
  })
  const start = () => {
    store.selectedCell = 'a'
    tiltmode.toggle()
    flushSync()
    expect(tiltmode.on).toBe(true)
  }
  it('снятие выделения выключает режим', () => {
    start()
    store.selectedCell = null
    flushSync()
    expect(tiltmode.on).toBe(false)
  })
  it('группа выключает режим', () => {
    start()
    sel.add('b')
    flushSync()
    expect(tiltmode.on).toBe(false)
  })
  it('текст в группе и стикер выключают режим', () => {
    start()
    placesel.texts = ['t1']
    flushSync()
    expect(tiltmode.on).toBe(false)
    placesel.texts = []
    start()
    placesel.stickers = ['k1']
    flushSync()
    expect(tiltmode.on).toBe(false)
  })
  it('фото пропало с диска — выход', () => {
    start()
    store.missing = ['a']
    flushSync()
    expect(tiltmode.on).toBe(false)
  })
  it('фото убрали с разворота — выход', () => {
    start()
    setView(spread({ photos: ['b'], rects: [{ x: 0, y: 0, w: 10, h: 10 }], frames: [{ cellRot: 0 }] } as never))
    flushSync()
    expect(tiltmode.on).toBe(false)
  })
  it('уровень без «Наклона» — выход', () => {
    start()
    prefs.level = 'simple'
    flushSync()
    expect(tiltmode.on).toBe(false)
  })
  it('перенос — выход', () => {
    start()
    drag.active = true
    flushSync()
    expect(tiltmode.on).toBe(false)
  })
  it('режим фона — выход', () => {
    start()
    setView(spread({ backdrops: [{ scope: 'spread', photo: 'p1' }] } as never))
    bgmode.enter('s1', 'spread')
    flushSync()
    expect(tiltmode.on).toBe(false)
  })
})

const IN = { mode: 'in', vx: 100, vy: 0, vw: 200 } as const

describe('выход при смене выделения', () => {
  it('другое фото — выход (владелец 2026-10-16)', () => {
    store.selectedCell = 'a'
    tiltmode.toggle()
    flushSync()
    expect(tiltmode.on).toBe(true)
    store.selectedCell = 'b'
    flushSync()
    expect(tiltmode.on).toBe(false)
  })
  it('щелчок мимо (снятие выделения) — выход (владелец 2026-10-16)', () => {
    store.selectedCell = 'a'
    tiltmode.toggle()
    flushSync()
    store.selectedCell = null
    flushSync()
    expect(tiltmode.on).toBe(false)
  })
  it('повторное нажатие — выход (владелец 2026-10-16)', () => {
    store.selectedCell = 'a'
    tiltmode.toggle()
    expect(tiltmode.on).toBe(true)
    tiltmode.toggle()
    expect(tiltmode.on).toBe(false)
  })
  it('серия: на следующем фото режим включают заново', () => {
    store.selectedCell = 'a'
    tiltmode.toggle()
    flushSync()
    store.selectedCell = 'b'
    flushSync()
    expect(tiltmode.on).toBe(false)
    tiltmode.toggle()
    flushSync()
    expect(tiltmode.on).toBe(true)
    store.selectedCell = 'a'
    flushSync()
    expect(tiltmode.on).toBe(false)
  })
  it('чистая проверка: same и room', () => {
    expect(tiltAlive({ ...alive, same: false })).toBe(false)
    expect(tiltAlive({ ...alive, room: false })).toBe(false)
    expect(tiltAlive({ ...alive, same: true, room: true })).toBe(true)
  })
})

describe('hidesRotate', () => {
  it('⟳ спрятан у фото в режиме (владелец 2026-10-16)', () => {
    expect(hidesRotate(true, 'a', 'a')).toBe(true)
    expect(hidesRotate(false, 'a', 'a')).toBe(false) // режима нет
    expect(hidesRotate(true, 'a', 'b')).toBe(false) // чужая рамка
    expect(hidesRotate(true, undefined, null)).toBe(false) // место без фото
    expect(hidesRotate(true, '', '')).toBe(false)
  })
})

describe('место для дуги', () => {
  it('дуге не стало места — выход; пока панель перерисовывается (photo пуст) — режим держится', () => {
    store.selectedCell = 'a'
    miniSpot.photo = 'a'
    miniSpot.arc = IN
    tiltmode.toggle()
    flushSync()
    expect(tiltmode.on).toBe(true)
    miniSpot.photo = ''
    miniSpot.arc = NO_ROOM
    flushSync()
    expect(tiltmode.on).toBe(true) // сведений нет
    miniSpot.photo = 'a'
    flushSync()
    expect(tiltmode.on).toBe(false) // места нет — выход
  })
  it('сведения о другом фото режим не гасят', () => {
    store.selectedCell = 'a'
    miniSpot.photo = 'a'
    miniSpot.arc = IN
    tiltmode.toggle()
    flushSync()
    miniSpot.photo = 'b'
    miniSpot.arc = NO_ROOM
    flushSync()
    expect(tiltmode.on).toBe(true)
  })
  it('мелкое фото: кнопка активна, нажатие показывает строку «Наклон» в карточке, дуга не включается (владелец 2026-10-17)', () => {
    const show = vi.spyOn(tiltRow, 'show').mockImplementation(() => {})
    store.selectedCell = 'a'
    miniSpot.photo = 'a'
    miniSpot.arc = NO_ROOM
    tiltmode.toggle()
    flushSync()
    expect(show).toHaveBeenCalledTimes(1)
    expect(tiltmode.on).toBe(false)
    show.mockRestore()
  })
  it('дуге есть место — строка в карточке не показывается', () => {
    const show = vi.spyOn(tiltRow, 'show').mockImplementation(() => {})
    store.selectedCell = 'a'
    miniSpot.photo = 'a'
    miniSpot.arc = IN
    tiltmode.toggle()
    expect(show).not.toHaveBeenCalled()
    expect(tiltmode.on).toBe(true)
    show.mockRestore()
  })
  it('нечего выпрямлять и мелкое фото — карточка не показывается', () => {
    const show = vi.spyOn(tiltRow, 'show').mockImplementation(() => {})
    miniSpot.photo = 'a'
    miniSpot.arc = NO_ROOM
    tiltmode.toggle() // ничего не выделено
    expect(show).not.toHaveBeenCalled()
    show.mockRestore()
  })
})

describe('esc order', () => {
  const start = () => {
    store.selectedCell = 'a'
    tiltmode.toggle()
    flushSync()
  }
  it('режима нет или клавиша чужая — не наша', () => {
    expect(tiltmode.key({ key: 'Escape' })).toBe(false)
    start()
    expect(tiltmode.key({ key: 'Enter' })).toBe(false)
    expect(tiltmode.on).toBe(true)
  })
  it('окно открыто — не наша, режим остаётся', () => {
    start()
    const id = modalStack.push()
    expect(tiltmode.key({ key: 'Escape' })).toBe(false)
    expect(tiltmode.on).toBe(true)
    modalStack.remove(id)
  })
  it('галерея раскладок открыта — не наша: Esc закрывает её', () => {
    start()
    gallery.cur = { spread: 's1', scope: 'spread', tab: 'layouts' } as never
    expect(tiltmode.key({ key: 'Escape' })).toBe(false)
    expect(tiltmode.on).toBe(true)
    gallery.cur = null
  })
  it('идёт протяжка шкалы — не наша: отмену жеста делает шкала', () => {
    start()
    tiltmode.dialing = true
    expect(tiltmode.key({ key: 'Escape' })).toBe(false)
    expect(tiltmode.on).toBe(true)
  })
  it('«по линии» включено — не наша; режим шкалы остаётся', () => {
    start()
    straighten.startPhoto('a')
    expect(straighten.cur).not.toBeNull()
    expect(tiltmode.key({ key: 'Escape' })).toBe(false)
    expect(straighten.key({ key: 'Escape' })).toBe(true) // линию закрывает её обработчик
    expect(tiltmode.on).toBe(true)
  })
  it('меню фото открыто — не наша: Esc закрывает только меню, режим остаётся', () => {
    start()
    photoui.menu = { x: 1, y: 1 }
    expect(tiltmode.key({ key: 'Escape' })).toBe(false)
    expect(tiltmode.on).toBe(true)
    photoui.close() // меню закрыл свой обработчик
    expect(tiltmode.key({ key: 'Escape' })).toBe(true) // следующий Esc — уже выход из режима
    expect(tiltmode.on).toBe(false)
  })
  it('сдвиг кадра за ✥ идёт — не наша: Esc отменяет только его', () => {
    start()
    framelive.set('a', { s: 1, x: 0, y: 0 }, false, true)
    expect(tiltmode.key({ key: 'Escape' })).toBe(false)
    expect(tiltmode.on).toBe(true)
    framelive.clear()
    expect(tiltmode.key({ key: 'Escape' })).toBe(true)
  })
  it('Esc уже занят кем-то (defaultPrevented: слой, палитра) — не наша', () => {
    start()
    expect(tiltmode.key({ key: 'Escape', defaultPrevented: true })).toBe(false)
    expect(tiltmode.on).toBe(true)
  })
  it('Esc — выход, выделение остаётся; следующий Esc не наш (владелец 2026-10-16)', () => {
    start()
    expect(tiltmode.key({ key: 'Escape' })).toBe(true)
    expect(tiltmode.on).toBe(false)
    expect(store.selectedCell).toBe('a')
    expect(tiltmode.key({ key: 'Escape' })).toBe(false) // дальше хоткей «снять выделение»
  })
  it('выход сбрасывает признак протяжки', () => {
    start()
    tiltmode.dialing = true
    tiltmode.exit()
    expect(tiltmode.dialing).toBe(false)
  })
})
