// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// Окно «Сохранить альбом как макет…» (131 U3): поля, «Выделенные», повторяемые, флажки, успех и ошибки Go, переходы входов.
const api = vi.hoisted(() => ({ catalog: vi.fn(), save: vi.fn(), planSave: vi.fn() }))
vi.mock('./lookapi', () => ({ lookApi: api }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { default: SaveLookDialog } = await import('./SaveLookDialog.svelte')
const { saveLookUi } = await import('./savelook.svelte')
const { looks } = await import('./looks.svelte')
const { store } = await import('../../base/core/store.svelte')
const { spreadsel } = await import('../../panels/timeline/spreadsel.svelte')

const sheet = (id: string) => ({ id, name: '', photos: [], rects: [], w: 400, h: 200, kind: '' })
const set = (o: object, n = 5) => {
  store.view = {
    ...store.view,
    spreads: Array.from({ length: n }, (_, i) => sheet('s' + (i + 1))),
    settings: {},
    cover: undefined,
    ...o,
  } as never
}
const cat = {
  themes: [
    { id: 'recent', name: 'Недавние' },
    { id: 'all', name: 'Все' },
    { id: 'wedding', name: 'Свадьба' },
    { id: 'mine', name: 'Мои' },
  ],
}

beforeEach(() => {
  api.catalog.mockResolvedValue(cat)
  api.save.mockResolvedValue({ id: 'my:1' })
  api.planSave.mockResolvedValue({ ok: true, steps: 3, max: 24, reason: '' })
  set({})
  spreadsel.clear()
  store.toast = null
  saveLookUi.close()
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.clearAllMocks()
})

const q = <T extends Element = HTMLElement>(s: string) => document.querySelector(s) as T
const nameEl = () => q<HTMLInputElement>('#sl-name')
const save = () => Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Сохранить') as HTMLButtonElement
const seg = (label: string) =>
  Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.trim() === label) as HTMLButtonElement
const nums = () => Array.from(document.querySelectorAll<HTMLInputElement>('input[type=number]'))
async function open() {
  render(SaveLookDialog)
  await saveLookUi.show()
  await tick()
}

describe('открытие', () => {
  it('поля, подписи и умолчания: все флажки включены, «Все», повторяемые с 2 по N−1', async () => {
    await open()
    expect(nameEl().value).toBe('')
    expect(Array.from(document.querySelectorAll('#sl-theme option')).map((o) => o.textContent)).toEqual(['Свадьба', 'Мои'])
    expect(q<HTMLSelectElement>('#sl-theme').value).toBe('mine')
    expect(nums().map((n) => n.value)).toEqual(['2', '4'])
    const boxes = Array.from(document.querySelectorAll<HTMLInputElement>('input[type=checkbox]'))
    expect(boxes.map((b) => b.checked)).toEqual([true, true, false]) // обложки в альбоме нет: серый и выключен
    expect(boxes[2].disabled).toBe(true)
    expect(document.body.textContent).toContain('Места для фото')
    expect(document.body.textContent).toContain('Надписи — заготовками')
    expect(document.body.textContent).toContain('из 5 разворотов')
    expect(seg('Все').getAttribute('aria-pressed')).toBe('true')
  })
  it('с обложкой флажок «Обложка» включён', async () => {
    set({ cover: { sheet: sheet('cover') } })
    await open()
    const box = Array.from(document.querySelectorAll<HTMLInputElement>('input[type=checkbox]'))[2]
    expect(box.checked).toBe(true)
    expect(box.disabled).toBe(false)
  })
  it('«Сохранить» серая, пока имя пусто; Enter в пустом поле молчит', async () => {
    await open()
    expect(save().disabled).toBe(true)
    await fireEvent.keyDown(nameEl(), { key: 'Enter' })
    expect(api.save).not.toHaveBeenCalled()
  })
  it('проект-лист — окно не открывается; только обложка — тост и тоже нет', async () => {
    set({ settings: { sheet: { w: 200, h: 300 } } })
    await saveLookUi.show()
    expect(saveLookUi.open).toBe(false)
    set({}, 0)
    await saveLookUi.show()
    expect(saveLookUi.open).toBe(false)
    expect(store.toast?.text).toBe('В альбоме нет разворотов — макет из него не сохранить')
  })
})

describe('«Выделенные» и повторяемые', () => {
  it('нет выделения — «Выделенные» серые и не нажимаются', async () => {
    await open()
    await fireEvent.click(seg('Выделенные'))
    expect(seg('Все').getAttribute('aria-pressed')).toBe('true')
  })
  it('в ленте выделено 3 разворота — окно открывается на «Выделенные», id в порядке альбома, повторяемые с 2 по 2', async () => {
    spreadsel.toggle('s4')
    spreadsel.toggle('s2')
    spreadsel.toggle('s3')
    await open()
    expect(seg('Выделенные').getAttribute('aria-pressed')).toBe('true')
    expect(nums().map((n) => n.value)).toEqual(['2', '2'])
    nameEl().value = 'Три'
    await fireEvent.input(nameEl())
    await fireEvent.click(save())
    await tick()
    expect(api.save.mock.calls[0][0]).toMatchObject({ ids: ['s2', 's3', 's4'], repFrom: 2, repTo: 2 })
  })
  it('смена «Все» ↔ «Выделенные» пересчитывает повторяемые; повторный щелчок по выбранному ничего не меняет', async () => {
    spreadsel.toggle('s1')
    spreadsel.toggle('s2')
    await open()
    await fireEvent.click(seg('Все'))
    expect(nums().map((n) => n.value)).toEqual(['2', '4'])
    await fireEvent.click(seg('Все'))
    expect(nums().map((n) => n.value)).toEqual(['2', '4'])
    await fireEvent.click(seg('Выделенные'))
    expect(nums().map((n) => n.value)).toEqual(['2', '1']) // два разворота — повторяемых нет
    expect(document.body.textContent).toContain('Повторяемых нет')
  })
  it('число вне пределов зажимается: «с» в 1…N, «по» в 0…N', async () => {
    await open()
    nums()[0].value = '99'
    await fireEvent.change(nums()[0])
    nums()[1].value = '99'
    await fireEvent.change(nums()[1])
    expect(nums().map((n) => n.value)).toEqual(['5', '5'])
    nums()[0].value = ''
    await fireEvent.change(nums()[0])
    expect(nums()[0].value).toBe('1')
  })
})

describe('Enter в числовых полях (доправка ревью)', () => {
  it('набрал 3 в «по» → Enter → сохраняется repTo: 3, а не старое 4', async () => {
    await open()
    nameEl().value = 'Мак'
    await fireEvent.input(nameEl())
    nums()[1].value = '3'
    await fireEvent.keyDown(nums()[1], { key: 'Enter' })
    await tick()
    expect(api.save.mock.calls[0][0]).toMatchObject({ repFrom: 2, repTo: 3 })
  })
  it('набрал 3 в «с» → Enter → repFrom: 3', async () => {
    await open()
    nameEl().value = 'Мак'
    await fireEvent.input(nameEl())
    nums()[0].value = '3'
    await fireEvent.keyDown(nums()[0], { key: 'Enter' })
    await tick()
    expect(api.save.mock.calls[0][0]).toMatchObject({ repFrom: 3, repTo: 4 })
  })
  it('число вне пределов в Enter зажимается, как при уходе из поля', async () => {
    await open()
    nameEl().value = 'Мак'
    await fireEvent.input(nameEl())
    nums()[1].value = '99'
    await fireEvent.keyDown(nums()[1], { key: 'Enter' })
    await tick()
    expect(api.save.mock.calls[0][0]).toMatchObject({ repTo: 5 })
  })
  it('повторный щелчок по выбранному «Все» не сбрасывает «с/по»', async () => {
    await open()
    nums()[0].value = '3'
    await fireEvent.change(nums()[0])
    nums()[1].value = '4'
    await fireEvent.change(nums()[1])
    await fireEvent.click(seg('Все'))
    expect(nums().map((n) => n.value)).toEqual(['3', '4'])
  })
})

describe('сохранение', () => {
  async function fill(name: string) {
    nameEl().value = name
    await fireEvent.input(nameEl())
  }
  it('имя и флажки уходят в SaveLook; тост; окно «Макеты альбома» перечитывается; окно закрывается', async () => {
    const changed = vi.spyOn(looks, 'changed').mockImplementation(() => {})
    await open()
    await fill(' Мой макет ')
    await fireEvent.click(
      Array.from(document.querySelectorAll('label'))
        .find((l) => l.textContent?.includes('Места для фото'))!
        .querySelector('input')!,
    )
    await fireEvent.click(save())
    await tick()
    expect(api.save).toHaveBeenCalledExactlyOnceWith({
      theme: 'mine',
      name: 'Мой макет',
      ids: ['s1', 's2', 's3', 's4', 's5'],
      repFrom: 2,
      repTo: 4,
      places: false,
      drafts: true,
      cover: false,
    })
    expect(store.toast?.text).toBe('Макет «Мой макет» сохранён в библиотеку')
    expect(changed).toHaveBeenCalledTimes(1)
    expect(saveLookUi.open).toBe(false)
  })
  it('Enter в поле имени сохраняет', async () => {
    await open()
    await fill('Свой')
    await fireEvent.keyDown(nameEl(), { key: 'Enter' })
    await tick()
    expect(api.save).toHaveBeenCalledTimes(1)
  })
  it('больше 24 разных (ошибка Go) — тост ошибки, окно остаётся, второй раз можно', async () => {
    api.save.mockRejectedValueOnce(new Error('Не больше 24 разных разворотов — выделите нужные в ленте'))
    await open()
    await fill('Большой')
    await fireEvent.click(save())
    await tick()
    expect(store.toast).toMatchObject({ kind: 'error' })
    expect(store.toast?.text).toContain('Не больше 24 разных разворотов')
    expect(saveLookUi.open).toBe(true)
    expect(save().disabled).toBe(false)
    await fireEvent.click(save())
    await tick()
    expect(api.save).toHaveBeenCalledTimes(2)
    expect(saveLookUi.open).toBe(false)
  })
  it('Отмена закрывает без вызова Go; каталог недоступен — остаётся «Мои»', async () => {
    api.catalog.mockRejectedValueOnce(new Error('нет'))
    await open()
    expect(Array.from(document.querySelectorAll('#sl-theme option')).map((o) => o.textContent)).toEqual(['Мои'])
    await fireEvent.click(Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Отмена')!)
    expect(saveLookUi.open).toBe(false)
    expect(api.save).not.toHaveBeenCalled()
  })
  it('один разворот в альбоме — сохраняется, повторяемых нет', async () => {
    set({}, 1)
    await open()
    await fill('Один')
    await fireEvent.click(save())
    await tick()
    expect(api.save.mock.calls[0][0]).toMatchObject({ ids: ['s1'], repFrom: 2, repTo: 0 })
  })
})
