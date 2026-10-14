import { beforeEach, describe, expect, it, vi } from 'vitest'
import { hadDraft, DRAFT_NOTE } from './draft'

// Go подменён: NewProject отдаёт пустой альбом. Копии не читаем — окно только открывается.
let state = { view: { rev: 1, path: '', folders: [], spreads: [], settings: { sheet: false } }, photos: [] }
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  NewProject: async () => state,
  NewVignetteSheet: async () => ({ ...state, view: { ...state.view, settings: { sheet: true } } }),
  Backups: async () => [],
}))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { newDoc } = await import('./newdoc.svelte')
const { store } = await import('../../base/core/store.svelte')
const { backupsUi } = await import('../dialogs/backups.svelte')
const { COMMANDS } = await import('../menu/commands')

// Текущий проект до «Нового»: путь, число разворотов и папок.
function before(path: string, spreads: number, folders: number) {
  store.view = {
    ...store.view,
    path,
    spreads: Array.from({ length: spreads }, (_, i) => ({ id: 's' + i })) as never,
    folders: Array.from({ length: folders }, (_, i) => ({ id: 'f' + i })) as never,
  }
  store.toast = null
  backupsUi.open = false
}

const blank = state

describe('hadDraft', () => {
  it('черновик с содержимым — да; пустой и сохранённый — нет', () => {
    expect(hadDraft({ path: '', spreads: [1], folders: [] })).toBe(true)
    expect(hadDraft({ path: '', spreads: [], folders: [1] })).toBe(true)
    expect(hadDraft({ path: '', spreads: [], folders: [] })).toBe(false)
    expect(hadDraft({ path: 'a.salbum', spreads: [1], folders: [1] })).toBe(false)
  })
})

describe('«Новый альбом» и черновик', () => {
  beforeEach(() => {
    newDoc.kind = 'album'
    state = blank
  })

  it('черновик с разворотами — тост с «Открыть копии», кнопка открывает окно копий', async () => {
    before('', 2, 0)
    expect(await newDoc.create()).toBe(true)
    expect(store.toast?.text).toBe(DRAFT_NOTE)
    expect(store.toast?.action?.label).toBe('Открыть копии')
    store.toast?.action?.run()
    expect(backupsUi.open).toBe(true)
  })

  it('черновик только с папками — тоже тост (меню «Файл ▸ Новый»)', async () => {
    before('', 0, 1)
    await COMMANDS.new(undefined)
    expect(store.toast?.text).toBe(DRAFT_NOTE)
  })

  it('пустой черновик — без тоста', async () => {
    before('', 0, 0)
    await newDoc.create()
    expect(store.toast).toBeNull()
  })

  it('сохранённый альбом — без тоста', async () => {
    before('album.salbum', 3, 1)
    await newDoc.create()
    expect(store.toast).toBeNull()
  })

  it('новый лист тоже говорит о черновике', async () => {
    before('', 1, 0)
    newDoc.kind = 'sheet'
    newDoc.choose(0)
    expect(await newDoc.create()).toBe(true)
    expect(store.toast?.text).toBe(DRAFT_NOTE)
  })

  it('Go отказал (черновик занят) и вернул прежнее состояние — тоста о копиях нет', async () => {
    before('', 2, 0)
    state = { ...blank, view: { ...blank.view, spreads: [{ id: 's0' }, { id: 's1' }] as never } }
    expect(await newDoc.create()).toBe(true)
    expect(store.toast).toBeNull()
  })
})
