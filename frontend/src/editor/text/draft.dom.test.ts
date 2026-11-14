// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'

// 131: заготовка на холсте — бледная, с пунктиром и подсказкой; левый щелчок и правка работают на любом уровне.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))
vi.mock('./fonts', async (orig) => ({ ...(await orig<object>()), loadFonts: async () => {} }))

const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { tip } = await import('../../base/ui/tip.svelte')
const { edit } = await import('./edit.svelte')
const { default: TextLayer } = await import('./TextLayer.svelte')
const { DRAFT_TIP } = await import('./draft')

const style = { font: 'montserrat', size: 12, color: '#333333', align: 'center' }
const txt = (id: string, draft?: boolean) => ({
  id,
  x: 10,
  y: 10,
  w: 80,
  h: 20,
  rot: 0,
  z: 0,
  auto: false,
  text: '[Имена]',
  style,
  lines: [],
  ...(draft ? { draft } : {}),
})
const spread = { id: 's1', w: 600, h: 300, texts: [txt('d1', true), txt('t1')], captions: [], stubs: [] }
const tg = {
  editing: () => false,
  eff: (t: { x: number; y: number; w: number; h: number; rot: number }) => ({
    mode: 'idle',
    x: t.x,
    y: t.y,
    w: t.w,
    h: t.h,
    rot: t.rot,
    scale: 1,
  }),
  inGroup: () => false,
  selected: () => false,
  start: vi.fn(),
  rect: () => null,
  root: null,
}

beforeEach(() => {
  store.view = { ...store.view, spreads: [spread] } as never
  prefs.level = 'simple'
  edit.clear()
  tip.hide()
})
afterEach(() => cleanup())

const mount = (mini = false) => render(TextLayer, { spread: spread as never, tg: tg as never, mini })
const hits = () => Array.from(document.querySelectorAll<HTMLElement>('.hit.block'))

describe('заготовка на холсте', () => {
  it('у заготовки пунктирная рамка и бледный блок, у обычной надписи — нет', () => {
    mount()
    expect(document.querySelectorAll('.dash')).toHaveLength(1)
    expect(document.querySelectorAll('.blk.draft')).toHaveLength(1)
  })
  it('на мини-развороте рамка тоже видна, рамки для мыши нет', () => {
    mount(true)
    expect(document.querySelectorAll('.dash')).toHaveLength(1)
    expect(hits()).toHaveLength(0)
  })
  it('подсказка при наведении — слова спеки; у обычной надписи подсказки нет', async () => {
    mount()
    const [draft, plain] = hits()
    await fireEvent.pointerEnter(draft)
    expect(tip.state?.text).toBe(DRAFT_TIP)
    tip.hide()
    await fireEvent.pointerEnter(plain)
    expect(tip.state).toBeNull()
  })
  it('«Простой» уровень: заготовка выделяется щелчком, обычная надпись — нет (вопрос 20)', async () => {
    mount()
    const [draft, plain] = hits()
    await fireEvent.pointerDown(draft, { button: 0 })
    expect(edit.sel).toMatchObject({ kind: 'text', id: 'd1' })
    edit.clear()
    await fireEvent.pointerDown(plain, { button: 0 })
    expect(edit.sel).toBeNull()
  })
  it('двойной щелчок по заготовке входит в правку с выделенным текстом', async () => {
    mount()
    await fireEvent.pointerDown(hits()[0], { button: 0 })
    await fireEvent.dblClick(hits()[0])
    expect(edit.editing).toBe(true)
    expect(edit.selectAll).toBe(true)
  })
})
