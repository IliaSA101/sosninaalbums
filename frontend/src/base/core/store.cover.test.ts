import { beforeEach, describe, expect, it, vi } from 'vitest'

// Обложка в store (032): лист вне view.spreads, но первый в ряду. Ответы Go подставляет тест.
const calls: string[] = []
let reply: unknown = null

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  CreateCover: async (preset: string, photo: string) => {
    calls.push(`CreateCover:${preset}:${photo}`)
    return reply
  },
  DeleteCover: async () => {
    calls.push('DeleteCover')
    return reply
  },
  DeleteSpread: async (id: string) => {
    calls.push(`DeleteSpread:${id}`)
    return reply
  },
  NewSpread: async (at: number) => {
    calls.push(`NewSpread:${at}`)
    return reply
  },
  AddPhotos: async (id: string) => {
    calls.push(`AddPhotos:${id}`)
    return reply
  },
  Undo: async () => {
    calls.push('Undo')
    return reply
  },
  Redo: async () => {
    calls.push('Redo')
    return reply
  },
}))

vi.stubGlobal('window', globalThis)

const { store, Api } = await import('./store.svelte')

const sp = (id: string, photos: string[] = []) => ({ id, photos })
const view = (o: { spreads?: string[]; cover?: string[] | null; sheet?: boolean; rev?: number } = {}) => ({
  rev: o.rev ?? 1,
  folders: [],
  settings: { sheet: !!o.sheet },
  spreads: (o.spreads ?? ['a', 'b', 'c']).map((id) => sp(id, [`ph-${id}`])),
  cover: o.cover ? { sheet: sp('cover', o.cover), spine: 14 } : null,
})
const load = (o: Parameters<typeof view>[0] = {}) => {
  store.onCover = false
  store.apply(view(o) as never)
  store.go(0)
}

beforeEach(() => {
  calls.length = 0
  reply = null
  store.toast = null
  store.exporting = null
  store.exportOpen = false
})

describe('store: ряд листов без обложки', () => {
  it('лист = разворот: индексы совпадают', () => {
    load()
    store.go(2)
    expect(store.lead).toBe(0)
    expect(store.sheets.map((s) => s.id)).toEqual(['a', 'b', 'c'])
    expect(store.sheetIndex).toBe(2)
    expect(store.spread?.id).toBe('c')
    expect(store.onCover).toBe(false)
  })
})

describe('store: обложка первым листом', () => {
  it('обложка перед первым разворотом, индекс в ряду сдвинут', () => {
    load({ cover: [] })
    expect(store.lead).toBe(1)
    expect(store.sheets.map((s) => s.id)).toEqual(['cover', 'a', 'b', 'c'])
    expect(store.sheetIndex).toBe(1) // текущий — разворот a
    expect(store.spread?.id).toBe('a')
  })

  it('goCover: текущий лист — обложка, разворот запоминается', () => {
    load({ cover: [] })
    store.go(1)
    store.goCover()
    expect(store.onCover).toBe(true)
    expect(store.sheetIndex).toBe(0)
    expect(store.spread?.id).toBe('cover')
    expect(store.current).toBe(1)
    store.go(1) // обратно на разворот
    expect(store.onCover).toBe(false)
    expect(store.spread?.id).toBe('b')
  })

  it('goSheet: слот 0 — обложка, остальные — развороты со сдвигом', () => {
    load({ cover: [] })
    store.goSheet(0)
    expect(store.spread?.id).toBe('cover')
    store.goSheet(3)
    expect(store.spread?.id).toBe('c')
    expect(store.sheetIndex).toBe(3)
    store.goId('cover')
    expect(store.onCover).toBe(true)
    store.goId('b')
    expect(store.onCover).toBe(false)
    expect(store.current).toBe(1)
  })

  it('goCover без обложки и в проекте-листе — ничего', () => {
    load()
    store.goCover()
    expect(store.onCover).toBe(false)
    load({ cover: [], sheet: true })
    expect(store.cover).toBeNull()
    expect(store.lead).toBe(0)
    store.goCover()
    expect(store.onCover).toBe(false)
  })

  it('sheet(id) находит и обложку', () => {
    load({ cover: [] })
    expect(store.sheet('cover')?.id).toBe('cover')
    expect(store.sheet('b')?.id).toBe('b')
    expect(store.sheet('zzz')).toBeUndefined()
  })

  it('фото мест обложки считаются использованными', () => {
    load({ cover: ['ph-cover'] })
    expect(store.used.has('ph-cover')).toBe(true)
    expect(store.used.has('ph-a')).toBe(true)
  })
})

describe('store.apply с обложкой', () => {
  it('обложку удалили, пока на ней стояли, — возврат на разворот', () => {
    load({ cover: [] })
    store.go(2)
    store.goCover()
    store.apply(view({ rev: 2 }) as never)
    expect(store.onCover).toBe(false)
    expect(store.spread?.id).toBe('c')
  })

  it('появилась обложка — текущий разворот остаётся тем же', () => {
    load()
    store.go(1)
    store.apply(view({ cover: [], rev: 2 }) as never)
    expect(store.spread?.id).toBe('b')
    expect(store.sheetIndex).toBe(2)
  })

  it('только обложка, разворотов нет — стоим на ней', () => {
    load({ cover: [] })
    store.goCover()
    store.apply(view({ spreads: [], cover: [], rev: 2 }) as never)
    expect(store.onCover).toBe(true)
    expect(store.spread?.id).toBe('cover')
    store.go(0)
    expect(store.onCover).toBe(true)
  })

  it('на обложке правка разворотов текущий разворот не сбивает', () => {
    load({ cover: [] })
    store.go(1)
    store.goCover()
    store.apply(view({ cover: [], spreads: ['a', 'b', 'c', 'd'], rev: 2 }) as never)
    expect(store.onCover).toBe(true)
    expect(store.current).toBe(1)
  })
})

describe('store: создание и удаление обложки', () => {
  it('createCover(фото): пресет по умолчанию, один вызов Go, затем обложка открыта', async () => {
    load()
    reply = view({ cover: ['p1'], rev: 2 })
    expect(await store.createCover('p1')).toBe(true)
    expect(calls).toEqual(['CreateCover::p1'])
    expect(store.onCover).toBe(true)
    expect(store.spread?.id).toBe('cover')
  })

  it('обложка уже есть или проект-лист — Go не зовётся', async () => {
    load({ cover: [] })
    expect(await store.createCover('p1')).toBe(false)
    load({ sheet: true })
    expect(await store.createCover('p1')).toBe(false)
    expect(calls).toEqual([])
  })

  it('«Удалить разворот» на обложке удаляет обложку, а не разворот', async () => {
    load({ cover: [] })
    store.goCover()
    reply = view({ rev: 2 })
    await store.deleteSpread()
    expect(calls).toEqual(['DeleteCover'])
    expect(store.onCover).toBe(false)
    expect(store.toast?.action?.label).toBe('Вернуть')
  })

  it('фото на обложку добавляют броском на зону: «в текущий лист» молчит тостом', async () => {
    load({ cover: [] })
    store.goCover()
    await store.addToCurrent(['x'])
    expect(store.toast?.text).toMatch(/зону или в место/)
    expect(calls).toEqual([])
  })
})

describe('store: новый разворот при открытой обложке', () => {
  it('«+» открывает созданный разворот, а не соседний (сдвиг на обложку не нужен)', async () => {
    load({ cover: [], spreads: ['a', 'b', 'c'] })
    store.goCover()
    reply = view({ cover: [], spreads: ['a', 'b', 'c', 'd'], rev: 2 })
    await store.addEmptySpread(3)
    expect(calls).toEqual(['NewSpread:3'])
    expect(store.onCover).toBe(false)
    expect(store.spread?.id).toBe('d')
    expect(store.current).toBe(3)
    expect(store.sheetIndex).toBe(4) // в ряду обложка первая
  })
})

describe('store.runHistory: обложка и Ctrl+Z / Ctrl+Y', () => {
  it('Undo вернул удалённую обложку — она открыта', async () => {
    load({ spreads: ['a', 'b', 'c'] })
    store.go(1)
    reply = view({ cover: [], rev: 2 })
    expect(await store.runHistory(Api.Undo())).toBe(true)
    expect(store.onCover).toBe(true)
    expect(store.spread?.id).toBe('cover')
    expect(store.current).toBe(1) // разворот, откуда пришли, помнится
  })

  it('Redo создания обложки — она открыта', async () => {
    load()
    reply = view({ cover: ['p'], rev: 2 })
    await store.runHistory(Api.Redo())
    expect(store.onCover).toBe(true)
  })

  it('Undo создания обложки: стоим на том же развороте, что и до неё', async () => {
    load()
    store.go(1)
    reply = view({ cover: [], rev: 2 })
    await store.run(Promise.resolve(reply))
    store.goCover()
    reply = view({ rev: 3 })
    await store.runHistory(Api.Undo())
    expect(store.onCover).toBe(false)
    expect(store.current).toBe(1)
    expect(store.sheetIndex).toBe(1) // без обложки лист = разворот
    expect(store.spread?.id).toBe('b')
  })

  it('Undo не про обложку — переход не нужен: обложка остаётся открытой', async () => {
    load({ cover: [] })
    store.goCover()
    const seq = store.navSeq
    reply = view({ cover: [], rev: 2 })
    await store.runHistory(Api.Undo())
    expect(store.onCover).toBe(true)
    expect(store.navSeq).toBe(seq)
  })
})

describe('store.exportAll', () => {
  it('одна обложка без разворотов выгружается', () => {
    load({ spreads: [], cover: [] })
    store.exportAll()
    expect(store.exportOpen).toBe(true)
  })
  it('пустой альбом — окно не открывается', () => {
    load({ spreads: [] })
    store.exportAll()
    expect(store.exportOpen).toBe(false)
  })
})

describe('fromStrip: откуда перенесено фото', () => {
  it('с места обложки — тост «с обложки»', async () => {
    load({ cover: ['x'] })
    reply = view({ cover: [] })
    expect(await store.fromStrip(['x'], 'a', async () => reply)).toBe(true)
    expect(store.toast?.text).toBe('Фото перенесено с обложки')
  })
  it('с другого разворота — номер разворота', async () => {
    load({ cover: ['x'] })
    reply = view({ cover: ['x'] })
    await store.fromStrip(['ph-b'], 'a', async () => reply)
    expect(store.toast?.text).toBe('Фото перенесено из разворота 2')
  })
  it('в проекте-листе — «из листа N»', async () => {
    load({ sheet: true })
    reply = view({ sheet: true })
    await store.fromStrip(['ph-c'], 'a', async () => reply)
    expect(store.toast?.text).toBe('Фото перенесено из листа 3')
  })
  it('несколько источников — счётчик, свой лист не считается', async () => {
    load({ cover: ['x'] })
    reply = view({ cover: ['x'] })
    await store.fromStrip(['x', 'ph-b', 'ph-a'], 'a', async () => reply)
    expect(store.toast?.text).toBe('Перенесено фото из других разворотов: 2')
  })
})

describe('createCover: двойной щелчок', () => {
  it('пока Go отвечает, второй вызов не уходит', async () => {
    load()
    reply = view({ cover: [] })
    const [a, b] = await Promise.all([store.createCover(''), store.createCover('')])
    expect([a, b]).toEqual([true, false])
    expect(calls.filter((c) => c.startsWith('CreateCover'))).toHaveLength(1)
  })
})
