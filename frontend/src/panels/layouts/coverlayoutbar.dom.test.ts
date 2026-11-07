// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'
import type { CoverSide } from '../../base/types/print'

// Полоса «‹ ▦ ›» над сторонками обложки (127): группы в LayoutBar, метка «· Ручная ·», что делают ‹ › и ▦. Go подменён.
const api = vi.hoisted(() => ({ step: vi.fn(async (..._a: unknown[]) => ({})) }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ StepCoverGrid: api.step, Marks: async () => ({}) }))

const { default: LayoutBar } = await import('./LayoutBar.svelte')
const { store } = await import('../../base/core/store.svelte')
const { gallery } = await import('../gallery/gallery.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { align } = await import('./align.svelte')

const sheet = { id: 'cover', w: 320, h: 260, layout: '', photos: [], grid: true }
const side = (o: Partial<CoverSide> = {}): CoverSide => ({
  scope: 'right',
  label: 'Лицевая',
  box: { x: 180, y: 30, w: 100, h: 200 },
  places: 3,
  known: true,
  alone: false,
  ...o,
})
function setup(sides: CoverSide[], o: { compact?: boolean } = {}) {
  store.view = { rev: 1, folders: [], spreads: [], settings: { pageW: 150, pageH: 260 }, cover: { sheet, sides } } as never
  store.onCover = true
  return render(LayoutBar, { spread: sheet as never, show: true, compact: o.compact })
}
const by = (re: RegExp) =>
  [...document.querySelectorAll<HTMLButtonElement>('button')].filter((b) => re.test(b.getAttribute('aria-label') ?? ''))

let open: ReturnType<typeof vi.spyOn>
beforeEach(() => {
  api.step.mockClear()
  prefs.level = 'full'
  prefs.custom = null
  api.step.mockImplementation(async () => ({ ...store.view, rev: store.view.rev + 1 }))
  open = vi.spyOn(gallery, 'open').mockImplementation(() => {})
})
afterEach(() => {
  cleanup()
  open.mockRestore()
  gallery.cur = null
  store.onCover = false
})

describe('полоса над сторонками обложки', () => {
  it('группа на каждую сторонку из Sides; зона пуста — группы нет', () => {
    setup([side({ scope: 'left', label: 'Задняя', box: { x: 20, y: 30, w: 100, h: 200 } }), side()])
    expect(document.querySelectorAll('.group')).toHaveLength(2)
    expect(by(/^Раскладки/).map((b) => b.getAttribute('aria-label'))).toEqual(['Раскладки задней', 'Раскладки лицевой'])
    cleanup()
    setup([side({ scope: 'left', label: 'Задняя', box: { x: 20, y: 30, w: 100, h: 200 } })])
    expect(document.querySelectorAll('.group')).toHaveLength(1)
  })
  it('полоса видна у обложки с Sides; в зуме (compact) и без Sides — нет', () => {
    setup([side()])
    expect(document.querySelector('.layout-bar')!.classList.contains('visible')).toBe(true)
    cleanup()
    setup([side()], { compact: true })
    expect(document.querySelector('.layout-bar')!.classList.contains('visible')).toBe(false)
    expect(document.querySelectorAll('.group')).toHaveLength(0)
    cleanup()
    setup([])
    expect(document.querySelector('.layout-bar')!.classList.contains('visible')).toBe(false)
  })
  it('«· Ручная ·» при !known и местах; при known и без мест — нет', () => {
    setup([side({ known: false })])
    expect(document.body.textContent).toContain('· Ручная ·')
    cleanup()
    setup([side()])
    expect(document.body.textContent).not.toContain('Ручная')
    cleanup()
    setup([side({ known: false, places: 0 })])
    expect(document.body.textContent).not.toContain('Ручная')
  })
  it('вид с тем же rev, но known = true (холостая постановка своего шаблона, 132-9) снимает «· Ручная ·» и возвращает листание', async () => {
    setup([side({ known: false })])
    expect(document.body.textContent).toContain('· Ручная ·')
    store.apply({ ...store.view, cover: { sheet, sides: [side({ known: true })] } } as never)
    await tick()
    expect(store.view.rev).toBe(1)
    expect(document.body.textContent).not.toContain('Ручная')
    await fireEvent.click(by(/^Следующая/)[0])
    expect(api.step).toHaveBeenCalledWith('right', 1)
  })
  it('› при узнанной форме — StepCoverGrid(scope, ±1), галерея не открывается', async () => {
    setup([side()])
    await fireEvent.click(by(/^Следующая/)[0])
    await fireEvent.click(by(/^Предыдущая/)[0])
    expect(api.step.mock.calls).toEqual([
      ['right', 1],
      ['right', -1],
    ])
    expect(open).not.toHaveBeenCalled()
  })
  it('не узнана: ‹ › открывают ▦ сторонки, Go не зовут; 0 мест — тоже ▦', async () => {
    setup([side({ known: false, places: 4 })])
    await fireEvent.click(by(/^Следующая/)[0])
    expect(open).toHaveBeenCalledWith('right', 'grids')
    expect(api.step).not.toHaveBeenCalled()
    cleanup()
    open.mockClear()
    setup([side({ known: false, places: 0 })])
    await fireEvent.click(by(/^Предыдущая/)[0])
    expect(open).toHaveBeenCalledWith('right', 'grids')
  })
  it('▦ — открыть галерею сторонки; в aria-pressed — открыта ли она', async () => {
    setup([side()])
    await fireEvent.click(by(/^Раскладки/)[0])
    expect(open).toHaveBeenCalledWith('right', 'grids')
    cleanup()
    gallery.cur = { spread: 'cover', scope: 'right', tab: 'grids' }
    setup([side()])
    expect(by(/^Раскладки/)[0].getAttribute('aria-pressed')).toBe('true')
  })
  it('‹ › при открытой галерее этой сторонки её не закрывают', async () => {
    gallery.cur = { spread: 'cover', scope: 'right', tab: 'grids' }
    setup([side({ known: false })])
    await fireEvent.click(by(/^Следующая/)[0])
    expect(open).not.toHaveBeenCalled()
  })
  it('alone: стрелки недоступны с подсказкой, ▦ работает', async () => {
    setup([side({ alone: true })])
    for (const b of [...by(/^Предыдущая/), ...by(/^Следующая/)]) {
      expect(b.disabled).toBe(true)
      expect(b.title).toBe('Других раскладок для этих фото нет')
    }
    await fireEvent.click(by(/^Раскладки/)[0])
    expect(open).toHaveBeenCalled()
    expect(api.step).not.toHaveBeenCalled()
  })
  it('есть выделение (выравнивание) — вместо групп полоса выравнивания', () => {
    const ids = vi.spyOn(align, 'ids', 'get').mockReturnValue(['p1', 'p2'])
    setup([side()])
    expect(by(/^Раскладки/)).toHaveLength(0) // у AlignBar свой .group, считаем по кнопкам ▦
    expect(document.querySelector('.layout-bar')!.classList.contains('visible')).toBe(true)
    ids.mockRestore()
  })
  it('без уровня «Варианты» полосы нет', () => {
    prefs.level = 'custom'
    prefs.custom = { base: 'simple', on: [], off: ['variants'] }
    setup([side()])
    expect(document.querySelectorAll('.group')).toHaveLength(0)
    expect(prefs.can('variants')).toBe(false)
  })
})
