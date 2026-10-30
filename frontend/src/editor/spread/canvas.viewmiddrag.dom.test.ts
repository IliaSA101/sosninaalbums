// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'
import type { View } from '../../base/core/types'

// 149 Т5 шаг 1: агент прислал правку, пока человек тащит фото по холсту, — Go шлёт событие `view` (как вторым
// клиентам). Холст обязан пережить: перенос не рвётся с ошибкой, фото не двоится и не теряется, призрак не
// залипает, отпускание применяется к актуальному виду или честно отменяется. Go подменён (AddPhotos, GetView);
// события `view` подаёт тест через store.apply — ровно то, что делает подписка store.init.
const m = vi.hoisted(() => ({ add: vi.fn(), get: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ AddPhotos: m.add, GetView: m.get }))

const { default: SpreadSlots } = await import('./SpreadSlots.svelte')
const { default: Overlay } = await import('../../base/drag/Overlay.svelte')
const { store } = await import('../../base/core/store.svelte')
const { drag } = await import('../../base/drag/drag.svelte')
const { row } = await import('./spreadrow.svelte')
const { pitch, slotCenters } = await import('./spreadrow')

const rect = (x: number) => ({ x, y: 0, w: 100, h: 100 })
const fr = { s: 1, x: 0, y: 0, r: 0, img: { x: 0, y: 0, w: 40, h: 60 }, cellRot: 0, dpi: 300 }
const sp = (id: string, photos: string[]) => ({
  id,
  name: '',
  kind: '',
  photos,
  rects: photos.map((_, i) => rect(i * 100)),
  frames: photos.map(() => fr),
  texts: [],
  captions: [],
  backdrops: [],
  w: 400,
  h: 200,
  mid: 200,
})
type Album = Record<string, string[]>
// Вид альбома: порядок и состав разворотов; seq не ставим — вид приходит «мимо цепочки» (как в соседних тестах).
const album = (rev: number, a: Album) =>
  ({ ...store.view, seq: 0, rev, spreads: Object.entries(a).map(([id, p]) => sp(id, p)), settings: {} }) as unknown as View
const photosOf = () => Object.fromEntries(store.view.spreads.map((s) => [s.id, s.photos]))

const base = { w: 400, h: 200, mid: 200 }
const fit = { left: 500, top: 50, width: 400, height: 200 }
const mountRow = (n: number) => {
  const centers = slotCenters(Array(n).fill(400))
  row.geo(pitch(400), n, false, centers)
  return render(SpreadSlots, {
    settings: store.view.settings,
    k: 1,
    base,
    spreadW: 400,
    w: 1400,
    fit,
    place: fit,
    zoomed: false,
    centers,
    onhover: () => {},
  })
}
const peer = (id: string) => document.querySelector<HTMLElement>(`[data-drop="spread"][data-spread="${id}"]`)
// Событие `view` от Go; Stage при смене числа листов сообщает ряду новые размеры — здесь это делает тест.
const pushView = (rev: number, a: Album) => {
  const n = Object.keys(a).length
  row.geo(pitch(400), n, false, slotCenters(Array(n).fill(400)))
  store.apply(album(rev, a))
}
const cell = (photo: string) => document.querySelector<HTMLElement>(`[data-drop="cell"][data-photo="${photo}"]`)
const ghost = () => document.querySelector('.ghost-stack')

// Цель под курсором определяет тест по живому DOM: что смонтировано — то и «под мышью».
let under: () => Element | null = () => null
const ptr = (type: string, x = 50) => {
  window.dispatchEvent(new MouseEvent(type, { clientX: x, clientY: 40, buttons: type === 'pointerup' ? 0 : 1 }))
  flushSync()
}
async function grab(photo: string, x = 10) {
  await fireEvent.pointerDown(cell(photo)!, { button: 0, clientX: x, clientY: 40 })
  ptr('pointermove', x + 20)
}

beforeEach(async () => {
  await new Promise((r) => setTimeout(r, 0)) // гашение щелчка после прежнего броска
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
    left: 0,
    top: 0,
    width: 100,
    height: 100,
    right: 100,
    bottom: 100,
    x: 0,
    y: 0,
    toJSON: () => ({}),
  })
  document.elementsFromPoint = () => {
    const el = under()
    return el ? [el] : []
  }
  under = () => null
  for (const f of Object.values(m)) f.mockReset()
  store.toast = null
  store.onCover = false
  store.view = album(1, { s1: ['a', 'b'], s2: ['c'] })
  store.currentId = 's1'
  store.current = 0
  row.moving = false
  mountRow(2)
  render(Overlay)
})
afterEach(() => {
  drag.cancel()
  cleanup()
  vi.restoreAllMocks()
  delete (document as { elementsFromPoint?: unknown }).elementsFromPoint
})

describe('view посреди переноса фото (149 Т5)', () => {
  it('тот же альбом, другой rev: перенос жив, призрак на месте, отпускание применяется к новому виду', async () => {
    await grab('a')
    expect(drag.active).toBe(true)
    expect(ghost()).not.toBeNull()
    under = () => cell('b')
    ptr('pointermove', 90) // правая половина b — «после»
    expect(drag.over).toBe('cell:b:after')

    pushView(2, { s1: ['a', 'b'], s2: ['c'] }) // агент поправил что-то постороннее
    await tick()
    expect(drag.active).toBe(true) // не оборван
    expect(ghost()).not.toBeNull()
    expect(drag.payload).toEqual({ kind: 'photos', ids: ['a'], fromSpread: 's1' })
    expect(store.toast).toBeNull()

    m.add.mockImplementation(async () => album(3, { s1: ['b', 'a'], s2: ['c'] }))
    ptr('pointerup', 90)
    expect(drag.active).toBe(false)
    expect(ghost()).toBeNull()
    expect(document.body.classList.contains('dragging')).toBe(false)
    await vi.waitFor(() => expect(m.add).toHaveBeenCalledOnce())
    expect(m.add).toHaveBeenCalledWith('s1', ['a'], 2)
    await vi.waitFor(() => expect(store.view.rev).toBe(3))
    expect(photosOf()).toEqual({ s1: ['b', 'a'], s2: ['c'] }) // фото одно и на месте
    expect(store.toast).toBeNull()
  })

  it('агент вставил фото перед целью, мышь стоит: бросок идёт на актуальный индекс, а не на прежний', async () => {
    await grab('a')
    under = () => cell('b')
    ptr('pointermove', 90)
    expect(drag.over).toBe('cell:b:after')
    pushView(2, { s1: ['a', 'x', 'b'], s2: ['c'] }) // между a и b встал x: индекс b был 1, стал 2
    await tick()
    expect(drag.active).toBe(true)
    m.add.mockImplementation(async () => album(3, { s1: ['x', 'b', 'a'], s2: ['c'] }))
    ptr('pointerup', 90) // мышь с прошлого движения не сдвигалась
    await vi.waitFor(() => expect(m.add).toHaveBeenCalledOnce())
    expect(m.add).toHaveBeenCalledWith('s1', ['a'], 3) // после b: 2 + 1, не прежнее 2
    await vi.waitFor(() => expect(store.view.rev).toBe(3))
    expect(photosOf()).toEqual({ s1: ['x', 'b', 'a'], s2: ['c'] })
  })

  it('разворот-цель исчез, мышь стоит: бросок отменяется без обращения к Go, фото на месте, призрак гаснет', async () => {
    await grab('a')
    under = () => peer('s2') // сосед в ряду — мини-разворот
    ptr('pointermove', 50)
    expect(drag.over).toBe('spread:s2')
    pushView(2, { s1: ['a', 'b'] }) // s2 удалён целиком
    await tick()
    expect(peer('s2')).toBeNull()
    expect(drag.active).toBe(true) // перенос не оборван видом
    ptr('pointerup', 50)
    expect(drag.active).toBe(false)
    expect(ghost()).toBeNull()
    expect(document.body.classList.contains('dragging')).toBe(false)
    await tick()
    expect(m.add).not.toHaveBeenCalled() // в исчезнувший разворот ничего не шлём
    expect(store.toast).toBeNull()
    expect(photosOf()).toEqual({ s1: ['a', 'b'] })
  })

  it('перетаскиваемое фото убрали из альбома: призрак гаснет по отпусканию, ошибка Go — тост, вид не тронут', async () => {
    await grab('a')
    under = () => cell('b')
    ptr('pointermove', 90)
    pushView(2, { s1: ['b'], s2: ['c'] }) // фото a пропало
    await tick()
    expect(drag.active).toBe(true)
    m.add.mockRejectedValue(new Error('фото не на этом развороте'))
    ptr('pointerup', 90)
    expect(drag.active).toBe(false)
    expect(ghost()).toBeNull()
    await vi.waitFor(() => expect(store.toast?.kind).toBe('error'))
    expect(photosOf()).toEqual({ s1: ['b'], s2: ['c'] })
  })

  it('Esc после view: перенос снят, Go не зовётся, призрак гаснет', async () => {
    await grab('a')
    pushView(2, { s1: ['a', 'b'], s2: ['c', 'd'] })
    await tick()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    flushSync()
    expect(drag.active).toBe(false)
    expect(ghost()).toBeNull()
    ptr('pointerup')
    expect(m.add).not.toHaveBeenCalled()
  })

  it('дельта не к нашему виду (пересинхрон GetView) посреди переноса: перенос жив, после ответа вид актуален', async () => {
    await grab('a')
    under = () => cell('b')
    ptr('pointermove', 90)
    let answer!: (v: unknown) => void
    m.get.mockReturnValue(new Promise((r) => (answer = r)))
    // base не совпадает с нашим seq — цепочка запрашивает полный вид
    store.apply({ ...album(5, { s1: ['a', 'b'], s2: ['c'] }), seq: 7, base: 6, order: ['s1', 's2'] } as never)
    expect(m.get).toHaveBeenCalledOnce()
    expect(drag.active).toBe(true)
    answer({ ...album(5, { s1: ['a', 'b'], s2: ['c', 'z'] }), seq: 7 })
    await vi.waitFor(() => expect(store.view.rev).toBe(5))
    await tick()
    expect(drag.active).toBe(true)
    expect(ghost()).not.toBeNull()
    m.add.mockImplementation(async () => album(6, { s1: ['b', 'a'], s2: ['c', 'z'] }))
    ptr('pointerup', 90)
    await vi.waitFor(() => expect(store.view.rev).toBe(6))
    expect(photosOf()).toEqual({ s1: ['b', 'a'], s2: ['c', 'z'] })
    expect(drag.active).toBe(false)
  })
})
