// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// «Применить» в настройках альбома при смене цвета и своих цветах у разворотов (115 А): вопрос до правки,
// «Оставить свой» — прежний SetSettings, «Как в альбоме» — SetSettingsToAll, × — правки нет, черновик остаётся.
const api = vi.hoisted(() => ({
  SetSettings: vi.fn(),
  SetSettingsToAll: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    PagePresets: vi.fn(async () => []),
    MarginNote: vi.fn(async () => ''),
    MyColors: vi.fn(async () => []),
    RememberColor: vi.fn(async (h: string) => [h]),
    ...api,
  }
})

const { default: SettingsForm } = await import('./SettingsForm.svelte')
const { store } = await import('../../base/core/store.svelte')

const settings = { pageW: 300, pageH: 300, dpi: 300, margin: 10, gap: 3, safe: 5, bleed: 3, bg: '' }
const sp = (id: string, bgSet: [string, string] = ['', '']) => ({
  id,
  photos: [],
  frames: [],
  texts: [],
  captions: [],
  bg: ['#ffffff', '#ffffff'],
  bgSet,
  backdrops: null,
  n: 0,
})
const show = (spreads: ReturnType<typeof sp>[], st: Record<string, unknown> = {}) => {
  store.view = { rev: 4, folders: [], spreads, settings: { ...settings, ...st } } as never
  store.currentId = spreads[0]?.id ?? ''
}
const OWN = [sp('s1'), sp('s2', ['#4f7a5a', '']), sp('s3'), sp('s4', ['', '#111111'])]
// Ответ Go: новый rev и принятые настройки
const answer = (st: Record<string, unknown>) => async () => ({ ...store.view, rev: store.view.rev + 1, settings: st })

const pickColor = async (hex: string, label = 'Цвет страниц альбома') => {
  await fireEvent.click(screen.getByRole('button', { name: label }))
  flushSync()
  await fireEvent.click(screen.getAllByRole('button', { name: new RegExp(hex) })[0])
  flushSync()
}
const apply = async () => {
  await fireEvent.click(screen.getByRole('button', { name: 'Применить' }))
  flushSync()
}
const dlg = () => screen.queryByRole('dialog', { name: 'Развороты со своим цветом' })

beforeEach(() => {
  api.SetSettings.mockReset()
  api.SetSettingsToAll.mockReset()
  api.SetSettings.mockImplementation(answer({ ...settings, bg: '#4a4643' }))
  api.SetSettingsToAll.mockImplementation(answer({ ...settings, bg: '#4a4643' }))
})
afterEach(() => cleanup())

describe('SettingsForm: вопрос про развороты со своим цветом', () => {
  it('цвет сменён, свои есть — вопрос с номерами, фокус на «Оставить свой», правок нет', async () => {
    show(OWN)
    render(SettingsForm)
    await pickColor('#4a4643')
    await apply()
    expect(dlg()).not.toBeNull()
    expect(dlg()?.textContent).toContain('У 2 разворотов свой цвет: 2, 4. Новый цвет альбома их не коснётся. Сделать их как в альбоме?')
    await vi.waitFor(() => expect(document.activeElement?.textContent?.trim()).toBe('Оставить свой'))
    expect(api.SetSettings).not.toHaveBeenCalled()
    expect(api.SetSettingsToAll).not.toHaveBeenCalled()
  })

  it('«Оставить свой» — SetSettings один раз с черновиком, SetSettingsToAll не звали', async () => {
    show(OWN)
    render(SettingsForm)
    await pickColor('#4a4643')
    await apply()
    await fireEvent.click(screen.getByRole('button', { name: 'Оставить свой' }))
    await vi.waitFor(() => expect(api.SetSettings).toHaveBeenCalledTimes(1))
    expect(api.SetSettings.mock.calls[0][0]).toMatchObject({ bg: '#4a4643', pageW: 300 })
    expect(api.SetSettingsToAll).not.toHaveBeenCalled()
    expect(dlg()).toBeNull()
  })

  it('«Как в альбоме» — SetSettingsToAll один раз с черновиком, тост с числом', async () => {
    show(OWN)
    render(SettingsForm)
    await pickColor('#4a4643')
    await apply()
    await fireEvent.click(screen.getByRole('button', { name: 'Как в альбоме' }))
    await vi.waitFor(() => expect(api.SetSettingsToAll).toHaveBeenCalledTimes(1))
    expect(api.SetSettingsToAll.mock.calls[0][0]).toMatchObject({ bg: '#4a4643', pageW: 300 })
    expect(api.SetSettings).not.toHaveBeenCalled()
    await vi.waitFor(() => expect(store.toast?.text).toBe('Цвет альбома — теперь и на 2 разворотах'))
    expect(dlg()).toBeNull()
  })

  it('ошибка Go на «Как в альбоме» — красный тост, «теперь и на» не показан', async () => {
    api.SetSettingsToAll.mockRejectedValue(new Error('нет'))
    show(OWN)
    render(SettingsForm)
    await pickColor('#4a4643')
    await apply()
    await fireEvent.click(screen.getByRole('button', { name: 'Как в альбоме' }))
    await vi.waitFor(() => expect(store.toast?.kind).toBe('error'))
  })

  it('× и Esc — вызовов нет, цвет в форме остался, можно применить снова', async () => {
    show(OWN)
    render(SettingsForm)
    await pickColor('#4a4643')
    await apply()
    await fireEvent.click(screen.getByRole('button', { name: 'Закрыть' }))
    flushSync()
    expect(dlg()).toBeNull()
    expect(api.SetSettings).not.toHaveBeenCalled()
    expect(api.SetSettingsToAll).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Цвет страниц альбома' }).style.getPropertyValue('--c')).toBe('#4a4643')
    await apply()
    expect(dlg()).not.toBeNull()
    await fireEvent.keyDown(window, { key: 'Escape' })
    flushSync()
    expect(dlg()).toBeNull()
    expect(api.SetSettings).not.toHaveBeenCalled()
  })

  it('цвет тот же, правили поля — вопроса нет, SetSettings сразу', async () => {
    show(OWN)
    render(SettingsForm)
    await fireEvent.input(screen.getByLabelText('Поля, мм'), { target: { value: '12' } })
    await apply()
    expect(dlg()).toBeNull()
    await vi.waitFor(() => expect(api.SetSettings).toHaveBeenCalledTimes(1))
    expect(api.SetSettingsToAll).not.toHaveBeenCalled()
  })

  it('белый записан по-разному («» и «#FFFFFF») — это не смена цвета', async () => {
    show(OWN, { bg: '#FFFFFF' })
    render(SettingsForm)
    await fireEvent.input(screen.getByLabelText('Поля, мм'), { target: { value: '12' } })
    await apply()
    expect(dlg()).toBeNull()
    await vi.waitFor(() => expect(api.SetSettings).toHaveBeenCalledTimes(1))
  })

  it('своих цветов нет — вопроса нет, SetSettings сразу', async () => {
    show([sp('s1'), sp('s2')])
    render(SettingsForm)
    await pickColor('#4a4643')
    await apply()
    expect(dlg()).toBeNull()
    await vi.waitFor(() => expect(api.SetSettings).toHaveBeenCalledTimes(1))
    expect(api.SetSettingsToAll).not.toHaveBeenCalled()
  })

  it('проект из листов: «Листы со своим цветом», слово «листов»', async () => {
    show([sp('s1', ['#4f7a5a', '']), sp('s2')], { sheet: true })
    render(SettingsForm)
    await pickColor('#4a4643', 'Цвет листов')
    await apply()
    const d = screen.getByRole('dialog', { name: 'Листы со своим цветом' })
    expect(d.textContent).toContain('У 1 листа свой цвет: 1. Новый цвет альбома его не коснётся.')
  })
})
