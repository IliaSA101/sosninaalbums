import { beforeEach, describe, expect, it, vi } from 'vitest'

// 131: заготовка макета. Признак снимает Go; UI входит в правку с выделенным текстом и зовёт SetTextDraft(…, false).
const order: string[] = []
vi.mock('../../base/core/store.svelte', () => ({
  store: {
    run: vi.fn(async (p: Promise<unknown>) => (await p, true)),
    spread: undefined,
    view: { spreads: [] as unknown[] },
    selectedCell: null,
    fonts: [],
  },
  Api: {
    SetText: vi.fn(async (id: string, text: string) => void order.push(`text:${id}:${text}`)),
    SetTextDraft: vi.fn(async (s: string, id: string, d: boolean) => void order.push(`draft:${s}:${id}:${d}`)),
  },
}))

import { edit } from './edit.svelte'
import { Api, store } from '../../base/core/store.svelte'
import { DRAFT_TIP } from './draft'

const sp = (draft: boolean | undefined) => ({ id: 's1', captions: [], texts: [{ id: 't1', text: '[Имена]', draft }] })
function put(draft: boolean | undefined) {
  ;(store as unknown as { view: { spreads: unknown[] } }).view.spreads = [sp(draft)]
  edit.sel = { kind: 'text', spread: 's1', id: 't1' }
  edit.target = null
  edit.closing = false
  edit.selectAll = false
}

beforeEach(() => {
  vi.stubGlobal('window', globalThis)
  order.length = 0
  vi.mocked(Api.SetTextDraft).mockClear()
})

describe('заготовка: вход в правку', () => {
  it('двойной щелчок по заготовке выделяет весь текст', () => {
    put(true)
    edit.startEdit()
    expect(edit.selectAll).toBe(true)
    expect(edit.editing).toBe(true)
  })
  it('обычная надпись — курсор в конец, как раньше', () => {
    put(undefined)
    edit.startEdit()
    expect(edit.selectAll).toBe(false)
  })
  it('новый текст уходит обычным SetText — признак снимает Go, UI SetTextDraft не зовёт', async () => {
    put(true)
    edit.startEdit()
    edit.input('Анна и Павел')
    await edit.idle()
    expect(order).toEqual(['text:t1:Анна и Павел'])
    expect(Api.SetTextDraft).not.toHaveBeenCalled()
  })
})

describe('«Сделать обычной надписью»', () => {
  it('зовёт SetTextDraft(…, false); недосланный набор уходит первым', async () => {
    put(true)
    edit.startEdit()
    edit.input('Свой')
    await edit.undraft('s1', 't1')
    expect(order).toEqual(['text:t1:Свой', 'draft:s1:t1:false'])
  })
  it('без ввода — один вызов', async () => {
    put(true)
    await edit.undraft('s1', 't1')
    expect(order).toEqual(['draft:s1:t1:false'])
  })
})

describe('слова', () => {
  it('подсказка заготовки — словами спеки', () => {
    expect(DRAFT_TIP).toBe('Заготовка — не напечатается, пока вы её не измените')
  })
})
