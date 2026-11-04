// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// Окно «Применить макет «…»» поверх просмотра (163 U3, 163-4 а, 163-27): «Что сделать», предупреждение по плану,
// «Отмена» и Esc назад в просмотр, Enter, второе нажатие при изменившемся плане.
const api = vi.hoisted(() => ({
  catalog: vi.fn(),
  info: vi.fn(),
  count: vi.fn(),
  plan: vi.fn(),
  apply: vi.fn(),
  rename: vi.fn(),
  hide: vi.fn(),
}))
vi.mock('./lookapi', () => ({ lookApi: api }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])) }
})

const { default: LooksDialog } = await import('./LooksDialog.svelte')
const { looks } = await import('./looks.svelte')
const { store } = await import('../../base/core/store.svelte')
const { modalStack } = await import('../../base/modal/modalstack.svelte')

const A = 'my:0a1b2c3d4e5f6071'
const item = { id: A, name: 'Светлая', theme: 'w', url: '', own: false, aspect: 0, w: 0, h: 0, steps: 4, from: 10 }
const info = () => ({
  steps: Array.from({ length: 4 }, (_, k) => ({ name: `Шаг ${k + 1}`, role: '', places: 3, pages: 2 })),
  cover: false,
  min: 1,
  max: 20,
})
const plan = (o: Record<string, unknown> = {}) => ({
  mode: 'series',
  trial: false,
  touched: 0,
  full: false,
  spreads: 4,
  places: 12,
  drafts: 0,
  replaced: 0,
  anchors: 0,
  cover: false,
  coverTouched: false,
  coverReplaced: false,
  left: 0,
  prev: '',
  missing: 0,
  ...o,
})
type Opts = { mode: string; spreads: number }

const text = () => document.body.textContent ?? ''
const btn = (label: string) =>
  Array.from(document.querySelectorAll('button'))
    .filter((b) => b.textContent?.trim() === label)
    .at(-1) as HTMLButtonElement
const win = () => document.querySelector<HTMLElement>('[aria-label^="Применить макет"]')
const spreadsInput = () => document.querySelector<HTMLInputElement>('#look-spreads')
const key = (k: string, init: KeyboardEventInit, on: Element) => {
  const e = new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true, ...init })
  on.dispatchEvent(e)
  return e
}

async function opened(p: (o: Opts) => unknown = (o) => plan({ mode: o.mode, spreads: o.spreads })) {
  api.plan.mockImplementation(async (_i: string, o: Opts) => p(o))
  api.catalog.mockResolvedValue({
    kind: 'album',
    themes: [{ id: 'all', name: 'Все', count: 1 }],
    items: [item],
    recent: [],
    readOnly: false,
  })
  render(LooksDialog)
  await looks.show()
  await tick()
  await fireEvent.click(document.querySelector('button.tile') as HTMLElement)
  await vi.waitFor(() => expect(btn('Применить…').disabled).toBe(false))
}
const toForm = async () => {
  await fireEvent.click(btn('Применить…'))
  await vi.waitFor(() => expect(win()).not.toBeNull())
}

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.info.mockImplementation(async () => info())
  api.count.mockImplementation(async (_id: string, n: number) => ({ n, places: n * 3, fit: 0 }))
  api.apply.mockImplementation(async () => ({ view: store.view, plan: plan() }))
  looks.close()
  Object.assign(looks, {
    cat: null,
    theme: '',
    sel: '',
    spreads: 0,
    pick: 'bg',
    step: 1,
    info: null,
    cnt: null,
    plan: null,
    modePlan: null,
    busy: false,
  })
  store.toast = null
  store.view = { ...store.view, rev: 1, settings: { ...store.view.settings, pageW: 300, pageH: 300, sheet: false } } as never
})
afterEach(() => {
  cleanup()
  looks.close()
  vi.restoreAllMocks()
})

describe('LookApply: состав окна', () => {
  it('заголовок с именем; форма 131 без своих кнопок просмотра; «Отмена» · «Применить»', async () => {
    await opened()
    await toForm()
    expect(win()?.querySelector('header .title')?.textContent).toBe('Применить макет «Светлая»')
    expect(spreadsInput()?.value).toBe('4')
    expect(text()).toContain('мест 12')
    expect(text()).toContain('Формат альбома: 30×30')
    const names = Array.from(win()!.querySelectorAll('.actions button')).map((b) => b.textContent?.trim())
    expect(names).toEqual(['Отмена', 'Применить'])
    expect(modalStack.count()).toBe(3) // окно макетов, просмотр, «Применить»
  })

  it('«Что сделать» — только в собранном альбоме, с пояснением режима; в несобранном его нет', async () => {
    await opened()
    await toForm()
    expect(win()?.textContent).not.toContain('Что сделать')
    cleanup()
    looks.close()
    await opened((o) => plan({ mode: o.mode, spreads: o.spreads, full: true }))
    await toForm()
    expect(win()?.textContent).toContain('Что сделать')
    expect(win()?.textContent).toContain('Стикеры макета легли бы на ваши фото')
    expect(spreadsInput()).toBeNull() // «Фоны по серии»: числа нет
  })

  it('предупреждения нет, пока плану не о чем; есть — по плану выбранного режима, в собранном «Фоны» берёт свой план', async () => {
    await opened()
    await toForm()
    expect(win()?.querySelector('.warn')).toBeNull()
    cleanup()
    looks.close()
    // серия предупреждения не даёт, а «Фоны по серии» заменят свой фон — окно показывает именно второе
    await opened((o) => plan({ mode: o.mode, spreads: o.spreads, full: true, replaced: o.mode === 'bg' ? 2 : 0 }))
    await toForm()
    expect(win()?.querySelector('.warn')?.textContent).toBe('У 2 разворотов заменится свой фон.')
    await fireEvent.click(btn('Добавить развороты макета'))
    await vi.waitFor(() => expect(win()?.querySelector('.warn')).toBeNull())
  })
})

describe('LookApply: назад', () => {
  it('«Отмена» — в просмотр, окно «Применить» закрыто, фокус на «Применить…»', async () => {
    await opened()
    await toForm()
    await fireEvent.click(btn('Отмена'))
    await tick()
    expect(win()).toBeNull()
    expect(looks.peek.id).toBe(A)
    expect(modalStack.count()).toBe(2)
    await vi.waitFor(() => expect(document.activeElement).toBe(btn('Применить…')))
    expect(api.apply).not.toHaveBeenCalled()
  })

  it('Esc и ✕ окна — то же, что «Отмена»; Esc просмотра потом закрывает уже просмотр', async () => {
    await opened()
    await toForm()
    await fireEvent.keyDown(window, { key: 'Escape' })
    await tick()
    expect(win()).toBeNull()
    expect(looks.peek.id).toBe(A)
    await vi.waitFor(() => expect(document.activeElement).toBe(btn('Применить…')))
    await toForm()
    await fireEvent.click(win()!.querySelector('header .x') as HTMLElement)
    await tick()
    expect(win()).toBeNull()
    expect(looks.peek.id).toBe(A)
    await vi.waitFor(() => expect(document.activeElement).toBe(btn('Применить…')))
    await fireEvent.keyDown(window, { key: 'Escape' })
    await tick()
    expect(looks.peek.id).toBe('')
  })

  it('Esc в просмотре, пока окна «Применить» нет, окно макетов не закрывает', async () => {
    await opened()
    await fireEvent.keyDown(window, { key: 'Escape' })
    await tick()
    expect(looks.peek.id).toBe('')
    expect(looks.open).toBe(true)
  })
})

describe('LookApply: Enter и «Применить»', () => {
  it('Enter в окне — один ApplyLook; ещё раз, пока идёт, не уходит; окна закрыты', async () => {
    let done!: () => void
    api.apply.mockImplementation(() => new Promise((r) => (done = () => r({ view: store.view, plan: plan() }))))
    await opened()
    await toForm()
    expect(document.activeElement).toBe(btn('Применить')) // фокус по умолчанию — на «Применить»
    const e = key('Enter', {}, win()!)
    expect(e.defaultPrevented).toBe(true)
    key('Enter', {}, win()!)
    await vi.waitFor(() => expect(api.apply).toHaveBeenCalledTimes(1))
    done()
    await vi.waitFor(() => expect(looks.open).toBe(false))
    expect(api.apply).toHaveBeenCalledTimes(1)
    expect(api.apply).toHaveBeenCalledWith(A, { mode: 'series', spreads: 4 })
    expect(modalStack.count()).toBe(0)
  })

  it('Enter на кнопке и автоповтор Enter не ставят: кнопка — её щелчок, повтор гасится (иначе нажал бы «Применить» с «Применить…»)', async () => {
    await opened()
    await toForm()
    expect(key('Enter', {}, btn('Отмена')).defaultPrevented).toBe(false) // «Отмена» остаётся «Отменой»
    const rep = key('Enter', { repeat: true }, btn('Применить'))
    expect(rep.defaultPrevented).toBe(true)
    const rep2 = key('Enter', { repeat: true }, win()!)
    expect(rep2.defaultPrevented).toBe(true)
    await tick()
    expect(api.apply).not.toHaveBeenCalled()
  })

  it('Enter в поле числа только принимает число, макет не ставится', async () => {
    await opened()
    await toForm()
    const input = spreadsInput() as HTMLInputElement
    input.value = '6'
    await fireEvent.keyDown(input, { key: 'Enter' })
    await vi.waitFor(() => expect(looks.spreads).toBe(6))
    await tick()
    expect(api.apply).not.toHaveBeenCalled()
    expect(win()).not.toBeNull()
  })

  it('изменившийся план: первое нажатие показывает новый текст и ждёт, второе ставит', async () => {
    await opened()
    await toForm()
    expect(win()?.querySelector('.warn')).toBeNull()
    api.plan.mockImplementation(async (_i: string, o: Opts) => plan({ mode: o.mode, spreads: o.spreads, touched: 2 })) // альбом правили
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(win()?.querySelector('.warn')?.textContent).toBe('Ваши правки на 2 разворотах заменятся.'))
    expect(api.apply).not.toHaveBeenCalled()
    expect(looks.busy).toBe(false)
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(api.apply).toHaveBeenCalledTimes(1))
    await vi.waitFor(() => expect(looks.open).toBe(false))
  })

  it('план стал чище, чем показан: текст другой — тоже второе нажатие, а не молчаливая постановка', async () => {
    await opened((o) => plan({ mode: o.mode, spreads: o.spreads, left: 3 }))
    await toForm()
    expect(win()?.querySelector('.warn')?.textContent).toBe('3 фото вернутся в ленту.')
    api.plan.mockImplementation(async (_i: string, o: Opts) => plan({ mode: o.mode, spreads: o.spreads }))
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(win()?.querySelector('.warn')).toBeNull())
    expect(api.apply).not.toHaveBeenCalled()
  })

  it('альбом правят, пока окно открыто (rev): числа и предупреждение обновляются сами, «Применить» серая до ответа', async () => {
    await opened()
    await toForm()
    api.plan.mockImplementation(async (_i: string, o: Opts) => plan({ mode: o.mode, spreads: o.spreads, replaced: 1 }))
    store.view = { ...store.view, rev: 2 } as never
    await tick()
    expect(btn('Применить').disabled).toBe(true)
    await vi.waitFor(() => expect(win()?.querySelector('.warn')?.textContent).toBe('У 1 разворота заменится свой фон.'))
    expect(btn('Применить').disabled).toBe(false)
  })

  it('ошибка Go: окно остаётся, красный тост', async () => {
    api.apply.mockRejectedValue(new Error('не получилось'))
    await opened()
    await toForm()
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(store.toast?.kind).toBe('error'))
    expect(win()).not.toBeNull()
    await vi.waitFor(() => expect(looks.busy).toBe(false))
  })
})
