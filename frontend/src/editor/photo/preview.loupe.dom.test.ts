// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Лупа окна крупного просмотра (128-4): щелчок и клавиша — 1:1 в точке, тянуть — двигать, Ctrl+колесо и Mod+=/− — зум,
// 0 — вписать, перенос при листании, значок зума и строка клавиш.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { default: PhotoPreview } = await import('./PhotoPreview.svelte')
const { photoui } = await import('./photoui.svelte')
const { store } = await import('../../base/core/store.svelte')
const { keymap } = await import('../../base/keys/keymap.svelte')
const { useCatalog, dropCatalog } = await import('../../base/keys/keymap.fixture')
const { setOsForTest } = await import('../../base/core/platform')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { zoneOf } = await import('../places/groupsel')

// Превью: a, b, c — горизонтальные 2048×1365 (b чуть другого размера, пропорции те же), v — вертикальное 1365×2048.
const sizes: Record<string, [number, number]> = { a: [2048, 1365], s: [400, 300], b: [2000, 1333], c: [2048, 1365], v: [1365, 2048] }
const natOf = (el: HTMLImageElement) => sizes[el.getAttribute('src')?.match(/\/p\/(\w+)/)?.[1] ?? 'a']

beforeEach(() => {
  useCatalog()
  vi.spyOn(HTMLImageElement.prototype, 'naturalWidth', 'get').mockImplementation(function (this: HTMLImageElement) {
    return natOf(this)[0]
  })
  vi.spyOn(HTMLImageElement.prototype, 'naturalHeight', 'get').mockImplementation(function (this: HTMLImageElement) {
    return natOf(this)[1]
  })
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(1200)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(700)
  store.view = { ...store.view, rev: 1, spreads: [], folders: [], missing: [] } as never
  store.photos = ['a', 'b', 'v', 'c'].map((id) => ({ id, name: `${id}.jpg`, w: 3, h: 2 })) as never
  store.selected.clear()
})
afterEach(() => {
  cleanup()
  photoui.preview = null
  prefs.level = 'simple'
  prefs.custom = null
  setOsForTest(null)
  dropCatalog()
  vi.restoreAllMocks()
})

const img = () => document.querySelector<HTMLImageElement>('img.ph')!
const frame = () => document.querySelector<HTMLElement>('.frame')!
const badge = () => document.querySelector('.cap .zoom')?.textContent
const w = () => parseFloat(frame().style.width)
const move = () => frame().style.transform
const loaded = () => {
  flushSync()
  fireEvent.load(img())
  flushSync()
}
const open = (id = 'a') => {
  render(PhotoPreview)
  photoui.openLibrary(id)
  loaded()
}
// Щелчок по кадру в точке окна (мышь без сдвига).
function click(x = 650, y = 350) {
  const at = { clientX: x, clientY: y, button: 0, pointerId: 1 }
  fireEvent.pointerDown(img(), at)
  fireEvent.pointerUp(img(), at)
  flushSync()
}
function key(code: string, init: KeyboardEventInit = {}) {
  const e = new KeyboardEvent('keydown', { code, key: init.key ?? code, bubbles: true, cancelable: true, ...init })
  document.querySelector('[role=dialog]')!.dispatchEvent(e)
  flushSync()
  return e
}
const next = (id: string) => {
  photoui.preview = { ids: ['a', 'b', 'v', 'c'], at: ['a', 'b', 'v', 'c'].indexOf(id), from: 'library' }
  loaded()
}

describe('вписано и 1:1', () => {
  it('открывается вписанным, значок «Вписано · N %», курсор — лупа', () => {
    open()
    expect(badge()).toBe('Вписано · 51 %')
    expect(w()).toBeCloseTo(2048 * (700 / 1365), 1)
    expect(frame().classList.contains('canzoom')).toBe(true)
    expect(frame().classList.contains('zoomed')).toBe(false)
  })

  it('щелчок — 1:1, точка под курсором на месте; второй щелчок — вписано', () => {
    open()
    const fitW = w()
    const left0 = parseFloat(move().match(/translate\(([-\d.]+)px/)![1])
    click(650, 350)
    expect(badge()).toBe('1:1')
    expect(w()).toBe(2048)
    expect(frame().classList.contains('zoomed')).toBe(true)
    // доля кадра под курсором (650 - left) / w не изменилась
    const left1 = parseFloat(move().match(/translate\(([-\d.]+)px/)![1])
    expect((650 - left1) / 2048).toBeCloseTo((650 - left0) / fitW, 4)
    click(650, 350)
    expect(badge()).toBe('Вписано · 51 %')
    expect(w()).toBeCloseTo(fitW, 3)
  })

  it('тянуть: сдвиг ≥ 5 px не переключает масштаб, в 1:1 двигает кадр, вписанный не двигает', () => {
    open()
    const start = { clientX: 600, clientY: 300, button: 0, pointerId: 1 }
    fireEvent.pointerDown(img(), start)
    fireEvent.pointerMove(img(), { ...start, clientX: 640 })
    fireEvent.pointerUp(img(), { ...start, clientX: 640 })
    flushSync()
    expect(badge()).toBe('Вписано · 51 %') // вписанный не двигается и по отпусканию не переключается
    click()
    const before = move()
    fireEvent.pointerDown(img(), start)
    fireEvent.pointerMove(img(), { ...start, clientX: 640 })
    flushSync()
    expect(frame().classList.contains('panning')).toBe(true)
    fireEvent.pointerUp(img(), { ...start, clientX: 640 })
    flushSync()
    expect(badge()).toBe('1:1')
    expect(move()).not.toBe(before)
    expect(frame().classList.contains('panning')).toBe(false)
  })

  it('сдвиг меньше 5 px — всё ещё щелчок', () => {
    open()
    const at = { clientX: 600, clientY: 300, button: 0, pointerId: 1 }
    fireEvent.pointerDown(img(), at)
    fireEvent.pointerMove(img(), { ...at, clientX: 603 })
    fireEvent.pointerUp(img(), { ...at, clientX: 603 })
    flushSync()
    expect(badge()).toBe('1:1')
  })

  it('правая кнопка не запускает лупу', () => {
    open()
    fireEvent.pointerDown(img(), { clientX: 600, clientY: 300, button: 2, pointerId: 1 })
    fireEvent.pointerUp(img(), { clientX: 600, clientY: 300, button: 2, pointerId: 1 })
    flushSync()
    expect(badge()).toBe('Вписано · 51 %')
  })

  it('картинка не загружена: щелчок и клавиши ничего не делают', () => {
    render(PhotoPreview)
    photoui.openLibrary('a')
    flushSync()
    expect(badge()).toBeUndefined()
    click()
    key('KeyZ')
    expect(w()).toBeGreaterThan(0)
    loaded()
    expect(badge()).toBe('Вписано · 51 %')
  })
})

describe('клавиши окна', () => {
  it('Z — 1:1 и обратно; повтор автоповтором не мигает', () => {
    open()
    key('KeyZ')
    expect(badge()).toBe('1:1')
    key('KeyZ', { repeat: true })
    expect(badge()).toBe('1:1')
    key('KeyZ')
    expect(badge()).toBe('Вписано · 51 %')
  })

  it('0 и Ctrl+0 вписывают', () => {
    open()
    key('KeyZ')
    key('Digit0')
    expect(badge()).toBe('Вписано · 51 %')
    key('KeyZ')
    key('Digit0', { ctrlKey: true })
    expect(badge()).toBe('Вписано · 51 %')
  })

  it('Ctrl+= и Ctrl+− меняют масштаб шагом холста, 1:1 — предел', () => {
    open()
    key('Equal', { ctrlKey: true })
    expect(badge()).toBe('64 %')
    for (let i = 0; i < 8; i++) key('Equal', { ctrlKey: true })
    expect(badge()).toBe('1:1')
    key('Minus', { ctrlKey: true })
    expect(badge()).toBe('80 %')
    for (let i = 0; i < 8; i++) key('Minus', { ctrlKey: true })
    expect(badge()).toBe('Вписано · 51 %')
  })

  it('клавиши гасятся окном (не доходят до холста)', () => {
    open()
    for (const [code, init] of [
      ['KeyZ', {}],
      ['Digit0', {}],
      ['Equal', { ctrlKey: true }],
      ['Minus', { ctrlKey: true }],
    ] as const) {
      expect(key(code, init).defaultPrevented, code).toBe(true)
    }
  })

  it('переназначенная клавиша работает, Z — нет', () => {
    useCatalog({ photoPreview: ['KeyG'] })
    open()
    key('KeyZ')
    expect(badge()).toBe('Вписано · 51 %')
    key('KeyG')
    expect(badge()).toBe('1:1')
  })

  it('Esc из 1:1 закрывает окно', () => {
    open()
    key('KeyZ')
    key('Escape', { key: 'Escape' })
    expect(photoui.preview).toBeNull()
  })
})

describe('колесо', () => {
  const wheel = (init: WheelEventInit) => {
    // WheelEvent в happy-dom не несёт модификаторов и координат — кладём их на обычное событие
    const e = Object.assign(new Event('wheel', { bubbles: true, cancelable: true }), {
      clientX: 600,
      clientY: 350,
      deltaMode: 0,
      ctrlKey: false,
      ...init,
    })
    document.querySelector('.view')!.dispatchEvent(e)
    flushSync()
    return e
  }
  it('Ctrl+колесо — плавный зум у курсора, вниз — обратно', () => {
    open()
    const e = wheel({ deltaY: -100, ctrlKey: true })
    expect(e.defaultPrevented).toBe(true)
    const up = parseFloat(badge()!)
    expect(up).toBeGreaterThan(51)
    wheel({ deltaY: 100, ctrlKey: true })
    expect(badge()).toBe('Вписано · 51 %')
  })
  // 134-5: щипок Mac — GestureEvent (happy-dom его не знает: событие собираем руками)
  const gesture = (type: string, init: { scale?: number; clientX?: number; clientY?: number }) => {
    const e = new Event(type, { bubbles: true, cancelable: true })
    for (const [k, v] of Object.entries(init)) Object.defineProperty(e, k, { value: v })
    document.querySelector('.view')!.dispatchEvent(e)
    flushSync()
    return e
  }
  it('щипок: gesturechange scale 1,5 — масштаб ×1,5 у точки жеста (вписано 51 %, дальше 1:1 — предел), событие погашено', () => {
    open()
    const s0 = w() / 2048
    const left0 = parseFloat(move().match(/translate\(([-\d.]+)px/)![1])
    expect(gesture('gesturestart', { scale: 1, clientX: 900, clientY: 350 }).defaultPrevented).toBe(true)
    const e = gesture('gesturechange', { scale: 1.5, clientX: 900, clientY: 350 })
    expect(e.defaultPrevented).toBe(true)
    expect(w() / 2048).toBeCloseTo(s0 * 1.5, 6)
    // точка под пальцами осталась на месте: доля кадра по x та же
    const left1 = parseFloat(move().match(/translate\(([-\d.]+)px/)![1])
    expect((900 - left1) / w()).toBeCloseTo((900 - left0) / (s0 * 2048), 4)
    gesture('gesturechange', { scale: 1, clientX: 900, clientY: 350 }) // свёл обратно
    expect(badge()).toBe('Вписано · 51 %')
    gesture('gestureend', { scale: 1, clientX: 900, clientY: 350 })
  })
  it('Ctrl-колесо посреди щипка не зумит (старый WebKit дублирует жест)', () => {
    open()
    gesture('gesturestart', { scale: 1, clientX: 900, clientY: 350 })
    const e = wheel({ deltaY: -100, ctrlKey: true })
    expect(e.defaultPrevented).toBe(true)
    expect(badge()).toBe('Вписано · 51 %')
    gesture('gestureend', { scale: 1 })
    wheel({ deltaY: -100, ctrlKey: true })
    expect(parseFloat(badge()!)).toBeGreaterThan(51)
  })
  it('предел — 1:1', () => {
    open()
    for (let i = 0; i < 10; i++) wheel({ deltaY: -100, ctrlKey: true })
    expect(badge()).toBe('1:1')
  })
  it('колесо без Ctrl ничего не делает, но страница не прокручивается', () => {
    open()
    const e = wheel({ deltaY: -100 })
    expect(e.defaultPrevented).toBe(true)
    expect(badge()).toBe('Вписано · 51 %')
    expect(photoui.previewId).toBe('a')
  })
})

describe('листание', () => {
  it('те же пропорции: 1:1 и точка сохраняются («мигание» серии)', () => {
    open()
    click(900, 200)
    const at = move()
    next('b')
    expect(photoui.previewId).toBe('b')
    expect(badge()).toBe('1:1')
    expect(w()).toBe(2000)
    // точка кадра в центре окна (доля) та же, хотя кадр другого размера
    const centre = (t: string, wid: number) => (600 - parseFloat(t.match(/translate\(([-\d.]+)px/)![1])) / wid
    expect(centre(move(), 2000)).toBeCloseTo(centre(at, 2048), 3)
  })

  it('другие пропорции (вертикаль) — вписано; вернулись на горизонталь — вписано, прежний зум не всплывает', () => {
    open()
    click()
    next('v')
    expect(badge()).toMatch(/^Вписано/)
    expect(frame().classList.contains('zoomed')).toBe(false)
    next('c')
    expect(badge()).toMatch(/^Вписано/)
  })

  it('пока грузится соседний кадр, рамка не прыгает во «вписано»', () => {
    open()
    click()
    const before = frame().style.width
    photoui.step(1)
    flushSync() // новая картинка ещё не загрузилась
    expect(frame().style.width).toBe(before)
    fireEvent.load(img())
    flushSync()
    expect(badge()).toBe('1:1')
  })

  it('закрыли и открыли снова — вписано', () => {
    open()
    click()
    photoui.closePreview()
    flushSync()
    photoui.openLibrary('a')
    loaded()
    expect(badge()).toMatch(/^Вписано/)
  })
})

describe('фон и подпись', () => {
  it('нажатие на пустое место окна вокруг вписанного кадра закрывает его; по кадру — нет', () => {
    open('v')
    fireEvent.pointerDown(img(), { clientX: 600, clientY: 300, button: 0, pointerId: 1 })
    expect(photoui.preview).not.toBeNull()
    fireEvent.pointerDown(document.querySelector('.view')!, { clientX: 5, clientY: 5, button: 0 })
    expect(photoui.preview).toBeNull()
  })

  it('значок зума стоит в подписи перед ✕, и с холста тоже', () => {
    render(PhotoPreview)
    photoui.openCanvas('a')
    loaded()
    const cap = document.querySelector('.cap')!
    expect([...cap.children].map((c) => ['name', 'zoom', 'x'].find((n) => c.classList.contains(n)))).toEqual(['name', 'zoom', 'x'])
  })

  it('строка клавиш из ленты: клавиши из реестра; Mac — ⌘', () => {
    open()
    const hint = () => document.querySelector('.hint')?.textContent
    expect(hint()).toBe(
      '← → — листать · щелчок или Z — 1:1 · Ctrl+колесо или щипок — зум · 0 — вписать · отметки — клавишами ленты · C — сравнить · Esc — закрыть',
    )
    cleanup()
    setOsForTest('mac')
    open()
    expect(hint()).toContain('⌘+колесо или щипок — зум')
    expect(keymap.hint('fitZoom')).toBe('0')
  })

  it('строка клавиш с холста — без «листать» и «отметки»', () => {
    render(PhotoPreview)
    photoui.openCanvas('a')
    loaded()
    expect(document.querySelector('.hint')?.textContent).toBe(
      'щелчок или Z — 1:1 · Ctrl+колесо или щипок — зум · 0 — вписать · Esc — закрыть',
    )
  })

  it('клавиша снята — в строке её нет', () => {
    useCatalog({ photoPreview: [], fitZoom: [] })
    render(PhotoPreview)
    photoui.openCanvas('a')
    loaded()
    expect(document.querySelector('.hint')?.textContent).toBe('щелчок — 1:1 · Ctrl+колесо или щипок — зум · Esc — закрыть')
  })
})

describe('мелкое фото и зона', () => {
  it('картинка меньше окна не растягивается: вписано = 1:1, щелчок ничего не обещает и не меняет', () => {
    store.photos = [...store.photos, { id: 's', name: 's.jpg', w: 4, h: 3 }] as never
    open('s')
    expect(w()).toBe(400)
    expect(badge()).toBe('1:1')
    expect(frame().classList.contains('canzoom')).toBe(false)
    click()
    expect(w()).toBe(400)
    expect(frame().classList.contains('zoomed')).toBe(false)
    const e = new KeyboardEvent('keydown', { code: 'Equal', key: '=', ctrlKey: true, bubbles: true, cancelable: true })
    document.querySelector('[role=dialog]')!.dispatchEvent(e)
    flushSync()
    expect(w()).toBe(400)
  })

  it('область кадра не .stage, нажатие в окне не меняет зону программы (после закрытия X и 1–5 работают)', () => {
    open()
    expect(document.querySelector('.stage')).toBeNull()
    store.zone = 'library'
    for (const el of [document.querySelector('.view')!, img(), document.querySelector('.cap')!]) {
      expect(zoneOf(el)).toBeNull()
    }
    // и под предком .stage (окно не обязано лежать вне холста по DOM)
    const wrap = document.createElement('div')
    wrap.className = 'stage'
    document.body.appendChild(wrap)
    wrap.appendChild(document.querySelector('.back')!)
    expect(zoneOf(document.querySelector('.view'))).toBeNull()
    wrap.remove()
  })

  it('строка клавиш: «отметки — клавишами ленты» только при включённых отметках', () => {
    prefs.level = 'custom'
    prefs.custom = { base: 'simple', on: [], off: ['marks'] }
    expect(prefs.can('marks')).toBe(false)
    open()
    expect(document.querySelector('.hint')?.textContent).toBe(
      '← → — листать · щелчок или Z — 1:1 · Ctrl+колесо или щипок — зум · 0 — вписать · C — сравнить · Esc — закрыть',
    )
  })
})
