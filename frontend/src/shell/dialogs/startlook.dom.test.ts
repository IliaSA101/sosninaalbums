// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// 131, вопрос 19: «Альбом по макету…» стоит в ряду изделий после «Новый альбом», создаёт новый альбом и открывает «Макеты альбома».
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ Recent: async () => [] }))

const { default: StartDialog } = await import('./StartDialog.svelte')
const { default: NewProduct } = await import('../newdoc/NewProduct.svelte')
const { looks } = await import('../looks/looks.svelte')
const { newDoc } = await import('../newdoc/newdoc.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')

const label = 'Альбом по макету…'
const order: string[] = []
let show: ReturnType<typeof vi.spyOn>
let create: ReturnType<typeof vi.spyOn>
beforeEach(() => {
  order.length = 0
  store.loaded = true
  store.assoc = { can: true, on: false, ask: false }
  prefs.startOpen = false
  newDoc.kind = 'album'
  show = vi.spyOn(looks, 'show').mockImplementation(async () => void order.push('show'))
  create = vi.spyOn(newDoc, 'create').mockImplementation(async () => (order.push('create'), true))
})
afterEach(() => {
  cleanup()
  prefs.startOpen = false
  vi.restoreAllMocks()
})

const names = () => screen.getAllByRole('button').map((b) => b.textContent?.trim())

describe('«Альбом по макету…» в ряду изделий', () => {
  it('стоит сразу после «Новый альбом», в стартовом окне — не в ряду «Продолжить · Открыть…»', () => {
    prefs.startOpen = true
    render(StartDialog)
    const n = names()
    expect(n[n.indexOf('Новый альбом') + 1]).toBe(label)
    expect(n[n.indexOf('Открыть…') + 1]).not.toBe(label)
  })

  it('есть на «Простом» (макеты — с первого уровня, вопрос 20)', () => {
    prefs.level = 'simple' as never
    render(NewProduct, { oncreated: () => {} })
    expect(screen.getByText(label)).toBeTruthy()
  })

  it('создаёт новый альбом, закрывает стартовое окно и только потом открывает макеты', async () => {
    const oncreated = vi.fn(() => void order.push('created'))
    newDoc.kind = 'sheet'
    render(NewProduct, { oncreated })
    await fireEvent.click(screen.getByText(label))
    await vi.waitFor(() => expect(show).toHaveBeenCalledTimes(1))
    expect(newDoc.kind).toBe('album') // был выбран лист-виньетка — макет ставится на альбом
    expect(create).toHaveBeenCalledTimes(1)
    expect(order).toEqual(['create', 'created', 'show'])
  })

  it('альбом не создан (отказ Go) — стартовое окно остаётся, макеты не открываются', async () => {
    create.mockResolvedValue(false)
    const oncreated = vi.fn()
    render(NewProduct, { oncreated })
    await fireEvent.click(screen.getByText(label))
    await vi.waitFor(() => expect(create).toHaveBeenCalledTimes(1))
    expect(oncreated).not.toHaveBeenCalled()
    expect(show).not.toHaveBeenCalled()
  })

  it('пока идёт создание — кнопка серая (повторный щелчок не создаёт второй альбом)', () => {
    newDoc.busy = true
    render(NewProduct, { oncreated: () => {} })
    expect((screen.getByText(label) as HTMLButtonElement).disabled).toBe(true)
    newDoc.busy = false
  })
})
