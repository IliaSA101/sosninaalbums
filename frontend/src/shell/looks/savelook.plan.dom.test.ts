// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// «Сохранить» серо заранее (131-212): PlanSaveLook при открытии и при смене выбора, «Повторяемых», флажков; причина Go под полями;
// устаревший ответ не перетирает новый. Альбом окно не меняет, отката нет (проверка: закрытие и повторное открытие чисты).
const api = vi.hoisted(() => ({ catalog: vi.fn(), save: vi.fn(), planSave: vi.fn() }))
vi.mock('./lookapi', () => ({ lookApi: api }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { default: SaveLookDialog } = await import('./SaveLookDialog.svelte')
const { saveLookUi } = await import('./savelook.svelte')
const { store } = await import('../../base/core/store.svelte')
const { spreadsel } = await import('../../panels/timeline/spreadsel.svelte')

const MANY = 'Не больше 24 разных разворотов — выделите нужные в ленте'
const ok = { ok: true, steps: 3, max: 24, reason: '' }
const no = (reason: string) => ({ ok: false, steps: 0, max: 24, reason })
const sheet = (id: string) => ({ id, name: '', photos: [], rects: [], w: 400, h: 200, kind: '' })
const set = (n = 5) => {
  store.view = {
    ...store.view,
    spreads: Array.from({ length: n }, (_, i) => sheet('s' + (i + 1))),
    settings: {},
    cover: undefined,
  } as never
}
const defer = <T>() => {
  let res!: (v: T) => void
  const p = new Promise<T>((r) => (res = r))
  return { p, res }
}

beforeEach(() => {
  api.catalog.mockResolvedValue({ themes: [{ id: 'mine', name: 'Мои' }] })
  api.save.mockResolvedValue({ id: 'my:1' })
  api.planSave.mockResolvedValue(ok)
  set()
  spreadsel.clear()
  store.toast = null
  saveLookUi.close()
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.clearAllMocks()
})

const nameEl = () => document.querySelector('#sl-name') as HTMLInputElement
const btn = (label: string) =>
  Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.trim() === label) as HTMLButtonElement
const nums = () => Array.from(document.querySelectorAll<HTMLInputElement>('input[type=number]'))
const warn = () => document.querySelector('.warn')?.textContent ?? null
async function open(name = 'Мой') {
  render(SaveLookDialog)
  await saveLookUi.show()
  nameEl().value = name
  await fireEvent.input(nameEl())
  await vi.waitFor(() => expect(api.planSave).toHaveBeenCalled())
  await tick()
}

describe('PlanSaveLook при открытии', () => {
  it('ok=true: кнопка активна, причины нет; запрос с выбором окна', async () => {
    await open()
    expect(api.planSave).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({ ids: ['s1', 's2', 's3', 's4', 's5'], repFrom: 2, repTo: 4, places: true, drafts: true, cover: false }),
    )
    expect(btn('Сохранить').disabled).toBe(false)
    expect(warn()).toBeNull()
  })
  it('ok=false: «Сохранить» серая, причина Go слово в слово видна; Enter не сохраняет', async () => {
    api.planSave.mockResolvedValue(no(MANY))
    await open()
    expect(btn('Сохранить').disabled).toBe(true)
    expect(warn()).toBe(MANY)
    await fireEvent.keyDown(nameEl(), { key: 'Enter' })
    expect(api.save).not.toHaveBeenCalled()
  })
  it('пока ответа нет, кнопка не серится (решает запись); сбой вызова тоже не серит', async () => {
    const d = defer<unknown>()
    api.planSave.mockReturnValue(d.p)
    await open()
    expect(btn('Сохранить').disabled).toBe(false)
    d.res(no(MANY))
    await vi.waitFor(() => expect(btn('Сохранить').disabled).toBe(true))
    api.planSave.mockRejectedValue(new Error('сбой'))
    await fireEvent.click(btn('Все'))
    nums()[0].value = '3'
    await fireEvent.change(nums()[0])
    await vi.waitFor(() => expect(btn('Сохранить').disabled).toBe(false))
    expect(warn()).toBeNull()
  })
  it('имя пустое: кнопка серая своим правилом, причины Go нет', async () => {
    await open('')
    expect(btn('Сохранить').disabled).toBe(true)
    expect(warn()).toBeNull()
  })
})

describe('перезапрос при смене выбора', () => {
  it('«Повторяемые»: смена «с» и «по» — по вызову; то же число или мусор — без вызова', async () => {
    await open()
    api.planSave.mockClear()
    nums()[0].value = '3'
    await fireEvent.change(nums()[0])
    expect(api.planSave).toHaveBeenCalledTimes(1)
    expect(api.planSave.mock.calls[0][0]).toMatchObject({ repFrom: 3, repTo: 4 })
    nums()[1].value = '5'
    await fireEvent.change(nums()[1])
    expect(api.planSave).toHaveBeenCalledTimes(2)
    expect(api.planSave.mock.calls[1][0]).toMatchObject({ repFrom: 3, repTo: 5 })
    nums()[1].value = '5'
    await fireEvent.change(nums()[1])
    nums()[1].value = '99' // зажим до 5 — то же число
    await fireEvent.change(nums()[1])
    expect(api.planSave).toHaveBeenCalledTimes(2)
  })
  it('ошибка уходит при смене «по»: 25 шагов → серо, вернули → снова активна', async () => {
    await open()
    api.planSave.mockResolvedValue(no(MANY))
    nums()[1].value = '5'
    await fireEvent.change(nums()[1])
    await vi.waitFor(() => expect(btn('Сохранить').disabled).toBe(true))
    expect(warn()).toBe(MANY)
    api.planSave.mockResolvedValue(ok)
    nums()[1].value = '4'
    await fireEvent.change(nums()[1])
    await vi.waitFor(() => expect(btn('Сохранить').disabled).toBe(false))
    expect(warn()).toBeNull()
  })
  it('«Выделенные» и «Все»: ids в запросе; повторный щелчок по выбранному — без вызова', async () => {
    spreadsel.toggle('s2')
    spreadsel.toggle('s4')
    await open()
    expect(api.planSave.mock.calls[0][0]).toMatchObject({ ids: ['s2', 's4'] })
    api.planSave.mockClear()
    await fireEvent.click(btn('Выделенные'))
    expect(api.planSave).not.toHaveBeenCalled()
    await fireEvent.click(btn('Все'))
    expect(api.planSave).toHaveBeenCalledTimes(1)
    expect(api.planSave.mock.calls[0][0]).toMatchObject({ ids: ['s1', 's2', 's3', 's4', 's5'], repFrom: 2, repTo: 4 })
  })
  it('флажки «Места для фото», «Надписи — заготовками»: перезапрос с новым значением', async () => {
    await open()
    api.planSave.mockClear()
    const box = (t: string) =>
      Array.from(document.querySelectorAll('label'))
        .find((l) => l.textContent?.includes(t))!
        .querySelector('input')!
    await fireEvent.click(box('Места для фото'))
    expect(api.planSave.mock.calls[0][0]).toMatchObject({ places: false, drafts: true })
    await fireEvent.click(box('Надписи'))
    expect(api.planSave.mock.calls[1][0]).toMatchObject({ places: false, drafts: false })
  })
  it('флажок «Обложка»: с обложкой в альбоме перезапрашивает', async () => {
    store.view = { ...store.view, cover: { sheet: sheet('cover') } } as never
    await open()
    api.planSave.mockClear()
    await fireEvent.click(document.querySelectorAll<HTMLInputElement>('input[type=checkbox]')[2])
    expect(api.planSave.mock.calls[0][0]).toMatchObject({ cover: false })
  })
})

describe('устаревшие ответы', () => {
  it('первый запрос отвечает позже второго — побеждает последний', async () => {
    await open()
    const a = defer<unknown>()
    const b = defer<unknown>()
    api.planSave.mockReturnValueOnce(a.p).mockReturnValueOnce(b.p)
    nums()[0].value = '3'
    await fireEvent.change(nums()[0])
    nums()[1].value = '5'
    await fireEvent.change(nums()[1])
    b.res(ok)
    await tick()
    a.res(no(MANY)) // опоздавший отказ не должен посерить кнопку
    await tick()
    await tick()
    expect(btn('Сохранить').disabled).toBe(false)
    expect(warn()).toBeNull()
  })
  it('и наоборот: старый ok не стирает свежий отказ', async () => {
    await open()
    const a = defer<unknown>()
    const b = defer<unknown>()
    api.planSave.mockReturnValueOnce(a.p).mockReturnValueOnce(b.p)
    nums()[0].value = '3'
    await fireEvent.change(nums()[0])
    nums()[1].value = '5'
    await fireEvent.change(nums()[1])
    b.res(no(MANY))
    await vi.waitFor(() => expect(btn('Сохранить').disabled).toBe(true))
    a.res(ok)
    await tick()
    await tick()
    expect(btn('Сохранить').disabled).toBe(true)
    expect(warn()).toBe(MANY)
  })
  it('ответ после закрытия окна не оживает при следующем открытии', async () => {
    const d = defer<unknown>()
    api.planSave.mockReturnValueOnce(d.p)
    await open()
    saveLookUi.close()
    await tick()
    api.planSave.mockResolvedValue(ok)
    await saveLookUi.show()
    await tick()
    d.res(no(MANY))
    await tick()
    await tick()
    expect(saveLookUi.refusal).toBe('')
    expect(btn('Сохранить')).toBeDefined()
  })
})

describe('открытие заново и крайние', () => {
  it('закрыл и открыл: прежний отказ не висит до ответа (сброс), новый запрос уходит', async () => {
    api.planSave.mockResolvedValue(no(MANY))
    await open()
    expect(warn()).toBe(MANY)
    saveLookUi.close()
    await tick()
    const d = defer<unknown>()
    api.planSave.mockReturnValue(d.p)
    await saveLookUi.show()
    await tick()
    expect(saveLookUi.refusal).toBe('')
    expect(api.planSave).toHaveBeenCalledTimes(2)
  })
  it('окно не открывается (лист-проект, нет разворотов) — PlanSaveLook не зовётся', async () => {
    store.view = { ...store.view, settings: { sheet: { w: 200, h: 300 } } } as never
    await saveLookUi.show()
    set(0)
    await saveLookUi.show()
    expect(api.planSave).not.toHaveBeenCalled()
  })
  it('один разворот: запрос уходит с ids из одного; отказ «нечего сохранять» виден', async () => {
    set(1)
    api.planSave.mockResolvedValue(no('Нечего сохранять'))
    await open()
    expect(api.planSave.mock.calls[0][0]).toMatchObject({ ids: ['s1'] })
    expect(warn()).toBe('Нечего сохранять')
  })
  it('запись не зовётся сама: открытие и перезапросы только планируют', async () => {
    await open()
    nums()[0].value = '3'
    await fireEvent.change(nums()[0])
    expect(api.save).not.toHaveBeenCalled()
  })
  it('отказ → набрал «по», при котором Go ответит ok → Enter → SaveLook вызван (ждёт свежий ответ)', async () => {
    api.planSave.mockResolvedValue(no(MANY))
    await open()
    expect(btn('Сохранить').disabled).toBe(true)
    api.planSave.mockResolvedValue(ok)
    nums()[1].value = '3'
    await fireEvent.keyDown(nums()[1], { key: 'Enter' })
    await vi.waitFor(() => expect(api.save).toHaveBeenCalledTimes(1))
    expect(api.save.mock.calls[0][0]).toMatchObject({ repFrom: 2, repTo: 3 })
  })
  it('отказ остаётся и после правки (Go снова ok=false): Enter не сохраняет', async () => {
    api.planSave.mockResolvedValue(no(MANY))
    await open()
    nums()[1].value = '3'
    await fireEvent.keyDown(nums()[1], { key: 'Enter' })
    await vi.waitFor(() => expect(api.planSave).toHaveBeenCalledTimes(2))
    await tick()
    expect(api.save).not.toHaveBeenCalled()
    expect(warn()).toBe(MANY)
  })
  it('при ok=false клик по серой «Сохранить» ничего не сохраняет, при последующем ok — сохраняет', async () => {
    api.planSave.mockResolvedValue(no(MANY))
    await open()
    await fireEvent.click(btn('Сохранить'))
    expect(api.save).not.toHaveBeenCalled()
    api.planSave.mockResolvedValue(ok)
    nums()[0].value = '3'
    await fireEvent.change(nums()[0])
    await vi.waitFor(() => expect(btn('Сохранить').disabled).toBe(false))
    await fireEvent.click(btn('Сохранить'))
    await vi.waitFor(() => expect(api.save).toHaveBeenCalledTimes(1))
  })
})
