// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// Окно «Макеты альбома» и пример темы (142): пустое окно ведёт к «Наборам», указатель «Ещё макеты…» при demoOnly.
const api = vi.hoisted(() => ({
  catalog: vi.fn(),
  info: vi.fn(),
  count: vi.fn(),
  plan: vi.fn(),
  demoOffer: vi.fn(),
}))
vi.mock('./lookapi', () => ({ lookApi: api }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])) }
})

const { default: LooksDialog } = await import('./LooksDialog.svelte')
const { looks } = await import('./looks.svelte')
const { store } = await import('../../base/core/store.svelte')
const { packsUi } = await import('../packs/packs.svelte')

const A = 'my:0a1b2c3d4e5f6071'
const item = (id: string, name: string, o: Record<string, unknown> = {}) => ({
  id,
  name,
  theme: 'w',
  url: '',
  own: false,
  aspect: 0,
  w: 0,
  h: 0,
  ...o,
})
const catalog = (items: unknown[]) => ({
  kind: 'album',
  themes: [
    { id: 'all', name: 'Все', count: items.length },
    { id: 'w', name: 'Свадьба', count: items.length },
  ],
  items,
  recent: [],
  readOnly: false,
})

const tiles = () => Array.from(document.querySelectorAll<HTMLButtonElement>('button.tile'))
const text = () => document.body.textContent ?? ''
const btn = (label: string) => Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.trim() === label)

async function opened(items: unknown[]) {
  api.catalog.mockResolvedValue(catalog(items))
  render(LooksDialog)
  await looks.show()
  await tick()
}

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  looks.close()
  Object.assign(looks, { cat: null, theme: '', sel: '', info: null, cnt: null, plan: null, demoOffer: false })
  store.toast = null
})
afterEach(() => {
  cleanup()
  looks.close()
  vi.restoreAllMocks()
})

describe('LooksDialog: пример темы (142)', () => {
  it('пустая библиотека, пример есть в «Можно скачать» (142): слова ведут в «Наборы»', async () => {
    const show = vi.spyOn(packsUi, 'show').mockResolvedValue()
    api.demoOffer.mockResolvedValue(true)
    await opened([])
    expect(text()).toContain('Макетов пока нет — пример свадебного макета есть в «Наборах»')
    expect(text()).not.toContain('их приносят наборы библиотеки')
    await fireEvent.click(btn('Наборы…') as HTMLElement)
    expect(show).toHaveBeenCalled()
  })

  it('пустая библиотека, пример удалён или не предложен: прежние слова; ошибка чтения списка наборов — тоже', async () => {
    api.demoOffer.mockResolvedValue(false)
    await opened([])
    expect(text()).toContain('Макетов пока нет — их приносят наборы библиотеки')
    cleanup()
    looks.close()
    api.demoOffer.mockRejectedValue(new Error('нет'))
    await opened([])
    expect(text()).toContain('Макетов пока нет — их приносят наборы библиотеки')
    expect(store.toast).toBeNull()
  })

  it('есть макеты — список наборов не спрашивается', async () => {
    await opened([item(A, 'Светлая', { steps: 9, from: 26 })])
    expect(api.demoOffer).not.toHaveBeenCalled()
    expect(looks.demoOffer).toBe(false)
  })

  it('пример установился под открытым окном (library:changed): пустое окно сменяется плитками, указатель — под ними', async () => {
    api.demoOffer.mockResolvedValue(true)
    await opened([])
    expect(tiles()).toHaveLength(0)
    api.catalog.mockResolvedValue({ ...catalog([item(A, 'Свадьба: синяя акварель', { steps: 4, from: 12 })]), demoOnly: true })
    looks.changed(['Свадьба — синяя акварель (пример)'])
    await vi.waitFor(() => expect(tiles()).toHaveLength(1))
    expect(looks.demoOffer).toBe(false)
    expect(text()).toContain('Ещё макеты — в наборах библиотеки')
  })

  it('demoOnly: под плитками «Ещё макеты — в наборах библиотеки · Наборы…»; ссылка открывает «Наборы»', async () => {
    const show = vi.spyOn(packsUi, 'show').mockResolvedValue()
    api.catalog.mockResolvedValue({ ...catalog([item(A, 'Свадьба: синяя акварель', { steps: 4, from: 12 })]), demoOnly: true })
    render(LooksDialog)
    await looks.show()
    await tick()
    const more = document.querySelector<HTMLElement>('.more')
    expect(more?.textContent?.replace(/\s+/g, ' ').trim()).toBe('Ещё макеты — в наборах библиотеки · Наборы…')
    await fireEvent.click(more!.querySelector('button.link') as HTMLElement)
    expect(show).toHaveBeenCalled()
  })

  it('без demoOnly (есть и другие макеты, поля нет у старого Go) указателя нет', async () => {
    await opened([item(A, 'Светлая', { steps: 9, from: 26 })])
    expect(document.querySelector('.more')).toBeNull()
    expect(text()).not.toContain('Ещё макеты')
  })
})
