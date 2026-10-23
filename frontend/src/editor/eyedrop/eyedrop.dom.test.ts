// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'

// Пипетка на холсте (109, design §7): слой над листом окна цвета, лупа, щелчок берёт цвет у Go, ошибка — выход и тост.
const api = vi.hoisted(() => ({
  PickColorAt: vi.fn(),
  MyColors: vi.fn(),
  ImportColors: vi.fn(),
  RememberColor: vi.fn(),
  ForgetColor: vi.fn(),
  SpreadPhotoColors: vi.fn(),
}))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))

const { default: EyedropLayer } = await import('./EyedropLayer.svelte')
const { default: EyedropBar } = await import('./EyedropBar.svelte')
const { default: ColorPicker } = await import('../../base/color/ColorPicker.svelte')
const { eyedrop } = await import('../../base/color/eyedrop.svelte')
const { mycolors } = await import('../../base/color/mycolors.svelte')
const { store } = await import('../../base/core/store.svelte')
const { canvas } = await import('../canvas/canvas.svelte')

const spread = { id: 's1', w: 600, h: 300 } as never
// Слой на экране: левый верхний угол (100, 50), ширина 300 px — значит 2 мм на пиксель.
const RECT = { left: 100, top: 50, width: 300, height: 150, right: 400, bottom: 200, x: 100, y: 50 }

beforeEach(async () => {
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({ ...RECT, toJSON: () => RECT })
  api.PickColorAt.mockResolvedValue('#aabbcc')
  api.MyColors.mockResolvedValue([])
  api.RememberColor.mockImplementation(async (h: string) => [h])
  api.SpreadPhotoColors.mockResolvedValue([])
  await mycolors.load()
  canvas.space = false
})
afterEach(() => {
  cleanup()
  eyedrop.cancel()
  store.toast = null
  vi.clearAllMocks()
  vi.restoreAllMocks()
})

const layer = () => document.querySelector('.layer') as HTMLElement
const loupe = () => document.body.querySelector('.loupe')
const at = (x: number, y: number) => ({ clientX: x, clientY: y, button: 0 })
async function mount(done = vi.fn()) {
  const owner = {}
  eyedrop.start(owner, 's1', done)
  render(EyedropLayer, { spread })
  await tick()
  return { owner, done }
}

describe('пипетка: щелчок', () => {
  it('щелчок по листу — PickColorAt с мм листа и finish', async () => {
    const { done } = await mount()
    await fireEvent.pointerDown(layer(), at(250, 100)) // (150, 50) px от угла → (300, 100) мм
    await tick()
    expect(api.PickColorAt).toHaveBeenCalledExactlyOnceWith('s1', 300, 100)
    expect(done).toHaveBeenCalledExactlyOnceWith('#aabbcc')
    expect(eyedrop.on).toBe(false)
    expect(store.toast).toBeNull()
  })

  it('слой помечен data-text-ui: нажатие на него не снимает выделение текста (TextLayer.keepsSelection)', async () => {
    await mount()
    expect(layer().closest('[data-text-ui]')).not.toBeNull()
  })

  it('щелчок не выделяет место под слоем: pointerdown, click, dblclick и меню дальше не идут', async () => {
    const seen = vi.fn()
    const host = document.createElement('div')
    document.body.append(host)
    for (const t of ['pointerdown', 'click', 'dblclick', 'contextmenu']) document.body.addEventListener(t, seen)
    eyedrop.start({}, 's1', vi.fn())
    render(EyedropLayer, { target: host, props: { spread } })
    await tick()
    await fireEvent.pointerDown(layer(), at(250, 100))
    await fireEvent.click(layer())
    await fireEvent.dblClick(layer())
    await fireEvent.contextMenu(layer())
    expect(seen).not.toHaveBeenCalled()
    host.remove()
    for (const t of ['pointerdown', 'click', 'dblclick', 'contextmenu']) document.body.removeEventListener(t, seen)
  })

  it('Пробел-рука и правая кнопка: слой молчит, событие идёт к холсту', async () => {
    const seen = vi.fn()
    const host = document.createElement('div')
    document.body.append(host)
    document.body.addEventListener('pointerdown', seen)
    eyedrop.start({}, 's1', vi.fn())
    render(EyedropLayer, { target: host, props: { spread } })
    await tick()
    canvas.space = true
    await fireEvent.pointerDown(layer(), at(250, 100))
    canvas.space = false
    await fireEvent.pointerDown(layer(), { ...at(250, 100), button: 2 })
    expect(seen).toHaveBeenCalledTimes(2)
    expect(api.PickColorAt).not.toHaveBeenCalled()
    expect(eyedrop.on).toBe(true)
    host.remove()
    document.body.removeEventListener('pointerdown', seen)
  })

  it('ошибка Go — выход из режима и тост', async () => {
    api.PickColorAt.mockRejectedValue('точка вне листа')
    const { done } = await mount()
    await fireEvent.pointerDown(layer(), at(250, 100))
    await tick()
    expect(done).not.toHaveBeenCalled()
    expect(eyedrop.on).toBe(false)
    expect(store.toast).toMatchObject({ text: 'Точка вне листа', kind: 'error' })
  })

  it('второй щелчок, пока Go считает, не шлёт второй запрос', async () => {
    let ok: (h: string) => void = () => {}
    api.PickColorAt.mockReturnValue(new Promise<string>((r) => (ok = r)))
    const { done } = await mount()
    await fireEvent.pointerDown(layer(), at(250, 100))
    await fireEvent.pointerDown(layer(), at(260, 110))
    expect(api.PickColorAt).toHaveBeenCalledTimes(1)
    ok('#010203')
    await tick()
    expect(done).toHaveBeenCalledExactlyOnceWith('#010203')
  })

  it('режим сняли, пока Go считал, — цвет не отдаётся', async () => {
    let ok: (h: string) => void = () => {}
    api.PickColorAt.mockReturnValue(new Promise<string>((r) => (ok = r)))
    const { done } = await mount()
    await fireEvent.pointerDown(layer(), at(250, 100))
    eyedrop.cancel()
    ok('#010203')
    await tick()
    expect(done).not.toHaveBeenCalled()
  })
})

describe('пипетка: лупа', () => {
  it('движение — лупа с «…» до ответа, потом HEX; уход с листа убирает', async () => {
    let ok: (h: string) => void = () => {}
    api.PickColorAt.mockReturnValue(new Promise<string>((r) => (ok = r)))
    await mount()
    expect(loupe()).toBeNull()
    await fireEvent.pointerMove(layer(), at(250, 100))
    expect(loupe()?.textContent).toBe('…')
    expect(api.PickColorAt).toHaveBeenCalledExactlyOnceWith('s1', 300, 100)
    ok('#123456')
    await tick()
    expect(loupe()?.textContent).toBe('#123456')
    await fireEvent.pointerLeave(layer())
    expect(loupe()).toBeNull()
  })

  it('очередь: один запрос в пути, следующий берёт последнюю точку', async () => {
    const waits: ((h: string) => void)[] = []
    api.PickColorAt.mockImplementation(() => new Promise<string>((r) => waits.push(r)))
    await mount()
    await fireEvent.pointerMove(layer(), at(110, 60)) // (20, 20)
    await fireEvent.pointerMove(layer(), at(120, 70))
    await fireEvent.pointerMove(layer(), at(130, 80)) // (60, 60) — последняя
    expect(api.PickColorAt).toHaveBeenCalledTimes(1)
    waits[0]('#111111')
    await tick()
    expect(api.PickColorAt).toHaveBeenCalledTimes(2)
    expect(api.PickColorAt).toHaveBeenLastCalledWith('s1', 60, 60)
    waits[1]('#222222')
    await tick()
    expect(loupe()?.textContent).toBe('#222222')
    expect(api.PickColorAt).toHaveBeenCalledTimes(2)
  })

  it('ошибка лупы режим не снимает: «…»', async () => {
    api.PickColorAt.mockRejectedValue('точка вне листа')
    await mount()
    await fireEvent.pointerMove(layer(), at(250, 100))
    await tick()
    expect(eyedrop.on).toBe(true)
    expect(loupe()?.textContent).toBe('…')
    expect(store.toast).toBeNull()
  })

  it('слой ушёл — лупа убрана из body', async () => {
    await mount()
    await fireEvent.pointerMove(layer(), at(250, 100))
    expect(loupe()).not.toBeNull()
    cleanup()
    expect(loupe()).toBeNull()
  })
})

describe('пипетка: Esc и полоса', () => {
  it('Esc выходит из режима', async () => {
    await mount()
    await fireEvent.keyDown(window, { key: 'Escape' })
    expect(eyedrop.on).toBe(false)
  })

  it('Esc без режима слой не трогает событие', async () => {
    render(EyedropLayer, { spread })
    const e = new KeyboardEvent('keydown', { key: 'Escape', cancelable: true })
    window.dispatchEvent(e)
    expect(e.defaultPrevented).toBe(false)
  })

  // Один Esc — одно действие: выход из режима; окно закрывает только следующий Esc. Порядок слушателей не важен.
  it.each([
    ['окно раньше слоя', false],
    ['слой раньше окна', true],
  ])('один Esc: режим снят, окно цвета открыто (%s); второй Esc закрывает', async (_n, layerFirst) => {
    if (layerFirst) render(EyedropLayer, { spread })
    render(ColorPicker, { value: '#ffffff', label: 'Цвет фона', onpick: vi.fn(), sheet: 's1' })
    await fireEvent.click(screen.getByRole('button', { name: 'Цвет фона' }))
    flushSync()
    await fireEvent.click(screen.getByRole('button', { name: /Взять цвет с фото/ }))
    if (!layerFirst) render(EyedropLayer, { spread })
    await tick()
    expect(eyedrop.on).toBe(true)
    await fireEvent.keyDown(window, { key: 'Escape' })
    flushSync()
    expect(eyedrop.on).toBe(false)
    expect(screen.getByRole('dialog')).toBeTruthy()
    await fireEvent.keyDown(window, { key: 'Escape' })
    flushSync()
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('щелчок по листу при открытом окне: цвет выбран, окно закрыто, режим снят', async () => {
    const onpick = vi.fn()
    render(ColorPicker, { value: '#ffffff', label: 'Цвет фона', onpick, sheet: 's1' })
    await fireEvent.click(screen.getByRole('button', { name: 'Цвет фона' }))
    flushSync()
    await fireEvent.click(screen.getByRole('button', { name: /Взять цвет с фото/ }))
    render(EyedropLayer, { spread })
    await tick()
    await fireEvent.pointerDown(layer(), at(250, 100))
    await tick()
    flushSync()
    expect(onpick).toHaveBeenCalledExactlyOnceWith('#aabbcc')
    expect(eyedrop.on).toBe(false)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('полоса: подсказка и «Отмена» снимает режим', async () => {
    eyedrop.start({}, 's1', vi.fn())
    render(EyedropBar)
    expect(screen.getByText(/возьмём цвет, каким он выйдет в печати/)).toBeTruthy()
    await fireEvent.click(screen.getByRole('button', { name: 'Отмена' }))
    expect(eyedrop.on).toBe(false)
  })
})

describe('пипетка: смена листа', () => {
  it('другой разворот текущий — режим снят; тот же — идёт; слой над чужим листом не нужен', async () => {
    await mount()
    eyedrop.check('s1')
    expect(eyedrop.on).toBe(true)
    eyedrop.check('s2')
    expect(eyedrop.on).toBe(false)
  })
})
