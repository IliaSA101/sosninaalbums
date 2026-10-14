import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../base/core/store.svelte', () => ({
  store: { run: vi.fn(async () => true), spread: undefined, view: { spreads: [] }, selectedCell: null, fonts: [] },
  Api: {},
}))

import { TextGestures } from './textgesture.svelte'
import { canvas } from '../canvas/canvas.svelte'
import type { TextView } from '../../base/core/types'

const text = {
  id: 't1',
  x: 10,
  y: 10,
  w: 40,
  h: 10,
  rot: 0,
  auto: false,
  text: 'a',
  style: { size: 12, align: 'left' },
  lines: [],
  z: 1,
} as unknown as TextView

// Минимальный window: EventTarget; события несут только нужные поля.
const fire = (type: string, props: Record<string, unknown> = {}) =>
  window.dispatchEvent(Object.assign(new Event(type, { cancelable: true }), props))

function gestures() {
  const settings = { pageW: 200, pageH: 200, margin: 5, gap: 2, layflat: false, dpi: 300, bg: '' }
  const spread = { id: 's1', w: 400, h: 200, mid: 200, rects: [], photos: [], texts: [text], captions: [] }
  return new TextGestures(() => ({ spread, settings, width: 800, root: undefined }) as never)
}
const down = { clientX: 0, clientY: 0 } as PointerEvent

beforeEach(() => {
  vi.stubGlobal('window', new EventTarget())
  canvas.busy = false
})
afterEach(() => vi.unstubAllGlobals())

describe('жест текста', () => {
  it('Esc отменяет жест и снимает canvas.busy', () => {
    const tg = gestures()
    tg.start(down, text, 'move')
    expect(canvas.busy).toBe(true)
    fire('pointermove', { clientX: 60, clientY: 40 })
    expect(tg.gesture).not.toBeNull()
    fire('keydown', { key: 'Escape' })
    expect(tg.gesture).toBeNull()
    expect(canvas.busy).toBe(false)
  })

  it('после Esc отпускание кнопки ничего не меняет', () => {
    const tg = gestures()
    tg.start(down, text, 'move')
    fire('pointermove', { clientX: 60, clientY: 40 })
    fire('keydown', { key: 'Escape' })
    fire('pointerup')
    expect(tg.gesture).toBeNull()
    expect(canvas.busy).toBe(false)
  })

  it('другие клавиши жест не трогают', () => {
    const tg = gestures()
    tg.start(down, text, 'move')
    fire('pointermove', { clientX: 60, clientY: 40 })
    fire('keydown', { key: 'a' })
    expect(tg.gesture).not.toBeNull()
    fire('keydown', { key: 'Escape' })
  })
})
