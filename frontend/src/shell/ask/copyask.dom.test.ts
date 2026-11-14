// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import type { UseKind } from '../../base/core/types'

// Окно «Фото уже в альбоме» (098): три кнопки, «Отмена» в фокусе, Esc, ссылки «где», «Перенести» только при одном месте.
const whereUsed = vi.hoisted(() => vi.fn())
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ WhereUsed: whereUsed }))
const goUse = vi.hoisted(() => vi.fn())
// Сам переход — в gouse.svelte.test.ts; здесь только что окно зовёт его и отменяет бросок.
vi.mock('../../editor/photoprops/gouse.svelte', () => ({ goUse }))

const { default: CopyAskDialog } = await import('./CopyAskDialog.svelte')
const { askCopy, answerCopy } = await import('../../base/drag/copyask.svelte')
const { store } = await import('../../base/core/store.svelte')
const { modalStack } = await import('../../base/modal/modalstack.svelte')

// Вхождение как от Go: вид по слову (только в тесте), pick — что выделить.
const KINDS: Record<string, UseKind> = { место: 'place', фон: 'backdrop', стикер: 'sticker', рамочка: 'border' }
const use = (sheet: number, label: string, pick = 'p') => {
  const [page, what = ''] = label.split(' — ')
  return { sheet, label, kind: KINDS[what] ?? ('albumBg' as const), spread: sheet < 0 ? '' : `s${sheet + 1}`, page, what, pick }
}
const one = (move: string) => [{ id: 'f1', uses: [use(1, 'Разворот 2 — место', move)], places: 1, move }]
const two = [{ id: 'f1', uses: [use(0, 'Разворот 1 — место'), use(1, 'Разворот 2 — место', 'f1-2')], places: 2, move: '' }]
const sp = (id: string) => ({ id, photos: [], grid: false })
const dialog = () => screen.queryByRole('dialog')
const shown = () => vi.waitFor(() => expect(dialog()).not.toBeNull())

afterEach(() => {
  answerCopy('cancel') // вопрос, оставленный тестом открытым
  cleanup()
  whereUsed.mockReset()
  goUse.mockReset()
})

// Открыть вопрос про ids; ответ Go — used. Возвращает обещание ответа.
async function open(used: unknown, ids = ['f1']) {
  store.view = { ...store.view, rev: 1, spreads: [sp('s1'), sp('s2')] } as never
  whereUsed.mockResolvedValueOnce(used)
  render(CopyAskDialog)
  const p = askCopy(ids)
  await shown()
  return p
}

describe('CopyAskDialog', () => {
  it('нет вопроса — окна в DOM нет', () => {
    render(CopyAskDialog)
    expect(dialog()).toBeNull()
  })

  it('одно место: название, строка со ссылкой и три кнопки; «Отмена» в фокусе', async () => {
    void open(one('f1'))
    await shown()
    expect(screen.getByRole('dialog').getAttribute('aria-label')).toBe('Фото уже в альбоме')
    expect(screen.getByText('Это фото уже стоит:')).toBeTruthy()
    expect(screen.getByText('Разворот 2', { selector: 'button' })).toBeTruthy()
    const buttons = [...screen.getByRole('dialog').querySelectorAll('.actions button')].map((b) => b.textContent)
    expect(buttons).toEqual(['Отмена', 'Перенести сюда', 'Поставить копию'])
    expect(document.activeElement?.textContent).toBe('Отмена')
    expect(modalStack.any()).toBe(true)
  })

  it('два места — «Перенести сюда» нет, подсказка про холст', async () => {
    void open(two)
    await shown()
    expect(screen.queryByText('Перенести сюда')).toBeNull()
    expect(screen.getByText('Перенести одно из мест — перетащите его на холсте.')).toBeTruthy()
    expect(screen.getByText('Поставить копию')).toBeTruthy()
  })

  it.each([
    ['Поставить копию', 'copy'],
    ['Перенести сюда', 'move'],
    ['Отмена', 'cancel'],
  ])('кнопка «%s» отвечает «%s» и закрывает окно', async (label, answer) => {
    const p = open(one('f1'))
    await shown()
    await fireEvent.click(screen.getByText(label, { selector: 'button' }))
    expect(await p).toBe(answer)
    expect(dialog()).toBeNull()
    expect(modalStack.any()).toBe(false)
  })

  it('Esc — «Отмена» и не доходит до страницы', async () => {
    const page = vi.fn()
    window.addEventListener('keydown', page)
    const p = open(one('f1'))
    await shown()
    await fireEvent.keyDown(document.activeElement as Element, { key: 'Escape' })
    expect(await p).toBe('cancel')
    expect(page).not.toHaveBeenCalled()
    window.removeEventListener('keydown', page)
  })

  it('щелчок по фону закрывает как «Отмена», внутри окна — нет', async () => {
    const p = open(one('f1'))
    await shown()
    const box = screen.getByRole('dialog')
    await fireEvent.pointerDown(box)
    expect(dialog()).not.toBeNull()
    await fireEvent.pointerDown(box.parentElement as Element)
    expect(await p).toBe('cancel')
  })

  it('ссылка «Разворот 2»: вопрос отменён, лист открыт', async () => {
    const p = open(one('f1-2'))
    await shown()
    await fireEvent.click(screen.getByText('Разворот 2', { selector: 'button' }))
    expect(await p).toBe('cancel')
    expect(goUse).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ kind: 'place', spread: 's2', pick: 'f1-2' }))
    expect(dialog()).toBeNull()
  })

  const withAlso = [
    {
      id: 'f1',
      uses: [use(1, 'Разворот 2 — место', 'f1'), use(4, 'Разворот 5 — фон', 'spread'), use(-1, 'Фон альбома')],
      places: 1,
      move: 'f1',
    },
  ]

  it('«Ещё:» — серая строка без мест, фон альбома в ней же', async () => {
    void open(withAlso)
    await shown()
    const note = screen.getByText(/^Ещё:/)
    expect(note.classList.contains('note')).toBe(true)
    expect(note.textContent).toContain('Ещё: Разворот 5 — фон, Фон альбома')
    expect(note.textContent).not.toContain('место')
  })

  it('«Ещё:» нет, когда кроме мест ничего нет', async () => {
    void open(one('f1'))
    await shown()
    expect(screen.queryByText(/^Ещё:/)).toBeNull()
  })

  it('ссылка «Разворот 5» в «Ещё» отменяет бросок и включает режим фона листа', async () => {
    const p = open(withAlso)
    await shown()
    await fireEvent.click(screen.getByText('Разворот 5', { selector: 'button' }))
    expect(await p).toBe('cancel')
    expect(dialog()).toBeNull()
    expect(goUse).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ kind: 'backdrop', spread: 's5', pick: 'spread' }))
  })

  it('несколько фото: «2 из 3 фото уже стоят», остальные встанут', async () => {
    void open(
      [
        { id: 'a', uses: [use(0, 'Разворот 1 — место')], places: 1, move: 'a' },
        { id: 'b', uses: [use(1, 'Разворот 2 — место')], places: 1, move: 'b' },
        { id: 'c', uses: [], places: 0, move: '' },
      ],
      ['a', 'b', 'c'],
    )
    await shown()
    expect(screen.getByText('2 из 3 фото уже стоят в альбоме')).toBeTruthy()
    expect(screen.getByText('Остальные фото встанут в любом случае.')).toBeTruthy()
  })
})
