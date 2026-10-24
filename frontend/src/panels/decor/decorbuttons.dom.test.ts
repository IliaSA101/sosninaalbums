// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Окно плиток (100): кнопка своего на виду, «QR-код по ссылке…», строка «На» в «Фонах». Go подменён.
const api = vi.hoisted(() => ({
  DecorCatalog: vi.fn(),
  QRPreview: vi.fn(),
  AddQRSticker: vi.fn(),
  PlanBackgroundTo: vi.fn(),
  ApplyBackground: vi.fn(),
  ApplyBackgroundTo: vi.fn(),
  LibraryPacks: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))

const { default: DecorDialog } = await import('./DecorDialog.svelte')
const { decor } = await import('./decor.svelte')
const { qrForm } = await import('./qr.svelte')
const { store } = await import('../../base/core/store.svelte')

const cat = (kind: string, o: Record<string, unknown> = {}) => ({
  kind,
  themes: [
    { id: 'all', name: 'Все', count: 1 },
    { id: 'mine', name: 'Мои', count: 0 },
  ],
  items: [{ id: 'sa:x', theme: 'basic', name: 'Икс', url: '/media/gen/art?k=x&v=1', own: false, aspect: 0, w: 0, h: 0 }],
  recent: [],
  readOnly: false,
  ...o,
})
const sheet = (id: string) => ({
  id,
  photos: [],
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
const open = async (kind: 'bg' | 'sticker' | 'frame', o: Record<string, unknown> = {}, replace?: string) => {
  api.DecorCatalog.mockResolvedValue(cat(kind, o))
  const r = render(DecorDialog)
  decor.show(kind, 's1', 'spread', replace)
  await vi.waitFor(() => expect(decor.cat).not.toBeNull())
  flushSync()
  return r
}
const btn = (c: HTMLElement, text: string) =>
  Array.from(c.ownerDocument.querySelectorAll('button')).find((b) => b.textContent?.trim() === text)
const bar = () => Array.from(document.querySelectorAll('.bar button'), (b) => b.textContent?.trim())

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.QRPreview.mockImplementation(async (l: string) => ({ text: `https://${l}`, side: 33, path: 'M4 4h7v1h-7z', minMM: 26 }))
  api.LibraryPacks.mockResolvedValue({ packs: [], net: {}, busy: false, readOnly: false })
  decor.close()
  decor.cat = null
  decor.theme = ''
  qrForm.close()
  store.view = { rev: 1, folders: [], settings: { pageW: 300, pageH: 300 }, spreads: [sheet('s1'), sheet('s2'), sheet('s3')] } as never
})
afterEach(() => cleanup())

describe('кнопка своего (100-1)', () => {
  it.each([
    ['bg', 'Добавить свой фон…'],
    ['sticker', 'Добавить свой стикер…'],
    ['frame', 'Добавить свою рамочку…'],
  ] as const)('%s: primary, крайняя справа, «Загрузить своё…» нет', async (kind, name) => {
    const r = await open(kind)
    const b = btn(r.container, name)!
    expect(b.classList.contains('primary')).toBe(true)
    expect(bar().at(-1)).toBe(name)
    expect(bar()).not.toContain('Загрузить своё…')
    expect(bar()[bar().indexOf('Наборы…')]).toBe('Наборы…')
  })

  it('стикеры: слева от неё «QR-код по ссылке…», ещё левее «Наборы…»', async () => {
    await open('sticker')
    expect(bar()).toEqual(['Наборы…', 'QR-код по ссылке…', 'Добавить свой стикер…'])
  })

  it('QR-кнопки нет у фонов и рамочек и при «Заменить…»', async () => {
    await open('bg')
    expect(bar()).not.toContain('QR-код по ссылке…')
    cleanup()
    await open('sticker', {}, 'k1')
    expect(bar()).not.toContain('QR-код по ссылке…')
  })

  it('только для чтения: обе кнопки серые', async () => {
    await open('sticker', { readOnly: true })
    const off = Array.from(document.querySelectorAll<HTMLButtonElement>('.bar button'), (b) => [b.textContent?.trim(), b.disabled])
    expect(off).toEqual([
      ['Наборы…', false], // наборы открываются и при чтении
      ['QR-код по ссылке…', true],
      ['Добавить свой стикер…', true],
    ])
  })

  it('в теме «Мои» у стикеров — строка про логотип; пустая «Мои» — текст и та же кнопка по центру', async () => {
    const r = await open('sticker')
    expect(document.querySelector('.tip')).toBeNull()
    decor.setTheme('mine')
    flushSync()
    expect(document.querySelector('.tip')?.textContent).toBe(
      'Логотип и надписи — PNG или SVG с прозрачным фоном; JPEG ляжет с белым прямоугольником',
    )
    expect(document.querySelector('.empty')?.textContent).toContain('Здесь появятся ваши стикеры')
    expect(document.querySelectorAll('.center button.primary')).toHaveLength(1)
    expect(btn(r.container, 'Добавить свой стикер…')).toBeTruthy()
  })

  it('пустая «Мои» только для чтения — кнопки по центру нет', async () => {
    await open('sticker', { readOnly: true })
    decor.setTheme('mine')
    flushSync()
    expect(document.querySelector('.center')).toBeNull()
  })
})

describe('окно «QR-код»', () => {
  it('кнопка открывает окно с полем «Ссылка», «Отмена · Поставить»; Поставить серая до ввода', async () => {
    const r = await open('sticker')
    await fireEvent.click(btn(r.container, 'QR-код по ссылке…')!)
    expect(qrForm.open).toBe(true)
    const field = document.querySelector<HTMLInputElement>('#qr-link')!
    expect(field.labels?.[0]?.textContent).toBe('Ссылка')
    const actions = Array.from(document.querySelectorAll('.actions button'), (b) => b.textContent?.trim())
    expect(actions).toEqual(['Отмена', 'Поставить'])
    expect(btn(r.container, 'Поставить')!.disabled).toBe(true)
  })

  it('«Отмена» закрывает только QR, окно «Стикеры» остаётся', async () => {
    const r = await open('sticker')
    await fireEvent.click(btn(r.container, 'QR-код по ссылке…')!)
    await fireEvent.click(btn(r.container, 'Отмена')!)
    expect(qrForm.open).toBe(false)
    expect(decor.cur).not.toBeNull()
  })

  it('ввод: образец-svg, «Будет в коде» и минимум печати; Поставить оживает', async () => {
    const r = await open('sticker')
    await fireEvent.click(btn(r.container, 'QR-код по ссылке…')!)
    const field = document.querySelector<HTMLInputElement>('#qr-link')!
    await fireEvent.input(field, { target: { value: 't.me/test' } })
    await vi.waitFor(() => expect(document.querySelector('svg[shape-rendering="crispEdges"]')).not.toBeNull())
    const text = document.querySelector('.form')?.textContent ?? ''
    expect(text).toContain('Будет в коде: https://t.me/test')
    expect(text).toContain('Печатать не мельче 26 мм — иначе телефон может не прочитать')
    expect(btn(r.container, 'Поставить')!.disabled).toBe(false)
  })

  it('ошибка Go — строка под полем, Поставить серая', async () => {
    api.QRPreview.mockRejectedValue('в ссылке управляющие знаки')
    const r = await open('sticker')
    await fireEvent.click(btn(r.container, 'QR-код по ссылке…')!)
    await fireEvent.input(document.querySelector<HTMLInputElement>('#qr-link')!, { target: { value: 'a\nb' } })
    await vi.waitFor(() => expect(document.querySelector('.err')?.textContent).toBe('В ссылке управляющие знаки'))
    expect(btn(r.container, 'Поставить')!.disabled).toBe(true)
    expect(document.querySelector('svg[shape-rendering]')).toBeNull()
  })
})

describe('строка «На» (100-7)', () => {
  it('Seg с подписью «На»: Этот разворот · Все развороты (3) · Весь альбом · Номера; радиокнопок нет', async () => {
    await open('bg')
    expect(document.querySelector('input[type="radio"]')).toBeNull()
    const seg = document.querySelector('[aria-label="На какие развороты"]')!
    expect(Array.from(seg.querySelectorAll('button'), (b) => b.textContent?.trim())).toEqual([
      'Этот разворот',
      'Все развороты (3)',
      'Весь альбом',
      'Номера',
    ])
    expect(seg.querySelector('[aria-pressed="true"]')?.textContent).toBe('Этот разворот')
  })

  it('«Номера» — поле справа и фокус в нём; повторный щелчок по выбранному не ломает; «Этот разворот» убирает поле', async () => {
    const r = await open('bg')
    await fireEvent.click(btn(r.container, 'Номера')!)
    await vi.waitFor(() => expect(document.activeElement?.getAttribute('aria-label')).toBe('Номера разворотов'))
    await fireEvent.click(btn(r.container, 'Номера')!)
    expect(decor.bgPick).toBe('nums')
    await fireEvent.click(btn(r.container, 'Этот разворот')!)
    flushSync()
    expect(document.querySelector('input.ranges')).toBeNull()
    expect(decor.bgPick).toBe('this')
  })

  it('«Все развороты» — подсказка «Затронет 3 разворота»', async () => {
    const r = await open('bg')
    await fireEvent.click(btn(r.container, 'Все развороты (3)')!)
    flushSync()
    expect(document.querySelector('.hint')?.textContent).toBe('Затронет 3 разворота')
  })

  it('светлый стикер (dark из Go) — плитка с тёмной шашечкой; у цветного класса нет; в «Заменить…» так же', async () => {
    const it = (id: string, o: Record<string, unknown> = {}) => ({
      id,
      theme: 'basic',
      name: id,
      url: `/media/gen/art?k=${id}`,
      own: false,
      aspect: 1,
      w: 0,
      h: 0,
      ...o,
    })
    const items = [it('sa:white', { dark: true }), it('sa:gold'), it('sa:black', { dark: false })]
    const dark = () => Array.from(document.querySelectorAll('.tile'), (b) => b.classList.contains('stkdark'))
    await open('sticker', { items })
    expect(dark()).toEqual([true, false, false])
    expect(document.querySelectorAll('.tile.stk')).toHaveLength(3)
    cleanup()
    await open('sticker', { items }, 'st1')
    expect(dark()).toEqual([true, false, false])
  })

  it('у стикеров и в альбоме из одного разворота строки нет', async () => {
    await open('sticker')
    expect(document.querySelector('[aria-label="На какие развороты"]')).toBeNull()
    cleanup()
    store.view = { ...store.view, spreads: [sheet('s1')] } as never
    await open('bg')
    expect(document.querySelector('[aria-label="На какие развороты"]')).toBeNull()
  })
})
