// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Крупный просмотр плитки окон оформления (133 U2, вариант Б вопроса 2): правый щелчок и Ctrl+правый — просмотр на любом уровне,
// ← → по видимым, Enter — один pick, сбой картинки — миниатюра, Esc — плитка в фокусе, предмет пропал — pick нет. Go подменён.
const api = vi.hoisted(() => ({
  DecorCatalog: vi.fn(),
  LibraryPacks: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))

const { default: DecorDialog } = await import('../decor/DecorDialog.svelte')
const { decor } = await import('../decor/decor.svelte')
const { tilePx } = await import('../decor/decor')
const { store } = await import('../../base/core/store.svelte')
const { modes } = await import('../../base/prefs/view.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { modalStack } = await import('../../base/modal/modalstack.svelte')
const { screenDpr } = await import('../../base/ui/dpr.svelte')
const { useCatalog, dropCatalog } = await import('../../base/keys/keymap.fixture')
const { peek } = await import('./peek.svelte')
const { setOsForTest } = await import('../../base/core/platform')

type Item = Record<string, unknown>
const item = (id: string, over: Item = {}) => ({
  id,
  theme: 'a',
  name: id,
  url: `/media/gen/art?k=${id}&v=1`,
  own: false,
  aspect: 0,
  w: 0,
  h: 0,
  ...over,
})
const ITEMS = [
  item('sa:x', { name: 'Звезда' }),
  item('my:a', { name: 'Логотип', url: '/media/lib/t/aa?v=3', peek: '/media/lib/p/aa?v=3', own: true, theme: 'mine', w: 800, h: 400 }),
  item('pk:1', { name: 'Колокольчик', url: '/media/lib/t/bb?v=1', peek: '/media/lib/p/bb?v=1', pack: 'Школа', packId: 'p1', theme: 'b' }),
  item('pk:2', { name: 'Ромашка', url: '/media/lib/t/cc?v=1', peek: '/media/lib/p/cc?v=1', pack: 'Школа', packId: 'p1', theme: 'b' }),
  item('pk:3', { name: 'Тюльпан', url: '/media/lib/t/dd?v=1', peek: '/media/lib/p/dd?v=1', pack: 'Школа', packId: 'p1', theme: 'b' }),
]
const cat = (kind: string, items: Item[] = ITEMS, readOnly = false) => ({
  kind,
  themes: [
    { id: 'all', name: 'Все', count: items.length },
    { id: 'b', name: 'Школа', count: 3 },
  ],
  items,
  recent: [],
  readOnly,
})
const sheet = (id: string) => ({
  id,
  photos: [],
  stickers: [{ id: 'sticker-1' }], // заменяемый в тесте режима замены
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
})
type Kind = 'bg' | 'sticker' | 'frame' | 'spread'
const open = async (kind: Kind, items: Item[] = ITEMS, readOnly = false, replace?: string) => {
  api.DecorCatalog.mockResolvedValue(cat(kind, items, readOnly))
  decor.show(kind, 's1', 'spread', replace)
  await vi.waitFor(() => expect(decor.cat?.kind).toBe(kind))
  flushSync()
}

const tileBtn = (id: string) => document.querySelector<HTMLElement>(`[data-decor-id="${id}"]`)!
const view = () => document.querySelector<HTMLElement>('[aria-label="Крупный просмотр"]')
const frame = () => document.querySelector<HTMLElement>('.frame')!
// Крупная — последняя картинка в рамке (под ней, пока грузится, миниатюра)
const bigImg = () => Array.from(document.querySelectorAll<HTMLImageElement>('.frame img.ph')).at(-1)!
const caption = () => document.querySelector('.cap .name')?.textContent
const hint = () => document.querySelector('.hint')?.textContent ?? ''
const put = () => document.querySelector<HTMLButtonElement>('.cap .primary')!
const sizes: Record<string, [number, number]> = { aa: [1600, 800], bb: [900, 900], cc: [900, 900], dd: [900, 900] }
const natOf = (el: HTMLImageElement) => sizes[el.getAttribute('src')?.match(/\/p\/(\w+)/)?.[1] ?? 'bb'] ?? [900, 900]

// Правый щелчок по плитке; Ctrl — тот же жест с модификатором.
function rclick(id: string, init: MouseEventInit = {}) {
  const e = new MouseEvent('contextmenu', { bubbles: true, cancelable: true, button: 2, ...init })
  tileBtn(id).dispatchEvent(e)
  flushSync()
  return e
}
function key(k: string, init: KeyboardEventInit = {}) {
  const e = new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true, ...init })
  ;(view() ?? document.body).dispatchEvent(e)
  flushSync()
  return e
}
const loadBig = () => {
  flushSync()
  fireEvent.load(bigImg())
  flushSync()
}
const names = () => (document.querySelector('.cap .name')?.textContent ?? '').split(' · ')[0]

let pick: ReturnType<typeof vi.spyOn>
beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.LibraryPacks.mockResolvedValue({ packs: [], net: {}, busy: false, readOnly: false })
  useCatalog()
  localStorage.clear()
  modes.decorTile = {}
  prefs.level = 'simple'
  prefs.custom = null
  decor.close()
  decor.cat = null
  decor.theme = ''
  decor.busy = false
  peek.close()
  store.view = { rev: 1, folders: [], settings: { pageW: 300, pageH: 300 }, spreads: [sheet('s1')] } as never
  vi.spyOn(HTMLImageElement.prototype, 'naturalWidth', 'get').mockImplementation(function (this: HTMLImageElement) {
    return natOf(this)[0]
  })
  vi.spyOn(HTMLImageElement.prototype, 'naturalHeight', 'get').mockImplementation(function (this: HTMLImageElement) {
    return natOf(this)[1]
  })
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(1200)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(700)
  pick = vi.spyOn(decor, 'pick').mockResolvedValue(undefined)
  render(DecorDialog)
})
afterEach(() => {
  cleanup()
  decor.close()
  dropCatalog()
  setOsForTest(null)
  vi.restoreAllMocks()
})

describe('133-7 (вариант Б): правый щелчок — сразу просмотр', () => {
  it('ПКМ на «Простом» уровне (меню по правой кнопке нет) открывает просмотр и не ставит плитку', async () => {
    await open('sticker')
    expect(prefs.can('photoMenu')).toBe(false)
    const e = rclick('sa:x')
    expect(e.defaultPrevented).toBe(true) // системное меню не всплывает
    expect(view()).not.toBeNull()
    expect(pick).not.toHaveBeenCalled()
    expect(caption()).toBe('Звезда · 1 из 5')
  })

  it('Ctrl+ПКМ — то же', async () => {
    await open('sticker')
    rclick('pk:1', { ctrlKey: true })
    expect(view()).not.toBeNull()
    expect(pick).not.toHaveBeenCalled()
  })

  it('при «только чтение» просмотр тоже открывается', async () => {
    await open('sticker', ITEMS, true)
    rclick('sa:x')
    expect(view()).not.toBeNull()
  })

  it('щелчок по плитке по-прежнему ставит, Ctrl+щелчок на Mac (правый щелчок) — нет', async () => {
    await open('sticker')
    await fireEvent.click(tileBtn('sa:x'))
    expect(pick).toHaveBeenCalledTimes(1)
    setOsForTest('mac')
    await fireEvent.click(tileBtn('sa:x'), { ctrlKey: true })
    expect(pick).toHaveBeenCalledTimes(1)
    await fireEvent.click(tileBtn('sa:x'))
    expect(pick).toHaveBeenCalledTimes(2)
  })

  it('на Windows и Linux Ctrl+щелчок ставит, как раньше', async () => {
    await open('sticker')
    setOsForTest('windows')
    await fireEvent.click(tileBtn('sa:x'), { ctrlKey: true })
    expect(pick).toHaveBeenCalledTimes(1)
    setOsForTest('linux')
    await fireEvent.click(tileBtn('sa:x'), { ctrlKey: true })
    expect(pick).toHaveBeenCalledTimes(2)
  })

  it('после закрытия просмотра в стеке одно окно, после закрытия окна — ни одного', async () => {
    await open('sticker')
    const base = modalStack.count()
    rclick('sa:x')
    expect(modalStack.count()).toBe(base + 1)
    key('Escape')
    flushSync()
    expect(view()).toBeNull()
    expect(modalStack.count()).toBe(base)
    decor.close()
    flushSync()
    expect(modalStack.count()).toBe(0)
  })

  it('окно закрыли, пока просмотр открыт, — запись просмотра из стека снята', async () => {
    await open('sticker')
    rclick('sa:x')
    decor.close()
    flushSync()
    expect(view()).toBeNull()
    expect(modalStack.count()).toBe(0)
    // окно открыли снова: просмотр не возвращается сам (состояние снято при закрытии окна)
    await open('sticker')
    expect(view()).toBeNull()
    expect(peek.id).toBe('')
  })

  it('подсказка плитки: вторая строка «Правый щелчок — крупно»', async () => {
    await open('sticker')
    expect(tileBtn('sa:x').title).toBe('Звезда\nПравый щелчок — крупно')
    expect(tileBtn('pk:1').title).toBe('Колокольчик · набор «Школа»\nПравый щелчок — крупно')
  })
})

describe('подложка, закрытие правым щелчком, фокус', () => {
  it('фон: цвет страниц пресета подложкой, как у плитки; рамочка без цвета — без своей подложки', async () => {
    const bg = [item('sa:bg', { name: 'Акварель', color: '#ffeedd' }), item('sa:plain', { name: 'Без цвета' })]
    await open('bg', bg)
    rclick('sa:bg')
    expect(frame().style.background).toBe(tileBtn('sa:bg').style.background)
    expect(frame().style.background).not.toBe('')
    key('Escape')
    flushSync()
    rclick('sa:plain')
    expect(frame().style.background).toBe('')
  })

  it('правая кнопка вниз по фону не закрывает; contextmenu закрывает и не открывает просмотр снова', async () => {
    await open('sticker')
    rclick('sa:x')
    const back = view()!.parentElement!
    await fireEvent.pointerDown(back, { button: 2 })
    expect(view()).not.toBeNull()
    const e = new MouseEvent('contextmenu', { bubbles: true, cancelable: true, button: 2 })
    back.dispatchEvent(e)
    flushSync()
    expect(e.defaultPrevented).toBe(true)
    expect(view()).toBeNull()
    expect(pick).not.toHaveBeenCalled()
  })

  it('правый щелчок по кадру тоже закрывает (всплывает до фона); Ctrl+щелчок на Mac вниз просмотр не закрывает', async () => {
    await open('sticker')
    rclick('sa:x')
    setOsForTest('mac')
    await fireEvent.pointerDown(view()!.parentElement!, { button: 0, ctrlKey: true })
    expect(view()).not.toBeNull()
    view()!
      .querySelector('.frame')!
      .dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, button: 0, ctrlKey: true }))
    flushSync()
    expect(view()).toBeNull()
  })

  it('фокус ушёл к плитке под просмотром (Shift+Tab): Enter её не ставит, фокус возвращается в окно просмотра', async () => {
    await open('sticker')
    rclick('sa:x')
    const t = tileBtn('pk:1')
    t.focus()
    const e = new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })
    t.dispatchEvent(e)
    flushSync()
    expect(pick).not.toHaveBeenCalled()
    expect(e.defaultPrevented).toBe(true)
    expect(view()).not.toBeNull()
    expect(document.activeElement).toBe(document.querySelector('[aria-label="Крупный просмотр"]'))
  })
})

describe('адреса картинок', () => {
  it('своё и набор — peek из Go без px; встроенное — px по большей стороне окна', async () => {
    await open('sticker')
    rclick('my:a')
    expect(bigImg().getAttribute('src')).toBe('/media/lib/p/aa?v=3')
    key('Escape')
    flushSync()
    rclick('sa:x')
    const px = tilePx(Math.max(window.innerWidth, window.innerHeight), screenDpr.value)
    expect(bigImg().getAttribute('src')).toBe(`/media/gen/art?k=sa:x&v=1&px=${px}`)
  })

  it('пока крупная грузится, под ней миниатюра плитки (тот же адрес, что у плитки); загрузилась — миниатюры нет', async () => {
    await open('sticker')
    rclick('my:a')
    const tileSrc = tileBtn('my:a').querySelector('img')!.getAttribute('src')
    const thumbs = () => Array.from(document.querySelectorAll('.frame img.ph'), (i) => i.getAttribute('src'))
    expect(thumbs()).toEqual([tileSrc, '/media/lib/p/aa?v=3'])
    expect(document.querySelector('.frame img.wait')).not.toBeNull()
    loadBig()
    expect(thumbs()).toEqual(['/media/lib/p/aa?v=3'])
  })

  it('развороты — /media/tpl с px ≤ 1024 и r, без лупы', async () => {
    const tpl = [item('t1', { name: 'Школа', url: '/media/tpl/t1?v=1', pages: 2, theme: 'b' })]
    await open('spread', tpl)
    rclick('t1')
    expect(bigImg().getAttribute('src')).toMatch(/^\/media\/tpl\/t1\?v=1&px=\d+&r=[\d.]+$/)
    expect(
      Number(
        bigImg()
          .getAttribute('src')!
          .match(/px=(\d+)/)![1],
      ),
    ).toBeLessThanOrEqual(1024)
    loadBig()
    expect(hint()).toBe('← → — соседние · Enter — поставить · Esc — назад')
    expect(frame().classList.contains('canzoom')).toBe(false)
  })
})

describe('листание ← →', () => {
  it('по видимым плиткам темы; на краю ничего', async () => {
    await open('sticker')
    decor.setTheme('b')
    flushSync()
    rclick('pk:1')
    expect(caption()).toBe('Колокольчик · набор «Школа» · 1 из 3')
    key('ArrowLeft')
    expect(names()).toBe('Колокольчик')
    key('ArrowRight')
    key('ArrowRight')
    expect(caption()).toBe('Тюльпан · набор «Школа» · 3 из 3')
    key('ArrowRight')
    expect(names()).toBe('Тюльпан')
  })

  it('быстрые ← → снимают запрос прежней картинки (src убран у снятого узла)', async () => {
    await open('sticker')
    decor.setTheme('b')
    flushSync()
    rclick('pk:1')
    const first = bigImg()
    expect(first.getAttribute('src')).toBe('/media/lib/p/bb?v=1')
    key('ArrowRight')
    expect(first.hasAttribute('src')).toBe(false)
    const second = bigImg()
    key('ArrowRight')
    expect(second.hasAttribute('src')).toBe(false)
    expect(bigImg().getAttribute('src')).toBe('/media/lib/p/dd?v=1')
  })

  it('своё в подписи — «свой», встроенное — без второй части', async () => {
    await open('sticker')
    rclick('my:a')
    expect(caption()).toBe('Логотип · свой · 2 из 5')
    key('ArrowLeft')
    expect(caption()).toBe('Звезда · 1 из 5')
  })

  it('с Ctrl/Alt/Shift стрелки не листают', async () => {
    await open('sticker')
    rclick('sa:x')
    key('ArrowRight', { ctrlKey: true })
    expect(names()).toBe('Звезда')
  })
})

describe('сбой картинки', () => {
  it('ошибка загрузки — остаётся миниатюра, лупы нет, подпись прежняя', async () => {
    await open('sticker')
    rclick('my:a')
    flushSync()
    fireEvent.error(bigImg())
    flushSync()
    const imgs = Array.from(document.querySelectorAll('.frame img.ph'))
    expect(imgs.map((i) => i.getAttribute('src'))).toEqual([tileBtn('my:a').querySelector('img')!.getAttribute('src')])
    expect(frame().classList.contains('canzoom')).toBe(false)
    expect(hint()).not.toContain('1:1')
    expect(caption()).toBe('Логотип · свой · 2 из 5')
    expect(document.querySelector('.toast')).toBeNull() // без тоста
  })

  it('на следующей плитке сбой не остаётся', async () => {
    await open('sticker')
    rclick('my:a')
    fireEvent.error(bigImg())
    flushSync()
    key('ArrowLeft')
    expect(document.querySelectorAll('.frame img.ph').length).toBe(2) // миниатюра и крупная
    expect(hint()).toContain('1:1')
  })
})

describe('лупа', () => {
  it('щелчок по кадру — 1:1, второй — вписано; Ctrl+колесо подключено через loupe.bind', async () => {
    await open('sticker')
    rclick('my:a')
    loadBig()
    const w0 = parseFloat(frame().style.width)
    expect(frame().classList.contains('canzoom')).toBe(true)
    const at = { clientX: 600, clientY: 350, button: 0, pointerId: 1 }
    fireEvent.pointerDown(bigImg(), at)
    fireEvent.pointerUp(bigImg(), at)
    flushSync()
    expect(parseFloat(frame().style.width)).toBeGreaterThan(w0) // 1600 px при кадре 1200
    fireEvent.pointerDown(bigImg(), at)
    fireEvent.pointerUp(bigImg(), at)
    flushSync()
    expect(parseFloat(frame().style.width)).toBeCloseTo(w0, 3)
    const back = view()!.parentElement!
    const wheel = new WheelEvent('wheel', { bubbles: true, cancelable: true, deltaY: -100, clientX: 600, clientY: 350 })
    Object.defineProperty(wheel, 'ctrlKey', { value: true })
    back.dispatchEvent(wheel)
    expect(wheel.defaultPrevented).toBe(true)
    flushSync()
    expect(parseFloat(frame().style.width)).toBeGreaterThan(w0)
  })

  it('клавиша «Крупно» (Z) внутри просмотра — 1:1 и обратно; автоповтор не мигает', async () => {
    await open('sticker')
    rclick('my:a')
    loadBig()
    const w0 = parseFloat(frame().style.width)
    const z = (repeat = false) =>
      view()!.dispatchEvent(new KeyboardEvent('keydown', { code: 'KeyZ', key: 'z', repeat, bubbles: true, cancelable: true }))
    z()
    flushSync()
    expect(parseFloat(frame().style.width)).toBeGreaterThan(w0)
    z(true)
    flushSync()
    expect(parseFloat(frame().style.width)).toBeGreaterThan(w0)
    z()
    flushSync()
    expect(parseFloat(frame().style.width)).toBeCloseTo(w0, 3)
  })

  it('шашечка у стикера; у рамочки не своей её нет', async () => {
    await open('sticker')
    rclick('sa:x')
    expect(frame().classList.contains('chk')).toBe(true)
    key('Escape')
    flushSync()
    decor.close()
    flushSync()
    await open('frame')
    rclick('sa:x')
    expect(frame().classList.contains('chk')).toBe(false)
  })
})

describe('Enter, «Поставить», Esc', () => {
  it('Enter — один pick, просмотр закрыт', async () => {
    await open('sticker')
    rclick('my:a')
    key('Enter')
    expect(pick).toHaveBeenCalledTimes(1)
    expect(pick.mock.calls[0][0]).toMatchObject({ id: 'my:a' })
    expect(view()).toBeNull()
  })

  it('автоповтор Enter не ставит', async () => {
    await open('sticker')
    rclick('my:a')
    key('Enter', { repeat: true })
    expect(pick).not.toHaveBeenCalled()
    expect(view()).not.toBeNull()
  })

  it('во время загрузки своих (busy) Enter и кнопка не ставят', async () => {
    await open('sticker')
    rclick('my:a')
    decor.busy = true
    flushSync()
    key('Enter')
    expect(pick).not.toHaveBeenCalled()
    expect(put().disabled).toBe(true)
    expect(view()).not.toBeNull()
  })

  it('кнопка «Поставить» — тот же pick; в режиме замены она «Заменить»', async () => {
    await open('sticker')
    rclick('my:a')
    expect(put().textContent).toBe('Поставить')
    await fireEvent.click(put())
    expect(pick).toHaveBeenCalledTimes(1)
    decor.close()
    flushSync()
    await open('sticker', ITEMS, false, 'sticker-1')
    rclick('my:a')
    expect(put().textContent).toBe('Заменить')
    expect(hint()).toContain('Enter — заменить')
  })

  it('режим замены: стикер убрали отменой — просмотр говорит «Поставить», вернули — «Заменить» (0.28-133-78)', async () => {
    await open('sticker', ITEMS, false, 'sticker-1')
    rclick('my:a')
    expect(put().textContent).toBe('Заменить')
    const withSt = store.view
    store.view = { ...withSt, rev: 2, spreads: [{ ...withSt.spreads[0], stickers: [] }] } as never
    flushSync()
    expect(put().textContent).toBe('Поставить')
    expect(hint()).toContain('Enter — поставить')
    store.view = withSt
    flushSync()
    expect(put().textContent).toBe('Заменить')
  })

  it('Enter на кнопке ✕ не ставит (это щелчок по ✕)', async () => {
    await open('sticker')
    rclick('my:a')
    const x = document.querySelector<HTMLElement>('.cap .x')!
    x.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
    flushSync()
    expect(pick).not.toHaveBeenCalled()
  })

  it('Esc — назад: окно оформления открыто, плитка в фокусе, pick нет', async () => {
    await open('sticker')
    rclick('pk:2')
    const e = key('Escape')
    expect(e.defaultPrevented).toBe(true)
    await vi.waitFor(() => expect(document.activeElement).toBe(tileBtn('pk:2')))
    expect(view()).toBeNull()
    expect(document.querySelector('.tiles')).not.toBeNull()
    expect(pick).not.toHaveBeenCalled()
  })

  it('✕ и щелчок по фону возвращают к плитке той карточки, на которой листали', async () => {
    await open('sticker')
    rclick('pk:1')
    key('ArrowRight')
    await fireEvent.click(document.querySelector('.cap .x')!)
    await vi.waitFor(() => expect(document.activeElement).toBe(tileBtn('pk:2')))
    rclick('pk:3')
    await fireEvent.pointerDown(view()!.parentElement!)
    await vi.waitFor(() => expect(document.activeElement).toBe(tileBtn('pk:3')))
    expect(view()).toBeNull()
  })

  it('клавиши просмотра не доходят до окна оформления: Esc не закрывает его', async () => {
    await open('sticker')
    rclick('sa:x')
    key('Escape')
    await vi.waitFor(() => expect(document.activeElement).toBe(tileBtn('sa:x')))
    expect(decor.cur).not.toBeNull()
  })
})

describe('предмет пропал', () => {
  it('убрали из каталога — просмотр закрывается, pick не зовётся, и Enter после этого тоже', async () => {
    await open('sticker')
    rclick('my:a')
    decor.cat = cat(
      'sticker',
      ITEMS.filter((i) => i.id !== 'my:a'),
    ) as never
    flushSync()
    expect(view()).toBeNull()
    expect(modalStack.count()).toBe(1)
    key('Enter')
    expect(pick).not.toHaveBeenCalled()
    // предмет вернулся в каталог: просмотр сам не открывается (состояние сброшено, а не спрятано)
    decor.cat = cat('sticker', ITEMS) as never
    flushSync()
    expect(view()).toBeNull()
    expect(peek.id).toBe('')
  })

  it('пропал сосед — ← → его пропускают, число «из M» уменьшается', async () => {
    await open('sticker')
    rclick('sa:x')
    decor.cat = cat(
      'sticker',
      ITEMS.filter((i) => i.id !== 'my:a'),
    ) as never
    flushSync()
    expect(caption()).toBe('Звезда · 1 из 4')
    key('ArrowRight')
    expect(names()).toBe('Колокольчик')
  })
})
