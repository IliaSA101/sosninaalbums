// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Go подменён: бэкенда в DOM-тесте нет; окно «Вернуть авто?» зовёт CropsToKeep (число для галочки, 098)
// и BackToAuto (один вызов вместо SetGrid(false) / SetLayout с сетки).
const view = { rev: 2, folders: [], spreads: [], settings: {} }
const backToAuto = vi.hoisted(() => vi.fn(async () => view))
const cropsToKeep = vi.hoisted(() => vi.fn(async () => 0))
const mergeAuto = vi.hoisted(() => vi.fn(async () => ({ rev: 3, folders: [], spreads: [], settings: {} })))
const splitAuto = vi.hoisted(() => vi.fn(async () => ({ rev: 3, folders: [], spreads: [], settings: {} })))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  BackToAuto: backToAuto,
  CropsToKeep: cropsToKeep,
  MergeSpreadsAuto: mergeAuto,
  SplitSpreadAuto: splitAuto,
  SetGrid: vi.fn(),
  SetLayout: vi.fn(),
}))

const { default: AutoAskDialog } = await import('./AutoAskDialog.svelte')
const { gridAct } = await import('../../panels/layouts/gridact.svelte')
const { store } = await import('../../base/core/store.svelte')
const { glueAsk } = await import('../../panels/timeline/menuact.svelte')
const { modalStack } = await import('../../base/modal/modalstack.svelte')

const grid = { id: 's1', grid: true, photos: [], rects: [] }

afterEach(() => {
  cleanup()
  gridAct.cancelAuto()
  backToAuto.mockClear()
  mergeAuto.mockClear()
  splitAuto.mockClear()
  cropsToKeep.mockReset()
  cropsToKeep.mockResolvedValue(0)
})

// Открыть вопрос: CropsToKeep асинхронный — окно появляется после ответа Go.
async function open(key = '') {
  store.view = { ...store.view, rev: 1, spreads: [grid] } as never
  store.currentId = 's1'
  await gridAct.backToAuto('s1', key)
  flushSync()
}

describe('AutoAskDialog: «Вернуть авто?»', () => {
  it('нет вопроса — окна в DOM нет', () => {
    render(AutoAskDialog)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('открыть → «Отмена» → окно ушло, Go не звали; снова открыть — окно снова есть', async () => {
    render(AutoAskDialog)
    await open()
    expect(screen.getByRole('dialog').getAttribute('aria-label')).toBe('Вернуть авто?')
    await fireEvent.click(screen.getByText('Отмена'))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(backToAuto).not.toHaveBeenCalled()
    expect(modalStack.any()).toBe(false)
    await open()
    expect(screen.getByRole('dialog')).toBeTruthy()
    expect(modalStack.any()).toBe(true)
  })

  it('безопасный выбор «Отмена» получает фокус', async () => {
    render(AutoAskDialog)
    await open()
    expect(document.activeElement?.textContent).toBe('Отмена')
  })

  it('Esc закрывает как «Отмена» и не доходит до страницы', async () => {
    const page = vi.fn()
    window.addEventListener('keydown', page)
    render(AutoAskDialog)
    await open()
    await fireEvent.keyDown(document.activeElement as Element, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(backToAuto).not.toHaveBeenCalled()
    expect(page).not.toHaveBeenCalled()
    window.removeEventListener('keydown', page)
  })

  it('щелчок по фону закрывает, щелчок внутри окна — нет', async () => {
    render(AutoAskDialog)
    await open()
    const box = screen.getByRole('dialog')
    await fireEvent.pointerDown(box)
    expect(screen.queryByRole('dialog')).not.toBeNull()
    await fireEvent.pointerDown(box.parentElement as Element)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('«Вернуть авто» зовёт Go один раз и закрывает окно', async () => {
    render(AutoAskDialog)
    await open()
    const ok = screen.getByText('Вернуть авто', { selector: 'button' })
    await fireEvent.click(ok)
    expect(backToAuto).toHaveBeenCalledExactlyOnceWith('s1', '', false)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('разворот пропал из вида (prune) — окно закрывается само', async () => {
    render(AutoAskDialog)
    await open()
    gridAct.prune([])
    flushSync()
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('N = 0 — галочки нет, окно прежнее; «Вернуть авто» зовёт Go с keep = false', async () => {
    render(AutoAskDialog)
    await open()
    expect(screen.queryByRole('checkbox')).toBeNull()
    expect(screen.getByText('Ваши правки мест сбросятся. Вернуть авто?')).toBeTruthy()
    await fireEvent.click(screen.getByText('Вернуть авто', { selector: 'button' }))
    expect(backToAuto).toHaveBeenCalledExactlyOnceWith('s1', '', false)
  })

  it('N > 0 — галочка «Сохранить кадр правленых фото: N», включена, «Вернуть авто» → keep = true', async () => {
    cropsToKeep.mockResolvedValue(2)
    render(AutoAskDialog)
    await open()
    expect(cropsToKeep).toHaveBeenCalledExactlyOnceWith('s1')
    const box = screen.getByRole('checkbox', { name: 'Сохранить кадр правленых фото: 2' }) as HTMLInputElement
    expect(box.checked).toBe(true)
    expect(document.activeElement?.textContent).toBe('Отмена')
    await fireEvent.click(screen.getByText('Вернуть авто', { selector: 'button' }))
    expect(backToAuto).toHaveBeenCalledExactlyOnceWith('s1', '', true)
  })

  it('галочка снята — keep = false; ключ из галереи уходит в тот же вызов', async () => {
    cropsToKeep.mockResolvedValue(1)
    render(AutoAskDialog)
    await open('k1')
    await fireEvent.click(screen.getByRole('checkbox'))
    expect((screen.getByRole('checkbox') as HTMLInputElement).checked).toBe(false)
    await fireEvent.click(screen.getByText('Вернуть авто', { selector: 'button' }))
    expect(backToAuto).toHaveBeenCalledExactlyOnceWith('s1', 'k1', false)
  })

  it('«Отмена» с галочкой — Go не звали; снова открыть — галочка снова включена', async () => {
    cropsToKeep.mockResolvedValue(1)
    render(AutoAskDialog)
    await open()
    await fireEvent.click(screen.getByRole('checkbox'))
    await fireEvent.click(screen.getByText('Отмена'))
    expect(backToAuto).not.toHaveBeenCalled()
    await open()
    expect((screen.getByRole('checkbox') as HTMLInputElement).checked).toBe(true)
  })

  it('CropsToKeep упал — окно прежнее, без галочки', async () => {
    cropsToKeep.mockRejectedValue(new Error('нет связи'))
    render(AutoAskDialog)
    await open()
    expect(screen.getByRole('dialog')).toBeTruthy()
    expect(screen.queryByRole('checkbox')).toBeNull()
  })

  it('ушли с разворота, пока Go считал кадры, — окно не открывается (ревью 0.23 п. 7)', async () => {
    let free!: (n: number) => void
    cropsToKeep.mockImplementationOnce(() => new Promise<number>((r) => (free = r)))
    render(AutoAskDialog)
    store.view = { ...store.view, rev: 1, spreads: [grid, { ...grid, id: 's2' }] } as never
    store.currentId = 's1'
    const p = gridAct.backToAuto('s1')
    store.currentId = 's2'
    free(2)
    await p
    flushSync()
    expect(gridAct.autoAsk).toBeNull()
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('виньетка: вопрос без галочки, CropsToKeep не зовут', async () => {
    render(AutoAskDialog)
    store.view = { ...store.view, rev: 1, spreads: [{ ...grid, vig: { manual: true } }] } as never
    await gridAct.vigGate('s1', 'reset', async () => {})
    flushSync()
    expect(screen.getByRole('dialog')).toBeTruthy()
    expect(screen.queryByRole('checkbox')).toBeNull()
    expect(cropsToKeep).not.toHaveBeenCalled()
  })
})

// Склейка и разрез через ручной разворот (103): то же окно, свои тексты, действие — MergeSpreadsAuto / SplitSpreadAuto.
describe('AutoAskDialog: склейка и разрез через ручной (103)', () => {
  const sp = (id: string, grid: boolean) => ({ id, grid, photos: ['p1', 'p2', 'p3'], rects: [] })
  const setup = (...list: ReturnType<typeof sp>[]) => {
    store.view = { ...store.view, rev: 1, spreads: list } as never
    store.currentId = list[0].id
  }
  // CropsToKeep асинхронный: окно появляется после ответа Go
  const ask = async (act: () => void) => {
    act()
    await vi.waitFor(() => expect(gridAct.autoAsk).not.toBeNull())
    flushSync()
  }

  it('склейка: текст, Отмена с фокусом; «Отмена» — без вызова Go', async () => {
    render(AutoAskDialog)
    setup(sp('a', false), sp('b', true))
    await ask(() => glueAsk.join('a'))
    expect(screen.getByRole('dialog').getAttribute('aria-label')).toBe('Склеить развороты?')
    expect(screen.getByText('У разворота 2 ручная раскладка. Ваши правки мест сбросятся. Склеить развороты 1 и 2?')).toBeTruthy()
    expect(document.activeElement?.textContent).toBe('Отмена')
    expect(cropsToKeep).toHaveBeenCalledExactlyOnceWith('b')
    await fireEvent.click(screen.getByText('Отмена'))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(mergeAuto).not.toHaveBeenCalled()
  })

  it('N = 0 — галочки нет; «Склеить» → MergeSpreadsAuto(id, false)', async () => {
    render(AutoAskDialog)
    setup(sp('a', true), sp('b', false))
    await ask(() => glueAsk.join('a'))
    expect(screen.queryByRole('checkbox')).toBeNull()
    await fireEvent.click(screen.getByText('Склеить', { selector: 'button' }))
    expect(mergeAuto).toHaveBeenCalledExactlyOnceWith('a', false)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('N > 0 — сумма по ручным из пары, галочка включена; «Склеить» → keep = true, снята → false', async () => {
    cropsToKeep.mockResolvedValueOnce(2).mockResolvedValueOnce(1)
    render(AutoAskDialog)
    setup(sp('a', true), sp('b', true))
    await ask(() => glueAsk.join('a'))
    expect(cropsToKeep).toHaveBeenCalledTimes(2)
    const box = screen.getByRole('checkbox', { name: 'Сохранить кадр правленых фото: 3' }) as HTMLInputElement
    expect(box.checked).toBe(true)
    await fireEvent.click(screen.getByText('Склеить', { selector: 'button' }))
    expect(mergeAuto).toHaveBeenCalledExactlyOnceWith('a', true)

    cropsToKeep.mockResolvedValue(1)
    setup(sp('a', true), sp('b', false))
    await ask(() => glueAsk.join('a'))
    await fireEvent.click(screen.getByRole('checkbox'))
    await fireEvent.click(screen.getByText('Склеить', { selector: 'button' }))
    expect(mergeAuto).toHaveBeenLastCalledWith('a', false)
  })

  it('разрез: «Разрезать» → SplitSpreadAuto(id, j, keep)', async () => {
    cropsToKeep.mockResolvedValue(1)
    render(AutoAskDialog)
    setup(sp('a', true))
    await ask(() => glueAsk.cut('a', 2))
    expect(screen.getByRole('dialog').getAttribute('aria-label')).toBe('Разрезать разворот?')
    expect(screen.getByText('У разворота 1 ручная раскладка. Ваши правки мест сбросятся. Разрезать разворот 1 перед фото 3?')).toBeTruthy()
    await fireEvent.click(screen.getByText('Разрезать', { selector: 'button' }))
    expect(splitAuto).toHaveBeenCalledExactlyOnceWith('a', 2, true)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('пара перестала быть ручной или пропала — окно закрыто само', async () => {
    render(AutoAskDialog)
    setup(sp('a', true), sp('b', false))
    await ask(() => glueAsk.join('a'))
    gridAct.prune([sp('a', false), sp('b', false)] as never)
    flushSync()
    expect(screen.queryByRole('dialog')).toBeNull()

    await ask(() => {
      setup(sp('a', true), sp('b', false))
      glueAsk.join('a')
    })
    gridAct.prune([sp('a', true), sp('c', false), sp('b', false)] as never) // между ними встал третий
    flushSync()
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(mergeAuto).not.toHaveBeenCalled()
  })

  it('пара изменилась, пока Go считал кадры, — окно не открывается', async () => {
    let free!: (n: number) => void
    cropsToKeep.mockImplementationOnce(() => new Promise<number>((r) => (free = r)))
    render(AutoAskDialog)
    setup(sp('a', true), sp('b', false))
    glueAsk.join('a')
    store.view = { ...store.view, spreads: [sp('a', false), sp('b', false)] } as never
    free(2)
    await new Promise((r) => setTimeout(r, 0))
    flushSync()
    expect(gridAct.autoAsk).toBeNull()
  })
})
