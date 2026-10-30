// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Go подменён: карточка зовёт VignetteChoices (схема), подгон — facesapi. Окно «Подогнать по лицам?» — настоящий AutoAskDialog.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ VignetteChoices: async () => [], CropsToKeep: async () => 0 }))
const fit = vi.hoisted(() => vi.fn())
const progress = vi.hoisted(() => ({ fn: null as null | ((d: number, t: number) => void) }))
vi.mock('./facesapi', () => ({
  fitFaces: fit,
  onFacesProgress: (fn: (d: number, t: number) => void) => {
    progress.fn = fn
    return () => {}
  },
}))

const { default: VignettePanel } = await import('./VignettePanel.svelte')
const { default: AutoAskDialog } = await import('../../shell/ask/AutoAskDialog.svelte')
const { facesTip } = await import('./vigfaces')
const { gridAct } = await import('../layouts/gridact.svelte')
const { store } = await import('../../base/core/store.svelte')

const vig = { tpl: 't', teachers: 1, pupils: 24, ratio: '3:4', manual: false }
const sp = { id: 's1', grid: true, photos: [], places: [], rects: [], vig } as never
const view = (rev: number) => ({ rev, folders: [], spreads: [sp], settings: {} }) as never
const report = (o = {}) => ({
  fitted: 25,
  noFace: 0,
  many: 0,
  missing: 0,
  small: 0,
  big: 0,
  edge: 0,
  contain: 0,
  manual: 0,
  changed: 0,
  check: [],
  short: [],
  asked: false,
  ...o,
})

afterEach(() => {
  cleanup()
  gridAct.cancelAuto()
  fit.mockReset()
})

const btn = () => screen.getByRole('button', { name: /Подогнать по лицам|^Лица/ })

describe('кнопка «Подогнать по лицам» в карточке виньетки', () => {
  it('стоит в ряду после «Сортировать по фамилии», с подсказкой спеки', () => {
    store.apply(view(1))
    render(VignettePanel, { sp })
    const names = [...document.querySelectorAll('.tools button')].map((b) => b.textContent?.trim())
    expect(names).toEqual(['Шаблоны…', 'Сортировать по фамилии', 'Подогнать по лицам'])
    expect(btn().getAttribute('title')).toBe(facesTip())
    expect(facesTip()).toMatch(
      /^Лица одного размера, глаза на одной линии\. Групповое фото и «Целиком» не трогаются; «Отменить»( \(.+\))? возвращает$/,
    )
    expect(btn().getAttribute('aria-disabled')).toBe('false')
  })

  it('пока ищет: aria-disabled и «Лица 12 / 25»; повторное нажатие Go не зовёт; потом кнопка прежняя', async () => {
    store.apply(view(1))
    render(VignettePanel, { sp })
    let done: (v: unknown) => void = () => {}
    fit.mockReturnValue(new Promise((r) => (done = r)))
    await fireEvent.click(btn())
    flushSync()
    expect(btn().getAttribute('aria-disabled')).toBe('true')
    expect(btn().textContent?.trim()).toBe('Лица…')
    progress.fn?.(12, 25)
    flushSync()
    expect(btn().textContent?.trim()).toBe('Лица 12 / 25')
    await fireEvent.click(btn())
    expect(fit).toHaveBeenCalledTimes(1)
    done({ view: view(2), report: report() })
    await vi.waitFor(() => expect(btn().getAttribute('aria-disabled')).toBe('false'))
    expect(btn().textContent?.trim()).toBe('Подогнать по лицам')
  })
})

describe('окно «Подогнать по лицам?» при правленых кадрах', () => {
  it('вопрос и кнопки по спеке, N только в галочке (включена), «Отмена» в фокусе', async () => {
    store.apply(view(1))
    render(VignettePanel, { sp })
    render(AutoAskDialog)
    fit.mockResolvedValueOnce({ view: view(1), report: report({ asked: true, manual: 5, fitted: 0 }) })
    await fireEvent.click(btn())
    await vi.waitFor(() => expect(screen.getByRole('dialog')).toBeTruthy())
    expect(screen.getByRole('dialog').getAttribute('aria-label')).toBe('Подогнать по лицам?')
    expect(screen.getByText('Кадр у части фото уже настроен. Подогнать по лицам?')).toBeTruthy()
    const box = screen.getByLabelText('Сохранить кадр правленых фото: 5') as HTMLInputElement
    expect(box.checked).toBe(true)
    expect(screen.getByText('Подогнать', { selector: '.primary' })).toBeTruthy()
    expect(document.activeElement?.textContent).toBe('Отмена')
  })

  it('нет правленых — окна нет', async () => {
    store.apply(view(1))
    render(VignettePanel, { sp })
    render(AutoAskDialog)
    fit.mockResolvedValueOnce({ view: view(2), report: report() })
    await fireEvent.click(btn())
    await vi.waitFor(() => expect(btn().getAttribute('aria-disabled')).toBe('false'))
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('«Подогнать» с галочкой — mode keep; без галочки — all', async () => {
    store.apply(view(1))
    render(VignettePanel, { sp })
    render(AutoAskDialog)
    fit.mockResolvedValueOnce({ view: view(1), report: report({ asked: true, manual: 5 }) })
    await fireEvent.click(btn())
    await vi.waitFor(() => expect(screen.getByRole('dialog')).toBeTruthy())
    fit.mockResolvedValueOnce({ view: view(2), report: report() })
    await fireEvent.click(screen.getByText('Подогнать', { selector: '.primary' }))
    await vi.waitFor(() => expect(fit).toHaveBeenCalledTimes(2))
    expect(fit).toHaveBeenLastCalledWith('s1', '', 'keep')

    fit.mockResolvedValueOnce({ view: view(2), report: report({ asked: true, manual: 5 }) })
    await vi.waitFor(() => expect(btn().getAttribute('aria-disabled')).toBe('false'))
    await fireEvent.click(btn())
    await vi.waitFor(() => expect(screen.getByRole('dialog')).toBeTruthy())
    await fireEvent.click(screen.getByLabelText('Сохранить кадр правленых фото: 5'))
    fit.mockResolvedValueOnce({ view: view(3), report: report() })
    await fireEvent.click(screen.getByText('Подогнать', { selector: '.primary' }))
    await vi.waitFor(() => expect(fit).toHaveBeenCalledTimes(4))
    expect(fit).toHaveBeenLastCalledWith('s1', '', 'all')
  })
})
