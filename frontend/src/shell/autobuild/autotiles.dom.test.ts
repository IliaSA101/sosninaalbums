// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Карточки окна «Автосборка» (AutoTiles): якорь, ⇄, набранное значение; состояние окна — настоящий AutoDialog, Go подменён.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { default: AutoTiles } = await import('./AutoTiles.svelte')
const { AutoDialog } = await import('./autobuild.svelte')
const { unitOf } = await import('../../base/geom/sheet')

const plan = (n: number) => ({
  photos: 20,
  spreads: n,
  minSpreads: 1,
  maxSpreads: 20,
  avg: 20 / n,
  min: 1,
  max: 4,
  reuse: 0,
  from: 1,
  to: 4,
  cuts: 0,
})
const make = () =>
  new AutoDialog(
    Array.from({ length: 20 }, (_, i) => `p${i}`),
    async (_ids, n) => plan(n || 5),
    { spreads: 5 } as never,
  )
const props = (ui: InstanceType<typeof AutoDialog>, o = {}) => ({ ui, unit: unitOf({}), empty: false, low: 1, top: 20, typing: null, ...o })
const n = () => document.getElementById('auto-n') as HTMLInputElement
const per = () => document.getElementById('auto-per') as HTMLInputElement

afterEach(cleanup)

describe('AutoTiles', () => {
  it('число разворотов — из окна; якорь — карточка «разворотов»; ⇄ переводит якорь на «фото на разворот»', async () => {
    const ui = make()
    const { container } = render(AutoTiles, props(ui))
    expect(n().value).toBe('5')
    expect(container.querySelectorAll('.tile.anchor')).toHaveLength(1)
    expect(ui.anchor).toBe('spreads')
    await fireEvent.click(container.querySelector('button.link')!)
    flushSync()
    expect(ui.anchor).toBe('per')
    expect(document.activeElement).toBe(per())
    await fireEvent.click(container.querySelector('button.link')!)
    expect(ui.anchor).toBe('spreads')
  })

  it('ввод в поле числа ставит N и якорь; набранное остаётся в поле, пока оно в фокусе, а на blur — фактическое', async () => {
    const ui = make()
    render(AutoTiles, props(ui))
    n().focus()
    n().value = '8'
    await fireEvent.input(n())
    flushSync()
    expect(ui.n).toBe(8)
    n().value = ''
    await fireEvent.input(n())
    flushSync()
    expect(n().value).toBe('') // пусто — набирают, число не трогаем
    expect(ui.n).toBe(8)
    await fireEvent.blur(n())
    flushSync()
    expect(n().value).toBe('8')
  })

  it('щелчок по карточке «фото на разворот» делает её якорем; повторный щелчок по якорю ничего не меняет', async () => {
    const ui = make()
    const { container } = render(AutoTiles, props(ui))
    const tile = container.querySelector('label[for="auto-per"]')!
    await fireEvent.pointerDown(tile)
    expect(ui.anchor).toBe('per')
    await fireEvent.pointerDown(tile)
    expect(ui.anchor).toBe('per')
  })

  it('нет фото для сборки (empty): поля и ⇄ серые, подпись «Нет фото для сборки»', () => {
    const ui = make()
    const { container } = render(AutoTiles, props(ui, { empty: true }))
    expect(n().disabled).toBe(true)
    expect(per().disabled).toBe(true)
    expect((container.querySelector('button.link') as HTMLButtonElement).disabled).toBe(true)
    expect(container.textContent).toContain('Нет фото для сборки')
  })

  it('альбом листов: подписи «лист», страниц у N нет', () => {
    const ui = make()
    const { container } = render(AutoTiles, props(ui, { unit: unitOf({ sheet: true }) }))
    expect(container.textContent).toContain('фото на лист')
    expect(container.textContent).not.toContain('стр.')
  })
})
