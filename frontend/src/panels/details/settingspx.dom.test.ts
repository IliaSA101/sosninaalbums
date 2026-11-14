// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// Строка «Файл разворота при 300 dpi, px: Ш × В» в «Настройках альбома» (123, design §1.3): ввод → App.PagePx → поля формы,
// ошибка Go — строкой под полем, «Применить» — как всегда (один SetSettings). Go подменён.
const api = vi.hoisted(() => ({ PagePx: vi.fn(), SetSettings: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    PagePresets: vi.fn(async () => [{ label: '30×20', group: '', w: 304, h: 203 }]),
    MarginNote: vi.fn(async () => ''),
    ...api,
  }
})

const { default: SettingsForm } = await import('./SettingsForm.svelte')
const { store } = await import('../../base/core/store.svelte')

const settings = { pageW: 300, pageH: 300, dpi: 300, margin: 10, gap: 3, safe: 5, bleed: 3, bg: '' }
const tick = () => new Promise((r) => setTimeout(r, 0))
const px = (which: 'Ширина' | 'Высота') => screen.getByLabelText(`${which} файла разворота, px`) as HTMLInputElement
const page = (which: 'Ширина' | 'Высота') =>
  Array.from(document.querySelectorAll('label'))
    .find((l) => l.textContent?.includes(`${which} страницы, мм`))!
    .querySelector('input') as HTMLInputElement
const apply = () => screen.getByRole('button', { name: 'Применить' }) as HTMLButtonElement
const alertText = () => document.querySelector('[role="alert"]')?.textContent
// Ввод в поле и уход из него (change)
const type = async (el: HTMLInputElement, v: string) => {
  await fireEvent.input(el, { target: { value: v } })
  await fireEvent.change(el)
}
const show = (st: Record<string, unknown> = {}) => {
  store.view = { rev: 1, folders: [], spreads: [], settings: { ...settings, ...st } } as never
}

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.PagePx.mockResolvedValue({ pageW: 304, pageH: 203, err: '' })
  show()
})
afterEach(() => cleanup())

describe('строка «Файл разворота при 300 dpi, px»', () => {
  it('стоит под строкой «при экспорте … px»; поля — файл текущей страницы при 300 dpi', async () => {
    render(SettingsForm)
    await tick()
    const row = px('Ширина').closest('label')!
    expect(row.textContent).toContain('Файл разворота при 300 dpi, px: Ш × В')
    expect(px('Ширина').value).toBe('7087')
    expect(px('Высота').value).toBe('3543')
    const summary = Array.from(document.querySelectorAll('p.note')).find((p) => p.textContent?.includes('при экспорте'))!
    expect(summary.nextElementSibling).toBe(row)
  })
  it('dpi альбома 150: строка «при экспорте» по 150, файл типографии по-прежнему при 300 dpi', async () => {
    show({ dpi: 150 })
    render(SettingsForm)
    await tick()
    expect(px('Ширина').value).toBe('7087')
    expect(document.body.textContent).toMatch(/при экспорте 3543\s+×\s+1772 px/)
  })
  it('ввод 7181 × 2398 зовёт PagePx и ставит страницу Go в поля формы; «Применить» — один SetSettings', async () => {
    render(SettingsForm)
    await tick()
    await fireEvent.input(px('Ширина'), { target: { value: '7181' } })
    await type(px('Высота'), '2398') // уход из второго поля — один вызов с обоими числами
    expect(api.PagePx).toHaveBeenCalledTimes(1)
    expect(api.PagePx).toHaveBeenLastCalledWith(7181, 2398)
    await tick()
    expect(page('Ширина').value).toBe('304')
    expect(page('Высота').value).toBe('203')
    expect(apply().disabled).toBe(false)
    expect(api.SetSettings).not.toHaveBeenCalled() // ввод px проект не правит
    await fireEvent.click(apply())
    await tick()
    expect(api.SetSettings).toHaveBeenCalledTimes(1)
    expect(api.SetSettings.mock.calls[0][0]).toMatchObject({ pageW: 304, pageH: 203 })
  })
  it('Enter в поле — тоже ввод (уход из поля)', async () => {
    render(SettingsForm)
    await tick()
    await fireEvent.input(px('Ширина'), { target: { value: '7181' } })
    px('Ширина').focus()
    await fireEvent.keyDown(px('Ширина'), { key: 'Enter' })
    await fireEvent.change(px('Ширина'))
    expect(api.PagePx).toHaveBeenCalledWith(7181, 3543)
  })
  it('ошибка Go — строкой под полем, форма не меняется; правка страницы руками её убирает', async () => {
    api.PagePx.mockResolvedValue({ pageW: 0, pageH: 0, err: 'Разворот 90000 px при 300 dpi — страница больше 1000 мм' })
    render(SettingsForm)
    await tick()
    await type(px('Ширина'), '90000')
    await tick()
    expect(alertText()).toBe('Разворот 90000 px при 300 dpi — страница больше 1000 мм')
    expect(page('Ширина').value).toBe('300')
    expect(px('Ширина').value).toBe('90000') // введённое остаётся, чтобы его поправить
    expect(apply().disabled).toBe(true)
    await fireEvent.input(page('Ширина'), { target: { value: '310' } })
    await tick()
    expect(alertText()).toBeUndefined()
  })
  it('успешный ответ после ошибки убирает строку', async () => {
    api.PagePx.mockResolvedValueOnce({ pageW: 0, pageH: 0, err: 'плохо' })
    render(SettingsForm)
    await tick()
    await type(px('Ширина'), '1')
    await tick()
    expect(alertText()).toBe('плохо')
    await type(px('Ширина'), '7181')
    await tick()
    expect(alertText()).toBeUndefined()
    expect(page('Ширина').value).toBe('304')
  })
  it('мусор, ноль, дробь, пусто и то же число: Go не зовётся, поле возвращает прежнее', async () => {
    render(SettingsForm)
    await tick()
    for (const v of ['', '0', '-5', '12.5', '7087']) {
      await type(px('Ширина'), v)
      expect(px('Ширина').value, `«${v}»`).toBe('7087')
    }
    expect(api.PagePx).not.toHaveBeenCalled()
  })
  it('ноль без input-события (только change): поле всё равно возвращается к прежнему, Go не зовётся (как CoverPx)', async () => {
    render(SettingsForm)
    await tick()
    await fireEvent.change(px('Ширина'), { target: { value: '0' } })
    expect(px('Ширина').value).toBe('7087')
    await fireEvent.change(px('Высота'), { target: { value: '0' } })
    expect(px('Высота').value).toBe('3543')
    expect(api.PagePx).not.toHaveBeenCalled()
    expect(alertText()).toBeUndefined()
  })
  it('дробная страница 303,95 × 202,95: подпись без float-хвостов, с запятой', async () => {
    show({ pageW: 303.95, pageH: 202.95 })
    render(SettingsForm)
    await tick()
    const note = Array.from(document.querySelectorAll('p.note')).find((p) => p.textContent?.includes('при экспорте'))!
    expect(note.textContent).toMatch(/Разворот = две страницы: 60,79 × 20,29 см/)
    expect(note.textContent).not.toContain('9999')
  })
  it('«Применить» сразу после ввода ждёт ответ Go и уходит с новой страницей', async () => {
    let resolve: (v: unknown) => void = () => {}
    api.PagePx.mockReturnValue(new Promise((r) => (resolve = r)))
    render(SettingsForm)
    await tick()
    await type(px('Ширина'), '7181')
    await fireEvent.input(page('Высота'), { target: { value: '250' } }) // чтобы кнопка была живой до ответа
    await fireEvent.click(apply())
    await tick()
    expect(api.SetSettings).not.toHaveBeenCalled()
    resolve({ pageW: 304, pageH: 203, err: '' })
    await tick()
    await tick()
    expect(api.SetSettings).toHaveBeenCalledTimes(1)
    expect(api.SetSettings.mock.calls[0][0]).toMatchObject({ pageW: 304, pageH: 203 })
  })
  it('два ввода подряд, ответы наоборот: считается последний', async () => {
    const waits: ((v: unknown) => void)[] = []
    api.PagePx.mockImplementation(() => new Promise((r) => waits.push(r)))
    render(SettingsForm)
    await tick()
    await type(px('Ширина'), '7181')
    await type(px('Ширина'), '7100')
    waits[1]({ pageW: 301, pageH: 300, err: '' })
    await tick()
    waits[0]({ pageW: 304, pageH: 203, err: '' })
    await tick()
    expect(page('Ширина').value).toBe('301')
  })
  it('кнопка формата меняет и поля px; Сбросить возвращает прежние', async () => {
    render(SettingsForm)
    await tick()
    await fireEvent.click(await screen.findByRole('button', { name: '30×20' }))
    await tick()
    expect(px('Ширина').value).toBe('7181')
    expect(px('Высота').value).toBe('2398')
    await fireEvent.click(screen.getByRole('button', { name: 'Сбросить' }))
    await tick()
    expect(px('Ширина').value).toBe('7087')
  })
  it('страница на 1 px от шаблона: кнопка 30×20 подсвечена (допуск 0,25 мм)', async () => {
    show({ pageW: 303.95, pageH: 202.95 })
    render(SettingsForm)
    await tick()
    const b = await screen.findByRole('button', { name: '30×20' })
    expect(b.classList.contains('active')).toBe(true)
  })
  it('лист виньетки и неверная форма: строки нет', async () => {
    show({ sheet: true })
    render(SettingsForm)
    await tick()
    expect(screen.queryByLabelText('Ширина файла разворота, px')).toBeNull()
    cleanup()
    show()
    render(SettingsForm)
    await tick()
    await fireEvent.input(page('Ширина'), { target: { value: '' } })
    await tick()
    expect(screen.queryByLabelText('Ширина файла разворота, px')).toBeNull()
    expect(document.body.textContent).toContain('Проверьте значения')
  })
})
