// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Оригинал при зуме выше 1:1 по превью (128-14): /media/o только выше 1:1, Adobe и > 100 Мп не запрошены (Т6),
// error и другие пропорции — превью, при листании src прежнего снят, запрос после паузы зума.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { default: PhotoPreview } = await import('./PhotoPreview.svelte')
const { photoui } = await import('./photoui.svelte')
const { store } = await import('../../base/core/store.svelte')
const { useCatalog, dropCatalog } = await import('../../base/keys/keymap.fixture')
const { ORIG_DELAY } = await import('./original.svelte')

// Превью 2048×1365; оригинал a, d вдвое больше (max = 2), у e вертикальный (WebView повернул иначе).
const PREVIEW: [number, number] = [2048, 1365]
const orig: Record<string, [number, number]> = { a: [4096, 2730], d: [4096, 2730], e: [1365, 2048] }
const sizeOf = (el: HTMLImageElement): [number, number] => {
  const src = el.getAttribute('src') ?? ''
  const id = src.match(/\/o\/(\w+)/)?.[1]
  return id ? (orig[id] ?? PREVIEW) : PREVIEW
}

beforeEach(() => {
  vi.useFakeTimers()
  useCatalog()
  vi.spyOn(HTMLImageElement.prototype, 'naturalWidth', 'get').mockImplementation(function (this: HTMLImageElement) {
    return sizeOf(this)[0]
  })
  vi.spyOn(HTMLImageElement.prototype, 'naturalHeight', 'get').mockImplementation(function (this: HTMLImageElement) {
    return sizeOf(this)[1]
  })
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(1200)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(700)
  store.view = { ...store.view, rev: 1, spreads: [], folders: [], missing: [] } as never
  store.photos = [
    { id: 'a', name: 'a.jpg', w: 4096, h: 2730 },
    { id: 'd', name: 'd.jpg', w: 4096, h: 2730 },
    { id: 'e', name: 'e.jpg', w: 4096, h: 2730 },
    { id: 'adobe', name: 'adobe.jpg', w: 4096, h: 2730, color: 'adobe' },
    { id: 'p3', name: 'p3.jpg', w: 4096, h: 2730, color: 'p3' },
    { id: 'big', name: 'big.jpg', w: 12000, h: 9000 },
    { id: 'srgb', name: 'srgb.jpg', w: 4096, h: 2730, color: 'srgb' },
  ] as never
  store.selected.clear()
})
afterEach(() => {
  cleanup()
  photoui.preview = null
  dropCatalog()
  vi.restoreAllMocks()
  vi.useRealTimers()
})

const origImg = () => document.querySelector<HTMLImageElement>('img.orig')
const origSrcs = () => [...document.querySelectorAll<HTMLImageElement>('img.orig')].map((i) => i.getAttribute('src'))
const badge = () => document.querySelector('.cap .zoom')?.textContent
const note = () => document.querySelector('.cap .note')?.textContent
const frame = () => document.querySelector<HTMLElement>('.frame')!
const w = () => parseFloat(frame().style.width)
const tick = (ms: number) => {
  vi.advanceTimersByTime(ms)
  flushSync()
}
const loadPreview = () => {
  flushSync()
  fireEvent.load(document.querySelector('img.ph:not(.orig)')!)
  flushSync()
}
const open = (id = 'a') => {
  render(PhotoPreview)
  photoui.openLibrary(id)
  loadPreview()
}
const wheel = (deltaY: number) => {
  const e = Object.assign(new Event('wheel', { bubbles: true, cancelable: true }), {
    clientX: 600,
    clientY: 350,
    deltaMode: 0,
    ctrlKey: true,
    deltaY,
  })
  document.querySelector('.view')!.dispatchEvent(e)
  flushSync()
}
const zoomIn = (n: number) => {
  for (let i = 0; i < n; i++) wheel(-100)
}
const key = (code: string, init: KeyboardEventInit = {}) => {
  document
    .querySelector('[role=dialog]')!
    .dispatchEvent(new KeyboardEvent('keydown', { code, key: init.key ?? code, bubbles: true, cancelable: true, ...init }))
  flushSync()
}
const click = () => {
  const at = { clientX: 650, clientY: 350, button: 0, pointerId: 1 }
  fireEvent.pointerDown(document.querySelector('img.ph')!, at)
  fireEvent.pointerUp(document.querySelector('img.ph')!, at)
  flushSync()
}
const loadOrig = () => {
  fireEvent.load(origImg()!)
  flushSync()
}
const step = (id: string) => {
  photoui.preview = {
    ids: ['a', 'd', 'e', 'adobe', 'p3', 'big', 'srgb'],
    at: ['a', 'd', 'e', 'adobe', 'p3', 'big', 'srgb'].indexOf(id),
    from: 'library',
  }
  loadPreview()
}

describe('оригинал только выше 1:1', () => {
  it('вписано, щелчок и Z (1:1 по превью) оригинал не запрашивают, сколько ни жди', () => {
    open()
    tick(ORIG_DELAY * 3)
    expect(origImg()).toBeNull()
    click()
    expect(badge()).toBe('1:1')
    key('KeyZ')
    key('KeyZ')
    tick(ORIG_DELAY * 3)
    expect(origImg()).toBeNull()
  })

  it('Ctrl+колесо выше 1:1: растянутое превью, «Оригинал загружается…», после паузы — /media/o/<id>', () => {
    open()
    zoomIn(30)
    expect(w()).toBeGreaterThan(2048)
    expect(badge()).toBe('Оригинал загружается…')
    expect(origImg()).toBeNull() // зум ещё не устоялся
    tick(ORIG_DELAY)
    expect(origSrcs()).toEqual(['/media/o/a?v=5'])
    expect(origImg()!.classList.contains('ready')).toBe(false) // скрыт до load
    expect(badge()).toBe('Оригинал загружается…')
    loadOrig()
    expect(origImg()!.classList.contains('ready')).toBe(true)
    expect(badge()).toBe('200 %') // предел — 1:1 по оригиналу: вдвое больше превью
    expect(w()).toBe(4096)
  })

  it('Mod+= выше 1:1 ведёт к оригиналу так же, как колесо', () => {
    open()
    for (let i = 0; i < 20; i++) key('Equal', { ctrlKey: true })
    tick(ORIG_DELAY)
    expect(origSrcs()).toEqual(['/media/o/a?v=5'])
  })

  it('зум назад на 1:1 после загрузки: оригинал остаётся, повторного запроса нет', () => {
    open()
    zoomIn(30)
    tick(ORIG_DELAY)
    loadOrig()
    for (let i = 0; i < 30; i++) wheel(100)
    zoomIn(30)
    tick(ORIG_DELAY * 2)
    expect(origSrcs()).toEqual(['/media/o/a?v=5'])
  })
})

describe('пауза зума (debounce)', () => {
  it('шаги колеса короче паузы запрос откладывают: он уходит один, через паузу после последнего шага', () => {
    open()
    zoomIn(2) // 113 %: выше 1:1, до предела 2:1 далеко
    expect(w()).toBeGreaterThan(2048)
    for (let i = 0; i < 4; i++) {
      tick(ORIG_DELAY - 50)
      expect(origImg()).toBeNull()
      wheel(-10) // мелкий шаг: s остаётся ниже предела и меняется
    }
    expect(w()).toBeLessThan(4096)
    tick(ORIG_DELAY - 1)
    expect(origImg()).toBeNull()
    tick(1)
    expect(origSrcs()).toEqual(['/media/o/a?v=5'])
  })

  it('зум вернули на 1:1 до паузы — запроса нет', () => {
    open()
    zoomIn(2)
    for (let i = 0; i < 30; i++) wheel(100)
    tick(ORIG_DELAY * 3)
    expect(origImg()).toBeNull()
  })
})

describe('оригинал не запрошен, Т6', () => {
  const toLimit = () => {
    zoomIn(40)
    tick(ORIG_DELAY * 3)
  }
  it('профиль Adobe: зум упирается в 1:1 по превью, значок «1:1» и причина, /media/o нет', () => {
    open('adobe')
    toLimit()
    expect(origImg()).toBeNull()
    expect(w()).toBe(2048)
    expect(badge()).toBe('1:1')
    expect(note()).toBe('Крупнее превью не показывается: профиль Adobe RGB')
  })
  it('больше 100 Мп: то же с причиной «больше 100 Мп»', () => {
    open('big')
    toLimit()
    expect(origImg()).toBeNull()
    expect(w()).toBe(2048)
    expect(note()).toBe('Крупнее превью не показывается: больше 100 Мп')
  })
  it('Display P3 — тоже без оригинала, своя причина', () => {
    open('p3')
    toLimit()
    expect(origImg()).toBeNull()
    expect(note()).toBe('Крупнее превью не показывается: профиль Display P3')
  })
  it('причины нет, пока кадр вписан; у sRGB её нет совсем', () => {
    open('adobe')
    expect(note()).toBeUndefined()
    cleanup()
    open('srgb')
    toLimit()
    expect(note()).toBeUndefined()
    expect(origSrcs()).toEqual(['/media/o/srgb?v=5'])
  })
})

describe('оригинал не годится — остаётся превью', () => {
  it('error (404, 415): оригинал снят, зум вернулся к 1:1 по превью, ошибки и причины нет', () => {
    open()
    zoomIn(30)
    tick(ORIG_DELAY)
    fireEvent.error(origImg()!)
    flushSync()
    expect(origImg()).toBeNull()
    expect(w()).toBe(2048)
    expect(badge()).toBe('1:1')
    expect(note()).toBeUndefined()
    zoomIn(10)
    tick(ORIG_DELAY * 3)
    expect(origImg()).toBeNull() // повторно не просим
    expect(w()).toBe(2048)
  })

  it('пропорции оригинала не совпали с превью: не показывается, предел 1:1', () => {
    open('e')
    zoomIn(30)
    tick(ORIG_DELAY)
    loadOrig()
    expect(origImg()).toBeNull()
    expect(w()).toBe(2048)
    expect(badge()).toBe('1:1')
  })

  it('пропорции в пределах 1 % принимаются', () => {
    orig.a = [4096, 2736]
    open()
    zoomIn(30)
    tick(ORIG_DELAY)
    loadOrig()
    expect(origImg()!.classList.contains('ready')).toBe(true)
    orig.a = [4096, 2730]
  })
})

describe('листание', () => {
  it('src прежнего оригинала снят раньше, чем запрошен следующий; одна <img> за раз', () => {
    open()
    zoomIn(30)
    tick(ORIG_DELAY)
    const first = origImg()!
    loadOrig()
    step('d')
    expect(first.getAttribute('src')).toBeNull()
    expect(origSrcs()).toEqual([]) // у нового фото запрос ещё не ушёл: зум устаивается заново
    expect(badge()).toBe('Оригинал загружается…') // зум 2:1 перенесён, оригинал для d ждёт паузы
    tick(ORIG_DELAY)
    expect(origSrcs()).toEqual(['/media/o/d?v=5'])
    step('a')
    expect(origSrcs()).toEqual([])
  })

  it('закрытие окна снимает оригинал, следующее открытие — вписано и без запросов', () => {
    open()
    zoomIn(30)
    tick(ORIG_DELAY)
    const first = origImg()!
    photoui.closePreview()
    flushSync()
    expect(first.getAttribute('src')).toBeNull()
    photoui.openLibrary('a')
    loadPreview()
    tick(ORIG_DELAY * 3)
    expect(origImg()).toBeNull()
  })

  it('листание на фото с профилем Adobe: зум ужимается до 1:1 по превью, оригинал не просим', () => {
    open()
    zoomIn(30)
    tick(ORIG_DELAY)
    loadOrig()
    step('adobe')
    tick(ORIG_DELAY * 3)
    expect(origImg()).toBeNull()
    expect(w()).toBe(2048)
    expect(note()).toBe('Крупнее превью не показывается: профиль Adobe RGB')
  })
})
