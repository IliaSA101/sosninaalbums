// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// Окна «своих» шаблонов у разворота (GridGallery) после выноса в общий GridTplModals (132 U1): слова, вызовы и порядок
// кнопок те же, что были. Пара к covergrid.dom.test.ts, где те же окна открыты на обложке. Go подменён.
const calls: { name: string; args: unknown[] }[] = []
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  GridChoices: async (...a: unknown[]) => {
    calls.push({ name: 'GridChoices', args: a })
    return { builtin: [], mine: [{ id: 'm1', name: 'Мой', scope: 'page', drop: 0, boxes: [{ x: 0, y: 0, w: 1, h: 1 }] }] }
  },
  SaveGridTemplate: async (...a: unknown[]) => void calls.push({ name: 'SaveGridTemplate', args: a }),
  RenameGridTemplate: async (...a: unknown[]) => void calls.push({ name: 'RenameGridTemplate', args: a }),
  DeleteGridTemplate: async (...a: unknown[]) => void calls.push({ name: 'DeleteGridTemplate', args: a }),
  SetCustom: async () => {},
  SetLevel: async () => ({ first: false }),
  Marks: async () => ({}),
}))
vi.mock('../../shell/guide/guidestate.svelte', () => ({ guide: { show: () => {} } }))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { default: GridGallery } = await import('./GridGallery.svelte')
const { store } = await import('../../base/core/store.svelte')
const { gallery } = await import('./gallery.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')

const flush = async () => {
  await tick()
  await new Promise((r) => setTimeout(r, 0))
  await tick()
}
const dialog = () => document.querySelector<HTMLElement>('[role="dialog"]')
const btns = () => [...dialog()!.querySelectorAll('.actions button')].map((b) => b.textContent)
const hints = () => [...dialog()!.querySelectorAll('.hint')].map((h) => h.textContent)

async function setup(settings: object = { pageW: 150, pageH: 200 }, places: unknown[] = []) {
  const sp = { id: 's1', photos: [], rects: [], places, w: 300, h: 200, mid: 150 }
  store.view = { rev: 1, folders: [], spreads: [sp], settings } as never
  gallery.cur = { spread: 's1', scope: 'spread', tab: 'grids' }
  render(GridGallery, { g: { spread: 's1', scope: 'spread' } })
  await flush()
}

beforeEach(() => {
  calls.length = 0
  store.toast = null
  prefs.level = 'full'
})
afterEach(() => {
  cleanup()
  gallery.close()
})

describe('окна шаблонов разворота (общий GridTplModals)', () => {
  it('«Сохранить как шаблон»: Область «Разворот · Левая · Правая», подсказка про страницы, вызов с id разворота и областью', async () => {
    await setup()
    await fireEvent.click(document.querySelector('.save')!)
    await tick()
    expect(dialog()?.getAttribute('aria-label')).toBe('Сохранить как шаблон')
    expect([...dialog()!.querySelectorAll('[aria-label="Область шаблона"] button')].map((b) => b.textContent)).toEqual([
      'Разворот',
      'Левая',
      'Правая',
    ])
    expect(hints()).toEqual(['Запоминаются только места, без фото. «Левая» и «Правая» дают шаблон страницы.'])
    expect(btns()).toEqual(['Отмена', 'Сохранить'])
    const input = dialog()!.querySelector('input')!
    await fireEvent.input(input, { target: { value: ' Мой ' } })
    await fireEvent.keyDown(input, { key: 'Enter' })
    await flush()
    expect(calls.find((c) => c.name === 'SaveGridTemplate')?.args).toEqual(['s1', 'spread', 'Мой'])
    expect(store.toast?.text).toBe('Шаблон сохранён')
    expect(calls.filter((c) => c.name === 'GridChoices')).toHaveLength(2) // ряд перезапрошен
  })

  it('лист (альбом без шва): области нет, подсказка без «Левая/Правая»', async () => {
    await setup({ sheet: true, pageW: 300, pageH: 200 })
    await fireEvent.click(document.querySelector('.save')!)
    await tick()
    expect(dialog()!.querySelector('[aria-label="Область шаблона"]')).toBeNull()
    expect(hints()).toEqual(['Запоминаются только места, без фото.'])
  })

  it('рамочка у мест — «Рамочки в шаблон не входят»', async () => {
    await setup({ pageW: 150, pageH: 200 }, [{ id: 'p', x: 10, y: 10, w: 40, h: 40, photo: '', border: { width: 1, color: '#000' } }])
    await fireEvent.click(document.querySelector('.save')!)
    await tick()
    expect(hints()).toEqual(['Запоминаются только места, без фото. «Левая» и «Правая» дают шаблон страницы.', 'Рамочки в шаблон не входят'])
  })

  it('✎ и ×: переименование и «Удалить шаблон?» с концовкой «Развороты, где он уже применён, не изменятся.»', async () => {
    await setup()
    await fireEvent.click(document.querySelector('[aria-label="Переименовать шаблон"]')!)
    await tick()
    expect(dialog()?.getAttribute('aria-label')).toBe('Переименовать шаблон')
    expect(dialog()!.querySelector('input')!.value).toBe('Мой')
    expect(btns()).toEqual(['Отмена', 'Сохранить'])
    await fireEvent.click([...dialog()!.querySelectorAll('.actions button')][1])
    await flush()
    expect(calls.find((c) => c.name === 'RenameGridTemplate')?.args).toEqual(['m1', 'Мой'])

    await fireEvent.click(document.querySelector('[aria-label="Удалить шаблон"]')!)
    await tick()
    expect(dialog()?.getAttribute('aria-label')).toBe('Удалить шаблон?')
    expect(dialog()!.querySelector('p')?.textContent).toBe(
      'Шаблон «Мой» будет удалён из списка. Развороты, где он уже применён, не изменятся.',
    )
    expect(btns()).toEqual(['Отмена', 'Удалить'])
    await fireEvent.click([...dialog()!.querySelectorAll('.actions button')][1])
    await flush()
    expect(calls.find((c) => c.name === 'DeleteGridTemplate')?.args).toEqual(['m1'])
  })

  it('у листа текст удаления — «Листы, где он уже применён, не изменятся.»', async () => {
    await setup({ sheet: true, pageW: 300, pageH: 200 })
    await fireEvent.click(document.querySelector('[aria-label="Удалить шаблон"]')!)
    await tick()
    expect(dialog()!.querySelector('p')?.textContent).toBe('Шаблон «Мой» будет удалён из списка. Листы, где он уже применён, не изменятся.')
  })
})
