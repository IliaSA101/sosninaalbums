import { beforeEach, describe, expect, it, vi } from 'vitest'

// Go подменён: binapi записывает вызовы и отвечает видом, который задал тест.
const calls: unknown[][] = []
let answer: () => Promise<unknown> = async () => ({})
const BINS = [
  { id: 'b1', name: 'Обязательно' },
  { id: 'b2', name: 'Запас' },
]
const view = (rev: number, bins = BINS) => ({ rev, marksRev: rev, folders: [], spreads: [], bins })

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ Marks: async () => ({}), GetView: async () => ({}) }))
vi.mock('./binapi', () => ({
  NewBin: (...a: unknown[]) => (calls.push(['new', ...a]), answer()),
  RenameBin: (...a: unknown[]) => (calls.push(['rename', ...a]), answer()),
  DeleteBin: (...a: unknown[]) => (calls.push(['delete', ...a]), answer()),
  SetBin: (...a: unknown[]) => (calls.push(['set', ...a]), answer()),
}))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { ownMarks } = await import('../marks/ownmarks.svelte')
const { binsel } = await import('./binsel.svelte')
const { create, drop, openNew, remove, removeSelected, rename, setFor, toggleSelected } = await import('./binact')

beforeEach(() => {
  calls.length = 0
  prefs.level = 'standard'
  binsel.reset()
  ownMarks.map = { a: { bins: ['b1'] }, b: { bins: ['b1'] }, c: { bins: ['b2'] } }
  store.toast = null
  store.apply(view(1) as never)
  // Лента из четырёх фото; a, b — в «Обязательно», c — в «Запас»
  store.photos = ['a', 'b', 'c', 'd'].map((id) => ({ id, path: id, name: id }) as never)
  store.selected.clear()
})

const select = (...ids: string[]) => {
  store.selected.clear()
  for (const id of ids) store.selected.add(id)
}

describe('create', () => {
  it('новая с выделенными: один вызов, становится целевой, лента не переключается', async () => {
    answer = async () => view(2, [...BINS, { id: 'b3', name: 'Для родителей' }])
    const r = await create('Для родителей', ['a', 'c'])
    expect(r.ok).toBe(true)
    expect(calls).toEqual([['new', 'Для родителей', ['a', 'c']]])
    expect(binsel.target).toBe('b3')
    expect(binsel.cur).toBe('')
  })

  it('имя занято (без учёта регистра) — ошибка для поля, Go не зовётся', async () => {
    const r = await create('запас', [])
    expect(r).toEqual({ ok: false, err: 'Такая подборка уже есть' })
    expect(calls).toEqual([])
    expect(binsel.target).toBe('')
  })

  it('пустое имя уходит в Go как есть («Подборка N» даёт Go)', async () => {
    answer = async () => view(2, [...BINS, { id: 'b3', name: 'Подборка 1' }])
    await create('  ', [])
    expect(calls).toEqual([['new', '  ', []]])
  })

  it('ошибка Go — текстом для поля', async () => {
    answer = async () => {
      throw new Error('подборок не больше 50')
    }
    expect(await create('Новая', [])).toEqual({ ok: false, err: 'подборок не больше 50' })
    expect(binsel.target).toBe('')
  })
})

describe('rename', () => {
  it('имя другое — вызов с очищенным именем', async () => {
    answer = async () => view(2)
    expect((await rename('b2', '  Резерв ')).ok).toBe(true)
    expect(calls).toEqual([['rename', 'b2', 'Резерв']])
  })

  it('пустое и прежнее имя — холостая правка без вызова', async () => {
    expect(await rename('b2', '   ')).toEqual({ ok: true })
    expect(await rename('b2', 'Запас')).toEqual({ ok: true })
    expect(calls).toEqual([])
  })

  it('имя другой подборки — «Такая подборка уже есть», свой регистр — можно', async () => {
    expect(await rename('b2', 'обязательно')).toEqual({ ok: false, err: 'Такая подборка уже есть' })
    answer = async () => view(2)
    expect((await rename('b2', 'ЗАПАС')).ok).toBe(true)
    expect(calls).toEqual([['rename', 'b2', 'ЗАПАС']])
  })

  it('подборки больше нет — холостая', async () => {
    expect(await rename('gone', 'X')).toEqual({ ok: true })
    expect(calls).toEqual([])
  })
})

describe('remove', () => {
  it('без вопроса: вызов, тост с «Вернуть»', async () => {
    answer = async () => view(2, [BINS[0]])
    expect(await remove('b2')).toBe(true)
    expect(calls).toEqual([['delete', 'b2']])
    expect(store.toast?.text).toBe('Подборка «Запас» удалена')
    expect(store.toast?.action?.label).toBe('Вернуть')
  })

  it('неизвестная подборка — ничего', async () => {
    expect(await remove('gone')).toBe(false)
    expect(calls).toEqual([])
    expect(store.toast).toBeNull()
  })

  it('ошибка Go — тост-ошибка, без «Вернуть»', async () => {
    answer = async () => {
      throw new Error('сбой')
    }
    expect(await remove('b2')).toBe(false)
    expect(store.toast?.kind).toBe('error')
    expect(store.toast?.action).toBeUndefined()
  })
})

describe('setFor', () => {
  it('добавить: считает только новых, целевая — эта подборка, тост «+N» с «Вернуть»', async () => {
    answer = async () => view(2)
    expect(await setFor('b1', ['a', 'c', 'd'], true)).toBe(true)
    expect(calls).toEqual([['set', 'b1', ['a', 'c', 'd'], true]])
    expect(binsel.target).toBe('b1')
    expect(store.toast?.text).toBe('В «Обязательно»: +2')
    expect(store.toast?.action?.label).toBe('Вернуть')
  })

  it('убрать: тост «−N»', async () => {
    answer = async () => view(2)
    expect(await setFor('b1', ['a', 'b', 'c'], false)).toBe(true)
    expect(calls[0]).toEqual(['set', 'b1', ['a', 'b', 'c'], false])
    expect(store.toast?.text).toBe('Из «Обязательно»: −2')
  })

  it('всё уже так — тост без вызова и без шага', async () => {
    expect(await setFor('b1', ['a', 'b'], true)).toBe(false)
    expect(store.toast?.text).toBe('Эти фото уже в «Обязательно»')
    expect(store.toast?.action).toBeUndefined()
    expect(await setFor('b2', ['a'], false)).toBe(false)
    expect(store.toast?.text).toBe('Этих фото нет в «Запас»')
    expect(calls).toEqual([])
    expect(binsel.target).toBe('')
  })

  it('без выделения — тост, вызова нет', async () => {
    expect(await setFor('b1', [], true)).toBe(false)
    expect(store.toast?.text).toBe('Выделите фото в ленте')
    expect(calls).toEqual([])
  })

  it('ошибка Go (предел отметок) — тост-ошибка, цель не меняется', async () => {
    answer = async () => {
      throw new Error('слишком много своих отметок (больше 100 000)')
    }
    expect(await setFor('b1', ['c'], true)).toBe(false)
    expect(store.toast?.kind).toBe('error')
    expect(binsel.target).toBe('')
  })

  it('подборки больше нет — холостая', async () => {
    expect(await setFor('gone', ['a'], true)).toBe(false)
    expect(calls).toEqual([])
  })

  it('Go ответил холостой правкой (rev тот же): тост «уже» без «Вернуть», цель не меняется', async () => {
    answer = async () => view(1)
    expect(await setFor('b1', ['c'], true)).toBe(false)
    expect(calls).toHaveLength(1)
    expect(store.toast?.text).toBe('Эти фото уже в «Обязательно»')
    expect(store.toast?.action).toBeUndefined()
    expect(binsel.target).toBe('')
  })

  it('быстрый второй B, пока индекс членства не обновился: второй вызов холостой — тост «уже», не «+N»', async () => {
    let rev = 1
    answer = async () => view(rev === 1 ? ++rev : rev) // первая правка меняет rev, вторая — нет
    select('c')
    binsel.target = 'b1'
    const first = toggleSelected()
    const second = toggleSelected() // индекс всё ещё старый → снова «добавить»
    expect(await first).toBe(true)
    expect(await second).toBe(false)
    expect(calls).toHaveLength(2)
    expect(store.toast?.text).toBe('Эти фото уже в «Обязательно»')
    expect(store.toast?.action).toBeUndefined()
  })
})

describe('toggleSelected (B)', () => {
  it('без выделения — тост, вызова нет, окно не открывается', async () => {
    expect(await toggleSelected()).toBe(false)
    expect(store.toast?.text).toBe('Выделите фото в ленте')
    expect(binsel.open).toBe(false)
    expect(calls).toEqual([])
  })

  it('нет подборок — тост и окно открывается', async () => {
    store.apply(view(2, []) as never)
    select('a')
    expect(await toggleSelected()).toBe(false)
    expect(store.toast?.text).toBe('Сначала создайте подборку: кнопка «Подборки»')
    expect(binsel.open).toBe(true)
    expect(calls).toEqual([])
  })

  it('нет подборок — окно сразу со строкой «Новая из выделенных» (как Shift+B): поле с фокусом, выделение не брошенное', async () => {
    store.apply(view(2, []) as never)
    select('a', 'b')
    binsel.pending = ['x']
    await toggleSelected()
    expect(binsel.wantNew).toBe(true)
    expect(binsel.pending).toEqual([])
  })

  it('нет подборок, кнопка в «⋯» — тост называет путь через «⋯»', async () => {
    store.apply(view(2, []) as never)
    select('a')
    binsel.host = 'more'
    await toggleSelected()
    expect(store.toast?.text).toBe('Сначала создайте подборку: кнопка «⋯» → «Подборки»')
    expect(binsel.open).toBe(true)
    binsel.host = 'bar'
  })

  it('подборки есть, целевой нет — тост и окно (выбрать)', async () => {
    select('a')
    expect(await toggleSelected()).toBe(false)
    expect(store.toast?.text).toBe('Выберите подборку в окне «Подборки»')
    expect(binsel.open).toBe(true)
    expect(calls).toEqual([])
  })

  it('показанная подборка — целевая: новые фото добавляются', async () => {
    answer = async () => view(2)
    binsel.show('b1')
    select('c', 'd')
    expect(await toggleSelected()).toBe(true)
    expect(calls).toEqual([['set', 'b1', ['c', 'd'], true]])
  })

  it('повтор у всех выделенных, уже входящих в подборку, — убрать', async () => {
    answer = async () => view(2)
    binsel.target = 'b1'
    select('a', 'b')
    expect(await toggleSelected()).toBe(true)
    expect(calls).toEqual([['set', 'b1', ['a', 'b'], false]])
  })

  it('часть выделенных уже там — добавить остальных; при «Все фото» целевая — последняя', async () => {
    answer = async () => view(2)
    binsel.target = 'b1'
    select('a', 'd')
    await toggleSelected()
    expect(calls).toEqual([['set', 'b1', ['a', 'd'], true]])
    expect(store.toast?.text).toBe('В «Обязательно»: +1')
  })

  it('целевая подборка удалена (prune) — как без целевой', async () => {
    binsel.target = 'gone'
    select('a')
    expect(await toggleSelected()).toBe(false)
    expect(binsel.open).toBe(true)
    expect(calls).toEqual([])
  })
})

describe('removeSelected (Del)', () => {
  it('в показанной подборке — убрать выделенные', async () => {
    answer = async () => view(2)
    binsel.show('b1')
    select('a', 'b')
    expect(await removeSelected()).toBe(true)
    expect(calls).toEqual([['set', 'b1', ['a', 'b'], false]])
    expect(store.toast?.text).toBe('Из «Обязательно»: −2')
  })

  it('«Все фото» — тост-подсказка, вызова нет', async () => {
    select('a')
    expect(await removeSelected()).toBe(false)
    expect(store.toast?.text).toBe('Del убирает фото из подборки — сначала покажите подборку')
    expect(calls).toEqual([])
  })

  it('выделенных в подборке нет — «Этих фото нет», шага нет', async () => {
    binsel.show('b1')
    select('d')
    expect(await removeSelected()).toBe(false)
    expect(store.toast?.text).toBe('Этих фото нет в «Обязательно»')
    expect(calls).toEqual([])
  })

  it('на «Простом» подборка не показана: Del — подсказка, лента не режется', async () => {
    binsel.show('b1')
    prefs.level = 'simple'
    select('a')
    expect(await removeSelected()).toBe(false)
    expect(calls).toEqual([])
  })
})

describe('drop (бросок на кнопку)', () => {
  it('есть целевая — сразу в неё, тост «+N», окно не открывается', async () => {
    answer = async () => view(2)
    binsel.target = 'b2'
    expect(await drop(['a', 'd'])).toBe(true)
    expect(calls).toEqual([['set', 'b2', ['a', 'd'], true]])
    expect(store.toast?.text).toBe('В «Запас»: +2')
    expect(binsel.open).toBe(false)
    expect(binsel.pending).toEqual([])
  })

  it('показанная — целевая', async () => {
    answer = async () => view(2)
    binsel.show('b1')
    await drop(['d'])
    expect(calls).toEqual([['set', 'b1', ['d'], true]])
  })

  it('все брошенные уже в подборке — тост «уже», шага нет', async () => {
    binsel.target = 'b1'
    expect(await drop(['a'])).toBe(false)
    expect(store.toast?.text).toBe('Эти фото уже в «Обязательно»')
    expect(calls).toEqual([])
  })

  it('целевой нет — окно открывается, брошенные ждут в pending, Go не зовётся', async () => {
    expect(await drop(['c', 'd'])).toBe(false)
    expect(binsel.pending).toEqual(['c', 'd'])
    expect(binsel.open).toBe(true)
    expect(calls).toEqual([])
  })

  it('нет подборок вообще — окно с брошенными (создать из них)', async () => {
    store.apply(view(2, []) as never)
    await drop(['a'])
    expect(binsel.pending).toEqual(['a'])
    expect(binsel.open).toBe(true)
  })

  it('пустой бросок — ничего', async () => {
    expect(await drop([])).toBe(false)
    expect(binsel.open).toBe(false)
  })
})

describe('openNew (Shift+B)', () => {
  it('окно открывается с флагом «новая строка», pending сброшен', () => {
    binsel.pending = ['x']
    openNew()
    expect(binsel.open).toBe(true)
    expect(binsel.wantNew).toBe(true)
    expect(binsel.pending).toEqual([])
  })
})
