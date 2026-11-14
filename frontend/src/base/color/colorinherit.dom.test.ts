// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Строка «Как в альбоме» в окне цвета (115 В): отметка и шапка, показ при наведении, щелчок без «Своих».
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
const { tip } = await import('../ui/tip.svelte')

const ALBUM = '#e1ebd8'

beforeEach(async () => {
  localStorage.clear()
  api.MyColors.mockResolvedValue([])
  api.RememberColor.mockImplementation(async (h: string) => [h])
  api.ForgetColor.mockResolvedValue([])
  api.SpreadPhotoColors.mockResolvedValue([])
  await mycolors.load()
  mycolors.list = []
})
afterEach(() => {
  cleanup()
  tip.hide()
  vi.clearAllMocks()
})

type Props = { value?: string; inherit?: { hex: string; on: boolean }; oninherit?: () => unknown }
async function open(props: Props = {}) {
  const onpick = vi.fn()
  const onpreview = vi.fn()
  const oninherit = props.oninherit ?? vi.fn()
  render(ColorPicker, {
    value: props.value ?? '#4f7a5a',
    label: 'Цвет фона',
    onpick,
    onpreview,
    inherit: props.inherit,
    oninherit,
  })
  await fireEvent.click(screen.getByRole('button', { name: 'Цвет фона' }))
  flushSync()
  return { onpick, onpreview, oninherit }
}
const row = () => screen.queryByRole('button', { name: /Как в альбоме/ })
const pop = () => screen.queryByRole('dialog')

describe('ColorPicker: строка «Как в альбоме»', () => {
  it('без inherit строки нет', async () => {
    await open()
    expect(row()).toBeNull()
  })

  it('есть inherit, но нет oninherit — строки нет (нечем снимать)', async () => {
    render(ColorPicker, { value: '#fff', label: 'Цвет фона', onpick: vi.fn(), inherit: { hex: ALBUM, on: false } })
    await fireEvent.click(screen.getByRole('button', { name: 'Цвет фона' }))
    flushSync()
    expect(row()).toBeNull()
  })

  it('свой цвет: строка не отмечена, показывает имя набора и HEX, шапка называет сам цвет', async () => {
    await open({ inherit: { hex: ALBUM, on: false } })
    expect(row()?.getAttribute('aria-pressed')).toBe('false')
    expect(row()?.textContent).toContain(ALBUM)
    expect(row()?.textContent).not.toContain('✓')
    expect(pop()?.querySelector('.name b')?.textContent).toBe('Свой цвет')
  })

  it('цвет альбома не из наборов — серая строка только с HEX, без «Свой цвет»', async () => {
    await open({ inherit: { hex: '#123456', on: false } })
    expect(row()?.querySelector('.sub')?.textContent).toBe('#123456')
  })

  it('цвет альбома из наборов — имя и HEX', async () => {
    await open({ inherit: { hex: '#4a4643', on: false } })
    expect(row()?.querySelector('.sub')?.textContent).toBe('Графит · #4a4643')
  })

  it('своего цвета нет: aria-pressed=true, ✓ и шапка «Как в альбоме» с HEX', async () => {
    await open({ value: ALBUM, inherit: { hex: ALBUM, on: true } })
    expect(row()?.getAttribute('aria-pressed')).toBe('true')
    expect(row()?.textContent).toContain('✓')
    expect(pop()?.querySelector('.name b')?.textContent).toBe('Как в альбоме')
    expect(pop()?.querySelector('.name span')?.textContent).toBe(ALBUM)
  })

  it('наведение и фокус показывают цвет альбома, уход снимает', async () => {
    const { onpreview } = await open({ inherit: { hex: ALBUM, on: false } })
    await fireEvent.pointerEnter(row()!)
    expect(onpreview).toHaveBeenLastCalledWith(ALBUM)
    await fireEvent.pointerLeave(row()!)
    expect(onpreview).toHaveBeenLastCalledWith(null)
    await fireEvent.focus(row()!)
    expect(onpreview).toHaveBeenLastCalledWith(ALBUM)
    await fireEvent.blur(row()!)
    expect(onpreview).toHaveBeenLastCalledWith(null)
  })

  it('щелчок при своём цвете: один oninherit, окно закрыто, в «Свои» ничего, onpick не звали', async () => {
    const { onpick, oninherit } = await open({ inherit: { hex: ALBUM, on: false } })
    await fireEvent.click(row()!)
    flushSync()
    expect(oninherit).toHaveBeenCalledTimes(1)
    expect(onpick).not.toHaveBeenCalled()
    expect(pop()).toBeNull()
    expect(api.RememberColor).not.toHaveBeenCalled()
  })

  it('показ держится до ответа Go, потом снимается', async () => {
    let answer!: () => void
    const oninherit = vi.fn(() => new Promise<void>((r) => (answer = r)))
    const { onpreview } = await open({ inherit: { hex: ALBUM, on: false }, oninherit })
    await fireEvent.pointerEnter(row()!)
    await fireEvent.click(row()!)
    flushSync()
    await fireEvent.pointerLeave(document.body)
    expect(onpreview).toHaveBeenLastCalledWith(ALBUM)
    answer()
    await vi.waitFor(() => expect(onpreview).toHaveBeenLastCalledWith(null))
  })

  it('щелчок по отмеченной: без вызова, окно закрыто', async () => {
    const { onpick, oninherit } = await open({ value: ALBUM, inherit: { hex: ALBUM, on: true } })
    await fireEvent.click(row()!)
    flushSync()
    expect(oninherit).not.toHaveBeenCalled()
    expect(onpick).not.toHaveBeenCalled()
    expect(pop()).toBeNull()
  })

  it('образец палитры рядом по-прежнему выбирает цвет и запоминает его', async () => {
    const { onpick, oninherit } = await open({ inherit: { hex: ALBUM, on: false } })
    await fireEvent.click(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    expect(onpick).toHaveBeenCalledWith('#4a4643')
    expect(oninherit).not.toHaveBeenCalled()
    expect(api.RememberColor).toHaveBeenCalledTimes(1)
  })
})
