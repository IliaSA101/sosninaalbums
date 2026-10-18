import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushSync } from 'svelte'
import { SvelteSet } from 'svelte/reactivity'
import type { BackdropView, SpreadView } from '../../base/core/types'

// Ответы Go подставляет тест; вызовы Api пишем в calls.
const calls: unknown[][] = []
const view = { rev: 2, folders: [], settings: {}, spreads: [] }

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  StraightenPhoto: async (...a: unknown[]) => {
    calls.push(['StraightenPhoto', ...a])
    return view
  },
  StraightenBackdrop: async (...a: unknown[]) => {
    calls.push(['StraightenBackdrop', ...a])
    return view
  },
}))

vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { noHooks } = await import('../../base/core/hooks')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { straighten } = await import('./straighten.svelte')
const { bgmode } = await import('../backdrop/bgmode.svelte')
const { placesel } = await import('../places/placesel.svelte')
const { drag } = await import('../../base/drag/drag.svelte')
const { modalStack } = await import('../../base/ui/modalstack.svelte')
const { keepStraighten, isLine, lineMM, placeBox, hitTarget, photoUnder, MIN_LINE_PX } = await import('./straighten')

const bd = (o: Partial<BackdropView> = {}): BackdropView => ({
  scope: 'spread',
  photo: 'p1',
  area: { x: 0, y: 0, w: 600, h: 300 },
  box: { x: 0, y: 0, w: 600, h: 300, rot: 0 },
  img: { x: 0, y: -50, w: 600, h: 400 },
  frame: { s: 1, x: 0, y: 0 },
  flipH: false,
  flipV: false,
  dpi: 300,
  fade: 0,
  ...o,
})
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

// Выделение подставляет тест: реактивный набор вместо placesel (верх не подключён).
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
  calls.length = 0
  prefs.level = 'full'
  straighten.exit()
  bgmode.exit()
  sel.clear()
  placesel.texts = []
  drag.active = false
  store.toast = null
  setView(spread())
})

describe('линия', () => {
  it('короче 12 px — не линия, ровно 12 — линия', () => {
    expect(MIN_LINE_PX).toBe(12)
    expect(isLine({ x: 0, y: 0 }, { x: 11, y: 0 })).toBe(false)
    expect(isLine({ x: 0, y: 0 }, { x: 12, y: 0 })).toBe(true)
    expect(isLine({ x: 5, y: 5 }, { x: 5, y: 5 })).toBe(false)
  })
  it('вектор в мм: разность точек × мм на пиксель, знак сохраняется', () => {
    expect(lineMM({ x: 10, y: 20 }, { x: 110, y: 10 }, 0.5)).toEqual({ dx: 50, dy: -5 })
  })
})

describe('цель', () => {
  it('рамка места без поворота', () => {
    expect(placeBox(spread(), 'a')).toEqual({ x: 10, y: 10, w: 100, h: 60, rot: 0 })
    expect(placeBox(spread(), 'нет')).toBeNull()
  })
  it('повёрнутое место: попадание по повёрнутой рамке, не по описанной', () => {
    const s = spread({ rects: [{ x: 0, y: 0, w: 100, h: 20 }], photos: ['a'], frames: [{ cellRot: 90 }] } as never)
    const t = { kind: 'photo', spread: 's1', photo: 'a' } as const
    // Центр (50, 10); после поворота на 90° место стоит вертикально: 20 по x, 100 по y
    expect(hitTarget(s, t, { x: 50, y: 50 })).toBe(true)
    expect(hitTarget(s, t, { x: 5, y: 10 })).toBe(false) // там место было бы без поворота
  })
  it('фон: внутри области и рамки', () => {
    const s = spread({ backdrops: [bd({ scope: 'left', area: { x: 0, y: 0, w: 300, h: 300 } })] })
    const t = { kind: 'backdrop', spread: 's1', scope: 'left' } as const
    expect(hitTarget(s, t, { x: 100, y: 100 })).toBe(true)
    expect(hitTarget(s, t, { x: 400, y: 100 })).toBe(false)
  })
})

describe('keepStraighten', () => {
  const photo = { kind: 'photo', spread: 's1', photo: 'a' } as const
  const back = { kind: 'backdrop', spread: 's1', scope: 'spread' } as const
  it('фото: тот же разворот и то же выделение', () => {
    expect(keepStraighten(photo, spread(), 'a', null)).toBe(true)
    expect(keepStraighten(photo, spread(), 'b', null)).toBe(false) // выделение сменилось
    expect(keepStraighten(photo, spread(), null, null)).toBe(false)
    expect(keepStraighten(photo, spread({ id: 's2' }), 'a', null)).toBe(false) // другой разворот
    expect(keepStraighten(photo, spread({ photos: ['b'] }), 'a', null)).toBe(false) // фото убрали
    expect(keepStraighten(null, spread(), 'a', null)).toBe(false)
    expect(keepStraighten(photo, undefined, 'a', null)).toBe(false)
  })
  it('фон: нужен режим фона той же области и сам фон', () => {
    const s = spread({ backdrops: [bd()] })
    expect(keepStraighten(back, s, null, { spread: 's1', scope: 'spread' })).toBe(true)
    expect(keepStraighten(back, s, null, null)).toBe(false) // «Готово»
    expect(keepStraighten(back, s, null, { spread: 's1', scope: 'left' })).toBe(false)
    expect(keepStraighten(back, spread(), null, { spread: 's1', scope: 'spread' })).toBe(false) // фон убрали
  })
})

describe('режим', () => {
  it('startPhoto включает режим для выделенного фото', () => {
    store.selectedCell = 'a'
    straighten.startPhoto()
    expect(straighten.cur).toEqual({ kind: 'photo', spread: 's1', photo: 'a' })
  })
  it('ничего не выделено — режим не включается, тост', () => {
    straighten.startPhoto()
    expect(straighten.cur).toBeNull()
    expect(store.toast?.text).toBe('Выделите одно фото')
  })
  it('выделенного фото нет в развороте — тост', () => {
    store.selectedCell = 'нет'
    straighten.startPhoto()
    expect(straighten.cur).toBeNull()
    expect(store.toast?.text).toBe('Выделите одно фото')
  })
  it('группа фото — тост, режим не включён (не «первое фото»)', () => {
    sel.add('a')
    sel.add('b')
    straighten.startPhoto()
    expect(straighten.cur).toBeNull()
    expect(store.toast?.text).toBe('Выделите одно фото')
  })
  it('фото и текст в группе — тост, режим не включён', () => {
    store.selectedCell = 'a'
    placesel.texts = ['t1']
    straighten.startPhoto()
    expect(straighten.cur).toBeNull()
    expect(store.toast?.text).toBe('Выделите одно фото')
  })
  it('без уровня «Наклон» режим не включается и тоста нет', () => {
    prefs.level = 'simple'
    store.selectedCell = 'a'
    straighten.startPhoto()
    expect(straighten.cur).toBeNull()
    expect(store.toast).toBeNull()
    setView(spread({ backdrops: [bd()] }))
    bgmode.enter('s1', 'spread')
    straighten.startBackdrop()
    expect(straighten.cur).toBeNull()
  })
  it('startBackdrop без режима фона ничего не включает', () => {
    setView(spread({ backdrops: [bd()] }))
    straighten.startBackdrop()
    expect(straighten.cur).toBeNull()
  })
  it('Esc выключает режим; чужая клавиша — нет', () => {
    store.selectedCell = 'a'
    straighten.startPhoto()
    expect(straighten.key({ key: 'Enter' })).toBe(false)
    expect(straighten.cur).not.toBeNull()
    expect(straighten.key({ key: 'Escape' })).toBe(true)
    expect(straighten.cur).toBeNull()
    expect(straighten.key({ key: 'Escape' })).toBe(false) // режима нет — Esc не наш
  })
  it('Esc при открытом окне: режим остаётся, событие не наше', () => {
    store.selectedCell = 'a'
    straighten.startPhoto()
    const id = modalStack.push()
    expect(straighten.key({ key: 'Escape' })).toBe(false)
    expect(straighten.cur).not.toBeNull()
    modalStack.remove(id)
    expect(straighten.key({ key: 'Escape' })).toBe(true) // окно закрыли — Esc снова наш
  })
})

// Реактивная связка $effect → check(): без ручного вызова check.
describe('режим живёт, пока цель на месте (эффект)', () => {
  const start = () => {
    store.selectedCell = 'a'
    straighten.startPhoto()
    flushSync()
    expect(straighten.cur).not.toBeNull()
  }
  it('смена выделения выключает режим', () => {
    start()
    store.selectedCell = 'b'
    flushSync()
    expect(straighten.cur).toBeNull()
  })
  it('добавили второе фото (Shift) — выход', () => {
    start()
    sel.add('b')
    flushSync()
    expect(straighten.cur).toBeNull()
  })
  it('в группу попал текст — выход', () => {
    start()
    placesel.texts = ['t1']
    flushSync()
    expect(straighten.cur).toBeNull()
  })
  it('смена разворота выключает режим', () => {
    start()
    setView(spread({ id: 's2' }))
    flushSync()
    expect(straighten.cur).toBeNull()
  })
  it('смена уровня без «Наклона» — выход', () => {
    start()
    prefs.level = 'simple'
    flushSync()
    expect(straighten.cur).toBeNull()
  })
  it('начало перетаскивания — выход', () => {
    start()
    drag.active = true
    flushSync()
    expect(straighten.cur).toBeNull()
  })
  it('режим фона закрыли («Готово») — выход из режима для фона', () => {
    setView(spread({ backdrops: [bd()] }))
    bgmode.enter('s1', 'spread')
    straighten.startBackdrop()
    flushSync()
    expect(straighten.cur).not.toBeNull()
    bgmode.exit()
    flushSync()
    expect(straighten.cur).toBeNull()
  })
})

describe('щелчок мимо цели', () => {
  // Размер листа и пустые тексты нужны stackAt.
  const full = () => spread({ w: 400, h: 100, captions: [], texts: [] } as never)
  it('по другому фото: выход и выделение этого фото', () => {
    setView(full())
    store.selectedCell = 'a'
    straighten.startPhoto()
    straighten.miss(full(), { x: 250, y: 40 })
    expect(straighten.cur).toBeNull()
    expect(store.selectedCell).toBe('b')
  })
  it('по пустому месту: только выход, выделение прежнее', () => {
    setView(full())
    store.selectedCell = 'a'
    straighten.startPhoto()
    straighten.miss(full(), { x: 150, y: 90 })
    expect(straighten.cur).toBeNull()
    expect(store.selectedCell).toBe('a')
  })
  it('мимо фона: только выход, выделение не меняется', () => {
    setView(spread({ w: 400, h: 100, captions: [], texts: [], backdrops: [bd({ scope: 'left' })] } as never))
    bgmode.enter('s1', 'left')
    sel.add('b')
    straighten.startBackdrop()
    expect(straighten.cur).not.toBeNull()
    straighten.miss(store.spread!, { x: 250, y: 40 }) // там лежит фото b
    expect(straighten.cur).toBeNull()
    expect([...sel]).toEqual(['b'])
  })
  it('photoUnder: цель в расчёт не берётся', () => {
    expect(photoUnder(full(), { x: 50, y: 40 })).toBe('a')
    expect(photoUnder(full(), { x: 50, y: 40 }, 'a')).toBeNull()
  })
})

describe('finish', () => {
  it('короткая линия Api не зовёт, режим остаётся', async () => {
    store.selectedCell = 'a'
    straighten.startPhoto()
    expect(await straighten.finish({ x: 0, y: 0 }, { x: 5, y: 5 }, 1)).toBe(false)
    expect(calls).toEqual([])
    expect(straighten.cur).not.toBeNull()
  })
  it('линия фото: StraightenPhoto с вектором в мм, режим выключается', async () => {
    store.selectedCell = 'a'
    straighten.startPhoto()
    expect(await straighten.finish({ x: 10, y: 20 }, { x: 110, y: 28 }, 0.5)).toBe(true)
    expect(calls).toEqual([['StraightenPhoto', 's1', 'a', 50, 4]])
    expect(straighten.cur).toBeNull()
  })
  it('линия фона: StraightenBackdrop с областью', async () => {
    setView(spread({ backdrops: [bd({ scope: 'left' })] }))
    bgmode.enter('s1', 'left')
    straighten.startBackdrop()
    expect(straighten.cur).toEqual({ kind: 'backdrop', spread: 's1', scope: 'left' })
    await straighten.finish({ x: 100, y: 0 }, { x: 0, y: 30 }, 2)
    expect(calls).toEqual([['StraightenBackdrop', 's1', 'left', -200, 60]])
    expect(straighten.cur).toBeNull()
  })
  it('без режима ничего не шлёт', async () => {
    expect(await straighten.finish({ x: 0, y: 0 }, { x: 100, y: 0 }, 1)).toBe(false)
    expect(calls).toEqual([])
  })
})
