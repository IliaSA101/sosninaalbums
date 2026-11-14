// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Go подменён: считаем вызовы связи; остальные методы окну не нужны.
const setAssoc = vi.hoisted(() => vi.fn(async (on: boolean) => ({ can: true, on, ask: false })))
const assocLater = vi.hoisted(() => vi.fn(async () => ({ can: true, on: false, ask: false })))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  SetAssoc: setAssoc,
  AssocLater: assocLater,
  Recent: async () => [],
}))

const { default: StartDialog } = await import('./StartDialog.svelte')
const { default: StartAssoc } = await import('./StartAssoc.svelte')
const { assocUi, ASSOC_ASK, ASSOC_ON, ASSOC_NEVER } = await import('./assoc.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')

const ask = () => (store.assoc = { can: true, on: false, ask: true })

beforeEach(() => {
  store.loaded = true
  store.toast = null
  store.assoc = { can: true, on: false, ask: false }
  prefs.startOpen = false
  assocUi.reset()
  setAssoc.mockClear()
  assocLater.mockClear()
})
afterEach(() => {
  cleanup()
  prefs.startOpen = false
})

const flush = async () => {
  for (let i = 0; i < 6; i++) await Promise.resolve()
  flushSync()
}

describe('StartAssoc: строка вопроса', () => {
  it('ask выключен — строки нет', () => {
    render(StartAssoc)
    expect(screen.queryByText(ASSOC_ASK)).toBeNull()
  })

  it('ask включён — вопрос и три кнопки в порядке Да · Не сейчас · Не спрашивать', () => {
    ask()
    render(StartAssoc)
    expect(screen.getByText('Открывать альбомы .salbum двойным щелчком?')).toBeTruthy()
    const names = screen.getAllByRole('button').map((b) => b.textContent)
    expect(names).toEqual(['Да', 'Не сейчас', 'Не спрашивать'])
  })

  it('«Да»: SetAssoc(true), строка исчезает, тост-подтверждение', async () => {
    ask()
    render(StartAssoc)
    await fireEvent.click(screen.getByText('Да'))
    await flush()
    expect(setAssoc).toHaveBeenCalledExactlyOnceWith(true)
    expect(store.assoc.ask).toBe(false)
    expect(screen.queryByText(ASSOC_ASK)).toBeNull()
    expect(store.toast?.text).toBe(ASSOC_ON)
  })

  it('«Не сейчас»: AssocLater, SetAssoc не зовётся, тоста нет', async () => {
    ask()
    render(StartAssoc)
    await fireEvent.click(screen.getByText('Не сейчас'))
    await flush()
    expect(assocLater).toHaveBeenCalledOnce()
    expect(setAssoc).not.toHaveBeenCalled()
    expect(screen.queryByText(ASSOC_ASK)).toBeNull()
    expect(store.toast).toBeNull()
  })

  it('«Не спрашивать»: SetAssoc(false) и подсказка, где включить', async () => {
    ask()
    render(StartAssoc)
    await fireEvent.click(screen.getByText('Не спрашивать'))
    await flush()
    expect(setAssoc).toHaveBeenCalledExactlyOnceWith(false)
    expect(store.toast?.text).toBe(ASSOC_NEVER)
  })

  it('ошибка Go: короткий тост, вопрос остаётся, кнопки снова доступны', async () => {
    ask()
    setAssoc.mockRejectedValueOnce(new Error('не удалось записать связь'))
    render(StartAssoc)
    await fireEvent.click(screen.getByText('Да'))
    await flush()
    expect(store.toast).toMatchObject({ kind: 'error', text: 'Не удалось записать связь' })
    expect(store.assoc.ask).toBe(true)
    expect((screen.getByText('Да') as HTMLButtonElement).disabled).toBe(false)
  })

  it('быстрый двойной щелчок — один вызов', async () => {
    ask()
    render(StartAssoc)
    const b = screen.getByText('Да')
    void fireEvent.click(b)
    void fireEvent.click(b)
    await flush()
    expect(setAssoc).toHaveBeenCalledOnce()
  })
})

describe('StartDialog: вопрос в окне или тостом', () => {
  it('окно открыто — вопрос строкой, тоста нет', () => {
    ask()
    prefs.startOpen = true
    render(StartDialog)
    flushSync()
    expect(screen.getByText(ASSOC_ASK)).toBeTruthy()
    expect(store.toast).toBeNull()
  })

  it('окно скрыто — вопрос тостом с кнопкой «Да», она включает связь', async () => {
    ask()
    render(StartDialog)
    flushSync()
    expect(store.toast?.text).toBe(ASSOC_ASK)
    expect(store.toast?.action?.label).toBe('Да')
    store.toast?.action?.run()
    await flush()
    expect(setAssoc).toHaveBeenCalledExactlyOnceWith(true)
    expect(store.toast?.text).toBe(ASSOC_ON)
  })

  it('тост один на сеанс: закрыли — второй раз не появляется', () => {
    ask()
    render(StartDialog)
    flushSync()
    store.toast = null
    assocUi.see(false)
    expect(store.toast).toBeNull()
  })

  it('окно было открыто и закрыто без ответа — тоста после него нет', () => {
    ask()
    prefs.startOpen = true
    render(StartDialog)
    flushSync()
    prefs.startOpen = false
    flushSync()
    expect(store.toast).toBeNull()
  })

  it('ask выключен (связь есть, «Не сейчас», нельзя) — ни строки, ни тоста', () => {
    render(StartDialog)
    flushSync()
    expect(store.toast).toBeNull()
    prefs.startOpen = true
    flushSync()
    expect(screen.queryByText(ASSOC_ASK)).toBeNull()
  })

  it('на старте уже есть тост (ошибка открытия) — его не затираем', () => {
    ask()
    store.notify('Не удалось открыть альбом', 'error')
    render(StartDialog)
    flushSync()
    expect(store.toast?.text).toBe('Не удалось открыть альбом')
  })

  it('альбом ещё не загружен — молчим', () => {
    ask()
    store.loaded = false
    render(StartDialog)
    flushSync()
    expect(store.toast).toBeNull()
  })
})
