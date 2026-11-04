// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// Галерея сторонки обложки (127-17): вкладка по умолчанию, ряды, «−N», щелчок, наведение, уровни. Go подменён.
const calls: { name: string; args: unknown[] }[] = []
let answer: (scope: string, n: number) => unknown = () => ({})
let saveError: unknown = null

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  CoverGridChoices: async (scope: string, n: number) => {
    calls.push({ name: 'CoverGridChoices', args: [scope, n] })
    return answer(scope, n)
  },
  ApplyCoverGrid: async (scope: string, boxes: unknown) => {
    calls.push({ name: 'ApplyCoverGrid', args: [scope, boxes] })
    return { view: coverView(9), left: 2, named: [] }
  },
  SaveGridTemplate: async (spread: string, scope: string, name: string) => {
    calls.push({ name: 'SaveGridTemplate', args: [spread, scope, name] })
    if (saveError) throw saveError
  },
  RenameGridTemplate: async (id: string, name: string) => {
    calls.push({ name: 'RenameGridTemplate', args: [id, name] })
  },
  DeleteGridTemplate: async (id: string) => {
    calls.push({ name: 'DeleteGridTemplate', args: [id] })
  },
  SetCustom: async () => {},
  SetLevel: async () => ({ first: false }),
  LayoutChoices: async () => ({}),
  Marks: async () => ({}),
}))
vi.mock('../../shell/guide/guidestate.svelte', () => ({ guide: { show: () => {} } }))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { default: CoverGridGallery } = await import('./CoverGridGallery.svelte')
const { store } = await import('../../base/core/store.svelte')
const { gallery } = await import('./gallery.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')

const BOX = { x: 160, y: 10, w: 130, h: 280 }
const sides = (places: number) => [
  { scope: 'left', label: 'Задняя', box: { ...BOX, x: 10 }, places: 0, known: false, alone: false },
  { scope: 'right', label: 'Лицевая', box: BOX, places, known: places > 0, alone: false },
]
function coverView(rev: number, places = 3, o: { sides?: unknown[]; cellPlaces?: unknown[] } = {}) {
  return {
    rev,
    marksRev: 0,
    folders: [],
    settings: { sheet: false, pageW: 300, pageH: 300 },
    spreads: [{ id: 's1', photos: [] }],
    cover: {
      sheet: {
        id: 'cover',
        photos: ['a', 'b'],
        rects: [
          { x: 170, y: 20, w: 50, h: 50 },
          { x: 230, y: 20, w: 50, h: 50 },
        ],
        places: o.cellPlaces ?? [],
        w: 300,
        h: 300,
        mid: 150,
      },
      sides: o.sides ?? sides(places),
      spine: 10,
    },
  } as never
}
const tile = (id: string, drop = 0, n = 2, name = '') => ({
  id,
  name,
  scope: 'page',
  drop,
  boxes: Array.from({ length: n }, (_, i) => ({ x: i / n, y: 0, w: 1 / n, h: 1 })),
})
const full = (n = 3) => ({
  scope: 'right',
  n,
  box: BOX,
  places: 3,
  photos: 3,
  byPhoto: [tile('p1'), tile('p2')],
  builtin: [tile('b1', 1)],
  mine: [tile('m1', 0, 3, 'Мои три')],
})
const cards = () => [...document.querySelectorAll<HTMLButtonElement>('button.card')]
const side = (label: string) =>
  [...document.querySelectorAll<HTMLButtonElement>('.vseg button')].find((b) => b.textContent?.startsWith(label))
const flush = async () => {
  await tick()
  await new Promise((r) => setTimeout(r, 0))
  await tick()
}

async function open(places = 3, scope: 'left' | 'right' = 'right', o: Parameters<typeof coverView>[2] = {}) {
  store.onCover = false
  store.apply(coverView(5, places, o))
  store.goCover()
  gallery.open(scope, 'grids')
  const r = render(CoverGridGallery, { g: { spread: 'cover', scope } })
  await flush()
  return r
}

beforeEach(() => {
  calls.length = 0
  saveError = null
  store.toast = null
  answer = () => full()
  prefs.level = 'full'
})
afterEach(() => {
  cleanup()
  gallery.close()
})

describe('галерея сторонки обложки', () => {
  it('вкладка по умолчанию — число мест сторонки; шапка без вкладок, но с «Сохранить как шаблон…» (132)', async () => {
    await open(3)
    expect(calls[0]).toEqual({ name: 'CoverGridChoices', args: ['right', 3] })
    expect(document.querySelector('[aria-label="Число мест"] .on, [aria-label="Число мест"] [aria-pressed="true"]')?.textContent).toBe('3')
    expect(document.querySelector('.save')?.textContent).toContain('Сохранить как шаблон…')
    expect(document.querySelector('.save')?.getAttribute('title')).toBe('Запомнить места этой сторонки как шаблон')
    expect(document.querySelector('[aria-label="Что показать"]')).toBeNull()
    expect(document.querySelector('.title')?.textContent).toBe('Лицевая')
  })

  it('0 мест — вкладка «1»', async () => {
    answer = () => ({ ...full(1), places: 0, byPhoto: [] })
    await open(0)
    expect(calls[0].args).toEqual(['right', 1])
  })

  it('«Все»: по фото, встроенные, мои; слева четыре пункта; «−N» только у плиток с потерей', async () => {
    await open()
    expect(cards()).toHaveLength(4)
    expect([...document.querySelectorAll('.vseg button')].map((b) => b.textContent)).toEqual(['Все4', 'По фото2', 'Встроенные1', 'Мои1'])
    expect(cards().map((c) => c.querySelector('.drop')?.textContent ?? '')).toEqual(['', '', '−1', ''])
  })

  it('«По фото» пуст — пункт серый с причиной, плитки идут дальше', async () => {
    answer = (_, n) => (n === 5 ? { ...full(5), byPhoto: [] } : full(n))
    await open(3)
    const seg = document.querySelector('[aria-label="Число мест"]')!
    await fireEvent.click([...seg.querySelectorAll('button')].find((b) => b.textContent === '5')!)
    await flush()
    const b = side('По фото')!
    expect(b.getAttribute('aria-disabled')).toBe('true')
    expect(b.title).toBe('На сторонке 3 места — откройте вкладку «3»')
    expect(cards()).toHaveLength(2)
  })

  it('пункт слева переключает ряд', async () => {
    await open()
    await fireEvent.click(side('Встроенные')!)
    expect(cards()).toHaveLength(1)
    await fireEvent.click(side('Мои')!)
    expect(cards()).toHaveLength(1)
  })

  it('число мест — новый запрос Go', async () => {
    await open()
    const seg = document.querySelector('[aria-label="Число мест"]')!
    await fireEvent.click([...seg.querySelectorAll('button')].find((b) => b.textContent === '5')!)
    await flush()
    expect(calls.at(-1)).toEqual({ name: 'CoverGridChoices', args: ['right', 5] })
  })

  it('щелчок: галерея закрывается, Go получает доли как есть, лишние фото — тост «Вернуть»', async () => {
    await open()
    const first = cards()[0]
    await fireEvent.click(first)
    await flush()
    expect(gallery.cur).toBeNull()
    const ap = calls.find((c) => c.name === 'ApplyCoverGrid')!
    expect(ap.args[0]).toBe('right')
    expect(ap.args[1]).toEqual(full().byPhoto[0].boxes)
    expect(store.toast?.text).toBe('2 фото вернулись в ленту')
    expect(store.toast?.action?.label).toBe('Вернуть')
  })

  it('наведение: предпросмотр на листе обложки в мм листа; уход курсора снимает', async () => {
    await open()
    await fireEvent.mouseEnter(cards()[0])
    expect(gallery.preview?.spread).toBe('cover')
    // две пары фото лицевой встают в половины зоны: x = 160 и 160 + 65
    expect(gallery.preview?.rects.map((r) => [r.x, r.w])).toEqual([
      [160, 65],
      [225, 65],
    ])
    await fireEvent.mouseLeave(cards()[0])
    expect(gallery.preview).toBeNull()
  })

  it('у «Моих» плиток есть имя, ✎ и ×; у «По фото» и «Встроенных» их нет (132)', async () => {
    await open()
    expect(document.querySelectorAll('.tools')).toHaveLength(1)
    const mineTile = document.querySelector('.tools')!.closest('.item')!
    expect(mineTile.querySelector('.name')?.textContent).toBe('Мои три')
    expect(mineTile.querySelector('[aria-label="Переименовать шаблон"]')).not.toBeNull()
    expect(mineTile.querySelector('[aria-label="Удалить шаблон"]')).not.toBeNull()
    await fireEvent.click(side('Встроенные')!)
    expect(document.querySelector('.tools')).toBeNull()
  })

  // Ряды по уровням: «Варианты» держат «По фото» и «Встроенные», «Шаблоны» — «Мои» (0.26-127-14).
  const reset = () => {
    cleanup()
    gallery.close()
    prefs.custom = null
    prefs.level = 'full'
  }

  it('без уровня «Шаблоны» нет «Мои»: ни пункта, ни плиток в «Все»', async () => {
    prefs.level = 'simple'
    await open()
    expect(side('Мои')).toBeUndefined()
    expect(side('Встроенные')).toBeDefined()
    expect(side('По фото')).toBeDefined()
    expect(cards()).toHaveLength(3) // p1, p2, b1 — без m1 (три места)
    reset()
  })

  it('без «Вариантов» нет «По фото» и «Встроенных»: ни пунктов, ни плиток в «Все»', async () => {
    prefs.toggle('variants', false)
    await open()
    expect(prefs.can('variants')).toBe(false)
    expect(prefs.can('templates')).toBe(true)
    expect(side('По фото')).toBeUndefined()
    expect(side('Встроенные')).toBeUndefined()
    expect(side('Мои')).toBeDefined()
    expect(cards()).toHaveLength(1) // только m1
    expect(cards()[0].querySelectorAll('.cell')).toHaveLength(3)
    reset()
  })
})

// Окна «своих» шаблонов на обложке (132, U1): те же слова и кнопки, что у разворота (GridTplModals).
describe('шаблоны сторонки: сохранить, переименовать, удалить', () => {
  const dialog = () => document.querySelector<HTMLElement>('[role="dialog"]')
  const nameField = () => dialog()!.querySelector<HTMLInputElement>('input[type="text"]')!
  const btn = (label: string) => [...dialog()!.querySelectorAll<HTMLButtonElement>('.actions button')].find((b) => b.textContent === label)!
  const areaBtn = (label: string) =>
    [...dialog()!.querySelectorAll<HTMLButtonElement>('[aria-label="Область шаблона"] button')].find((b) => b.textContent === label)!
  const areaItems = () => [...dialog()!.querySelectorAll('[aria-label="Область шаблона"] button')].map((b) => b.textContent)
  const hints = () => [...dialog()!.querySelectorAll('.hint')].map((h) => h.textContent)
  const openSave = async (...a: Parameters<typeof open>) => {
    await open(...a)
    await fireEvent.click(document.querySelector('.save')!)
    await tick()
  }
  const HINT = 'Запоминаются только места в пределах пунктира, без фото. Получится шаблон страницы.'

  it('окно «Сохранить как шаблон»: поле, «Область: Задняя · Лицевая» (по умолчанию — сторонка галереи), подсказка, «Отмена · Сохранить»', async () => {
    await openSave()
    expect(dialog()?.getAttribute('aria-label')).toBe('Сохранить как шаблон')
    expect(nameField().placeholder).toBe('Шаблон N')
    expect(areaItems()).toEqual(['Задняя', 'Лицевая'])
    expect(areaBtn('Лицевая').getAttribute('aria-pressed')).toBe('true')
    expect(areaBtn('Задняя').getAttribute('aria-pressed')).toBe('false')
    expect(hints()).toEqual([HINT])
    expect([...dialog()!.querySelectorAll('.actions button')].map((b) => b.textContent)).toEqual(['Отмена', 'Сохранить'])
  })

  it('Enter в поле: SaveGridTemplate("cover", "right", имя), тост «Шаблон сохранён», ряд перезапрошен', async () => {
    await openSave()
    const asked = calls.filter((c) => c.name === 'CoverGridChoices').length
    await fireEvent.input(nameField(), { target: { value: '  Три в ряд ' } })
    await fireEvent.keyDown(nameField(), { key: 'Enter' })
    await flush()
    expect(calls.filter((c) => c.name === 'SaveGridTemplate')).toEqual([
      { name: 'SaveGridTemplate', args: ['cover', 'right', 'Три в ряд'] },
    ])
    expect(store.toast?.text).toBe('Шаблон сохранён')
    expect(dialog()).toBeNull()
    expect(calls.filter((c) => c.name === 'CoverGridChoices').length).toBe(asked + 1)
  })

  it('галерея задней: область по умолчанию — «Задняя», Go получает "left", пустое имя уходит пустым («Шаблон N» назовёт Go)', async () => {
    await openSave(0, 'left')
    expect(areaBtn('Задняя').getAttribute('aria-pressed')).toBe('true')
    await fireEvent.click(btn('Сохранить'))
    await flush()
    expect(calls.find((c) => c.name === 'SaveGridTemplate')?.args).toEqual(['cover', 'left', ''])
  })

  it('выбор «Область» в окне меняет сторону вызова', async () => {
    await openSave(3, 'right')
    await fireEvent.click(areaBtn('Задняя'))
    await fireEvent.click(btn('Сохранить'))
    await flush()
    expect(calls.find((c) => c.name === 'SaveGridTemplate')?.args).toEqual(['cover', 'left', ''])
  })

  it('«Отмена» и Esc закрывают окно без вызова', async () => {
    await openSave()
    await fireEvent.click(btn('Отмена'))
    expect(dialog()).toBeNull()
    await fireEvent.click(document.querySelector('.save')!)
    await tick()
    await fireEvent.keyDown(dialog()!, { key: 'Escape' })
    expect(dialog()).toBeNull()
    expect(calls.some((c) => c.name === 'SaveGridTemplate')).toBe(false)
  })

  it('ошибка Go — красный тост её текстом, без «Шаблон сохранён» и без перезапроса', async () => {
    saveError = 'На лицевой нет мест'
    await openSave()
    const asked = calls.filter((c) => c.name === 'CoverGridChoices').length
    await fireEvent.click(btn('Сохранить'))
    await flush()
    expect(store.toast?.text).toBe('На лицевой нет мест')
    expect(store.toast?.kind).toBe('error')
    expect(calls.filter((c) => c.name === 'CoverGridChoices').length).toBe(asked)
  })

  it('одна сторонка в виде — выбора «Область» нет', async () => {
    await openSave(3, 'right', { sides: [sides(3)[1]] })
    expect(dialog()!.querySelector('[aria-label="Область шаблона"]')).toBeNull()
    expect(dialog()!.textContent).not.toContain('Область')
  })

  it('рамочки у мест выбранной сторонки — строка «Рамочки в шаблон не входят»; у другой сторонки и без рамочек её нет', async () => {
    const border = { width: 1, color: '#000' }
    await openSave(3, 'right', { cellPlaces: [{ x: 180, y: 30, w: 40, h: 40, border }] })
    expect(hints()).toEqual([HINT, 'Рамочки в шаблон не входят'])
    await fireEvent.click(areaBtn('Задняя')) // рамочка только на лицевой
    expect(hints()).toEqual([HINT])
    cleanup()
    gallery.close()
    await openSave(3, 'right', { cellPlaces: [{ x: 180, y: 30, w: 40, h: 40 }] })
    expect(hints()).toEqual([HINT])
  })

  it('без уровня «Шаблоны» кнопки «Сохранить как шаблон…» нет', async () => {
    prefs.toggle('templates', false)
    await open()
    expect(prefs.can('templates')).toBe(false)
    expect(document.querySelector('.save')).toBeNull()
    prefs.custom = null
    prefs.level = 'full'
  })

  it('✎: окно «Переименовать шаблон» с именем; Enter — RenameGridTemplate(id, имя), ряд перезапрошен', async () => {
    await open()
    const asked = calls.filter((c) => c.name === 'CoverGridChoices').length
    await fireEvent.click(document.querySelector('[aria-label="Переименовать шаблон"]')!)
    await tick()
    expect(dialog()?.getAttribute('aria-label')).toBe('Переименовать шаблон')
    expect(nameField().value).toBe('Мои три')
    expect([...dialog()!.querySelectorAll('.actions button')].map((b) => b.textContent)).toEqual(['Отмена', 'Сохранить'])
    await fireEvent.input(nameField(), { target: { value: 'Новое имя' } })
    await fireEvent.keyDown(nameField(), { key: 'Enter' })
    await flush()
    expect(calls.find((c) => c.name === 'RenameGridTemplate')?.args).toEqual(['m1', 'Новое имя'])
    expect(calls.filter((c) => c.name === 'CoverGridChoices').length).toBe(asked + 1)
  })

  it('×: «Удалить шаблон?» с текстом обложки; «Отмена» — без вызова, «Удалить» — DeleteGridTemplate(id) и перезапрос', async () => {
    await open()
    await fireEvent.click(document.querySelector('[aria-label="Удалить шаблон"]')!)
    await tick()
    expect(dialog()?.getAttribute('aria-label')).toBe('Удалить шаблон?')
    expect(dialog()!.querySelector('p')?.textContent).toBe(
      'Шаблон «Мои три» будет удалён из списка. Обложка и развороты, где он уже применён, не изменятся.',
    )
    expect([...dialog()!.querySelectorAll('.actions button')].map((b) => b.textContent)).toEqual(['Отмена', 'Удалить'])
    await fireEvent.click(btn('Отмена'))
    expect(calls.some((c) => c.name === 'DeleteGridTemplate')).toBe(false)
    const asked = calls.filter((c) => c.name === 'CoverGridChoices').length
    await fireEvent.click(document.querySelector('[aria-label="Удалить шаблон"]')!)
    await tick()
    await fireEvent.click(btn('Удалить'))
    await flush()
    expect(calls.find((c) => c.name === 'DeleteGridTemplate')?.args).toEqual(['m1'])
    expect(calls.filter((c) => c.name === 'CoverGridChoices').length).toBe(asked + 1)
  })

  it('✎/× не ставят плитку: галерея остаётся открытой, ApplyCoverGrid не зовётся', async () => {
    await open()
    await fireEvent.click(document.querySelector('[aria-label="Удалить шаблон"]')!)
    await tick()
    expect(gallery.cur).not.toBeNull()
    expect(calls.some((c) => c.name === 'ApplyCoverGrid')).toBe(false)
  })
})
