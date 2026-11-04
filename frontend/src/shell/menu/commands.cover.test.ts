import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { COMMANDS } from './commands'
import { store } from '../../base/core/store.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { gallery } from '../../panels/gallery/gallery.svelte'
import { bgmode } from '../../editor/backdrop/bgmode.svelte'
import * as App from '../../../wailsjs/go/app/App'
import type { CoverSide, CoverView, SpreadView } from '../../base/core/types'

// Go не зовём: считаем вызовы шага сторонки.
vi.mock('../../../wailsjs/go/app/App', async (orig) => ({
  ...(await orig<typeof import('../../../wailsjs/go/app/App')>()),
  StepCoverGrid: vi.fn(async () => ({})),
}))

// 127-19: Пробел на обложке листает лицевую по coverAct (то же решение, что у ›): шаг, ▦ или тост.
describe('Пробел на обложке (127)', () => {
  const side = (o: Partial<CoverSide> = {}): CoverSide => ({
    scope: 'right',
    label: 'Лицевая',
    box: { x: 0, y: 0, w: 100, h: 100 },
    places: 3,
    known: true,
    alone: false,
    ...o,
  })
  const coverWith = (sides: CoverSide[]) => ({ sheet: { id: 'cover', photos: [] }, sides }) as unknown as CoverView
  const was = { view: store.view, current: store.current, onCover: store.onCover, level: prefs.level, bg: bgmode.cur }
  const stepApi = App.StepCoverGrid as unknown as ReturnType<typeof vi.fn>
  let open: ReturnType<typeof vi.spyOn>
  let notify: ReturnType<typeof vi.spyOn>
  const setup = (sides: CoverSide[], onCover = true) => {
    store.view = {
      ...was.view,
      cover: coverWith(sides),
      spreads: [{ id: 'a', photos: [] } as unknown as SpreadView],
      settings: { ...was.view.settings, sheet: false },
    }
    store.current = 0
    store.onCover = onCover
    prefs.level = 'standard'
  }
  beforeEach(() => {
    open = vi.spyOn(gallery, 'open').mockImplementation(() => {})
    notify = vi.spyOn(store, 'notify').mockImplementation(() => {})
    stepApi.mockClear()
  })
  afterEach(() => {
    open.mockRestore()
    notify.mockRestore()
    gallery.cur = null
    store.view = was.view
    store.current = was.current
    store.onCover = was.onCover
    prefs.level = was.level
    bgmode.cur = was.bg
  })

  it('узнанная форма: Пробел и Shift+Пробел — StepCoverGrid лицевой на +1 и −1', () => {
    setup([side()])
    COMMANDS.nextLayout()
    COMMANDS.prevLayout()
    expect(stepApi.mock.calls).toEqual([
      ['right', 1],
      ['right', -1],
    ])
    expect(open).not.toHaveBeenCalled()
  })

  it('форма не узнана или мест нет — открывается ▦ лицевой, места не трогаются', () => {
    for (const o of [{ known: false }, { places: 0, known: false }]) {
      setup([side(o)])
      COMMANDS.nextLayout()
    }
    expect(open.mock.calls).toEqual([
      ['right', 'grids'],
      ['right', 'grids'],
    ])
    expect(stepApi).not.toHaveBeenCalled()
  })

  it('▦ лицевой уже открыта — повторный Пробел её не переключает', () => {
    setup([side({ known: false })])
    gallery.cur = { spread: 'cover', scope: 'right', tab: 'grids' }
    COMMANDS.nextLayout()
    expect(open).not.toHaveBeenCalled()
  })

  it('в ряду одна раскладка — тост «Других раскладок для этих фото нет», без шага', () => {
    setup([side({ places: 1, alone: true })])
    COMMANDS.nextLayout()
    expect(notify).toHaveBeenCalledExactlyOnceWith('Других раскладок для этих фото нет')
    expect(stepApi).not.toHaveBeenCalled()
    expect(open).not.toHaveBeenCalled()
  })

  it('задняя Пробелом не листается: только задняя в виде — молча; лицевая берётся из пары', () => {
    setup([side({ scope: 'left', label: 'Задняя' })])
    COMMANDS.nextLayout()
    expect(stepApi).not.toHaveBeenCalled()
    expect(open).not.toHaveBeenCalled()
    expect(notify).not.toHaveBeenCalled()
    setup([side({ scope: 'left', label: 'Задняя', known: false }), side()])
    COMMANDS.nextLayout()
    expect(stepApi).toHaveBeenCalledExactlyOnceWith('right', 1)
  })

  it('на обычном развороте обложка не листается', () => {
    setup([side()], false)
    COMMANDS.nextLayout()
    expect(stepApi).not.toHaveBeenCalled()
    expect(open).not.toHaveBeenCalledWith('right', 'grids')
  })

  it('без уровня «Варианты» и в режиме фона — молчит', () => {
    setup([side()])
    const off = vi.spyOn(prefs, 'can').mockImplementation((id) => id !== 'variants')
    COMMANDS.nextLayout()
    off.mockRestore()
    bgmode.cur = { spread: 'cover', scope: 'right' } as typeof bgmode.cur
    COMMANDS.nextLayout()
    expect(stepApi).not.toHaveBeenCalled()
  })

  it('обложки нет в виде — молча', () => {
    setup([side()])
    store.view = { ...store.view, cover: undefined }
    COMMANDS.nextLayout()
    expect(stepApi).not.toHaveBeenCalled()
    expect(notify).not.toHaveBeenCalled()
  })
})
