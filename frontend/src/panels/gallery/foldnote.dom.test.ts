// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'

// Строка «Ещё N раскладок с фото на сгибе» в «Других вариантах» (105, В1-в). Go и справка подменены.
const show = vi.hoisted(() => vi.fn())
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ LayoutChoices: async () => ({}), Marks: async () => ({}) }))
vi.mock('../../shell/guide/guidestate.svelte', () => ({ guide: { show } }))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))

const { default: VariantGallery } = await import('./VariantGallery.svelte')
const { store } = await import('../../base/core/store.svelte')
const { gallery } = await import('./gallery.svelte')

const choice = (key: string, family = 'figure') => ({ key, rects: [], family, twin: -1, formName: 'Разброс', crop: 0 })
const note = () => document.querySelector('.foldnote')?.textContent?.replace(/\s+/g, ' ').trim()

function setup(foldHidden: number | undefined, o: { extra?: boolean; family?: string; scope?: 'spread' | 'left' } = {}) {
  const sp = { id: 's', layout: 'k0', photos: [], w: 300, h: 300, foldHidden }
  store.view = { rev: 1, folders: [], spreads: [sp], settings: { pageW: 300, pageH: 300 } } as never
  gallery.cur = { spread: 's', scope: o.scope ?? 'spread', tab: 'layouts' }
  const whole = { inOrder: [choice('k0', 'dense')], rearranged: [], extra: o.extra === false ? [] : [choice('z1'), choice('z2')] }
  gallery.choices = { spread: 's', rev: 1, whole, left: whole, right: whole } as never
  gallery.family = (o.family ?? 'all') as never
  gallery.order = 'inOrder'
  return render(VariantGallery, { g: { spread: 's', scope: o.scope ?? 'spread' } })
}

beforeEach(() => show.mockReset())
afterEach(() => cleanup())

describe('строка сгиба в галерее вариантов', () => {
  it.each([
    [0, undefined],
    [undefined, undefined],
    [1, 'Ещё 1 раскладка с фото на сгибе — Настройки альбома ▸ Любые фото'],
    [2, 'Ещё 2 раскладки с фото на сгибе — Настройки альбома ▸ Любые фото'],
    [5, 'Ещё 5 раскладок с фото на сгибе — Настройки альбома ▸ Любые фото'],
    [8, 'Ещё 8 раскладок с фото на сгибе — Настройки альбома ▸ Любые фото'],
  ])('foldHidden %s', (n, text) => {
    setup(n)
    expect(note()).toBe(text)
  })

  it('щелчок по ссылке открывает «Фото через корешок» (статья fold, место ui:fold-mode)', async () => {
    setup(8)
    await fireEvent.click(document.querySelector('.foldnote button')!)
    expect(show).toHaveBeenCalledWith('fold', 'ui:fold-mode')
  })

  it('строка стоит в «Других вариантах», перед их плитками', () => {
    setup(3)
    const n = document.querySelector('.foldnote')!
    expect(n.previousElementSibling?.className).toContain('sep')
    expect(n.nextElementSibling).not.toBeNull()
  })

  it('«Других вариантов» нет или они отфильтрованы другим семейством — строки нет', () => {
    setup(3, { extra: false })
    expect(note()).toBeUndefined()
    cleanup()
    setup(3, { family: 'dense' })
    expect(note()).toBeUndefined()
  })

  it('фильтр «Фигуры» строку оставляет', () => {
    setup(3, { family: 'figure' })
    expect(note()).toContain('Ещё 3 раскладки')
  })

  it('галерея страницы — строки нет', () => {
    setup(3, { scope: 'left' })
    expect(note()).toBeUndefined()
  })
})
