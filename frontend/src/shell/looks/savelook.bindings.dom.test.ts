// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// Окно «Сохранить альбом как макет…» на настоящем lookApi, мок — биндинги Go (131 U6): «Сохранить» → SaveLook с точными полями →
// тост → looks.changed; ошибка Go — красный тост, окно остаётся открытым, повтор уходит в Go ещё раз.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])) }
})

const { default: SaveLookDialog } = await import('./SaveLookDialog.svelte')
const { saveLookUi } = await import('./savelook.svelte')
const { looks } = await import('./looks.svelte')
const { store } = await import('../../base/core/store.svelte')
const { spreadsel } = await import('../../panels/timeline/spreadsel.svelte')
const Api = await import('../../../wailsjs/go/app/App')
const M = vi.mocked

const sheet = (id: string) => ({ id, name: '', photos: [], rects: [], w: 400, h: 200, kind: '' })
const set = (n: number, extra: object = {}) => {
  store.view = {
    ...store.view,
    spreads: Array.from({ length: n }, (_, i) => sheet('s' + (i + 1))),
    settings: {},
    cover: undefined,
    ...extra,
  } as never
}
const btn = (label: string) =>
  Array.from(document.querySelectorAll('button'))
    .filter((b) => b.textContent?.trim() === label)
    .at(-1) as HTMLButtonElement
const nameEl = () => document.querySelector('#sl-name') as HTMLInputElement

async function opened(name: string) {
  render(SaveLookDialog)
  await saveLookUi.show()
  await tick()
  nameEl().value = name
  await fireEvent.input(nameEl())
}

beforeEach(() => {
  vi.clearAllMocks()
  M(Api.DecorCatalog).mockResolvedValue({
    kind: 'album',
    themes: [{ id: 'mine', name: 'Мои' }],
    items: [],
    recent: [],
    readOnly: false,
  } as never)
  M(Api.SaveLook).mockResolvedValue({ id: 'my:0a1b2c3d4e5f6071', name: 'Свой' } as never)
  set(5)
  spreadsel.clear()
  store.toast = null
  saveLookUi.close()
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('«Сохранить» через биндинги', () => {
  it('SaveLook получает точные поля; тост «сохранён», looks.changed один раз, окно закрыто', async () => {
    const changed = vi.spyOn(looks, 'changed').mockImplementation(() => {})
    set(5, { cover: { sheet: sheet('cover') } })
    await opened(' Свой ')
    await fireEvent.click(btn('Сохранить'))
    await vi.waitFor(() => expect(saveLookUi.open).toBe(false))
    expect(Api.SaveLook).toHaveBeenCalledExactlyOnceWith({
      theme: 'mine',
      name: 'Свой',
      ids: ['s1', 's2', 's3', 's4', 's5'],
      repFrom: 2,
      repTo: 4,
      places: true,
      drafts: true,
      cover: true,
    })
    expect(store.toast?.text).toBe('Макет «Свой» сохранён в библиотеку')
    expect(store.toast?.kind).not.toBe('error')
    expect(changed).toHaveBeenCalledTimes(1)
  })

  it('«Выделенные»: ids выделенных в порядке альбома, повторяемые по ним', async () => {
    vi.spyOn(looks, 'changed').mockImplementation(() => {})
    spreadsel.toggle('s4')
    spreadsel.toggle('s2')
    spreadsel.toggle('s3')
    await opened('Три')
    await fireEvent.click(btn('Сохранить'))
    await vi.waitFor(() => expect(saveLookUi.open).toBe(false))
    expect(Api.SaveLook).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({ ids: ['s2', 's3', 's4'], repFrom: 2, repTo: 2, cover: false }),
    )
  })

  it('ошибка Go: текст Go красным тостом, окно открыто, looks.changed не зовётся; повтор уходит в Go', async () => {
    const changed = vi.spyOn(looks, 'changed').mockImplementation(() => {})
    M(Api.SaveLook).mockRejectedValueOnce(new Error('не больше 24 разных разворотов — выделите нужные в ленте'))
    await opened('Большой')
    await fireEvent.click(btn('Сохранить'))
    await vi.waitFor(() => expect(store.toast?.kind).toBe('error'))
    expect(store.toast?.text).toBe('Не больше 24 разных разворотов — выделите нужные в ленте')
    expect(saveLookUi.open).toBe(true)
    expect(changed).not.toHaveBeenCalled()
    await fireEvent.click(btn('Сохранить'))
    await vi.waitFor(() => expect(saveLookUi.open).toBe(false))
    expect(Api.SaveLook).toHaveBeenCalledTimes(2)
    expect(changed).toHaveBeenCalledTimes(1)
  })

  it('пустое имя: «Сохранить» серая, в Go не идёт', async () => {
    await opened('  ')
    expect(btn('Сохранить').disabled).toBe(true)
    await fireEvent.keyDown(nameEl(), { key: 'Enter' })
    expect(Api.SaveLook).not.toHaveBeenCalled()
  })

  it('PlanSaveLook при открытии: те же поля, что у SaveLook; отказ Go серит кнопку и показывает причину', async () => {
    const why = 'Не больше 24 разных разворотов — выделите нужные в ленте'
    M(Api.PlanSaveLook).mockResolvedValue({ ok: false, steps: 0, max: 24, reason: why } as never)
    await opened('Большой')
    await vi.waitFor(() => expect(btn('Сохранить').disabled).toBe(true))
    expect(document.querySelector('.warn')?.textContent).toBe(why)
    expect(Api.PlanSaveLook).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({ ids: ['s1', 's2', 's3', 's4', 's5'], repFrom: 2, repTo: 4, places: true, drafts: true, cover: false }),
    )
    expect(Api.SaveLook).not.toHaveBeenCalled()
  })
})
