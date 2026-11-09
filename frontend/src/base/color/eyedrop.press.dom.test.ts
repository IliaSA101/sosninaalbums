// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Щелчок мимо листа в режиме пипетки (109 п. 4, 109-10) при реальном порядке слушателей: снятие выделения стоит на
// window в захвате (как TextLayer) и зарегистрировано раньше окна цвета — гасить надо до него, а не в окне.
const api = vi.hoisted(() => ({
  MyColors: vi.fn(),
  ImportColors: vi.fn(),
  RememberColor: vi.fn(),
  ForgetColor: vi.fn(),
  SpreadPhotoColors: vi.fn(),
}))
vi.mock('../../../wailsjs/go/app/App', () => api)

const { default: ColorPicker } = await import('./ColorPicker.svelte')
const { mycolors } = await import('./mycolorlist.svelte')
const { eyedrop } = await import('./eyedrop.svelte')

const clearSel = vi.fn()
const onDown = () => clearSel()
const onClick = () => clearSel()

beforeEach(async () => {
  localStorage.clear()
  api.MyColors.mockResolvedValue([])
  api.SpreadPhotoColors.mockResolvedValue([])
  await mycolors.load()
  mycolors.list = []
  clearSel.mockClear()
  // Слушатели снятия выделения — раньше окна цвета, в захвате на window (pointerdown, click); mousedown после погашенного pointerdown браузер не шлёт.
  window.addEventListener('pointerdown', onDown, true)
  window.addEventListener('click', onClick, true)
})
afterEach(() => {
  window.removeEventListener('pointerdown', onDown, true)
  window.removeEventListener('click', onClick, true)
  cleanup()
  eyedrop.cancel()
  vi.clearAllMocks()
})

async function inMode() {
  render(ColorPicker, { value: '#ffffff', label: 'Цвет текста', onpick: vi.fn(), onpreview: vi.fn(), sheet: 's1' })
  await fireEvent.click(screen.getByRole('button', { name: 'Цвет текста' }))
  flushSync()
  await fireEvent.click(screen.getByRole('button', { name: /Взять цвет с фото/ }))
  flushSync()
  expect(eyedrop.on).toBe(true)
  clearSel.mockClear() // клики по кнопкам окна — не «мимо»
}

describe('пипетка: щелчок мимо листа, реальный порядок слушателей', () => {
  it('нажатие и щелчок мимо до снятия выделения не доходят; режим снят, окно открыто', async () => {
    await inMode()
    const row = document.createElement('div') // как .row под листом
    document.body.appendChild(row)
    await fireEvent.pointerDown(row)
    await fireEvent.click(row)
    flushSync()
    expect(clearSel).not.toHaveBeenCalled()
    expect(eyedrop.on).toBe(false)
    expect(screen.getByRole('dialog')).toBeTruthy()
    row.remove()
  })

  it('следующий щелчок мимо обычный: выделение снимается, окно закрыто', async () => {
    await inMode()
    await fireEvent.pointerDown(document.body)
    await fireEvent.click(document.body)
    clearSel.mockClear()
    await fireEvent.pointerDown(document.body)
    await fireEvent.click(document.body)
    flushSync()
    expect(clearSel).toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('нажатие без щелчка (увели курсор): следующий щелчок не глушится', async () => {
    await inMode()
    await fireEvent.pointerDown(document.body)
    await fireEvent.pointerDown(document.body) // нового нажатия — сброс глушилки
    clearSel.mockClear()
    await fireEvent.click(document.body)
    expect(clearSel).toHaveBeenCalled()
  })

  it('нажатие по окну цвета и по слою пипетки — не «мимо»', async () => {
    await inMode()
    await fireEvent.pointerDown(screen.getByRole('dialog'))
    expect(eyedrop.on).toBe(true)
    const layer = document.createElement('div')
    layer.dataset.eyedrop = ''
    document.body.appendChild(layer)
    await fireEvent.pointerDown(layer)
    expect(eyedrop.on).toBe(true)
    layer.remove()
  })

  it('без режима пипетки eyedrop не вмешивается', async () => {
    render(ColorPicker, { value: '#ffffff', label: 'Цвет текста', onpick: vi.fn(), onpreview: vi.fn(), sheet: 's1' })
    await fireEvent.pointerDown(document.body)
    await fireEvent.click(document.body)
    expect(clearSel).toHaveBeenCalled()
  })
})
