// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'

// Кнопка «Другие ▸» в шапке галереи вариантов (105-qa-3): быстрый путь к черте «другие». Go подменён.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ LayoutChoices: async () => ({}), Marks: async () => ({}) }))
vi.mock('../../shell/guide/guidestate.svelte', () => ({ guide: { show: () => {} } }))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))

const { default: VariantGallery } = await import('./VariantGallery.svelte')
const { store } = await import('../../base/core/store.svelte')
const { gallery } = await import('./gallery.svelte')

const choice = (key: string, family = 'figure') => ({ key, rects: [], family, twin: -1, formName: 'Разброс', crop: 0 })
const btn = () => [...document.querySelectorAll('header button')].find((b) => b.textContent?.includes('Другие'))

function setup(o: { extra?: boolean; inOrder?: boolean; family?: string; scope?: 'spread' | 'left' } = {}) {
  const sp = { id: 's', layout: 'k0', photos: [], w: 300, h: 300 }
  store.view = { rev: 1, folders: [], spreads: [sp], settings: { pageW: 300, pageH: 300 } } as never
  const scope = o.scope ?? 'spread'
  gallery.cur = { spread: 's', scope, tab: 'layouts' }
  const whole = {
    inOrder: o.inOrder === false ? [] : [choice('k0', 'dense')],
    rearranged: [],
    extra: o.extra === false ? [] : [choice('z1'), choice('z2')],
  }
  gallery.choices = { spread: 's', rev: 1, whole, left: whole, right: whole } as never
  gallery.family = (o.family ?? 'all') as never
  gallery.order = 'inOrder'
  return render(VariantGallery, { g: { spread: 's', scope } })
}

afterEach(() => cleanup())

describe('кнопка «Другие ▸» в шапке галереи', () => {
  it('есть, когда на экране черта «другие»', () => {
    setup()
    expect(btn()).toBeTruthy()
  })

  it.each([
    ['«Других» нет', { extra: false }],
    ['они отфильтрованы другим семейством', { family: 'dense' }],
    ['«Авто» пусто — «другие» и так в начале ряда', { inOrder: false }],
    ['страница: «Других вариантов» у неё нет', { scope: 'left' as const }],
  ])('нет: %s', (_, o) => {
    setup(o)
    expect(btn()).toBeUndefined()
  })

  it('щелчок плавно прокручивает ряд к черте «другие» (с полем 8 px)', async () => {
    setup()
    const sep = document.querySelector<HTMLElement>('[data-sep="extra"]')!
    const sc = document.querySelector<HTMLElement>('.scroll')!
    Object.defineProperty(sep, 'offsetLeft', { value: 2750 })
    sc.scrollTo = vi.fn() as never
    await fireEvent.click(btn()!)
    expect(sc.scrollTo).toHaveBeenCalledWith({ left: 2742, behavior: 'smooth' })
  })
})
