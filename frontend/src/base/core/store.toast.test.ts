import { afterEach, describe, expect, it, vi } from 'vitest'

// Go подменён: каждая правка отвечает альбомом нужной ревизии.
const ids = ['a', 'b', 'c']
const view = (rev: number, gone: string[] = []) => ({
  rev,
  folders: [],
  spreads: ids.filter((i) => !gone.includes(i)).map((id) => ({ id, photos: [] as string[] })),
})

// Ревизия ответа на удаление и необязательная задержка ответа.
let delRev = 2
let gate: Promise<void> | null = null
let undos = 0

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  DeleteSpread: async () => {
    if (gate) await gate
    return view(delRev, ['a'])
  },
  SplitSpread: async () => view(2),
  // Go отвечает отказом строкой (Wails): на развороте нет места.
  AddPhotos: async () => {
    throw 'на развороте нет свободных мест'
  },
  Undo: async () => {
    undos++
    return view(3)
  },
}))
vi.stubGlobal('window', globalThis)

const { store } = await import('./store.svelte')

afterEach(() => {
  delRev = 2
  gate = null
})

describe('тост с «Вернуть» и ревизия альбома', () => {
  it('гаснет, когда ревизия сменилась (Ctrl+Z, «Отменить», любая другая правка)', async () => {
    store.apply(view(1) as never)
    store.goId('a')
    await store.deleteSpread()
    expect(store.toast?.action?.label).toBe('Вернуть')
    expect(store.toast?.rev).toBe(2)
    store.apply(view(3) as never) // Ctrl+Z вернул разворот
    expect(store.toast).toBeNull()
  })

  it('событие view с той же ревизией, пришедшее после ответа, тост не гасит', async () => {
    store.apply(view(1) as never)
    store.goId('a')
    await store.deleteSpread()
    store.apply(view(2, ['a']) as never)
    expect(store.toast?.text).toBe('Разворот удалён')
  })

  it('разрез: тост тоже привязан к ревизии', async () => {
    store.apply(view(1) as never)
    await store.split('a', 1)
    expect(store.toast?.text).toBe('Разворот разрезан')
    store.apply(view(4) as never)
    expect(store.toast).toBeNull()
  })

  it('обычный тост без «Вернуть» смена ревизии не трогает', () => {
    store.apply(view(1) as never)
    store.notify('Готово')
    store.apply(view(5) as never)
    expect(store.toast?.text).toBe('Готово')
  })

  it('«Вернуть» откатывает, и тост гаснет', async () => {
    store.apply(view(1) as never)
    store.goId('a')
    await store.deleteSpread()
    await store.toast!.action!.run()
    expect(store.toast).toBeNull()
    expect(store.view.rev).toBe(3)
  })

  it('холостое удаление (rev не изменился) — тоста нет', async () => {
    store.toast = null
    store.apply(view(1) as never)
    store.goId('a')
    delRev = 1
    await store.deleteSpread()
    expect(store.toast).toBeNull()
  })

  it('«Вернуть» на тосте №1 до ответа на удаление №2 не отменяет №2', async () => {
    store.apply(view(1) as never)
    store.goId('a')
    await store.deleteSpread()
    const first = store.toast!.action!.run
    undos = 0
    let release!: () => void
    gate = new Promise<void>((r) => (release = r))
    delRev = 3
    const second = store.deleteSpread('b')
    const back = first() // щелчок по «Вернуть» первого тоста, пока Go думает над вторым
    release()
    await Promise.all([second, back])
    // Ревизия сменилась вторым удалением: Undo не вызван.
    expect(undos).toBe(0)
    expect(store.view.rev).toBe(3)
  })
})

describe('ошибка добавления фото из ленты', () => {
  it('«Заполнить»/Enter без свободных мест: красный тост с текстом Go', async () => {
    store.toast = null
    store.apply(view(1) as never)
    store.goId('a')
    await store.addToCurrent(['x', 'y'])
    expect(store.toast).toMatchObject({ kind: 'error', text: 'На развороте нет свободных мест' })
  })
})
