// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// У кнопки «+» нет текста — имя для скринридера и UIA (проверка Windows-exe, спека 118) даёт aria-label.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { default: AddColumn } = await import('./AddColumn.svelte')
const { store } = await import('../../base/core/store.svelte')

afterEach(cleanup)

describe('AddColumn: доступное имя кнопки', () => {
  it('у кнопки «+» есть aria-label «Новый разворот»', () => {
    store.view = { ...store.view, spreads: [], settings: {} } as never
    const { container } = render(AddColumn)
    const b = container.querySelector('button.add')
    expect(b?.textContent?.trim()).toBe('+')
    expect(b?.getAttribute('aria-label')).toBe('Новый разворот')
  })
})

// 131: правый щелчок по «+» — меню с «Разворот макета «…»», когда макет стоит в альбоме и найден; виньетка — по уровню.
describe('AddColumn: меню правого щелчка', () => {
  const look = { key: 'aa', name: 'Светлая', found: true }
  async function menuLabels(view: Record<string, unknown>, canVig: boolean) {
    const { prefs } = await import('../../base/prefs/prefs.svelte')
    vi.spyOn(prefs, 'can').mockImplementation((f) => (f === 'vignette' ? canVig : true))
    store.view = { ...store.view, spreads: [], settings: {}, look: undefined, ...view } as never
    const { container } = render(AddColumn)
    await fireEvent.contextMenu(container.querySelector('button.add') as HTMLElement)
    await tick()
    return Array.from(document.querySelectorAll('[role="menuitem"]')).map((e) => e.textContent?.replace(/\s+/g, ' ').trim())
  }
  afterEach(() => vi.restoreAllMocks())

  it('макет стоит: пункт с именем макета, после «Новой виньетки»', async () => {
    expect(await menuLabels({ look }, true)).toEqual(['Новая виньетка', 'Разворот макета «Светлая»'])
  })
  it('без функции «Виньетка» меню всё равно есть и в нём только разворот макета', async () => {
    expect(await menuLabels({ look }, false)).toEqual(['Разворот макета «Светлая»'])
  })
  it('макета нет или он не найден на этом компьютере — пункта нет', async () => {
    expect(await menuLabels({}, true)).toEqual(['Новая виньетка'])
    cleanup()
    expect(await menuLabels({ look: { ...look, found: false } }, true)).toEqual(['Новая виньетка'])
  })
  it('ни виньетки, ни макета — меню не открывается', async () => {
    expect(await menuLabels({}, false)).toEqual([])
  })
  it('выбор пункта зовёт NewLookSpread через looks.newSpread', async () => {
    const { looks } = await import('../../shell/looks/looks.svelte')
    const call = vi.spyOn(looks, 'newSpread').mockResolvedValue()
    await menuLabels({ look }, false)
    await fireEvent.click(document.querySelector('[role="menuitem"]') as HTMLElement)
    expect(call).toHaveBeenCalledTimes(1)
  })
})
